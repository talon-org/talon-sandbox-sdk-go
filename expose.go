package talonsandbox

import (
	"context"
	"errors"
	"fmt"
)

// Expose registers a port for external access and returns the preview URL.
//
// If the server returns 404 (endpoint not yet implemented per Spec 50),
// ErrNotImplemented is returned with a human-friendly message.
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
		var ae *APIError
		if errors.As(err, &ae) && ae.StatusCode == 404 {
			return "", fmt.Errorf("%w: expose endpoint not yet available on this server (Spec 50 pending)", ErrNotImplemented)
		}
		return "", fmt.Errorf("expose port %d: %w", port, err)
	}
	return resp.URL, nil
}

// Unexpose removes the explicit port exposure.
func (s *Sandbox) Unexpose(ctx context.Context, port int) error {
	_, err := s.client.delete(ctx, fmt.Sprintf("/v1/sandboxes/%s/expose/%d", s.info.ID, port))
	if err != nil {
		var ae *APIError
		if errors.As(err, &ae) && ae.StatusCode == 404 {
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
		var ae *APIError
		if errors.As(err, &ae) && ae.StatusCode == 404 {
			return nil, fmt.Errorf("%w: expose endpoint not yet available on this server", ErrNotImplemented)
		}
		return nil, fmt.Errorf("list exposed ports: %w", err)
	}
	return resp.Ports, nil
}
