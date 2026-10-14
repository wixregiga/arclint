# TypeScript adapter validation

Validated on 2026-10-14 with Node 24.17.0, TypeScript 5.9.3, and Go 1.27.1.
The runtime implementation uses Go and the pinned tree-sitter grammars.

## Compiler comparison

| Repository | Checked | Agreed | Undetermined | Unavailable files | Disagreements |
| --- | ---: | ---: | ---: | ---: | ---: |
| boxoffice | 184 | 184 | 168 | 0 | 0 |
| white-label | 922 | 922 | 571 | 0 | 0 |
| hexagon | 661 | 661 | 340 | 0 | 0 |

All 1,767 references the adapter resolves agree with the checker. The 1,079
undetermined references are open questions, not agreements. No disagreements
remain. Full reason counts are committed in [results.json](results.json).
ArcLint's `web/` directory is absent on this branch; `testing/boxoffice/web`
is present and was checked.

Pinned source commits:

- ArcLint/boxoffice: `e86b3d759053f08458cdbff1f59c2de4eb2e6622`.
- [stemmlerjs/white-label](https://github.com/stemmlerjs/white-label/tree/503ee491192a3eff026704f4de9a7477daebb630):
  `503ee491192a3eff026704f4de9a7477daebb630`.
- [Sairyss/domain-driven-hexagon](https://github.com/Sairyss/domain-driven-hexagon/tree/5c2d15a7e2d69e83dfddf28468ee9f30e02c30de):
  `5c2d15a7e2d69e83dfddf28468ee9f30e02c30de`.

Both public repositories were cloned outside ArcLint. Dependencies were installed
with lifecycle scripts disabled: `npm ci --legacy-peer-deps --ignore-scripts`
for white-label and `npm install --legacy-peer-deps --ignore-scripts` for hexagon
(which does not commit an npm lockfile). Boxoffice used `npm ci --ignore-scripts`.

White-label's old manifest omits several JavaScript dependency declarations.
The scratch clone also received these pinned declaration packages, without
changing its source:

```sh
npm install --no-save --ignore-scripts --legacy-peer-deps   @types/express@4.17.21 @types/body-parser@1.19.5   @types/compression@1.7.5 @types/cors@2.8.17 @types/morgan@1.9.9   @types/sinon@7.5.2 @types/helmet@0.0.45
```

The oracle must reach a declaration. An unresolved dependency is a comparison
failure, so these packages are prerequisites for reproducing the white-label
run, not exclusions from the comparison.

The five negative controls passed: an incorrect declared target, external module,
builtin owner, declared receiver, and held type each produced a disagreement
and exit status 1. The builtin declaration tables were regenerated independently
and compared byte for byte with the committed tables.

## Inspected production references

The checker compared every resolved reference. Source inspection also covered:

- white-label `Entity.ts:31`: `_id` has static type `UniqueEntityID`, while
  `equals` is declared by `Identifier`.
- white-label `user.ts:111`: the receiver is `User`; `addDomainEvent` belongs
  to `AggregateRoot`.
- white-label HTTP routes `index.ts:8`: the barrel-exported controller value
  has type `CreateUserController`; `execute` belongs to `BaseController`.
- hexagon `user.repository.ts:52`: the receiver is `UserEntity`; `getProps`
  belongs to `Entity`.
- hexagon `user.entity.ts:26`: `addEvent` belongs to `AggregateRoot`, while
  the receiver remains `UserEntity`.

These cover written annotations, imported `new` instances, and inheritance.
No false positives were found in these samples. Initial comparisons caught
wrong imported-value and static/instance ownership, and wrong builtin declaration
owners; each has an adapter regression test. The oracle also needed correct
external re-export provenance, dependency declarations, and source matching for
tagged templates and automatic semicolon insertion.

## Remaining uncertainty

Most open questions come from unannotated factory results, destructured or
computed receivers, generic parameters, and decorators. The adapter also leaves
conditional/mapped/indexed-access computations, ambiguous global or module
merging, skipped declaration targets, unsupported namespace re-exports,
unnamed tuple receivers, lexical private member invocations, and unreachable
imports undetermined with a reason. Runtime proxy effects and arbitrary type
inference are not modeled. These are reported through the existing Reference
contract; no domain recording or architecture rule was weakened.

## Gates

`go test ./...`, `go run ./cmd/arclint check .`, `make check`, and `make ci`
passed. CI builds the binary, vets the repository and examples, runs the full
race-enabled test suite, checks ArcLint and both examples, lints schemas, and
scans for secrets.

ArcLint's final output:

```text
0 active finding(s) · 0 suppressed · 13 baselined · 63 rule(s) applied
```

CI's final output:

```text
No results with a severity of 'error' found!
gitleaks detect --no-git --verbose
INF scanned ~7803581 bytes (7.80 MB)
INF no leaks found
```

## Benchmarks

Benchmarks compare the base adapter and this implementation in process against
exactly the same boxoffice source path. Six alternating samples use
`GOMAXPROCS=4`, `-benchtime=1s`, and the committed `BenchmarkBoxoffice`.
The base worktree receives only the benchmark test file. Directory walking and
file-list construction happen outside the measured producer calls.

```sh
ARCLINT_TS_BENCH_ROOT=/absolute/path/to/testing/boxoffice/web GOMAXPROCS=4   go test ./internal/infrastructure/language/typescript   -run '^$' -bench BenchmarkBoxoffice -benchmem -benchtime=1s -count=6
```

| Requested facts | Base median | New median | Time comparison |
| --- | ---: | ---: | --- |
| Imports | 7.872 ms/op | 7.868 ms/op | p=1.000, n=6 |
| Declarations and syntactic calls | 211.2 ms/op | 223.1 ms/op | p=0.180, n=6 |
| Resolved calls and field types | not implemented | 443.0 ms/op | incremental request cost |

No statistically significant regression was measured when the new facts were
not requested. Bytes and allocation counts also show no significant regression:
imports used 352.0 versus 352.7 KiB/op (p=0.180), and syntactic facts used
210.1 versus 208.1 MiB/op (p=0.589). Resolution used 366.6 MiB/op. The machine
was busy; the time samples have wide ranges, so these are local measurements,
not a throughput guarantee. The generated builtin tables initialize only when
resolution is requested.

Results were compared with `benchstat` pinned to
`golang.org/x/perf@v0.0.0-20261009192801-be2c69fb417e`. Early measurements used
different absolute source paths; the final comparison uses the same path for
both versions so path allocations cannot distort the result.
