package yaml_test

import (
	"strings"
	"testing"
	"testing/fstest"

	"github.com/wixregiga/arclint/internal/domain/rule"
	patternyaml "github.com/wixregiga/arclint/internal/infrastructure/pattern/yaml"
)

const completeManifest = `pattern:
  namespace: acme
  name: complete
  version: 1.2.3
coverage: [go, typescript, python]
modules:
  - name: app
    description: Application actions.
    paths: ["internal/app/**"]
  - name: domain
    paths: ["internal/domain/**"]
rules:
  - id: acme:complete/domain/stdlib-only
    kind: consumes
    module: domain
    allow: []
    forbid: [external]
  - id: acme:complete/domain/root-present
    kind: structure
    module: domain
    require: ["internal/domain/root.go"]
  - id: acme:complete/domain/snake-case
    kind: naming
    module: domain
    files: "**/*.go"
    case: snake_case
  - id: acme:complete/dependencies/inward
    kind: layers
    layers: [app, domain]
  - id: acme:complete/domain/protected
    kind: protected
    module: domain
    allow: [app]
  - id: acme:complete/features/independent
    kind: independence
    folders: ["internal/features/*"]
  - id: acme:complete/dependencies/acyclic
    kind: acyclic
    modules: [app, domain]
  - id: acme:complete/domain/custom
    kind: extension
    module: domain
    files: "**/*.go"
    uses: forbid-imports
    with:
      prefix: forbidden/
extensions:
  - name: forbid-imports
    entry: extensions/forbid_imports.ts
tests:
  root: tests
`

const ruleTest = `rule: acme:complete/domain/stdlib-only
files:
  internal/domain/root.go: |
    package domain
expect: []
`

func completeFS(manifest string) fstest.MapFS {
	return fstest.MapFS{
		"pattern.yaml":                  {Data: []byte(manifest)},
		"extensions/forbid_imports.ts":  {Data: []byte("export default {};\n")},
		"tests/domain_stdlib_only.yaml": {Data: []byte(ruleTest)},
		"notes/authoring.txt":           {Data: []byte("preserved byte-for-byte\n")},
	}
}

func TestLoadCompletePatternCoversEveryRuleKind(t *testing.T) {
	fileSystem := completeFS(completeManifest)
	p, err := patternyaml.Load(fileSystem, ".")
	if err != nil {
		t.Fatalf("Load: %v", err)
	}
	if got := p.Reference().String(); got != "acme/complete@1.2.3" {
		t.Errorf("Reference = %q", got)
	}
	if len(p.Modules()) != 2 || len(p.Rules()) != 8 || len(p.Extensions()) != 1 || len(p.Tests()) != 1 {
		t.Fatalf("loaded counts: modules=%d rules=%d extensions=%d tests=%d",
			len(p.Modules()), len(p.Rules()), len(p.Extensions()), len(p.Tests()))
	}
	wantKinds := []rule.Type{
		rule.TypeConsumes, rule.TypeStructure, rule.TypeNaming, rule.TypeLayers,
		rule.TypeProtected, rule.TypeIndependence, rule.TypeAcyclic, rule.TypeExtension,
	}
	for i, carried := range p.Rules() {
		if carried.Type() != wantKinds[i] {
			t.Errorf("Rules[%d].Type = %q, want %q", i, carried.Type(), wantKinds[i])
		}
		origin, ok := carried.Provenance()
		if !ok || origin.String() != p.Reference().String() {
			t.Errorf("Rules[%d] origin = %v, %v", i, origin, ok)
		}
	}
	if got := p.Coverage(); len(got) != 3 || got[0] != rule.LanguageGo ||
		got[1] != rule.LanguageTypeScript || got[2] != rule.LanguagePython {
		t.Errorf("Coverage = %v", got)
	}
	extension := p.Extensions()[0]
	if extension.Name() != "forbid-imports" || extension.Entry() != "extensions/forbid_imports.ts" ||
		string(extension.Bytes()) != "export default {};\n" {
		t.Errorf("Extension = %q %q %q", extension.Name(), extension.Entry(), extension.Bytes())
	}
	if p.Tests()[0].Name() != "domain_stdlib_only" {
		t.Errorf("Test name = %q", p.Tests()[0].Name())
	}
	if len(p.Files()) != len(fileSystem) || p.Digest() == "" {
		t.Errorf("tree files=%d digest=%q", len(p.Files()), p.Digest())
	}
}

func TestLoadPreservesAndDigestsTheFullTree(t *testing.T) {
	firstFS := completeFS(completeManifest)
	first, err := patternyaml.Load(firstFS, ".")
	if err != nil {
		t.Fatalf("Load first: %v", err)
	}
	manifestBytes := first.Files()[2].Bytes()
	if string(manifestBytes) != completeManifest {
		// Files are lexical: extensions, notes, pattern.yaml, tests.
		t.Fatalf("manifest bytes were rewritten")
	}
	firstFS["pattern.yaml"].Data[0] = '#'
	if string(first.Files()[2].Bytes()) != completeManifest {
		t.Fatalf("Pattern retained mutable caller bytes")
	}

	secondFS := completeFS(completeManifest)
	secondFS["notes/authoring.txt"] = &fstest.MapFile{Data: []byte("different\n")}
	second, err := patternyaml.Load(secondFS, ".")
	if err != nil {
		t.Fatalf("Load second: %v", err)
	}
	if first.Digest() == second.Digest() {
		t.Fatalf("digest did not cover auxiliary full-tree bytes")
	}
}

