package talonsandbox_test

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"

	sandbox "x.xgit.pro/dark/talon-sandbox-sdk-go"
)

// newTestClient creates a test server from the given mux and returns a Client
// pointing at it with a fresh httptest client.
func newTestClient(t *testing.T, mux *http.ServeMux) *sandbox.Client {
	t.Helper()
	srv := httptest.NewServer(mux)
	t.Cleanup(srv.Close)
	return sandbox.New(srv.URL, sandbox.WithAPIKey("test-key"), sandbox.WithHTTPClient(srv.Client()))
}

func writeSandboxJSON(w http.ResponseWriter, info sandbox.SandboxInfo, status int) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	json.NewEncoder(w).Encode(info)
}

// optionsFromClient extracts WithBaseURL + WithHTTPClient + WithAPIKey from an
// existing test client so that sandbox.Create/Get/List use the same test server.
func optionsFromClient(c *sandbox.Client) []sandbox.Option {
	return []sandbox.Option{
		sandbox.WithBaseURL(c.BaseURL()),
		sandbox.WithHTTPClient(c.HTTPClient()),
		sandbox.WithAPIKey("test-key"),
	}
}

func TestCreate(t *testing.T) {
	mux := http.NewServeMux()
	mux.HandleFunc("/v1/sandboxes", func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			http.Error(w, "method", 405)
			return
		}
		writeSandboxJSON(w, sandbox.SandboxInfo{
			ID:    "sb_test",
			State: "running",
			Image: "node:20-bookworm",
		}, 201)
	})
	c := newTestClient(t, mux)
	sb, err := sandbox.Create(context.Background(), sandbox.Opts{
		Image:     "node:20-bookworm",
		Resources: sandbox.Resources{CPU: 2, Memory: "4GiB"},
		Network:   "allowlist",
		Timeout:   "30m",
		TTL:       "6h",
	}, optionsFromClient(c)...)
	if err != nil {
		t.Fatal(err)
	}
	if sb.ID() != "sb_test" {
		t.Fatalf("got id %q, want sb_test", sb.ID())
	}
	if sb.State() != "running" {
		t.Fatalf("got state %q, want running", sb.State())
	}
}

func TestCreate_InvalidMemory(t *testing.T) {
	_, err := sandbox.Create(context.Background(), sandbox.Opts{
		Image:     "x",
		Resources: sandbox.Resources{Memory: "notvalid"},
	})
	if err == nil {
		t.Fatal("expected error for invalid memory")
	}
}

func TestCreate_InvalidTimeout(t *testing.T) {
	_, err := sandbox.Create(context.Background(), sandbox.Opts{
		Image:   "x",
		Timeout: "1y",
	})
	if err == nil {
		t.Fatal("expected error for invalid timeout")
	}
}

func TestGet(t *testing.T) {
	mux := http.NewServeMux()
	mux.HandleFunc("/v1/sandboxes/sb_abc", func(w http.ResponseWriter, r *http.Request) {
		writeSandboxJSON(w, sandbox.SandboxInfo{ID: "sb_abc", State: "running"}, 200)
	})
	c := newTestClient(t, mux)
	sb, err := sandbox.Get(context.Background(), "sb_abc", optionsFromClient(c)...)
	if err != nil {
		t.Fatal(err)
	}
	if sb.ID() != "sb_abc" {
		t.Fatalf("got %q", sb.ID())
	}
}

func TestGet_NotFound(t *testing.T) {
	mux := http.NewServeMux()
	mux.HandleFunc("/v1/sandboxes/missing", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(404)
		w.Write([]byte(`{"error":"sandbox not found"}`))
	})
	c := newTestClient(t, mux)
	_, err := sandbox.Get(context.Background(), "missing", optionsFromClient(c)...)
	if err == nil {
		t.Fatal("expected error")
	}
	if !errors.Is(err, sandbox.ErrNotFound) {
		t.Fatalf("expected ErrNotFound, got %v", err)
	}
}

