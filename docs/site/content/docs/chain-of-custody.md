+++
title = "Chain of custody"
description = "One path from collected observations to each Rule's supplied facts and reported results."
weight = 7
+++

Every Rule receives `Facts`. Native evaluators and extension adapters use the
same preparation and dependency interpretation. An evaluator implements its
Constraint; it does not collect facts, resolve memberships, or decide file access.

```text
Configuration → enabled Rules → required fact classes
                                      ↓
                         scan + language producers
                                      ↓
                         validated Observations
                                      ↓
                    shared membership/dependency indexes
                                      ↓
                       Facts prepared for each Rule
                              ↙               ↘
                     native evaluator    Sobek → SDK
                              ↘               ↙
                         evaluations and findings
                                      ↓
                        suppressions → baseline → report
```

## Collection and consumption have different jobs

`AssessConformance.Execute` selects Rules and unions their declared fact classes
through `requiredFacts`. The filesystem observer walks regular files under
`scan.exclude`, the testdata policy, and built-in directory exclusions. It skips
symlinks and does not consult `.gitignore`. Configured language producers return
normalized `LanguageFacts`: imports, declarations, calls, availability, and parse
failures. Producers currently collect imports even when only another class was
requested; the collection request is not an access boundary.

`Observations` validates and orders this input and copies mutable collections.
It already contains normalized facts. There is no second interpretation of an
import for native rules, extensions, or inspection. Source content is read on
demand through a capability; it is not an immutable copy of file bytes.

`conformance.Run` creates one private facts source for the check. It resolves
Zone membership once and indexes dependencies lazily. These indexes are private
query machinery, not additional domain fact models. Before dispatching each
Rule, `forRule` prepares its `Facts` from its Scope, Exclusions, and enforcement
requirements. All evaluators receive that value instead of observations or
membership indexes. The same preparation partitions selected and excluded file
subjects, so reporting and file access cannot select different files.

## One dependency meaning

`ImportsFor(path)` returns outgoing `DependencyImport` values. Each enriches the
original parsed import with its source path and both endpoint memberships.
`DependenciesFor(path)` returns incoming and outgoing evidence involving a
selected file. Both queries use the same conversion and membership resolution.

`Import.Target()` preserves resolution precision: an exact file, a package
directory, or unresolved. An exact target uses that file's memberships; a package
target uses the directory's union of memberships. A package import never becomes
an asserted dependency on every file in the package. A Go blank import is still
an import. Native graph edges derive from these same enriched imports.

Availability and failure remain separate. A supported empty import list is
observed absence; missing observations are unavailable; a parse failure makes
otherwise supported facts unusable. `FactsFor` preserves that distinction while
removing unrequested classes. SDK availability flags describe usable facts and
retain the failure separately as `parseError`.

## Subjects and evidence

Scope determines the subjects a Rule judges. Its Constraint determines the
evidence needed to judge them. The shared facts implementation preserves these
necessary differences:

| Consumer | Supplied evidence |
|---|---|
| File rules | Selected files after file Exclusions and their required classes |
| Import rules | Outgoing imports and target memberships |
| Zone graph rules | Dependency edges involving the selected Zones, including incoming sources outside them |
| Folder independence rules | Observed sibling Folder identities, including Folders whose files are excluded, and outgoing imports from selected files |
| Structure rules | The Zone's composition, including an excluded file that still witnesses a required member |
| Domain rules | Context declarations located through the matching Zone or repository fallback, after file Exclusions |
| CLI context/domain listings | A declaration inspection view through the same facts queries and domain locator |
| CLI dependency inspection | A repository inspection view of imports, target observation status, coverage, and diagnostics |

A Zone excluded as an evaluation subject is still evidence for another Zone's
cycle check. Graph nesting uses complete membership, not a restricted file list
that could make different Zones appear identical. These decisions live in the
facts implementation; individual evaluators do not reconstruct them.

Excluding a sibling Folder's only file does not erase that Folder as a dependency
target. Its existence remains evidence for the importing Folder's check; its file
content and facts remain inaccessible. A graph query without prepared graph
evidence returns an error, rather than an empty graph that could falsely conform.

Dependency endpoints grant no additional file access. A Rule about `order.ts`
can receive an incoming import at `handler.ts:7`, while reads, declarations,
and unrelated dependencies of `handler.ts` remain unavailable. Explicitly
excluded import sources are omitted from a selected file's incident evidence.

## Repository inspection

Repository inspection uses `NewInspectionFacts` with explicitly requested fact
classes. It does not construct a Rule. `context --dependencies` requests imports;
when it also locates recorded contracts, it requests declarations in the same
observation call. The resulting Facts supplies both reports through shared
membership and dependency queries. Coverage counts and diagnostic formatting
belong to the application report; it never reconstructs target membership.

