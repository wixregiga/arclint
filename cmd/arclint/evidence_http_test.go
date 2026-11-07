package main

import (
	"bytes"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io/fs"
	"net/http"
	"net/http/httptest"
	"net/url"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"testing"

	"github.com/wixregiga/arclint/internal/application"
	"github.com/wixregiga/arclint/internal/domain/rule"
	"github.com/wixregiga/arclint/internal/domain/vocab"
	jsonbaseline "github.com/wixregiga/arclint/internal/infrastructure/baseline/json"
	sobekextension "github.com/wixregiga/arclint/internal/infrastructure/extension/sobek"
	golangfacts "github.com/wixregiga/arclint/internal/infrastructure/language/golang"
	pythonfacts "github.com/wixregiga/arclint/internal/infrastructure/language/python"
	typescriptfacts "github.com/wixregiga/arclint/internal/infrastructure/language/typescript"
	filesystemobservation "github.com/wixregiga/arclint/internal/infrastructure/observation/filesystem"
	embeddedpattern "github.com/wixregiga/arclint/internal/infrastructure/pattern/embedded"
	filesystempattern "github.com/wixregiga/arclint/internal/infrastructure/pattern/filesystem"
	yamlrule "github.com/wixregiga/arclint/internal/infrastructure/rule/yaml"
)

// composedAssessment composes the Conformance Check for root as run
// composes it for `arclint check`.
func composedAssessment(t *testing.T, root string, knowledge vocab.Repository) application.AssessConformance {
	t.Helper()
	rulesPath := filepath.Join(root, "rules.arclint.yaml")
	local, err := filesystempattern.NewSource(filepath.Join(root, ".arclint", "patterns"))
	if err != nil {
		t.Fatal(err)
	}
	repository, err := yamlrule.NewRepository(rulesPath, knowledge, embeddedpattern.NewSource(), local)
	if err != nil {
		t.Fatal(err)
	}
	observations, err := filesystemobservation.NewSource(root, golangfacts.NewProducer(), typescriptfacts.NewProducer(), pythonfacts.NewProducer())
	if err != nil {
		t.Fatal(err)
	}
	extensions, err := sobekextension.NewEvaluator(root, func() ([]rule.ConfiguredExtension, error) {
		cfg, err := repository.ConfiguredRules()
		if err != nil {
			return nil, err
		}
		return cfg.Extensions, nil
	})
	if err != nil {
		t.Fatal(err)
	}
	baselines, err := jsonbaseline.NewStore(root)
	if err != nil {
		t.Fatal(err)
	}
	assess, err := application.NewAssessConformance(repository, observations, baselines, extensions, knowledge)
	if err != nil {
		t.Fatal(err)
	}
	return assess
}

