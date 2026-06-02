// Package env provides environment variable operations inside a Talon Sandbox.
package env

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"
)

// Env provides environment variable operations inside a sandbox.
type Env struct {
	sandboxID  string
	baseURL    string
	authHeader string
	httpClient *http.Client
}

// New creates an Env handle. Called by Sandbox.Env().
func New(sandboxID, baseURL, authHeader string, httpClient *http.Client) *Env {
	return &Env{
		sandboxID:  sandboxID,
		baseURL:    strings.TrimRight(baseURL, "/"),
		authHeader: authHeader,
		httpClient: httpClient,
	}
}

// Get returns the value of an environment variable from the sandbox.
// Returns an empty string if the key does not exist.
// Calls GET /v1/sandboxes/{id}/env/{key}.
func (e *Env) Get(ctx context.Context, key string) (string, error) {
	u := fmt.Sprintf("%s/v1/sandboxes/%s/env/%s", e.baseURL, e.sandboxID, url.PathEscape(key))
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, u, nil)
	if err != nil {
		return "", err
	}
	e.setAuth(req)
	req.Header.Set("Accept", "application/json")

	resp, err := e.httpClient.Do(req)
	if err != nil {
		return "", fmt.Errorf("env get %q: %w", key, err)
	}
	defer resp.Body.Close()
	body, _ := io.ReadAll(resp.Body)

	if resp.StatusCode >= 400 {
		return "", fmt.Errorf("env get %q: HTTP %d", key, resp.StatusCode)
	}
	var out struct {
		Value string `json:"value"`
	}
	json.Unmarshal(body, &out) //nolint:errcheck — empty response → empty string is fine
	return out.Value, nil
}

// All returns all environment variables for the sandbox as a map.
// Calls GET /v1/sandboxes/{id}/env.
func (e *Env) All(ctx context.Context) (map[string]string, error) {
	u := fmt.Sprintf("%s/v1/sandboxes/%s/env", e.baseURL, e.sandboxID)
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, u, nil)
	if err != nil {
		return nil, err
	}
	e.setAuth(req)
	req.Header.Set("Accept", "application/json")

	resp, err := e.httpClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("env all: %w", err)
	}
	defer resp.Body.Close()
	body, _ := io.ReadAll(resp.Body)

	if resp.StatusCode >= 400 {
		return nil, fmt.Errorf("env all: HTTP %d", resp.StatusCode)
	}
	var out struct {
		Env map[string]string `json:"env"`
	}
	if err := json.Unmarshal(body, &out); err != nil {
		return nil, fmt.Errorf("env all: decode: %w", err)
	}
	if out.Env == nil {
		out.Env = map[string]string{}
	}
	return out.Env, nil
}

// Set sets an environment variable in the sandbox.
// This updates the persisted value; already-running processes are not restarted
// and will not see the new value. The next process started via Start or spawn
// will inherit the updated environment.
// Calls PUT /v1/sandboxes/{id}/env/{key} with body {"value": "..."}.
func (e *Env) Set(ctx context.Context, key, value string) error {
	payload, _ := json.Marshal(map[string]string{"value": value})
	u := fmt.Sprintf("%s/v1/sandboxes/%s/env/%s", e.baseURL, e.sandboxID, url.PathEscape(key))
	req, err := http.NewRequestWithContext(ctx, http.MethodPut, u, bytes.NewReader(payload))
	if err != nil {
		return err
	}
	e.setAuth(req)
	req.Header.Set("Content-Type", "application/json")

	resp, err := e.httpClient.Do(req)
	if err != nil {
		return fmt.Errorf("env set %q: %w", key, err)
	}
	defer resp.Body.Close()
	if resp.StatusCode >= 400 {
		return fmt.Errorf("env set %q: HTTP %d", key, resp.StatusCode)
	}
	return nil
}

// Unset removes an environment variable from the sandbox.
// Already-running processes are not affected; the variable will be absent
// in the next process started via Start or spawn.
// Calls DELETE /v1/sandboxes/{id}/env/{key}.
func (e *Env) Unset(ctx context.Context, key string) error {
	u := fmt.Sprintf("%s/v1/sandboxes/%s/env/%s", e.baseURL, e.sandboxID, url.PathEscape(key))
	req, err := http.NewRequestWithContext(ctx, http.MethodDelete, u, nil)
	if err != nil {
		return err
	}
	e.setAuth(req)

	resp, err := e.httpClient.Do(req)
	if err != nil {
		return fmt.Errorf("env unset %q: %w", key, err)
	}
	defer resp.Body.Close()
	if resp.StatusCode >= 400 {
		return fmt.Errorf("env unset %q: HTTP %d", key, resp.StatusCode)
	}
	return nil
}

func (e *Env) setAuth(req *http.Request) {
	if e.authHeader != "" {
		req.Header.Set("Authorization", e.authHeader)
	}
}
