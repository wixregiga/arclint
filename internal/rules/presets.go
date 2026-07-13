package rules

import (
	"fmt"
	"maps"

	"github.com/wixregiga/arclint/internal/config"
)

// Presets are rule sets compiled into the binary and referenced from
// `extends` (rules.md §9.3: binary-embedded presets only in v1 — no
// registry, no URL fetch).

// presetRules returns the rules of a named preset.
func presetRules(name string) (map[string]config.Rule, bool) {
	switch name {
	case "arclint:recommended":
		return map[string]config.Rule{
			"no-utils-dir": {
				Type:        config.CategoryStructure,
				Severity:    config.SeverityError,
				Description: "No grab-bag utility directories.",
				FixHint:     "Move helpers next to the code that uses them.",
				Structure: &config.StructureParams{
					Forbid: []string{"**/utils/**", "**/helpers/**"},
				},
			},
			"readme-required": {
				Type:        config.CategoryStructure,
				Severity:    config.SeverityWarn,
				Description: "Repo must carry a README at the root.",
				FixHint:     "Add a README.md at the repo root.",
				Structure: &config.StructureParams{
					Require: []string{"README.md"},
				},
			},
		}, true
	}
	return nil, false
}

// MergedRules resolves extends: presets in listed order, local rules last.
// Merge is per rule id and whole-rule — a local rule with the same id fully
// replaces the inherited one (rules.md §3). An unknown preset is a config
// error (the caller's exit-2 path).
func MergedRules(cfg *config.File) (map[string]config.Rule, error) {
	out := make(map[string]config.Rule)
	for _, e := range cfg.Extends {
		p, ok := presetRules(e)
		if !ok {
			return nil, fmt.Errorf("unknown preset %q in extends — v1 ships built-in presets only (available: arclint:recommended)", e)
		}
		maps.Copy(out, p)
	}
	maps.Copy(out, cfg.Rules)
	return out, nil
}
