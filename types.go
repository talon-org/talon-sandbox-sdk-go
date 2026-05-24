package talonsandbox

import "time"

// Opts configures a new sandbox.
type Opts struct {
	// Image is the container image reference, e.g. "node:20-bookworm".
	Image string
	// Resources describes compute allocation. Strings accepted: "4GiB", "10GiB".
	Resources Resources
	// Network policy: "allowlist" | "open" | "sealed". Defaults to server default.
	Network string
	// Env is startup environment variables.
	Env map[string]string
	// Timeout is the idle-timeout duration string, e.g. "30m". Empty = disabled.
	Timeout string
	// TTL is the hard time-to-live string, e.g. "6h". Empty = disabled.
	TTL string
	// Labels are arbitrary key-value metadata.
	Labels map[string]string
}

// Resources describes compute resources using human-readable strings.
type Resources struct {
	// CPU cores (integer or float). 2 = 2 cores, 0.5 = half a core.
	CPU float64
	// Memory string: "4GiB", "512MiB", etc.
	Memory string
	// Disk string: "10GiB", etc.
	Disk string
}

// ListOpts filters sandbox listing.
type ListOpts struct {
	// Labels filters sandboxes to those matching all specified labels.
	Labels map[string]string
}

// ExposeOpts configures optional port exposure settings.
type ExposeOpts struct {
	// Sign requests a signed preview URL (Spec 48).
	Sign bool
	// TTL is the signed URL lifetime, e.g. "1h". Only meaningful when Sign=true.
	TTL string
	// Subdomain is a custom subdomain prefix (default: random).
	Subdomain string
}

// SandboxInfo is the read model returned by the API for a sandbox.
type SandboxInfo struct {
	ID                 string            `json:"id"`
	State              string            `json:"state"`
	Image              string            `json:"image_id,omitempty"`
	CPUMillis          int64             `json:"cpu_millis,omitempty"`
	MemoryBytes        int64             `json:"memory_bytes,omitempty"`
	IdleTimeoutSeconds int64             `json:"idle_timeout_seconds,omitempty"`
	TTLSeconds         int64             `json:"ttl_seconds,omitempty"`
	CreatedAt          int64             `json:"created_at,omitempty"`
	NetworkPolicy      string            `json:"network_policy,omitempty"`
	Labels             map[string]string `json:"labels,omitempty"`
}

// ProcessResult is the outcome of a synchronous Run call.
type ProcessResult struct {
	ExitCode int
	// Combined is stdout+stderr as returned by the process log endpoint.
	Combined string
}

// processDTO is the wire format returned by POST /v1/sandboxes/{id}/processes.
type processDTO struct {
	ID        string   `json:"id"`
	SandboxID string   `json:"sandbox_id"`
	Command   []string `json:"command"`
	PID       int32    `json:"pid"`
	State     string   `json:"state"`
	ExitCode  int32    `json:"exit_code"`
	StartedAt int64    `json:"started_at"`
	ExitedAt  int64    `json:"exited_at"`
}

// processListDTO is the wire format returned by GET /v1/sandboxes/{id}/processes.
type processListDTO struct {
	Processes []processDTO `json:"processes"`
}

// ExposedPort describes a currently exposed port.
type ExposedPort struct {
	Port      int       `json:"port"`
	URL       string    `json:"url"`
	Signed    bool      `json:"signed"`
	ExpiresAt time.Time `json:"expires_at,omitempty"`
	Source    string    `json:"source,omitempty"` // "explicit" | "dynamic"
}

// FsEntry is a single filesystem entry (file or directory).
type FsEntry struct {
	Name    string `json:"name"`
	Size    int64  `json:"size"`
	ModTime int64  `json:"mod_time"`
	IsDir   bool   `json:"is_dir"`
}