`DependencyImport.TargetObserved` distinguishes an observed target outside all
Zones from a resolved target absent from the observations. The same value reaches
native consumers, dependency inspection, and the SDK. It grants no file access.

## The extension adapter only translates

The `ExtensionEvaluator` port receives `Facts`. Sobek maps those values to SDK
DTOs; it does not repeat Scope filtering, classify imports, or resolve Zones.
`ctx.imports(path)` and `ctx.facts(path).imports` share the same outgoing query.
`ctx.files`, `ctx.read`, `ctx.zoneOf`, and `ctx.zones` use the same selected files.
The [extension contract](/docs/extensions/#the-ctx-surface) documents the wire API.

`ctx.domain()` is an explicit exception to file Scope: it supplies the project's
recorded domain knowledge. Repository-authored and Pattern-supplied extensions
have the same capabilities. Each invocation uses fresh JavaScript state and
reuses compiled code, preventing retained contexts from crossing Rule invocations.

## Extension contract invariants

The published SDK declarations describe the calls and fact shapes provided by
the corresponding ArcLint binary. An extension receives the same public SDK
contract whether supplied by a repository or a Pattern.

An extension's parameters undergo the same validation and default application
through every supported execution path. With equivalent Rules and supplied
inputs, repository checks and Rule tests apply the same evaluation semantics and
access boundaries. Source attribution remains attached to the actual originating
Pattern or repository extension.

These are architectural invariants. The domain's Facts invariants own fact
meaning and access; SDK generation and execution tests enforce this delivery
contract. Rule tests compare fixture findings without applying a Baseline, so
equivalence concerns evaluation before Baseline application, not CLI exit codes.

## Findings preserve custody

Extensions report a selected `subjectPath`, defaulting to `path`. A different
reported location must match the source file and line of dependency evidence
supplied for that subject. `Facts.AllowsFinding` checks this before results are
accepted. A breach discards the extension invocation's findings and records
failed subjects and operational diagnostics. Other Rules continue.

The configured Rule owns Severity. Native evaluators decide their own Constraint;
extension findings retain heuristic assurance. Facts keep unavailable and failed
analysis distinct from observed empty results. Native import checks use those
states for unsupported and failed outcomes. Existing graph checks still judge
the supplied edges; missing edges do not establish complete observation coverage.
Repository-wide diagnostics report parse failures and unknown imports independently
of any Rule's restricted input.
Rule completion applies Suppressions; the application applies the Baseline and
reports stale entries without changing fact meaning.

## Where to make a change

| Change | Owner |
|---|---|
| Extract a source-language fact | Language producer and normalized observation shape |
| Change availability or required-class filtering | `conformance/facts.go` |
| Change selection or required native evidence | `conformance/facts_source.go` |
| Change dependency enrichment or indexing | `conformance/dependency_import.go`; target precision is `Import.Target()` |
| Change dependency inspection coverage or reporting | `application/observe_dependencies.go`, consuming prepared Facts |
| Add a Rule's judgment | Declare its requirements, then consume `Facts` in its evaluator |
| Expose a fact to TypeScript | Sobek's DTO mapping and Go wire types; generate the declarations |
| Change the SDK callable contract | `sdkAPIDecl` in `sobek/sdkinit.go`; generate `sdk/api_gen.ts` for the runtime source |

`facts_chain_test.go` scans every production file in the conformance package for
access to raw observations and indexes. It allows only the files implementing
Facts and named observation entry points. New helper filenames are covered too.
This is an architecture test within one Go package, not a Go visibility boundary.
Behavioral tests cover native/extension dependency parity, selected subjects versus
evidence locations, excluded sibling Folders, unavailable versus failed facts,
unprepared graph queries, and runtime isolation. The SDK generator test regenerates
wire types from the current Go structs and `api_gen.ts` from the Go contract string
into a temporary directory, then compares both with the committed outputs. The SDK
implementation imports the generated contract. The SDK declaration test checks the
published editor contract against the generated wire types and that same Go string.
`sdkinit_contract_test.go` also exercises every declared context call against the
host. `e2e_extension_contract_test.go` runs one compatibility fixture through
repository and Pattern loading, under both `check` and `rules test`. It checks
Go, TypeScript, and Python inputs for defaults, parameter rejection, incoming and
outgoing dependencies, observed empty imports, access boundaries, findings, and
source attribution. SDK drift checks cover both ArcLint and BoxOffice.
These checks make the intended path verifiable when another evaluator is added.
