+++
title = "Patterns"
description = "Shipped architectural patterns, and how to define your own."
weight = 5
+++

A pattern is a complete, working starting point: a rules.yaml template
plus any TypeScript extensions it needs. `arclint init` installs one;
`arclint patterns` lists what is available.

## Shipped patterns

### feature-slice (go)

Feature slices over fixed places, with open sets classified by shape:

```text
cmd/               thin bootstrap
internal/app/      the composer: wires adapters into use cases
internal/shared/   every technology adapter, sealed behind NewAdapters
internal/<d>       owning command.go     a FEATURE (user capability)
internal/<d>       anything else         a CONCEPT (shared rules + port)
```

Because identity is structural, `internal/newfeature/` with a
`command.go` joins every rule the moment it exists. No list to maintain.
The YAML half enforces the fixed places (protected shared and app, the
feature shape correspondence, wiring registration, banned buckets,
naming); the paired extension enforces what YAML cannot scope without
named modules: the feature/concept dependency matrix, third-party bans,
concept purity, ports, drift, and thin use cases.

### layers (go, ts, py)

The hexagonal spine, fully declarative: `cmd` composes, `app`
orchestrates, `domain` decides and depends on nothing, `infra` adapts
behind domain ports and is reachable only from the composition root.

### starter (go, ts, py)

One module over the whole tree, unknown imports surfaced, and pointers
to `arclint explain` for growing real contracts as boundaries emerge.

## Your own patterns

A repository defines local patterns under `.arclint/patterns/<name>/`
(nested names like `fsd/go` are legal; a local name shadows a builtin):

```text
.arclint/patterns/fsd/go/
  pattern.yaml       description + compatible runtimes
  rules.yaml         complete template (valid as-is)
  extensions/*.ts    optional rule extensions
```

```yaml
# pattern.yaml
description: "Our team's FSD variant for Go services."
runtimes: [go]
```

`arclint patterns` lists it next to the builtins; `arclint init
--pattern fsd/go` installs it. Templates keep a literal `runtime:` line;
init rewrites it to the chosen targets, so a template always loads
standalone during development.
