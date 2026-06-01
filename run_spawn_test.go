package talonsandbox_test

import (
	"context"
	"encoding/json"
	"io"
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

// TestSpawn_ExposePorts 验证 SpawnOpts.ExposePorts 非空时请求体包含 expose_ports 字段，
// 且其值与传入的端口列表一致；nil/空时请求体不含该键（向后兼容）。
func TestSpawn_ExposePorts(t *testing.T) {
	var capturedBody map[string]any

	mux := http.NewServeMux()
	mux.HandleFunc("/v1/sandboxes/sb_ep", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(sandbox.SandboxInfo{ID: "sb_ep", State: "running"})
	})
	mux.HandleFunc("/v1/sandboxes/sb_ep/processes", func(w http.ResponseWriter, r *http.Request) {
		// 解析并保存请求体，供断言使用。
		raw, _ := io.ReadAll(r.Body)
		json.Unmarshal(raw, &capturedBody)

		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(201)
		json.NewEncoder(w).Encode(map[string]any{
			"id": "proc_ep1", "sandbox_id": "sb_ep",
			"command": []string{"npm", "run", "dev"},
			"state": "running", "exit_code": 0, "pid": 10, "started_at": 0, "exited_at": 0,
		})
	})

	c := newTestClient(t, mux)
	sb, _ := sandbox.Get(context.Background(), "sb_ep", optionsFromClient(c)...)

	// 场景一：ExposePorts 非空，body 应含 expose_ports。
	capturedBody = nil
	_, err := sb.Spawn(context.Background(), "npm run dev", sandbox.SpawnOpts{
		ExposePorts: []int32{5173, 8080},
	})
	if err != nil {
		t.Fatal(err)
	}
	ports, ok := capturedBody["expose_ports"]
	if !ok {
		t.Fatal("expose_ports 未写入请求体")
	}
	// JSON 反序列化数字为 float64，转回比较。
	portsSlice, ok := ports.([]any)
	if !ok || len(portsSlice) != 2 {
		t.Fatalf("expose_ports 期望长度 2，实际 %v", ports)
	}
	if int(portsSlice[0].(float64)) != 5173 || int(portsSlice[1].(float64)) != 8080 {
		t.Fatalf("expose_ports 值不符：%v", portsSlice)
	}

	// 场景二：ExposePorts 为 nil，body 不应含 expose_ports（向后兼容）。
	capturedBody = nil
	_, err = sb.Spawn(context.Background(), "npm run dev", sandbox.SpawnOpts{})
	if err != nil {
		t.Fatal(err)
	}
	if _, exists := capturedBody["expose_ports"]; exists {
		t.Fatal("ExposePorts 为空时不应写入 expose_ports 键")
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