func TestLoadAllowsOmittedCoverageAndTests(t *testing.T) {
	manifest := strings.Replace(completeManifest, "coverage: [go, typescript, python]\n", "", 1)
	manifest = strings.Replace(manifest, "tests:\n  root: tests\n", "", 1)
	fileSystem := completeFS(manifest)
	delete(fileSystem, "tests/domain_stdlib_only.yaml")
	p, err := patternyaml.Load(fileSystem, ".")
	if err != nil {
		t.Fatalf("Load: %v", err)
	}
	if len(p.Coverage()) != 0 || len(p.Tests()) != 0 {
		t.Fatalf("coverage=%v tests=%v, want both optional", p.Coverage(), p.Tests())
	}
}

func TestLoadRejectsEveryManifestContractClass(t *testing.T) {
	tests := []struct {
		name     string
		manifest string
		mutate   func(fstest.MapFS)
		want     []string
	}{
		{
			name: "inexact identity", manifest: strings.Replace(completeManifest, "version: 1.2.3", "version: latest", 1),
			want: []string{"pattern.yaml: pattern.version", "not exact semver"},
		},
		{
			name:     "duplicate modules",
			manifest: strings.Replace(completeManifest, "rules:\n", "  - name: domain\n    paths: [\"other/**\"]\nrules:\n", 1),
			want:     []string{"modules[2].name", "duplicate module"},
		},
		{
			name:     "duplicate Rule IDs",
			manifest: strings.Replace(completeManifest, "acme:complete/domain/root-present", "acme:complete/domain/stdlib-only", 1),
			want:     []string{"rules[1].id", "duplicate rule id"},
		},
		{
			name: "unsupported coverage", manifest: strings.Replace(completeManifest, "coverage: [go, typescript, python]", "coverage: [rust]", 1),
			want: []string{"coverage[0]", "language \"rust\""},
		},
		{
			name: "missing referenced Extension", manifest: completeManifest,
			mutate: func(files fstest.MapFS) { delete(files, "extensions/forbid_imports.ts") },
			want:   []string{"extensions[0].entry", "forbid_imports.ts"},
		},
		{
			name:     "Rule references undeclared Extension",
			manifest: strings.Replace(completeManifest, "uses: forbid-imports", "uses: absent-extension", 1),
			want:     []string{"rules[7].uses", "Extension \"absent-extension\" is not declared"},
		},
		{
			name: "missing declared tests", manifest: completeManifest,
			mutate: func(files fstest.MapFS) { delete(files, "tests/domain_stdlib_only.yaml") },
			want:   []string{"tests.root", "contains no Rule Test"},
		},
		{
			name: "invalid declared test", manifest: completeManifest,
			mutate: func(files fstest.MapFS) {
				files["tests/domain_stdlib_only.yaml"] = &fstest.MapFile{Data: []byte("unknown: true\n")}
			},
			want: []string{"tests.root/domain_stdlib_only.yaml", "field unknown"},
		},
		{
			name: "declared test root is not a directory", manifest: completeManifest,
			mutate: func(files fstest.MapFS) {
				delete(files, "tests/domain_stdlib_only.yaml")
				files["tests"] = &fstest.MapFile{Data: []byte("not a directory\n")}
			},
			want: []string{"pattern.yaml: tests.root", "rule tests"},
		},
		{
			name: "unknown key", manifest: strings.Replace(completeManifest, "coverage:", "unexpected: true\ncoverage:", 1),
			want: []string{"pattern.yaml", "field unexpected"},
		},
		{
			name: "unsafe referenced path", manifest: strings.Replace(completeManifest, "extensions/forbid_imports.ts", "../forbid_imports.ts", 1),
			want: []string{"extensions[0].entry", "not a canonical relative path"},
		},
		{
			name:     "field from another Rule kind",
			manifest: strings.Replace(completeManifest, "    case: snake_case\n", "    case: snake_case\n    allow: []\n", 1),
			want:     []string{"rules[2]", "allow: field is not accepted by kind naming"},
		},
		{
			name:     "undeclared module",
			manifest: strings.Replace(completeManifest, "    module: domain\n    allow: []", "    module: missing\n    allow: []", 1),
			want:     []string{"rules[0]", "module \"missing\" is not declared"},
		},
		{
			name: "multiple documents", manifest: completeManifest + "---\n{}\n",
			want: []string{"pattern.yaml", "multiple YAML documents"},
		},
		{
			name:     "identity whitespace",
			manifest: strings.Replace(completeManifest, "name: complete", "name: complete pattern", 1),
			want:     []string{"pattern.name", "must not contain whitespace"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			fileSystem := completeFS(tt.manifest)
			if tt.mutate != nil {
				tt.mutate(fileSystem)
			}
			_, err := patternyaml.Load(fileSystem, ".")
			if err == nil {
				t.Fatal("Load succeeded, want rejection")
			}
			for _, want := range tt.want {
				if !strings.Contains(err.Error(), want) {
					t.Errorf("err = %v, want substring %q", err, want)
				}
			}
		})
	}
}

func TestLoadAcceptsNestedPackageRoot(t *testing.T) {
	fileSystem := fstest.MapFS{}
	for name, file := range completeFS(completeManifest) {
		fileSystem["catalog/acme/complete/"+name] = file
	}
	p, err := patternyaml.Load(fileSystem, "catalog/acme/complete")
	if err != nil {
		t.Fatalf("Load: %v", err)
	}
	if p.Reference().String() != "acme/complete@1.2.3" {
		t.Errorf("Reference = %s", p.Reference())
	}
}
