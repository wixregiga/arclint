# boxoffice — agent guide

boxoffice is a small box office for live events, governed end to end by arclint.

## Orientation

IMPORTANT: ask arclint before reading around. `make arclint` builds the arclint binary at `.bin/arclint` and runs the check. The command surface, the contracts, and the recorded domain live in the architecture block at the bottom of this file.

## Conventions the rules don't spell out

- Hold expiry is lazy: no timers, no janitors; `now` is passed into the domain, and the clock is injected in app.
- Ids are minted in app (`newID` for holds and orders, `slugify` for events); entities receive them.
- No messaging machinery, ever: contexts meet inside features as plain calls (see placeorder).
- The organizer gate is one bearer token from config; an empty token keeps that side locked.

<!-- arclint:agents:begin -->
## Architecture contracts (arclint)

Enforced from rules.arclint.yaml: 52 rules over languages [go, typescript].

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

3 contexts, 3 aggregates, 6 value objects, 16 invariants (domain.arclint.yaml).

- **catalog**: aggregates Event; value objects TicketTier, Price
- **ordering**: aggregates Order; value objects OrderLine, Attendee, Refund
- **capacity**: aggregates Capacity; value objects Hold

Relations: catalog → ordering (conformist); catalog → capacity (conformist); capacity → ordering (customer_supplier). Full text: `arclint domain`.

### Changing the language

If your change speaks about something new, or changes what a recorded term means, record it in `domain.arclint.yaml` before writing code. Invoke the domain-librarian skill for that work: it decides how a concept is classified, what evidence a recording needs, and when an open question is recorded instead of a guess. If your harness does not have the skill, `arclint agents skill` writes it to `.agents/skills/domain-librarian/`.

### Zones and their rules

