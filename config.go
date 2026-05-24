package talonsandbox

import (
	"os"
	"sync"
)

const (
	envServer = "TALON_SANDBOX_SERVER"
	envAPIKey = "TALON_SANDBOX_API_KEY"

	defaultServer = "http://localhost:18080"
)

var (
	globalMu     sync.RWMutex
	globalClient *Client
)

// Configure sets the package-level default server and API key used when no
// explicit client options are passed to Create/Get/List.
//
// Call once at program start, or rely on TALON_SANDBOX_SERVER /
// TALON_SANDBOX_API_KEY environment variables instead.
func Configure(server, apiKey string) {
	globalMu.Lock()
	defer globalMu.Unlock()
	globalClient = New(server, WithAPIKey(apiKey))
}

// defaultClient returns the global client, lazily initialising it from env.
func defaultClient() *Client {
	globalMu.RLock()
	c := globalClient
	globalMu.RUnlock()
	if c != nil {
		return c
	}

	globalMu.Lock()
	defer globalMu.Unlock()
	if globalClient == nil {
		server := os.Getenv(envServer)
		if server == "" {
			server = defaultServer
		}
		apiKey := os.Getenv(envAPIKey)
		globalClient = New(server, WithAPIKey(apiKey))
	}
	return globalClient
}
