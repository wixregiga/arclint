# arclint — Design Proposal: Contract-Driven, Pluggable, Multi-Language

Proposal, 2026-08-10. Synthesized from six research reports (in `docs/research/`): [oh-my-pi-extension-architecture.md], [design-by-contract.md], [reference-tools-survey.md], [multi-language-rule-engines.md], [embedded-extension-runtimes.md]. Claims below reference those reports; their claims carry primary-source links. Syntax throughout is illustrative — shapes matter, spellings do not.

## 1. Goal

One static Go binary that enforces architecture contracts over any repository: multiple languages, rules as data in one YAML file, extensible with TypeScript rules that execute in-process with zero user-side toolchain. Nothing invented: configuration is plain YAML, predicates use an existing expression language (expr), and full logic uses TypeScript.

Driving scenario:

```bash
arclint load rules.yaml
arclint list
arclint rules ls
arclint check .
```

Driving rules (the acceptance tests for this design):

1. No third-party imports in `internal/entities/`.
2. Every feature is registered via the RegistryFactory in `internal/shared/registry.go`.
3. Everything in `internal/entities/*_{substrate}.go` matches the substrates in `internal/setup/**/*.go`.
4. A rule authored in TypeScript against an SDK, executed without TypeScript on the user's machine.

## 2. The contract model

Design by contract, applied for real rather than as naming (evidence: design-by-contract.md §4). A ruleset is a set of module contracts with three clause kinds:

