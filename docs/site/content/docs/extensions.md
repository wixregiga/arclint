+++
title = "TypeScript extensions"
description = "Full rule logic in .arclint/extensions/*.ts, executed by the binary itself."
weight = 4
+++

When the declarative vocabulary runs out, a rule becomes a TypeScript
file. The binary transpiles and executes it in-process (esbuild + sobek,
the k6 pattern): contributors and CI need no Node, npm, or tsc.

## Anatomy

```ts
// .arclint/extensions/no-cross-feature.ts
import { defineRule, s, type ImportInfo } from "arclint";

export default defineRule({
  type: "no-cross-feature",
  description: "Features must not import other features.",
  contract: "consumes",       // default clause for findings
  blame: "consumer",          // default blame side
  capability: "structural",   // how it enforces: exact | structural | heuristic | advisory
  params: s.object({
    root: s.string().default("internal/features").describe("Feature root."),
  }),
  check(ctx, params) {
    const root = params.root as string;
    for (const f of ctx.files(`${root}/**`)) {
      const from = f.path.split("/")[2];
      for (const imp of ctx.imports(f.path)) {
        if (imp.class !== "internal" || !imp.targetDir.startsWith(root)) continue;
        const to = imp.targetDir.split("/")[2];
        if (to !== from) {
          ctx.report({
            path: f.path, line: imp.line,
            message: `feature "${from}" imports feature "${to}"`,
            fixHint: "extract the shared rule into a package both features can use",
          });
        }
      }
    }
  },
});
```

Instances stay pure data in rules.yaml and are validated against the
declared schema before the extension runs:

```yaml
rules:
  - type: no-cross-feature
    params: { root: "internal/features" }
```

## The ctx surface

Rules see exactly this, and nothing else. No filesystem, no network, no
Node globals.

| call | returns |
|---|---|
| `ctx.files(glob?)` | repository files, optionally filtered by a doublestar glob |
| `ctx.read(path)` | one file's content |
| `ctx.imports(path)` | classified imports for every active language target, with `targetDir` and `targetFile` resolution |
| `ctx.modules()` | declared module names to member file paths |
| `ctx.facts(path)` | declaration facts for one file (lazy, cached): kinds, names, owners, visibility, line spans. Go facts are parser-exact; TypeScript and Python come from pinned tree-sitter grammars. `null` when no active target owns the file |
| `ctx.moduleOf(path)` | the sorted module names a file belongs to |
| `ctx.report(v)` | record one violation |

`ctx.report` accepts optional `severity`, `contract`, and `blame`
overrides per finding, so one rule type can label a provider-side broken
promise and a consumer-side bad import truthfully.

## Params schemas

`s` is a zod-style builder that produces JSON Schema at registration:
`s.string()`, `s.integer()`, `s.number()`, `s.boolean()`,
`s.enum(...)`, `s.array(items)`, `s.object(props)`, with `.optional()`,
`.default(v)`, and `.describe(text)`. The host applies defaults and
rejects bad params before `check` is ever invoked.

## Editor typing

```bash
arclint sdk init
```

writes `arclint.d.ts` (generated from the Go host types, so it cannot
drift) and a `tsconfig.json` into `.arclint/extensions/`. Full
completion, no npm install. Note that types are author-time only:
esbuild strips them without checking, and the host enforces the params
schema instead.

## Sandbox and failure

Extensions run on a bare ES runtime: `Date.now` and `Math.random` are
host-controlled, timeouts interrupt runaway rules (5s per phase), and a
crashing rule becomes an error-severity violation anchored at the
extension file, so CI fails visibly instead of silently skipping.
Relative imports are bundled; bare npm specifiers are rejected with a
designed error.
