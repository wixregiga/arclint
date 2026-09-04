# Agent guide

Repo-specific instructions for coding agents. The architecture block
below is generated from rules.yaml; refresh it with
`arclint agents --write` after changing the ruleset (a test fails when
it drifts). Add hand-written guidance outside the markers.

<!-- arclint:agents:begin -->
## Architecture contracts (arclint)

Enforced from rules.yaml: 32 rules over languages [go, typescript].

Extended Patterns: `arclint/domain-model@0.1.0` (3 rules, ids qualified `arclint/domain-model:`). A Pattern Rule is listed and reported under its qualified id; change it through an Override under that id in rules.yaml (`arclint rules <id>` prints it), never by editing the Pattern.

### Ask arclint first

IMPORTANT: you MUST ask arclint before reading around. The architecture, the rules, and the recorded domain are queryable; run `arclint context` on the paths you expect to touch BEFORE opening source files, and do NOT learn the architecture by reading file after file or guessing from folder names.

- `arclint context [paths...]`: run before editing under any path: the owning modules, their import contracts, and the recorded domain in one answer (`--module <names>`, `--format json`)
- `arclint domain`: the ubiquitous language: contexts, aggregates, value objects, invariants, relations
- `arclint rules [selector]`: every configured rule with its claim; one match prints the complete rule
- `arclint check .`: evaluate every rule; the findings are your to-do list; exit 1 on error-severity findings
- `arclint rules test`: run the rule fixtures under `.arclint/tests` after changing any rule
- `arclint sdk init`: regenerate the extension SDK artifacts under `.arclint/extensions`
- `arclint agents md --write`: refresh this block after changing rules.yaml or the vocabulary
- `arclint baseline`: manage the committed baseline of adopted findings
- `arclint patterns`: list the Patterns that resolve offline (embedded, vendored, authored); `patterns install <pattern>` extends rules.yaml with one, `patterns vendor` copies one under `.arclint/patterns`

### The recorded domain

4 contexts, 1 aggregates, 25 invariants (ubiquitous-language.yaml).

- **rule**: Rule [aggregate], Module, Pattern; value objects RuleID, ModuleName, Claim, Assertion, Severity, Language, PatternReference, Expansion, ExpansionSource, TermCase, CaseSpec
- **adoption**: value objects Binding, Override, Disablement, Exclusion, Suppression, Installation
- **conformance**: value objects Violation
- **distribution**: value objects Catalog, Digest, Index, Manifest, PatternFile, PatternSource, Registry, Selection, VendoredPattern

Relations: rule → conformance (conformist); rule → adoption (conformist); rule → distribution (conformist); distribution → adoption (conformist). Full text: `arclint domain`.

### Changing the language

If your change speaks about something new, or changes what a recorded term means, record it in `ubiquitous-language.yaml` before writing code. Invoke the domain-librarian skill for that work: it decides how a concept is classified, what evidence a recording needs, and when an open question is recorded instead of a guess. If your harness does not have the skill, `arclint agents skill` writes it to `.agents/skills/domain-librarian/`.

### Modules and their rules

- **vocabulary**: The project's recorded Ubiquitous Language vocabulary. (paths ubiquitous-language.yaml)
  - arclint/domain-model:vocabulary/terms-carry-definitions: Every recorded term carries a definition.
  - arclint/domain-model:vocabulary/invariants-name-recorded-owners: Every recorded invariant names a recorded term of its own context as its owner.