- **consumes** (preconditions) — what a module may depend on: internal allow/deny, layer position, third-party and stdlib policy. This is the territory every existing boundary linter occupies (import-linter's contract taxonomy).
- **provides** (postconditions) — what a module must supply: required files, registration obligations, correspondence obligations. Research finding: no surveyed tool checks this side declaratively (multi-language-rule-engines.md §5) — it is the genuine differentiator.
- **invariants** — properties that always hold: naming conventions, forbidden paths, content rules (ls-lint-style closed vocabulary; reference-tools-survey.md §2).

Every violation carries **blame**: a `consumes` break points at the importing file (consumer fault); a `provides` break points at the module that failed its promise (provider fault). Meyer's blame assignment as a checkable output field, not vocabulary (design-by-contract.md §4).

The driving rules 1-3 as module contracts:

```yaml
runtime: [go]            # language targets: activates extractors + stdlib tables

modules:
  entities: ["internal/entities/**"]
  features: ["internal/features/*"]
  registry: ["internal/shared/registry.go"]
  setup:    ["internal/setup/**"]

contracts:
  entities:
    consumes:
      internal: []            # rule 1: no other internal modules
      external: forbid        # rule 1: no third-party imports
      stdlib: allow
    provides:
      - kind: correspondence  # rule 3
        of:   { files: 'internal/entities/*_(?P<substrate>[a-z0-9]+)\.go', value: "{substrate}" }
        in:   { files: 'internal/setup/**/*(?P<substrate>[a-z0-9]+)\.go',  value: "{substrate}" }
        relation: subset      # every entity substrate exists in setup

  features:
    provides:
      - kind: registration    # rule 2
        each: 'internal/features/(?P<feature>[^/]+)/'
        in: registry
        match: 'RegistryFactory\.Register\("{feature}"'
```

The `correspondence` primitive — derive a named set from path or content captures on one side, another set on the other side, assert a relation (`subset` | `equal`) — has no prior art in any surveyed tool and must be designed from scratch; everything else in the vocabulary is borrowed from proven tools (multi-language-rule-engines.md §5, "Viable paths"). `registration` is the special case of correspondence where the right-hand side is content captures in one file; it gets its own kind because it is the common case.

## 3. Layered rule surface — no invented DSL

Three layers; a constraint uses the lowest layer that can express it:

- **Layer 0 — declarative vocabulary (pure data).** Naming (ls-lint's closed case vocabulary plus `regex:`), structure require/forbid globs, content must/must-not regex, dependency contract kinds (import-linter's proven set: `layers`, `forbidden`, `independence`, `protected`, `acyclic`, plus per-module allow-lists), external/stdlib policy, correspondence/registration. Everything statically parseable; a published JSON Schema drives editor completion (schema pipeline in §4).
- **Layer 1 — expr predicates.** Where the vocabulary runs out but full code is overkill, rule params accept expr expressions (`expr-lang/expr`): an existing, documented language — type-checked at load time against the host's Go structs, side-effect-free, always terminating, production-embedded by Argo, CoreDNS, and the OpenTelemetry Collector (reference-tools-survey.md §5). Chosen over CEL because expr type-checks directly against native Go types with no protobuf adapter layer; CEL wins only when expressions must be portable across non-Go hosts. Example: `file.lines > 400 && !file.path.matches("_test")`.
- **Layer 2 — TypeScript SDK extensions.** Full rule logic in `.arclint/extensions/*.ts` (§5).

Explicitly rejected as the rule engine: grule and forward-chaining rule engines generally — salience and mutable working memory solve inference chains, while lint rules are stateless single-pass predicates; DRL strings surface type errors only at runtime (reference-tools-survey.md §4).

## 4. Pluggable architecture

**Everything is a rule-type provider** behind one interface, shaped like steiger's minimal rule contract (reference-tools-survey.md §1):

```
check(ctx, params) -> []Violation
```

`ctx` exposes host services, not the filesystem: the walked file tree (plain recursive folder/file values), file read, the per-language import graph, module membership, and capture-set derivation. Built-in providers implement layers 0-1; extensions add new rule types at layer 2. Rule *instances* in `rules.yaml` stay pure data either way: each names a type plus params, and params are validated against the provider's schema.

**The schema pipeline is the load-bearing idea** (ported from oh-my-pi's omptype invariant, which is the portable part of its design — oh-my-pi-extension-architecture.md §e): one schema definition yields both runtime validation and published JSON Schema.

- Built-in types: schemas generated from Go structs (`invopop/jsonschema`), merged into the published `rules.yaml` schema for editor completion.
- Extension types: the SDK's schema builder (zod v4-style, `z.toJSONSchema()` is now built into zod itself — embedded-extension-runtimes.md §E) declares params; the host receives the JSON Schema at registration and validates YAML params before ever invoking the extension.

**Two-phase lifecycle** from oh-my-pi (oh-my-pi-extension-architecture.md §b): extensions run a registration phase (declare rule types, schemas) strictly before the evaluation phase; runtime calls during registration are errors. Discovery: `.arclint/extensions/`, deduplicated by real path.

**Never native plugins.** golangci-lint's `.so` system demands exact build parity (every overlapping dependency version identical between host and plugin) and its replacement compiles a custom binary per user — both fail the zero-toolchain requirement (reference-tools-survey.md §3). A stdin/stdout JSON subprocess rule (SARIF-compatible output) can exist as a low-priority escape hatch for teams that already own scripts, but it is not the extension story.

## 5. TypeScript execution — the k6 pattern

esbuild (a Go library) transpiles TS in-process; sobek (grafana's maintained goja fork, pure Go) executes it. This is k6's production-proven architecture, reproduced during research: a Go binary fed `let x: number = 1; x+1` printed `2` with no Node, no npm, nothing user-side (embedded-extension-runtimes.md §A). Measured cost: **+14.6 MB** on the binary (measured table, same report). Alternatives ranked and rejected: Extism JS PDK (author needs `extism-js` + Binaryen; rules become opaque `.wasm`), raw wazero (cheap host, but rules stop being readable source), starlark-go (best sandbox and determinism at +1.2 MB, but Python-ish, fails the TS requirement — the documented fallback if the TS requirement is ever dropped).

Sandbox and determinism:

- A bare VM exposes only ES built-ins; the host injects a read-only API (`ctx`) and nothing else — no filesystem, no network, no Node shims.
- `Date.now` and `Math.random` are overridden with host-controlled implementations (the documented determinism gap of the engine).
- The engine's interrupt mechanism enforces per-rule timeouts.
- v1 extensions are single-file; esbuild bundles relative imports; bare npm imports are rejected with a clear error.

Author experience: `arclint sdk init` writes `arclint.d.ts` (generated from the host's Go types via `tygo`) plus a `tsconfig.json`, so authors get full editor typing without npm. Honest caveat carried from k6: esbuild strips types without checking them — type safety is an author-time editor concern, and the host validates params by schema instead.

```ts
// .arclint/extensions/handler-naming.ts
import { defineRule, s } from "arclint";

export default defineRule({
  type: "handler-naming",
  params: s.object({ suffix: s.string().default("Handler") }),
  check(ctx, params) {
    for (const f of ctx.files("internal/**/handlers/*.go")) {
      if (!f.stem.endsWith(params.suffix)) {
        ctx.report({ path: f.path, message: `handler files end in ${params.suffix}` });
      }
    }
  },
});
```

## 6. Multi-language import analysis — tiered and honest

`runtime: [go, ts, py]` declares targets, activating per-language extraction, stdlib tables, and manifest readers (`go.mod`, `package.json`, `pyproject.toml`) for third-party classification. Every import classifies as `internal` (resolves into the repo), `stdlib`, or `external` — which is what makes `external: forbid` (driving rule 1) expressible.

Tiered extraction (multi-language-rule-engines.md §§3-4, "Viable paths" ranking):

- **Go: exact and free.** `go/parser` with `ImportsOnly` — stdlib, precise, no size cost. Go's grammar structurally forecloses computed and function-scoped imports, so there is no false-negative class at all.
- **JS/TS and Python: lexer-grade extractors with documented false negatives.** Static `import`/`export from`/`require` and `import`/`from` forms are extracted; computed specifiers (`import(x)`, `require(v)`, `importlib.import_module(name)`) are undetectable statically at this tier and are documented as such. `node:` prefix and the embedded Python stdlib module list drive stdlib classification; relative specifiers resolve internally; manifest dependencies classify external.
- **Upgrade seam, not upfront cost.** Extraction sits behind an internal parser interface. If AST-grade analysis becomes necessary, the only route preserving `CGO_ENABLED=0` and one static binary is the pure-Go tree-sitter runtime (gotreesitter) with a small grammar subset — adoptable later, pinned, parse-failure-as-skip; its risks (young project, ~4x slower than C, reimplemented external scanners) are documented in the research. cgo tree-sitter and dlopen'd grammars are rejected outright: they forfeit the single-binary premise (ast-grep pays 48.7 MB and still needs per-OS shared objects for custom languages).

## 7. CLI

- `arclint load rules.yaml` — parse, schema-validate, semantically validate (module references, regex compilation), transpile and register extensions, validate every rule's params against its provider schema, cache the compiled ruleset. Prints what loaded: rule count by layer, targets, extensions.
- `arclint list` — one-line-per-rule summary of the loaded ruleset.
- `arclint rules ls` — detailed table: id, contract clause (consumes | provides | invariant), provider (builtin | extension name), targets, severity, description.
- `arclint check [path]` — evaluate against the tree; human output grouped by contract with blame shown; `--format json` emits the stable violation shape `{ruleId, contract, blame, severity, path, line?, message, fixHint}`; optional SARIF export for CI interop. Exit 0 clean, 1 violations, 2 config/usage error.

## 8. Rejected approaches — summary table

| Approach | Rejected because | Evidence |
|---|---|---|
| grule / forward-chaining engines | Inference machinery (salience, working memory) for stateless predicates; runtime-typed DSL | reference-tools-survey.md §4 |
| CEL | Protobuf-shaped typing needs an adapter layer; portability benefit unused in a single Go host | reference-tools-survey.md §5 |
| Inventing a DSL | Constraint; YAML data + expr + TS covers all three altitudes | this proposal §3 |
| Bun-style runtime embedding | Bun compiles Bun-hosted binaries; unavailable to a Go host — esbuild+sobek is the Go-native equivalent | oh-my-pi-extension-architecture.md §c, §e |
| Extism JS PDK / prebuilt wasm rules | Author-side toolchain (extism-js, Binaryen); opaque artifacts instead of readable `.ts` | embedded-extension-runtimes.md §B |
| golangci-style `.so` plugins | Exact build parity between host and plugin; per-user rebuild | reference-tools-survey.md §3 |
| cgo tree-sitter / dlopen grammars | Loses `CGO_ENABLED=0`, cross-compilation, or single-artifact distribution | multi-language-rule-engines.md §3 |
| semgrep-style dual-program | Two runtimes and a wrapper; contradicts one-binary cold-start goal | multi-language-rule-engines.md §2 |

## 9. Open questions

1. The exact YAML shape of `correspondence` (novel — no prior art to copy; needs a spike against real repos, especially content-derived sets and multi-capture values).
2. Baseline/grandfathering for adoption in repos with existing violations.
3. Extension distribution beyond repo-local files (a registry is explicitly out of scope until the SDK contract is stable — steiger's lesson: do not advertise third-party extensibility before the contract is stable).
4. Watch mode and editor integration (the published JSON Schema already gives editors completion for free).
