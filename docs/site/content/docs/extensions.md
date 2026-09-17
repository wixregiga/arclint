+++
title = "TypeScript extensions"
description = "Full rule logic in .arclint/extensions/*.ts, executed by the binary itself."
weight = 4
+++

When the declarative vocabulary runs out, a Rule whose constraint is
`uses` delegates enforcement to a TypeScript file. The binary transpiles
and executes it in-process (esbuild + sobek, the k6 pattern):
contributors and CI need no Node, npm, or tsc. Extensions do not add
Rule Types; they supply enforcement for the finite `extension` Type.
Reach for one only after the built-in constraints run out: a line
pattern is a `content` Rule, not an Extension.

## Resolution

The directory that contains `rules.arclint.yaml` is the repository root and the
Extension root. Extensions load from `<root>/.arclint/extensions/`.
`--rules path/to/rules.arclint.yaml` moves the root and that directory together.
Without `--rules`, ArcLint discovers `rules.arclint.yaml` upward from the working
directory (`check [path]` starts discovery from the optional path).

Discovery is top-level only: every `*.ts` or `*.js` directly under
`.arclint/extensions/` is one entry (dotfiles and `*.d.ts` are ignored).
Shared helpers live in subdirectories and are pulled in through relative
imports. A missing extensions directory is an empty registry, not an
error. When a configured Rule names an extension that never registered,
the error names the absolute directory that was searched.

## Anatomy

The built-in `arclint/vertical` Pattern ships this one, which forbids
named packages and their subpackages through the classified import
facts rather than by grepping source:

```ts
// .arclint/extensions/forbid_imports.ts
import { defineRule, s } from "arclint";

export default defineRule({
  type: "forbid-imports",
  description: "forbid imports of named packages and their subpackages",
  capability: "exact",
  params: s.object({
    packages: s.array(s.string()).describe("Import paths no selected file may import."),
  }),
  check(ctx, params) {
    const packages = params.packages as string[];
    for (const file of ctx.files()) {
      for (const imp of ctx.imports(file.path)) {
        for (const pkg of packages) {
          if (imp.path === pkg || imp.path.startsWith(pkg + "/")) {
            ctx.report({
              path: file.path,
              line: imp.line,
              message: `import of ${imp.path} is forbidden`,
              fixHint: "move the dependency behind a port the application layer owns",
            });
            break;
          }
        }
      }
    }
  },
});
```

`defineRule` accepts only:

| field | required | meaning |
|---|---|---|
| `type` | yes | Registered extension name; `uses` in rules.arclint.yaml |
| `check` | yes | `(ctx, params) => void` |
| `description` | no | One-line summary |
| `capability` | no | Author claim: `exact` \| `structural` \| `heuristic` \| `advisory` (default `heuristic`) |
| `params` | no | Schema built with `s`; default empty object, no additional properties |

Default-export one `defineRule(...)` result, or an array of them. Duplicate
`type` names across entries fail registration.

Wire the Rule in `rules.arclint.yaml` with `uses` as its constraint. Severity,
identity, Rationale, and Scope belong to the Rule, not the
TypeScript file:

```yaml
zones:
  domain: "internal/domain/**"

rules:
  domain/no-io:
    rationale: "Domain performs no I/O; adapters do."
    # severity defaults to error when omitted
    on: domain
    files: "internal/domain/**/*.go"   # optional member-file narrow
    uses: forbid-imports
    with:
      packages: [bufio, database/sql, io, log, net, os, syscall]
```

`uses` is the registered extension name. `on` names the Zone or
Zones whose members the Extension sees; omit it to inspect files
outside every declared Zone, which is repository-scoped enforcement
with the same Rule Type. `files` narrows the selected files (one glob
or a list). `with` is validated host-side against the extension's
published schema before `check` runs, and is rejected on any Rule
whose constraint is not `uses`.

```yaml
rules:
  repositories/application-only:
    rationale: "Repository interfaces are declared only in application packages."
    uses: repository-location
    with:
      zone: application
```

Extensions a Pattern carries are supplied to the runtime when the
Pattern is extended; nothing is copied into `.arclint/extensions`, and
the Pattern's Rules name them by the Pattern's own type names
(`vertical/forbid-imports`). A local Rule may not use a Pattern's
extension unless the Pattern is extended; the check fails with
`no extension registers rule "vertical/forbid-imports"`.

## The ctx surface

