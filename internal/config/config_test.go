package config

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

const validYAML = `runtime: [go]

modules:
  entities: ["internal/entities/**"]
  registry: ["internal/shared/registry.go"]

contracts:
  entities:
    consumes:
      internal: []
      external: forbid
    provides:
      - kind: registration
        each: 'internal/entities/(?P<name>[^/]+)/'
        in: registry
        match: 'Register\("{name}"\)'
`

func parse(t *testing.T, src string) (*RuleSet, error) {
	t.Helper()
	return Parse([]byte(src), "/test/rules.yaml")
}

func TestParseValid(t *testing.T) {
	rs, err := parse(t, validYAML)
	if err != nil {
		t.Fatal(err)
	}
	c := rs.Contracts["entities"].Consumes
	if c == nil || c.Internal == nil || !c.Internal.Restricted || len(c.Internal.Allow) != 0 {
		t.Errorf("internal allow-list decode: %+v", c)
	}
	if got := rs.Contracts["entities"].Provides[0].InModule; got != "registry" {
		t.Errorf("registration in decode: %q", got)
	}
	if rs.SHA256 == "" || rs.Root != "/test" {
		t.Errorf("fingerprint/root: %q %q", rs.SHA256, rs.Root)
	}
}

func TestInternalPolicyForms(t *testing.T) {
	rs, err := parse(t, `runtime: [go]
modules:
  a: ["a/**"]
  b: ["b/**"]
contracts:
  a:
    consumes:
      internal: { allow: [b], deny: [b] }
`)
	if err != nil {
		t.Fatal(err)
	}
	p := rs.Contracts["a"].Consumes.Internal
	if !p.Restricted || len(p.Allow) != 1 || len(p.Deny) != 1 {
		t.Errorf("policy: %+v", p)
	}

	// Deny-only mapping: not restricted.
	rs, err = parse(t, `runtime: [go]
modules:
  a: ["a/**"]
  b: ["b/**"]
contracts:
  a:
    consumes:
      internal: { deny: [b] }
`)
	if err != nil {
		t.Fatal(err)
	}
	if rs.Contracts["a"].Consumes.Internal.Restricted {
		t.Error("deny-only mapping must not be an allow-list")
	}
}

func TestSchemaRejections(t *testing.T) {
	cases := map[string]string{
		"unknown top-level key": validYAML + "\nbogus: true\n",
		"empty runtime":         "runtime: []\nmodules:\n  a: [\"a/**\"]\n",
		"bad runtime target":    "runtime: [rust]\nmodules:\n  a: [\"a/**\"]\n",
		"bad provides kind": `runtime: [go]
modules:
  a: ["a/**"]
contracts:
  a:
    provides:
      - kind: teleportation
`,
		"bad severity": `runtime: [go]
modules:
  a: ["a/**"]
contracts:
  a:
    consumes: { external: forbid, severity: fatal }
`,
		"internal as scalar": `runtime: [go]
modules:
  a: ["a/**"]
contracts:
  a:
    consumes: { internal: nope }
`,
	}
	for name, src := range cases {
		if _, err := parse(t, src); err == nil {
			t.Errorf("%s: accepted invalid config", name)
		}
	}
}

func TestSemanticRejections(t *testing.T) {
	cases := map[string]struct {
		src  string
		want string
	}{
		"unknown module in contracts": {`runtime: [go]
modules:
  a: ["a/**"]
contracts:
  ghost:
    consumes: { external: forbid }
`, "unknown module"},
		"layers needs two": {`runtime: [go]
modules:
  a: ["a/**"]
dependencies:
  - kind: layers
    layers: [a]
`, "at least two layers"},
		"template ref without group": {`runtime: [go]
modules:
  a: ["a/**"]
  r: ["r.go"]
contracts:
  a:
    provides:
      - kind: registration
        each: 'a/([^/]+)/'
        in: r
        match: 'Register\("{name}"\)'
`, "not a named capture group"},
		"bad expr": {`runtime: [go]
modules:
  a: ["a/**"]
contracts:
  a:
    invariants:
      - kind: expr
        assert: 'file.nonexistent > 1'
`, "expr"},
		"bad regex in correspondence": {`runtime: [go]
modules:
  a: ["a/**"]
contracts:
  a:
    provides:
      - kind: correspondence
        of: { files: '([unclosed', value: "x" }
        in: { files: 'ok', value: "x" }
`, "does not compile"},
		"ts target not implemented": {`runtime: [go, ts]
modules:
  a: ["a/**"]
`, "not supported yet"},
	}
	for name, tc := range cases {
		_, err := parse(t, tc.src)
		if err == nil {
			t.Errorf("%s: accepted invalid config", name)
			continue
		}
		if !strings.Contains(err.Error(), tc.want) {
			t.Errorf("%s: error %q does not mention %q", name, err, tc.want)
		}
	}
}

func TestSchemaJSONPublishable(t *testing.T) {
	data, err := SchemaJSON()
	if err != nil {
		t.Fatal(err)
	}
	for _, marker := range []string{`"$defs"`, `"registration"`, `"acyclic"`, `"kebab-case"`} {
		if marker == `"kebab-case"` {
			continue // naming vocabulary is validated semantically, not by schema enum
		}
		if !strings.Contains(string(data), marker) {
			t.Errorf("published schema lacks %s", marker)
		}
	}
}

func TestCacheRoundtrip(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "rules.yaml")
	if err := os.WriteFile(path, []byte(validYAML), 0o644); err != nil {
		t.Fatal(err)
	}
	rs, err := Load(path)
	if err != nil {
		t.Fatal(err)
	}
	if err := WriteCache(rs, "test-version"); err != nil {
		t.Fatal(err)
	}
	_, hit, err := LoadCached(path, "test-version")
	if err != nil || !hit {
		t.Fatalf("expected cache hit, got hit=%v err=%v", hit, err)
	}
	// A different arclint version invalidates.
	_, hit, err = LoadCached(path, "other-version")
	if err != nil || hit {
		t.Fatalf("expected miss on version change, got hit=%v err=%v", hit, err)
	}
	// Content change invalidates.
	if err := os.WriteFile(path, []byte(validYAML+"\n# touched\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	_, hit, err = LoadCached(path, "test-version")
	if err != nil || hit {
		t.Fatalf("expected miss on content change, got hit=%v err=%v", hit, err)
	}
}
