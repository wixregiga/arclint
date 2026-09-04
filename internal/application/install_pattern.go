package application

import (
	"fmt"
	"strings"

	"github.com/wixregiga/arclint/internal/domain/distribution"
	"github.com/wixregiga/arclint/internal/domain/rule"
)

// InstallPatternRequest selects the Pattern to extend and how to reach
// it when it is not offline yet.
type InstallPatternRequest struct {
	// Selection is namespace/name@version, namespace/name, or a name.
	Selection string
	// Registry is the Registry location; empty means offline only.
	Registry string
	// Languages are the runtime targets written when the ruleset is
	// created by this install; empty means the Pattern's coverage.
	Languages []string
}

// BoundModule is one drafted Binding as the install report shows it.
type BoundModule struct {
	Module string
	Paths  []string
}

// InstallPatternResult reports everything the install touched.
type InstallPatternResult struct {
	Reference string
	Digest    string
	Source    distribution.SourceKind
	// VendoredPath is the directory a Registry Pattern was vendored to,
	// "" when the Pattern was already offline.
	VendoredPath string
	// VendorReplaced is the vendored version replaced, "" when none.
	VendorReplaced string
	// RulesetPath is the ruleset written or edited.
	RulesetPath string
	// RulesetCreated reports that no ruleset existed and one was drafted.
	RulesetCreated bool
	// RulesetReplaced is the version of the same Pattern the ruleset
	// extended before, "" when the extends entry is new.
	RulesetReplaced string
	// Bound lists the Bindings written: the ruleset's own paths for a
	// Module it already declared, the Pattern's suggested paths otherwise.
	Bound []BoundModule
	// Unbound lists the Pattern Modules the owner still has to bind.
	Unbound []string
	// Adopted lists the Modules the ruleset declared itself that are now
	// bound through the Pattern instead.
	Adopted []string
}

// InstallPattern makes one Pattern part of the repository's
// architecture in one step: it resolves the Pattern offline first,
// vendors it when it came from a Registry, and records the Installation
// in rules.arclint.yaml with every suggested Binding, replacing an older
// version of the same Pattern without touching its bindings.
type InstallPattern struct {
	resolver patternResolver
	store    PatternStore
	editor   RulesetEditor
	scaffold RulesetScaffold
}

// NewInstallPattern requires the store, editor, scaffold, and offline
// sources; registry may be nil when only offline installs are offered.
func NewInstallPattern(store PatternStore, editor RulesetEditor, scaffold RulesetScaffold, registry PatternRegistry, sources ...PatternSource) (InstallPattern, error) {
	switch {
	case store == nil:
		return InstallPattern{}, fmt.Errorf("install pattern: missing pattern store")
	case editor == nil:
		return InstallPattern{}, fmt.Errorf("install pattern: missing ruleset editor")
	case scaffold == nil:
		return InstallPattern{}, fmt.Errorf("install pattern: missing ruleset scaffold")
	case len(sources) == 0:
		return InstallPattern{}, fmt.Errorf("install pattern: at least one pattern source required")
	}
	if err := validSources("install pattern", sources); err != nil {
		return InstallPattern{}, err
	}
	return InstallPattern{
		resolver: patternResolver{sources: sources, registry: registry},
		store:    store, editor: editor, scaffold: scaffold,
	}, nil
}

// Execute resolves, vendors when needed, and records the Installation.
func (uc InstallPattern) Execute(req InstallPatternRequest) (InstallPatternResult, error) {
	for _, l := range req.Languages {
		if !supportsLanguage(l) {
			return InstallPatternResult{}, fmt.Errorf("install pattern: language %q is not one of %s", l, strings.Join(supportedLanguages, ", "))
		}
	}
	a, err := uc.resolver.resolve(req.Selection, req.Registry)
	if err != nil {
		return InstallPatternResult{}, fmt.Errorf("install pattern: %w", err)
	}
	result := InstallPatternResult{
		Reference: a.Reference().String(),
		Digest:    a.Digest().String(),
		Source:    a.Kind,
	}
	if a.Kind == distribution.SourceRegistry {
		stored, err := uc.store.Write(a.Vendored)
		if err != nil {
			return InstallPatternResult{}, fmt.Errorf("install pattern %s: vendor: %w", a.Reference(), err)
		}
		result.VendoredPath = stored.Path
		result.VendorReplaced = stored.Replaced
	}
	inst, err := rule.NewInstallation(a.Pattern)
	if err != nil {
		return InstallPatternResult{}, fmt.Errorf("install pattern: %w", err)
	}
	exists, err := uc.editor.Exists()
	if err != nil {
		return InstallPatternResult{}, fmt.Errorf("install pattern %s: %w", a.Reference(), err)
	}
	if exists {
		change, err := uc.editor.Extend(inst)
		if err != nil {
			return InstallPatternResult{}, fmt.Errorf("install pattern %s: %w", a.Reference(), err)
		}
		result.RulesetPath = change.Path
		result.RulesetReplaced = change.Replaced
		if !change.Installation.IsZero() {
			inst = change.Installation
		}
		for _, m := range change.Adopted {
			result.Adopted = append(result.Adopted, m.String())
		}
		result.Bound, result.Unbound = describeBindings(inst)
		return result, nil
	}
	result.Bound, result.Unbound = describeBindings(inst)
	languages := req.Languages
	if len(languages) == 0 {
		languages = runtimeTargetsOf(a.Pattern.Coverage())
	}
	if len(languages) == 0 {
		languages = []string{"go"}
	}
	path, err := uc.scaffold.Write(adoptingRuleset(a.Pattern, inst, languages), false)
	if err != nil {
		return InstallPatternResult{}, fmt.Errorf("install pattern %s: write ruleset: %w", a.Reference(), err)
	}
	result.RulesetPath = path
	result.RulesetCreated = true
	return result, nil
}

// describeBindings spells an Installation for the report.
func describeBindings(inst rule.Installation) (bound []BoundModule, unbound []string) {
	for _, b := range inst.Bindings() {
		paths := make([]string, 0, len(b.Paths()))
		for _, g := range b.Paths() {
			paths = append(paths, g.String())
		}
		bound = append(bound, BoundModule{Module: b.Module().String(), Paths: paths})
	}
	for _, m := range inst.Unbound() {
		unbound = append(unbound, m.Name().String())
	}
	return bound, unbound
}