The [chain of custody](/docs/chain-of-custody/) explains when facts are
collected and where the host applies each Rule's file-access boundary.

The conformance domain supplies the same `Facts` used by native evaluators. It restricts
facts to the Rule's requirements, selected subjects, and Exclusions.
Dependency evidence may originate outside Scope when it involves a selected
subject; that evidence grants no access to the originating file. The adapter
translates this value into the SDK; it receives no repository-wide observations.

During `check`, the host lends exactly this read-only surface. File-scoped
calls are limited to the Rule's selected subjects: paths outside
Scope are invisible to `files` / `imports` / `facts` / `zoneOf`
and unreadable via `read`. `ctx.domain()` is project-wide recorded
knowledge, not path-scoped. No ambient filesystem, network, or Node
globals.

| call | returns |
|---|---|
| `ctx.files(glob?)` | selected subjects as `FileInfo`, optionally filtered by a doublestar glob |
| `ctx.read(path)` | one selected file's content; throws when out of scope or unreadable |
| `ctx.imports(path)` | classified imports (`stdlib` \| `internal` \| `external` \| `unknown` \| `cgo`) with `targetDir` / `targetFile` when resolved |
| `ctx.zones()` | declared Zone names to their **selected** member paths |
| `ctx.facts(path)` | available declarations, outgoing imports, incident dependencies, and Zone memberships for a selected subject; `null` outside Scope |
| `ctx.zoneOf(path)` | sorted Zone names containing the path (empty when out of scope) |
| `ctx.report(v)` | record one finding |
| `ctx.domain()` | the project's recorded domain model (`DomainInfo`); empty `contexts` and `relations` when none is recorded |

`ctx.report` accepts only:

```ts
{ path: string; message: string; line?: number; fixHint?: string; subjectPath?: string }
```

`path` and `message` are required. Severity is not on the wire: the Rule
owns it. Legacy per-finding `severity`, `contract`, and `blame` fields are
ignored if present.

`ctx.domain()` returns read-only `DomainInfo` (camelCase JSON), the
same shape `arclint domain --format json` prints:

```ts
{
  source: string;  // repository-relative path of the domain file
  project: string;
  contexts: Array<{
    name: string;
    definition: string;
    aggregates: Array<{
      name: string;
      definition: string;
      identity: string;      // the value object that identifies the root
      aliases?: string[];
      entities: Array<{ name: string; definition: string; identity?: string; aliases?: string[]; line: number }>;
      invariants: Array<{ key: string; statement: string; line: number }>;
      assertions: Array<{ key: string; on: string; statement: string; line: number }>;
      repository?: string;
      factory?: string;
      line: number;
    }>;
    valueObjects: Array<{
      name: string;
      definition: string;
      aliases?: string[];
      invariants: Array<{ key: string; statement: string; line: number }>;
      line: number;
    }>;
    events: Array<{ name: string; definition: string; raisedBy?: string; line: number }>;
    services: Array<{ name: string; definition: string; line: number }>;
    specifications: Array<{ name: string; definition: string; line: number }>;
    questions: Array<{ key: string; text: string; line: number }>;
    line: number;
  }>;
  relations: Array<{
    from: string;
    to: string;
    kind: string; // partnership | shared_kernel | customer_supplier | ...
    description?: string;
    line: number;
  }>;
}
```

Collections are always arrays (empty when the project records none or
the file is absent). Each term lives inside a named bounded context;
entities, invariants, and assertions live under the aggregate that
owns them, and a value object carries its own invariants. `source` is
the repository-relative path of the domain file (`domain.arclint.yaml`),
and every entry carries the `line` it is written on there, so a finding
about a context, term, invariant, or relation anchors at the entry
instead of at the top of the file, without the Extension spelling the
file name:

```ts
const domain = ctx.domain();
for (const context of domain.contexts) {
  for (const aggregate of context.aggregates) {
    if (aggregate.repository === undefined) {
      ctx.report({
        path: domain.source,
        line: aggregate.line,
        message: `aggregate "${aggregate.name}" records no repository`,
      });
    }
  }
}
```

`line` is 0 for a vocabulary that was not read from a file. The
built-in rules already judge the recorded language against the code
(see [Domain Contracts](/docs/contracts/)); an Extension adds what the
project wants beyond them, and surfaces findings only when its `check`
calls `ctx.report`.

