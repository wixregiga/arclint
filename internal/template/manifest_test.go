package template

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func writeManifest(t *testing.T, yaml string) string {
	t.Helper()
	path := filepath.Join(t.TempDir(), "template.yaml")
	if err := os.WriteFile(path, []byte(yaml), 0o644); err != nil {
		t.Fatal(err)
	}
	return path
}

const validManifest = `
version: 2
description: "A test thing"
destination: "things/{{ name | kebab }}"
variables:
  - name: name
    description: "Thing name"
    type: string
    validate: "^[a-zA-Z][a-zA-Z0-9 _-]*$"
  - name: flavor
    description: "Which flavor?"
    type: choice
    choices: [basic, fancy]
    default: basic
  - name: with_docs
    description: "Docs stub?"
    type: bool
    default: true
  - name: docs_title
    description: "Docs title"
    type: string
    default: "{{ name | pascal }} Docs"
    when: "with_docs == true"
`

func TestLoadManifestValid(t *testing.T) {
	m, err := LoadManifest(writeManifest(t, validManifest))
	if err != nil {
		t.Fatalf("valid manifest rejected: %v", err)
	}
	if m.Version != 2 || len(m.Variables) != 4 {
		t.Fatalf("bad parse: version=%d vars=%d", m.Version, len(m.Variables))
	}
	if !m.Variables[0].Required() {
		t.Error("name has no default, must be required")
	}
	if m.Variables[1].Required() {
		t.Error("flavor has a default, must not be required")
	}
	if d, ok := m.Variables[2].DefaultString(); !ok || d != "true" {
		t.Errorf("bool default = %q, %v", d, ok)
	}
}

func TestLoadManifestErrors(t *testing.T) {
	cases := []struct {
		name    string
		yaml    string
		errPart string
	}{
		{"missing version", "destination: x\nvariables: [{name: a, description: d, type: string}]", "version must be a positive integer"},
		{"missing destination", "version: 1\nvariables: [{name: a, description: d, type: string}]", "destination is required"},
		{"no variables", "version: 1\ndestination: x", "variables list is required"},
		{"bad var name", "version: 1\ndestination: x\nvariables: [{name: BadName, description: d, type: string}]", "name must match"},
		{"dup name", "version: 1\ndestination: x\nvariables: [{name: a, description: d, type: string}, {name: a, description: d, type: string}]", "duplicate name"},
		{"missing description", "version: 1\ndestination: x\nvariables: [{name: a, type: string}]", "description is required"},
		{"bad type", "version: 1\ndestination: x\nvariables: [{name: a, description: d, type: number}]", "unknown type"},
		{"choices on string", "version: 1\ndestination: x\nvariables: [{name: a, description: d, type: string, choices: [x]}]", "choices is only valid"},
		{"choice without choices", "version: 1\ndestination: x\nvariables: [{name: a, description: d, type: choice}]", "requires a choices list"},
		{"default not in choices", "version: 1\ndestination: x\nvariables: [{name: a, description: d, type: choice, choices: [x, y], default: z}]", "not in choices"},
		{"validate on choice", "version: 1\ndestination: x\nvariables: [{name: a, description: d, type: choice, choices: [x], validate: y}]", "validate is only valid"},
		{"bad regex", `version: 1
destination: x
variables: [{name: a, description: d, type: string, validate: "["}]`, "invalid validate pattern"},
		{"bad bool default", "version: 1\ndestination: x\nvariables: [{name: a, description: d, type: bool, default: maybe}]", "is not a bool"},
		{"unknown field", "version: 1\ndestination: x\nsurprise: 1\nvariables: [{name: a, description: d, type: string}]", "unknown field"},
		{"when forward ref", `version: 1
destination: x
variables:
  - {name: a, description: d, type: string, when: "b == true"}
  - {name: b, description: d, type: bool, default: false}`, "not declared earlier"},
		{"when self ref", `version: 1
destination: x
variables:
  - {name: a, description: d, type: string, when: "a == x"}`, "not declared earlier"},
		{"when bad grammar", `version: 1
destination: x
variables:
  - {name: a, description: d, type: bool, default: false}
  - {name: b, description: d, type: string, when: "a > 1"}`, "invalid when clause"},
		{"when or unsupported", `version: 1
destination: x
variables:
  - {name: a, description: d, type: bool, default: false}
  - {name: b, description: d, type: string, when: "a == true || a == false"}`, "invalid when clause"},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			_, err := LoadManifest(writeManifest(t, c.yaml))
			if err == nil {
				t.Fatalf("want error containing %q, got nil", c.errPart)
			}
			if !strings.Contains(err.Error(), c.errPart) {
				t.Fatalf("error = %q, want substring %q", err, c.errPart)
			}
		})
	}
}

