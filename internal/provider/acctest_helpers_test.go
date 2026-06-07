package provider

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"os/exec"
	"strconv"
	"strings"
	"sync"
	"testing"

	"github.com/hashicorp/terraform-plugin-framework/providerserver"
	"github.com/hashicorp/terraform-plugin-go/tfprotov6"
	"github.com/hashicorp/terraform-plugin-testing/helper/resource"
	"github.com/hashicorp/terraform-plugin-testing/terraform"
)

// testAccProtoV6ProviderFactories wires the in-process provider for the
// terraform-plugin-testing acceptance harness. Each test points the provider at
// a mockForge via the FORGE_ENDPOINT env var, so the full plan/apply/refresh/
// import/destroy cycle runs against an in-memory API — no token, no real Forge.
func testAccProtoV6ProviderFactories() map[string]func() (tfprotov6.ProviderServer, error) {
	return map[string]func() (tfprotov6.ProviderServer, error){
		"laravelforge": providerserver.NewProtocol6WithError(New("test")()),
	}
}

// mockOpts configures a mockForge for one resource's shape.
type mockOpts struct {
	// typeName is the JSON:API "type" echoed back (cosmetic).
	typeName string
	// defaults are merged into every created resource — the computed fields the
	// real API would populate (status, created_at, …) so state stays consistent.
	defaults map[string]any
	// onRead, if set, mutates a copy of the attributes just before they are
	// returned on a read. Used to emulate quirks like the scheduled-job API
	// Capitalizing the frequency it echoes back.
	onRead func(attrs map[string]any)
	// singleton marks an integration-toggle resource whose create/read/delete all
	// hit the same path and carry no id (e.g. site_horizon). The store is then
	// keyed by request path instead of by a trailing id segment.
	singleton bool
}

// mockForge is an in-memory stand-in for the Forge JSON:API — enough to drive a
// resource through create → read → update → delete. It mirrors the real
// contract: writes carry a FLAT JSON body, reads return JSON:API
// ({"data":{"id":"<string>","type":...,"attributes":{...}}}). It is path-shape
// agnostic (keys purely on the trailing id segment), so it serves both the
// server-scoped write paths and the org-level read paths a resource may use.
type mockForge struct {
	url string

	mu       sync.Mutex
	nextID   int
	store    map[string]map[string]any
	opts     mockOpts
	requests []string // "METHOD /path" log, for assertions
}

// newMockForge starts a mock server, points the provider at it, and wires the
// rest of the acceptance environment so a test only needs this one call. (TF_ACC
// itself stays external — it gates whether acceptance tests run at all.)
func newMockForge(t *testing.T, opts mockOpts) *mockForge {
	t.Helper()
	m := &mockForge{store: map[string]map[string]any{}, opts: opts}
	srv := httptest.NewServer(m)
	t.Cleanup(srv.Close)
	m.url = srv.URL

	// Point the provider at the mock.
	t.Setenv("FORGE_ENDPOINT", m.url)
	t.Setenv("FORGE_TOKEN", "test")

	// The harness reattaches the provider under the address tofu resolves a bare
	// "laravelforge" to (registry.opentofu.org/hashicorp/...); the framework
	// otherwise defaults to registry.terraform.io/-/..., which tofu rejects.
	t.Setenv("TF_ACC_PROVIDER_HOST", "registry.opentofu.org")
	t.Setenv("TF_ACC_PROVIDER_NAMESPACE", "hashicorp")

	// Let `TF_ACC=1 go test` find a TF binary without the caller exporting a
	// path: prefer an explicit one, else fall back to tofu on PATH.
	if os.Getenv("TF_ACC_TERRAFORM_PATH") == "" {
		if tofu, err := exec.LookPath("tofu"); err == nil {
			t.Setenv("TF_ACC_TERRAFORM_PATH", tofu)
		}
	}
	return m
}

