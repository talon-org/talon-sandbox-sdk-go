package env_test

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"x.xgit.pro/dark/talon-sandbox-sdk-go/env"
)

func newEnv(t *testing.T, mux *http.ServeMux) *env.Env {
	t.Helper()
	srv := httptest.NewServer(mux)
	t.Cleanup(srv.Close)
	return env.New("sb_test", srv.URL, "Bearer test-key", srv.Client())
}

func TestEnvGet(t *testing.T) {
	mux := http.NewServeMux()
	mux.HandleFunc("/v1/sandboxes/sb_test/env/NODE_ENV", func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodGet {
			http.Error(w, "method", 405)
			return
		}
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(map[string]string{"value": "development"})
	})

	e := newEnv(t, mux)
	val, err := e.Get(context.Background(), "NODE_ENV")
	if err != nil {
		t.Fatal(err)
	}
	if val != "development" {
		t.Fatalf("got %q, want %q", val, "development")
	}
}

func TestEnvSet(t *testing.T) {
	var gotBody map[string]string
	mux := http.NewServeMux()
	mux.HandleFunc("/v1/sandboxes/sb_test/env", func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			http.Error(w, "method", 405)
			return
		}
		json.NewDecoder(r.Body).Decode(&gotBody)
		w.WriteHeader(204)
	})

	e := newEnv(t, mux)
	if err := e.Set(context.Background(), "API_KEY", "sk-test"); err != nil {
		t.Fatal(err)
	}
	if gotBody["key"] != "API_KEY" {
		t.Fatalf("key not sent: %v", gotBody)
	}
	if gotBody["value"] != "sk-test" {
		t.Fatalf("value not sent: %v", gotBody)
	}
}
