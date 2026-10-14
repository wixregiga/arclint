# Python fact validation for issue 106

Implementation branch: `feat/issue-106-python-resolution`.
Base: `e86b3d759053f08458cdbff1f59c2de4eb2e6622`.
Validation used CPython 3.11.13 and Pyright 1.1.407 on Linux amd64.

The implementation extends the Python adapter's existing Producer.Facts,
tree-sitter walker and module resolver. It adds no runtime interpreter,
subprocess, language server, plugin mechanism or dependency on Pyright.
The existing CPython stdlib generator now also records builtin names and
which of them are types.

## Behavior verified

Fields cover class-body annotations, dataclasses, NamedTuple classes,
annotated self attributes and unannotated self assignments in any method.
ClassVar and InitVar are excluded through their imported names and aliases.
The first declaration wins, and unannotated fields report their missing type.

Calls follow repository imports, aliases, package submodules and __init__
re-exports. Annotated parameters, locals and fields establish static receivers;
C3 inheritance establishes declaring classes. Constructor calls reach classes,
module calls reach functions, and builtin references have an empty Package.
Protocol classes resolve without inferring structural implementations.

Field Holds includes container types and their arguments, Optional and union
members, quoted annotations and future annotations. It excludes types inside
Callable signatures, Annotated metadata and Literal values. The unit tests
also check that unrequested facts are absent and that EnclosingOwner is
available with ordinary syntactic call facts.

Unannotated values, Any, getattr/setattr, star imports, kwargs forwarding,
observed monkey patches, descriptors, metaclasses, custom decorators,
unresolved bases, ambiguous unions and competing/namespace roots carry
undetermined reasons. Callable values and callable fields do not become calls
to their annotation's constructor. Dependency annotations retain their external
receiver type; an unobserved dependency member remains undetermined. Generated
dataclass methods have no source declaration, so explicit calls to them remain
undetermined. Zero-argument super with one resolvable base is supported;
explicit super arguments and multiple-base super dispatch remain open.

The field-declarations milestone remains because neither no-identity nor
occurrence-time has a domain evaluator. Their language lists were not expanded.
The milestone now describes that remaining evaluator gap. The metamodel table
was regenerated; `arclint agents md --write` reported AGENTS.md already current.
No new domain recording was needed: unresolved-is-undetermined already covers
these facts.

## Pyright differential oracle

The public repositories were cloned outside the worktree and pinned at:

- cosmicpython/code: `14c84797ffa77255d53cf1a02fe6aafda2b68aeb`.
- pgorecki/python-ddd: `429cd4b1ffed571ccd845e64f05b484f4f8c52ac`.

Cosmic Python: 310 references checked, 310 agreed, zero disagreements;
464 undetermined. Agreements include 265 call targets, 23 receivers and
22 field types.

python-ddd: 1,267 checked, 1,267 agreed, zero disagreements;
1,141 undetermined. Agreements include 904 call targets, 210 receivers and
153 field types.

ArcLint's own Python oracle driver: 75 checked, 75 agreed, zero disagreements;
148 undetermined. There were no tracked Python files under testing/ or docs/.

Total: 1,652 checked, 1,652 agreed, zero disagreements; 1,753 undetermined.
Every produced Target, Receiver and Holds reference enters the exporter;
its query count is checked against the production fact count, and the facts
also pass NewObservations validation. External Package is verified by a second
Pyright query through an import-only document in memory. Missing definitions
and mismatches fail the run; no case is silently removed from the denominator.
The pinned scratch dependency environment is in oracle-requirements.txt.

Manual inspection included repository config calls and SMTP construction in
Cosmic Python's notifications adapter; AbstractRepository receivers; command
string fields and the unit-of-work's repository field; python-ddd's migration
calls, unannotated config values, Application receivers, LoggerFactory calls,
and GenericUUID fields. The untyped server field in Cosmic Python and the
untyped container field in python-ddd stayed undetermined. No false resolved
reference remained in the inspected sample or the differential results.

Earlier disagreements exposed callable-value/field confusion, imported values
being mistaken for types, custom attribute access inherited by LazyObject,
and generated dataclass methods. Regression tests cover those failures.
Final review also added tests for with/except aliases shadowing imports, star
imports replacing earlier declarations, and untyped values used as receivers.
Pyright's implicit-self and super endpoint conventions were normalized in the
oracle. Missing external dependency definitions were addressed by installing
the pinned scratch environment. A dependency method absent from that library
stays undetermined; the oracle does not excuse it as an agreement.

## In-process benchmark

Cosmic Python at the SHA above; Core i9-10885H; GOMAXPROCS=4; five samples,
one second per sample. The parent adapter was compiled using Go's -overlay
with its original python.go, declarations.go and stdlib_gen.go from the base
commit. Newly added adapter files were replaced with empty package files in
the overlay; a copy of the benchmark was retained. No repository file changed.

- Imports only, alternating parent/current runs: paired means 2.973 ms before,
  2.974 ms after. The paired difference is indistinguishable from noise
  (two-sided sign permutation p=1.0); both allocate 6,588 times per operation.
- Existing declarations and calls: median 45.70 ms before, 46.82 ms after.
  Parent samples span 42.16-50.01 ms; current samples span 40.84-50.84 ms.
  A two-sided permutation comparison gives p=0.992; no measurable regression.
- Both new fact classes requested: median 81.97 ms, including imports,
  declarations, calls, module indexing and resolution.

The shipped benchmark also exposes legacy_imports, the prior import path called
directly in-process. All benchmark invocations ended in PASS.

## Gates

All completed with exit zero:

- go test ./...
- go test -tags pyoracle ./internal/infrastructure/language/python
- Compiled adapter test binary with PATH empty: PASS, proving the tested
  fact path does not require Python or another command at runtime.
- go run ./cmd/arclint check .
- make check
- make ci, including the race suite, build, example checks, schemas and secrets.

Python adapter race coverage: 87.1%.
The main repository reports zero active findings and 13 existing baselined
findings; both example checks report zero active findings and zero baselines.
No baseline, rule severity or exclusion was weakened.

Final make ci output:

```text
0 active finding(s) · 0 suppressed · 0 baselined · 28 rule(s) applied
spectral lint -F error -D --ruleset .spectral.yaml docs/schemas/rules.arclint.schema.json docs/schemas/domain.arclint.schema.json .arclint/schemas/rules.arclint.schema.json .arclint/schemas/domain.arclint.schema.json testing/boxoffice/.arclint/schemas/rules.arclint.schema.json testing/boxoffice/.arclint/schemas/domain.arclint.schema.json
No results with a severity of 'error' found!
gitleaks detect --no-git --verbose
11:26PM INF scanned ~7313070 bytes (7.31 MB) in 790ms
11:26PM INF no leaks found
```
