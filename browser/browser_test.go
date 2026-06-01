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

// TestBrowserGet 验证 Browser.Get 发出 GET .../browser 并正确解析响应。
func TestBrowserGet(t *testing.T) {
	mux := http.NewServeMux()
	mux.HandleFunc("/v1/sandboxes/sb_test/browser", func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodGet {
			http.Error(w, "method", 405)
			return
		}
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(browser.BrowserSession{
			SandboxID: "sb_test",
			ProcessID: "proc_cdp",
			CDPPort:   9222,
			CDPPath:   "/devtools/browser/xyz",
			CDPURL:    "ws://localhost:9222/devtools/browser/xyz",
		})
	})

	b := newBrowser(t, mux)
	sess, err := b.Get(context.Background())
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

// TestBrowserGet_NotFound 验证浏览器未启动时 Get 返回错误。
func TestBrowserGet_NotFound(t *testing.T) {
	mux := http.NewServeMux()
	mux.HandleFunc("/v1/sandboxes/sb_test/browser", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(404)
		json.NewEncoder(w).Encode(map[string]string{"error": "browser not running"})
	})

	b := newBrowser(t, mux)
	_, err := b.Get(context.Background())
	if err == nil {
		t.Fatal("expected error when browser not running")
	}
}

// TestBrowserStop 验证 Browser.Stop 发出 DELETE .../browser 并接受 204。
func TestBrowserStop(t *testing.T) {
	stopped := false
	mux := http.NewServeMux()
	mux.HandleFunc("/v1/sandboxes/sb_test/browser", func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodDelete {
			http.Error(w, "method", 405)
			return
		}
		stopped = true
		w.WriteHeader(204)
	})

	b := newBrowser(t, mux)
	if err := b.Stop(context.Background()); err != nil {
		t.Fatal(err)
	}
	if !stopped {
		t.Fatal("DELETE .../browser was not called")
	}
}

// TestBrowserStop_Error 验证服务端返回错误时 Stop 能正确透传。
func TestBrowserStop_Error(t *testing.T) {
	mux := http.NewServeMux()
	mux.HandleFunc("/v1/sandboxes/sb_test/browser", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(404)
		json.NewEncoder(w).Encode(map[string]string{"error": "browser not running"})
	})

	b := newBrowser(t, mux)
	err := b.Stop(context.Background())
	if err == nil {
		t.Fatal("expected error")
	}
}