- **domain**: Rule aggregate and domain values; stdlib-only. (paths internal/domain/**)
  - imports no other module; external imports forbidden
  - rule-is-sole-aggregate: Rule is the only aggregate: it has a root, and no other aggregate root exists.
  - no-panic: Domain code never panics; a representation that cannot become a value is an error.
  - files-speak-the-vocabulary: Domain files are named for the concept they hold, never for a generic container.
  - errors-name-their-subject: Domain errors name their subject; a bare ErrNotFound or ErrInvalid is forbidden.
  - aggregate-skeleton (warning): Every recorded aggregate owns a home declaring its root and its Repository.
- **application**: Action-named use cases coordinating domain objects through ports. (paths internal/application/**)
  - imports only: domain; external imports forbidden
  - core-actions-present: The core use cases exist under their action names.
- **infrastructure**: Outbound technology adapters implementing inward-owned ports. (paths internal/infrastructure/**)
  - imports only: application, domain
  - stdlib-table-present: The Go language adapter embeds its generated stdlib table.
- **delivery**: CLI adapters for inbound command translation and outbound Report rendering. (paths internal/delivery/**)
  - imports only: application, domain
  - cli-seal-present: The CLI seal and the report seal are both complete.
- **cli_interface**: Framework-neutral CLI commands, reports, and adapter ports. (paths internal/delivery/cli/*.go)
  - imports only: application, domain; external imports forbidden
- **cli_factory**: Sealed CLI factory selecting an adapter by ArcLint-owned identity. (paths internal/delivery/cli/factory/**)
  - imports only: application, domain, cobra_adapter, delivery; external imports forbidden
- **cobra_adapter**: The only package permitted to import Cobra. (paths internal/delivery/cli/adapters/cobra/**)
  - imports only: application, domain, delivery
- **report_factory**: Sealed report factory selecting a renderer by ArcLint-owned identity. (paths internal/delivery/cli/reportfactory/**)
  - imports only: delivery, plain_report, json_report, lipgloss_report; external imports forbidden
- **plain_report**: Plain-text report renderer adapter. (paths internal/delivery/cli/adapters/report/plain/**)
  - imports only: delivery, application, domain; external imports forbidden
- **json_report**: JSON report renderer adapter. (paths internal/delivery/cli/adapters/report/json/**)
  - imports only: delivery, application, domain; external imports forbidden
- **lipgloss_report**: The only package permitted to import Lipgloss. (paths internal/delivery/cli/adapters/report/lipgloss/**)
  - imports only: delivery, application, domain
- **composition**: Composition roots selecting and connecting concrete adapters. (paths cmd/**)
  - imports only: delivery, infrastructure, application, domain, cli_factory, cobra_adapter, report_factory
  - main-present: The arclint binary has a main.
- **source**: Common source invariants for internal packages. (paths internal/**)
  - snake-case: Go file names use snake_case.
- **rule**: The rule bounded context: the Rule aggregate's home. (paths internal/domain/rule/**)
- **conformance**: The conformance bounded context, downstream conformist of rule. (paths internal/domain/conformance/**)

### Repository-wide rules

- arclint/domain-model:contexts/respect-relations (warning): Imports between context-named Modules respect the recorded context-map relations.
- dependencies/application-inward: Dependencies point inward: application, then domain.
- infrastructure/composition-only: Only composition imports infrastructure.
- delivery/cobra-factory-only: Only the CLI factory imports the Cobra adapter.
- delivery/plain-report-factory-only: Only the report factory imports the plain renderer.
- delivery/json-report-factory-only: Only the report factory imports the JSON renderer.
- delivery/lipgloss-report-factory-only: Only the report factory imports the Lipgloss renderer.
- dependencies/acyclic: Dependencies among the top-level Modules contain no cycle.

### Extension rules

`arclint/domain-model@0.1.0/extensions/domain_model_invariants_name_recorded_owners.ts` default-exports the rule definitions: domain-model/invariants-name-recorded-owners.
`arclint/domain-model@0.1.0/extensions/domain_model_require_defined_terms.ts` default-exports the rule definitions: domain-model/require-defined-terms.
`arclint/domain-model@0.1.0/extensions/domain_model_respect_context_relations.ts` default-exports the rule definitions: domain-model/respect-context-relations.
<!-- arclint:agents:end -->

## Finish gate

Before yielding a completed session or goal, run:

```bash
make check
```

or the same pair through mise:

```bash
mise run check
```

For read-only sessions (reviews, audits, anything that must not mutate
the tree), the gate is the non-mutating, network-free variant:

```bash
make check-ro
```
