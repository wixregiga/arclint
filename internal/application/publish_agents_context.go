package application

import (
	"fmt"
	"sort"
	"strings"

	"github.com/wixregiga/arclint/internal/domain/rule"
	"github.com/wixregiga/arclint/internal/domain/vocab"
)

// AgentsPublisher installs the generated architecture block into the
// repository's agent documentation, preserving everything outside the
// managed markers. It reports whether the document changed.
type AgentsPublisher interface {
	Install(block string) (changed bool, path string, err error)
}

// RegisteredExtensionRule is one extension-registered rule definition
// and the repo-relative extension file that default-exports it.
type RegisteredExtensionRule struct {
	Name   string
	Source string
}

// ExtensionInventory is the port to the repository's local extension
// registry: every registered rule definition, in registration order.
type ExtensionInventory interface {
	RegisteredExtensionRules() ([]RegisteredExtensionRule, error)
}

// AgentsMarkers delimit the generated block; hand-written content
// outside them survives every regeneration.
const (
	AgentsBegin = "<!-- arclint:agents:begin -->"
	AgentsEnd   = "<!-- arclint:agents:end -->"
)

// AgentCommandDoc is one command bullet of the generated block:
// Command is the bare command path for mechanical verification against
// the CLI, Usage the display form the bullet renders, Doc the
// when-to-use guidance.
type AgentCommandDoc struct {
	Command string
	Usage   string
	Doc     string
}

// AgentCommandSurface is the command surface the block teaches, in
// rendered order. The e2e suite proves every entry resolves to a
// registered command and every user-facing root command is taught, so
// the surface cannot silently go stale.
func AgentCommandSurface() []AgentCommandDoc {
	return []AgentCommandDoc{
		{"context", "context [paths...]", "run before editing under any path: the owning modules, their import contracts, and the recorded domain in one answer (`--module <names>`, `--format json`)"},
		{"domain", "domain", "the ubiquitous language: contexts, aggregates, value objects, invariants, relations"},
		{"rules", "rules [selector]", "every configured rule with its claim; one match prints the complete rule"},
		{"check", "check .", "evaluate every rule; the findings are your to-do list; exit 1 on error-severity findings"},
		{"rules test", "rules test", "run the rule fixtures under `.arclint/tests` after changing any rule"},
		{"sdk init", "sdk init", "regenerate the extension SDK artifacts under `.arclint/extensions`"},
		{"agents md", "agents md --write", "refresh this block after changing " + rule.RulesetFileName + " or the vocabulary"},
		{"baseline", "baseline", "manage the committed baseline of adopted findings"},
		{"patterns", "patterns", "list the Patterns that resolve offline (embedded, vendored, authored); `patterns install <pattern>` extends " + rule.RulesetFileName + " with one, `patterns vendor` copies one under `.arclint/patterns`"},
	}
}

// PublishAgentsContext compiles the ruleset, the recorded domain, and
// the local extension inventory into the architecture block a coding
// agent needs at prompt time, and installs it through the publisher
// port. The rendered block is the business artifact: deterministic for
// one repository state, no timestamps.
type PublishAgentsContext struct {
	rules      rule.Repository
	knowledge  vocab.Repository
	extensions ExtensionInventory
	publisher  AgentsPublisher
}

// NewPublishAgentsContext requires the Rule, domain-model, and
// extension-inventory ports plus the publisher.
func NewPublishAgentsContext(rules rule.Repository, knowledge vocab.Repository,
	extensions ExtensionInventory, publisher AgentsPublisher,
) (PublishAgentsContext, error) {
	if rules == nil {
		return PublishAgentsContext{}, fmt.Errorf("publish agents context: missing rule repository")
	}
	if knowledge == nil {
		return PublishAgentsContext{}, fmt.Errorf("publish agents context: missing domain model repository")
	}
	if extensions == nil {
		return PublishAgentsContext{}, fmt.Errorf("publish agents context: missing extension inventory")
	}
	if publisher == nil {
		return PublishAgentsContext{}, fmt.Errorf("publish agents context: missing publisher")
	}
	return PublishAgentsContext{rules: rules, knowledge: knowledge, extensions: extensions, publisher: publisher}, nil
}

