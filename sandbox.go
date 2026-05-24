package talonsandbox

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"
	"time"

	"x.xgit.pro/dark/talon-sandbox-sdk-go/browser"
	tsenv "x.xgit.pro/dark/talon-sandbox-sdk-go/env"
	"x.xgit.pro/dark/talon-sandbox-sdk-go/fs"
	"x.xgit.pro/dark/talon-sandbox-sdk-go/terminal"
)

// networkAliases maps v2 friendly names to API canonical values.
var networkAliases = map[string]string{
	"allowlist": "restricted-egress",
	"open":      "full-egress",
	"sealed":    "offline",
	"deny":      "offline",
}

func normalizeNetwork(s string) string {
	if v, ok := networkAliases[s]; ok {
		return v
	}
	return s
}

// Sandbox is a live handle to a sandbox environment.
// All methods make API calls. Safe for concurrent use.
type Sandbox struct {
	info   SandboxInfo
	client *Client
}

// ID returns the sandbox identifier.
func (s *Sandbox) ID() string { return s.info.ID }

// State returns the last known state.
func (s *Sandbox) State() string { return s.info.State }

// Info returns the cached SandboxInfo snapshot.
func (s *Sandbox) Info() SandboxInfo { return s.info }

// FS returns a filesystem handle for this sandbox.
func (s *Sandbox) FS() *fs.FS {
	return fs.New(s.info.ID, s.client.baseURL, s.client.authHeader(), s.client.httpClient)
}

// Env returns an environment-variable handle for this sandbox.
func (s *Sandbox) Env() *tsenv.Env {
	return tsenv.New(s.info.ID, s.client.baseURL, s.client.authHeader(), s.client.httpClient)
}

// Browser returns a browser-session handle for this sandbox.
func (s *Sandbox) Browser() *browser.Browser {
	return browser.New(s.info.ID, s.client.baseURL, s.client.authHeader(), s.client.httpClient)
}

// Terminal returns a terminal handle for PTY sessions.
func (s *Sandbox) Terminal() *terminal.Terminal {
	return terminal.New(
		s.info.ID,
		s.client.wsURL(""),
		s.client.authHeader(),
		s.client.authCookies(),
	)
}

// Pause freezes all processes inside the sandbox.
func (s *Sandbox) Pause(ctx context.Context) error {
	_, err := s.client.post(ctx, "/v1/sandboxes/"+s.info.ID+"/pause", nil, nil)
	if err != nil {
		return fmt.Errorf("pause sandbox %s: %w", s.info.ID, err)
	}
	return nil
}

// Resume resumes a paused sandbox.
func (s *Sandbox) Resume(ctx context.Context) error {
	_, err := s.client.post(ctx, "/v1/sandboxes/"+s.info.ID+"/resume", nil, nil)
	if err != nil {
		return fmt.Errorf("resume sandbox %s: %w", s.info.ID, err)
	}
	return nil
}

// Kill permanently destroys the sandbox.
func (s *Sandbox) Kill(ctx context.Context) error {
	if _, err := s.client.delete(ctx, "/v1/sandboxes/"+s.info.ID); err != nil {
		return fmt.Errorf("kill sandbox %s: %w", s.info.ID, err)
	}
	return nil
}

// Refresh fetches the latest state from the API.
func (s *Sandbox) Refresh(ctx context.Context) (SandboxInfo, error) {
	var info SandboxInfo
	if _, err := s.client.get(ctx, "/v1/sandboxes/"+s.info.ID, &info); err != nil {
		return SandboxInfo{}, fmt.Errorf("refresh sandbox %s: %w", s.info.ID, err)
	}
	s.info = info
	return info, nil
}

// ─── Package-level constructors ───────────────────────────────────────────────

