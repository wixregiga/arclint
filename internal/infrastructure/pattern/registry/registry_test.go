package registrypattern_test

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/wixregiga/arclint/internal/domain/distribution"
	"github.com/wixregiga/arclint/internal/domain/rule"
	embeddedpattern "github.com/wixregiga/arclint/internal/infrastructure/pattern/embedded"
	registrypattern "github.com/wixregiga/arclint/internal/infrastructure/pattern/registry"
)

// builtIn returns the named built-in Pattern with its files.
func builtIn(t *testing.T, name string) distribution.Available {
	t.Helper()
	available, err := embeddedpattern.NewSource().Available()
	if err != nil {
		t.Fatalf("Available: %v", err)
	}
	for _, a := range available {
		if a.Reference().Name() == name {
			return a
		}
	}
	t.Fatalf("no built-in pattern %q", name)
	return distribution.Available{}
}

func reference(t *testing.T, spelling string) rule.PatternReference {
	t.Helper()
	ref, err := rule.ParsePatternReference(spelling)
	if err != nil {
		t.Fatalf("ParsePatternReference(%q): %v", spelling, err)
	}
	return ref
}

func TestPublishThenFetchOverFileURL(t *testing.T) {
	dir := t.TempDir()
	publisher := registrypattern.NewPublisher()
	vertical := builtIn(t, "vertical")
	published, err := publisher.Publish(dir, vertical)
	if err != nil {
		t.Fatalf("Publish: %v", err)
	}
	if published.Replaced || published.VersionDir != filepath.Join(dir, "arclint", "vertical", "0.1.0") ||
		published.IndexPath != filepath.Join(dir, "index.json") {
		t.Errorf("published = %+v", published)
	}
	if _, err := publisher.Publish(dir, builtIn(t, "domain-model")); err != nil {
		t.Fatalf("Publish second: %v", err)
	}
	again, err := publisher.Publish(dir, vertical)
	if err != nil {
		t.Fatalf("Publish again: %v", err)
	}
	if !again.Replaced {
		t.Errorf("publishing a listed version again must report the replacement")
	}
	indexData, err := os.ReadFile(published.IndexPath)
	if err != nil {
		t.Fatalf("ReadFile index: %v", err)
	}
	var indexDoc struct {
		Patterns []struct {
			Pattern    string   `json:"pattern"`
			Digest     string   `json:"digest"`
			Coverage   []string `json:"coverage"`
			Rules      int      `json:"rules"`
			Extensions int      `json:"extensions"`
		} `json:"patterns"`
	}
	if err := json.Unmarshal(indexData, &indexDoc); err != nil {
		t.Fatalf("index.json: %v", err)
	}
	if len(indexDoc.Patterns) != 2 || indexDoc.Patterns[0].Pattern != "arclint/domain-model@0.1.0" ||
		indexDoc.Patterns[1].Pattern != "arclint/vertical@0.1.0" {
		t.Errorf("index lists %+v", indexDoc.Patterns)
	}
	if indexDoc.Patterns[1].Digest != vertical.Digest().String() || indexDoc.Patterns[1].Rules != 16 ||
		indexDoc.Patterns[1].Extensions != 5 || strings.Join(indexDoc.Patterns[1].Coverage, ",") != "go" {
		t.Errorf("vertical index entry = %+v", indexDoc.Patterns[1])
	}
	if _, err := os.Stat(filepath.Join(published.VersionDir, "manifest.json")); err != nil {
		t.Errorf("manifest.json: %v", err)
	}
	if _, err := os.Stat(filepath.Join(published.VersionDir, "extensions", "vertical_usecase.ts")); err != nil {
		t.Errorf("extension file: %v", err)
	}

	reg, err := distribution.NewRegistry("file://" + filepath.ToSlash(dir))
	if err != nil {
		t.Fatalf("NewRegistry: %v", err)
	}
	client := registrypattern.NewClient("")
	index, err := client.Index(reg)
	if err != nil {
		t.Fatalf("Index: %v", err)
	}
	if entries := index.Entries(); len(entries) != 2 {
		t.Errorf("index entries = %d", len(entries))
	}
	fetched, err := client.Fetch(reg, vertical.Reference())
	if err != nil {
		t.Fatalf("Fetch: %v", err)
	}
	if fetched.Kind != distribution.SourceRegistry || fetched.Authored {
		t.Errorf("fetched kind %q authored %v", fetched.Kind, fetched.Authored)
	}
	if !fetched.Digest().Equals(vertical.Digest()) {
		t.Errorf("fetched digest %s, published %s", fetched.Digest(), vertical.Digest())
	}
	if len(fetched.Pattern.Rules()) != 16 || len(fetched.Pattern.Extensions()) != 5 {
		t.Errorf("fetched pattern has %d rules and %d extensions", len(fetched.Pattern.Rules()), len(fetched.Pattern.Extensions()))
	}
	if _, err := client.Fetch(reg, reference(t, "arclint/vertical@9.9.9")); err == nil || !strings.Contains(err.Error(), "not found") {
		t.Errorf("an unpublished version must fail as not found, got %v", err)
	}

	tampered := filepath.Join(published.VersionDir, "pattern.yaml")
	data, err := os.ReadFile(tampered)
	if err != nil {
		t.Fatalf("ReadFile: %v", err)
	}
	if err := os.WriteFile(tampered, append(data, []byte("# tampered\n")...), 0o644); err != nil {
		t.Fatalf("WriteFile: %v", err)
	}
	if _, err := client.Fetch(reg, vertical.Reference()); err == nil || !strings.Contains(err.Error(), "has digest") {
		t.Errorf("a file that disagrees with its manifest must be refused, got %v", err)
	}
}

