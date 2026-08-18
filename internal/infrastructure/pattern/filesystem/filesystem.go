// Package filesystempattern supplies local Pattern distribution
// packages from a directory: each subdirectory holding a pattern.yaml
// — a target-format ruleset file with a pattern identity header — is
// one distributable Pattern, returned as a validated domain value.
package filesystempattern

import (
	"fmt"
	"os"
	"path/filepath"
	"sort"

	"github.com/wixregiga/arclint/internal/domain/rule"
	yamlrule "github.com/wixregiga/arclint/internal/infrastructure/rule/yaml"
)

// FileName is the Pattern distribution file inside each Pattern
// directory.
const FileName = "pattern.yaml"

// Source implements the application's PatternSource port over one
// directory of Pattern packages.
type Source struct {
	dir string
}

// NewSource binds the source to a directory; the directory may not
// exist yet, which simply means no local Patterns.
func NewSource(dir string) (Source, error) {
	abs, err := filepath.Abs(dir)
	if err != nil {
		return Source{}, fmt.Errorf("patterns dir: %w", err)
	}
	return Source{dir: abs}, nil
}

// Patterns loads every Pattern package under the directory, in
// deterministic reference order. An invalid Pattern file is an error,
// never a silently skipped entry.
func (s Source) Patterns() ([]rule.Pattern, error) {
	entries, err := os.ReadDir(s.dir)
	if os.IsNotExist(err) {
		return nil, nil
	}
	if err != nil {
		return nil, fmt.Errorf("patterns: %w", err)
	}
	var out []rule.Pattern
	for _, e := range entries {
		if !e.IsDir() {
			continue
		}
		file := filepath.Join(s.dir, e.Name(), FileName)
		data, err := os.ReadFile(file)
		if os.IsNotExist(err) {
			continue
		}
		if err != nil {
			return nil, fmt.Errorf("read %s: %w", file, err)
		}
		doc, err := yamlrule.Load(data, file)
		if err != nil {
			return nil, fmt.Errorf("load pattern: %w", err)
		}
		if doc.Pattern == nil {
			return nil, fmt.Errorf("%s: missing pattern identity header (namespace, name, version)", file)
		}
		p, err := rule.NewPattern(doc.Pattern.Namespace, doc.Pattern.Name, doc.Pattern.Version,
			doc.Configured.Rules, doc.Pattern.Coverage)
		if err != nil {
			return nil, fmt.Errorf("%s: %v", file, err)
		}
		out = append(out, p)
	}
	sort.Slice(out, func(i, j int) bool {
		return out[i].Reference().String() < out[j].Reference().String()
	})
	return out, nil
}
