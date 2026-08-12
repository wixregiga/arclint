# arclint

Architecture contracts as data. One static Go binary enforces module
contracts over a repository: what each module may **consume**
(dependencies, layers, third-party policy), what it must **provide**
(registrations, correspondences), and the **invariants** that always hold
(naming, structure, content). Every violation carries blame: a consumes
break points at the importing file, a provides break points at the module
that broke its promise.

Rules are plain YAML validated by a published JSON Schema
([docs/rules.schema.json](docs/rules.schema.json)). Predicates use
[expr](https://github.com/expr-lang/expr). No invented DSL.

## Quickstart

Build (CGO_ENABLED=0, single static binary):

```bash
make build
```

Set a repository up from a shipped architectural pattern:

```bash
./arclint init
```

`init` detects the languages present, asks which to analyze and which
pattern to start from (`--runtimes go,ts --pattern layers` skips the
prompts), writes rules.yaml plus the pattern's TypeScript extensions,
generates editor typings, and validates everything. Patterns:

- **feature-slice** (go) — open-set feature/concept slices: a directory
  owning `command.go` is a feature, any other `internal/` directory is a
  concept, and every rule applies to new features automatically.
- **layers** (go, ts, py) — hexagonal: cmd composes, app orchestrates,
  domain decides and depends on nothing, infra adapts behind ports.
- **starter** (go, ts, py) — one module, unknown imports surfaced; grow
  contracts from there.

`arclint patterns` lists them; `--extensions` shows the rule extensions
each installs. A repository defines its own under
`.arclint/patterns/<name>/` (`pattern.yaml` + `rules.yaml` +
`extensions/`), and a local name shadows a builtin.

Or write a `rules.yaml` at your repo root yourself:

```yaml
runtime: [go]

modules:
  entities:
    paths: ["internal/entities/**"]
    description: "Domain types and invariants; depends on nothing."
  features: ["internal/features/*"]
  registry: ["internal/shared/registry.go"]
  setup:    ["internal/setup/**"]

contracts:
  entities:
    consumes:
      internal: []            # no other internal modules
      external: forbid        # no third-party imports
      stdlib: allow
    provides:
      - kind: correspondence  # every entity substrate has a setup counterpart
        of: { files: 'internal/entities/[^/]+_(?P<substrate>[a-z0-9]+)\.go', value: "{substrate}" }
        in: { files: 'internal/setup/(?:[^/]+/)*(?P<substrate>[a-z0-9]+)\.go', value: "{substrate}" }
        relation: subset

  features:
    provides:
      - kind: registration    # every feature registers itself
        each: 'internal/features/(?P<feature>[^/]+)/'
        in: registry
        match: 'RegistryFactory\.Register\("{feature}"\)'
```

Load, inspect, check:

```bash
./arclint load rules.yaml
./arclint list
./arclint rules ls
./arclint check .
./arclint check . --format json
```

Learn the vocabulary and your own ruleset from the terminal:

```bash
./arclint explain              # every rule kind, one line each
./arclint explain consumes     # what internal/external/stdlib mean, with an example
./arclint module ls            # declared modules: files, languages, description
./arclint module info entities # one module: description, members, every rule that binds it
```

Exit codes: `0` clean, `1` violations (severity `error`), `2` config or
usage error. JSON violations have the stable shape
`{ruleId, contract, blame, severity, path, line?, message, fixHint}`.

## Rule surface

- **consumes** — per-module `internal` allow/deny lists, `external`
  `allow|forbid`, `stdlib` `allow|forbid`; graph-wide `dependencies:`
  rules: `layers`, `forbidden`, `independence`, `protected`, `acyclic`.
- **provides** — `registration` (every capture of `each` must have a
  `match` hit in the `in` module) and `correspondence` (the value set
  derived from `of` path/content captures must be `subset`/`equal` of the
  `in` side).
- **invariants** — `naming` (kebab-case, snake_case, camelCase,
  PascalCase, `regex:`, combinable with `|`), `structure`
  (`require`/`forbid` globs), `content` (`must`/`must_not` regexes), and
  `expr` predicates over `file` (`file.lines <= 400`,
  `"os" in file.imports`), type-checked at load time.

## TypeScript extensions (layer 2)

Full rule logic in `.arclint/extensions/*.ts`, executed in-process via
esbuild + sobek (the k6 pattern): no Node, npm, or tsc on the machine.
`arclint sdk init` writes `arclint.d.ts` (generated from the Go host
types) and a `tsconfig.json` for editor typing.

```ts
// .arclint/extensions/handler-naming.ts
import { defineRule, s } from "arclint";

export default defineRule({
  type: "handler-naming",
  description: "Handler files carry the configured suffix.",
  params: s.object({ suffix: s.string().default("Handler") }),
  check(ctx, params) {
    for (const f of ctx.files("internal/**/handlers/*.go")) {
      if (!f.stem.endsWith(params.suffix as string)) {
        ctx.report({ path: f.path, message: `handler files end in ${params.suffix}` });
      }
    }
  },
});
```

`ctx.imports(path)` serves every active language target, with classified
specifiers and tree-resolved targets (`targetDir`, `targetFile`). A rule
type declares its contract clause and blame side once, and any single
finding may override both (`ctx.report({..., contract: "consumes",
blame: "consumer"})`) so multi-sided rules label each finding
truthfully. Descriptions surface in `arclint explain` and `rules ls`.

Instances stay pure data in rules.yaml, validated against the extension's
schema before any extension code runs:

```yaml
rules:
  - type: handler-naming
    params: { suffix: "Handler" }
```

Sandbox: extensions see only the read-only ctx (files, read, imports,
modules, report); `Date.now`/`Math.random` are host-controlled; runaway
rules are interrupted after 5s. Relative imports are bundled; bare npm
imports are rejected.

## Multi-language targets

`runtime: [go, ts, py]` activates per-language extraction, embedded
stdlib tables, and manifest-based external classification. Go is exact
(`go/parser`, no false-negative class). JS/TS and Python are lexer-grade
with documented, test-asserted false-negative classes: computed
specifiers (`import(x)`, `require(v)`, `importlib.import_module(name)`)
are invisible at this tier by design.

- **JS/TS**: static `import`/`export ... from`/side-effect imports plus
  literal `import()`/`require()`; `node:` and the Node builtin table
  (from `require('module').builtinModules`) → stdlib; relative
  specifiers resolve with extension probing; a bare specifier naming an
  in-repo package.json → internal (workspace semantics); the nearest
  package.json's dependency sections → external.
- **Python**: `import`/`from ... import` at any indentation with
  continuations and parenthesized forms, docstring-aware; the embedded
  `sys.stdlib_module_names` table → stdlib; source roots (root, src/,
  pyproject dirs) resolve dotted modules, including PEP 420 namespace
  dirs → internal; pyproject.toml dependencies (PEP 621,
  dependency-groups, poetry) via PEP 503 normalization → external;
  dist/module name mismatches (PyYAML→yaml) classify unknown, never
  silently external.

## Go import resolution

`go/parser` in ImportsOnly mode; exact classification, no heuristics:

- **stdlib**: embedded table generated from `go list std` of the pinned
  toolchain (`go generate ./...` refreshes it).
- **internal**: under the owning module's path, resolved through a
  `replace` directive to a local directory, or a `go.work` workspace
  member. Nested modules own their files.
- **external**: resolvable via `go.mod` require.
- **unknown**: anything else, governed by `scan.unknown_imports`
  (`warn` default, `error`, `ignore`).
- `vendor/` is never scanned and vendored packages classify external;
  `testdata/` is excluded by Go convention (`scan.include_testdata: true`
  overrides); build-constrained files are still scanned — the divergence
  from a GOOS/tag-filtered `go build` is documented and asserted in the
  differential oracle.

Correctness is proven mechanically, not by fixtures alone:
`make oracle` clones five pinned real repositories (cobra, gin,
go-testfixtures with go.work, runc with vendor and cgo, opentelemetry-go
with 28 modules and sibling replaces) and asserts per-file import
extraction and classification against `go list -deps -test -json` ground
truth — 7,500+ imports, zero mismatches.

## Self-hosting and performance

`make selfcheck` lints arclint with its own [rules.yaml](rules.yaml).
Measured on WSL2 (go1.26.4, `make bench`): cold start median 7.99ms
(bound 100ms); a synthetic 5,000-file repository checks in a median of
79.6ms (bound: low single-digit seconds).

## Development

```bash
make ci        # vet + tests + selfcheck
make oracle    # differential oracle (network)
make bench     # gate-4 numbers
make release   # CGO_ENABLED=0 linux/amd64 + linux/arm64
make docs      # build the docs site (zola; content in docs/site/content)
```

Documentation lives in [docs/site/content](docs/site/content) as plain
markdown; the [rule reference](docs/site/content/docs/rules.md) is
generated from the same source as `arclint explain` and the schema
hovers (`go generate ./tools/gendocs`), and a test fails when it
drifts.

Design: [docs/design-proposal.md](docs/design-proposal.md). Decisions
log: [docs/decisions.md](docs/decisions.md).
