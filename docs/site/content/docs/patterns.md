+++
title = "Patterns"
description = "Distribute Rules as a versioned Pattern; adopt one with extends, bind, and Overrides."
weight = 5
+++

A **Pattern** is a named, versioned collection of Rules packaged for
distribution: an identity header, the Modules its Rules speak about
(without paths), the Rules, and any Extensions those Rules use. A
repository adopts a Pattern by reference from `rules.yaml`, binds each
Pattern Module to the paths it owns locally, and adjusts individual
Rules through Overrides. Rule text is never copied; when a folder moves,
only the binding changes.

## Listing what is available

```bash
arclint patterns
```

Lists the Pattern distribution packages the running CLI can see:
built-in packages embedded in the binary, then local packages under
`.arclint/patterns/<name>/pattern.yaml`. Both kinds appear the same way
and adopt the same way; they differ only in where their bytes come
from.

```
arclint/vertical@0.1.0  16 rule(s)  5 extension(s)  coverage [go]
```

## Adopting a Pattern

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
  arclint:shared/concerns:
    severity: warning
  arclint:domain/no-context:
    disable: "this service threads context through domain services on purpose"
  arclint:features/independent:
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
  under `on`, and a local `modules:` entry with the same name is
  rejected because the Pattern already bound it.
- The Pattern's Rules load under their namespaced IDs
  (`arclint:domain/stdlib-only`); `arclint rules` lists them beside the
  local Rules, and `arclint rules arclint:domain/stdlib-only` shows the
  Pattern it came from under `provenance`.
- Extensions the Pattern carries are supplied to the runtime for the
  Pattern's Rules; nothing is copied into `.arclint/extensions`.
- One Pattern is extended at most once, and two extended Patterns may
  not distribute the same qualified ID.

`arclint init --pattern arclint/vertical@0.1.0` (or `--pattern vertical`
by name) drafts exactly this file with the Pattern's suggested paths
filled into `bind`. `--pattern bare` (the no-flag default) writes the
commented single-module draft instead.

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

A Pattern is one `pattern.yaml` under `.arclint/patterns/<name>/`, with
its Extensions as `*.ts` files under `.arclint/patterns/<name>/extensions/`.
The file has no `runtime`, `scan`, or `extends`; its `pattern:` header
carries the identity:

```yaml
# .arclint/patterns/hexagonal/pattern.yaml
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
    paths: "internal/*/ports/**"     # a suggestion init copies into bind
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
  written for; `documentation` is the prose `init` copies into the
  drafted `rules.yaml` header.
- A Module is listed by its description: a bare string, or an object
  with `description` (required) and optional `paths` that
  `arclint init --pattern` offers as the starting binding. A list of
  globs is rejected, because the adopting repository binds the paths.
- Rule IDs are local (`core/stdlib-only`); the loader qualifies them
  with the namespace (`acme:core/stdlib-only`). A namespaced ID inside a
  Pattern file is rejected.
- Every Rule names only Modules the Pattern lists, and every entry
  carries an assertion key; a Pattern distributes Rules and cannot
  override. A Pattern with no Rules is rejected.
- Extension names a Pattern Rule uses (`acme/check`) are registered by
  the `*.ts` files under the Pattern's `extensions/` directory; the
  adopting repository never copies them. A name no file registers
  fails `arclint check` with the message
  `no extension registers rule "acme/check"`.

`arclint patterns` lists the local package once the file loads, and
`extends: [{pattern: acme/hexagonal@1.0.0, bind: {...}}]` adopts it the
same way as a built-in.
