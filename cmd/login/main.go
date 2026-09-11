// login.go — WorkBuddy OAuth 登录（设备授权流程，双 realm）。
//
// 两个子命令，由 login.sh 顺序驱动：
//
//	login url [cn|global]   → POST /v2/plugin/auth/state?platform=CLI 拿 state+authUrl，
//	                          state 落临时目录（per-region 独立文件），stdout 打印授权 URL
//	login poll [cn|global]  → 读 state，GET /v2/plugin/auth/token?state= 一次，
//	                          成功再 GET /v2/plugin/login/account?state= 拿 uid/nickname，
//	                          stdout 打印完整 token+account JSON
//
// region 缺省 cn（login.sh 无参调用时老 CN 用户行为不变）；非法值直接报错，
// 避免静默把账号登到另一个 realm。无 PKCE（workbuddy 设备流由服务端签发 state）。
package main

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/http/cookiejar"
	"os"
	"path/filepath"
	"time"

	"workbuddy2api/internal/upstream"
)

// 上游 base/origin 按 region 切换：CN（默认）与 Global 同一套 /v2/plugin/* 路径，
// 仅 host 不同（Global 经 PR #23 对 workbuddy.ai 实测可用，platform=CLI 不变）。
const (
	originCN     = "https://www.codebuddy.cn"
	originGlobal = "https://www.workbuddy.ai"
)

// stateDir 返回登录 state 文件目录：WB2A_STATE_DIR 优先，否则系统临时目录。
func stateDir() string {
	if d := os.Getenv("WB2A_STATE_DIR"); d != "" {
		return d
	}
	return os.TempDir()
}

// parseRegion 校验 region 参数；空值缺省 cn（向后兼容 login.sh 无参调用）。
func parseRegion(arg string) (string, error) {
	switch arg {
	case "", "cn":
		return "cn", nil
	case "global":
		return "global", nil
	default:
		return "", fmt.Errorf("invalid region %q (want cn|global)", arg)
	}
}

// bases 返回 region 对应的上游 base、Origin/Referer 与 state 文件。
func bases(region string) (base, origin, stateFile string) {
	if region == "global" {
		return upstream.DefaultChatBaseGlobal, originGlobal,
			filepath.Join(stateDir(), ".wb2api-login-state-global.json")
	}
	return upstream.DefaultChatBaseCN, originCN,
		filepath.Join(stateDir(), ".wb2api-login-state-cn.json")
}

// commonHeaders 通用请求头
func commonHeaders(origin string) func(*http.Request) {
	return func(req *http.Request) {
		req.Header.Set("Content-Type", "application/json")
		req.Header.Set("Accept", "application/json, text/plain, */*")
		req.Header.Set("X-Requested-With", "XMLHttpRequest")
		req.Header.Set("Origin", origin)
		req.Header.Set("Referer", origin+"/")
		req.Header.Set("User-Agent", "CLI/2.63.2 CodeBuddy/2.63.2")
	}
}

// apiEnvelope 与 main.go:429-433 一致
type apiEnvelope struct {
	Code int             `json:"code"`
	Msg  string          `json:"msg"`
	Data json.RawMessage `json:"data"`
}

// doJSON 与 oauth.go:33-66 一致：{code,msg,data} 信封，code!=0 → error
func doJSON(client *http.Client, method, fullURL string, headers func(*http.Request), body io.Reader) (json.RawMessage, int, error) {
	req, err := http.NewRequest(method, fullURL, body)
	if err != nil {
		return nil, 0, err
	}
	if headers != nil {
		headers(req)
	}
	resp, err := client.Do(req)
	if err != nil {
		return nil, 0, err
	}
	defer resp.Body.Close()
	raw, _ := io.ReadAll(resp.Body)
	if resp.StatusCode >= 400 {
		return nil, resp.StatusCode, fmt.Errorf("http_error: upstream %d", resp.StatusCode)
	}
	if resp.StatusCode >= 300 {
		return nil, resp.StatusCode, fmt.Errorf("http_error: upstream redirect %d", resp.StatusCode)
	}
	var env apiEnvelope
	if err := json.Unmarshal(raw, &env); err != nil {
		return nil, resp.StatusCode, fmt.Errorf("parse failed: %w", err)
	}
	if env.Code != 0 {
		return nil, resp.StatusCode, fmt.Errorf("code=%d msg=%s", env.Code, env.Msg)
	}
	return env.Data, resp.StatusCode, nil
}

