// Package ruletest runs rule test cases: small fixture trees with the
// complete expected violation set, materialized and checked in a fresh
// temp root. Cases prove that rules mean what their descriptions claim
// (M7 ADR) without hand-building throwaway repositories.
//
// A case file is a YAML document (multi-document streams hold several):
//
//	case: domain-imports-application
//	runtime: [go]          # optional; pattern targets render for it
//	files:
//	  domain/book.go: |
//	    package domain
//	    import _ "test.local/case/application"
//	  application/svc.go: |
//	    package application
//	expect:
//	  - rule: "ddd:ARCH-001"
//	    path: domain/book.go
//	    line: 2
//
// Matching is strict set equality: every expectation must match a
// violation and every violation must be matched, so a case with an empty
// expect list asserts a clean tree.
package ruletest

import (
	"bytes"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"sort"
	"strings"

	"gopkg.in/yaml.v3"

	"github.com/wixregiga/arclint/internal/config"
	"github.com/wixregiga/arclint/internal/engine"
	"github.com/wixregiga/arclint/internal/report"
)

// Case is one rule test.
type Case struct {
	Name string `yaml:"case"`
	// Runtime picks the targets a pattern template is rendered for
	// (ignored for repo targets, whose rules.yaml fixes the runtimes).
	Runtime []string          `yaml:"runtime"`
	Files   map[string]string `yaml:"files"`
	Expect  []Expect          `yaml:"expect"`

	// Source locates the case for reporting.
	Source string `yaml:"-"`
}

// Expect matches one violation. Rule and Path are required; every other
// field narrows the match. Message matches as a substring.
type Expect struct {
	Rule     string `yaml:"rule"`
	Path     string `yaml:"path"`
	Line     int    `yaml:"line"`
	Contract string `yaml:"contract"`
	Blame    string `yaml:"blame"`
	Severity string `yaml:"severity"`
	Message  string `yaml:"message"`
}

func (e Expect) String() string {
	parts := []string{"rule=" + e.Rule, "path=" + e.Path}
	if e.Line > 0 {
		parts = append(parts, fmt.Sprintf("line=%d", e.Line))
	}
	for _, p := range [][2]string{{"contract", e.Contract}, {"blame", e.Blame}, {"severity", e.Severity}, {"message", e.Message}} {
		if p[1] != "" {
			parts = append(parts, p[0]+"="+p[1])
		}
	}
	return strings.Join(parts, " ")
}

func (e Expect) matches(v report.Violation) bool {
	if e.Rule != v.RuleID || e.Path != v.Path {
		return false
	}
	if e.Line > 0 && v.LineValue() != e.Line {
		return false
	}
	if e.Contract != "" && string(v.Contract) != e.Contract {
		return false
	}
	if e.Blame != "" && string(v.Blame) != e.Blame {
		return false
	}
	if e.Severity != "" && string(v.Severity) != e.Severity {
		return false
	}
	if e.Message != "" && !strings.Contains(v.Message, e.Message) {
		return false
	}
	return true
}

// Target supplies the ruleset a case runs against.
type Target struct {
	// RulesFor renders rules.yaml for the case's runtimes (nil = the
	// target's default).
	RulesFor func(runtimes []string) ([]byte, error)
	// Extensions maps repo-relative paths under .arclint/extensions/ to
	// contents.
	Extensions map[string][]byte
}

// RepoTarget targets a repository's own rules.yaml and extensions.
func RepoTarget(root string) (Target, error) {
	rules, err := os.ReadFile(filepath.Join(root, "rules.yaml"))
	if err != nil {
		return Target{}, fmt.Errorf("rule tests need a rules.yaml at the repo root: %w", err)
	}
	exts := map[string][]byte{}
	extDir := filepath.Join(root, ".arclint", "extensions")
	_ = filepath.WalkDir(extDir, func(p string, d os.DirEntry, err error) error {
		if err != nil || d.IsDir() {
			return nil
		}
		rel, relErr := filepath.Rel(extDir, p)
		if relErr != nil {
			return nil
		}
		data, readErr := os.ReadFile(p)
		if readErr != nil {
			return nil
		}
		exts[filepath.ToSlash(rel)] = data
		return nil
	})
	return Target{
		RulesFor:   func([]string) ([]byte, error) { return rules, nil },
		Extensions: exts,
	}, nil
}

// Load parses one case file (a YAML stream of one or more cases).
func Load(path string) ([]Case, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}
	return Parse(data, path)
}

// Parse decodes a YAML stream of cases; source labels them in reports.
func Parse(data []byte, source string) ([]Case, error) {
	path := source
	dec := yaml.NewDecoder(bytes.NewReader(data))
	dec.KnownFields(true)
	var cases []Case
	for i := 0; ; i++ {
		var c Case
		if err := dec.Decode(&c); err != nil {
			if errors.Is(err, io.EOF) {
				break
			}
			return nil, fmt.Errorf("%s: document %d: %w", path, i+1, err)
		}
		if c.Name == "" {
			return nil, fmt.Errorf("%s: document %d: `case:` name is required", path, i+1)
		}
		if len(c.Files) == 0 {
			return nil, fmt.Errorf("%s: case %q: `files:` is required", path, c.Name)
		}
		for _, e := range c.Expect {
			if e.Rule == "" || e.Path == "" {
				return nil, fmt.Errorf("%s: case %q: every expectation needs rule and path", path, c.Name)
			}
		}
		c.Source = path
		cases = append(cases, c)
	}
	if len(cases) == 0 {
		return nil, fmt.Errorf("%s: no cases", path)
	}
	return cases, nil
}

