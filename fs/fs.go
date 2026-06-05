// Package fs provides filesystem operations inside a Talon Sandbox.
package fs

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"

	"x.xgit.pro/dark/talon-sandbox-sdk-go/internal/httpx"
)

// FS provides filesystem operations inside a sandbox.
type FS struct {
	sandboxID  string
	baseURL    string
	authHeader string
	httpClient *http.Client
}

// New creates an FS handle. Called by Sandbox.FS().
func New(sandboxID, baseURL, authHeader string, httpClient *http.Client) *FS {
	return &FS{
		sandboxID:  sandboxID,
		baseURL:    strings.TrimRight(baseURL, "/"),
		authHeader: authHeader,
		httpClient: httpClient,
	}
}

// Read returns the contents of a file.
func (f *FS) Read(ctx context.Context, path string) ([]byte, error) {
	u := f.baseURL + "/v1/sandboxes/" + f.sandboxID + "/fs/" + cleanPath(path)
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, u, nil)
	if err != nil {
		return nil, err
	}
	f.setAuth(req)
	req.Header.Set("Accept", "application/octet-stream")

	resp, err := f.httpClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("fs read: %w", err)
	}
	defer resp.Body.Close()
	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, err
	}
	if resp.StatusCode >= 400 {
		return nil, fmt.Errorf("fs read: HTTP %d: %s", resp.StatusCode, extractMsg(body))
	}
	return body, nil
}

// Write writes data to a file path, creating parent directories as needed.
func (f *FS) Write(ctx context.Context, path string, data []byte) error {
	u := f.baseURL + "/v1/sandboxes/" + f.sandboxID + "/fs/" + cleanPath(path)
	req, err := http.NewRequestWithContext(ctx, http.MethodPut, u, bytes.NewReader(data))
	if err != nil {
		return err
	}
	f.setAuth(req)
	req.Header.Set("Content-Type", "application/octet-stream")

	resp, err := f.httpClient.Do(req)
	if err != nil {
		return fmt.Errorf("fs write: %w", err)
	}
	defer resp.Body.Close()
	body, _ := io.ReadAll(resp.Body)
	if resp.StatusCode >= 400 {
		return fmt.Errorf("fs write: HTTP %d: %s", resp.StatusCode, extractMsg(body))
	}
	return nil
}

// FsEntry is a single directory entry.
type FsEntry struct {
	Name    string `json:"name"`
	Size    int64  `json:"size"`
	ModTime int64  `json:"mod_time"`
	IsDir   bool   `json:"is_dir"`
}

// List returns the entries in a directory.
func (f *FS) List(ctx context.Context, path string) ([]FsEntry, error) {
	u := f.baseURL + "/v1/sandboxes/" + f.sandboxID + "/fs-list/" + cleanPath(path)
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, u, nil)
	if err != nil {
		return nil, err
	}
	f.setAuth(req)
	req.Header.Set("Accept", "application/json")

	resp, err := f.httpClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("fs list: %w", err)
	}
	defer resp.Body.Close()
	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, err
	}
	if resp.StatusCode >= 400 {
		return nil, fmt.Errorf("fs list: HTTP %d: %s", resp.StatusCode, extractMsg(body))
	}
	var out struct {
		Entries []FsEntry `json:"entries"`
	}
	if err := json.Unmarshal(body, &out); err != nil {
		return nil, fmt.Errorf("fs list decode: %w", err)
	}
	return out.Entries, nil
}

// Remove deletes a file or directory.
func (f *FS) Remove(ctx context.Context, path string) error {
	u := f.baseURL + "/v1/sandboxes/" + f.sandboxID + "/fs/" + cleanPath(path)
	req, err := http.NewRequestWithContext(ctx, http.MethodDelete, u, nil)
	if err != nil {
		return err
	}
	f.setAuth(req)

	resp, err := f.httpClient.Do(req)
	if err != nil {
		return fmt.Errorf("fs remove: %w", err)
	}
	defer resp.Body.Close()
	body, _ := io.ReadAll(resp.Body)
	if resp.StatusCode >= 400 {
		return fmt.Errorf("fs remove: HTTP %d: %s", resp.StatusCode, extractMsg(body))
	}
	return nil
}

func (f *FS) setAuth(req *http.Request) {
	if f.authHeader != "" {
		req.Header.Set("Authorization", f.authHeader)
	}
	// 规范 User-Agent,与根客户端一致,保证来源归因为 sdk-go。
	req.Header.Set("User-Agent", httpx.UserAgent())
}

// cleanPath strips leading slash and URL-encodes each path segment.
func cleanPath(p string) string {
	p = strings.TrimPrefix(p, "/")
	parts := strings.Split(p, "/")
	for i, part := range parts {
		parts[i] = url.PathEscape(part)
	}
	return strings.Join(parts, "/")
}

func extractMsg(body []byte) string {
	var e struct{ Error string `json:"error"` }
	if json.Unmarshal(body, &e) == nil && e.Error != "" {
		return e.Error
	}
	s := string(body)
	if len(s) > 128 {
		s = s[:128]
	}
	return s
}