func fatal(format string, args ...any) {
	fmt.Fprintf(os.Stderr, "login: "+format+"\n", args...)
	os.Exit(1)
}

type loginState struct {
	State string `json:"state"`
}

func main() {
	if len(os.Args) < 2 {
		fatal("usage: login <url|poll> [cn|global] (default cn)")
	}
	sub := os.Args[1]
	regionArg := ""
	if len(os.Args) >= 3 {
		regionArg = os.Args[2]
	}
	region, err := parseRegion(regionArg)
	if err != nil {
		fatal("%v", err)
	}
	base, origin, stateFile := bases(region)
	h := commonHeaders(origin)

	// 每个流程独立 cookie jar（oauth.go:22-29：多账号登录互不串会话）
	jar, _ := cookiejar.New(nil)
	client := &http.Client{Timeout: 30 * time.Second, Jar: jar}

	switch sub {
	case "url":
		// handleStartLogin (oauth.go:68-87)
		data, _, err := doJSON(client, http.MethodPost, base+"/v2/plugin/auth/state?platform=CLI", h, bytes.NewReader([]byte("{}")))
		if err != nil {
			fatal("auth state failed: %v", err)
		}
		var st struct {
			State   string `json:"state"`
			AuthURL string `json:"authUrl"`
		}
		if err := json.Unmarshal(data, &st); err != nil || st.State == "" {
			fatal("auth state: missing state (authUrl may be region-local login page)")
		}
		raw, _ := json.Marshal(loginState{State: st.State})
		if err := os.WriteFile(stateFile, raw, 0o600); err != nil {
			fatal("write state: %v", err)
		}
		url := st.AuthURL
		if url == "" {
			url = base + "/login?state=" + st.State + "&platform=CLI"
		}
		fmt.Println(url)

	case "poll":
		raw, err := os.ReadFile(stateFile)
		if err != nil {
			fatal("read state: %v (先跑 login url %s)", err, regionSuffix(region))
		}
		var ls loginState
		if err := json.Unmarshal(raw, &ls); err != nil {
			fatal("parse state: %v", err)
		}
		// handlePollLogin (oauth.go:108-162)：auth/token 是权威登录状态端点，
		// pending 时业务 code 非 0（"login ing"），完成时 code=0 + token bundle
		tokRaw, status, errTok := doJSON(client, http.MethodGet, base+"/v2/plugin/auth/token?state="+ls.State, h, nil)
		if errTok != nil {
			if status == 0 || status >= 500 {
				fatal("token endpoint error: %v", errTok)
			}
			fatal("登录未完成（waiting for login）。请确认已在浏览器完成登录再按 y")
		}
		var tok struct {
			AccessToken  string `json:"accessToken"`
			RefreshToken string `json:"refreshToken"`
			ExpiresIn    int64  `json:"expiresIn"`
			Domain       string `json:"domain"`
		}
		if err := json.Unmarshal(tokRaw, &tok); err != nil || tok.AccessToken == "" {
			fatal("登录未完成（waiting for login）。请确认已在浏览器完成登录再按 y")
		}
		// login/account 拿 uid/nickname（带 Bearer）
		var acct struct {
			UID          string `json:"uid"`
			EnterpriseID string `json:"enterpriseId"`
			Nickname     string `json:"nickname"`
		}
		acctHeaders := func(r *http.Request) {
			h(r)
			r.Header.Set("Authorization", "Bearer "+tok.AccessToken)
		}
		if acctRaw, _, errAcct := doJSON(client, http.MethodGet, base+"/v2/plugin/login/account?state="+ls.State, acctHeaders, nil); errAcct == nil {
			_ = json.Unmarshal(acctRaw, &acct)
		}
		if tok.Domain == "" && region == "global" {
			tok.Domain = upstream.DefaultGlobalDomain
		}
		out := map[string]any{
			"access_token":  tok.AccessToken,
			"refresh_token": tok.RefreshToken,
			"expires_in":    tok.ExpiresIn,
			"domain":        tok.Domain,
			"uid":           acct.UID,
			"enterprise_id": acct.EnterpriseID,
			"nickname":      acct.Nickname,
		}
		oraw, _ := json.Marshal(out)
		fmt.Println(string(oraw))
		os.Remove(stateFile)

	default:
		fatal("unknown subcommand %q (want url|poll [cn|global])", sub)
	}
}

// regionSuffix 拼 poll 提示里的 region 参数（cn 缺省可省略）。
func regionSuffix(region string) string {
	if region == "cn" {
		return ""
	}
	return region
}
