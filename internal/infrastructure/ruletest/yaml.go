// Package ruletest adapts the Rule Test capability to infrastructure:
// a YAML source loading authored Rule Tests from the repository's
// .arclint/tests directory, and a fixture observer materializing test
// fixtures on the filesystem so they pass through the same observation
// pipeline production uses.
package ruletest

import (
	"bytes"
	"errors"
	"fmt"
	"io"
	"io/fs"
	"os"
	"path/filepath"
	"sort"
	"strings"

	"gopkg.in/yaml.v3"

	"github.com/wixregiga/arclint/internal/domain/rule"
)

// Source implements the application's RuleTestSource port over the
// repository's .arclint/tests directory: one YAML file per Rule Test,
// the file stem naming the test. A missing directory means zero
// tests, not an error.
type Source struct {
	dir string
}

// NewSource binds the source to the .arclint/tests directory under
// the repository root.
func NewSource(root string) Source {
	return Source{dir: filepath.Join(root, ".arclint", "tests")}
}

// testDoc is the strict YAML grammar of one Rule Test file.
type testDoc struct {
	Rule   string            `yaml:"rule"`
	Files  map[string]string `yaml:"files"`
	Expect []expectDoc       `yaml:"expect"`
}

// expectDoc is one expected finding; kind defaults to violation.
type expectDoc struct {
	Kind    string `yaml:"kind"`
	Path    string `yaml:"path"`
	Line    int    `yaml:"line"`
	Message string `yaml:"message"`
}

// Tests loads every .arclint/tests/*.yaml file strictly, in
// deterministic name order. Unknown keys, a missing rule, and a
// fixture-less test are loud errors; a representation that cannot
// become a valid Rule Test never loads partially.
func (s Source) Tests() ([]rule.Test, error) {
	entries, err := os.ReadDir(s.dir)
	if errors.Is(err, fs.ErrNotExist) {
		return nil, nil
	}
	if err != nil {
		return nil, fmt.Errorf("rule tests: %w", err)
	}
	names := make([]string, 0, len(entries))
	for _, e := range entries {
		if e.IsDir() || !strings.HasSuffix(e.Name(), ".yaml") {
			continue
		}
		names = append(names, e.Name())
	}
	sort.Strings(names)
	tests := make([]rule.Test, 0, len(names))
	for _, name := range names {
		t, err := s.load(filepath.Join(s.dir, name), strings.TrimSuffix(name, ".yaml"))
		if err != nil {
			return nil, err
		}
		tests = append(tests, t)
	}
	return tests, nil
}

// load parses one test file strictly and constructs the domain value.
func (s Source) load(path, name string) (rule.Test, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return rule.Test{}, fmt.Errorf("rule test: %w", err)
	}
	decoder := yaml.NewDecoder(bytes.NewReader(data))
	decoder.KnownFields(true)
	var doc testDoc
	if err := decoder.Decode(&doc); err != nil {
		if errors.Is(err, io.EOF) {
			return rule.Test{}, fmt.Errorf("%s: empty rule test file", path)
		}
		return rule.Test{}, fmt.Errorf("%s: %w", path, err)
	}
	if strings.TrimSpace(doc.Rule) == "" {
		return rule.Test{}, fmt.Errorf("%s: missing rule (every Rule Test identifies its Rule)", path)
	}
	if len(doc.Files) == 0 {
		return rule.Test{}, fmt.Errorf("%s: missing files (a Rule Test supplies its complete fixture)", path)
	}
	paths := make([]string, 0, len(doc.Files))
	for p := range doc.Files {
		paths = append(paths, p)
	}
	sort.Strings(paths)
	files := make([]rule.TestFile, 0, len(paths))
	for _, p := range paths {
		files = append(files, rule.TestFile{Path: p, Content: doc.Files[p]})
	}
	expect := make([]rule.ExpectedFinding, 0, len(doc.Expect))
	for _, e := range doc.Expect {
		expect = append(expect, rule.ExpectedFinding{
			Kind:    rule.FindingKind(e.Kind),
			Path:    e.Path,
			Line:    e.Line,
			Message: e.Message,
		})
	}
	t, err := rule.NewTest(name, doc.Rule, files, expect)
	if err != nil {
		return rule.Test{}, fmt.Errorf("%s: %w", path, err)
	}
	return t, nil
}
