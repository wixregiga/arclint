package application

import (
	"fmt"
	"strings"
)

const (
	// BarePattern is the name of the commented starter ruleset.
	BarePattern = "bare"
)

// RulesetScaffold persists a drafted repository ruleset. Write refuses
// to overwrite an existing ruleset unless forced.
type RulesetScaffold interface {
	Write(content string, force bool) (path string, err error)
}

// PatternScaffolds supplies built-in Pattern packages as
// repository-form rulesets init can materialize.
type PatternScaffolds interface {
	Names() []string
	Ruleset(name string) (content string, ok bool)
}

// InitializeRepositoryRequest carries the explicit choices a draft is
// built from.
type InitializeRepositoryRequest struct {
	// Languages are the runtime targets: go, ts, py. Empty means go.
	Languages []string
	// Pattern selects a built-in Pattern name, or BarePattern for the
	// commented starter. Empty means BarePattern.
	Pattern string
	// Force overwrites an existing ruleset.
	Force bool
}

// InitializeRepository drafts repository Rule and Module configuration
// from explicit choices: a commented starter ruleset the owner grows
// into real Modules and contracts, or a built-in Pattern materialized
// as the repository ruleset. The draft must load through the same
// strict loader that governs every ruleset.
type InitializeRepository struct {
	scaffold RulesetScaffold
	patterns PatternScaffolds
}

// NewInitializeRepository requires the scaffold and built-in Pattern
// ports.
func NewInitializeRepository(scaffold RulesetScaffold, patterns PatternScaffolds) (InitializeRepository, error) {
	if scaffold == nil {
		return InitializeRepository{}, fmt.Errorf("initialize repository: missing ruleset scaffold")
	}
	if patterns == nil {
		return InitializeRepository{}, fmt.Errorf("initialize repository: missing pattern scaffolds")
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

// Patterns returns the init choices: bare, then every built-in Pattern
// name.
func (uc InitializeRepository) Patterns() []string {
	return append([]string{"bare"}, uc.patterns.Names()...)
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

func (uc InitializeRepository) rulesetContent(pattern string, languages []string) (string, error) {
	if pattern == "" {
		pattern = BarePattern
	}
	if pattern == BarePattern {
		return starterRuleset(languages), nil
	}
	content, ok := uc.patterns.Ruleset(pattern)
	if !ok {
		return "", fmt.Errorf("initialize repository: pattern %q is not one of %s", pattern, strings.Join(uc.Patterns(), ", "))
	}
	return strings.Replace(content, "runtime: [go]", fmt.Sprintf("runtime: [%s]", strings.Join(languages, ", ")), 1), nil
}

// starterRuleset renders the commented draft. It declares one Module
// covering the repository and one vacuously-satisfied consumes Rule so
// the owner sees the shape to grow: real Modules, real allow-lists.
func starterRuleset(languages []string) string {
	var b strings.Builder
	b.WriteString("# ArcLint architecture contracts.\n")
	b.WriteString("# Grow this file module by module: declare real Modules under `modules`,\n")
	b.WriteString("# then state what each may import under `contracts`.\n")
	b.WriteString("# Query commands: arclint rules [selector] · arclint context <path>\n\n")
	fmt.Fprintf(&b, "runtime: [%s]\n\n", strings.Join(languages, ", "))
	b.WriteString("scan:\n")
	b.WriteString("  # error | warn | ignore for imports that classify neither stdlib,\n")
	b.WriteString("  # internal, nor declared in the dependency manifest.\n")
	b.WriteString("  unknown_imports: warn\n\n")
	b.WriteString("modules:\n")
	b.WriteString("  source:\n")
	b.WriteString("    paths: [\"**\"]\n")
	b.WriteString("    description: \"Every file. Split into real modules as the architecture takes shape.\"\n\n")
	b.WriteString("contracts:\n")
	b.WriteString("  source:\n")
	b.WriteString("    consumes:\n")
	b.WriteString("      id: \"repo:source/dependencies\"\n")
	b.WriteString("      # An allow-list of other declared modules. Empty means this module\n")
	b.WriteString("      # may import no other declared module; with one module this is\n")
	b.WriteString("      # vacuously true — it starts binding the moment you split modules.\n")
	b.WriteString("      internal: []\n")
	return b.String()
}
