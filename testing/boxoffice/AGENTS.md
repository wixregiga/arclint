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

Enforced from rules.yaml: 33 rules over languages [go, typescript].

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

3 contexts, 3 aggregates, 21 invariants (ubiquitous-language.yaml).

- **catalog**: Event [aggregate], Organizer; value objects TicketTier, Price
- **ordering**: Order [aggregate]; value objects OrderLine, Attendee, Refund; events OrderPlaced
- **capacity**: Capacity [aggregate]; value objects Hold

Relations: catalog → ordering (conformist); catalog → capacity (conformist); capacity → ordering (customer_supplier). Full text: `arclint domain`.

### Changing the language

If your change speaks about something new, or changes what a recorded term means, record it in `ubiquitous-language.yaml` before writing code. Invoke the domain-librarian skill for that work: it decides how a concept is classified, what evidence a recording needs, and when an open question is recorded instead of a guess. If your harness does not have the skill, `arclint agents skill` writes it to `.agents/skills/domain-librarian/`.

### Modules and their rules

- **composition** — Composition root: flags, wiring, the http server. (paths cmd/**)
  - imports only: app, features, entities, shared, web_embed
  - main-and-seed-present: The boxoffice binary has its main, its seed, and its version test.
- **app** — FSD app layer: chi router, handlers, DTOs, the organizer gate, and the in-memory repositories. (paths internal/app/**)
  - imports only: features, entities, shared
  - surface-tested: The app surface and its memory repositories exist with their tests.
- **features** — FSD features layer: use cases that change the world, one slice per use case, technology-free. (paths internal/features/**)
  - imports only: entities; external imports forbidden
  - use-cases-tested: Every use-case slice carries its named file and its tests. (require: [{slice}.go, {slice}_test.go], root: internal/features)
  - technology-free: Use cases name no transport, logging, or JSON package.
  - deterministic: Use cases never read the clock or roll dice.
- **entities** — FSD entities layer: the domain aggregates. Domain logic only, enforced. (paths internal/entities/**)
  - imports no other module; external imports forbidden
  - contracts-visible: Every aggregate's invariants are visible through its cluster method.
  - aggregate-slices (warning): Every recorded aggregate owns a slice with its file, its repository interface, and its tests.
  - technology-free: The entities layer names no transport, logging, or JSON package.
  - no-panic: The entities layer never panics.
  - errors-name-their-subject: Entity errors name their subject; a bare ErrNotFound or ErrInvalid is forbidden.
  - deterministic: The entities layer never reads the clock or rolls dice.
  - aggregates-encapsulate: The struct of every recorded aggregate has no exported fields. (root: internal/entities)
  - no-store-machinery: The entities layer imports no sync machinery.
- **shared** — FSD shared layer: kit the app layer builds on. (paths internal/shared/**)
  - imports no other module; external imports forbidden
- **server_source** — Source-wide invariants for the Go server. (paths internal/**)
  - slog-only: The server logs through slog only.
  - snake-case: Go file names use snake_case.
- **web_embed** — The built web app carried into the single binary behind the embedweb tag. (paths web/*.go)
  - imports no other module; external imports forbidden
- **web_app** — FSD app layer on the web: router, providers, entry. (paths web/src/app/**)
  - imports only: web_pages, web_features, web_shared
- **web_pages** — FSD pages layer: one slice per screen. (paths web/src/pages/**)
  - imports only: web_features, web_shared
  - slices-export-public-api: Every web page slice exports a public API through index.ts. (require: [index.ts], root: web/src/pages)
- **web_features** — FSD features layer: one slice per user interaction. (paths web/src/features/**)
  - imports only: web_shared
  - slices-export-public-api: Every web feature slice exports a public API through index.ts. (require: [index.ts], root: web/src/features)
- **web_shared** — FSD shared layer: the api client, per-aggregate api files, and the ui kit. (paths web/src/shared/**)
  - imports no other module
  - aggregates-speak-through-api (warning): Every recorded aggregate owns one api file in the web shared layer.
- **vocabulary** — The recorded Ubiquitous Language of the box office. (paths ubiquitous-language.yaml)
- **toolchain** — The build and lint surfaces the repo promises to keep. (paths Makefile .golangci.yml go.mod web/package.json)
  - gates-present: The build and lint gates the repo promises are present.

### Repository-wide rules

- fsd/slice-isolation: Sibling slices within one FSD layer never import each other. (layers: [internal/features, internal/entities, web/src/pages, web/src/features])
- dependencies/server-layers: Server dependencies point inward: app, then features, then entities.
- dependencies/web-layers: Web dependencies point inward: app, then pages, then features, then shared.
- dependencies/acyclic: Dependencies among the layer Modules contain no cycle.

### Extension rules

`.arclint/extensions/boxoffice.ts` default-exports the rule definitions: fsd-slice-isolation, slice-files, aggregate-encapsulation.
<!-- arclint:agents:end -->
