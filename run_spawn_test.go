package talonsandbox_test

import (
	"context"
	"encoding/json"
	"net/http"
	"testing"

	sandbox "x.xgit.pro/dark/talon-sandbox-sdk-go"
)

func TestRun(t *testing.T) {
	callCount := 0
	mux := http.NewServeMux()

	mux.HandleFunc("/v1/sandboxes/sb_run", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(sandbox.SandboxInfo{ID: "sb_run", State: "running"})
	})
	mux.HandleFunc("/v1/sandboxes/sb_run/processes", func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			http.Error(w, "method", 405)
			return
		}
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(201)
		json.NewEncoder(w).Encode(map[string]any{
			"id": "proc_1", "sandbox_id": "sb_run",
			"command":    []string{"/bin/sh", "-c", "echo hi"},
			"state":      "running",
			"exit_code":  0,
			"pid":        42,
			"started_at": 0,
			"exited_at":  0,
		})
	})
	mux.HandleFunc("/v1/sandboxes/sb_run/processes/proc_1", func(w http.ResponseWriter, r *http.Request) {
		callCount++
		state := "running"
		if callCount >= 2 {
			state = "exited"
		}
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(map[string]any{
			"id": "proc_1", "sandbox_id": "sb_run",
			"command":    []string{"/bin/sh", "-c", "echo hi"},
			"state":      state,
			"exit_code":  0,
			"pid":        42,
			"started_at": 0,
			"exited_at":  0,
		})
	})
	mux.HandleFunc("/v1/sandboxes/sb_run/processes/proc_1/logs", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/plain")
		w.Write([]byte("hi\n"))
	})

	c := newTestClient(t, mux)
	sb, err := sandbox.Get(context.Background(), "sb_run", optionsFromClient(c)...)
	if err != nil {
		t.Fatal(err)
	}
	result, err := sb.Run(context.Background(), "echo hi")
	if err != nil {
		t.Fatal(err)
	}
	if result.ExitCode != 0 {
		t.Fatalf("exit code %d", result.ExitCode)
	}
	if result.Combined != "hi\n" {
		t.Fatalf("combined %q", result.Combined)
	}
}

func TestRun_NonZeroExit(t *testing.T) {
	mux := http.NewServeMux()

	mux.HandleFunc("/v1/sandboxes/sb_fail", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(sandbox.SandboxInfo{ID: "sb_fail", State: "running"})
	})
	mux.HandleFunc("/v1/sandboxes/sb_fail/processes", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(201)
		json.NewEncoder(w).Encode(map[string]any{
			"id": "proc_f", "sandbox_id": "sb_fail",
			"command": []string{"/bin/sh", "-c", "exit 1"},
			"state": "running", "exit_code": 0, "pid": 1, "started_at": 0, "exited_at": 0,
		})
	})
	mux.HandleFunc("/v1/sandboxes/sb_fail/processes/proc_f", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(map[string]any{
			"id": "proc_f", "sandbox_id": "sb_fail",
			"command": []string{"/bin/sh", "-c", "exit 1"},
			"state": "exited", "exit_code": 1, "pid": 1, "started_at": 0, "exited_at": 1,
		})
	})
	mux.HandleFunc("/v1/sandboxes/sb_fail/processes/proc_f/logs", func(w http.ResponseWriter, r *http.Request) {
		w.Write([]byte(""))
	})

	c := newTestClient(t, mux)
	sb, _ := sandbox.Get(context.Background(), "sb_fail", optionsFromClient(c)...)
	result, err := sb.Run(context.Background(), "exit 1")
	if err != nil {
		t.Fatal(err)
	}
	if result.ExitCode != 1 {
		t.Fatalf("expected exit_code 1, got %d", result.ExitCode)
	}
}

func TestSpawn(t *testing.T) {
	mux := http.NewServeMux()

	mux.HandleFunc("/v1/sandboxes/sb_spawn", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(sandbox.SandboxInfo{ID: "sb_spawn", State: "running"})
	})
	mux.HandleFunc("/v1/sandboxes/sb_spawn/processes", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(201)
		json.NewEncoder(w).Encode(map[string]any{
			"id": "proc_s1", "sandbox_id": "sb_spawn",
			"command": []string{"npm", "run", "dev"},
			"state": "running", "exit_code": 0, "pid": 99, "started_at": 0, "exited_at": 0,
		})
	})

	c := newTestClient(t, mux)
	sb, _ := sandbox.Get(context.Background(), "sb_spawn", optionsFromClient(c)...)
	proc, err := sb.Spawn(context.Background(), "npm run dev")
	if err != nil {
		t.Fatal(err)
	}
	if proc == nil {
		t.Fatal("proc is nil")
	}
}

func TestSpawn_OnExitCallback(t *testing.T) {
	mux := http.NewServeMux()

	mux.HandleFunc("/v1/sandboxes/sb_wait", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(sandbox.SandboxInfo{ID: "sb_wait", State: "running"})
	})
	mux.HandleFunc("/v1/sandboxes/sb_wait/processes", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(201)
		json.NewEncoder(w).Encode(map[string]any{
			"id": "proc_w1", "sandbox_id": "sb_wait",
			"command": []string{"sleep", "0"},
			"state": "running", "exit_code": 0, "pid": 77, "started_at": 0, "exited_at": 0,
		})
	})
	mux.HandleFunc("/v1/sandboxes/sb_wait/processes/proc_w1", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(map[string]any{
			"id": "proc_w1", "sandbox_id": "sb_wait",
			"command": []string{"sleep", "0"},
			"state": "exited", "exit_code": 0, "pid": 77, "started_at": 0, "exited_at": 1,
		})
	})

	c := newTestClient(t, mux)
	sb, _ := sandbox.Get(context.Background(), "sb_wait", optionsFromClient(c)...)
	proc, err := sb.Spawn(context.Background(), "sleep 0")
	if err != nil {
		t.Fatal(err)
	}

	exitCode := -1
	proc.OnExit(func(code int) { exitCode = code })

	if err := proc.Wait(context.Background()); err != nil {
		t.Fatal(err)
	}
	if exitCode != 0 {
		t.Fatalf("OnExit got code %d, want 0", exitCode)
	}
}
