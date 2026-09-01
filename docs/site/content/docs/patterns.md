+++
title = "Patterns"
description = "Author and inspect validated Pattern distribution packages."
weight = 5
+++

A **Pattern** is a named, versioned, namespaced collection of Rules
packaged for distribution. It may declare language coverage, Extensions,
and a test root. A Pattern describes Modules and the Rules that govern
them; it does not create the files or directories those Modules select.
Rule order does not give one Rule precedence over another.

`pattern.yaml` is the package contract. ArcLint validates the same
contract for embedded and local Patterns before making a Pattern
available.

## Authoring a Pattern

Put `pattern.yaml` at the root of the Pattern tree. This is the canonical
shape:

```yaml
pattern:
  namespace: arclint
  name: vertical
  version: 0.1.0
coverage: [go]
modules:
  - name: domain
    paths: ["internal/domain/**"]
rules:
  - id: arclint:vertical/domain/stdlib-only
    kind: consumes
    module: domain
    forbid: [external]
extensions:
  - name: forbid-imports
    entry: extensions/forbid_imports.ts
tests:
  root: tests
```

`pattern.namespace`, `pattern.name`, and `pattern.version` form the exact
reference `namespace/name@version`. They must be written explicitly;
directory names do not supply missing identity. Coverage is optional and
may be empty. Every declared language must be supported by ArcLint.

Module names and Rule IDs must each be unique. A Module-scoped Rule
names its Module explicitly; repository-wide Rule kinds carry their own
scope fields. Extensions are optional and each declaration points to a
file inside the Pattern tree. Tests are also optional; when a test root
is declared, it must exist and contain valid Rule tests. Missing declared
files and invalid contracts are errors reported at their manifest
locations.

A published Pattern version is immutable. Its digest covers the full
Pattern tree, including `pattern.yaml`, declared Extension files, and
packaged tests. Loading preserves those bytes; ArcLint does not rewrite
the package to make it valid.

`pattern.yaml` is not a repository `rules.yaml`. Repository Rules remain
in `rules.yaml`; a Pattern manifest is the distribution contract that
groups Modules, Rules, and optional package assets.

## Inspecting available Patterns

```bash
arclint patterns
```

Lists Pattern distribution packages the running CLI can see: built-in
packages embedded in the binary, then local packages under
`.arclint/patterns`.

```
arclint/vertical@0.1.0  16 rule(s)  5 extension(s)  coverage [go]
```

`arclint init --pattern vertical` continues to materialize the built-in
vertical Pattern as a repository-form `rules.yaml` and copies its Extensions into
`.arclint/extensions`. Installed files become repository-owned copies.
`--pattern bare` (the no-flag default) writes the commented single-module
draft and no Extensions. Local Pattern bundles remain listable, but init
still materializes built-ins only.

Pattern installation, application records, registry resolution, and
remote-source precedence are separate distribution work and are not
part of the manifest contract.
