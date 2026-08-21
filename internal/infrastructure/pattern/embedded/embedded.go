// Package embeddedpattern supplies built-in Pattern packages embedded
// in the binary. Built-in Patterns differ from local ones only in
// where their bytes come from: identity is owned here, not a header.
package embeddedpattern

import (
	"embed"
	"fmt"
	"sort"

	"github.com/wixregiga/arclint/internal/domain/rule"
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

// Ruleset returns the repository-form ruleset text for a built-in
// Pattern name, verbatim.
func (s Source) Ruleset(name string) (string, bool) {
	data, err := assets.ReadFile(name + "/rules.yaml")
	if err != nil {
		return "", false
	}
	return string(data), true
}

// Patterns loads every built-in Pattern package in deterministic
// reference order. Identity comes from code constants, not a header.
func (s Source) Patterns() ([]rule.Pattern, error) {
	var out []rule.Pattern
	for _, name := range s.Names() {
		content, ok := s.Ruleset(name)
		if !ok {
			return nil, fmt.Errorf("embedded pattern %s: missing ruleset", name)
		}
		path := "embedded/" + name + "/rules.yaml"
		doc, err := yamlrule.Load([]byte(content), path)
		if err != nil {
			return nil, fmt.Errorf("load pattern: %w", err)
		}
		p, err := rule.NewPattern(Namespace, name, Version, doc.Configured.Rules,
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
