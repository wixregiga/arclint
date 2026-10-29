package main

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"

	"github.com/wixregiga/arclint/internal/delivery/mapserve"
)

// TestStudioCreatesAProjectTheCommandLineThenWorksIn creates a project
// through the map server as the binary composes it, builds a bounded
// context and an aggregate in it from the page's requests, and runs the
// binary in the new project: its ruleset loads, its domain file holds
// what the page published, and its check runs.
func TestStudioCreatesAProjectTheCommandLineThenWorksIn(t *testing.T) {
	if _, err := exec.LookPath("git"); err != nil {
		t.Skip("git is not installed")
	}
	studio, err := composeStudioProject(repoCopy)
	if err != nil {
		t.Fatal(err)
	}
	server, err := composeStudio(repoCopy, mustDefaultRecording(t), studio.Overview, studio.Define, studio.Remove)
	if err != nil {
		t.Fatal(err)
	}
	h, err := server.Handler()
	if err != nil {
		t.Fatal(err)
	}
	dir := filepath.Join(t.TempDir(), "orders")
	request := func(method, path string, body any) *httptest.ResponseRecorder {
		t.Helper()
		data, _ := json.Marshal(body)
		req := httptest.NewRequest(method, path, strings.NewReader(string(data)))
		req.Host = "127.0.0.1:7380"
		req.Header.Set("Content-Type", "application/json")
		rec := httptest.NewRecorder()
		h.ServeHTTP(rec, req)
		return rec
	}
	if rec := request(http.MethodPost, mapserve.ProjectsPath, map[string]any{"directory": dir, "name": "orders", "languages": []string{"go"}}); rec.Code != http.StatusCreated {
		t.Fatalf("create: status = %d, body %s", rec.Code, rec.Body.String())
	}
	for _, edit := range []map[string]any{
		{"project": dir, "concept": "bounded_context", "name": "ordering", "change": map[string]any{"definition": "Taking and fulfilling customer orders."}},
		{"project": dir, "concept": "aggregate", "context": "ordering", "name": "Order", "change": map[string]any{"definition": "A customer's request for goods.", "identity": "OrderID"}},
	} {
		if rec := request(http.MethodPost, mapserve.DefinePath, edit); rec.Code != http.StatusOK {
			t.Fatalf("define %v: status = %d, body %s", edit["name"], rec.Code, rec.Body.String())
		}
	}

	stdout, stderr, code := runBin(t, dir, nil, "--format", "json", "domain", "overview")
	if code != 0 {
		t.Fatalf("domain overview: exit %d\n%s", code, stderr)
	}
	var overview struct {
		Project  string `json:"project"`
		Contexts []struct {
			Name       string `json:"name"`
			Aggregates []struct {
				Name string `json:"name"`
			} `json:"aggregates"`
		} `json:"contexts"`
	}
	if err := json.Unmarshal([]byte(stdout), &overview); err != nil {
		t.Fatalf("decode: %v\n%s", err, stdout)
	}
	if overview.Project != "orders" || len(overview.Contexts) != 1 || overview.Contexts[0].Name != "ordering" ||
		len(overview.Contexts[0].Aggregates) != 1 || overview.Contexts[0].Aggregates[0].Name != "Order" {
		t.Fatalf("the new project's domain = %+v", overview)
	}
	// The check runs over the new ruleset and judges the recorded
	// language: an aggregate with no invariant is a warning, not a failure.
	stdout, stderr, code = runBin(t, dir, nil, "check", ".")
	if code != 0 || !strings.Contains(stdout, "aggregate/protects-an-invariant aggregate Order of context ordering") {
		t.Fatalf("check: exit %d, want the recorded Order judged\n%s\n%s", code, stdout, stderr)
	}
}
