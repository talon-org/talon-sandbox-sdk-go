// Package browser provides browser session management inside a Talon Sandbox.
package browser

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
)

// Browser manages a headless Chromium session inside a sandbox.
type Browser struct {
	sandboxID  string
	baseURL    string
	authHeader string
	httpClient *http.Client
}

// New creates a Browser handle. Called by Sandbox.Browser().
func New(sandboxID, baseURL, authHeader string, httpClient *http.Client) *Browser {
	return &Browser{
		sandboxID:  sandboxID,
		baseURL:    strings.TrimRight(baseURL, "/"),
		authHeader: authHeader,
		httpClient: httpClient,
	}
}

// BrowserSession describes a running headless browser.
type BrowserSession struct {
	SandboxID string `json:"sandbox_id"`
	ProcessID string `json:"process_id"`
	CDPPort   int32  `json:"cdp_port"`
	CDPPath   string `json:"cdp_path"`
	// CDPURL is the WebSocket URL for the Chrome DevTools Protocol.
	CDPURL string `json:"cdp_ws_url"`
}

// Start launches a headless Chromium inside the sandbox.
// POST /v1/sandboxes/{id}/browser，成功返回 BrowserSession（含 cdp_ws_url）。
func (b *Browser) Start(ctx context.Context) (*BrowserSession, error) {
	u := fmt.Sprintf("%s/v1/sandboxes/%s/browser", b.baseURL, b.sandboxID)
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, u, nil)
	if err != nil {
		return nil, err
	}
	if b.authHeader != "" {
		req.Header.Set("Authorization", b.authHeader)
	}
	req.Header.Set("Accept", "application/json")

	resp, err := b.httpClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("browser start: %w", err)
	}
	defer resp.Body.Close()
	body, _ := io.ReadAll(resp.Body)

	if resp.StatusCode >= 400 {
		var e struct{ Error string `json:"error"` }
		json.Unmarshal(body, &e)
		return nil, fmt.Errorf("browser start: HTTP %d: %s", resp.StatusCode, e.Error)
	}

	var sess BrowserSession
	if err := json.Unmarshal(body, &sess); err != nil {
		return nil, fmt.Errorf("browser start decode: %w", err)
	}
	return &sess, nil
}

// Get 查询当前 sandbox 的浏览器 session 状态。
// GET /v1/sandboxes/{id}/browser。浏览器未启动时返回 404 错误。
func (b *Browser) Get(ctx context.Context) (*BrowserSession, error) {
	u := fmt.Sprintf("%s/v1/sandboxes/%s/browser", b.baseURL, b.sandboxID)
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, u, nil)
	if err != nil {
		return nil, err
	}
	if b.authHeader != "" {
		req.Header.Set("Authorization", b.authHeader)
	}
	req.Header.Set("Accept", "application/json")

	resp, err := b.httpClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("browser get: %w", err)
	}
	defer resp.Body.Close()
	body, _ := io.ReadAll(resp.Body)

	if resp.StatusCode >= 400 {
		var e struct{ Error string `json:"error"` }
		json.Unmarshal(body, &e)
		return nil, fmt.Errorf("browser get: HTTP %d: %s", resp.StatusCode, e.Error)
	}

	var sess BrowserSession
	if err := json.Unmarshal(body, &sess); err != nil {
		return nil, fmt.Errorf("browser get decode: %w", err)
	}
	return &sess, nil
}

// Stop 停止并销毁 sandbox 内的浏览器 session。
// DELETE /v1/sandboxes/{id}/browser，成功返回 204 无 body。
func (b *Browser) Stop(ctx context.Context) error {
	u := fmt.Sprintf("%s/v1/sandboxes/%s/browser", b.baseURL, b.sandboxID)
	req, err := http.NewRequestWithContext(ctx, http.MethodDelete, u, nil)
	if err != nil {
		return err
	}
	if b.authHeader != "" {
		req.Header.Set("Authorization", b.authHeader)
	}

	resp, err := b.httpClient.Do(req)
	if err != nil {
		return fmt.Errorf("browser stop: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode >= 400 {
		body, _ := io.ReadAll(resp.Body)
		var e struct{ Error string `json:"error"` }
		json.Unmarshal(body, &e)
		return fmt.Errorf("browser stop: HTTP %d: %s", resp.StatusCode, e.Error)
	}
	return nil
}
