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
		{"context", "context [paths...]", "run before editing under any path: the owning zones, their import contracts, and the recorded domain in one answer (`--zone <names>`, `--format json`)"},
		{"domain", "domain", "the ubiquitous language: contexts, aggregates, value objects, invariants, relations"},
		{"rules", "rules [selector]", "every configured rule with its constraint and rationale; one match prints the complete rule"},
		{"check", "check .", "evaluate every rule; the findings are your to-do list; exit 1 on error-severity findings"},
		{"rules test", "rules test", "run the rule fixtures under `.arclint/tests` after changing any rule"},
		{"sdk init", "sdk init", "regenerate the extension SDK artifacts under `.arclint/extensions`"},
		{"agents workflow install", "agents workflow install", "write the workflow hooks into Claude Code and Codex project configuration; they advise when work skips context, domain recording or the check, and never block"},
		{"agents workflow status", "agents workflow status", "show which hosts' configuration lists the workflow hooks; Codex runs them after you trust them in /hooks"},
		{"agents md", "agents md --write", "refresh this block after changing " + rule.RulesetFileName + " or the vocabulary"},
		{"agents skill", "agents skill", "write the " + vocab.SkillName + " skill to `" + vocab.SkillDirectory + "/` when your harness lacks it"},
		{"baseline", "baseline", "manage the committed baseline of adopted findings"},
		{"patterns", "patterns", "list the Patterns that resolve offline (embedded, vendored, authored); `patterns install <pattern>` extends " + rule.RulesetFileName + " with one, `patterns vendor` copies one under `.arclint/patterns`"},
	}
}

