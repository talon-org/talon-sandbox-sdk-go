package talonsandbox

import (
	"context"
	"fmt"
	"io"
	"log/slog"
	"strings"
	"sync"
	"time"
)

// pollInterval is the wait between process state polls. Default 500ms is
// the same cadence as the Python and .NET SDKs; the server has no
// GET /processes/{id} endpoint, so each tick costs one list call (+ one log
// fetch on Run).
var pollInterval = 500 * time.Millisecond

// SetPollInterval overrides the process state poll cadence (for tests).
func SetPollInterval(d time.Duration) { pollInterval = d }

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

// waitProcess polls the LIST endpoint until the target process exits, then
// fetches its combined logs. Uses LIST + filter because the server has no
// single-process GET endpoint. Sleeps pollInterval between ticks to avoid
// hammering the server (every poll is one list call).
func (s *Sandbox) waitProcess(ctx context.Context, procID string) (*ProcessResult, error) {
	for {
		select {
		case <-ctx.Done():
			return nil, ctx.Err()
		default:
		}

		var list processListDTO
		if _, err := s.client.get(ctx, fmt.Sprintf("/v1/sandboxes/%s/processes", s.info.ID), &list); err != nil {
			return nil, fmt.Errorf("poll process %s: %w", procID, err)
		}
		var found *processDTO
		for i := range list.Processes {
			if list.Processes[i].ID == procID {
				found = &list.Processes[i]
				break
			}
		}
		if found == nil {
			// Disappeared between spawn and poll — fetch whatever logs
			// survived and return -1 to signal "unknown exit".
			logs, _ := s.fetchProcessLogs(ctx, procID)
			return &ProcessResult{ExitCode: -1, Combined: string(logs)}, nil
		}
		if found.State == "exited" || found.State == "killed" || found.State == "failed" {
			logs, logErr := s.fetchProcessLogs(ctx, procID)
			result := &ProcessResult{
				ExitCode: int(found.ExitCode),
				Combined: string(logs),
			}
			// Don't lose the log fetch error — empty Combined with a
			// nil error would let the caller misread "command printed
			// nothing" vs "we failed to fetch the output".
			if logErr != nil && len(logs) == 0 {
				return result, fmt.Errorf("fetch logs: %w", logErr)
			}
			return result, nil
		}

		if err := sleepCtx(ctx, pollInterval); err != nil {
			return nil, err
		}
	}
}

// sleepCtx sleeps for d or until ctx is cancelled, whichever first.
func sleepCtx(ctx context.Context, d time.Duration) error {
	timer := time.NewTimer(d)
	defer timer.Stop()
	select {
	case <-ctx.Done():
		return ctx.Err()
	case <-timer.C:
		return nil
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

// ID returns the platform process id (e.g. "proc_abc123") so callers can
// reference this process from custom HTTP calls or debugging output.
func (p *Process) ID() string { return p.id }

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

// Wait blocks until the process exits. Polls the LIST endpoint at pollInterval
// cadence; server has no single-process GET so each tick costs one list call.
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

		var list processListDTO
		if _, err := p.client.get(ctx, fmt.Sprintf("/v1/sandboxes/%s/processes", p.sandboxID), &list); err != nil {
			return fmt.Errorf("wait process %s: %w", p.id, err)
		}
		var found *processDTO
		for i := range list.Processes {
			if list.Processes[i].ID == p.id {
				found = &list.Processes[i]
				break
			}
		}
		if found == nil {
			// Process gone from list — treat as exit with unknown code.
			p.markExited(-1)
			return nil
		}
		if found.State == "exited" || found.State == "killed" || found.State == "failed" {
			p.markExited(int(found.ExitCode))
			return nil
		}

		if err := sleepCtx(ctx, pollInterval); err != nil {
			return err
		}
	}
}

// markExited fires exit callbacks once and marks the process stopped.
// Safe to call multiple times (no-op after first).
func (p *Process) markExited(code int) {
	p.mu.Lock()
	if p.stopped {
		p.mu.Unlock()
		return
	}
	p.stopped = true
	fns := append([]func(int){}, p.exitFns...)
	p.mu.Unlock()
	for _, fn := range fns {
		safeCallExit(fn, code)
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
