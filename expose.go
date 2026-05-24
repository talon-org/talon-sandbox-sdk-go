package talonsandbox

import (
	"context"
	"errors"
	"fmt"
	"strings"
)

// Expose registers a port for external access and returns the preview URL.
//
// If the server is missing the /expose route entirely (pre-Spec 50),
// ErrNotImplemented is returned. A 404 that names the sandbox or port in the
// body is treated as a real not-found and surfaced as ErrNotFound so callers
// can distinguish "old server" from "wrong sandbox id".
//
// Example:
//
//	url, err := sb.Expose(ctx, 5173)
//	signed, err := sb.Expose(ctx, 5173, sandbox.ExposeOpts{Sign: true, TTL: "1h"})
func (s *Sandbox) Expose(ctx context.Context, port int, opts ...ExposeOpts) (string, error) {
	body := map[string]any{"port": port}
	if len(opts) > 0 {
		o := opts[0]
		if o.Sign {
			body["sign"] = true
		}
		if o.TTL != "" {
			body["ttl"] = o.TTL
		}
		if o.Subdomain != "" {
			body["subdomain"] = o.Subdomain
		}
	}

	var resp struct {
		Port      int    `json:"port"`
		URL       string `json:"url"`
		Signed    bool   `json:"signed"`
		ExpiresAt string `json:"expires_at"`
	}
	_, err := s.client.post(ctx, fmt.Sprintf("/v1/sandboxes/%s/expose", s.info.ID), body, &resp)
	if err != nil {
		if endpointMissing(err) {
			return "", fmt.Errorf("%w: expose endpoint not yet available on this server", ErrNotImplemented)
		}
		return "", fmt.Errorf("expose port %d: %w", port, err)
	}
	return resp.URL, nil
}

// Unexpose removes the explicit port exposure.
func (s *Sandbox) Unexpose(ctx context.Context, port int) error {
	_, err := s.client.delete(ctx, fmt.Sprintf("/v1/sandboxes/%s/expose/%d", s.info.ID, port))
	if err != nil {
		if endpointMissing(err) {
			return fmt.Errorf("%w: expose endpoint not yet available on this server", ErrNotImplemented)
		}
		return fmt.Errorf("unexpose port %d: %w", port, err)
	}
	return nil
}

// Exposed returns the list of currently exposed ports for this sandbox.
func (s *Sandbox) Exposed(ctx context.Context) ([]ExposedPort, error) {
	var resp struct {
		Ports []ExposedPort `json:"ports"`
	}
	_, err := s.client.get(ctx, fmt.Sprintf("/v1/sandboxes/%s/expose", s.info.ID), &resp)
	if err != nil {
		if endpointMissing(err) {
			return nil, fmt.Errorf("%w: expose endpoint not yet available on this server", ErrNotImplemented)
		}
		return nil, fmt.Errorf("list exposed ports: %w", err)
	}
	return resp.Ports, nil
}

// endpointMissing returns true when a 404 error body is the chi router's
// default "404 page not found" (no JSON, no resource name) — i.e. the server
// genuinely doesn't expose this route. A 404 whose message names "sandbox" or
// "port" is a real not-found from the handler and should propagate as
// ErrNotFound to let the caller fix their input.
func endpointMissing(err error) bool {
	var ae *APIError
	if !errors.As(err, &ae) || ae.StatusCode != 404 {
		return false
	}
	msg := strings.ToLower(ae.Message)
	if strings.Contains(msg, "sandbox") || strings.Contains(msg, "port") {
		return false
	}
	return true
}
