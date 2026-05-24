package talonsandbox_test

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	sandbox "x.xgit.pro/dark/talon-sandbox-sdk-go"
)

// C3 regression: when the caller has Configure()d a production server and
// then passes additional opts (e.g. WithAPIKey for a different tenant), the
// resulting client must still hit the configured baseURL — not silently
// reset back to "http://localhost:18080".
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

	// Reset any previous global state, configure with srv.URL, then call
	// Get with WithAPIKey (no WithBaseURL). With the bug, the request would
	// go to http://localhost:18080; with the fix it stays on srv.URL.
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