// Render compiles the block without installing it.
func (uc PublishAgentsContext) Render() (string, error) {
	cfg, err := uc.rules.ConfiguredRules()
	if err != nil {
		return "", fmt.Errorf("load configured rules: %w", err)
	}
	lang, recorded, err := uc.knowledge.RecordedLanguage()
	if err != nil {
		return "", fmt.Errorf("load domain model: %w", err)
	}
	registered, err := uc.extensions.RegisteredExtensionRules()
	if err != nil {
		return "", fmt.Errorf("load extension inventory: %w", err)
	}
	return renderAgentsBlock(cfg, lang, recorded, registered), nil
}

// Execute compiles and installs the block, reporting whether the
// document changed and where it lives.
func (uc PublishAgentsContext) Execute() (changed bool, path string, err error) {
	block, err := uc.Render()
	if err != nil {
		return false, "", err
	}
	changed, path, err = uc.publisher.Install(block)
	if err != nil {
		return false, "", fmt.Errorf("install agents context: %w", err)
	}
	return changed, path, nil
}

func renderAgentsBlock(cfg rule.Configured, lang vocab.UbiquitousLanguage,
	recorded bool, registered []RegisteredExtensionRule,
) string {
	var b strings.Builder
	b.WriteString("## Architecture contracts (arclint)\n\n")
	languages := make([]string, 0, len(cfg.Languages))
	for _, l := range cfg.Languages {
		languages = append(languages, string(l))
	}
	fmt.Fprintf(&b, "Enforced from %s: %d rules over languages [%s].\n\n",
		rule.RulesetFileName,
		len(cfg.Rules), strings.Join(languages, ", "))
	writeExtendedPatterns(&b, cfg)
	writeAskFirst(&b)
	if recorded && !lang.Empty() {
		writeRecordedDomain(&b, lang)
	}
	writeChangingLanguage(&b)
	writeModuleRules(&b, cfg)
	writeRepositoryRules(&b, cfg)
	writeExtensionInventory(&b, registered)
	return AgentsBegin + "\n" + strings.TrimRight(b.String(), "\n") + "\n" + AgentsEnd + "\n"
}

// writeExtendedPatterns names every Pattern the ruleset extends with
// the number of Rules it distributes, so an agent reading a qualified
// Rule ID below knows which Pattern owns it and that the Rule is
// changed through an Override in rules.arclint.yaml, never by editing the
// Pattern.
func writeExtendedPatterns(b *strings.Builder, cfg rule.Configured) {
	var refs []rule.PatternReference
	counts := map[string]int{}
	for _, r := range cfg.Rules {
		ref, ok := r.Provenance()
		if !ok {
			continue
		}
		key := ref.String()
		if _, seen := counts[key]; !seen {
			refs = append(refs, ref)
		}
		counts[key]++
	}
	if len(refs) == 0 {
		return
	}
	parts := make([]string, 0, len(refs))
	for _, ref := range refs {
		parts = append(parts, fmt.Sprintf("`%s` (%d rules, ids qualified `%s:`)", ref, counts[ref.String()], ref.Qualifier()))
	}
	fmt.Fprintf(b, "Extended Patterns: %s. A Pattern Rule is listed and reported under its qualified id; "+
		"change it through an Override under that id in "+rule.RulesetFileName+" (`arclint rules <id>` prints it), never by editing the Pattern.\n\n",
		strings.Join(parts, "; "))
}

// writeAskFirst is the fixed imperative opening (ask the tool, never
// survey the tree) followed by the command surface with when-to-use
// guidance.
func writeAskFirst(b *strings.Builder) {
	b.WriteString("### Ask arclint first\n\n")
	b.WriteString("IMPORTANT: you MUST ask arclint before reading around. " +
		"The architecture, the rules, and the recorded domain are queryable; " +
		"run `arclint context` on the paths you expect to touch BEFORE opening source files, " +
		"and do NOT learn the architecture by reading file after file or guessing from folder names.\n\n")
	for _, c := range AgentCommandSurface() {
		fmt.Fprintf(b, "- `arclint %s`: %s\n", c.Usage, c.Doc)
	}
	b.WriteString("\n")
}

