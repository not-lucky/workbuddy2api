package server

import (
	"io"
	"net/http"
	"strings"
	"testing"

	"workbuddy2api/internal/auth"
	"workbuddy2api/internal/pool"
	"workbuddy2api/internal/upstream"
)

// resetModelsCache 清空包级按 realm 模型缓存（各测试互不污染）。
func resetModelsCache() {
	dynamicModelsCache.Lock()
	dynamicModelsCache.byRealm = nil
	dynamicModelsCache.Unlock()
}

const cnModelsBody = `{"code":0,"data":{"models":[
	{"id":"glm-5.2","name":"GLM-5.2","maxInputTokens":131072,"maxOutputTokens":8192}
],"agents":[{"name":"cli","models":["glm-5.2"]}]}}`

const globalModelsBody = `{"code":0,"data":{"models":[
	{"id":"hy4-preview","name":"HY4","maxInputTokens":1000000,"maxOutputTokens":32768}
],"agents":[{"name":"ide","models":["hy4-preview"]}]}}`

// realmUpstream 按请求 host 返回对应 realm 的模型响应（均无 cli 概念的按需定制）。
func realmUpstream(t *testing.T, bodies map[string]struct {
	status int
	body   string
}) *upstream.Client {
	t.Helper()
	return &upstream.Client{
		HTTP: &http.Client{Transport: roundTripFunc(func(r *http.Request) (*http.Response, error) {
			if b, ok := bodies[r.URL.Host]; ok {
				return &http.Response{
					StatusCode: b.status,
					Header:     http.Header{"Content-Type": []string{"application/json"}},
					Body:       io.NopCloser(strings.NewReader(b.body)),
				}, nil
			}
			return &http.Response{
				StatusCode: 404,
				Header:     http.Header{"Content-Type": []string{"application/json"}},
				Body:       io.NopCloser(strings.NewReader(`{"code":404,"msg":"not found"}`)),
			}, nil
		})},
		ChatBaseCN:        "https://cn.example",
		BillingBaseCN:     "https://cn.example",
		ChatBaseGlobal:    "https://global.example",
		BillingBaseGlobal: "https://global.example",
	}
}

func modelIDs(list []map[string]any) []string {
	ids := make([]string, 0, len(list))
	for _, m := range list {
		if id, _ := m["id"].(string); id != "" {
			ids = append(ids, id)
		}
	}
	return ids
}

func containsID(list []map[string]any, id string) bool {
	for _, got := range modelIDs(list) {
		if got == id {
			return true
		}
	}
	return false
}

// TestModelListGlobalOnly 全球池 + 无 cli 入口响应：必须返回动态 global 模型，
// 而不是静态 CN 表（用户线上故障的回归用例）。
func TestModelListGlobalOnly(t *testing.T) {
	resetModelsCache()
	defer resetModelsCache()
	p := pool.New("")
	p.Add(&auth.Auth{UID: "u-g", Domain: "www.workbuddy.ai", AccessToken: "at"})
	up := realmUpstream(t, map[string]struct {
		status int
		body   string
	}{"global.example": {200, globalModelsBody}})
	h := NewHandler(Config{Pool: p, Upstream: up})
	list := h.modelList()
	if !containsID(list, "hy4-preview") {
		t.Fatalf("ids=%v want hy4-preview from global dynamic", modelIDs(list))
	}
	if containsID(list, "kimi-k2.7") {
		t.Fatalf("ids=%v must not fall back to CN static table", modelIDs(list))
	}
}

// TestModelListMixedUnion 混合池：CN 与 global 动态列表取并集。
func TestModelListMixedUnion(t *testing.T) {
	resetModelsCache()
	defer resetModelsCache()
	p := pool.New("")
	p.Add(&auth.Auth{UID: "u-cn", Domain: "www.codebuddy.cn", AccessToken: "at"})
	p.Add(&auth.Auth{UID: "u-g", Domain: "www.workbuddy.ai", AccessToken: "at"})
	up := realmUpstream(t, map[string]struct {
		status int
		body   string
	}{
		"cn.example":     {200, cnModelsBody},
		"global.example": {200, globalModelsBody},
	})
	h := NewHandler(Config{Pool: p, Upstream: up})
	list := h.modelList()
	if !containsID(list, "glm-5.2") || !containsID(list, "hy4-preview") {
		t.Fatalf("ids=%v want union of both realms", modelIDs(list))
	}
	if len(list) != 2 {
		t.Fatalf("ids=%v want exactly 2 (dedupe)", modelIDs(list))
	}
}

// TestModelListGlobalFetchFails global 动态失败：回退 global 静态表（非 CN 表）。
func TestModelListGlobalFetchFails(t *testing.T) {
	resetModelsCache()
	defer resetModelsCache()
	p := pool.New("")
	p.Add(&auth.Auth{UID: "u-g", Domain: "www.workbuddy.ai", AccessToken: "at"})
	up := realmUpstream(t, map[string]struct {
		status int
		body   string
	}{"global.example": {500, `boom`}})
	h := NewHandler(Config{Pool: p, Upstream: up})
	list := h.modelList()
	if !containsID(list, "hy4-preview") {
		t.Fatalf("ids=%v want hy4-preview from global static fallback", modelIDs(list))
	}
	if containsID(list, "kimi-k2.7") {
		t.Fatalf("ids=%v must not use CN static table for global realm", modelIDs(list))
	}
}
