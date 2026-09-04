// Package embeddedpattern supplies the built-in Pattern packages
// embedded in the binary. A built-in Pattern is an ordinary Pattern
// distribution file (pattern.yaml plus an extensions directory) whose
// bytes ship with arclint; the only extra check is that its header
// claims the arclint namespace, so a shipped file cannot impersonate a
// third-party Pattern.
package embeddedpattern

import (
	"embed"
	"errors"
	"fmt"
	"io/fs"
	"sort"

	"github.com/wixregiga/arclint/internal/domain/rule"
	yamlrule "github.com/wixregiga/arclint/internal/infrastructure/rule/yaml"
)

//go:embed vertical
var assets embed.FS

// Namespace every built-in Pattern must declare.
const Namespace = "arclint"

// FileName is the Pattern distribution file inside each embedded
// Pattern directory.
const FileName = "pattern.yaml"

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

// Patterns loads every built-in Pattern package in deterministic
// reference order.
func (s Source) Patterns() ([]rule.Pattern, error) {
	names, err := s.Names()
	if err != nil {
		return nil, err
	}
	out := make([]rule.Pattern, 0, len(names))
	for _, name := range names {
		p, err := load(name)
		if err != nil {
			return nil, err
		}
		out = append(out, p)
	}
	sort.Slice(out, func(i, j int) bool {
		return out[i].Reference().String() < out[j].Reference().String()
	})
	return out, nil
}

func load(name string) (rule.Pattern, error) {
	path := name + "/" + FileName
	data, err := assets.ReadFile(path)
	if err != nil {
		return rule.Pattern{}, fmt.Errorf("embedded pattern %s: missing %s", name, FileName)
	}
	exts, err := loadExtensions(name)
	if err != nil {
		return rule.Pattern{}, err
	}
	p, err := yamlrule.LoadPattern(data, "embedded/"+path, exts)
	if err != nil {
		return rule.Pattern{}, fmt.Errorf("load pattern: %w", err)
	}
	if ns := p.Reference().Namespace(); ns != Namespace {
		return rule.Pattern{}, fmt.Errorf("embedded pattern %s: declares namespace %q, but built-in patterns are %q", name, ns, Namespace)
	}
	return p, nil
}

func loadExtensions(name string) ([]rule.PatternExtension, error) {
	dir := name + "/extensions"
	entries, err := fs.ReadDir(assets, dir)
	if errors.Is(err, fs.ErrNotExist) {
		return nil, nil
	}
	if err != nil {
		return nil, fmt.Errorf("embedded pattern %s: %v", name, err)
	}
	var names []string
	for _, e := range entries {
		if e.IsDir() || !rule.InstallableExtensionFileName(e.Name()) {
			continue
		}
		names = append(names, e.Name())
	}
	sort.Strings(names)
	out := make([]rule.PatternExtension, 0, len(names))
	for _, n := range names {
		assetPath := dir + "/" + n
		data, err := assets.ReadFile(assetPath)
		if err != nil {
			return nil, fmt.Errorf("%s: %v", assetPath, err)
		}
		ext, err := rule.NewPatternExtension(n, string(data))
		if err != nil {
			return nil, fmt.Errorf("%s: %v", assetPath, err)
		}
		out = append(out, ext)
	}
	return out, nil
}
