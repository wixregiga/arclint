package application

import (
	"fmt"
	"sort"
	"strings"

	"github.com/wixregiga/arclint/internal/domain/rule"
)

// BarePattern is the init choice that drafts the commented starter
// ruleset instead of adopting a Pattern.
const BarePattern = "bare"

// RulesetScaffold persists a drafted repository ruleset. Write refuses
// to overwrite an existing ruleset unless forced.
type RulesetScaffold interface {
	Write(content string, force bool) (path string, err error)
}

// InitializeRepositoryRequest carries the explicit choices a draft is
// built from.
type InitializeRepositoryRequest struct {
	// Languages are the runtime targets: go, ts, py. Empty means go.
	Languages []string
	// Pattern selects BarePattern for the commented starter, or a
	// Pattern to adopt by reference (namespace/name@version) or by
	// name when only one available Pattern carries it. Empty means
	// BarePattern.
	Pattern string
	// Force overwrites an existing ruleset.
	Force bool
}

// InitializeRepository drafts a repository ruleset from explicit
// choices: a commented starter the owner grows into real Modules and
// Rules, or a ruleset that extends an available Pattern with every
// Pattern Module bound to its suggested paths. The draft loads through
// the same strict loader that governs every ruleset; it is never a
// copy of the Pattern's Rules.
type InitializeRepository struct {
	scaffold RulesetScaffold
	patterns []PatternSource
}

// NewInitializeRepository requires the scaffold port and the Pattern
// sources init may adopt from.
func NewInitializeRepository(scaffold RulesetScaffold, patterns ...PatternSource) (InitializeRepository, error) {
	if scaffold == nil {
		return InitializeRepository{}, fmt.Errorf("initialize repository: missing ruleset scaffold")
	}
	for _, p := range patterns {
		if p == nil {
			return InitializeRepository{}, fmt.Errorf("initialize repository: nil pattern source")
		}
	}
	return InitializeRepository{scaffold: scaffold, patterns: patterns}, nil
}

var supportedLanguages = []string{"go", "ts", "py"}

// SupportedLanguages returns the runtime targets accepted by repository
// initialization, in presentation order.
func SupportedLanguages() []string {
	return append([]string(nil), supportedLanguages...)
}

func supportsLanguage(language string) bool {
	for _, supported := range supportedLanguages {
		if language == supported {
			return true
		}
	}
	return false
}

// Patterns returns the init choices: bare, then every available
// Pattern by reference.
func (uc InitializeRepository) Patterns() ([]string, error) {
	available, err := uc.available()
	if err != nil {
		return nil, err
	}
	choices := []string{BarePattern}
	for _, p := range available {
		choices = append(choices, p.Reference().String())
	}
	return choices, nil
}

func (uc InitializeRepository) available() ([]rule.Pattern, error) {
	var out []rule.Pattern
	for _, source := range uc.patterns {
		ps, err := source.Patterns()
		if err != nil {
			return nil, fmt.Errorf("initialize repository: %w", err)
		}
		out = append(out, ps...)
	}
	sort.SliceStable(out, func(i, j int) bool {
		return out[i].Reference().String() < out[j].Reference().String()
	})
	return out, nil
}

// Execute drafts and persists the starter ruleset, returning its path.
func (uc InitializeRepository) Execute(req InitializeRepositoryRequest) (string, error) {
	languages := req.Languages
	if len(languages) == 0 {
		languages = []string{"go"}
	}
	for _, l := range languages {
		if !supportsLanguage(l) {
			return "", fmt.Errorf("initialize repository: language %q is not one of %s", l, strings.Join(supportedLanguages, ", "))
		}
	}
	content, err := uc.rulesetContent(req.Pattern, languages)
	if err != nil {
		return "", err
	}
	path, err := uc.scaffold.Write(content, req.Force)
	if err != nil {
		return "", fmt.Errorf("write ruleset: %w", err)
	}
	return path, nil
}

func (uc InitializeRepository) rulesetContent(selection string, languages []string) (string, error) {
	if selection == "" || selection == BarePattern {
		return starterRuleset(languages), nil
	}
	available, err := uc.available()
	if err != nil {
		return "", err
	}
	p, err := selectPattern(selection, available)
	if err != nil {
		return "", err
	}
	return adoptingRuleset(p, languages), nil
}

// selectPattern resolves a selection by full reference, or by bare
// name when exactly one available Pattern carries it.
func selectPattern(selection string, available []rule.Pattern) (rule.Pattern, error) {
	var byName []rule.Pattern
	for _, p := range available {
		if p.Reference().String() == selection {
			return p, nil
		}
		if p.Reference().Name() == selection {
			byName = append(byName, p)
		}
	}
	switch len(byName) {
	case 1:
		return byName[0], nil
	case 0:
		choices := []string{BarePattern}
		for _, p := range available {
			choices = append(choices, p.Reference().String())
		}
		return rule.Pattern{}, fmt.Errorf("initialize repository: pattern %q is not one of %s", selection, strings.Join(choices, ", "))
	}
	refs := make([]string, 0, len(byName))
	for _, p := range byName {
		refs = append(refs, p.Reference().String())
	}
	return rule.Pattern{}, fmt.Errorf("initialize repository: pattern name %q is ambiguous; use one of %s", selection, strings.Join(refs, ", "))
}

