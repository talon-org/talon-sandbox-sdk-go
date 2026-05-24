package talonsandbox_test

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"testing"

	sandbox "x.xgit.pro/dark/talon-sandbox-sdk-go"
)

// stubSandbox registers GET /v1/sandboxes/{id} on the mux and returns a Sandbox handle.
func stubSandbox(t *testing.T, mux *http.ServeMux, id string, c *sandbox.Client) *sandbox.Sandbox {
	t.Helper()
	mux.HandleFunc("/v1/sandboxes/"+id, func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodGet {
			http.Error(w, "method", 405)
			return
		}
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(sandbox.SandboxInfo{ID: id, State: "running"})
	})
	sb, err := sandbox.Get(context.Background(), id, optionsFromClient(c)...)
	if err != nil {
		t.Fatal(err)
	}
	return sb
}

func TestExpose(t *testing.T) {
	mux := http.NewServeMux()
	c := newTestClient(t, mux)
	sb := stubSandbox(t, mux, "sb_expose", c)

	mux.HandleFunc("/v1/sandboxes/sb_expose/expose", func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			http.Error(w, "method", 405)
			return
		}
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(map[string]any{
			"port":   5173,
			"url":    "https://sb-expose-5173.preview.example.com",
			"signed": false,
		})
	})

	url, err := sb.Expose(context.Background(), 5173)
	if err != nil {
		t.Fatal(err)
	}
	if url != "https://sb-expose-5173.preview.example.com" {
		t.Fatalf("got %q", url)
	}
}

func TestExpose_WithSign(t *testing.T) {
	mux := http.NewServeMux()
	c := newTestClient(t, mux)
	sb := stubSandbox(t, mux, "sb_sign", c)

	var gotBody map[string]any
	mux.HandleFunc("/v1/sandboxes/sb_sign/expose", func(w http.ResponseWriter, r *http.Request) {
		json.NewDecoder(r.Body).Decode(&gotBody)
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(map[string]any{
			"port":       5173,
			"url":        "https://sb-sign-5173.preview.example.com?token=xxx",
			"signed":     true,
			"expires_at": "2026-06-01T00:00:00Z",
		})
	})

	url, err := sb.Expose(context.Background(), 5173, sandbox.ExposeOpts{Sign: true, TTL: "1h", Subdomain: "my-app"})
	if err != nil {
		t.Fatal(err)
	}
	if url == "" {
		t.Fatal("empty url")
	}
	if gotBody["sign"] != true {
		t.Fatalf("sign not sent: %v", gotBody)
	}
	if gotBody["subdomain"] != "my-app" {
		t.Fatalf("subdomain not sent: %v", gotBody)
	}
}

func TestExpose_NotImplemented(t *testing.T) {
	mux := http.NewServeMux()
	c := newTestClient(t, mux)
	sb := stubSandbox(t, mux, "sb_noimpl", c)

	mux.HandleFunc("/v1/sandboxes/sb_noimpl/expose", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(404)
		w.Write([]byte(`{"error":"not found"}`))
	})

	_, err := sb.Expose(context.Background(), 5173)
	if err == nil {
		t.Fatal("expected error")
	}
	if !errors.Is(err, sandbox.ErrNotImplemented) {
		t.Fatalf("expected ErrNotImplemented, got %v", err)
	}
}

func TestUnexpose(t *testing.T) {
	deleted := false
	mux := http.NewServeMux()
	c := newTestClient(t, mux)
	sb := stubSandbox(t, mux, "sb_unexp", c)

	mux.HandleFunc("/v1/sandboxes/sb_unexp/expose/5173", func(w http.ResponseWriter, r *http.Request) {
		if r.Method == http.MethodDelete {
			deleted = true
			w.WriteHeader(204)
		}
	})

	if err := sb.Unexpose(context.Background(), 5173); err != nil {
		t.Fatal(err)
	}
	if !deleted {
		t.Fatal("DELETE not called")
	}
}

func TestExposed(t *testing.T) {
	mux := http.NewServeMux()
	c := newTestClient(t, mux)
	sb := stubSandbox(t, mux, "sb_listed", c)

	mux.HandleFunc("/v1/sandboxes/sb_listed/expose", func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodGet {
			http.Error(w, "method", 405)
			return
		}
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(map[string]any{
			"ports": []sandbox.ExposedPort{
				{Port: 5173, URL: "https://x.example.com", Source: "explicit"},
				{Port: 3000, URL: "https://y.example.com", Source: "dynamic"},
			},
		})
	})

	ports, err := sb.Exposed(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	if len(ports) != 2 {
		t.Fatalf("expected 2 ports, got %d", len(ports))
	}
	if ports[0].Port != 5173 {
		t.Fatalf("port[0] = %d", ports[0].Port)
	}
}

func TestExposed_NotImplemented(t *testing.T) {
	mux := http.NewServeMux()
	c := newTestClient(t, mux)
	sb := stubSandbox(t, mux, "sb_listed2", c)

	mux.HandleFunc("/v1/sandboxes/sb_listed2/expose", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(404)
		w.Write([]byte(`{"error":"not found"}`))
	})

	_, err := sb.Exposed(context.Background())
	if err == nil {
		t.Fatal("expected error")
	}
	if !errors.Is(err, sandbox.ErrNotImplemented) {
		t.Fatalf("expected ErrNotImplemented, got %v", err)
	}
}
