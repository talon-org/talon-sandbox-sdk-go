package talonsandbox_test

import (
	"context"
	"encoding/json"
	"net/http"
	"sync/atomic"
	"testing"
	"time"

	sandbox "x.xgit.pro/dark/talon-sandbox-sdk-go"
)

// processListHandler returns a list with the named process. State and
// exit_code are read from atomics so tests can flip them mid-poll.
type processListHandler struct {
	procID   string
	command  []string
	pid      int32
	state    atomic.Value // string
	exitCode atomic.Int32
}

func (h *processListHandler) setState(s string)      { h.state.Store(s) }
func (h *processListHandler) setExitCode(c int32)    { h.exitCode.Store(c) }
func (h *processListHandler) currentState() string   { return h.state.Load().(string) }
func (h *processListHandler) currentExitCode() int32 { return h.exitCode.Load() }

func (h *processListHandler) handle(w http.ResponseWriter, _ *http.Request) {
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]any{
		"processes": []map[string]any{
			{
				"id": h.procID, "sandbox_id": "sb_test",
				"command":    h.command,
				"state":      h.currentState(),
				"exit_code":  h.currentExitCode(),
				"pid":        h.pid,
				"started_at": 0,
				"exited_at":  0,
			},
		},
	})
}

func TestRun(t *testing.T) {
	// Speed up the poll loop for tests.
	sandbox.SetPollInterval(5 * time.Millisecond)
	t.Cleanup(func() { sandbox.SetPollInterval(500 * time.Millisecond) })

	h := &processListHandler{procID: "proc_1", command: []string{"/bin/sh", "-c", "echo hi"}, pid: 42}
	h.setState("running")
	h.setExitCode(0)

	callCount := atomic.Int32{}
	mux := http.NewServeMux()
	mux.HandleFunc("/v1/sandboxes/sb_run", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(sandbox.SandboxInfo{ID: "sb_run", State: "running"})
	})
	mux.HandleFunc("/v1/sandboxes/sb_run/processes", func(w http.ResponseWriter, r *http.Request) {
		if r.Method == http.MethodPost {
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
			return
		}
		// GET /processes (list) — exit on 2nd call.
		if callCount.Add(1) >= 2 {
			h.setState("exited")
		}
		h.handle(w, r)
	})
	mux.HandleFunc("/v1/sandboxes/sb_run/processes/proc_1/logs", func(w http.ResponseWriter, _ *http.Request) {
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
	sandbox.SetPollInterval(5 * time.Millisecond)
	t.Cleanup(func() { sandbox.SetPollInterval(500 * time.Millisecond) })

	h := &processListHandler{procID: "proc_f", command: []string{"/bin/sh", "-c", "exit 1"}, pid: 1}
	h.setState("exited")
	h.setExitCode(1)

	mux := http.NewServeMux()
	mux.HandleFunc("/v1/sandboxes/sb_fail", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(sandbox.SandboxInfo{ID: "sb_fail", State: "running"})
	})
	mux.HandleFunc("/v1/sandboxes/sb_fail/processes", func(w http.ResponseWriter, r *http.Request) {
		if r.Method == http.MethodPost {
			w.Header().Set("Content-Type", "application/json")
			w.WriteHeader(201)
			json.NewEncoder(w).Encode(map[string]any{
				"id": "proc_f", "sandbox_id": "sb_fail",
				"command": []string{"/bin/sh", "-c", "exit 1"},
				"state": "running", "exit_code": 0, "pid": 1, "started_at": 0, "exited_at": 0,
			})
			return
		}
		h.handle(w, r)
	})
	mux.HandleFunc("/v1/sandboxes/sb_fail/processes/proc_f/logs", func(w http.ResponseWriter, _ *http.Request) {
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
	if proc.ID() != "proc_s1" {
		t.Fatalf("ID() = %q, want %q", proc.ID(), "proc_s1")
	}
}

func TestSpawn_OnExitCallback(t *testing.T) {
	sandbox.SetPollInterval(5 * time.Millisecond)
	t.Cleanup(func() { sandbox.SetPollInterval(500 * time.Millisecond) })

	h := &processListHandler{procID: "proc_w1", command: []string{"sleep", "0"}, pid: 77}
	h.setState("exited")
	h.setExitCode(0)

	mux := http.NewServeMux()
	mux.HandleFunc("/v1/sandboxes/sb_wait", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(sandbox.SandboxInfo{ID: "sb_wait", State: "running"})
	})
	mux.HandleFunc("/v1/sandboxes/sb_wait/processes", func(w http.ResponseWriter, r *http.Request) {
		if r.Method == http.MethodPost {
			w.Header().Set("Content-Type", "application/json")
			w.WriteHeader(201)
			json.NewEncoder(w).Encode(map[string]any{
				"id": "proc_w1", "sandbox_id": "sb_wait",
				"command": []string{"sleep", "0"},
				"state": "running", "exit_code": 0, "pid": 77, "started_at": 0, "exited_at": 0,
			})
			return
		}
		h.handle(w, r)
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
