package assets

import (
	"io/fs"
	"strings"
	"testing"

	"github.com/goccy/go-yaml"
)

// manifest mirrors the CLI-relevant subset of template.yaml
// (docs/design/templating.md §2).
type manifest struct {
	Version     int    `yaml:"version"`
	Description string `yaml:"description"`
	Destination string `yaml:"destination"`
	Variables   []struct {
		Name        string `yaml:"name"`
		Description string `yaml:"description"`
		Type        string `yaml:"type"`
		Default     any    `yaml:"default"`
	} `yaml:"variables"`
}

func TestDefaultRulesIsValidYAML(t *testing.T) {
	data := DefaultRules()
	if len(data) == 0 {
		t.Fatal("DefaultRules returned empty content")
	}
	var doc map[string]any
	if err := yaml.Unmarshal(data, &doc); err != nil {
		t.Fatalf("default rules.yaml is not valid YAML: %v", err)
	}
	if doc["version"] != uint64(1) && doc["version"] != 1 {
		t.Errorf("version = %v, want 1", doc["version"])
	}
	rules, ok := doc["rules"].(map[string]any)
	if !ok || len(rules) == 0 {
		t.Fatalf("rules registry missing or empty: %T", doc["rules"])
	}
}

func TestBuiltinTemplateManifestsParse(t *testing.T) {
	tfs := Templates()
	for _, name := range []string{"repo", "service", "component"} {
		data, err := fs.ReadFile(tfs, name+"/template.yaml")
		if err != nil {
			t.Errorf("builtin template %q has no template.yaml: %v", name, err)
			continue
		}
		var m manifest
		if err := yaml.Unmarshal(data, &m); err != nil {
			t.Errorf("%s/template.yaml is not valid YAML: %v", name, err)
			continue
		}
		if m.Version < 1 {
			t.Errorf("%s: version = %d, want >= 1", name, m.Version)
		}
		if m.Description == "" {
			t.Errorf("%s: empty description", name)
		}
		if m.Destination == "" {
			t.Errorf("%s: empty destination", name)
		}
		// Exactly the contract the CLI depends on: a required (no-default)
		// variable called "name" bound to the positional argument.
		found := false
		for _, v := range m.Variables {
			if v.Name == "name" {
				found = true
				if v.Default != nil {
					t.Errorf("%s: variable \"name\" must be required (no default), got default %v", name, v.Default)
				}
				if v.Type != "string" {
					t.Errorf("%s: variable \"name\" type = %q, want string", name, v.Type)
				}
			}
			if v.Description == "" {
				t.Errorf("%s: variable %q has no description", name, v.Name)
			}
		}
		if !found {
			t.Errorf("%s: no required \"name\" variable", name)
		}
	}
}

// TestEmbeddedPathsAreEncoded pins the embed-safety invariants: payload
// files carry .tmpl, and no raw interpolation syntax appears in embedded
// paths (Go's embed rejects '|' and friends).
func TestEmbeddedPathsAreEncoded(t *testing.T) {
	tfs := Templates()
	err := fs.WalkDir(tfs, ".", func(p string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if strings.ContainsAny(p, "{}|") {
			t.Errorf("embedded path contains raw interpolation characters: %s", p)
		}
		if d.IsDir() || d.Name() == "template.yaml" {
			return nil
		}
		if inFiles := strings.Contains(p, "/files/"); inFiles && !strings.HasSuffix(p, ".tmpl") {
			t.Errorf("payload file missing .tmpl suffix: %s", p)
		}
		return nil
	})
	if err != nil {
		t.Fatalf("walk: %v", err)
	}
}

func TestDecodePath(t *testing.T) {
	cases := map[string]string{
		"service/files/cmd/__name_kebab__/main.go.tmpl": "service/files/cmd/{{ name | kebab }}/main.go",
		"service/files/service.yaml.tmpl":               "service/files/service.yaml",
		"repo/template.yaml":                            "repo/template.yaml",
		"component/files":                               "component/files",
	}
	for in, want := range cases {
		if got := DecodePath(in); got != want {
			t.Errorf("DecodePath(%q) = %q, want %q", in, got, want)
		}
	}
}
