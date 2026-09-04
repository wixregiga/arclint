package application

import (
	"fmt"
	"strings"

	"github.com/wixregiga/arclint/internal/domain/distribution"
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
	// Pattern to adopt by reference (namespace/name@version), by
	// namespace/name at its highest available version, or by name when
	// only one available Pattern carries it. Empty means BarePattern.
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
	if err := validSources("initialize repository", patterns); err != nil {
		return InitializeRepository{}, err
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
// Pattern by reference in resolution order.
func (uc InitializeRepository) Patterns() ([]string, error) {
	catalog, err := loadCatalog(uc.patterns)
	if err != nil {
		return nil, fmt.Errorf("initialize repository: %w", err)
	}
	return append([]string{BarePattern}, catalog.Spellings()...), nil
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
	catalog, err := loadCatalog(uc.patterns)
	if err != nil {
		return "", fmt.Errorf("initialize repository: %w", err)
	}
	p, err := selectPattern(selection, catalog)
	if err != nil {
		return "", err
	}
	inst, err := rule.NewInstallation(p)
	if err != nil {
		return "", fmt.Errorf("initialize repository: %w", err)
	}
	return adoptingRuleset(p, inst, languages), nil
}

// selectPattern resolves a selection against the offline catalog:
// exact reference, namespace/name at its highest version, or bare name
// when exactly one namespace/name carries it.
func selectPattern(selection string, catalog distribution.Catalog) (rule.Pattern, error) {
	refs, err := distribution.Selection(selection, catalog.References())
	if err != nil {
		return rule.Pattern{}, fmt.Errorf("initialize repository: %w", err)
	}
	switch len(refs) {
	case 1:
		a, _ := catalog.Lookup(refs[0])
		return a.Pattern, nil
	case 0:
		choices := append([]string{BarePattern}, catalog.Spellings()...)
		return rule.Pattern{}, fmt.Errorf("initialize repository: pattern %q is not one of %s", selection, strings.Join(choices, ", "))
	}
	return rule.Pattern{}, fmt.Errorf("initialize repository: %w", ambiguous(selection, refs))
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

// adoptingRuleset renders a ruleset that extends the Pattern with the
// drafted Installation. A Module the Installation leaves unbound is
// written as a commented bind entry: the loader then names it unbound,
// so the owner binds it before the first check.
func adoptingRuleset(p rule.Pattern, inst rule.Installation, languages []string) string {
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
	b.WriteString(extendsEntry(inst, "  "))
	b.WriteString("\n")
	b.WriteString("# Override a Pattern Rule under its qualified id (arclint rules lists them):\n")
	b.WriteString("#   rules:\n")
	if rules := p.Rules(); len(rules) > 0 {
		fmt.Fprintf(&b, "#     %s:\n", rules[0].ID().Qualified())
	} else {
		fmt.Fprintf(&b, "#     %s:some/rule:\n", p.Reference().Qualifier())
	}
	b.WriteString("#       severity: warning\n")
	b.WriteString("#       disable: \"why this repository does not hold it\"\n")
	b.WriteString("# House Rules go under new ids beside the overrides.\n")
	return b.String()
}

// extendsEntry renders one extends list item at the given indent: the
// reference, then a bind entry per Pattern Module, commented out when
// the Installation leaves the Module unbound. When nothing is bound the
// whole bind block is commented, so the document stays valid and the
// loader names the unbound Modules.
func extendsEntry(inst rule.Installation, indent string) string {
	var b strings.Builder
	fmt.Fprintf(&b, "%s- pattern: %s\n", indent, inst.Reference())
	fmt.Fprintf(&b, "%s  # bind maps every Pattern Module to the paths it owns here. Paths live\n", indent)
	fmt.Fprintf(&b, "%s  # only in bind: the Pattern's Rules never change when a folder moves.\n", indent)
	off := ""
	if len(inst.Bindings()) == 0 {
		off = "# "
	}
	fmt.Fprintf(&b, "%s  %sbind:\n", indent, off)
	for _, m := range inst.Modules() {
		fmt.Fprintf(&b, "%s    # %s\n", indent, m.Description())
		bound, ok := inst.Binding(m.Name())
		if !ok {
			fmt.Fprintf(&b, "%s    # %s: <glob>\n", indent, m.Name())
			continue
		}
		fmt.Fprintf(&b, "%s    %s: %s\n", indent, m.Name(), bindPaths(bound.Paths()))
	}
	return b.String()
}

// bindPaths spells a Binding's paths: one quoted glob, or a flow list.
func bindPaths(paths []rule.Glob) string {
	if len(paths) == 1 {
		return yamlQuote(paths[0].String())
	}
	quoted := make([]string, 0, len(paths))
	for _, g := range paths {
		quoted = append(quoted, yamlQuote(g.String()))
	}
	return "[" + strings.Join(quoted, ", ") + "]"
}

// yamlQuote spells a glob as a double-quoted YAML scalar so `*` and
// `{` never read as YAML syntax.
func yamlQuote(s string) string {
	return `"` + strings.NewReplacer(`\`, `\\`, `"`, `\"`).Replace(s) + `"`
}
