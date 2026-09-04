package main

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// acmePattern is a Pattern no binary embeds, so a test can prove the
// Registry path: authored in one repository, exported, then installed
// elsewhere from the exported tree.
const acmePattern = `pattern:
  namespace: acme
  name: layers
  version: 1.0.0
  coverage: [go]
  documentation: "Two layers: a domain that imports nothing, and an app above it."

modules:
  domain: "The domain model; stdlib-only."
  app:
    description: "Application code above the domain."
    paths: internal/app/**

rules:
  domain/stdlib-only:
    description: "The domain imports no other Module and no third-party package."
    on: domain
    imports:
      internal: []
      external: forbid
`

// authorAcmePattern writes acme/layers under <root>/.arclint/patterns
// as an authored package and returns the repository root.
func authorAcmePattern(t *testing.T) string {
	t.Helper()
	root := t.TempDir()
	write(t, root, filepath.Join(".arclint", "patterns", "acme", "layers", "pattern.yaml"), acmePattern)
	return root
}

func TestPatternsListsEmbeddedAndLocalCopies(t *testing.T) {
	root := authorAcmePattern(t)
	stdout, stderr, code := runBin(t, root, os.Environ(), "patterns")
	if code != 0 {
		t.Fatalf("patterns: exit %d\nstderr: %s", code, stderr)
	}
	for _, want := range []string{"arclint/vertical@0.1.0", "arclint/domain-model@0.1.0", "acme/layers@1.0.0", "authored", "embedded"} {
		if !strings.Contains(stdout, want) {
			t.Errorf("patterns listing misses %q:\n%s", want, stdout)
		}
	}

	stdout, stderr, code = runBin(t, root, os.Environ(), "--format=json", "patterns")
	if code != 0 {
		t.Fatalf("patterns json: exit %d\nstderr: %s", code, stderr)
	}
	var doc struct {
		Patterns []struct {
			Reference string `json:"reference"`
			Source    string `json:"source"`
			Vendored  bool   `json:"vendored"`
			Authored  bool   `json:"authored"`
			Digest    string `json:"digest"`
			Rules     int    `json:"rules"`
		} `json:"patterns"`
	}
	if err := json.Unmarshal([]byte(stdout), &doc); err != nil {
		t.Fatalf("patterns json: %v\n%s", err, stdout)
	}
	if len(doc.Patterns) != 3 {
		t.Fatalf("patterns = %+v", doc.Patterns)
	}
	acme := doc.Patterns[2]
	if acme.Reference != "acme/layers@1.0.0" || acme.Source != "local" || !acme.Authored || acme.Vendored ||
		!strings.HasPrefix(acme.Digest, "sha256:") || acme.Rules != 1 {
		t.Errorf("authored pattern row = %+v", acme)
	}
	if doc.Patterns[0].Source != "embedded" || doc.Patterns[0].Vendored {
		t.Errorf("embedded row = %+v", doc.Patterns[0])
	}
}