func TestWhenGrammar(t *testing.T) {
	clauses, err := parseWhen(`transport == grpc && with_db != true && title == "Big Idea" && s == 'x'`)
	if err != nil {
		t.Fatal(err)
	}
	want := []whenClause{
		{"transport", "==", "grpc"},
		{"with_db", "!=", "true"},
		{"title", "==", "Big Idea"},
		{"s", "==", "x"},
	}
	if len(clauses) != len(want) {
		t.Fatalf("got %d clauses, want %d", len(clauses), len(want))
	}
	for i, c := range clauses {
		if c != want[i] {
			t.Errorf("clause %d = %+v, want %+v", i, c, want[i])
		}
	}

	if !evalWhen(want[:2], map[string]string{"transport": "grpc", "with_db": "false"}) {
		t.Error("want true")
	}
	if evalWhen(want[:1], map[string]string{"transport": "http"}) {
		t.Error("want false")
	}
	// Unresolved/skipped variables compare as empty string.
	if evalWhen([]whenClause{{"x", "==", "y"}}, map[string]string{}) {
		t.Error("missing var must not equal a literal")
	}
}

func TestResolvePriorityAndWhen(t *testing.T) {
	m, err := LoadManifest(writeManifest(t, validManifest))
	if err != nil {
		t.Fatal(err)
	}
	builtins := map[string]string{"repo_name": "repo"}

	// Flags beat saved answers beat defaults.
	res, err := m.Resolve(ResolveInput{
		Flags:    map[string]string{"name": "pay gw", "flavor": "fancy"},
		Saved:    map[string]string{"flavor": "basic", "with_docs": "false"},
		Builtins: builtins,
	})
	if err != nil {
		t.Fatal(err)
	}
	if res.Values["flavor"] != "fancy" {
		t.Errorf("flag must beat saved answer, got %q", res.Values["flavor"])
	}
	if res.Values["with_docs"] != "false" {
		t.Errorf("saved answer must beat default, got %q", res.Values["with_docs"])
	}
	if !res.Skipped["docs_title"] {
		t.Error("docs_title must be skipped when with_docs == false")
	}
	if _, ok := res.Values["docs_title"]; ok {
		t.Error("skipped variable must be absent from Values")
	}
	if res.RenderVars(builtins)["docs_title"] != "" {
		t.Error("skipped variable must render as empty string")
	}

	// Interpolated default referencing an earlier variable.
	res2, err := m.Resolve(ResolveInput{Flags: map[string]string{"name": "pay gw"}, Builtins: builtins})
	if err != nil {
		t.Fatal(err)
	}
	if res2.Values["docs_title"] != "PayGw Docs" {
		t.Errorf("interpolated default = %q, want %q", res2.Values["docs_title"], "PayGw Docs")
	}

	// Required with nothing supplied -> Missing.
	res3, err := m.Resolve(ResolveInput{Builtins: builtins})
	if err != nil {
		t.Fatal(err)
	}
	if len(res3.Missing) != 1 || res3.Missing[0].Name != "name" {
		t.Fatalf("Missing = %v", res3.Missing)
	}

	// Flag on a when-skipped variable is an error.
	_, err = m.Resolve(ResolveInput{
		Flags:    map[string]string{"name": "x", "with_docs": "false", "docs_title": "T"},
		Builtins: builtins,
	})
	if err == nil || !strings.Contains(err.Error(), "skipped by its when condition") {
		t.Errorf("want skipped-by-when flag error, got %v", err)
	}

	// Flag failing validation is an error.
	_, err = m.Resolve(ResolveInput{Flags: map[string]string{"name": "9bad"}, Builtins: builtins})
	if err == nil || !strings.Contains(err.Error(), "fails pattern") {
		t.Errorf("want validation error, got %v", err)
	}

	// Choice flag outside choices is an error.
	_, err = m.Resolve(ResolveInput{Flags: map[string]string{"name": "x", "flavor": "weird"}, Builtins: builtins})
	if err == nil || !strings.Contains(err.Error(), "not an allowed choice") {
		t.Errorf("want choice error, got %v", err)
	}
}