// LoadDir loads every *.yaml/*.yml under dir, sorted for determinism.
func LoadDir(dir string) ([]Case, error) {
	var paths []string
	err := filepath.WalkDir(dir, func(p string, d os.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if !d.IsDir() && (strings.HasSuffix(p, ".yaml") || strings.HasSuffix(p, ".yml")) {
			paths = append(paths, p)
		}
		return nil
	})
	if err != nil {
		return nil, err
	}
	sort.Strings(paths)
	var cases []Case
	for _, p := range paths {
		cs, err := Load(p)
		if err != nil {
			return nil, err
		}
		cases = append(cases, cs...)
	}
	return cases, nil
}

// Result of one case run.
type Result struct {
	Case string `json:"case"`
	// Source is the case file the case came from.
	Source string `json:"source"`
	Pass   bool   `json:"pass"`
	// Err is a setup or load failure, "" otherwise.
	Err string `json:"error,omitempty"`
	// Missing lists expectations no violation matched.
	Missing []string `json:"missing,omitempty"`
	// Unexpected lists violations no expectation matched.
	Unexpected []report.Violation `json:"unexpected,omitempty"`
	// Rules lists the rule ids the case's expectations exercised.
	Rules []string `json:"rules,omitempty"`
}

// Run materializes one case into a fresh temp root and checks it.
func Run(c Case, target Target) Result {
	res := Result{Case: c.Name, Source: c.Source}
	for _, e := range c.Expect {
		if !contains(res.Rules, e.Rule) {
			res.Rules = append(res.Rules, e.Rule)
		}
	}
	sort.Strings(res.Rules)

	root, err := os.MkdirTemp("", "arclint-ruletest-*")
	if err != nil {
		res.Err = err.Error()
		return res
	}
	defer os.RemoveAll(root)

	write := func(rel string, data []byte) error {
		abs := filepath.Join(root, filepath.FromSlash(rel))
		if err := os.MkdirAll(filepath.Dir(abs), 0o755); err != nil {
			return err
		}
		return os.WriteFile(abs, data, 0o644)
	}

	rules, err := target.RulesFor(c.Runtime)
	if err != nil {
		res.Err = err.Error()
		return res
	}
	if _, override := c.Files["rules.yaml"]; !override {
		if err := write("rules.yaml", rules); err != nil {
			res.Err = err.Error()
			return res
		}
	}
	for rel, data := range target.Extensions {
		if err := write(".arclint/extensions/"+rel, data); err != nil {
			res.Err = err.Error()
			return res
		}
	}
	for rel, content := range c.Files {
		if err := write(rel, []byte(content)); err != nil {
			res.Err = err.Error()
			return res
		}
	}
	for _, m := range defaultManifests(c.Files) {
		if err := write(m.path, []byte(m.content)); err != nil {
			res.Err = err.Error()
			return res
		}
	}

	rs, err := config.Load(filepath.Join(root, "rules.yaml"))
	if err != nil {
		res.Err = err.Error()
		return res
	}
	checked, err := engine.Check(rs)
	if err != nil {
		res.Err = err.Error()
		return res
	}

	matched := make([]bool, len(checked.Violations))
	for _, e := range c.Expect {
		found := false
		for i, v := range checked.Violations {
			if !matched[i] && e.matches(v) {
				matched[i] = true
				found = true
				break
			}
		}
		if !found {
			res.Missing = append(res.Missing, e.String())
		}
	}
	for i, v := range checked.Violations {
		if !matched[i] {
			res.Unexpected = append(res.Unexpected, v)
		}
	}
	res.Pass = res.Err == "" && len(res.Missing) == 0 && len(res.Unexpected) == 0
	return res
}

type manifest struct{ path, content string }

// defaultManifests supplies the minimal per-language manifests a case
// needs for import classification, unless the case brings its own. The
// Go module path is fixed so case files can write resolvable internal
// imports.
func defaultManifests(files map[string]string) []manifest {
	hasExt := func(exts ...string) bool {
		for path := range files {
			for _, e := range exts {
				if strings.HasSuffix(path, e) {
					return true
				}
			}
		}
		return false
	}
	var out []manifest
	if _, ok := files["go.mod"]; !ok && hasExt(".go") {
		out = append(out, manifest{"go.mod", "module test.local/case\n\ngo 1.24\n"})
	}
	if _, ok := files["package.json"]; !ok && hasExt(".ts", ".tsx", ".js", ".jsx", ".mjs", ".cjs") {
		out = append(out, manifest{"package.json", "{\n  \"name\": \"ruletest-case\",\n  \"private\": true\n}\n"})
	}
	if _, ok := files["pyproject.toml"]; !ok && hasExt(".py") {
		out = append(out, manifest{"pyproject.toml", "[project]\nname = \"ruletest-case\"\nversion = \"0\"\n"})
	}
	return out
}

func contains(list []string, s string) bool {
	for _, v := range list {
		if v == s {
			return true
		}
	}
	return false
}