- **composition**: Composition root: flags, wiring, the http server. (paths cmd/**)
  - imports only: app, features, entities, shared, web_embed
  - main-and-seed-present: contains files matching ["cmd/boxoffice/main.go", "cmd/boxoffice/seed.go", "cmd/boxoffice/version_test.go"] Rationale: The boxoffice binary has its main, its seed, and its version test.
- **app**: FSD app layer: chi router, handlers, DTOs, the organizer gate, and the in-memory repositories. (paths internal/app/**)
  - imports only: features, entities, shared
  - surface-tested: contains files matching ["internal/app/app.go", "internal/app/app_test.go", "internal/app/memory/memory.go"] Rationale: The app surface and its memory repositories exist with their tests.
- **features**: FSD features layer: use cases that change the world, one slice per use case, technology-free. (paths internal/features/**)
  - imports only: entities; external imports forbidden
  - use-cases-tested: satisfies extension rule "slice-files" (require: [{slice}.go, {slice}_test.go], root: internal/features) Rationale: Every use-case slice carries its named file and its tests.
  - technology-free: contains no line matching /"net/http"|"log/slog"|"encoding/json"/ Rationale: Use cases name no transport, logging, or JSON package.
  - deterministic: contains no line matching /time\.Now\(|math/rand/ Rationale: Use cases never read the clock or roll dice.
- **entities**: FSD entities layer: the domain aggregates. Domain logic only, enforced. (paths internal/entities/**)
  - imports no other zone; external imports forbidden
  - aggregate-slices (warning): contains files matching ["internal/entities/event/event.go", "internal/entities/event/repository.go", "internal/entities/event/event_test.go", "internal/entities/order/order.go", "internal/entities/order/repository.go", "internal/entities/order/order_test.go", "internal/entities/capacity/capacity.go", "internal/entities/capacity/repository.go", "internal/entities/capacity/capacity_test.go"] (derived from each recorded domain.aggregates) Rationale: Every recorded aggregate owns a slice with its file, its repository interface, and its tests.
  - technology-free: contains no line matching /"net/http"|"log/slog"|"encoding/json"/ Rationale: The entities layer names no transport, logging, or JSON package.
  - no-panic: contains no line matching /\bpanic\(/ Rationale: The entities layer never panics.
  - errors-name-their-subject: contains no line matching /\bErr(NotFound|Invalid|Failed|Exists)\b/ Rationale: Entity errors name their subject; a bare ErrNotFound or ErrInvalid is forbidden.
  - deterministic: contains no line matching /time\.Now\(|math/rand/ Rationale: The entities layer never reads the clock or rolls dice.
  - aggregates-encapsulate: satisfies extension rule "aggregate-encapsulation" (root: internal/entities) Rationale: The struct of every recorded aggregate has no exported fields.
  - no-store-machinery: contains no line matching /"sync"/ Rationale: The entities layer imports no sync machinery.
- **shared**: FSD shared layer: kit the app layer builds on. (paths internal/shared/**)
  - imports no other zone; external imports forbidden
- **server_source**: Source-wide invariants for the Go server. (paths internal/**)
  - slog-only: contains no line matching /\bfmt\.Print|\blog\.(Print|Fatal|Panic)/ Rationale: The server logs through slog only.
  - snake-case: file names use snake_case Rationale: Go file names use snake_case.
- **web_embed**: The built web app carried into the single binary behind the embedweb tag. (paths web/*.go)
  - imports no other zone; external imports forbidden
- **web_app**: FSD app layer on the web: router, providers, entry. (paths web/src/app/**)
  - imports only: web_pages, web_features, web_shared
- **web_pages**: FSD pages layer: one slice per screen. (paths web/src/pages/**)
  - imports only: web_features, web_shared
  - slices-export-public-api: satisfies extension rule "slice-files" (require: [index.ts], root: web/src/pages) Rationale: Every web page slice exports a public API through index.ts.
- **web_features**: FSD features layer: one slice per user interaction. (paths web/src/features/**)
  - imports only: web_shared
  - slices-export-public-api: satisfies extension rule "slice-files" (require: [index.ts], root: web/src/features) Rationale: Every web feature slice exports a public API through index.ts.
- **web_shared**: FSD shared layer: the api client, per-aggregate api files, and the ui kit. (paths web/src/shared/**)
  - imports no other zone
  - aggregates-speak-through-api (warning): contains files matching ["web/src/shared/api/event.ts", "web/src/shared/api/order.ts", "web/src/shared/api/capacity.ts"] (derived from each recorded domain.aggregates) Rationale: Every recorded aggregate owns one api file in the web shared layer.
- **vocabulary**: The recorded Ubiquitous Language of the box office. (paths domain.arclint.yaml)
- **toolchain**: The build and lint surfaces the repo promises to keep. (paths Makefile .golangci.yml go.mod web/package.json)
  - gates-present: contains files matching ["Makefile", ".golangci.yml", "go.mod", "web/package.json"] Rationale: The build and lint gates the repo promises are present.

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

- fsd/slice-isolation: satisfies extension rule "fsd-slice-isolation" (layers: [internal/features, internal/entities, web/src/pages, web/src/features]) Rationale: Sibling slices within one FSD layer never import each other.
- dependencies/server-layers: Zones layer highest first as ["app", "features", "entities"]; a Zone never imports a higher layer Rationale: Server dependencies point inward: app, then features, then entities.
- dependencies/web-layers: Zones layer highest first as ["web_app", "web_pages", "web_features", "web_shared"]; a Zone never imports a higher layer Rationale: Web dependencies point inward: app, then pages, then features, then shared.
- dependencies/acyclic: dependencies among ["composition", "app", "features", "entities", "shared", "web_embed", "web_app", "web_pages", "web_features", "web_shared"] contain no cycle Rationale: Dependencies among the layer Zones contain no cycle.

### Extension rules

`.arclint/extensions/boxoffice.ts` default-exports the rule definitions: fsd-slice-isolation, slice-files, aggregate-encapsulation.
<!-- arclint:agents:end -->