func runtimeLine(languages []string) string {
	return fmt.Sprintf("runtime: [%s]\n\n", strings.Join(languages, ", "))
}

func scanBlock() string {
	return "scan:\n" +
		"  # error | warn | ignore for imports that classify neither stdlib,\n" +
		"  # internal, nor declared in the dependency manifest.\n" +
		"  unknown_imports: warn\n\n"
}

// starterRuleset renders the commented draft. It declares one Module
// covering the repository and one vacuously-satisfied imports Rule so
// the owner sees the shape to grow: real Modules, real allow-lists.
func starterRuleset(languages []string) string {
	var b strings.Builder
	b.WriteString("# ArcLint architecture contracts.\n")
	b.WriteString("# Grow this file module by module: declare real Modules under `modules`,\n")
	b.WriteString("# then state what each may import under `rules`.\n")
	b.WriteString("# Query commands: arclint rules [selector] · arclint context <path>\n\n")
	b.WriteString(runtimeLine(languages))
	b.WriteString(scanBlock())
	b.WriteString("modules:\n")
	b.WriteString("  # A Module is a name and the paths it owns: one glob, a list of globs,\n")
	b.WriteString("  # or {paths, description}. Split into real Modules as the architecture\n")
	b.WriteString("  # takes shape.\n")
	b.WriteString("  source: \"**\"\n\n")
	b.WriteString("rules:\n")
	b.WriteString("  # Every Rule has an id, the Module(s) it judges under `on`, and exactly\n")
	b.WriteString("  # one assertion: imports, structure, naming, content, layers,\n")
	b.WriteString("  # imported_by, independent, acyclic, invariants, or uses.\n")
	b.WriteString("  source/dependencies:\n")
	b.WriteString("    description: \"Source imports no other declared Module.\"\n")
	b.WriteString("    on: source\n")
	b.WriteString("    imports:\n")
	b.WriteString("      # An allow-list of other declared Modules. Empty means this Module\n")
	b.WriteString("      # may import no other declared Module; with one Module this is\n")
	b.WriteString("      # vacuously true and starts binding the moment you split Modules.\n")
	b.WriteString("      internal: []\n")
	return b.String()
}

// adoptingRuleset renders a ruleset that extends the Pattern, binding
// every Pattern Module to its suggested paths. A Module the Pattern
// suggests no paths for is left as a commented bind entry: the loader
// then names it unbound, so the owner binds it before the first check.
func adoptingRuleset(p rule.Pattern, languages []string) string {
	ref := p.Reference().String()
	var b strings.Builder
	fmt.Fprintf(&b, "# ArcLint architecture contracts: this repository adopts %s.\n", ref)
	for _, line := range strings.Split(strings.TrimSpace(p.Documentation()), "\n") {
		if line = strings.TrimSpace(line); line != "" {
			fmt.Fprintf(&b, "# %s\n", line)
		}
	}
	b.WriteString("# Query commands: arclint rules [selector] · arclint context <path>\n\n")
	b.WriteString(runtimeLine(languages))
	b.WriteString(scanBlock())
	b.WriteString("extends:\n")
	fmt.Fprintf(&b, "  - pattern: %s\n", ref)
	b.WriteString("    # bind maps every Pattern Module to the paths it owns here. Paths live\n")
	b.WriteString("    # only in bind: the Pattern's Rules never change when a folder moves.\n")
	b.WriteString("    bind:\n")
	for _, m := range p.Modules() {
		fmt.Fprintf(&b, "      # %s\n", m.Description())
		paths := m.SuggestedPaths()
		switch len(paths) {
		case 0:
			fmt.Fprintf(&b, "      # %s: <glob>\n", m.Name())
		case 1:
			fmt.Fprintf(&b, "      %s: %s\n", m.Name(), yamlQuote(paths[0].String()))
		default:
			quoted := make([]string, 0, len(paths))
			for _, g := range paths {
				quoted = append(quoted, yamlQuote(g.String()))
			}
			fmt.Fprintf(&b, "      %s: [%s]\n", m.Name(), strings.Join(quoted, ", "))
		}
	}
	b.WriteString("\n")
	b.WriteString("# Override a Pattern Rule under its qualified id (arclint rules lists them):\n")
	b.WriteString("#   rules:\n")
	if rules := p.Rules(); len(rules) > 0 {
		fmt.Fprintf(&b, "#     %s:\n", rules[0].ID().Qualified())
	} else {
		fmt.Fprintf(&b, "#     %s:some/rule:\n", p.Reference().Namespace())
	}
	b.WriteString("#       severity: warning\n")
	b.WriteString("#       disable: \"why this repository does not hold it\"\n")
	b.WriteString("# House Rules go under new ids beside the overrides.\n")
	return b.String()
}

// yamlQuote spells a glob as a double-quoted YAML scalar so `*` and
// `{` never read as YAML syntax.
func yamlQuote(s string) string {
	return `"` + strings.NewReplacer(`\`, `\\`, `"`, `\"`).Replace(s) + `"`
}
