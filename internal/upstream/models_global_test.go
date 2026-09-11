package upstream

import (
	"net/http"
	"testing"

	"workbuddy2api/internal/auth"
)

// TestFetchModelsNoCliAgentFallsBackToAll Global 等 realm 的 agents 数组可能不含
// cli 入口：此时应回落为全部未禁用模型，而非报错（否则调用方只能看到静态 CN 表）。
func TestFetchModelsNoCliAgentFallsBackToAll(t *testing.T) {
	var gotHost string
	c := testClient(func(r *http.Request) (*http.Response, error) {
		gotHost = r.URL.Host
		return jsonResp(200, `{"code":0,"data":{"models":[
			{"id":"hy4-preview","name":"HY4","maxInputTokens":1000000,"maxOutputTokens":32768},
			{"id":"dead-model","name":"Dead","maxInputTokens":100,"maxOutputTokens":10,"disabled":true}
		],"agents":[{"name":"ide","models":["hy4-preview"]}]}}`), nil
	})
	a := &auth.Auth{AccessToken: "at", UID: "u1", Domain: "www.workbuddy.ai"}
	infos, err := c.FetchModels(a)
	if err != nil {
		t.Fatalf("fetch models: %v", err)
	}
	if len(infos) != 1 || infos[0].ID != "hy4-preview" {
		t.Errorf("infos=%+v want [hy4-preview] only (disabled excluded)", infos)
	}
	if gotHost != "chat-global.example" {
		t.Errorf("host=%q want global base for workbuddy.ai account", gotHost)
	}
	if infos[0].ContextWindow != 1000000 {
		t.Errorf("context=%d want 1000000", infos[0].ContextWindow)
	}
}