func (m *mockForge) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.requests = append(m.requests, r.Method+" "+r.URL.Path)

	if m.opts.singleton {
		m.serveSingleton(w, r)
		return
	}

	segs := strings.Split(strings.Trim(r.URL.Path, "/"), "/")
	last := segs[len(segs)-1]
	_, lastNotNumeric := strconv.Atoi(last) // err != nil ⇒ collection segment, not an item id

	switch r.Method {
	case http.MethodPost: // create on a collection
		body := decodeBody(r)
		m.nextID++
		id := strconv.Itoa(m.nextID)
		attrs := map[string]any{}
		for k, v := range m.opts.defaults {
			attrs[k] = v
		}
		for k, v := range body {
			attrs[k] = v
		}
		attrs["id"] = m.nextID
		m.store[id] = attrs
		writeJSONAPI(w, http.StatusCreated, id, m.opts.typeName, m.readAttrs(attrs))

	case http.MethodGet:
		attrs, ok := m.store[last]
		if !ok || lastNotNumeric != nil {
			http.Error(w, `{"message":"not found"}`, http.StatusNotFound)
			return
		}
		writeJSONAPI(w, http.StatusOK, last, m.opts.typeName, m.readAttrs(attrs))

	case http.MethodPut, http.MethodPatch:
		attrs, ok := m.store[last]
		if !ok {
			http.Error(w, `{"message":"not found"}`, http.StatusNotFound)
			return
		}
		for k, v := range decodeBody(r) {
			attrs[k] = v
		}
		m.store[last] = attrs
		writeJSONAPI(w, http.StatusOK, last, m.opts.typeName, m.readAttrs(attrs))

	case http.MethodDelete:
		delete(m.store, last)
		w.WriteHeader(http.StatusNoContent)

	default:
		http.Error(w, `{"message":"method not allowed"}`, http.StatusMethodNotAllowed)
	}
}

// readAttrs returns a copy of attrs with any onRead quirk applied.
func (m *mockForge) readAttrs(attrs map[string]any) map[string]any {
	out := make(map[string]any, len(attrs))
	for k, v := range attrs {
		out[k] = v
	}
	if m.opts.onRead != nil {
		m.opts.onRead(out)
	}
	return out
}

// serveSingleton handles integration-toggle resources keyed by request path.
func (m *mockForge) serveSingleton(w http.ResponseWriter, r *http.Request) {
	key := r.URL.Path
	switch r.Method {
	case http.MethodPost, http.MethodPut, http.MethodPatch:
		attrs := m.store[key]
		if attrs == nil {
			attrs = map[string]any{}
			for k, v := range m.opts.defaults {
				attrs[k] = v
			}
		}
		for k, v := range decodeBody(r) {
			attrs[k] = v
		}
		m.store[key] = attrs
		status := http.StatusOK
		if r.Method == http.MethodPost {
			status = http.StatusCreated
		}
		writeJSONAPI(w, status, "0", m.opts.typeName, m.readAttrs(attrs))
	case http.MethodGet:
		attrs, ok := m.store[key]
		if !ok {
			http.Error(w, `{"message":"not found"}`, http.StatusNotFound)
			return
		}
		writeJSONAPI(w, http.StatusOK, "0", m.opts.typeName, m.readAttrs(attrs))
	case http.MethodDelete:
		delete(m.store, key)
		w.WriteHeader(http.StatusNoContent)
	default:
		http.Error(w, `{"message":"method not allowed"}`, http.StatusMethodNotAllowed)
	}
}

// importIDFunc builds an ImportState id by joining the named state attributes
// with "/", matching each resource's ImportState format (e.g. "organization",
// "server_id", "id").
func importIDFunc(rn string, attrNames ...string) resource.ImportStateIdFunc {
	return func(s *terraform.State) (string, error) {
		rs, ok := s.RootModule().Resources[rn]
		if !ok {
			return "", fmt.Errorf("resource %s not found in state", rn)
		}
		parts := make([]string, len(attrNames))
		for i, name := range attrNames {
			parts[i] = rs.Primary.Attributes[name]
		}
		return strings.Join(parts, "/"), nil
	}
}

func decodeBody(r *http.Request) map[string]any {
	defer func() { _ = r.Body.Close() }()
	data, _ := io.ReadAll(r.Body)
	if len(data) == 0 {
		return map[string]any{}
	}
	var body map[string]any
	if err := json.Unmarshal(data, &body); err != nil {
		return map[string]any{}
	}
	return body
}

func writeJSONAPI(w http.ResponseWriter, status int, id, typeName string, attrs map[string]any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(map[string]any{
		"data": map[string]any{"id": id, "type": typeName, "attributes": attrs},
	})
}
