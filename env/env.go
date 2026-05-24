// Package env provides environment variable operations inside a Talon Sandbox.
package env

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
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
// Calls GET /v1/sandboxes/{id}/env/{key}.
func (e *Env) Get(ctx context.Context, key string) (string, error) {
	u := fmt.Sprintf("%s/v1/sandboxes/%s/env/%s", e.baseURL, e.sandboxID, key)
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

// Set sets an environment variable in the sandbox.
// Calls POST /v1/sandboxes/{id}/env.
func (e *Env) Set(ctx context.Context, key, value string) error {
	payload, _ := json.Marshal(map[string]string{"key": key, "value": value})
	u := fmt.Sprintf("%s/v1/sandboxes/%s/env", e.baseURL, e.sandboxID)
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, u, bytes.NewReader(payload))
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

func (e *Env) setAuth(req *http.Request) {
	if e.authHeader != "" {
		req.Header.Set("Authorization", e.authHeader)
	}
}
