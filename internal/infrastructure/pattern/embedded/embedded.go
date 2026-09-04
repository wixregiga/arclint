// Package embeddedpattern supplies the built-in Pattern packages
// embedded in the binary. A built-in Pattern is an ordinary Pattern
// distribution directory (pattern.yaml plus an extensions directory)
// whose bytes ship with arclint; the only extra check is that its
// header claims the arclint namespace, so a shipped file cannot
// impersonate a third-party Pattern. Because the bytes are the ones
// published, an embedded Pattern carries the same Digest as its
// Registry copy.
package embeddedpattern

import (
	"embed"
	"fmt"
	"io/fs"
	"sort"

	"github.com/wixregiga/arclint/internal/domain/distribution"
	"github.com/wixregiga/arclint/internal/domain/rule"
	patternfiles "github.com/wixregiga/arclint/internal/infrastructure/pattern/files"
)

//go:embed vertical domain-model
var assets embed.FS

// Namespace every built-in Pattern must declare.
const Namespace = "arclint"

// Source implements the application's PatternSource port over the
// embedded Pattern packages.
type Source struct{}

// NewSource returns the built-in Pattern source.
func NewSource() Source { return Source{} }

// Names returns the built-in Pattern package names in deterministic
// order.
func (s Source) Names() ([]string, error) {
	entries, err := fs.ReadDir(assets, ".")
	if err != nil {
		return nil, fmt.Errorf("embedded patterns: %v", err)
	}
	var names []string
	for _, e := range entries {
		if e.IsDir() {
			names = append(names, e.Name())
		}
	}
	sort.Strings(names)
	return names, nil
}

// Available loads every built-in Pattern with its exact files, in
// reference order.
func (s Source) Available() ([]distribution.Available, error) {
	names, err := s.Names()
	if err != nil {
		return nil, err
	}
	out := make([]distribution.Available, 0, len(names))
	for _, name := range names {
		a, err := load(name)
		if err != nil {
			return nil, err
		}
		out = append(out, a)
	}
	sort.Slice(out, func(i, j int) bool {
		return out[i].Reference().String() < out[j].Reference().String()
	})
	return out, nil
}

// Patterns loads every built-in Pattern in reference order.
func (s Source) Patterns() ([]rule.Pattern, error) {
	available, err := s.Available()
	if err != nil {
		return nil, err
	}
	out := make([]rule.Pattern, 0, len(available))
	for _, a := range available {
		out = append(out, a.Pattern)
	}
	return out, nil
}

func load(name string) (distribution.Available, error) {
	files, err := patternfiles.Collect(assets, name)
	if err != nil {
		return distribution.Available{}, fmt.Errorf("embedded pattern %s: %w", name, err)
	}
	p, err := patternfiles.Load(files, "embedded/"+name)
	if err != nil {
		return distribution.Available{}, fmt.Errorf("embedded pattern %s: %w", name, err)
	}
	if ns := p.Reference().Namespace(); ns != Namespace {
		return distribution.Available{}, fmt.Errorf("embedded pattern %s: declares namespace %q, but built-in patterns are %q", name, ns, Namespace)
	}
	if p.Reference().Name() != name {
		return distribution.Available{}, fmt.Errorf("embedded pattern %s: declares name %q; the directory must carry the pattern name", name, p.Reference().Name())
	}
	v, err := distribution.Vendor(p.Reference(), files)
	if err != nil {
		return distribution.Available{}, fmt.Errorf("embedded pattern %s: %w", name, err)
	}
	a, err := distribution.NewAvailable(distribution.SourceEmbedded, p, v, false)
	if err != nil {
		return distribution.Available{}, fmt.Errorf("embedded pattern %s: %w", name, err)
	}
	return a, nil
}