// Create creates a new sandbox. Uses the global default client unless
// explicit options are provided.
//
// Example:
//
//	sb, err := sandbox.Create(ctx, sandbox.Opts{
//	    Image:     "node:20-bookworm",
//	    Resources: sandbox.Resources{CPU: 2, Memory: "4GiB"},
//	    Network:   "allowlist",
//	    Timeout:   "30m",
//	    TTL:       "6h",
//	})
func Create(ctx context.Context, opts Opts, clientOpts ...Option) (*Sandbox, error) {
	c := resolveClient(clientOpts)
	body, err := buildCreateBody(opts)
	if err != nil {
		return nil, err
	}

	var info SandboxInfo
	if _, err := c.post(ctx, "/v1/sandboxes?wait=running", body, &info); err != nil {
		return nil, fmt.Errorf("create sandbox: %w", err)
	}
	return &Sandbox{info: info, client: c}, nil
}

// Get fetches an existing sandbox by ID.
func Get(ctx context.Context, id string, clientOpts ...Option) (*Sandbox, error) {
	c := resolveClient(clientOpts)
	var info SandboxInfo
	if _, err := c.get(ctx, "/v1/sandboxes/"+id, &info); err != nil {
		return nil, fmt.Errorf("get sandbox %s: %w", id, err)
	}
	return &Sandbox{info: info, client: c}, nil
}

// List returns sandboxes for the current tenant, optionally filtered by labels.
func List(ctx context.Context, opts ListOpts, clientOpts ...Option) ([]*Sandbox, error) {
	c := resolveClient(clientOpts)
	var out struct {
		Sandboxes []SandboxInfo `json:"sandboxes"`
	}
	if _, err := c.get(ctx, "/v1/sandboxes", &out); err != nil {
		return nil, fmt.Errorf("list sandboxes: %w", err)
	}

	var result []*Sandbox
	for _, info := range out.Sandboxes {
		info := info // capture
		if matchesLabels(info.Labels, opts.Labels) {
			result = append(result, &Sandbox{info: info, client: c})
		}
	}
	return result, nil
}

// ─── helpers ──────────────────────────────────────────────────────────────────

// resolveClient returns the explicit client (from opts) or the global default.
func resolveClient(opts []Option) *Client {
	if len(opts) == 0 {
		return defaultClient()
	}
	// Build a new client with the global defaults as base, then apply opts.
	c := New("")
	for _, o := range opts {
		o(c)
	}
	return c
}

func matchesLabels(have, want map[string]string) bool {
	for k, v := range want {
		if have[k] != v {
			return false
		}
	}
	return true
}

func buildCreateBody(opts Opts) (map[string]any, error) {
	body := map[string]any{}

	if opts.Image != "" {
		body["image_id"] = opts.Image
	}
	if n := normalizeNetwork(opts.Network); n != "" {
		body["network_policy"] = n
	}
	if len(opts.Env) > 0 {
		body["env"] = opts.Env
	}
	if len(opts.Labels) > 0 {
		body["labels"] = opts.Labels
	}

	r := opts.Resources
	if r.CPU != 0 {
		body["cpu_millis"] = int64(r.CPU * 1000)
	}
	if r.Memory != "" {
		mb, err := ParseSize(r.Memory)
		if err != nil {
			return nil, fmt.Errorf("invalid resources.Memory %q: %w", r.Memory, err)
		}
		body["memory_bytes"] = mb
	}
	if r.Disk != "" {
		db, err := ParseSize(r.Disk)
		if err != nil {
			return nil, fmt.Errorf("invalid resources.Disk %q: %w", r.Disk, err)
		}
		body["disk_bytes"] = db
	}

	if opts.Timeout != "" {
		td, err := ParseDuration(opts.Timeout)
		if err != nil {
			return nil, fmt.Errorf("invalid timeout %q: %w", opts.Timeout, err)
		}
		body["idle_timeout_seconds"] = int64(td / time.Second)
	}
	if opts.TTL != "" {
		td, err := ParseDuration(opts.TTL)
		if err != nil {
			return nil, fmt.Errorf("invalid ttl %q: %w", opts.TTL, err)
		}
		body["ttl_seconds"] = int64(td / time.Second)
	}

	return body, nil
}

// extractErrMsg parses {"error":"..."} from raw bytes.
func extractErrMsg(body []byte) string {
	var e struct {
		Error string `json:"error"`
	}
	if err := json.Unmarshal(body, &e); err == nil && e.Error != "" {
		return e.Error
	}
	s := string(body)
	if len(s) > 256 {
		s = s[:256]
	}
	return strings.TrimSpace(s)
}
