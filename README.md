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

Write a `rules.yaml` at your repo root:

```yaml
runtime: [go]

modules:
  entities: ["internal/entities/**"]
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
```

Design: [docs/design-proposal.md](docs/design-proposal.md). Decisions
log: [docs/decisions.md](docs/decisions.md).