func TestPatternsInstallDraftsThenExtendsAndChecks(t *testing.T) {
	root := t.TempDir()
	stdout, stderr, code := runBin(t, root, os.Environ(), "patterns", "install", "domain-model")
	if code != 0 {
		t.Fatalf("install: exit %d\nstderr: %s", code, stderr)
	}
	for _, want := range []string{"installed arclint/domain-model@0.1.0 (embedded, ", "wrote ", "vocabulary: domain.arclint.yaml", "next: run `arclint check .`"} {
		if !strings.Contains(stdout, want) {
			t.Errorf("install output misses %q:\n%s", want, stdout)
		}
	}
	ruleset, err := os.ReadFile(filepath.Join(root, "rules.arclint.yaml"))
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(ruleset), "extends:\n  - pattern: arclint/domain-model@0.1.0\n") {
		t.Errorf("drafted ruleset does not extend the pattern:\n%s", ruleset)
	}
	if _, err := os.Stat(filepath.Join(root, ".arclint")); !os.IsNotExist(err) {
		t.Errorf("installing an embedded pattern must not write .arclint, stat err = %v", err)
	}

	stdout, stderr, code = runBin(t, root, os.Environ(), "patterns", "install", "vertical")
	if code != 0 {
		t.Fatalf("second install: exit %d\nstderr: %s", code, stderr)
	}
	if !strings.Contains(stdout, "extended ") || !strings.Contains(stdout, "  domain: internal/*/domain/**") {
		t.Errorf("second install output:\n%s", stdout)
	}
	ruleset, err = os.ReadFile(filepath.Join(root, "rules.arclint.yaml"))
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(ruleset), "  - pattern: arclint/domain-model@0.1.0\n") ||
		!strings.Contains(string(ruleset), "  - pattern: arclint/vertical@0.1.0\n    bind:\n      domain: \"internal/*/domain/**\"\n") {
		t.Errorf("ruleset after the second install:\n%s", ruleset)
	}

	stdout, stderr, code = runBin(t, root, os.Environ(), "check", ".")
	if code != 0 {
		t.Fatalf("check after installs: exit %d\nstderr: %s\n%s", code, stderr, stdout)
	}
	if !strings.Contains(stdout, "19 rule(s) applied") {
		t.Errorf("both patterns' rules must apply:\n%s", stdout)
	}

	stdout, stderr, code = runBin(t, root, os.Environ(), "patterns", "install", "vertical")
	if code != 0 || strings.Contains(stdout, "moving the entry") {
		t.Errorf("installing the extended version again changes nothing: exit %d\n%s%s", code, stdout, stderr)
	}
	_, stderr, code = runBin(t, root, os.Environ(), "patterns", "install")
	if code != 2 || !strings.Contains(stderr, "a pattern is required") {
		t.Errorf("install without a selection: exit %d\nstderr: %s", code, stderr)
	}
}

func TestPatternsInstallFoldsDeclaredModules(t *testing.T) {
	root := t.TempDir()
	write(t, root, "rules.arclint.yaml", `runtime: [go]

modules:
  # The domain model.
  domain: "pkg/domain/**"
  web: "cmd/web/**"

rules:
  web/no-domain:
    description: "Web never imports the domain."
    on: web
    imports:
      internal: []
`)
	stdout, stderr, code := runBin(t, root, os.Environ(), "patterns", "install", "vertical")
	if code != 0 {
		t.Fatalf("install: exit %d\nstderr: %s", code, stderr)
	}
	if !strings.Contains(stdout, "adopted declared module(s): domain") || !strings.Contains(stdout, "  domain: pkg/domain/**") {
		t.Errorf("install must report the adopted declaration and its paths:\n%s", stdout)
	}
	ruleset, err := os.ReadFile(filepath.Join(root, "rules.arclint.yaml"))
	if err != nil {
		t.Fatal(err)
	}
	if strings.Contains(string(ruleset), "  # The domain model.\n") || strings.Contains(string(ruleset), "\n  domain: \"pkg/domain/**\"\n") {
		t.Errorf("the declared module must be folded into the binding:\n%s", ruleset)
	}
	if !strings.Contains(string(ruleset), "      domain: \"pkg/domain/**\"\n") || !strings.Contains(string(ruleset), "modules:\n  web: \"cmd/web/**\"\n") {
		t.Errorf("ruleset after folding:\n%s", ruleset)
	}
	if _, stderr, code := runBin(t, root, os.Environ(), "check", "."); code != 0 {
		t.Errorf("check after folding: exit %d\nstderr: %s", code, stderr)
	}
}