// AgentWorkflow is the order a coding agent works in, one action for each
// step. The generated block and the workflow hooks' session-start
// orientation both state it from here, so the two cannot drift. recording
// is the domain recording's path relative to the project root.
func AgentWorkflow(recording string) []string {
	return []string{
		"Run `arclint context <paths>` on the files you will read or change, before opening them. " +
			"It answers with the zones, contracts and recorded domain that bind them; " +
			"do not learn the architecture by reading file after file or guessing from folder names.",
		"Before editing, state in the project's language:\n" +
			"   - This change does ___.\n" +
			"   - These decisions belong to ___.\n" +
			"   - The caller uses ___.\n" +
			"   - Existing behavior ___ must remain intact.\n\n" +
			"   Resolve contradictions with the recorded domain and the user's instructions before proceeding. " +
			"Ask only when an unresolved product decision needs the user's input. " +
			"This statement is not an approval checkpoint.",
		"If the change introduces or changes a meaning, record it in `" + recording + "` before writing code, " +
			"using the " + vocab.SkillName + " skill. If it does not, say so.",
		"Implement one complete behavior path through the zones `arclint context` reported, " +
			"keeping existing behavior intact unless the task changes it. " +
			"Keep names and files understandable from their responsibilities: callers use contracts, " +
			"and implementations own their specific decisions.",
		"Verify the changed behavior through those contracts, then run `arclint check .` and the project's tests before finishing. " +
			"Fix the findings in the code you changed, including baseline findings there; " +
			"never weaken rules, baselines or exclusions to clear a finding. " +
			"Report the behavior verified, the checks passed and what remains open, separately.",
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
	writeWorkflow(&b)
	writeCommands(&b)
	if recorded && !lang.Empty() {
		writeRecordedDomain(&b, lang)
	}
	writeZoneRules(&b, cfg)
	writeBuiltInRules(&b, cfg)
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

// writeWorkflow states the order of every change. It is emitted
// unconditionally: the order holds whether or not a domain is recorded or
// the skill is installed, and it is how an unrecorded domain gets its
// first entry.
func writeWorkflow(b *strings.Builder) {
	b.WriteString("### Workflow\n\n")
	b.WriteString("IMPORTANT: work in this order on every change.\n\n")
	for index, step := range AgentWorkflow(vocab.UbiquitousLanguageFileName) {
		fmt.Fprintf(b, "%d. %s\n", index+1, step)
	}
	b.WriteString("\n")
}

// writeCommands lists the command surface with when-to-use guidance.
func writeCommands(b *strings.Builder) {
	b.WriteString("### Commands\n\n")
	for _, c := range AgentCommandSurface() {
		fmt.Fprintf(b, "- `arclint %s`: %s\n", c.Usage, c.Doc)
	}
	b.WriteString("\n")
}

// writeRecordedDomain snapshots the recorded Ubiquitous Language:
// tallies, each context's aggregates with their members, its value
// objects, events, and services, and the context map.
func writeRecordedDomain(b *strings.Builder, lang vocab.UbiquitousLanguage) {
	b.WriteString("### The recorded domain\n\n")
	counts := lang.Counts()
	fmt.Fprintf(b, "%d contexts, %d aggregates, %d value objects, %d invariants (%s).\n\n",
		counts.Contexts, counts.Aggregates, counts.ValueObjects, counts.Invariants, vocab.UbiquitousLanguageFileName)
	for _, ctx := range lang.Contexts {
		var parts []string
		if len(ctx.Aggregates) > 0 {
			names := make([]string, 0, len(ctx.Aggregates))
			for _, a := range ctx.Aggregates {
				name := a.Name
				if len(a.Entities) > 0 {
					members := make([]string, 0, len(a.Entities))
					for _, e := range a.Entities {
						members = append(members, e.Name)
					}
					name += " (" + strings.Join(members, ", ") + ")"
				}
				names = append(names, name)
			}
			parts = append(parts, "aggregates "+strings.Join(names, ", "))
		}
		if len(ctx.ValueObjects) > 0 {
			names := make([]string, 0, len(ctx.ValueObjects))
			for _, v := range ctx.ValueObjects {
				names = append(names, v.Name)
			}
			parts = append(parts, "value objects "+strings.Join(names, ", "))
		}
		if len(ctx.Events) > 0 {
			names := make([]string, 0, len(ctx.Events))
			for _, e := range ctx.Events {
				names = append(names, e.Name)
			}
			parts = append(parts, "events "+strings.Join(names, ", "))
		}
		if len(ctx.Services) > 0 {
			names := make([]string, 0, len(ctx.Services))
			for _, s := range ctx.Services {
				names = append(names, s.Name)
			}
			parts = append(parts, "services "+strings.Join(names, ", "))
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

// writeZoneRules lists every declared Zone with its import
// contract and the Rules bound to it, propositions included; the consumes
// Rule is folded into the imports line rather than repeated.
func writeZoneRules(b *strings.Builder, cfg rule.Configured) {
	if len(cfg.Zones) == 0 {
		return
	}
	b.WriteString("### Zones and their rules\n\n")
	for _, m := range cfg.Zones {
		p := zonePolicy(m, cfg.Rules)
		line := "- **" + p.Name + "**"
		if p.Description != "" {
			line += ": " + p.Description
		}
		b.WriteString(line + " (paths " + strings.Join(p.Paths, " ") + ")\n")
		if imports := importsLine(p); imports != "" {
			b.WriteString("  - " + imports + "\n")
		}
		for _, r := range cfg.Rules {
			if r.Type() == rule.TypeConsumes || !nameIn(r.Scope().Zones(), m.Name()) {
				continue
			}
			proposition := strings.TrimPrefix(r.Proposition(), fmt.Sprintf("Zone %q: ", m.Name()))
			b.WriteString("  - " + ruleLine(ruleName(r, true), r, proposition) + "\n")
		}
	}
	b.WriteString("\n")
}

// importsLine states one Zone's dependency policy in the block's
// compact voice; a Zone without a consumes Rule gets no line.
func importsLine(p ZonePolicy) string {
	var parts []string
	if p.InternalRestricted {
		if len(p.Internal) == 0 {
			parts = append(parts, "imports no other zone")
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

// writeBuiltInRules lists the Rules arclint composes from the DDD
// meta-model once a domain is recorded: they judge the recorded domain
// against the code without any Pattern, and a ruleset adopts one the
// way it adopts a distributed Rule, with an Override under its id.
func writeBuiltInRules(b *strings.Builder, cfg rule.Configured) {
	var lines []string
	for _, r := range cfg.Rules {
		if !r.BuiltIn() {
			continue
		}
		lines = append(lines, "- "+ruleLine(ruleName(r, false), r, r.Proposition()))
	}
	if len(lines) == 0 {
		return
	}
	b.WriteString("### Built-in rules\n\n")
	fmt.Fprintf(b, "%d rules %s judge the recorded domain against the code; no Pattern distributes them. "+
		"Change one through an Override under its id in %s (severity, or disable with a reason).\n\n",
		len(lines), BuiltInOrigin, rule.RulesetFileName)
	for _, l := range lines {
		b.WriteString(l + "\n")
	}
	b.WriteString("\n")
}

// writeRepositoryRules lists the Rules that range over the repository
// rather than one Zone: layers, protections, cycles, and
// repository-scoped extension rules.
func writeRepositoryRules(b *strings.Builder, cfg rule.Configured) {
	var lines []string
	for _, r := range cfg.Rules {
		if r.Type() == rule.TypeConsumes || r.BuiltIn() || len(r.Scope().Zones()) > 0 {
			continue
		}
		lines = append(lines, "- "+ruleLine(ruleName(r, false), r, r.Proposition()))
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
// proposition and optional rationale; an extension Rule also states its validated parameters.
func ruleLine(name string, r rule.Rule, proposition string) string {
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
		proposition += " (" + formatExtensionParams(params.With) + ")"
	}
	if rationale := r.Rationale().String(); rationale != "" {
		proposition += " Rationale: " + rationale
	}
	return name + ": " + proposition
}

// ruleName spells a Rule in the block. A Rule an extended Pattern
// distributes keeps its qualified id, the spelling an Override and
// `arclint rules` take. A local Rule reads by its local id, and under
// its Zone drops the leading segment, so "entities/aggregate-slices"
// reads "aggregate-slices".
func ruleName(r rule.Rule, underZone bool) string {
	if _, distributed := r.Provenance(); distributed {
		return r.ID().Qualified()
	}
	if underZone {
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
