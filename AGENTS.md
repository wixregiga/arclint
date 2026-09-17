# Agent guide

Repo-specific instructions for coding agents. The architecture block
below is generated from rules.arclint.yaml; refresh it with
`arclint agents --write` after changing the ruleset (a test fails when
it drifts). Add hand-written guidance outside the markers.

<!-- arclint:agents:begin -->
## Architecture contracts (arclint)

Enforced from rules.arclint.yaml: 60 rules over languages [go, typescript].

### Ask arclint first

IMPORTANT: you MUST ask arclint before reading around. The architecture, the rules, and the recorded domain are queryable; run `arclint context` on the paths you expect to touch BEFORE opening source files, and do NOT learn the architecture by reading file after file or guessing from folder names.

- `arclint context [paths...]`: run before editing under any path: the owning zones, their import contracts, and the recorded domain in one answer (`--zone <names>`, `--format json`)
- `arclint domain`: the ubiquitous language: contexts, aggregates, value objects, invariants, relations
- `arclint rules [selector]`: every configured rule with its constraint and rationale; one match prints the complete rule
- `arclint check .`: evaluate every rule; the findings are your to-do list; exit 1 on error-severity findings
- `arclint rules test`: run the rule fixtures under `.arclint/tests` after changing any rule
- `arclint sdk init`: regenerate the extension SDK artifacts under `.arclint/extensions`
- `arclint agents md --write`: refresh this block after changing rules.arclint.yaml or the vocabulary
- `arclint baseline`: manage the committed baseline of adopted findings
- `arclint patterns`: list the Patterns that resolve offline (embedded, vendored, authored); `patterns install <pattern>` extends rules.arclint.yaml with one, `patterns vendor` copies one under `.arclint/patterns`

### The recorded domain

5 contexts, 1 aggregates, 30 value objects, 37 invariants (domain.arclint.yaml).

- **vocabulary**
- **rule**: aggregates Rule (Zone, Pattern); value objects RuleID, ZoneName, Rationale, Constraint, Scope, Severity, Language, PatternReference, Expansion, ExpansionSource, TermCase, CaseSpec
- **adoption**: value objects Binding, Override, Disablement, Exclusion, Suppression, Installation
- **conformance**: value objects DependencyImport, Facts, Violation
- **distribution**: value objects Catalog, Digest, Index, Manifest, PatternFile, PatternSource, Registry, Selection, VendoredPattern

Relations: vocabulary → rule (conformist); vocabulary → conformance (conformist); rule → conformance (conformist); rule → adoption (conformist); rule → distribution (conformist); distribution → adoption (conformist). Full text: `arclint domain`.

### Changing the language

If your change speaks about something new, or changes what a recorded term means, record it in `domain.arclint.yaml` before writing code. Invoke the domain-librarian skill for that work: it decides how a concept is classified, what evidence a recording needs, and when an open question is recorded instead of a guess. If your harness does not have the skill, `arclint agents skill` writes it to `.agents/skills/domain-librarian/`.

### Zones and their rules

