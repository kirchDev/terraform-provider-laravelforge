package client

import (
	"context"
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"net/http/httptest"
	"testing"
)

// recorded captures what the test server saw on the last request, so a test can
// assert on the method, path, headers and (flat JSON) body the client sent.
type recorded struct {
	method string
	path   string
	auth   string
	accept string
	ctype  string
	body   []byte
}

// newServer spins up a test server whose handler records the inbound request
// into rec and then runs respond to produce the reply. The returned Client is
// pointed at the server.
func newServer(t *testing.T, rec *recorded, respond func(w http.ResponseWriter, r *http.Request)) *Client {
	t.Helper()
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		body, _ := io.ReadAll(r.Body)
		rec.method = r.Method
		rec.path = r.URL.Path
		rec.auth = r.Header.Get("Authorization")
		rec.accept = r.Header.Get("Accept")
		rec.ctype = r.Header.Get("Content-Type")
		rec.body = body
		respond(w, r)
	}))
	t.Cleanup(srv.Close)
	return New(srv.URL, "secret-token")
}

func TestNewDefaultsEndpoint(t *testing.T) {
	c := New("", "tok")
	if c.endpoint != DefaultEndpoint {
		t.Errorf("empty endpoint = %q, want %q", c.endpoint, DefaultEndpoint)
	}
	if got := New("https://example.test/", "tok").endpoint; got != "https://example.test" {
		t.Errorf("trailing slash not trimmed: %q", got)
	}
}

func TestGetParsesJSONAPIAndSetsHeaders(t *testing.T) {
	var rec recorded
	c := newServer(t, &rec, func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		// JSON:API single resource: real fields live under attributes; the
		// resource-level id is a string while attributes.id is numeric.
		_, _ = io.WriteString(w, `{"data":{"id":"42","type":"servers","attributes":{"id":42,"name":"web-1","region":"fra1"}}}`)
	})

	var attrs struct {
		ID     int64  `json:"id"`
		Name   string `json:"name"`
		Region string `json:"region"`
	}
	id, err := c.Get(context.Background(), "/api/orgs/kirchdev/servers/42", &attrs)
	if err != nil {
		t.Fatalf("Get: %v", err)
	}
	if id != "42" {
		t.Errorf("resource id = %q, want %q", id, "42")
	}
	if attrs.Name != "web-1" || attrs.Region != "fra1" || attrs.ID != 42 {
		t.Errorf("attributes not decoded: %+v", attrs)
	}
	if rec.method != http.MethodGet {
		t.Errorf("method = %q, want GET", rec.method)
	}
	if rec.path != "/api/orgs/kirchdev/servers/42" {
		t.Errorf("path = %q", rec.path)
	}
	if rec.auth != "Bearer secret-token" {
		t.Errorf("auth header = %q", rec.auth)
	}
	if rec.accept != "application/json" || rec.ctype != "application/json" {
		t.Errorf("accept=%q content-type=%q", rec.accept, rec.ctype)
	}
}

func TestListReturnsRawResources(t *testing.T) {
	var rec recorded
	c := newServer(t, &rec, func(w http.ResponseWriter, r *http.Request) {
		_, _ = io.WriteString(w, `{"data":[
			{"id":"1","type":"sites","attributes":{"name":"a.test"}},
			{"id":"2","type":"sites","attributes":{"name":"b.test"}}
		],"links":{"self":"x"},"meta":{}}`)
	})

	res, err := c.List(context.Background(), "/api/orgs/kirchdev/sites")
	if err != nil {
		t.Fatalf("List: %v", err)
	}
	if len(res) != 2 {
		t.Fatalf("len = %d, want 2", len(res))
	}
	if res[0].ID != "1" || res[1].ID != "2" {
		t.Errorf("ids = %q, %q", res[0].ID, res[1].ID)
	}
	var a struct {
		Name string `json:"name"`
	}
	if err := json.Unmarshal(res[0].Attributes, &a); err != nil {
		t.Fatalf("unmarshal attrs: %v", err)
	}
	if a.Name != "a.test" {
		t.Errorf("name = %q", a.Name)
	}
}