Rule Tests exercise `ctx.domain()` the same way they exercise files: a
fixture that authors `domain.arclint.yaml` at its tree root is
parsed with the production loader, and the extension under test
observes that vocabulary through `ctx.domain()`. Fixtures without one
see an empty model. See `vocabulary/terms-carry-definitions` in this
repository's `rules.arclint.yaml` and its cases under `.arclint/tests/` for a
complete example.

## Evidence honesty

ArcLint always treats Extension enforcement as heuristic, regardless of
the author's `capability` claim. Findings become suspected Violations at
the Rule's Severity (still gate when Severity is `error`). Subjects with
no findings evaluate undetermined, never conformance.

## Dependency facts

Go, TypeScript, and Python supply observations through the same scoped Facts
contract and the same SDK methods. The observations retain language-specific
precision: a Go package target remains a directory, while a resolved TypeScript
or Python file target retains its exact path.

`ctx.facts(path)` includes already-collected imports by default alongside
`decls`. No opt-in, command, second query, or additional parse is needed.
The source location is `facts.path` plus each import's `line`; `path` on
an import is its original specifier. `class` retains `stdlib`, `internal`,
`external`, `unknown`, or `cgo`.

```ts
const facts = ctx.facts(file.path);
if (facts?.importsAvailable) {
  for (const imp of facts.imports) {
    if (imp.targetZones.includes("infrastructure")) {
      ctx.report({
        path: facts.path,
        line: imp.line,
        message: `Imports infrastructure through ${imp.path}`,
      });
    }
  }
}
```

`facts.zones` contains the selected source file's sorted Zone memberships.
Each import includes `targetDir`, `targetFile`, and sorted `targetZones`,
using the check's existing membership index. An exact file target uses
that file's memberships; a package-directory target uses the directory's
union of memberships and leaves `targetFile` empty. An unresolved target
has empty target paths and an empty `targetZones` array. Package resolution
never invents dependencies on individual files.

`targetObserved` says whether the resolved file or package directory is
represented in the observations. A target can be observed while `targetZones`
is empty because no declared Zone contains it. A false value for a resolved
internal target means it was absent from the scan, not necessarily from disk.

These are parsed import observations, not inferred runtime relationships or
Rule outcomes. Endpoint paths and memberships describe dependency evidence;
they do not expand file access. Even when an import names an excluded or
out-of-Scope file, `facts`, `imports`, `zoneOf`, and `read` still deny access
to that file, and `zones` lists only selected members. Reporting at a target
without supplied source-location evidence is a Scope breach. Extensions have no file-writing capability.

Availability is explicit:

- `importsAvailable: true` with `imports: []` means the file was observed
  with no imports.
- `importsAvailable: false` with `imports: []` means import observations
  are unavailable or parsing failed, not that there are no dependencies.
- `declarationsAvailable` independently describes `decls`; available
  imports remain accessible when declarations were not supplied.
- A supplied parse failure is retained as `parseError`; both availability
  flags are false and `imports` / `decls` are empty. Incoming dependency
  evidence observed in other files can still be available.
- A selected path without language observations still has a facts object
  with false availability flags. `null` means the path is outside Scope.
- Go blank imports (`import _ "package"`) are valid dependency observations.

`ctx.imports(path)` remains available and returns the same available import
occurrences. Use `ctx.facts(path)` when absence must be distinguished from
an observed empty import list.

When upgrading an extension that used `if (!facts)` to detect unavailable
declarations, use `if (!facts?.declarationsAvailable)` instead. A facts object
can carry imports or incoming evidence even when declarations are unavailable.
Run `arclint sdk init` in each consuming project after upgrading ArcLint to
refresh its editor declarations. Keep invocation-specific data inside `check`;
module state starts fresh for each invocation.

### Incoming dependencies

`facts.dependencies` contains the already-observed incoming and outgoing
imports involving the selected subject. Each entry has `sourcePath`,
`targetPath`, `targetKind` (`file`, `directory`, or `unresolved`), `specifier`,
`line`, `classification`, `sourceZones`, `targetZones`, and `targetObserved`. An import appearing
in two subjects' views has the same attributes in both. Directory targets
remain package evidence, even when a selected subject is a file in that package.

For a Rule selecting `domain/order.ts`, this reports an import from an
unapproved Zone at its actual source location:

```ts
const subject = "domain/order.ts";
for (const edge of ctx.facts(subject)?.dependencies ?? []) {
  if (edge.targetKind === "file" && edge.targetPath === subject &&
      !edge.sourceZones.includes("application")) {
    ctx.report({
      subjectPath: subject,
      path: edge.sourcePath,
      line: edge.line,
      message: `Only application may import ${subject}`,
    });
  }
}
```

`subjectPath` keeps the finding attached to the selected subject. A distinct
`path` and `line` must match a supplied dependency's source location. Omitting
`subjectPath` means the reported `path` is itself the subject. Knowing an
importer's path does not permit `ctx.read(importer)`, `ctx.facts(importer)`, or
traversing that importer's other dependencies. Explicitly excluded import
sources are omitted from the supplied dependency evidence.

ArcLint indexes existing observations once per check and projects only incident
evidence for each Rule. It does not rescan, reparse, resolve again, or build a
repository export for every invocation. Native dependency graph checks use the
same endpoint and membership interpretation. Missing observations and unresolved
imports remain limits of this evidence: an empty incident list does not prove
that no importer exists. Extension results remain heuristic.

## Signature facts

When declarations are available, every `func` and `method` declaration
carries `params` and `results`. Types are whitespace-collapsed source
text, not resolved types, so signature comparison is structural rather
than proof.

Go facts are parser-exact. TypeScript and Python declarations come from
their pinned tree-sitter grammars. Check `facts.declarationsAvailable`
before treating an empty `decls` array as an observed absence of declarations.

```ts
// Find(id string) (Member, error) becomes:
{ kind: "method", name: "Find", owner: "Repo",
  params: [{ name: "id", type: "string" }],
  results: ["Member", "error"] }
```

Parameters may carry `name`, `type`, `optional`, and `variadic`. Python
splats retain their prefix, TypeScript destructuring has an empty name,
and Go result names are dropped. Use these facts for syntax-level checks
such as arity and parameter shape.

## Params schemas

`s` is a zod-style builder that produces JSON Schema at registration:
`s.string()`, `s.integer()`, `s.number()`, `s.boolean()`,
`s.enum(...)`, `s.array(items)`, `s.object(props)`, with `.optional()`,
`.default(v)`, and `.describe(text)`. Object schemas set
`additionalProperties: false`. The host applies top-level defaults and
rejects bad `with` values before `check` runs.

## Editor typing

```bash
arclint sdk init
```

writes `arclint.d.ts` (generated from the Go host types, so it cannot
drift) and a `tsconfig.json` into `.arclint/extensions/`. Full
completion, no npm install. Types are author-time only: esbuild strips
them without checking, and the host enforces the params schema instead.

## Sandbox and failure

Extensions run on a bare ES runtime:

- Each invocation has a fresh runtime. Compiled extension code is reused,
  while closures, globals, and retained contexts are never shared across
  invocations or different Rule types in the same extension.
- `Date.now` and `Math.random` are host-controlled (deterministic).
- Registration and each `check` invocation time out after 5s
  (interrupt-based).
- Relative imports are bundled; bare npm specifiers are rejected with a
  designed error. Import the SDK as `"arclint"`.
- Transpile results cache under `<root>/.arclint/cache/extensions/`.

A crashing or timed-out extension fails the Conformance Check with an
error (check exits 2). It does not become a Violation and is not
silently skipped. During `arclint rules test`, the same failure is that
test's error; later tests still run.

## Scope breaches

`ctx.report` accepts path strings, which the domain validates. A finding must
name a selected subject. Its location must be that subject or the exact source
file and line of a supplied dependency involving it. If any finding violates
this boundary, the whole Extension run is untrustworthy:

- every finding from that run is discarded (none become Violations),
- each selected subject evaluates `failed`,
- excluded subjects stay `not_applicable`,
- error-severity operational Diagnostics name each breach,
- the Assessment stays complete so other Rules still report,
- the gate fails (exit 1) via those operational Diagnostics.

## Rule tests

Author fixture-backed tests under `.arclint/tests/` and run
`arclint rules test`. Extension `ctx.read` sees the authored fixture
bytes for each path, not the live repository file at that path, so a
production tree with different content cannot hide a case.

Expect the exact CLI-emitted Diagnostic messages (kind, path, line,
message). Start from `expect: []`, paste the unexpected findings the
CLI prints, and keep only the intended ones. Full authoring loop:
[Rule tests](/docs/cli/#rule-tests).