func TestList(t *testing.T) {
	mux := http.NewServeMux()
	mux.HandleFunc("/v1/sandboxes", func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodGet {
			http.Error(w, "method", 405)
			return
		}
		resp := map[string]any{
			"sandboxes": []sandbox.SandboxInfo{
				{ID: "sb_1", State: "running", Labels: map[string]string{"project": "agent-x"}},
				{ID: "sb_2", State: "paused", Labels: map[string]string{"project": "other"}},
			},
		}
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(resp)
	})
	c := newTestClient(t, mux)
	sbs, err := sandbox.List(context.Background(), sandbox.ListOpts{
		Labels: map[string]string{"project": "agent-x"},
	}, optionsFromClient(c)...)
	if err != nil {
		t.Fatal(err)
	}
	if len(sbs) != 1 || sbs[0].ID() != "sb_1" {
		t.Fatalf("expected [sb_1], got %d items", len(sbs))
	}
}

func TestList_NoFilter(t *testing.T) {
	mux := http.NewServeMux()
	mux.HandleFunc("/v1/sandboxes", func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodGet {
			return
		}
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(map[string]any{
			"sandboxes": []sandbox.SandboxInfo{
				{ID: "sb_1", State: "running"},
				{ID: "sb_2", State: "paused"},
			},
		})
	})
	c := newTestClient(t, mux)
	sbs, err := sandbox.List(context.Background(), sandbox.ListOpts{}, optionsFromClient(c)...)
	if err != nil {
		t.Fatal(err)
	}
	if len(sbs) != 2 {
		t.Fatalf("expected 2 sandboxes, got %d", len(sbs))
	}
}

func TestKill(t *testing.T) {
	killed := false
	mux := http.NewServeMux()
	mux.HandleFunc("/v1/sandboxes/sb_kill", func(w http.ResponseWriter, r *http.Request) {
		switch r.Method {
		case http.MethodGet:
			writeSandboxJSON(w, sandbox.SandboxInfo{ID: "sb_kill", State: "running"}, 200)
		case http.MethodDelete:
			killed = true
			w.WriteHeader(204)
		default:
			http.Error(w, "method", 405)
		}
	})
	c := newTestClient(t, mux)
	sb, err := sandbox.Get(context.Background(), "sb_kill", optionsFromClient(c)...)
	if err != nil {
		t.Fatal(err)
	}
	if err := sb.Kill(context.Background()); err != nil {
		t.Fatal(err)
	}
	if !killed {
		t.Fatal("DELETE was not called")
	}
}

func TestPauseResume(t *testing.T) {
	paused, resumed := false, false
	mux := http.NewServeMux()
	mux.HandleFunc("/v1/sandboxes/sb_pr", func(w http.ResponseWriter, r *http.Request) {
		writeSandboxJSON(w, sandbox.SandboxInfo{ID: "sb_pr", State: "running"}, 200)
	})
	mux.HandleFunc("/v1/sandboxes/sb_pr/pause", func(w http.ResponseWriter, r *http.Request) {
		paused = true
		w.WriteHeader(204)
	})
	mux.HandleFunc("/v1/sandboxes/sb_pr/resume", func(w http.ResponseWriter, r *http.Request) {
		resumed = true
		w.WriteHeader(204)
	})
	c := newTestClient(t, mux)
	sb, err := sandbox.Get(context.Background(), "sb_pr", optionsFromClient(c)...)
	if err != nil {
		t.Fatal(err)
	}
	if err := sb.Pause(context.Background()); err != nil {
		t.Fatal(err)
	}
	if err := sb.Resume(context.Background()); err != nil {
		t.Fatal(err)
	}
	if !paused {
		t.Fatal("pause not called")
	}
	if !resumed {
		t.Fatal("resume not called")
	}
}

func TestRefresh(t *testing.T) {
	calls := 0
	mux := http.NewServeMux()
	mux.HandleFunc("/v1/sandboxes/sb_ref", func(w http.ResponseWriter, r *http.Request) {
		calls++
		state := "running"
		if calls > 1 {
			state = "paused"
		}
		writeSandboxJSON(w, sandbox.SandboxInfo{ID: "sb_ref", State: state}, 200)
	})
	c := newTestClient(t, mux)
	sb, err := sandbox.Get(context.Background(), "sb_ref", optionsFromClient(c)...)
	if err != nil {
		t.Fatal(err)
	}
	if sb.State() != "running" {
		t.Fatalf("initial state %q", sb.State())
	}
	info, err := sb.Refresh(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	if info.State != "paused" {
		t.Fatalf("refreshed state %q", info.State)
	}
}