func TestPatternsVendorExportAndInstallFromRegistry(t *testing.T) {
	author := authorAcmePattern(t)
	registry := t.TempDir()
	stdout, stderr, code := runBin(t, author, os.Environ(), "patterns", "export", "layers", "--dir", registry)
	if code != 0 {
		t.Fatalf("export: exit %d\nstderr: %s", code, stderr)
	}
	if !strings.Contains(stdout, "published acme/layers@1.0.0") || !strings.Contains(stdout, "updated "+filepath.Join(registry, "index.json")) {
		t.Errorf("export output:\n%s", stdout)
	}
	if _, _, code := runBin(t, author, os.Environ(), "patterns", "export", "vertical", "--dir", registry); code != 0 {
		t.Fatalf("export vertical: exit %d", code)
	}
	for _, path := range []string{
		filepath.Join("acme", "layers", "1.0.0", "pattern.yaml"),
		filepath.Join("acme", "layers", "1.0.0", "manifest.json"),
		filepath.Join("arclint", "vertical", "0.1.0", "extensions", "vertical_usecase.ts"),
		"index.json",
	} {
		if _, err := os.Stat(filepath.Join(registry, path)); err != nil {
			t.Errorf("exported tree misses %s: %v", path, err)
		}
	}
	_, stderr, code = runBin(t, author, os.Environ(), "patterns", "export", "layers")
	if code != 2 || !strings.Contains(stderr, "--dir") {
		t.Errorf("export without --dir: exit %d\nstderr: %s", code, stderr)
	}

	location := "file://" + filepath.ToSlash(registry)
	consumer := t.TempDir()
	stdout, stderr, code = runBin(t, consumer, os.Environ(), "patterns", "--remote", "--registry", location)
	if code != 0 {
		t.Fatalf("patterns --remote: exit %d\nstderr: %s", code, stderr)
	}
	if !strings.Contains(stdout, "registry "+location) || !strings.Contains(stdout, "acme/layers@1.0.0") || !strings.Contains(stdout, "arclint/vertical@0.1.0") {
		t.Errorf("remote listing:\n%s", stdout)
	}

	env := append(os.Environ(), "ARCLINT_REGISTRY="+location)
	stdout, stderr, code = runBin(t, consumer, env, "patterns", "install", "acme/layers")
	if code != 0 {
		t.Fatalf("install from registry: exit %d\nstderr: %s", code, stderr)
	}
	for _, want := range []string{
		"installed acme/layers@1.0.0 (registry, ",
		"vendored to " + filepath.Join(consumer, ".arclint", "patterns", "acme", "layers"),
		"  app: internal/app/**",
		"unbound (bind each under extends[].bind before the ruleset loads):\n  domain\n",
	} {
		if !strings.Contains(stdout, want) {
			t.Errorf("install output misses %q:\n%s", want, stdout)
		}
	}
	vendoredDir := filepath.Join(consumer, ".arclint", "patterns", "acme", "layers")
	if _, err := os.Stat(filepath.Join(vendoredDir, "manifest.json")); err != nil {
		t.Errorf("manifest.json not vendored: %v", err)
	}
	// The install left one Module unbound, so the ruleset says so until
	// the owner binds it.
	_, stderr, code = runBin(t, consumer, os.Environ(), "check", ".")
	if code != 2 || !strings.Contains(stderr, "unbound modules domain") {
		t.Errorf("check with an unbound module: exit %d\nstderr: %s", code, stderr)
	}
	ruleset, err := os.ReadFile(filepath.Join(consumer, "rules.arclint.yaml"))
	if err != nil {
		t.Fatal(err)
	}
	bound := strings.Replace(string(ruleset), "# domain: <glob>", "domain: \"internal/domain/**\"", 1)
	write(t, consumer, "rules.arclint.yaml", bound)
	write(t, consumer, "internal/domain/model.go", "package domain\n\ntype Order struct{ ID string }\n")
	write(t, consumer, "internal/app/service.go", "package app\n\nimport \"fmt\"\n\nfunc Run() { fmt.Println(\"ok\") }\n")
	stdout, stderr, code = runBin(t, consumer, os.Environ(), "check", ".")
	if code != 0 {
		t.Fatalf("check after binding: exit %d\nstderr: %s\n%s", code, stderr, stdout)
	}
	if !strings.Contains(stdout, "1 rule(s) applied") {
		t.Errorf("the vendored pattern's rule must apply:\n%s", stdout)
	}

	// The vendored copy resolves offline: no registry is needed again,
	// and vendoring it again writes nothing.
	stdout, stderr, code = runBin(t, consumer, os.Environ(), "patterns", "vendor", "acme/layers@1.0.0")
	if code != 0 || !strings.Contains(stdout, "already vendored") {
		t.Errorf("vendor of a vendored pattern: exit %d\n%s%s", code, stdout, stderr)
	}
	stdout, _, _ = runBin(t, consumer, os.Environ(), "patterns")
	if !strings.Contains(stdout, "acme/layers@1.0.0") || !strings.Contains(stdout, "vendored") {
		t.Errorf("listing after vendoring:\n%s", stdout)
	}

	// An edited vendored file is refused with guidance on every load.
	patternPath := filepath.Join(vendoredDir, "pattern.yaml")
	data, err := os.ReadFile(patternPath)
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(patternPath, append(data, []byte("# edited\n")...), 0o644); err != nil {
		t.Fatal(err)
	}
	_, stderr, code = runBin(t, consumer, os.Environ(), "check", ".")
	if code != 2 || !strings.Contains(stderr, "has digest") || !strings.Contains(stderr, "patterns vendor") {
		t.Errorf("check with a tampered vendored file: exit %d\nstderr: %s", code, stderr)
	}
	stdout, stderr, code = runBin(t, consumer, env, "patterns", "vendor", "acme/layers@1.0.0")
	if code != 2 {
		t.Errorf("vendoring over a tampered copy must fail loudly, not repair it silently: exit %d\n%s%s", code, stdout, stderr)
	}
	if err := os.Remove(filepath.Join(vendoredDir, "manifest.json")); err != nil {
		t.Fatal(err)
	}
	stdout, _, _ = runBin(t, consumer, os.Environ(), "patterns")
	if !strings.Contains(stdout, "authored") {
		t.Errorf("without manifest.json the copy is authored:\n%s", stdout)
	}
}

