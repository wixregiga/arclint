# boxoffice — agent guide

boxoffice is a small box office for live events ([README.md](README.md) tells the app's story), governed end to end by arclint.

## Orientation

IMPORTANT: ask arclint before reading around. `make arclint` builds the arclint binary at `.bin/arclint` and runs the check. The command surface, the contracts, and the recorded domain live in the architecture block at the bottom of this file.

What arclint cannot answer lives in exactly two other places: [README.md](README.md) for what the app is, and [requirements.ears](requirements.ears) for the domain invariants as EARS lines with their owning term in brackets. Keep requirements.ears in step with the vocabulary: a new invariant lands in both files or in neither.

## Adding a feature

0. **Ask first.** Run `.bin/arclint context` on the paths you expect to touch and `.bin/arclint domain`. You MUST do this before opening source files; the answers replace surveying the tree. Read only the files you will change.
1. **Language first.** If the feature speaks about something new, record the term in [ubiquitous-language.yaml](ubiquitous-language.yaml) before writing code: human voice, no em dashes, and anything object-level a definition mentions must itself be recorded or reworded in plain language. New invariants get an EARS line with their owner. Then run `.bin/arclint check`: a new aggregate makes the expansion rules derive the homes it is owed in both runtimes, and those findings are the structural to-do list.
2. **Place code where `arclint context` says it belongs.** The few things the contracts cannot express: features fetch and pass facts, aggregates decide — cross-aggregate rules live in the aggregate, fed values by the feature; reads do not get feature slices, the app layer serves them straight from entities; new domain refusals are mapped in `internal/app`'s `fail()` (404 unknown, 409 promise-cannot-be-kept-now, 422 never-valid); on the web, an aggregate's data model and calls live in one `web/src/shared/api/<kebab-case>.ts` file, and every feature or page slice exports its public API through `index.ts`.
3. **Rules move with structure — but slices are discovered.** New use-case, feature, and page slices are found from the tree (the slice-files rules), so adding one needs no rules.yaml edit; the yaml changes only for genuinely new structure or fences. Any new or changed rule gets fixture pairs (clean and violating) under `.arclint/tests`, proven by `.bin/arclint rules test`. Update the architecture block at the bottom of this file afterwards, by hand: read the note above its marker before reaching for `arclint agents md --write`. A new or changed vocabulary term lands in that block's recorded-domain line the same way.
4. **Gates.** `make check` runs vet, golangci-lint, the Go tests, `arclint check`, and `arclint rules test`. The web side: `make web` (install, typecheck, vite build); `make build-full` embeds the result in the single binary.

## Fixing a bug

0. **Ask first.** Run `.bin/arclint context` on the paths the symptom implicates. You MUST do this before reading around; then read only what you will change.
1. **Reproduce it as a test first**, at the lowest layer that shows it: a broken invariant is an entity test; wrong orchestration is a feature test with in-test fakes; wiring or HTTP behavior is `internal/app/app_test.go` through httptest with the fake clock. The failing test outlives the fix.
2. **Fix where the rule lives.** If a promise was broken, the enforcement belongs in the aggregate, never in a handler-side check.
3. **If the architecture allowed the bug, that is a second bug.** Tighten rules.yaml or the extensions and add the fixture that would have caught it.
4. `make check` green before done.

## Conventions the rules don't spell out

- Hold expiry is lazy: no timers, no janitors; `now` is passed into the domain, and the clock is injected in app.
- Ids are minted in app (`newID` for holds and orders, `slugify` for events); entities receive them.
- No messaging machinery, ever: contexts meet inside features as plain calls (see placeorder).
- The organizer gate is one bearer token from config; an empty token keeps that side locked.

## When arclint gets in your way

That is data, not an obstacle: this repo exists to surface it. Record the friction where you hit it (a comment in rules.yaml beside the workaround, like the extension-distribution note) and carry it to the parent repo's board. Never silently work around the tool this repo exists to prove.

<!-- The block below is hand-maintained. `arclint agents md --write`
     emits a strictly thinner version of it: modules with their import
     contracts and three query commands, dropping the per-module
     invariant rules, the recorded-domain summary, the repository-wide
     rules, the local extension list, and the "ask arclint first"
     directive this guide leans on. Regenerating therefore deletes
     guidance rather than refreshing it, so keep this block by hand
     until the generator carries the same content. Proving-ground
     finding for the agents-md track; carry it to the parent board. -->
<!-- arclint:agents:begin -->
## Architecture contracts (arclint)

Enforced from rules.yaml: 32 rules over languages [go, typescript].

### Ask arclint first

IMPORTANT: you MUST ask arclint before reading around. The architecture, the rules, and the recorded domain are queryable; run `arclint context` on the paths you expect to touch BEFORE opening source files, and do NOT learn the architecture by reading file after file or guessing from folder names.

- `arclint context [paths...]` — run before editing under any path: the owning modules, their import contracts, and the recorded domain in one answer (`--module <names>`, `--format json`)
- `arclint domain` — the ubiquitous language: contexts, aggregates, value objects, invariants, relations
- `arclint rules [selector]` — every configured rule with its claim; one match prints the complete rule
- `arclint check .` — evaluate every rule; the findings are your to-do list; exit 1 on error-severity findings
- `arclint rules test` — run the rule fixtures under `.arclint/tests` after changing any rule
- `arclint sdk init` — regenerate the extension SDK artifacts under `.arclint/extensions`
- `arclint agents md --write` — refresh this block after changing rules.yaml (see the note above it first)
- `arclint baseline` — manage the committed baseline of adopted findings
- `arclint patterns` — list available Pattern distribution packages

### The recorded domain

Three contexts, three aggregates. catalog: Event [aggregate], Organizer; TicketTier, Price. ordering: Order [aggregate]; OrderLine, Attendee, Refund; OrderPlaced. capacity: Capacity [aggregate]; Hold. Ten invariants. Relations: catalog upstream of ordering and of capacity (conformist); capacity supplies ordering (customer_supplier). Full text: `arclint domain`.

### Modules and their rules

- **app** — FSD app layer: chi router, handlers, DTOs, the organizer gate, and the in-memory repositories. (paths internal/app/**)
  - imports only: features, entities, shared
  - surface-tested: internal/app/app.go, app_test.go, and memory/memory.go stay present
- **composition** — Composition root: flags, wiring, the http server. (paths cmd/**)
  - imports only: app, features, entities, shared, web_embed
  - main-and-seed-present: cmd/boxoffice/main.go, seed.go, version_test.go stay present
- **entities** — FSD entities layer: the domain aggregates. Domain logic only, enforced. (paths internal/entities/**)
  - imports no other module; external imports forbidden
  - aggregate-slices (warning, derived from each recorded aggregate): `<slice>/<slice>.go`, `repository.go`, `<slice>_test.go` per aggregate
  - technology-free: no net/http, log/slog, or encoding/json
  - no-panic
  - errors-name-their-subject: no bare ErrNotFound/ErrInvalid/ErrFailed/ErrExists
  - deterministic: no time.Now or math/rand
  - aggregates-encapsulate: no exported fields on a recorded aggregate's struct
  - no-store-machinery: no "sync" import
- **features** — FSD features layer: use cases that change the world, one slice per use case. (paths internal/features/**)
  - imports only: entities; external imports forbidden
  - use-cases-tested: every discovered slice has `<slice>.go` and `<slice>_test.go`
  - technology-free · deterministic (same fences as entities)
- **server_source** — Source-wide invariants for the Go server. (paths internal/**)
  - slog-only: no fmt.Print or log.Print/Fatal/Panic
  - snake_case file names
- **shared** — FSD shared layer: kit the app layer builds on. (paths internal/shared/**)
  - imports no other module; external imports forbidden
- **toolchain** — The build and lint surfaces the repo promises to keep. (paths Makefile .golangci.yml go.mod web/package.json)
  - gates-present: all four files stay present
- **vocabulary** — The recorded Ubiquitous Language of the box office. (paths ubiquitous-language.yaml)
- **web_app** — FSD app layer on the web: router, providers, entry. (paths web/src/app/**)
  - imports only: web_pages, web_features, web_shared
- **web_embed** — The built web app carried into the single binary behind the embedweb tag. (paths web/*.go)
  - imports no other module; external imports forbidden
- **web_features** — FSD features layer: one slice per user interaction. (paths web/src/features/**)
  - imports only: web_shared
  - slices-export-public-api: every discovered slice has index.ts
- **web_pages** — FSD pages layer: one slice per screen. (paths web/src/pages/**)
  - imports only: web_features, web_shared
  - slices-export-public-api: every discovered slice has index.ts
- **web_shared** — FSD shared layer: the api client, per-aggregate api files, and the ui kit. (paths web/src/shared/**)
  - imports no other module
  - aggregates-speak-through-api (warning, derived from each recorded aggregate): `web/src/shared/api/<kebab-case>.ts` per aggregate

### Repository-wide rules

- fsd/slice-isolation: slices on the same layer never import each other, across internal/features, internal/entities, web/src/pages, web/src/features
- server layers: app → features → entities; a module never imports a higher layer
- web layers: web_app → web_pages → web_features → web_shared
- acyclic: no dependency cycles among the ten code modules

### Local extension rules

`.arclint/extensions/boxoffice.ts` default-exports the rule definitions: forbid-content, fsd-slice-isolation, slice-files, aggregate-encapsulation.
<!-- arclint:agents:end -->