- **domain**: Rule aggregate and domain values; stdlib-only. (paths internal/domain/**)
  - imports no other zone; external imports forbidden
  - rule-is-sole-aggregate: contains files matching ["internal/domain/rule/root.go"] and contains no files matching ["internal/domain/architecture/**", "internal/domain/pattern/**", "internal/domain/baseline/root.go", "internal/domain/conformance/root.go", "internal/domain/distribution/root.go"] Rationale: Rule is the only aggregate: it has a root, and no other aggregate root exists.
  - no-panic: contains no line matching /\bpanic\(/ Rationale: Domain code never panics; a representation that cannot become a value is an error.
  - files-speak-the-vocabulary: contains no files matching ["internal/domain/**/model.go", "internal/domain/**/types.go", "internal/domain/**/util.go", "internal/domain/**/utils.go", "internal/domain/**/helpers.go", "internal/domain/**/common.go"] Rationale: Domain files are named for the concept they hold, never for a generic container.
  - errors-name-their-subject: contains no line matching /\bErr(NotFound|Invalid|Failed|Exists)\b/ Rationale: Domain errors name their subject; a bare ErrNotFound or ErrInvalid is forbidden.
  - aggregate-skeleton (warning): contains files matching ["internal/domain/rule/root.go", "internal/domain/rule/repository.go"] (derived from each recorded domain.aggregates) Rationale: Every recorded aggregate owns a home declaring its root and its Repository.
- **application**: Action-named use cases coordinating domain objects through ports. (paths internal/application/**)
  - imports only: domain; external imports forbidden
  - core-actions-present: contains files matching ["internal/application/list_rules.go", "internal/application/assess_conformance.go", "internal/application/capture_baseline.go", "internal/application/list_patterns.go"] Rationale: The core use cases exist under their action names.
- **infrastructure**: Outbound technology adapters implementing inward-owned ports. (paths internal/infrastructure/**)
  - imports only: application, domain
  - stdlib-table-present: contains files matching ["internal/infrastructure/language/golang/stdlib_gen.go"] Rationale: The Go language adapter embeds its generated stdlib table.
- **delivery**: CLI adapters for inbound command translation and outbound Report rendering. (paths internal/delivery/**)
  - imports only: application, domain
  - cli-seal-present: contains files matching ["internal/delivery/cli/cli.go", "internal/delivery/cli/factory/factory.go", "internal/delivery/cli/adapters/cobra/cobra.go", "internal/delivery/cli/report.go", "internal/delivery/cli/reportfactory/factory.go", "internal/delivery/cli/adapters/report/plain/plain.go", "internal/delivery/cli/adapters/report/json/json.go", "internal/delivery/cli/adapters/report/lipgloss/lipgloss.go"] Rationale: The CLI seal and the report seal are both complete.
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
  - main-present: contains files matching ["cmd/arclint/main.go"] Rationale: The arclint binary has a main.
- **source**: Common source invariants for internal packages. (paths internal/**)
  - snake-case: file names use snake_case Rationale: Go file names use snake_case.
- **vocabulary**: The vocabulary bounded context: the recorded Ubiquitous Language and the meta-model it is checked against. (paths internal/domain/vocab/**)
- **rule**: The rule bounded context: the Rule aggregate's home. (paths internal/domain/rule/**)
- **conformance**: The conformance bounded context, downstream conformist of rule. (paths internal/domain/conformance/**)
- **distribution**: The distribution bounded context: Patterns travelling between repositories. (paths internal/domain/distribution/**)
- **web**: Web frontend package, build configuration, and public assets. (paths web/**)
  - launch-surfaces-present: contains files matching ["web/package.json", "web/vite.config.ts", "web/index.html", "web/public/manifest.webmanifest", "web/src/app/entrypoints/start-web.ts", "web/src/app/entrypoints/mount-web.ts"] Rationale: Web has separate standalone startup and host-controlled mounting entrypoints, plus an authored web app manifest; presence alone proves no runtime behavior.
- **web_source**: Browser-portable Web source, organized by FSD responsibility. (paths web/src/**)
  - imports no other zone; stdlib imports forbidden
  - source-layout: satisfies extension rule "web-source-layout" Rationale: A reader can find code by FSD layer, slice, and purposeful segment; implementation is not placed beside a slice's public interface.
  - slices-are-independent: satisfies extension rule "web-slice-isolation" Rationale: Sibling slices remain independently understandable; compose their behavior in a higher layer. Native independent skips Zone-owned folders, so this rule uses resolved imports in the selected Web source.
  - public-interfaces: satisfies extension rule "web-public-interfaces" Rationale: Other modules depend on a slice or Shared module's explicit public entry, while its own implementation imports local files directly.
  - files-speak-their-purpose: contains no files matching ["web/src/**/model.ts", "web/src/**/model.tsx", "web/src/**/types.ts", "web/src/**/types.tsx", "web/src/**/util.ts", "web/src/**/util.tsx", "web/src/**/utils.ts", "web/src/**/utils.tsx", "web/src/**/helper.ts", "web/src/**/helper.tsx", "web/src/**/helpers.ts", "web/src/**/helpers.tsx", "web/src/**/common.ts", "web/src/**/common.tsx", "web/src/**/constants.ts", "web/src/**/constants.tsx", "web/src/**/data.ts", "web/src/**/data.tsx", "web/src/**/service.ts", "web/src/**/service.tsx", "web/src/**/services.ts", "web/src/**/services.tsx", "web/src/**/component.ts", "web/src/**/component.tsx", "web/src/**/components.ts", "web/src/**/components.tsx", "web/src/**/hooks.ts", "web/src/**/hooks.tsx", "web/src/**/components/**", "web/src/**/hooks/**", "web/src/**/types/**", "web/src/**/utils/**", "web/src/**/helpers/**", "web/src/**/common/**", "web/src/**/services/**", "web/src/**/index.tsx", "web/src/**/index.test.ts", "web/src/**/index.spec.ts"] Rationale: Implementation filenames identify the concept or action they hold; index.ts is reserved for a public interface, and model remains a segment name rather than a generic file.
  - typescript-source: contains no files matching ["web/src/**/*.js", "web/src/**/*.jsx", "web/src/**/*.mjs", "web/src/**/*.cjs", "web/src/**/*.mts", "web/src/**/*.cts"] Rationale: Web executable source uses the TypeScript and TSX files ArcLint observes; JavaScript and alternate module extensions must not silently escape dependency checks.
  - explicit-public-exports: contains no line matching /^\s*export\s+(type\s+)?\*/ Rationale: A public interface names its exports explicitly. This line check catches wildcard export statements; it does not judge semantic cohesion.
- **web_app**: Web composition, routes, providers, host adapters, and explicit launch entrypoints. (paths web/src/app/**)
- **web_pages**: Screen slices owning their presentation and screen-specific behavior. (paths web/src/pages/**)
- **web_widgets**: Optional reusable screen compositions; introduce only when pages or features do not own them. (paths web/src/widgets/**)
- **web_features**: Reusable user-action slices named for the behavior they provide. (paths web/src/features/**)
- **web_entities**: Reusable Web concepts; an FSD entity slice does not classify a DDD Entity. (paths web/src/entities/**)
- **web_shared**: Purpose-named browser libraries, UI primitives, transport contracts, and configuration without project-domain policy. (paths web/src/shared/**)
- **web_standalone**: Standalone browser startup; chooses the mount target and opts into PWA lifecycle. (paths web/src/app/entrypoints/start-web.ts)
- **web_pwa**: Standalone PWA registration, updates, and worker lifecycle; never implicit in an embedded mount. (paths web/src/app/pwa/**)

### Built-in rules

20 rules built in from the DDD meta-model judge the recorded domain against the code; no Pattern distributes them. Change one through an Override under its id in rules.arclint.yaml (severity, or disable with a reason).

- ubiquitous_language/terms-declared-in-code: Every recorded member entity, value object, and identity names one type declaration in the context's code, spelled with the recorded name in the language's type case (in Go, a name the package's own name completes, so rule.ID spells RuleID); a name no declaration spells, or one that two declarations spell with nothing to choose between them, is a finding. Aggregates, events, services, specifications, repositories, and factories state the same for themselves.
- bounded_context/code-held-by-one-context: A file of a bounded context's code belongs to no other context; two contexts hold the same code only under a recorded shared_kernel relation between them, because a boundary two models straddle is not a boundary.
- bounded_context/isolated: Code of one bounded context imports code of another only along a recorded relation between the two; an import between contexts the context map does not relate is a finding.
- context_relation/imports-follow-influence: Under a one-way kind (customer_supplier, conformist, anticorruption_layer, open_host_service, published_language) only the downstream context's code imports the upstream's; under partnership or shared_kernel both may import each other; under separate_ways neither imports the other.
- domain_isolation/model-imports-nothing-outside-itself: Code of a bounded context imports no code of a declared Zone that no context holds; whatever the architecture, dependencies run toward the model and never out of it. Without a declared Zone there is no outside to judge, and the invariant does not apply.
- value_object/constructed-through-one-door: A value object with a recorded invariant declares a constructor: a function of its module that returns it, or a constructor or __init__ of its class, or a factory method on it named create, from, of, parse, new, or build; the invariant is enforced there, so a value that violates it never exists.
- value_object/no-setters: A value object declares no method whose name begins with set; nothing changes a value after construction.
- invariant/enforced-at-every-mutation: Every constructor and every command of the root calls the ensure method of the aggregate's invariant, so the invariant is evaluated at the completion of every state-mutating operation and a violation fails the operation; a root with no constructor has no door at which to enforce it.
- assertion/checked-by-its-operation: The root declares both the operation and the checking method (assert followed by the key in the language's method case), and the operation calls the checking method.
- specification/satisfaction-method: A specification is a declared type carrying a satisfaction method (SatisfiedBy, satisfiedBy, or satisfied_by).
- aggregate/protects-an-invariant (warning): An aggregate records at least one invariant; a boundary drawn around nothing that must stay consistent is not yet justified, and the finding asks whether the entry is an aggregate or a value.
- aggregate/root-declared: The root is one declared type spelled with the aggregate's name in the language's type case, found in the context's code; a type that can carry behaviour (a struct or named type in Go, a class in TypeScript and Python) is the root ahead of an interface or alias of the same name, two such declarations with nothing to choose between them are a finding, and none is a finding at the recording.
- aggregate/invariants-enforced-by-root: For every recorded invariant of the aggregate the root declares the method that enforces it, ensure followed by the key in the language's method case (EnsureLinesFrozen, ensureLinesFrozen, ensure_lines_frozen); the enforcement lives on the root and nowhere else.
- aggregate/commands-named-for-behavior (warning): The root declares no method whose name begins with set; a change of state is a command named for what it does in the ubiquitous language, and a root made of setters is an anemic model whose rules live somewhere else.
- aggregate/references-by-identity: No constructor or command of one aggregate's root takes or returns another aggregate's root; what one aggregate needs of another it receives as an identity or a value. Field types are not yet observed, so a root held in a field is not seen, and an import alias hides a root of another package.
- domain_event/declared: A recorded event is a declared type spelled with the event's name in the language's type case.
- domain_event/no-setters: An event declares no method whose name begins with set; a record of the past is not edited.
- domain_service/declared: A recorded service is a declared type spelled with the service's name in the language's type case.
- repository/declared: A recorded repository is a declared type (interface, or class in a language without interfaces) spelled with the recorded name in the language's type case, declared in the context's code.
- factory/declared: A recorded factory is a declared type or function spelled with the recorded name in the language's case, declared in the context's code.

### Repository-wide rules

- web/layers-point-downward: Zones layer highest first as ["web_app", "web_pages", "web_widgets", "web_features", "web_entities", "web_shared"]; a Zone never imports a higher layer Rationale: Web dependencies run from app through pages, optional widgets, features, entities, and shared; lower layers never import higher layers.
- web/standalone-is-an-entrypoint: Zone "web_standalone" is imported by no other Zone Rationale: No module imports standalone startup; a host invokes mount-web without importing browser auto-start code.
- web/pwa-lifecycle-is-opt-in: Zone "web_pwa" is imported only by ["web_standalone"] Rationale: Only standalone startup imports the PWA lifecycle; embedding Web must not implicitly install a service worker in the host's origin.
- dependencies/application-inward: Zones layer highest first as ["application", "domain"]; a Zone never imports a higher layer Rationale: Dependencies point inward: application, then domain.
- infrastructure/composition-only: Zone "infrastructure" is imported only by ["composition"] Rationale: Only composition imports infrastructure.
- delivery/cobra-factory-only: Zone "cobra_adapter" is imported only by ["cli_factory"] Rationale: Only the CLI factory imports the Cobra adapter.
- delivery/plain-report-factory-only: Zone "plain_report" is imported only by ["report_factory"] Rationale: Only the report factory imports the plain renderer.
- delivery/json-report-factory-only: Zone "json_report" is imported only by ["report_factory"] Rationale: Only the report factory imports the JSON renderer.
- delivery/lipgloss-report-factory-only: Zone "lipgloss_report" is imported only by ["report_factory"] Rationale: Only the report factory imports the Lipgloss renderer.
- dependencies/acyclic: dependencies among ["composition", "delivery", "infrastructure", "application", "domain"] contain no cycle Rationale: Dependencies among the top-level Zones contain no cycle.

### Extension rules

`.arclint/extensions/web_boundaries.ts` default-exports the rule definitions: web-source-layout, web-slice-isolation, web-public-interfaces.
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
