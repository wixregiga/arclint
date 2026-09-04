+++
title = "TypeScript extensions"
description = "Full rule logic in .arclint/extensions/*.ts, executed by the binary itself."
weight = 4
+++

When the declarative vocabulary runs out, a Rule whose assertion is
`uses` delegates enforcement to a TypeScript file. The binary transpiles
and executes it in-process (esbuild + sobek, the k6 pattern):
contributors and CI need no Node, npm, or tsc. Extensions do not add
Rule Types; they supply enforcement for the finite `extension` Type.
Reach for one only after the built-in assertions run out: a line
pattern is a `content` Rule, not an Extension.

## Resolution

The directory that contains `rules.yaml` is the repository root and the
Extension root. Extensions load from `<root>/.arclint/extensions/`.
`--rules path/to/rules.yaml` moves the root and that directory together.
Without `--rules`, ArcLint discovers `rules.yaml` upward from the working
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
| `type` | yes | Registered extension name; `uses` in rules.yaml |
| `check` | yes | `(ctx, params) => void` |
| `description` | no | One-line summary |
| `capability` | no | Author claim: `exact` \| `structural` \| `heuristic` \| `advisory` (default `heuristic`) |
| `params` | no | Schema built with `s`; default empty object, no additional properties |

Default-export one `defineRule(...)` result, or an array of them. Duplicate
`type` names across entries fail registration.

Wire the Rule in `rules.yaml` with `uses` as its assertion. Severity,
identity, Claim, and Applicability belong to the Rule, not the
TypeScript file:

```yaml
modules:
  domain: "internal/domain/**"

rules:
  domain/no-io:
    description: "Domain performs no I/O; adapters do."
    # severity defaults to error when omitted
    on: domain
    files: "internal/domain/**/*.go"   # optional member-file narrow
    uses: forbid-imports
    with:
      packages: [bufio, database/sql, io, log, net, os, syscall]
```

`uses` is the registered extension name. `on` names the Module or
Modules whose members the Extension sees; omit it to inspect files
outside every declared Module, which is repository-scoped enforcement
with the same Rule Type. `files` narrows the selected files (one glob
or a list). `with` is validated host-side against the extension's
published schema before `check` runs, and is rejected on any Rule
whose assertion is not `uses`.

```yaml
rules:
  repositories/application-only:
    description: "Repository interfaces are declared only in application packages."
    uses: repository-location
    with:
      module: application
```

Extensions a Pattern carries are supplied to the runtime when the
Pattern is extended; nothing is copied into `.arclint/extensions`, and
the Pattern's Rules name them by the Pattern's own type names
(`vertical/forbid-imports`). A local Rule may not use a Pattern's
extension unless the Pattern is extended; the check fails with
`no extension registers rule "vertical/forbid-imports"`.

## The ctx surface

During `check`, the host lends exactly this read-only surface. File-scoped
calls are limited to the Rule's selected subjects: paths outside
Applicability are invisible to `files` / `imports` / `facts` / `moduleOf`
and unreadable via `read`. `ctx.domain()` is project-wide recorded
knowledge, not path-scoped. No ambient filesystem, network, or Node
globals.

| call | returns |
|---|---|
| `ctx.files(glob?)` | selected subjects as `FileInfo`, optionally filtered by a doublestar glob |
| `ctx.read(path)` | one selected file's content; throws when out of scope or unreadable |
| `ctx.imports(path)` | classified imports (`stdlib` \| `internal` \| `external` \| `unknown` \| `cgo`) with `targetDir` / `targetFile` when resolved |
| `ctx.modules()` | declared Module names to their **selected** member paths |
| `ctx.facts(path)` | declaration facts, or `null` when the language did not supply them |
| `ctx.moduleOf(path)` | sorted Module names containing the path (empty when out of scope) |
| `ctx.report(v)` | record one finding |
| `ctx.domain()` | the project's recorded domain model (`DomainInfo`); empty `contexts` and `relations` when none is recorded |

`ctx.report` accepts only:

```ts
{ path: string; message: string; line?: number; fixHint?: string }
```

`path` and `message` are required. Severity is not on the wire: the Rule
owns it. Legacy per-finding `severity`, `contract`, and `blame` fields are
ignored if present.

`ctx.domain()` returns read-only `DomainInfo` (camelCase JSON):

```ts
{
  contexts: Array<{
    name: string;
    entities: Array<{
      name: string;
      definition?: string;
      aliases?: string[];
      aggregate?: boolean;
      line: number;
    }>;
    valueObjects: Array<{
      name: string;
      definition?: string;
      aliases?: string[];
      line: number;
    }>;
    invariants: Array<{
      statement: string;
      owner: string;
      line: number;
    }>;
    assertions: Array<{
      statement: string;
      owner: string;
      id: string;
      on: string;
      line: number;
    }>;
    specifications: Array<{
      name: string;
      definition?: string;
      line: number;
    }>;
    events: Array<{
      name: string;
      definition?: string;
      aliases?: string[];
      line: number;
    }>;
    line: number;
  }>;
  relations: Array<{
    from: string;
    to: string;
    kind: string; // partnership | shared_kernel | customer_supplier | ...
    line: number;
  }>;
}
```

Collections are always arrays (empty when the project records none or
the file is absent). Each term lives inside a named bounded context.
`aggregate` is an entity designation, never a separate collection.
Invariants carry a statement and exactly one owner. Every entry carries
the `line` it is written on in `ubiquitous-language.yaml`, so a finding
about a context, term, invariant, or relation anchors at the entry
instead of at the top of the file:

```ts
ctx.report({
  path: "ubiquitous-language.yaml",
  line: term.line,
  message: `entity "${term.name}" has no definition recorded`,
});
```

`line` is 0 for a vocabulary that was not read from a file. Declaring
knowledge never creates a Diagnostic by itself; an Extension only
surfaces findings when its `check` calls `ctx.report`.

Rule Tests exercise `ctx.domain()` the same way they exercise files: a
fixture that authors `ubiquitous-language.yaml` at its tree root is
parsed with the production loader, and the extension under test
observes that vocabulary through `ctx.domain()`. Fixtures without one
see an empty model. See `vocabulary/terms-carry-definitions` in this
repository's `rules.yaml` and its cases under `.arclint/tests/` for a
complete example.

## Evidence honesty

ArcLint always treats Extension enforcement as heuristic, regardless of
the author's `capability` claim. Findings become suspected Violations at
the Rule's Severity (still gate when Severity is `error`). Subjects with
no findings evaluate undetermined, never conformance.

## Signature facts

When declarations are available, every `func` and `method` declaration
carries `params` and `results`. Types are whitespace-collapsed source
text, not resolved types, so signature comparison is structural rather
than proof.

Go facts are parser-exact. TypeScript and Python declarations come from
their pinned tree-sitter grammars. `ctx.facts(path)` returns `null` when
the file's language did not supply declarations.

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

## Applicability breaches

`ctx.report` accepts any path string. If any reported path falls outside
the Rule's selected subjects, the whole Extension run is untrustworthy:

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
