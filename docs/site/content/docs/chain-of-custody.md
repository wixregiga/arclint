+++
title = "Chain of custody"
description = "How configured Rules become collected facts, bounded extension inputs, and reports."
weight = 7
+++

ArcLint keeps configuration, collected facts, and reported results connected
through one check. This page describes the existing implementation, including
where a Rule's Scope limits an extension's access.

```text
Load configuration and select Rules
    ↓
Determine which facts those Rules require
    ↓
Walk the repository under its scan policy
    ↓
Language adapters produce the requested facts
    ↓
Run native checks and extensions using those observations
    ↓
Collect results, apply the baseline, and report
```

## Configuration determines the work

`AssessConformance.Execute` loads the configured Rules and applies the command's
Rule selectors. `requiredFacts` collects and deduplicates the fact classes
declared by the enabled Rules' enforcement. ArcLint calls the observation
source once with those requirements, the configured languages, and the scan
policy, before running the Rules.

The observation source walks regular files under the repository root. It
applies `scan.exclude`, the `testdata` policy, and the built-in skipped
directories, including `.git`, `.arclint`, `vendor`, and `node_modules`.
Symbolic links are skipped. This walk does not consult `.gitignore`.
File discovery includes non-source files; configured languages determine
which language adapters produce facts.

The resulting observations contain file paths and metadata plus the available
language facts. Import records include locations and classified targets;
declaration records include names, kinds, and source locations. Source contents
for `ctx.read` are read on demand rather than copied into every extension's
inputs in advance.

## Shared collection does not grant shared access

ArcLint collects repository observations once per check invocation. It does
not construct a separate repository scan for each Rule. Those observations
are available to ArcLint's native implementations; they are not handed
directly to TypeScript extension code.

For a Rule using an extension, ArcLint resolves the selected files and applies
the Rule's path exclusions. The extension host uses that selected-file set to
restrict its file APIs:

| Operation | Boundary |
|---|---|
| `ctx.files(glob?)` | Lists only selected files, optionally narrowed by the glob |
| `ctx.read(path)` | Reads a selected file; an outside-Scope path throws an error |
| `ctx.imports(path)` | Returns available imports only for a selected file |
| `ctx.facts(path)` | Returns available declarations only for a selected file |
| `ctx.zones()` | Lists declared Zones with their selected member files |
| `ctx.zoneOf(path)` | Returns memberships only for a selected file |

Supplying a different path string does not expand that set. An outside-Scope
path supplies no imports or declarations. `ctx.domain()` is a documented
exception: it exposes the project's recorded domain knowledge, independently
of file Scope. The runtime provides no ambient filesystem or network API.

Repository-authored and Pattern-supplied TypeScript extensions use the same
host restrictions. The host also validates configured parameters against the
extension's declared parameter schema before calling its check function.
See the [extension API contract](/docs/extensions/#the-ctx-surface).

## Native import checks use the information their constraints require

The two import constraints ask different questions:

| Configuration | What ArcLint checks |
|---|---|
| `on: application` with `imports: { internal: [domain] }` | What Application imports, against its allowed targets |
| `on: adapter` with `imported_by: [factory]` | Who imports Adapter, against its allowed importers |

For the second Rule, an import in Application can violate a Rule about
Adapter. The native implementation examines repository dependency records,
keeps Adapter as the Rule's subject, and reports the offending import's file
and line. Imports within the selected Zone itself are permitted.

These native checks do not execute through the TypeScript host. Their access
to repository observations does not grant that access to bundled extensions.
Use the built-in constraint when it already expresses the required restriction.

## Results remain tied to the configured Rule

Extensions report locations and messages; the configured Rule owns Severity.
ArcLint checks every reported path against the selected files. An extension
that reports an outside-Scope path has its findings discarded, its selected
subjects marked failed, and operational diagnostics recorded. Other Rules
still contribute to the assessment. See [Scope breaches](/docs/extensions/#scope-breaches).

Unavailable facts and analysis failures are not proof of conformance.
ArcLint's result model distinguishes unsupported, failed, not-applicable,
undetermined, and conformance/violation outcomes. Extension authors must not
interpret an unavailable declaration result as proof that the declaration
does not exist. See [extension enforcement](/docs/extensions/).

Rule completion applies Suppressions; the application then applies the
committed Baseline to the assembled assessment, including a diagnostic for
stale entries. Rendering preserves the Rule identity and reported location.

## What is shared, and what is not

Fact collection is shared for the check invocation. That does not mean every
later calculation is cached or grouped: `evaluateGraph`, for example, builds
Zone dependency edges when each graph Rule runs. Performance work should
distinguish collecting facts from repeatedly processing those facts.

The implementation can be followed through `AssessConformance.Execute`,
`requiredFacts`, the filesystem observation source's `Observe`,
`conformance.Run`, `evaluateExtensionRule`, and the Sobek evaluator's `host`.
