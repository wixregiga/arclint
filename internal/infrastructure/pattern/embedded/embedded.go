// Package embeddedpattern supplies built-in Pattern packages embedded
// in the binary. Built-in Patterns differ from local ones only in
// where their exact distribution-tree bytes come from.
package embeddedpattern

import (
	"embed"
	"errors"
	"fmt"
	"io/fs"
	"path"
	"sort"

	"github.com/wixregiga/arclint/internal/application"
	"github.com/wixregiga/arclint/internal/domain/pattern"
	patternyaml "github.com/wixregiga/arclint/internal/infrastructure/pattern/yaml"
)

//go:embed vertical
var assets embed.FS

// Source implements the application's PatternSource port over the
// embedded Pattern packages.
type Source struct{}

// NewSource returns the built-in Pattern source.
func NewSource() Source { return Source{} }

// Names returns built-in Pattern names in deterministic order.
func (s Source) Names() []string {
	names, err := packageRoots()
	if err != nil {
		return nil
	}
	return names
}

func packageRoots() ([]string, error) {
	entries, err := fs.ReadDir(assets, ".")
	if err != nil {
		return nil, fmt.Errorf("embedded patterns: %w", err)
	}
	var names []string
	for _, e := range entries {
		if !e.IsDir() {
			continue
		}
		if _, err := fs.Stat(assets, path.Join(e.Name(), "pattern.yaml")); err == nil {
			names = append(names, e.Name())
		} else if !errors.Is(err, fs.ErrNotExist) {
			return nil, fmt.Errorf("embedded pattern %s: %w", e.Name(), err)
		}
	}
	sort.Strings(names)
	return names, nil
}

// Scaffold returns one built-in Pattern bundle for init materialization.
func (s Source) Scaffold(name string) (application.PatternScaffold, bool) {
	p, err := patternyaml.Load(assets, name)
	if err != nil {
		return application.PatternScaffold{}, false
	}
	ruleset, err := assets.ReadFile(path.Join(name, "rules.yaml"))
	if err != nil {
		return application.PatternScaffold{}, false
	}
	return application.PatternScaffold{Ruleset: string(ruleset), Extensions: p.Extensions()}, true
}

// Patterns loads every built-in Pattern package in deterministic
// reference order through the same manifest loader used by local packages.
func (s Source) Patterns() ([]pattern.Pattern, error) {
	names, err := packageRoots()
	if err != nil {
		return nil, err
	}
	out := make([]pattern.Pattern, 0, len(names))
	for _, name := range names {
		p, err := patternyaml.Load(assets, name)
		if err != nil {
			return nil, fmt.Errorf("load embedded pattern %s: %w", name, err)
		}
		out = append(out, p)
	}
	sort.Slice(out, func(i, j int) bool {
		return out[i].Reference().String() < out[j].Reference().String()
	})
	return out, nil
}