func TestFetchOverHTTPSendsTheToken(t *testing.T) {
	dir := t.TempDir()
	if _, err := registrypattern.NewPublisher().Publish(dir, builtIn(t, "domain-model")); err != nil {
		t.Fatalf("Publish: %v", err)
	}
	var sawToken bool
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Header.Get("Authorization") == "Bearer secret" {
			sawToken = true
		}
		http.FileServer(http.Dir(dir)).ServeHTTP(w, r)
	}))
	defer server.Close()
	reg, err := distribution.NewRegistry(server.URL)
	if err != nil {
		t.Fatalf("NewRegistry: %v", err)
	}
	client := registrypattern.NewClient("secret")
	index, err := client.Index(reg)
	if err != nil {
		t.Fatalf("Index: %v", err)
	}
	refs := index.References()
	if len(refs) != 1 || refs[0].String() != "arclint/domain-model@0.1.0" {
		t.Errorf("index = %v", refs)
	}
	fetched, err := client.Fetch(reg, refs[0])
	if err != nil {
		t.Fatalf("Fetch: %v", err)
	}
	if len(fetched.Pattern.Extensions()) != 3 {
		t.Errorf("fetched %d extensions, want 3", len(fetched.Pattern.Extensions()))
	}
	if !sawToken {
		t.Errorf("the bearer token was not sent")
	}
	if _, err := client.Fetch(reg, reference(t, "acme/none@1.0.0")); err == nil || !strings.Contains(err.Error(), "not found") {
		t.Errorf("missing manifest must be not found, got %v", err)
	}

	forbidden := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		http.Error(w, "nope", http.StatusForbidden)
	}))
	defer forbidden.Close()
	privateReg, err := distribution.NewRegistry(forbidden.URL)
	if err != nil {
		t.Fatalf("NewRegistry: %v", err)
	}
	if _, err := registrypattern.NewClient("").Index(privateReg); err == nil || !strings.Contains(err.Error(), "GITHUB_TOKEN") {
		t.Errorf("a forbidden registry must point at the token, got %v", err)
	}
}

// TestFileRegistryReadsStayInsideTheTree pins the confinement of a
// file Registry: a document that resolves outside the Registry
// directory, here through a symbolic link, is refused rather than read,
// and a Registry whose directory is missing says so.
func TestFileRegistryReadsStayInsideTheTree(t *testing.T) {
	outside := t.TempDir()
	secret := filepath.Join(outside, "index.json")
	if err := os.WriteFile(secret, []byte(`{"patterns":[]}`), 0o600); err != nil {
		t.Fatalf("WriteFile: %v", err)
	}
	tree := filepath.Join(t.TempDir(), "registry")
	if err := os.MkdirAll(tree, 0o750); err != nil {
		t.Fatalf("MkdirAll: %v", err)
	}
	if err := os.Symlink(secret, filepath.Join(tree, "index.json")); err != nil {
		t.Skipf("symlinks unavailable: %v", err)
	}
	client := registrypattern.NewClient("")
	reg, err := distribution.NewRegistry("file://" + filepath.ToSlash(tree))
	if err != nil {
		t.Fatalf("NewRegistry: %v", err)
	}
	if _, err := client.Index(reg); err == nil || !strings.Contains(err.Error(), "escapes") {
		t.Errorf("an index linked from outside the registry directory must be refused, got %v", err)
	}

	missing, err := distribution.NewRegistry("file://" + filepath.ToSlash(filepath.Join(tree, "nowhere")))
	if err != nil {
		t.Fatalf("NewRegistry: %v", err)
	}
	if _, err := client.Index(missing); err == nil || !strings.Contains(err.Error(), "does not exist") {
		t.Errorf("a missing registry directory must be named, got %v", err)
	}
}