// writeRecordedDomain snapshots the recorded Ubiquitous Language:
// tallies, each context's terms with aggregates marked, and the
// context map.
func writeRecordedDomain(b *strings.Builder, lang vocab.UbiquitousLanguage) {
	b.WriteString("### The recorded domain\n\n")
	counts := lang.Counts()
	fmt.Fprintf(b, "%d contexts, %d aggregates, %d invariants (%s).\n\n",
		counts.Contexts, counts.Aggregates, counts.Invariants, vocab.UbiquitousLanguageFileName)
	for _, ctx := range lang.Contexts {
		var parts []string
		if len(ctx.Entities) > 0 {
			names := make([]string, 0, len(ctx.Entities))
			for _, e := range ctx.Entities {
				name := e.Name
				if e.Aggregate {
					name += " [aggregate]"
				}
				names = append(names, name)
			}
			parts = append(parts, strings.Join(names, ", "))
		}
		if len(ctx.ValueObjects) > 0 {
			parts = append(parts, "value objects "+joinDefinitionNames(ctx.ValueObjects))
		}
		if len(ctx.Events) > 0 {
			parts = append(parts, "events "+joinDefinitionNames(ctx.Events))
		}
		line := "- **" + ctx.Name + "**"
		if len(parts) > 0 {
			line += ": " + strings.Join(parts, "; ")
		}
		b.WriteString(line + "\n")
	}
	b.WriteString("\n")
	if len(lang.Relations) > 0 {
		edges := make([]string, 0, len(lang.Relations))
		for _, r := range lang.Relations {
			edges = append(edges, fmt.Sprintf("%s → %s (%s)", r.From, r.To, r.Kind))
		}
		fmt.Fprintf(b, "Relations: %s. Full text: `arclint domain`.\n\n", strings.Join(edges, "; "))
		return
	}
	b.WriteString("Full text: `arclint domain`.\n\n")
}

// writeChangingLanguage is the fixed obligation to evolve the recorded
// vocabulary before the code, through the domain-librarian skill. It is
// emitted unconditionally; the obligation stands whether or not the
// skill files are installed, and it is how an unrecorded domain gets
// its first entry.
func writeChangingLanguage(b *strings.Builder) {
	b.WriteString("### Changing the language\n\n")
	fmt.Fprintf(b, "If your change speaks about something new, or changes what a recorded term means, "+
		"record it in `%s` before writing code. Invoke the %s skill for that work: "+
		"it decides how a concept is classified, what evidence a recording needs, "+
		"and when an open question is recorded instead of a guess. "+
		"If your harness does not have the skill, `arclint agents skill` writes it to `%s/`.\n\n",
		vocab.UbiquitousLanguageFileName, vocab.SkillName, vocab.SkillDirectory)
}

func joinDefinitionNames(defs []vocab.Definition) string {
	names := make([]string, 0, len(defs))
	for _, d := range defs {
		names = append(names, d.Name)
	}
	return strings.Join(names, ", ")
}

// writeModuleRules lists every declared Module with its import
// contract and the Rules bound to it, Claims included; the consumes
// Rule is folded into the imports line rather than repeated.
func writeModuleRules(b *strings.Builder, cfg rule.Configured) {
	if len(cfg.Modules) == 0 {
		return
	}
	b.WriteString("### Modules and their rules\n\n")
	for _, m := range cfg.Modules {
		p := modulePolicy(m, cfg.Rules)
		line := "- **" + p.Name + "**"
		if p.Description != "" {
			line += ": " + p.Description
		}
		b.WriteString(line + " (paths " + strings.Join(p.Paths, " ") + ")\n")
		if imports := importsLine(p); imports != "" {
			b.WriteString("  - " + imports + "\n")
		}
		for _, r := range cfg.Rules {
			if r.Type() == rule.TypeConsumes || !nameIn(r.Applicability().Modules(), m.Name()) {
				continue
			}
			claim := strings.TrimPrefix(r.Claim().Statement(), fmt.Sprintf("Module %q: ", m.Name()))
			b.WriteString("  - " + ruleLine(ruleName(r, true), r, claim) + "\n")
		}
	}
	b.WriteString("\n")
}

