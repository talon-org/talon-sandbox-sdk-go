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

func TestEnvGetEmpty(t *testing.T) {
	mux := http.NewServeMux()
	mux.HandleFunc("/v1/sandboxes/sb_test/env/MISSING", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(map[string]string{"value": ""})
	})

	e := newEnv(t, mux)
	val, err := e.Get(context.Background(), "MISSING")
	if err != nil {
		t.Fatal(err)
	}
	if val != "" {
		t.Fatalf("expected empty string, got %q", val)
	}
}

func TestEnvAll(t *testing.T) {
	vars := map[string]string{"NODE_ENV": "production", "PORT": "3000"}
	mux := http.NewServeMux()
	mux.HandleFunc("/v1/sandboxes/sb_test/env", func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodGet {
			http.Error(w, "method", 405)
			return
		}
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(map[string]any{"env": vars})
	})

	e := newEnv(t, mux)
	got, err := e.All(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	for k, want := range vars {
		if got[k] != want {
			t.Fatalf("env[%q] = %q, want %q", k, got[k], want)
		}
	}
	if len(got) != len(vars) {
		t.Fatalf("len(env) = %d, want %d", len(got), len(vars))
	}
}

func TestEnvSet(t *testing.T) {
	var (
		gotMethod string
		gotBody   map[string]string
	)
	mux := http.NewServeMux()
	// Set now uses PUT /v1/sandboxes/{id}/env/{key}
	mux.HandleFunc("/v1/sandboxes/sb_test/env/API_KEY", func(w http.ResponseWriter, r *http.Request) {
		gotMethod = r.Method
		if r.Method != http.MethodPut {
			http.Error(w, "method", 405)
			return
		}
		json.NewDecoder(r.Body).Decode(&gotBody)
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(map[string]any{"env": map[string]string{"API_KEY": "sk-test"}})
	})

	e := newEnv(t, mux)
	if err := e.Set(context.Background(), "API_KEY", "sk-test"); err != nil {
		t.Fatal(err)
	}
	if gotMethod != http.MethodPut {
		t.Fatalf("method = %q, want PUT", gotMethod)
	}
	// body must contain only "value", not "key"
	if _, hasKey := gotBody["key"]; hasKey {
		t.Fatalf("body must not contain 'key' field, got %v", gotBody)
	}
	if gotBody["value"] != "sk-test" {
		t.Fatalf("body[value] = %q, want %q", gotBody["value"], "sk-test")
	}
}

func TestEnvUnset(t *testing.T) {
	var gotMethod string
	mux := http.NewServeMux()
	mux.HandleFunc("/v1/sandboxes/sb_test/env/OLD_VAR", func(w http.ResponseWriter, r *http.Request) {
		gotMethod = r.Method
		if r.Method != http.MethodDelete {
			http.Error(w, "method", 405)
			return
		}
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(map[string]any{"env": map[string]string{}})
	})

	e := newEnv(t, mux)
	if err := e.Unset(context.Background(), "OLD_VAR"); err != nil {
		t.Fatal(err)
	}
	if gotMethod != http.MethodDelete {
		t.Fatalf("method = %q, want DELETE", gotMethod)
	}
}

func TestEnvSetUnsetAll_HTTPErrors(t *testing.T) {
	mux := http.NewServeMux()
	// Return 404 for everything
	mux.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
		http.Error(w, "not found", 404)
	})

	e := newEnv(t, mux)
	ctx := context.Background()

	if _, err := e.All(ctx); err == nil {
		t.Fatal("All: expected error on 404")
	}
	if err := e.Set(ctx, "K", "V"); err == nil {
		t.Fatal("Set: expected error on 404")
	}
	if err := e.Unset(ctx, "K"); err == nil {
		t.Fatal("Unset: expected error on 404")
	}
}
