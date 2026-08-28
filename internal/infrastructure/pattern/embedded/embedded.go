// Package embeddedpattern supplies built-in Pattern packages embedded
// in the binary. Built-in Patterns differ from local ones only in
// where their bytes come from: identity is owned here, not a header.
package embeddedpattern

import (
	"embed"
	"errors"
	"fmt"
	"io/fs"
	"sort"
	"strings"

	"github.com/wixregiga/arclint/internal/application"
	"github.com/wixregiga/arclint/internal/domain/rule"
	"github.com/wixregiga/arclint/internal/domain/vocab"
	yamlrule "github.com/wixregiga/arclint/internal/infrastructure/rule/yaml"
)

//go:embed vertical
var assets embed.FS

const (
	// Namespace of every built-in Pattern.
	Namespace = "arclint"
	// Version of the shipped vertical Pattern.
	Version = "0.1.0"
)

// Source implements the application's PatternSource port over the
// embedded Pattern packages.
type Source struct{}

// NewSource returns the built-in Pattern source.
func NewSource() Source { return Source{} }

// Names returns built-in Pattern names in deterministic order.
func (s Source) Names() []string {
	return []string{"vertical"}
}

type patternBundle struct {
	name       string
	ruleset    string
	extensions []rule.PatternExtension
}

func loadBundle(name string) (patternBundle, error) {
	rulesPath := name + "/rules.yaml"
	data, err := assets.ReadFile(rulesPath)
	if err != nil {
		return patternBundle{}, fmt.Errorf("embedded pattern %s: missing ruleset", name)
	}
	exts, err := loadEmbeddedExtensions(name)
	if err != nil {
		return patternBundle{}, err
	}
	return patternBundle{name: name, ruleset: string(data), extensions: exts}, nil
}

func loadEmbeddedExtensions(name string) ([]rule.PatternExtension, error) {
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
		if e.IsDir() {
			continue
		}
		n := e.Name()
		if strings.HasSuffix(n, ".ts") || strings.HasSuffix(n, ".js") {
			names = append(names, n)
		}
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

// Scaffold returns one built-in Pattern bundle for init materialization.
func (s Source) Scaffold(name string) (application.PatternScaffold, bool) {
	b, err := loadBundle(name)
	if err != nil {
		return application.PatternScaffold{}, false
	}
	return application.PatternScaffold{Ruleset: b.ruleset, Extensions: b.extensions}, true
}

// Patterns loads every built-in Pattern package in deterministic
// reference order. Identity comes from code constants, not a header.
func (s Source) Patterns() ([]rule.Pattern, error) {
	var out []rule.Pattern
	for _, name := range s.Names() {
		b, err := loadBundle(name)
		if err != nil {
			return nil, err
		}
		path := "embedded/" + name + "/rules.yaml"
		doc, err := yamlrule.Load([]byte(b.ruleset), path, vocab.UbiquitousLanguage{})
		if err != nil {
			return nil, fmt.Errorf("load pattern: %w", err)
		}
		p, err := rule.NewPattern(Namespace, name, Version, doc.Configured.Rules, b.extensions,
			[]rule.Language{rule.LanguageGo})
		if err != nil {
			return nil, fmt.Errorf("%s: %v", path, err)
		}
		out = append(out, p)
	}
	sort.Slice(out, func(i, j int) bool {
		return out[i].Reference().String() < out[j].Reference().String()
	})
	return out, nil
}
