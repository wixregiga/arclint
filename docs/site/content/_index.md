+++
template = "index.html"
+++

# Architecture contracts as data

arclint enforces module contracts over a repository: what each module may
**consume**, what it must **provide**, and the **invariants** that always
hold. Go, TypeScript, and Python, checked by one static binary. No Node,
no npm, no runtime dependencies.

```bash
arclint init      # pick runtimes and a pattern; writes rules.yaml
arclint check .   # enforce the contracts
```

Rules are plain YAML with a published JSON Schema. Every violation names
the broken contract and carries blame: a `consumes` break points at the
importing file, a `provides` break points at the module that failed its
promise.

```yaml
contracts:
  entities:
    consumes:
      internal: []          # no other internal modules
      external: forbid      # no third-party imports
    provides:
      - kind: registration  # every entity registers itself
        each: 'internal/entities/(?P<name>[^/]+)/'
        in: registry
        match: 'Register\("{name}"\)'
```

When YAML runs out, full rule logic is a TypeScript file in
`.arclint/extensions/`, executed in-process by the binary itself:

```ts
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

## Capabilities

- **consumes**: per-module allow and deny lists, third-party and stdlib
  policy, plus graph-wide layers, forbidden edges, independence,
  protected modules, and cycle detection.
- **provides**: registration obligations (every X registers itself) and
  correspondence obligations (every X has a matching Y). No surveyed
  tool checks this side declaratively.
- **invariants**: naming conventions, required and forbidden paths,
  content rules, and typed expr predicates.
- **Exact Go import analysis** proven against `go list` over pinned real
  repositories: 7,500+ imports, zero mismatches. Lexer-grade TypeScript
  and Python extraction with documented, test-asserted limits.
- **Fast**: ~13ms cold start; 5,000 files check in ~80ms.

[Get started](/docs/getting-started/) or read the
[concepts](/docs/concepts/).
