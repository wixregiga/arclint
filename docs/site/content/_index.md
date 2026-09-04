+++
template = "index.html"
+++

# Architecture rules as data

ArcLint evaluates Rules against Files, Folders, and Modules. It reports
what it proved, what it suspects, and what it could not determine. Go,
TypeScript, and Python repositories use the same static binary.

```bash
arclint init --languages go,ts
arclint check .
```

`init` drafts a starter `rules.arclint.yaml`. Define Modules, then add Rules that
state what those Modules may import and which invariants their Files must
satisfy:

```yaml
runtime: [go]

modules:
  domain:
    paths: ["internal/domain/**"]

contracts:
  domain:
    consumes:
      id: "repo:domain/stdlib-only"
      internal: []
      external: forbid
      stdlib: allow
    invariants:
      - id: "repo:domain/snake-case"
        kind: naming
        files: "internal/domain/**/*.go"
        case: snake_case
```

The published Rule Types cover Module imports, required or forbidden
paths, naming, dependency layers, protected Modules, dependency cycles,
and TypeScript Extension enforcement. Extensions run in-process through
a scoped SDK when the built-in Rule Types cannot express a check.

Every assessment preserves unsupported, undetermined, failed, and
not-applicable evaluations instead of treating silence as conformance.
Diagnostics distinguish active or suspected Violations from operational
and coverage problems.

Existing debt can be adopted into `.arclint/baseline.v2.json` with
`arclint baseline capture`. Later checks still show the covered count,
and `arclint baseline refresh` removes entries that no longer occur.

[Get started](/docs/getting-started/), read the
[concepts](/docs/concepts/), or inspect the complete
[Rule reference](/docs/rules/).