// evidenceStudio serves Studio over a copy of the Go example, composed
// as `arclint map serve` composes it.
func evidenceStudio(t *testing.T) (string, http.Handler) {
	t.Helper()
	root := t.TempDir()
	if err := os.CopyFS(root, os.DirFS(filepath.Join("..", "..", "docs", "examples", "go"))); err != nil {
		t.Fatal(err)
	}
	// An analyzed source file named for secrets, one holding key
	// material, and a link leading out of the project.
	write := func(name, content string) {
		t.Helper()
		if err := os.WriteFile(filepath.Join(root, name), []byte(content), 0o600); err != nil {
			t.Fatal(err)
		}
	}
	write("identityaccess/tenant/secrets.go", "package tenant\n\nconst plain = 1\n")
	write("identityaccess/tenant/vault.go", "package tenant\n\nconst pem = `\n-----BEGIN RSA PRIVATE KEY-----\nMIIBOgIBAAJBAK\n-----END RSA PRIVATE KEY-----\n`\n")
	write("identityaccess/tenant/config.go", "package tenant\n\nvar apiKey = \"live-0123456789abcdefghij\"\n")
	outside := filepath.Join(t.TempDir(), "outside.go")
	if err := os.WriteFile(outside, []byte("package tenant\n\nconst escaped = \"outside the project\"\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	if err := os.Symlink(outside, filepath.Join(root, "identityaccess/tenant/escape.go")); err != nil {
		t.Fatal(err)
	}
	recording, _, repo, err := composeKnowledge(root, filepath.Join(root, "rules.arclint.yaml"))
	if err != nil {
		t.Fatal(err)
	}
	overview, err := application.NewGetDomainOverview(repo)
	if err != nil {
		t.Fatal(err)
	}
	define, err := application.NewDefineDomainDefinition(repo, root)
	if err != nil {
		t.Fatal(err)
	}
	remove, err := application.NewRemoveDomainDefinition(repo)
	if err != nil {
		t.Fatal(err)
	}
	server, err := composeStudio(root, recording, overview, define, remove, composedAssessment(t, root, repo))
	if err != nil {
		t.Fatal(err)
	}
	h, err := server.Handler()
	if err != nil {
		t.Fatal(err)
	}
	return root, h
}

func serve(h http.Handler, method, target, body string, headers map[string]string) *httptest.ResponseRecorder {
	r := httptest.NewRequest(method, "http://localhost"+target, bytes.NewBufferString(body))
	for k, v := range headers {
		r.Header.Set(k, v)
	}
	w := httptest.NewRecorder()
	h.ServeHTTP(w, r)
	return w
}

type evidenceAnswer struct {
	Snapshot string `json:"snapshot"`
	Complete bool   `json:"complete"`
	Entries  []struct {
		Concept      string `json:"concept"`
		Name         string `json:"name"`
		Reading      string `json:"reading"`
		Declarations struct {
			State     string `json:"state"`
			Locations []struct {
				Path     string `json:"path"`
				Line     int    `json:"line"`
				Readable bool   `json:"readable"`
			} `json:"locations"`
		} `json:"declarations"`
		Contracts []struct {
			Key     string `json:"key"`
			Carrier struct {
				State string `json:"state"`
			} `json:"carrier"`
			Coverage string `json:"coverage"`
		} `json:"contracts"`
	} `json:"entries"`
}

func assessed(t *testing.T, h http.Handler) evidenceAnswer {
	t.Helper()
	w := serve(h, "GET", "/api/evidence", "", nil)
	if w.Code != http.StatusOK {
		t.Fatalf("GET /api/evidence: %d %s", w.Code, w.Body.String())
	}
	if got := w.Header().Get("Cache-Control"); got != "no-store" {
		t.Errorf("Cache-Control %q, want no-store", got)
	}
	var doc evidenceAnswer
	if err := json.Unmarshal(w.Body.Bytes(), &doc); err != nil {
		t.Fatal(err)
	}
	return doc
}

func excerpt(h http.Handler, snapshot, path string, line int) *httptest.ResponseRecorder {
	q := url.Values{"snapshot": {snapshot}, "path": {path}, "line": {fmt.Sprint(line)}}
	return serve(h, "GET", "/api/source?"+q.Encode(), "", nil)
}

// treeDigest hashes every file of the project but ArcLint's cache, so a
// change to source or to the domain recording shows.
func treeDigest(t *testing.T, root string) map[string]string {
	t.Helper()
	out := map[string]string{}
	err := filepath.WalkDir(root, func(p string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		rel, _ := filepath.Rel(root, p)
		if d.IsDir() && rel == filepath.Join(".arclint", "cache") {
			return filepath.SkipDir
		}
		if d.Type().IsRegular() {
			b, err := os.ReadFile(p)
			if err != nil {
				return err
			}
			sum := sha256.Sum256(b)
			out[rel] = hex.EncodeToString(sum[:])
		}
		return nil
	})
	if err != nil {
		t.Fatal(err)
	}
	return out
}

func TestComposedEvidenceAttributesTheExampleAndChangesNothing(t *testing.T) {
	root, h := evidenceStudio(t)
	before := treeDigest(t, root)
	doc := assessed(t, h)
	var tenant bool
	for _, e := range doc.Entries {
		if e.Concept == "aggregate" && e.Name == "Tenant" {
			tenant = true
			if e.Declarations.State != "located" || len(e.Declarations.Locations) != 1 || e.Declarations.Locations[0].Path != "identityaccess/tenant/tenant.go" || !e.Declarations.Locations[0].Readable {
				t.Errorf("Tenant declarations %+v", e.Declarations)
			}
			if len(e.Contracts) != 2 || e.Contracts[0].Carrier.State != "found" || e.Contracts[0].Coverage != "structurally_checked" {
				t.Errorf("Tenant contracts %+v", e.Contracts)
			}
			if e.Reading == "no_attributed_findings" {
				t.Errorf("Tenant reads without findings although a context check did not decide")
			}
		}
	}
	if !tenant {
		t.Fatal("no evidence for aggregate Tenant")
	}
	if w := excerpt(h, doc.Snapshot, "identityaccess/tenant/tenant.go", 16); w.Code != http.StatusOK || !strings.Contains(w.Body.String(), "type Tenant struct") {
		t.Errorf("excerpt of the Tenant root: %d %s", w.Code, w.Body.String())
	}
	if after := treeDigest(t, root); fmt.Sprint(after) != fmt.Sprint(before) {
		t.Errorf("assessing and reading excerpts changed the project:\nbefore %v\nafter  %v", before, after)
	}
}

func TestComposedExcerptsReadOnlyTheSnapshotsSafeSource(t *testing.T) {
	_, h := evidenceStudio(t)
	doc := assessed(t, h)
	cases := []struct {
		name, path string
		line, want int
	}{
		{"traversal", "../../../../etc/passwd", 1, http.StatusBadRequest},
		{"absolute", "/etc/passwd", 1, http.StatusBadRequest},
		{"unclean", "identityaccess/./tenant/tenant.go", 1, http.StatusBadRequest},
		{"backslash", `identityaccess\tenant\tenant.go`, 1, http.StatusBadRequest},
		{"line zero", "identityaccess/tenant/tenant.go", 0, http.StatusBadRequest},
		{"manifest", "go.mod", 1, http.StatusNotFound},
		{"ruleset", "rules.arclint.yaml", 1, http.StatusNotFound},
		{"secret by name", "identityaccess/tenant/secrets.go", 1, http.StatusForbidden},
		{"key material", "identityaccess/tenant/vault.go", 1, http.StatusForbidden},
		{"credential literal", "identityaccess/tenant/config.go", 1, http.StatusForbidden},
		{"beyond the end", "identityaccess/tenant/tenant.go", 100000, http.StatusUnprocessableEntity},
		{"recording", "domain.arclint.yaml", 27, http.StatusOK},
	}
	for _, c := range cases {
		if w := excerpt(h, doc.Snapshot, c.path, c.line); w.Code != c.want {
			t.Errorf("%s: %d %s, want %d", c.name, w.Code, w.Body.String(), c.want)
		}
	}
	// A link leading out of the project is never read, whether or not
	// the scan analyzed it.
	w := excerpt(h, doc.Snapshot, "identityaccess/tenant/escape.go", 1)
	if w.Code == http.StatusOK || strings.Contains(w.Body.String(), "outside the project") {
		t.Errorf("escaping link: %d %s", w.Code, w.Body.String())
	}
	if w := excerpt(h, strings.Repeat("0", 32), "identityaccess/tenant/tenant.go", 16); w.Code != http.StatusConflict {
		t.Errorf("unknown snapshot: %d, want 409", w.Code)
	}
	if w := serve(h, "GET", "/api/source?snapshot="+doc.Snapshot+"&path=identityaccess/tenant/tenant.go&line=x", "", nil); w.Code != http.StatusBadRequest {
		t.Errorf("line not a number: %d", w.Code)
	}
	if w := serve(h, "GET", "/api/evidence", "", map[string]string{"Origin": "http://evil.example"}); w.Code != http.StatusForbidden {
		t.Errorf("another page's assessment: %d, want 403", w.Code)
	}
	for _, site := range []string{"cross-site", "same-site"} {
		if w := serve(h, "GET", "/api/evidence", "", map[string]string{"Sec-Fetch-Site": site}); w.Code != http.StatusForbidden {
			t.Errorf("an assessment a %s page asks for: %d, want 403", site, w.Code)
		}
		if w := serve(h, "GET", "/api/source?snapshot="+doc.Snapshot+"&path=identityaccess/tenant/tenant.go&line=16", "", map[string]string{"Sec-Fetch-Site": site}); w.Code != http.StatusForbidden {
			t.Errorf("an excerpt a %s page asks for: %d, want 403", site, w.Code)
		}
	}
	if w := excerpt(h, doc.Snapshot, "identityaccess/tenant/tenant.go", 16); w.Code != http.StatusOK {
		t.Errorf("excerpt without Sec-Fetch-Site: %d", w.Code)
	}
	if w := serve(h, "POST", "/api/evidence", "", nil); w.Code != http.StatusMethodNotAllowed {
		t.Errorf("POST evidence: %d, want 405", w.Code)
	}
}

// A newer assessment keeps a few older snapshots readable, then the
// oldest is stale; an assessment and an edit at once both complete.
func TestComposedSnapshotsAgeAndAssessmentsWaitForEdits(t *testing.T) {
	_, h := evidenceStudio(t)
	first := assessed(t, h)
	for range 4 {
		assessed(t, h)
	}
	if w := excerpt(h, first.Snapshot, "identityaccess/tenant/tenant.go", 16); w.Code != http.StatusConflict {
		t.Errorf("a snapshot four assessments old: %d, want 409", w.Code)
	}
	var wg sync.WaitGroup
	codes := make([]int, 2)
	wg.Add(2)
	go func() {
		defer wg.Done()
		codes[0] = serve(h, "GET", "/api/evidence", "", nil).Code
	}()
	go func() {
		defer wg.Done()
		codes[1] = serve(h, "POST", "/api/define", `{"concept":"value_object","context":"identityaccess","name":"Seat","change":{"definition":"A place."}}`, map[string]string{"Content-Type": "application/json"}).Code
	}()
	wg.Wait()
	if codes[0] != http.StatusOK || codes[1] != http.StatusOK {
		t.Fatalf("assessment %d and edit %d at once", codes[0], codes[1])
	}
	doc := assessed(t, h)
	for _, e := range doc.Entries {
		if e.Name == "Seat" {
			return
		}
	}
	t.Error("the assessment after the edit does not hold the new entry")
}
