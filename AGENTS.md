# Agent guide

Repo-specific instructions for coding agents. The architecture block
below is generated from rules.yaml; refresh it with
`arclint agents --write` after changing the ruleset (a test fails when
it drifts). Add hand-written guidance outside the markers.

<!-- arclint:agents:begin -->
## Architecture contracts (arclint)

Enforced from rules.yaml: 24 rules over languages [go, typescript].

### Ask arclint first

IMPORTANT: you MUST ask arclint before reading around. The architecture, the rules, and the recorded domain are queryable; run `arclint context` on the paths you expect to touch BEFORE opening source files, and do NOT learn the architecture by reading file after file or guessing from folder names.

- `arclint context [paths...]` — run before editing under any path: the owning modules, their import contracts, and the recorded domain in one answer (`--module <names>`, `--format json`)
- `arclint domain` — the ubiquitous language: contexts, aggregates, value objects, invariants, relations
- `arclint rules [selector]` — every configured rule with its claim; one match prints the complete rule
- `arclint check .` — evaluate every rule; the findings are your to-do list; exit 1 on error-severity findings
- `arclint rules test` — run the rule fixtures under `.arclint/tests` after changing any rule
- `arclint sdk init` — regenerate the extension SDK artifacts under `.arclint/extensions`
- `arclint agents md --write` — refresh this block after changing rules.yaml or the vocabulary
- `arclint baseline` — manage the committed baseline of adopted findings
- `arclint patterns` — list available Pattern distribution packages

### The recorded domain

2 contexts, 1 aggregates, 4 invariants (ubiquitous-language.yaml).

- **rule**: Rule [aggregate]; value objects RuleID, Claim, Severity, Expansion, ExpansionSource, TermCase, CaseSpec
- **conformance**: value objects Violation

Relations: rule → conformance (conformist). Full text: `arclint domain`.

### Changing the language

If your change speaks about something new, or changes what a recorded term means, record it in `ubiquitous-language.yaml` before writing code. Invoke the domain-librarian skill for that work: it decides how a concept is classified, what evidence a recording needs, and when an open question is recorded instead of a guess. If your harness does not have the skill, `arclint agents skill` writes it to `.agents/skills/domain-librarian/`.

### Modules and their rules

- **application** — Action-named use cases coordinating domain objects through ports. (paths internal/application/**)
  - imports only: domain; external imports forbidden
  - core-actions-present: contains files matching ["internal/application/list_rules.go", "internal/application/assess_conformance.go", "internal/application/capture_baseline.go", "internal/application/list_patterns.go"]
- **cli_factory** — Sealed CLI factory selecting an adapter by ArcLint-owned identity. (paths internal/delivery/cli/factory/**)
  - imports only: application, domain, cobra_adapter, delivery; external imports forbidden
- **cobra_adapter** — The only package permitted to import Cobra. (paths internal/delivery/cli/adapters/cobra/**)
  - imports only: application, domain, delivery
- **composition** — Composition roots selecting and connecting concrete adapters. (paths cmd/**)
  - imports only: delivery, infrastructure, application, domain, cli_factory, cobra_adapter
  - main-present: contains files matching ["cmd/arclint/main.go"]
- **conformance** — The conformance bounded context, downstream conformist of rule. (paths internal/domain/conformance/**)
- **delivery** — Inbound adapters translating external input into application requests. (paths internal/delivery/**)
  - imports only: application, domain
  - cli-seal-present: contains files matching ["internal/delivery/cli/cli.go", "internal/delivery/cli/factory/factory.go", "internal/delivery/cli/adapters/cobra/cobra.go"]
- **domain** — Rule aggregate and domain values; stdlib-only. (paths internal/domain/**)
  - imports no other module; external imports forbidden
  - rule-is-sole-aggregate: contains files matching ["internal/domain/rule/root.go"] and contains no files matching ["internal/domain/architecture/**", "internal/domain/pattern/**", "internal/domain/baseline/root.go", "internal/domain/conformance/root.go"]
  - no-panic: satisfies extension rule "forbid-content" (pattern: \bpanic\()
  - files-speak-the-vocabulary: contains no files matching ["internal/domain/**/model.go", "internal/domain/**/types.go", "internal/domain/**/util.go", "internal/domain/**/utils.go", "internal/domain/**/helpers.go", "internal/domain/**/common.go"]
  - errors-name-their-subject: satisfies extension rule "forbid-content" (pattern: \bErr(NotFound|Invalid|Failed|Exists)\b)
  - aggregate-skeleton (warning): contains files matching ["internal/domain/rule/root.go", "internal/domain/rule/repository.go"] (derived from each recorded domain.aggregates)
- **infrastructure** — Outbound technology adapters implementing inward-owned ports. (paths internal/infrastructure/**)
  - imports only: application, domain
  - stdlib-table-present: contains files matching ["internal/infrastructure/language/golang/stdlib_gen.go"]
- **rule** — The rule bounded context: the Rule aggregate's home. (paths internal/domain/rule/**)
- **source** — Common source invariants for internal packages. (paths internal/**)
  - snake-case: file names use snake_case
- **vocabulary** — The project's recorded Ubiquitous Language vocabulary. (paths ubiquitous-language.yaml)
  - terms-carry-definitions: satisfies extension rule "require-defined-terms"
  - invariants-name-recorded-owners: satisfies extension rule "invariants-name-recorded-owners"

### Repository-wide rules

- domain-model/contexts-respect-relations (warning): satisfies extension rule "respect-context-relations"
- dependencies/application-inward: Modules layer highest first as ["application", "domain"]; a Module never imports a higher layer
- infrastructure/composition-only: Module "infrastructure" is imported only by ["composition"]
- delivery/cobra-factory-only: Module "cobra_adapter" is imported only by ["cli_factory", "composition"]
- dependencies/acyclic: dependencies among ["composition", "delivery", "infrastructure", "application", "domain"] contain no cycle

### Local extension rules

`.arclint/extensions/forbid_content.ts` default-exports the rule definitions: forbid-content.
`.arclint/extensions/invariants_name_recorded_owners.ts` default-exports the rule definitions: invariants-name-recorded-owners.
`.arclint/extensions/require_defined_terms.ts` default-exports the rule definitions: require-defined-terms.
`.arclint/extensions/respect_context_relations.ts` default-exports the rule definitions: respect-context-relations.
<!-- arclint:agents:end -->

## Finish gate

Before yielding a completed session or goal, run:

```bash
make check-fix
```

or the same pair through mise:

```bash
mise run check-fix
```

For read-only sessions (reviews, audits, anything that must not mutate
the tree), the gate is the non-mutating, network-free variant:

```bash
make check-ro
```
