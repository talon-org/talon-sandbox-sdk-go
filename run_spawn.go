package talonsandbox

import (
	"context"
	"fmt"
	"io"
	"log/slog"
	"strings"
	"sync"
)

// RunOpts configures a synchronous Run call.
type RunOpts struct {
	// Cwd is the working directory inside the sandbox.
	Cwd string
	// Env are additional environment variables in "KEY=value" form.
	Env []string
}

// Run executes a command synchronously and returns its combined output.
// The command string is passed to /bin/sh -c inside the sandbox.
// Run polls until the process exits.
func (s *Sandbox) Run(ctx context.Context, command string, opts ...RunOpts) (*ProcessResult, error) {
	body := map[string]any{
		"command": []string{"/bin/sh", "-c", command},
	}
	if len(opts) > 0 {
		o := opts[0]
		if o.Cwd != "" {
			body["cwd"] = o.Cwd
		}
		if len(o.Env) > 0 {
			body["env"] = o.Env
		}
	}

	var proc processDTO
	if _, err := s.client.post(ctx, fmt.Sprintf("/v1/sandboxes/%s/processes", s.info.ID), body, &proc); err != nil {
		return nil, fmt.Errorf("run %q: %w", command, err)
	}

	return s.waitProcess(ctx, proc.ID)
}

// waitProcess polls the process until it exits, then fetches combined logs.
func (s *Sandbox) waitProcess(ctx context.Context, procID string) (*ProcessResult, error) {
	for {
		select {
		case <-ctx.Done():
			return nil, ctx.Err()
		default:
		}

		var proc processDTO
		if _, err := s.client.get(ctx, fmt.Sprintf("/v1/sandboxes/%s/processes/%s", s.info.ID, procID), &proc); err != nil {
			return nil, fmt.Errorf("poll process %s: %w", procID, err)
		}
		if proc.State == "exited" || proc.State == "killed" || proc.State == "failed" {
			logs, _ := s.fetchProcessLogs(ctx, procID)
			return &ProcessResult{
				ExitCode: int(proc.ExitCode),
				Combined: string(logs),
			}, nil
		}
	}
}

func (s *Sandbox) fetchProcessLogs(ctx context.Context, procID string) ([]byte, error) {
	path := fmt.Sprintf("/v1/sandboxes/%s/processes/%s/logs", s.info.ID, procID)
	req, err := s.client.newRequest(ctx, "GET", path, nil)
	if err != nil {
		return nil, err
	}
	req.Header.Set("Accept", "text/plain")
	resp, err := s.client.httpClient.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	return io.ReadAll(resp.Body)
}

// ─── Spawn ────────────────────────────────────────────────────────────────────

// SpawnOpts configures an async Spawn call.
type SpawnOpts struct {
	// Cwd is the working directory inside the sandbox.
	Cwd string
	// Env are additional environment variables in "KEY=value" form.
	Env []string
}

// Process is a handle to a long-running spawned process.
type Process struct {
	id        string
	sandboxID string
	client    *Client

	mu        sync.Mutex
	stdoutFns []func([]byte)
	exitFns   []func(int)
	stopCh    chan struct{}
	stopped   bool
}

// OnStdout registers a callback for each stdout/stderr chunk.
// Note: the current implementation does not stream logs in real-time;
// callbacks are informational. Use Wait + Run for result capture.
func (p *Process) OnStdout(fn func([]byte)) {
	p.mu.Lock()
	defer p.mu.Unlock()
	p.stdoutFns = append(p.stdoutFns, fn)
}

// OnExit registers a callback called when the process exits.
func (p *Process) OnExit(fn func(exitCode int)) {
	p.mu.Lock()
	defer p.mu.Unlock()
	p.exitFns = append(p.exitFns, fn)
}

// Kill sends a kill signal to the process.
func (p *Process) Kill(ctx context.Context) error {
	_, err := p.client.delete(ctx, fmt.Sprintf("/v1/sandboxes/%s/processes/%s", p.sandboxID, p.id))
	return err
}

// Wait blocks until the process exits (polls the API).
func (p *Process) Wait(ctx context.Context) error {
	for {
		select {
		case <-ctx.Done():
			return ctx.Err()
		default:
		}

		p.mu.Lock()
		if p.stopped {
			p.mu.Unlock()
			return nil
		}
		p.mu.Unlock()

		var proc processDTO
		if _, err := p.client.get(ctx, fmt.Sprintf("/v1/sandboxes/%s/processes/%s", p.sandboxID, p.id), &proc); err != nil {
			return fmt.Errorf("wait process %s: %w", p.id, err)
		}
		if proc.State == "exited" || proc.State == "killed" || proc.State == "failed" {
			p.mu.Lock()
			fns := append([]func(int){}, p.exitFns...)
			p.stopped = true
			p.mu.Unlock()
			for _, fn := range fns {
				safeCallExit(fn, int(proc.ExitCode))
			}
			return nil
		}
	}
}

// Spawn starts a long-running process and returns a handle immediately.
// The command string is split on whitespace; for complex shell commands use
// sb.Run("...") or pass SpawnOpts with a shell invocation.
func (s *Sandbox) Spawn(ctx context.Context, command string, opts ...SpawnOpts) (*Process, error) {
	body := map[string]any{
		"command": strings.Fields(command),
	}
	if len(opts) > 0 {
		o := opts[0]
		if o.Cwd != "" {
			body["cwd"] = o.Cwd
		}
		if len(o.Env) > 0 {
			body["env"] = o.Env
		}
	}

	var proc processDTO
	if _, err := s.client.post(ctx, fmt.Sprintf("/v1/sandboxes/%s/processes", s.info.ID), body, &proc); err != nil {
		return nil, fmt.Errorf("spawn %q: %w", command, err)
	}

	p := &Process{
		id:        proc.ID,
		sandboxID: s.info.ID,
		client:    s.client,
		stopCh:    make(chan struct{}),
	}
	return p, nil
}

func safeCallExit(fn func(int), code int) {
	defer func() {
		if r := recover(); r != nil {
			slog.Error("Process OnExit callback panicked", "panic", r)
		}
	}()
	fn(code)
}