// importsLine states one Module's dependency policy in the block's
// compact voice; a Module without a consumes Rule gets no line.
func importsLine(p ModulePolicy) string {
	var parts []string
	if p.InternalRestricted {
		if len(p.Internal) == 0 {
			parts = append(parts, "imports no other module")
		} else {
			parts = append(parts, "imports only: "+strings.Join(p.Internal, ", "))
		}
	}
	if p.External == string(rule.ImportForbid) {
		parts = append(parts, "external imports forbidden")
	}
	if p.Stdlib == string(rule.ImportForbid) {
		parts = append(parts, "stdlib imports forbidden")
	}
	return strings.Join(parts, "; ")
}

// writeRepositoryRules lists the Rules that range over the repository
// rather than one Module: layers, protections, cycles, and
// repository-scoped extension rules.
func writeRepositoryRules(b *strings.Builder, cfg rule.Configured) {
	var lines []string
	for _, r := range cfg.Rules {
		if r.Type() == rule.TypeConsumes || len(r.Applicability().Modules()) > 0 {
			continue
		}
		lines = append(lines, "- "+ruleLine(ruleName(r, false), r, r.Claim().Statement()))
	}
	if len(lines) == 0 {
		return
	}
	b.WriteString("### Repository-wide rules\n\n")
	for _, l := range lines {
		b.WriteString(l + "\n")
	}
	b.WriteString("\n")
}

// writeExtensionInventory lists each extension source, local or
// distributed by an extended Pattern, with the rule definitions it
// registers, so an agent knows what enforcement exists beyond the
// built-in Rule Types.
func writeExtensionInventory(b *strings.Builder, registered []RegisteredExtensionRule) {
	if len(registered) == 0 {
		return
	}
	b.WriteString("### Extension rules\n\n")
	var sources []string
	names := map[string][]string{}
	for _, r := range registered {
		if _, seen := names[r.Source]; !seen {
			sources = append(sources, r.Source)
		}
		names[r.Source] = append(names[r.Source], r.Name)
	}
	for _, source := range sources {
		fmt.Fprintf(b, "`%s` default-exports the rule definitions: %s.\n",
			source, strings.Join(names[source], ", "))
	}
	b.WriteString("\n")
}

// ruleLine renders one Rule as name, non-default annotations, and the
// Claim; an extension Rule also states its validated parameters.
func ruleLine(name string, r rule.Rule, claim string) string {
	var notes []string
	if r.Severity() != rule.DefaultSeverity {
		notes = append(notes, string(r.Severity()))
	}
	if d, ok := r.Disablement(); ok {
		notes = append(notes, "disabled: "+d.Reason())
	}
	if len(notes) > 0 {
		name += " (" + strings.Join(notes, ", ") + ")"
	}
	if params, ok := r.Params().(rule.ExtensionParams); ok && len(params.With) > 0 {
		claim += " (" + formatExtensionParams(params.With) + ")"
	}
	return name + ": " + claim
}

// ruleName spells a Rule in the block. A Rule an extended Pattern
// distributes keeps its qualified id, the spelling an Override and
// `arclint rules` take. A local Rule reads by its local id, and under
// its Module drops the leading segment, so "entities/aggregate-slices"
// reads "aggregate-slices".
func ruleName(r rule.Rule, underModule bool) string {
	if _, distributed := r.Provenance(); distributed {
		return r.ID().Qualified()
	}
	if underModule {
		if _, rest, ok := strings.Cut(r.ID().Local(), "/"); ok {
			return rest
		}
	}
	return r.ID().Local()
}

// formatExtensionParams renders an extension Rule's with-parameters
// deterministically: keys sorted, values in plain notation.
func formatExtensionParams(with map[string]any) string {
	keys := make([]string, 0, len(with))
	for k := range with {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	parts := make([]string, 0, len(keys))
	for _, k := range keys {
		parts = append(parts, k+": "+formatParamValue(with[k]))
	}
	return strings.Join(parts, ", ")
}

func formatParamValue(v any) string {
	switch val := v.(type) {
	case string:
		return val
	case []any:
		parts := make([]string, 0, len(val))
		for _, e := range val {
			parts = append(parts, formatParamValue(e))
		}
		return "[" + strings.Join(parts, ", ") + "]"
	case map[string]any:
		return "{" + formatExtensionParams(val) + "}"
	default:
		return fmt.Sprint(val)
	}
}
