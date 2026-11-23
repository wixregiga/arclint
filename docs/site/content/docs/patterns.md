+++
title = "Patterns"
description = "Distribute Rules as a versioned Pattern; adopt one with install, extends, bind, and Overrides; vendor it so a check never needs the network."
weight = 5
+++

A **Pattern** is a named, versioned collection of Rules packaged for
distribution: an identity header, the Zones its Rules speak about
(without paths), the Rules, and any Extensions those Rules use. A
repository adopts a Pattern by reference from `rules.arclint.yaml`, binds each
Pattern Zone to the paths it owns locally, and adjusts individual
Rules through Overrides. Rule text is never copied; when a folder moves,
only the binding changes.

Everything a Pattern needs ships with the Pattern. The rules that
enforce a DDD team's context map, a vertical-slice layout, or a
hexagonal core come from the Pattern the team extends, not from files
under its own `.arclint/` directory, so what the author tested is
exactly what the adopter runs.

## Where Patterns come from

A Pattern resolves from three places, always in this order:

1. **Embedded**: the Patterns built into the `arclint` binary
   (`arclint/hexagonal` and `arclint/vertical`, described under
   [Built-in Patterns](#built-in-patterns)). They need no files in the
   repository and no network.
2. **Local**: `.arclint/patterns/<namespace>/<name>/`. A directory with
   a `manifest.json` is a **vendored** copy of a published version,
   verified byte for byte on every load. A directory without one is
   **authored** in place: the Pattern you are writing.
3. **Registry**: a static tree of published Patterns reachable by URL.
   The default is arclint's own registry
   (`https://raw.githubusercontent.com/wixregiga/arclint-pattern-registry/main`);
   `--registry` or `ARCLINT_REGISTRY` names another, including a
   `file://` tree on disk.

`arclint check` resolves through the first two only. A Registry is read
by `arclint patterns --remote`, `vendor`, and `install`, and only for a
Pattern that resolves nowhere offline. A repository that extends an
embedded or vendored Pattern therefore checks cleanly on a machine with
no network at all.

## Listing what is available

```bash
arclint patterns
```

```
arclint/hexagonal@0.1.0-beta.2  embedded            10 rule(s)  1 extension(s)  coverage [go, ts, py]  21ff05a03039
arclint/vertical@0.1.0          embedded, vendored  16 rule(s)  5 extension(s)  coverage [go]          ddf0767e51f9
acme/layers@1.0.0               authored             1 rule(s)  0 extension(s)  coverage [go]          3fb2cbda1af6
```

The second column says where a Pattern resolves from and what the
repository carries of it: `embedded`, `vendored`, `authored`, or
`embedded, vendored` when the repository keeps its own verified copy of
a built-in. The last column is the short digest that names exactly the
files a copy is verified against; two copies of one published version
always show the same digest.

```bash
arclint patterns --remote
arclint patterns --remote --registry file:///srv/patterns
```

lists what a Registry publishes instead, from its index alone; nothing
is fetched.

## Built-in Patterns

### arclint/hexagonal

Ports and adapters around an isolated core (domain and application), for Go, TypeScript, and Python. Inbound (driving) adapters call the application, outbound (driven) adapters implement ports, shared serves adapters, and composition sits outermost to wire everything together.

| Zone | Holds | Boundary (rule) |
|---|---|---|
| `domain` | aggregates, entities, value objects, domain services, aggregate repository interfaces | stdlib only, no other zone or third-party package (`domain/depends-on-nothing-outside`) |
| `application` | use cases and non-domain outbound ports they call | `domain` only (`application/through-ports`); no third-party package (`application/core-libraries-only`) |
| `inbound` | front doors calling the app (Cockburn: primary / driving): HTTP handlers, CLI commands, route files, consumers | `application`, `domain`, `shared`; never `outbound` or `composition` (`inbound/calls-the-application`) |
| `outbound` | back-end tools called by the app (Cockburn: secondary / driven): databases, clients, brokers | `application`, `domain`, `shared`; never `inbound` or `composition` (`outbound/implements-ports`) |
| `shared` | logging, tracing, generated contracts for adapters | no other zone (`shared/product-free`) |
| `composition` | entry points (Composition Root / Configurator) | any zone |

Four additional rules guard the core and overall graph:

- `core/performs-no-io` (`hexagonal/forbidden-modules`): `domain` and `application` import no standard-library module for files, processes, networks, or databases. Lists are per language and cover submodules (`os` forbids `os/exec`; TypeScript's `node:` prefix is stripped). Go's `net/url`, `net/netip`, `net/mail`, and `io` stay allowed; globals such as `fetch` are not imports and are not judged.
- `domain/no-wire-format` (`hexagonal/wire-format`): `domain` imports no standard-library encoder and declares no Go struct field tag such as `json:"id"`.
- `core/concept-named-files`: no `domain` or `application` file is named `util`, `utils`, `helper`, `helpers`, `common`, or `misc` (`.go`, `.ts`, `.tsx`, `.py`).
- `dependencies/acyclic`: no import cycle among the six zones.

**Sources and conventions.** The inside/outside split and actor-direction rule come from Alistair Cockburn's *"The Hexagonal (Ports & Adapters) Architecture"* (2005, `https://alistair.cockburn.us/hexagonal-architecture/`): *"code pertaining to the inside part should not leak into the outside part"*, where primary/driving (`inbound`) actors trigger the application and secondary/driven (`outbound`) actors are triggered by the application. Keeping `composition` outermost as the startup configurator follows Composition Root (Robert C. Martin, *Clean Architecture*; Juan Manuel Garrido de Paz, *"Ports and Adapters Pattern"*). Splitting the inside into `domain` and `application` and placing Aggregate Repository interfaces in `domain` (so both Domain Services and Application use cases can depend on them while `outbound` implements them) comes from Domain-Driven Design (Eric Evans, *DDD Reference*, 2015, pp. 10, 17; Vaughn Vernon, *IDDD*, 2013), so the core agrees with ArcLint's built-in `domain_isolation` and `repository/*` rules. The `shared` zone and the language-specific checks (`core/performs-no-io`, `domain/no-wire-format`, `core/concept-named-files`, and test exclusions) are ArcLint enforcement conventions.

Every rule except `core/concept-named-files` skips test files (`*_test.go`, `*.test.ts`, `*.test.tsx`, `*.spec.ts`, `*.spec.tsx`, `test_*.py`, `*_test.py`), because tests wire fakes and adapters across zones. `arclint init --pattern hexagonal` drafts bindings for a Go layout; bound missing folders pass as empty, and `arclint context --dependencies --format json` shows each zone's matched files.

#### Opting in to application packages and framework exceptions

To let `application` use specific third-party packages (such as Effect v4's root `effect` module for `Context.Service` ports, `sqlalchemy.orm`, or `github.com/pkg/errors`), disable `application/core-libraries-only` with a reason and add a local rule using the Pattern's shipped `hexagonal/allowed-packages` extension; `application/through-ports` stays untouched. Likewise, if a file-based router (such as Next.js or TanStack Start) requires route modules in `inbound` to import a runtime module in `composition`, opt in explicitly by overriding `inbound/calls-the-application` (keeping `composition` free of re-exported `outbound` adapters):

```yaml
runtime: [ts]
extends:
  - pattern: arclint/hexagonal@0.1.0-beta.2
    bind:
      domain: src/domain/**
      application: src/application/**
      inbound: src/routes/**
      outbound: src/adapters/**
      shared: src/shared/**
      composition: src/app/**
rules:
  arclint/hexagonal:application/core-libraries-only:
    disable: "The application declares its ports with Effect v4; application/effect-core replaces this rule."
  application/effect-core:
    rationale: "The application uses Effect's root module and no other third-party package."
    on: application
    exclude:
      paths: ["**/*.test.ts", "**/*.test.tsx", "**/*.spec.ts", "**/*.spec.tsx"]
      reason: "Tests wire fakes and adapters across zones."
    uses: hexagonal/allowed-packages
    with:
      packages: [effect]
  arclint/hexagonal:inbound/calls-the-application:
    disable: "File-based route modules invoke the application through src/app/runtime.ts."
  inbound/file-routes:
    rationale: "Route modules import application, domain, shared, and the runtime module, never outbound."
    on: inbound
    exclude:
      paths: ["**/*.test.ts", "**/*.test.tsx", "**/*.spec.ts", "**/*.spec.tsx"]
      reason: "Tests wire fakes and adapters across zones."
    imports:
      internal: [application, domain, shared, composition]
      external: allow
```

Listed packages match exactly as written (`effect` admits `"effect"` and not `effect/http`; Python dotted imports such as `sqlalchemy.orm` and Go package paths match the same way). TypeScript tsconfig aliases such as `@/domain/order` are judged like relative imports (see [Import classes](/docs/concepts/#import-classes)). To change which zones a zone may import, disable its `imports` rule with a reason and write your own under a new ID.

#### Several bounded contexts

Bind one hexagon per context with a wildcard and declare a Zone named for each context over its **core** (`domain` and `application`):

```yaml
extends:
  - pattern: arclint/hexagonal@0.1.0-beta.2
    bind:
      domain: "internal/*/domain/**"
      application: "internal/*/app/**"
      inbound: "internal/*/inbound/**"
      outbound: "internal/*/outbound/**"
      shared: "internal/shared/**"
      composition: "cmd/**"
zones:
  ordering:
    paths:
      - "internal/ordering/domain/**"
      - "internal/ordering/app/**"
    description: "The ordering context's core."
  billing:
    paths:
      - "internal/billing/domain/**"
      - "internal/billing/app/**"
    description: "The billing context's core."
```

Naming a Zone for each context lets the built-in `bounded_context/isolated` and `context_relation/imports-follow-influence` rules govern cross-context core imports from your domain recording, while keeping `inbound` and `outbound` adapters outside those context Zones so adapters can freely import `shared` without conflicting with `domain_isolation/model-imports-nothing-outside-itself`. Repositories with unrelated contexts that also want adapter-level slice isolation can add a local `independent: ["internal/*"]` rule (which automatically skips Zone-owned folders like `shared`).

### arclint/vertical

Vertical slices for Go: each feature folder under `internal/` owns its
domain, application, and infra packages, features never import each
other, `internal/app` holds transports and wiring, and `internal/shared`
holds cross-cutting mechanisms.

## Installing a Pattern

```bash
arclint patterns install vertical
arclint patterns install acme/layers
arclint patterns install acme/layers@1.2.0 --registry https://patterns.example.com
```

`install` takes a reference (`namespace/name@version`), a
`namespace/name` (its highest version), or a bare name carried by
exactly one namespace, and:

- resolves it offline first, then from the Registry;
- vendors it under `.arclint/patterns/<namespace>/<name>/` when it came
  from the Registry, so the next check needs no network;
- records it under `extends` in `rules.arclint.yaml` with a binding for every
  Zone the Pattern lists, drafted from the paths the Pattern suggests;
- adopts a Zone `rules.arclint.yaml` already declares under the same name:
  its declared paths become the binding and the local declaration is
  folded away, comments preserved;
- replaces the entry in place when the ruleset already extends another
  version of the same Pattern, keeping every binding;
- drafts a `rules.arclint.yaml` that extends the Pattern when there is none
  (`--languages go,ts` chooses the runtime; the default is the
  Pattern's coverage).

```
installed acme/layers@1.0.0 (registry, 3fb2cbda1af6)
vendored to /work/shop/.arclint/patterns/acme/layers
extended /work/shop/rules.arclint.yaml
bound:
  app: internal/app/**
unbound (bind each under extends[].bind before the ruleset loads):
  domain
next: bind the unbound zones, then run `arclint check .`
```

A Zone the Pattern lists without suggested paths is left commented
under `bind` (`# domain: <glob>`); the ruleset says so until the owner
binds it. `arclint init --pattern <name>` drafts the same file for a
new repository.

## Vendoring a Pattern

```bash
arclint patterns vendor vertical
arclint patterns vendor acme/layers@1.2.0
```

writes the Pattern's files under `.arclint/patterns/<namespace>/<name>/`
with a `manifest.json` recording every file's digest. Commit the
directory: every load verifies the copy against its manifest, an edited
file is refused with the advice to re-vendor or to delete the manifest
and author in place, and the Registry is never needed again. Vendoring a
Pattern that is already vendored writes nothing; vendoring another
version of the same name replaces the directory.

Embedded Patterns can be vendored too, which pins a repository to the
bytes it reviewed even when the binary that checks it is upgraded.

## Adopting a Pattern by hand

`extends` names the Pattern by exact reference (`namespace/name@version`,
exact semver, no ranges) and binds every Zone the Pattern lists:

```yaml
runtime: [go]

extends:
  - pattern: arclint/vertical@0.1.0
    bind:
      domain: "internal/*/domain/**"
      application: "internal/*/application/**"
      infra: "internal/*/infra/**"
      app: "internal/app/**"
      shared: "internal/shared/**"
      composition: "cmd/**"

zones:
  toolchain: ["Makefile", "go.mod"]

rules:
  # Overrides: the Pattern's qualified id, no constraint key.
  arclint/vertical:shared/concerns:
    severity: warning
  arclint/vertical:domain/no-context:
    disable: "this service threads context through domain services on purpose"
  arclint/vertical:features/independent:
    exclude:
      paths: ["internal/billing/**"]
      reason: "billing is being split; tracked in AL-52"

  # House Rules beside them, under new local ids.
  toolchain/gates-present:
    rationale: "The build gates the repo promises are present."
    on: toolchain
    structure:
      require: ["Makefile", "go.mod"]
```

The loader enforces the adoption contract:

- Every Zone the Pattern lists must be bound; a Zone left unbound
  is rejected (`unbound zones ports, adapters`), and a binding for a
  Zone the Pattern does not list is rejected too.
- A bound Zone is declared like any other: local Rules may name it
  under `on`. A local `zones:` entry with the same name must carry
  the same paths; different paths are rejected.
- The Pattern's Rules load under their qualified IDs
  (`arclint/vertical:domain/stdlib-only`). `arclint rules` lists them
  beside the local Rules with `from arclint/vertical@0.1.0`,
  `arclint check` reports their findings under the qualified id with
  the Pattern's version beside it (`(@0.1.0)`), and
  `arclint rules arclint/vertical:domain/stdlib-only` shows the Pattern
  under `provenance`. A finding always says whether it came from a
  Pattern or from the local ruleset: a local Rule's id has no
  qualifier.
- Extensions the Pattern carries are supplied to the runtime for the
  Pattern's Rules; nothing is copied into `.arclint/extensions`.
- One Pattern is extended at most once, and two extended Patterns may
  not distribute the same qualified ID.
- Two sources that carry one reference must agree on its digest; a
  disagreement is an error, because a published version is immutable.

## Overrides

An entry under `rules:` with no constraint key is an Override, and its
key must be the qualified ID of a Rule an extended Pattern distributes.
An Override changes at least one of:

| key | effect |
|---|---|
| `severity` | `error`, `warning`, or `info` |
| `time_limit` | for a `uses` Rule only: how long its extension's check may run, such as `30s` |
| `disable` | a reason string; the Rule stays listed, is marked disabled, and evaluates nothing |
| `exclude` | `{paths, zones, reason}`: files the Rule does not judge |
| `suppress` | `{paths, reason}`: findings kept in the Assessment but not active |

An Override never carries `rationale`, `on`, `files`, `with`, or a
constraint: a Pattern Rule keeps its own Rationale, Zones, and Constraint.
To require something different, disable the Pattern Rule with a reason
and add a local Rule under a new ID. Writing a local Rule under a
Pattern Rule's ID is rejected for the same reason, and an Override
whose ID no extended Pattern distributes is rejected with the list of
constraint keys a new Rule may carry.

## Authoring a Pattern

A Pattern is one `pattern.yaml` under
`.arclint/patterns/<namespace>/<name>/`, with its Extensions as `*.ts`
files under `extensions/` beside it. The file has no `runtime`, `scan`,
or `extends`; its `pattern:` header carries the identity:

```yaml
# .arclint/patterns/acme/hexagonal/pattern.yaml
pattern:
  namespace: acme
  name: hexagonal
  version: 1.0.0
  coverage: [go, ts]
  documentation: |
    Ports and adapters. The core owns the domain and the ports; adapters
    implement them; nothing else imports an adapter.

zones:
  core: "The domain and the ports it exposes."
  ports:
    description: "Interfaces the core owns and adapters implement."
    paths: "internal/*/ports/**"     # a suggestion install copies into bind
  adapters: "Implementations of the ports; technology lives here."

rules:
  core/stdlib-only:
    rationale: "The core imports no other Zone and no third-party package."
    on: core
    imports:
      internal: []
      external: forbid

  core/no-adapter-imports:
    rationale: "The core never imports an adapter."
    on: core
    imports:
      internal: [ports]

  adapters/private:
    rationale: "Only the core and the composition root import adapters."
    on: adapters
    imported_by: [core]

  core/checked:
    rationale: "Core files pass the acme check."
    on: core
    uses: acme/check
    with:
      strict: true

  dependencies/acyclic:
    rationale: "Zone dependencies contain no cycle."
    acyclic: {}
```

The rules of the Pattern file:

- `namespace`, `name`, and `version` are required; `version` is exact
  semver. `coverage` lists the languages the Pattern's Rules were
  written for; `documentation` is the prose `init` and `install` copy
  into the drafted `rules.arclint.yaml` header.
- A Zone is listed by its description: a bare string, or an object
  with `description` (required) and optional `paths` that `install` and
  `init --pattern` offer as the starting binding. A list of globs is
  rejected, because the adopting repository binds the paths.
- Rule IDs are local (`core/stdlib-only`); the loader qualifies them
  with the Pattern's namespace/name (`acme/hexagonal:core/stdlib-only`).
  A qualified ID inside a Pattern file is rejected. Because the
  qualifier names the Pattern and not only its publisher, two Patterns
  may distribute the same local ID (`acme/hexagonal:core/stdlib-only`
  and `acme/onion:core/stdlib-only`) and both apply; an Override under
  either qualified ID reaches exactly that Rule.
- Every Rule names only Zones the Pattern lists, and every entry
  carries a constraint key; a Pattern distributes Rules and cannot
  override. A Pattern with no Rules is rejected. An `acyclic: {}`
  inside a Pattern resolves to the Pattern's own Zones, so
  `arclint rules acme/hexagonal:dependencies/acyclic` lists
  `core, ports, adapters` and a sibling Pattern's Zones never enter
  that cycle scope.
- Extension names a Pattern Rule uses (`acme/check`) are registered by
  the `*.ts` files under the Pattern's `extensions/` directory; the
  adopting repository never copies them. A name no file registers
  fails `arclint check` with the message
  `no extension registers rule "acme/check"`.

`arclint patterns` lists the authored package once the file loads, and
`arclint patterns install acme/hexagonal` adopts it the same way as a
built-in. The authoring repository can extend its own Pattern to prove
it against real code before publishing.

## Publishing to a Registry

```bash
arclint patterns export acme/hexagonal --dir ../arclint-pattern-registry
```

writes `<dir>/acme/hexagonal/1.0.0/` with `pattern.yaml`, the
`extensions/` directory, and a `manifest.json`, and updates
`<dir>/index.json`. Any static file host that serves the tree is a
Registry: a GitHub repository served raw, an object store, or a
directory reachable as `file://`. Publish a version once; a later
export of the same version replaces the listed entry, and the index
records the digest, so an adopter's `install` cross-checks the files
it fetched against what the index promised.

Requests to an `https` Registry send `Authorization: Bearer` with
`GITHUB_TOKEN` or `GH_TOKEN` when either is set, so a private GitHub
repository can serve a team's Patterns.

## The recorded domain needs no Pattern

The rules that judge a repository's recorded Ubiquitous Language
against its code are built into the binary and apply as soon as
the domain file (the one `rules.arclint.yaml` names under `domain.file`, else
the first of `domain.arclint.yaml`, `domain.arclint.yml` and
`domain.arclint.cml` that exists) records a context; nothing is installed for
them.
arclint locates a context's code from its recorded terms, the
declarations that spell them, wherever they are in the repository;
name a Zone after the context (`billing: internal/billing/**`) only
to narrow that search when the repository declares a recorded term
more than once. Once a context and its relations are recorded, the
context map becomes import rules, every aggregate must have its root,
and every invariant its enforcing method. A Pattern adds what the
meta-model does not say: the layout around the model. See
[Domain Contracts](/docs/contracts/).
