package browser_test

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"x.xgit.pro/dark/talon-sandbox-sdk-go/browser"
)

func newBrowser(t *testing.T, mux *http.ServeMux) *browser.Browser {
	t.Helper()
	srv := httptest.NewServer(mux)
	t.Cleanup(srv.Close)
	return browser.New("sb_test", srv.URL, "Bearer test-key", srv.Client())
}

func TestBrowserStart(t *testing.T) {
	mux := http.NewServeMux()
	mux.HandleFunc("/v1/sandboxes/sb_test/browser", func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			http.Error(w, "method", 405)
			return
		}
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(201)
		json.NewEncoder(w).Encode(browser.BrowserSession{
			SandboxID: "sb_test",
			ProcessID: "proc_cdp",
			CDPPort:   9222,
			CDPPath:   "/devtools/browser/abc",
			CDPURL:    "ws://localhost:9222/devtools/browser/abc",
		})
	})

	b := newBrowser(t, mux)
	sess, err := b.Start(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	if sess.CDPPort != 9222 {
		t.Fatalf("cdp_port = %d", sess.CDPPort)
	}
	if sess.CDPURL == "" {
		t.Fatal("cdp_ws_url empty")
	}
}

func TestBrowserStart_Error(t *testing.T) {
	mux := http.NewServeMux()
	mux.HandleFunc("/v1/sandboxes/sb_test/browser", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(400)
		json.NewEncoder(w).Encode(map[string]string{"error": "image does not include chromium"})
	})

	b := newBrowser(t, mux)
	_, err := b.Start(context.Background())
	if err == nil {
		t.Fatal("expected error")
	}
}
