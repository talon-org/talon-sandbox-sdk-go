package talonsandbox

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/http/cookiejar"
	"net/url"
	"strings"
	"time"

	"x.xgit.pro/dark/talon-sandbox-sdk-go/internal/httpx"
)

const defaultRequestTimeout = 30 * time.Second

// Client is the HTTP client for all SDK operations.
// A Client is safe for concurrent use by multiple goroutines.
type Client struct {
	baseURL    string
	httpClient *http.Client
	apiKey     string
}

// Option is a functional option for New.
type Option func(*Client)

// WithAPIKey sets Bearer token authentication.
func WithAPIKey(key string) Option {
	return func(c *Client) { c.apiKey = key }
}

// WithHTTPClient replaces the default http.Client (useful for testing).
func WithHTTPClient(hc *http.Client) Option {
	return func(c *Client) { c.httpClient = hc }
}

// WithTimeout sets the default per-request timeout.
func WithTimeout(d time.Duration) Option {
	return func(c *Client) { c.httpClient.Timeout = d }
}

// WithBaseURL overrides the API server URL. Useful in tests.
func WithBaseURL(url string) Option {
	return func(c *Client) {
		c.baseURL = strings.TrimRight(url, "/")
	}
}

// New creates a Client.
//
// serverURL 默认值为官方托管端点 https://api.sandbox.talon.net.cn；
// 自部署用户可通过 TALON_SANDBOX_SERVER 环境变量或显式传入 serverURL 覆盖。
// 传入空字符串等同于使用默认值。
// 通过 WithAPIKey 设置认证 token，或依赖 TALON_SANDBOX_API_KEY 环境变量。
func New(serverURL string, opts ...Option) *Client {
	if serverURL == "" {
		serverURL = defaultServer
	}
	serverURL = strings.TrimRight(serverURL, "/")

	jar, _ := cookiejar.New(nil)
	c := &Client{
		baseURL: serverURL,
		httpClient: &http.Client{
			Timeout: defaultRequestTimeout,
			Jar:     jar,
		},
	}
	for _, o := range opts {
		o(c)
	}
	return c
}

// HTTPClient returns the underlying *http.Client (useful for testing with WithHTTPClient).
func (c *Client) HTTPClient() *http.Client { return c.httpClient }

// BaseURL returns the configured API base URL.
func (c *Client) BaseURL() string { return c.baseURL }

// ─── Internal HTTP helpers ─────────────────────────────────────────────────────

func (c *Client) newRequest(ctx context.Context, method, path string, body any) (*http.Request, error) {
	u := c.baseURL + path
	var bodyReader io.Reader
	if body != nil {
		b, err := json.Marshal(body)
		if err != nil {
			return nil, fmt.Errorf("marshal request: %w", err)
		}
		bodyReader = bytes.NewReader(b)
	}
	req, err := http.NewRequestWithContext(ctx, method, u, bodyReader)
	if err != nil {
		return nil, err
	}
	if body != nil {
		req.Header.Set("Content-Type", "application/json")
	}
	req.Header.Set("Accept", "application/json")
	if c.apiKey != "" {
		req.Header.Set("Authorization", "Bearer "+c.apiKey)
	}
	return req, nil
}

func (c *Client) do(req *http.Request, out any) (*http.Response, error) {
	resp, err := httpx.DoJSON(c.httpClient, req, out)
	if err != nil {
		if httpx.IsNetworkError(err) {
			return resp, &NetworkError{Cause: err}
		}
		if httpx.IsAPIError(err) {
			ae := newAPIError(httpx.StatusCode(err), httpx.Message(err))
			if resp != nil {
				ae.RequestID = resp.Header.Get("X-Request-ID")
			}
			return resp, ae
		}
		return resp, err
	}
	return resp, nil
}

func (c *Client) get(ctx context.Context, path string, out any) (*http.Response, error) {
	req, err := c.newRequest(ctx, http.MethodGet, path, nil)
	if err != nil {
		return nil, err
	}
	return c.do(req, out)
}

func (c *Client) post(ctx context.Context, path string, body any, out any) (*http.Response, error) {
	req, err := c.newRequest(ctx, http.MethodPost, path, body)
	if err != nil {
		return nil, err
	}
	return c.do(req, out)
}

func (c *Client) delete(ctx context.Context, path string) (*http.Response, error) {
	req, err := c.newRequest(ctx, http.MethodDelete, path, nil)
	if err != nil {
		return nil, err
	}
	return c.do(req, nil)
}

// wsURL converts the base HTTP URL to a WebSocket URL for the given path.
func (c *Client) wsURL(path string) string {
	base := c.baseURL
	base = strings.Replace(base, "https://", "wss://", 1)
	base = strings.Replace(base, "http://", "ws://", 1)
	return base + path
}

// authHeader returns the Authorization header value.
func (c *Client) authHeader() string {
	if c.apiKey != "" {
		return "Bearer " + c.apiKey
	}
	return ""
}

// authCookies returns cookies from the jar (for WebSocket handshakes).
func (c *Client) authCookies() []*http.Cookie {
	if c.httpClient.Jar == nil {
		return nil
	}
	u, err := url.Parse(c.baseURL)
	if err != nil {
		return nil
	}
	return c.httpClient.Jar.Cookies(u)
}
