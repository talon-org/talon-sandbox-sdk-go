package talonsandbox_test

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	sandbox "x.xgit.pro/dark/talon-sandbox-sdk-go"
)

// C3 回归测试：当调用方通过 Configure() 设置了生产端点后，
// 再传入额外 opts（如 WithAPIKey 切换租户），最终客户端必须仍打向
// Configure() 设定的 baseURL，而不是被静默重置为默认端点。
func TestConfigure_InheritedByExplicitOpts(t *testing.T) {
	hit := false
	mux := http.NewServeMux()
	mux.HandleFunc("/v1/sandboxes/sb_x", func(w http.ResponseWriter, _ *http.Request) {
		hit = true
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(sandbox.SandboxInfo{ID: "sb_x", State: "running"})
	})
	srv := httptest.NewServer(mux)
	t.Cleanup(srv.Close)

	// 重置全局状态，用 srv.URL 调用 Configure()，随后以 WithAPIKey（不带
	// WithBaseURL）调用 Get。存在 bug 时请求会打向默认端点；修复后应保持
	// 打向 srv.URL。
	sandbox.ResetConfigureForTest()
	t.Cleanup(sandbox.ResetConfigureForTest)
	sandbox.Configure(srv.URL, "ask_global")

	_, err := sandbox.Get(
		context.Background(),
		"sb_x",
		sandbox.WithAPIKey("ask_override"),
		sandbox.WithHTTPClient(srv.Client()),
	)
	if err != nil {
		t.Fatal(err)
	}
	if !hit {
		t.Fatal("request did not reach configured server (baseURL reverted to default)")
	}
}
