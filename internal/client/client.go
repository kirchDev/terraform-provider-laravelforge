// Package client is a minimal client for the new (org-scoped, JSON:API) Laravel
// Forge API.
//
// Shape verified against the live API (2026-06-06):
//   - Base URL: https://forge.laravel.com ; all paths under /api/orgs/{org}/...
//   - Auth: Authorization: Bearer <token>.
//   - READ responses are JSON:API: a single resource is
//     {"data": {"id": "<string>", "type": "...", "attributes": {...}}} and a
//     collection is {"data": [ <resource>, ... ], "links": ..., "meta": ...}.
//     The real fields live under "attributes" (where the numeric "id" also
//     appears); the resource-level "id" is a string.
//   - WRITE requests (create/update) send a FLAT JSON body (not JSON:API); the
//     response is JSON:API (often 202 Accepted, since Forge provisions async).
package client

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"
)

// DefaultEndpoint is the base URL of the Forge API.
const DefaultEndpoint = "https://forge.laravel.com"

// Client talks to the Laravel Forge API.
type Client struct {
	httpClient *http.Client
	endpoint   string
	token      string
}

// New constructs a Client. An empty endpoint falls back to DefaultEndpoint.
func New(endpoint, token string) *Client {
	if endpoint == "" {
		endpoint = DefaultEndpoint
	}
	return &Client{
		httpClient: &http.Client{Timeout: 60 * time.Second},
		endpoint:   strings.TrimRight(endpoint, "/"),
		token:      token,
	}
}

// RawResource is a single JSON:API resource object.
type RawResource struct {
	ID         string          `json:"id"`
	Type       string          `json:"type"`
	Attributes json.RawMessage `json:"attributes"`
}

type resourceDoc struct {
	Data RawResource `json:"data"`
}

type collectionDoc struct {
	Data []RawResource `json:"data"`
}

// Get fetches a single resource at path and, when attrs is non-nil, unmarshals
// its JSON:API attributes into it. Returns the resource-level (string) id.
func (c *Client) Get(ctx context.Context, path string, attrs any) (string, error) {
	var doc resourceDoc
	if err := c.do(ctx, http.MethodGet, path, nil, &doc); err != nil {
		return "", err
	}
	if attrs != nil && len(doc.Data.Attributes) > 0 {
		if err := json.Unmarshal(doc.Data.Attributes, attrs); err != nil {
			return doc.Data.ID, fmt.Errorf("decoding attributes for %s: %w", path, err)
		}
	}
	return doc.Data.ID, nil
}

// List fetches a collection at path and returns the raw resources. Each
// caller unmarshals RawResource.Attributes into its own type.
func (c *Client) List(ctx context.Context, path string) ([]RawResource, error) {
	var doc collectionDoc
	if err := c.do(ctx, http.MethodGet, path, nil, &doc); err != nil {
		return nil, err
	}
	return doc.Data, nil
}

// Write sends body as a FLAT JSON request (create/update) and, when attrs is
// non-nil, unmarshals the JSON:API response attributes into it. Returns the
// resource-level (string) id.
func (c *Client) Write(ctx context.Context, method, path string, body, attrs any) (string, error) {
	var r io.Reader
	if body != nil {
		b, err := json.Marshal(body)
		if err != nil {
			return "", err
		}
		r = bytes.NewReader(b)
	}
	var doc resourceDoc
	if err := c.do(ctx, method, path, r, &doc); err != nil {
		return "", err
	}
	if attrs != nil && len(doc.Data.Attributes) > 0 {
		if err := json.Unmarshal(doc.Data.Attributes, attrs); err != nil {
			return doc.Data.ID, fmt.Errorf("decoding attributes for %s: %w", path, err)
		}
	}
	return doc.Data.ID, nil
}

// Delete issues a DELETE against path.
func (c *Client) Delete(ctx context.Context, path string) error {
	return c.do(ctx, http.MethodDelete, path, nil, nil)
}

// NotFound reports whether err is a 404 from the API (useful for Read to drop
// resources from state).
func NotFound(err error) bool {
	var e *APIError
	if errors.As(err, &e) {
		return e.StatusCode == http.StatusNotFound
	}
	return false
}

// APIError is a non-2xx API response.
type APIError struct {
	StatusCode int
	Method     string
	Path       string
	Body       string
}

func (e *APIError) Error() string {
	return fmt.Sprintf("forge API %s %s: status %d: %s", e.Method, e.Path, e.StatusCode, e.Body)
}

// do performs an authenticated request and, when out is non-nil, decodes a 2xx
// JSON body into it.
func (c *Client) do(ctx context.Context, method, path string, body io.Reader, out any) error {
	req, err := http.NewRequestWithContext(ctx, method, c.endpoint+path, body)
	if err != nil {
		return err
	}
	req.Header.Set("Authorization", "Bearer "+c.token)
	req.Header.Set("Accept", "application/json")
	req.Header.Set("Content-Type", "application/json")

	res, err := c.httpClient.Do(req)
	if err != nil {
		return err
	}
	defer func() { _ = res.Body.Close() }()

	data, err := io.ReadAll(res.Body)
	if err != nil {
		return err
	}

	if res.StatusCode < 200 || res.StatusCode >= 300 {
		return &APIError{StatusCode: res.StatusCode, Method: method, Path: path, Body: strings.TrimSpace(string(data))}
	}

	if out != nil && len(data) > 0 {
		if err := json.Unmarshal(data, out); err != nil {
			return fmt.Errorf("decoding %s %s response: %w", method, path, err)
		}
	}
	return nil
}
