+++
title = "Patterns"
description = "Distribute Rules as a versioned Pattern; adopt one with install, extends, bind, and Overrides; vendor it so a check never needs the network."
weight = 5
+++

A **Pattern** is a named, versioned collection of Rules packaged for
distribution: an identity header, the Modules its Rules speak about
(without paths), the Rules, and any Extensions those Rules use. A
repository adopts a Pattern by reference from `rules.yaml`, binds each
Pattern Module to the paths it owns locally, and adjusts individual
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
   (`arclint/vertical`, `arclint/domain-model`). They need no files in
   the repository and no network.
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
arclint/domain-model@0.1.0  embedded            3 rule(s)  3 extension(s)  coverage [go, ts]  a5e0ad0146c3
arclint/vertical@0.1.0      embedded, vendored 16 rule(s)  5 extension(s)  coverage [go]      fc01898bee8f
acme/layers@1.0.0           authored            1 rule(s)  0 extension(s)  coverage [go]      3fb2cbda1af6
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
- records it under `extends` in `rules.yaml` with a binding for every
  Module the Pattern lists, drafted from the paths the Pattern suggests;
- adopts a Module `rules.yaml` already declares under the same name:
  its declared paths become the binding and the local declaration is
  folded away, comments preserved;
- replaces the entry in place when the ruleset already extends another
  version of the same Pattern, keeping every binding;
- drafts a `rules.yaml` that extends the Pattern when there is none
  (`--languages go,ts` chooses the runtime; the default is the
  Pattern's coverage).

```
installed acme/layers@1.0.0 (registry, 3fb2cbda1af6)
vendored to /work/shop/.arclint/patterns/acme/layers
extended /work/shop/rules.yaml
bound:
  app: internal/app/**
unbound (bind each under extends[].bind before the ruleset loads):
  domain
next: bind the unbound modules, then run `arclint check .`
```

A Module the Pattern lists without suggested paths is left commented
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
exact semver, no ranges) and binds every Module the Pattern lists:

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

modules:
  toolchain: ["Makefile", "go.mod"]

rules:
  # Overrides: the Pattern's qualified id, no assertion key.
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
    description: "The build gates the repo promises are present."
    on: toolchain
    structure:
      require: ["Makefile", "go.mod"]
```

The loader enforces the adoption contract:

- Every Module the Pattern lists must be bound; a Module left unbound
  is rejected (`unbound modules ports, adapters`), and a binding for a
  Module the Pattern does not list is rejected too.
- A bound Module is declared like any other: local Rules may name it
  under `on`. A local `modules:` entry with the same name must carry
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

An entry under `rules:` with no assertion key is an Override, and its
key must be the qualified ID of a Rule an extended Pattern distributes.
An Override changes at least one of:

| key | effect |
|---|---|
| `severity` | `error`, `warning`, or `info` |
| `disable` | a reason string; the Rule stays listed, is marked disabled, and evaluates nothing |
| `exclude` | `{paths, modules, reason}`: files the Rule does not judge |
| `suppress` | `{paths, reason}`: findings kept in the Assessment but not active |

An Override never carries `description`, `on`, `files`, `with`, or an
assertion: a Pattern Rule keeps its own Claim, Modules, and parameters.
To assert something different, disable the Pattern Rule with a reason
and add a local Rule under a new ID. Writing a local Rule under a
Pattern Rule's ID is rejected for the same reason, and an Override
whose ID no extended Pattern distributes is rejected with the list of
assertion keys a new Rule may carry.

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

modules:
  core: "The domain and the ports it exposes."
  ports:
    description: "Interfaces the core owns and adapters implement."
    paths: "internal/*/ports/**"     # a suggestion install copies into bind
  adapters: "Implementations of the ports; technology lives here."

rules:
  core/stdlib-only:
    description: "The core imports no other Module and no third-party package."
    on: core
    imports:
      internal: []
      external: forbid

  core/no-adapter-imports:
    description: "The core never imports an adapter."
    on: core
    imports:
      internal: [ports]

  adapters/private:
    description: "Only the core and the composition root import adapters."
    on: adapters
    imported_by: [core]

  core/checked:
    description: "Core files pass the acme check."
    on: core
    uses: acme/check
    with:
      strict: true

  dependencies/acyclic:
    description: "Module dependencies contain no cycle."
    acyclic: {}
```

The rules of the Pattern file:

- `namespace`, `name`, and `version` are required; `version` is exact
  semver. `coverage` lists the languages the Pattern's Rules were
  written for; `documentation` is the prose `init` and `install` copy
  into the drafted `rules.yaml` header.
- A Module is listed by its description: a bare string, or an object
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
- Every Rule names only Modules the Pattern lists, and every entry
  carries an assertion key; a Pattern distributes Rules and cannot
  override. A Pattern with no Rules is rejected.
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

## The domain-model Pattern

`arclint/domain-model` turns a repository's recorded Ubiquitous
Language into a contract. Its three Rules read
`ubiquitous-language.yaml` through the extension SDK: every recorded
term carries a definition, every recorded invariant names a recorded
term of its own context as its owner, and imports between Modules named
after bounded contexts respect the recorded context-map relations.

```bash
arclint patterns install domain-model
```

binds its one Module, `vocabulary`, to `ubiquitous-language.yaml`.
Declare one Module per bounded context whose imports the map should
govern (`billing: internal/billing/**`) and the context map becomes
import rules with no further configuration. arclint's own `rules.yaml`
extends this Pattern; the vocabulary rules it enforces on itself are
the ones every adopter receives.