func TestWriteSendsFlatBody(t *testing.T) {
	// Writes must be a FLAT JSON map, NOT a JSON:API {"data":{"attributes":...}}
	// envelope. This is the single most regression-prone part of the contract.
	for _, method := range []string{http.MethodPost, http.MethodPut, http.MethodPatch} {
		t.Run(method, func(t *testing.T) {
			var rec recorded
			c := newServer(t, &rec, func(w http.ResponseWriter, r *http.Request) {
				w.WriteHeader(http.StatusAccepted)
				_, _ = io.WriteString(w, `{"data":{"id":"7","type":"sites","attributes":{"id":7,"name":"new.test"}}}`)
			})

			body := map[string]any{"name": "new.test", "php_version": "php83"}
			var attrs struct {
				ID   int64  `json:"id"`
				Name string `json:"name"`
			}
			id, err := c.Write(context.Background(), method, "/api/orgs/kirchdev/servers/1/sites", body, &attrs)
			if err != nil {
				t.Fatalf("Write: %v", err)
			}
			if id != "7" || attrs.Name != "new.test" {
				t.Errorf("id=%q attrs=%+v", id, attrs)
			}
			if rec.method != method {
				t.Errorf("method = %q, want %q", rec.method, method)
			}

			var sent map[string]any
			if err := json.Unmarshal(rec.body, &sent); err != nil {
				t.Fatalf("sent body not JSON: %v (%s)", err, rec.body)
			}
			if _, wrapped := sent["data"]; wrapped {
				t.Errorf("body is JSON:API-wrapped, want flat: %s", rec.body)
			}
			if sent["name"] != "new.test" || sent["php_version"] != "php83" {
				t.Errorf("flat body fields wrong: %s", rec.body)
			}
		})
	}
}

func TestWriteNilBodySendsNoPayload(t *testing.T) {
	var rec recorded
	c := newServer(t, &rec, func(w http.ResponseWriter, r *http.Request) {
		_, _ = io.WriteString(w, `{"data":{"id":"1","type":"x","attributes":{}}}`)
	})
	if _, err := c.Write(context.Background(), http.MethodPost, "/p", nil, nil); err != nil {
		t.Fatalf("Write: %v", err)
	}
	if len(rec.body) != 0 {
		t.Errorf("nil body should send empty payload, got %q", rec.body)
	}
}

func TestDeleteIssuesDelete(t *testing.T) {
	var rec recorded
	c := newServer(t, &rec, func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusNoContent)
	})
	if err := c.Delete(context.Background(), "/api/orgs/kirchdev/sites/7"); err != nil {
		t.Fatalf("Delete: %v", err)
	}
	if rec.method != http.MethodDelete {
		t.Errorf("method = %q, want DELETE", rec.method)
	}
	if rec.path != "/api/orgs/kirchdev/sites/7" {
		t.Errorf("path = %q", rec.path)
	}
}

func TestNotFoundClassifiesStatus(t *testing.T) {
	var rec recorded
	c := newServer(t, &rec, func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusNotFound)
		_, _ = io.WriteString(w, `{"message":"not found"}`)
	})

	_, err := c.Get(context.Background(), "/api/orgs/kirchdev/sites/999", nil)
	if err == nil {
		t.Fatal("expected error on 404")
	}
	if !NotFound(err) {
		t.Errorf("NotFound(err) = false for a 404: %v", err)
	}

	var apiErr *APIError
	if c2 := newServer(t, &rec, func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusUnprocessableEntity)
		_, _ = io.WriteString(w, `{"message":"validation failed"}`)
	}); true {
		_, err = c2.Write(context.Background(), http.MethodPost, "/p", map[string]any{}, nil)
	}
	if err == nil {
		t.Fatal("expected error on 422")
	}
	if NotFound(err) {
		t.Error("NotFound(err) = true for a 422")
	}
	if !errors.As(err, &apiErr) {
		t.Fatalf("error is not *APIError: %v", err)
	}
	if apiErr.StatusCode != http.StatusUnprocessableEntity {
		t.Errorf("status = %d, want 422", apiErr.StatusCode)
	}
	if apiErr.Method != http.MethodPost {
		t.Errorf("APIError.Method = %q", apiErr.Method)
	}
}
