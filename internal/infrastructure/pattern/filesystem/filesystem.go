// Package filesystempattern supplies local Pattern distribution
// packages from a directory. Each immediate subdirectory holding a
// pattern.yaml is one distributable Pattern tree.
package filesystempattern

import (
	"fmt"
	"os"
	"path/filepath"
	"sort"

	"github.com/wixregiga/arclint/internal/domain/pattern"
	patternyaml "github.com/wixregiga/arclint/internal/infrastructure/pattern/yaml"
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
func (s Source) Patterns() ([]pattern.Pattern, error) {
	entries, err := os.ReadDir(s.dir)
	if os.IsNotExist(err) {
		return nil, nil
	}
	if err != nil {
		return nil, fmt.Errorf("patterns: %w", err)
	}
	fileSystem := os.DirFS(s.dir)
	var out []pattern.Pattern
	for _, e := range entries {
		if !e.IsDir() {
			continue
		}
		if _, err := os.Stat(filepath.Join(s.dir, e.Name(), FileName)); os.IsNotExist(err) {
			continue
		} else if err != nil {
			return nil, fmt.Errorf("pattern package %s: %w", e.Name(), err)
		}
		p, err := patternyaml.Load(fileSystem, e.Name())
		if err != nil {
			return nil, fmt.Errorf("load pattern %s: %w", e.Name(), err)
		}
		out = append(out, p)
	}
	sort.Slice(out, func(i, j int) bool {
		return out[i].Reference().String() < out[j].Reference().String()
	})
	return out, nil
}