func TestPatternsVendorEmbeddedAndFailures(t *testing.T) {
	root := t.TempDir()
	stdout, stderr, code := runBin(t, root, os.Environ(), "patterns", "vendor", "vertical")
	if code != 0 {
		t.Fatalf("vendor: exit %d\nstderr: %s", code, stderr)
	}
	vendoredDir := filepath.Join(root, ".arclint", "patterns", "arclint", "vertical")
	if !strings.Contains(stdout, "vendored arclint/vertical@0.1.0 (embedded, ") || !strings.Contains(stdout, vendoredDir) {
		t.Errorf("vendor output:\n%s", stdout)
	}
	for _, path := range []string{"pattern.yaml", "manifest.json", filepath.Join("extensions", "vertical_usecase.ts"), filepath.Join("extensions", "package.json")} {
		if _, err := os.Stat(filepath.Join(vendoredDir, path)); err != nil {
			t.Errorf("vendored copy misses %s: %v", path, err)
		}
	}
	stdout, _, _ = runBin(t, root, os.Environ(), "patterns")
	if !strings.Contains(stdout, "embedded, vendored") {
		t.Errorf("listing must show the embedded pattern is also vendored:\n%s", stdout)
	}

	_, stderr, code = runBin(t, root, os.Environ(), "patterns", "vendor", "nothing", "--registry", "file:///nowhere/registry")
	if code != 2 || !strings.Contains(stderr, "not embedded") {
		t.Errorf("unknown pattern with an unreachable registry: exit %d\nstderr: %s", code, stderr)
	}
	_, stderr, code = runBin(t, root, os.Environ(), "patterns", "vendor", "arclint/vertical@9.9.9", "--registry", "")
	if code != 2 || !strings.Contains(stderr, "not under .arclint/patterns") {
		t.Errorf("offline miss: exit %d\nstderr: %s", code, stderr)
	}
}

func TestCompletionListsPatternSelections(t *testing.T) {
	root := authorAcmePattern(t)
	for _, sub := range []string{"install", "vendor", "export"} {
		stdout, stderr, code := runBin(t, root, os.Environ(), "__complete", "patterns", sub, "")
		if code != 0 {
			t.Fatalf("__complete patterns %s: exit %d\nstderr: %s", sub, code, stderr)
		}
		for _, want := range []string{"arclint/vertical@0.1.0", "acme/layers@1.0.0"} {
			if !strings.Contains(stdout, want) {
				t.Errorf("patterns %s completion misses %q\n%s", sub, want, stdout)
			}
		}
	}
	stdout, _, _ := runBin(t, root, os.Environ(), "__complete", "patterns", "install", "vertical", "")
	if strings.Contains(stdout, "arclint/vertical@0.1.0") {
		t.Errorf("a second positional must not complete a pattern:\n%s", stdout)
	}
}
