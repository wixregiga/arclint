# arclint decisions log

Small ambiguities resolved in favor of the proposal's shapes, recorded per
milestone. Dates are decision dates.

## M10 (2026-08-14) — signature facts: ADR and gate results

Brandon's binding constraints, set 2026-08-13: one interface for every
language, shipped simultaneously; ts/py are not second-class; proceed
only if speed, size, and maintainability survive measurement. The plan
below passed his gate on 2026-08-14.

1. **Structured params/results, not a signature string**: `Decl` gains
   `params: [{name, type, optional, variadic}]` and `results:
   [typeText]`, identical for go/ts/py. The port-satisfaction failure in
   the field was name-subset comparison; the fix needs arity and
   per-parameter type comparison, and a single string would force rule
   authors to re-parse it — the heuristic reborn. Types are source
   text, whitespace-collapsed (`lang.NormalizeType`), never resolved:
   the M8 rejection of go/types stands, and capability labels stay
   honest (structural, not proof).
2. **One extraction rule per language, all syntactic**: Go slices type
   expressions out of the source by token offset (go/parser, exact
   AST, frozen by the Go 1 promise); TypeScript reads grammar fields
   `parameters`/`parameter`/`return_type` and nodes
   required_parameter, optional_parameter, rest_pattern,
   assignment_pattern, type_annotation; Python reads `parameters`/
   `return_type` and the five parameter node types (typed, default,
   typed_default, list_splat, dictionary_splat). Node names were
   verified against the pinned v0.49.0 grammars with a probe, not
   guessed.
3. **Information-preserving conventions** (Brandon delegated, decided
   2026-08-14; the test for each was "does the alternative lose
   information a rule might need"):
   - Python `self`/`cls` stay in the params list. Both sides of a
     Python-to-Python comparison carry them, so comparisons stay
     symmetric, and their absence is the only syntactic signal of a
     `@staticmethod`. Dropping them is interpretation, and trivially
     done rule-side.
   - Python splats keep their prefix in the name (`*args`,
     `**kwargs`): one `variadic` bool cannot distinguish the two
     flavors, a second field would be schema weight for one language's
     edge, and the prefixed name reads exactly like Python.
   - TS destructuring parameters keep `name: ""` with the type
     preserved: no single name exists syntactically, and inventing one
     from the pattern text would fabricate an identifier that no
     comparison should match on. Parameter names are not part of
     signature compatibility in any of the three languages.
4. **Unnamed and multi-name handling**: Go `a, b string` expands to two
   params; unnamed interface-method params keep `name: ""`; Go result
   names are dropped (results are types only); an unannotated ts/py
   return is an empty results list, indistinguishable from a Go func
   without results — documented, structural tier.
5. **Wire contract**: `ext.ParamInfo` mirrors `lang.Param`; the SDK
   `.d.ts` regenerates from the Go structs (tygo), so the extension
   view cannot drift.
6. **Known limits recorded**: TS function-typed properties
   (`foo: (x) => void`) remain `field` decls without signatures;
   Python quoted annotations keep their quotes (`'Book'`).

### M10 signature-facts gate results (measured 2026-08-14, WSL2 Ubuntu 24.04, go1.26.4)

- Speed, interleaved A/B on one identical corpus (baseline worktree at
  eef3f13; six samples per side): Go 272 real files (arclint +
  shelfbook + gin oracle cache) 37.4ms → 39.7ms per corpus pass,
  about +6%, ≈ +8µs per file; TS/JS 13 files and Python ~150 CPython
  stdlib files both within run-to-run noise.
- Check path unaffected: facts stay lazy and cached
  (internal/engine/facts.go untouched); selfcheck 127 files in 9ms;
  cold-start median 13.9ms against the 100ms bound; 5k-file bench
  passes.
- Binary size: 27,910,306 → 27,926,690 bytes (+16,384, +0.06%), same
  grammar blobs.
- Contract pinned per language: TestGoSignatureFacts,
  TestTypeScriptSignatureFacts, TestPythonSignatureFacts assert exact
  structs; a grammar update that renames a node or field fails them
  deterministically.

## M9 (2026-08-13) — uniform except clauses

Brandon's requirement, verbatim in spirit: a builtin-derived rules.yaml
must be able to exempt a specific thing from a rule that should keep
firing, uniformly, via YAML, regardless of which mechanism produced the
finding.

1. **One shape everywhere**: every clause kind (consumes, provides,
   invariants including expr, all five graph kinds, extension
   instances) accepts `except: [{paths, reason}]`. Uniformity rides on
   an existing engine guarantee: every finding carries exactly one
   anchor path, so suppression filters findings by rule id + anchor
   glob, independent of language, kind, or provider.
2. **Reason is required** (validated): an exception is policy, and the
   YAML is its audit trail. No owner/expiry fields: expiry would make
   check results depend on the wall clock, breaking determinism; revisit
   only with the baseline work.
3. **Never silent**: Result.Suppressed counts dropped findings and the
   human summary prints "N suppressed by except". The JSON violations
   array keeps its stable shape.
4. **Id-scoped grouping**: clauses sharing one explicit id merge their
   except lists, consistent with M7's requirement grouping; derived
   consumes ids resolve through the same per-aspect trimming as
   capability labels.
5. **Documented caveat**: acyclic findings anchor at a witness edge
   inside the cycle, so path excepts there are less direct; recorded in
   the `except` entry of the single-source doc table (which also feeds
   the schema hovers and the generated reference).

### M9 addendum (2026-08-13): suppression visibility

6. **`check --show-suppressed`** lists what was omitted: human output
   gains a SUPPRESSED section (finding plus its except reason); under
   `--format json` the suppressed findings join the stable array shape
   carrying additive `suppressed: true` and `suppressedReason` fields,
   still never affecting the exit code. Result.Suppressed became the
   findings themselves (marked, with reasons) rather than a bare count.
7. **`rules show <id>`** displays a requirement's exceptions beside its
   clauses (paths and reasons, human and JSON), completing the
   visibility triangle: the YAML declares, rules show displays the
   declaration, check --show-suppressed displays the effect, and the
   always-on count keeps suppression from ever being invisible.
8. A separate small commit added the published-schema drift test (the
   committed rules.schema.json now fails CI when stale, mirroring the
   generated-docs guard); its first version was itself committed red
   because a piped test invocation masked the exit code — recorded here
   as a process lesson: never pipe the gate.

### M9 gate results (measured 2026-08-13, WSL2 Ubuntu 24.04, go1.26.4)

- Engine test proves suppression across three clause families in one
  tree with counts; e2e proves it for extension instances through the
  binary with visible output; validation rejects missing reasons, empty
  paths, and bad globs.

## M8 (2026-08-13) — language facts: ADR and spike results

The parity law from M7 applied to syntax facts. Spike artifacts ran in a
scratch module against real files (arclint's own SDK TypeScript, the
feature-slice extension, CPython 3.12's dataclasses.py).

1. **gotreesitter v0.49.0 adopted, pinned, for TypeScript/TSX/JavaScript
   and Python facts.** It is the only route preserving CGO_ENABLED=0 and
   one static binary (research: multi-language-rule-engines.md §3).
   Spike evidence: pure-Go, arm64 cross-build clean; a subset build
   embedding only our four grammars is 12.7 MB against 31.8 MB with all
   206; parse+extract runs ~1-7ms per file (~0.6-0.8µs/byte, in line
   with the research's ~4x-slower-than-C figure). Risks stay recorded:
   the project is six months old with a single maintainer; the version
   is pinned; a parse failure yields absent facts plus a warning, never
   a crash.
2. **Go facts come from go/parser**, not tree-sitter: exact, free, and
   already the foundation the oracle proved. Uniformity of the SCHEMA
   matters; uniformity of the parser behind it does not.
3. **Facts are lazy and cached per file.** At ~1-7ms per parse, eager
   whole-repo facts would dwarf the 77ms 5,000-file check budget; only
   files a rule asks about are parsed, in parallel, once.
4. **arclint owns its fact queries.** The library's prebuilt FactProgram
   was probed and found insufficient: TypeScript interfaces, arrow-bound
   functions, class fields, and TS import statements were not extracted
   (Python fared better). The fact schema is therefore defined by
   arclint's own per-grammar tree-sitter queries, unit-tested per
   language; the M3 import pipeline remains the only import source
   (classified, resolved), and fact imports are not used.
5. **Build tags**: release and dev builds pass the grammar subset tags
   through one Makefile variable; a tag-less `go test ./...` still works
   (all grammars embedded in test binaries, correctness unchanged).

### M8 implementation decisions and gate results (2026-08-13)

6. **The fact schema** (internal/lang/facts.go): Decl {kind, name,
   owner, exported, startLine, endLine} with a small cross-language kind
   vocabulary. Documented edges, asserted by tests: plain JavaScript
   without ESM export markers is unexported (CommonJS module.exports is
   invisible structurally); Python spans anchor at the def line,
   decorators excluded; Python visibility is the underscore convention.
7. **ctx.facts and ctx.moduleOf** join the extension surface; moduleOf
   ends the alias-duplication the dddflat report complained about (path
   roots defined once in rules.yaml, never repeated in extensions).
8. **ddd-flat ships as arclint/ddd-flat**, ids ddd:ARCH-001..012.
   ARCH-001/008 moved fully declarative (consumes ids and structure
   forbid, which v1 could not express); the extension half runs on facts
   and replaced the predecessor's ~110-line hand-rolled Go tokenizer.
   ARCH-005 keeps the ShelfBook semantics (handler-name prefixes; any
   loop/switch or state mutation flags) after the parity check caught my
   looser first draft missing their handler finding.
9. **Parity proven mechanically**: the shipped pattern, initialized into
   the unmodified ShelfBook dddflat repository (aggregates configured),
   reports 11 violations (7 error, 3 warn, 1 info) — the same set, ids,
   anchors, and severities as the hand-built ruleset it replaces, plus
   capability labels on every finding.
10. **Cost recorded**: the four grammar blobs plus the tree-sitter
    runtime add ~6.8 MB (binary 27.9 MB, was 21.1 MB). Grammar subset
    tags live in one Makefile variable; a tag-less `go test ./...`
    embeds all grammars in test binaries and stays correct.
- Gates: full suite green including 53 pattern-suite cases (ddd-flat 14,
  all ids covered); self-hosting clean, 124 files in 9ms. Cold start
  median 16.2ms (bound 100ms); 5,000-file check median 86.5ms (facts are
  lazy: the check path does not parse). Static builds 27.89 MB amd64 /
  25.69 MB arm64 with the grammar subset tags.

## M7 (2026-08-13) — rule identity, capability labels, rule tests

Direction settled from the ShelfBook dddflat exercise: an external agent
implemented all twelve ARCH-* DDD requirements on v1 and reported back
(the report and its work sit in ../shelfbook/dddflat). Its ruleset needed
a 427-line extension whose first ~110 lines were a hand-rolled Go
tokenizer, it grouped two dependency rules under one requirement id by
accident (nothing validated id sharing), and consumes clauses could not
carry ids at all.

1. **LSPs rejected as the analysis engine.** Language servers are
   external per-language processes (gopls; tsserver and pyright need
   Node), which contradicts the premise the extension e2e proves with an
   empty PATH; their results are editor-shaped and version-dependent.
   omp uses LSPs for agent code intelligence, a different job
   (research: oh-my-pi-extension-architecture.md, lsp-config citation).
   Language facts (M8) are the deterministic in-process slice of the
   symbol tier.
2. **Language parity is a design law.** Analysis interfaces are designed
   language-neutral and every target implements and tests them the same
   way. M7 delivers it at the pattern tier: builtin suites carry real
   TypeScript and Python fixture cases, not just Go. M8 carries it into
   syntax facts (ADR + spike on the pure-Go tree-sitter runtime, per
   multi-language-rule-engines.md §3).
3. **Namespaces.** pattern.yaml declares `namespace:`; builtin patterns
   are qualified `arclint/<name>`; a pattern's rule ids carry the
   `<ns>:` prefix. Consumes clauses gain `id:` (dddflat could not bind
   ARCH-001/ARCH-011 ids to consumes clauses). The engine honors an
   explicit consumes id across every aspect — the layers suite caught
   the engine still emitting derived per-aspect ids before the fix.
4. **Id grouping is now a feature, not an accident**: several clauses
   sharing one explicit id form one requirement; `arclint rules show
   <id|namespace>` lists the group. Derived consumes ids keep their
   per-aspect suffixes.
5. **Capability labels** state how a rule enforces its claim:
   exact (import graph / syntax facts), structural (paths,
   declarations), heuristic (names, text regex, complexity), advisory.
   Builtin kinds are labeled in one table (content is heuristic — it is
   a text regex); extensions declare theirs in defineRule, defaulting to
   heuristic, the conservative claim. Every violation carries its
   capability. This answers the dddflat report's core finding: "the
   weak point was proving that semantic heuristics mean what their rule
   descriptions claim."
6. **Rule test framework** (internal/ruletest): YAML cases materialize
   files into a fresh tree and assert the COMPLETE violation set —
   unexpected findings fail a case like missing ones, and an empty
   expect list asserts cleanliness. Minimal per-language manifests are
   injected unless the case brings its own; a case may override
   rules.yaml. `arclint rules test` runs .arclint/tests or explicit
   paths against the repo's ruleset; `--pattern` runs a pattern's
   bundled tests/*.yaml against its own template.
7. **Patterns are curated test suites.** Every builtin ships per-runtime
   cases (layers: go, ts, py) and the e2e gate enforces coverage: every
   namespaced rule id must appear in at least one expectation. The
   suites immediately caught two real defects: the engine ignoring
   explicit consumes ids (above) and a redundant shallow-hierarchy glob
   (`**` matches zero segments, so one glob covers all depths).
8. **Deferred by decision**: baseline/adoption mode (Brandon: not ready);
   pattern distribution via npm (identity is designed so scoped names
   slot in; the registry itself waits, per the steiger lesson in the
   research).

### M7 gate results (measured 2026-08-13, WSL2 Ubuntu 24.04, go1.26.4)

- 39 pattern-suite cases pass through the compiled binary (starter 4,
  layers 14 across go/ts/py, feature-slice 21), with the coverage gate
  proving all 22 namespaced ids are exercised.
- `go vet ./...`, `go test ./...` green; self-hosting clean: 21 rules
  over 14 modules, 108 files in 8ms (ruletest joined the contracts; the
  report module's protected rule gained it as an allowed importer).
- Cold start median 17.7ms over 11 runs (bound 100ms; the embedded
  suites and two new commands cost ~5ms over M5); 5,000-file check
  median 77.1ms. Static builds 21.09 MB amd64 / 19.46 MB arm64. Docs
  site builds clean (zola, 65ms).

## M6 (2026-08-12) — docs site

1. **Zola over Starlight.** The handoff allowed either ("Starlette"
   read as Starlight); both render markdown. Zola wins on the same
   ground the tool itself stands on: one static binary, no Node
   toolchain in a repository whose selling point is needing none. It is
   already available via mise on the dev machine (zola 0.19.2).
2. **Custom minimal templates instead of a theme**: the standing visual
   constraints (true black background, white primary text,
   information-dense, no decorative chrome) cost ~90 lines of inline CSS
   to meet directly and a lot more to impose on a theme. Four Tera
   templates: base, landing, docs section, docs page.
3. **Content is plain markdown** under docs/site/content/ (the explicit
   requirement). Pages: landing, getting started, concepts (modules,
   import classes with the exact meaning of internal/external/stdlib/
   unknown, contracts, blame), rule reference, extensions, patterns,
   CLI.
4. **The rule reference page is generated** (tools/gendocs) from the
   same RuleDocs table that drives `arclint explain` and the schema
   hovers; a test fails the suite when the committed page drifts from
   the table. Terminal docs, editor hovers, and the site cannot
   disagree.
5. **`make docs` builds the site; it is not part of `make ci`** because
   zola is not assumed on every machine; the drift test IS in the normal
   suite, so stale generated docs still fail CI everywhere.

### M6 gate results (measured 2026-08-12, WSL2 Ubuntu 24.04)

- `zola build`: 6 pages, no orphans, 68ms (zola 0.19.2).
- `go test ./tools/gendocs/` green (reference page current); full
  `make ci` green.

## M5 (2026-08-12) — patterns and init

1. **A pattern is a directory bundle**: `pattern.yaml` (description +
   compatible runtimes), a complete and loadable `rules.yaml` template,
   and optional `extensions/*.ts`. Built-ins are embedded
   (internal/patterns/builtin, go:embed); repository-local patterns live
   under `.arclint/patterns/<name>/` where nested names (`fsd/go`) are
   legal and a local name shadows a builtin. Patterns do NOT live in
   `.arclint/extensions/` because that directory means "rule types
   registered in this repo" and mixing bundles into it would blur both
   contracts.
2. **Templates stay valid YAML.** Runtime selection rewrites the single
   `runtime:` line at materialize time instead of using template
   placeholders, so every shipped template loads as-is and the pattern
   gate can exercise it directly.
3. **Shipped set**: `feature-slice` (go; the open-set variant proven in
   the pattern-modeling experiments: features and concepts classified by
   shape, zero enumerated lists, paired extension for the dependency
   matrix), `layers` (go/ts/py hexagonal starter, fully declarative), and
   `starter` (one module, unknown imports surfaced, teaches growth via
   `arclint explain` pointers). The feature-slice extension now labels
   findings truthfully with M4's per-finding overrides: the
   missing-port finding reports provides/provider, drift reports
   invariant/provider — the exact limitation the experiments documented
   is gone.
4. **Module globs are repository-specific by nature**: layers/starter
   templates ship go-flavored defaults plus `src/`-style alternates and
   tell the user to check `arclint module ls`; init's closing output
   points there. The wizard does not ask for paths in v1.
5. **`arclint init`** prompts only for what flags do not provide
   (`--runtimes`, `--pattern`, `--force`): detected languages (file
   counts via lang.TargetOf) become the runtime default; compatible
   patterns are listed with descriptions; everything written is then
   validated exactly as `arclint load` would, extensions included, and
   SDK typings are generated when the pattern ships extensions.
6. **The pattern gate runs through the real binary** (cmd/arclint e2e):
   every builtin × supported runtime must init and check clean on an
   empty tree, and the feature-slice pattern is proven against a real
   repository shape both ways (conforming fixture clean; violating
   fixture fires findings from every enforcement source, including the
   provider-blamed port finding). The gate lives in e2e rather than the
   patterns package so `patterns` keeps a leaf contract
   (`consumes.internal: []`) in rules.yaml — self-hosting caught exactly
   that edge during development, again.

### M5 gate results (measured 2026-08-12, WSL2 Ubuntu 24.04, go1.26.4)

- `go vet ./...`, `go test ./...` green; self-hosting clean: 19 rules
  over 13 modules, 83 files in 6ms.
- Extra sanity outside the committed fixtures: the shipped feature-slice
  pattern, installed by `arclint init` into the pattern-modeling demo
  repos, keeps the conforming repo at exit 0 and the 14-violation-class
  repo at exit 1.
- Cold start median 12.48ms over 11 runs (bound 100ms; embedded patterns
  cost nothing measurable); 5,000-file check median 77.6ms. Static builds
  21.00 MB amd64 / 19.33 MB arm64.

## M4 (2026-08-12) — ADR: TypeScript as the single rule IR

Evaluated against what M2 already built: `defineRule` + the zod-style `s`
builder emitting JSON Schema, host-side param validation before any
extension code runs, ctx {files, read, imports, modules, report},
arclint.d.ts generated from the Go host types via tygo, and no-Node
execution (esbuild + sobek). The gap: builtin kinds were Go-only with no
shared metadata; ctx was not runtime-parameterized (ImportInfo dropped
TargetFile, and its docstring still claimed go-only imports); extensions
could not label a finding's contract per finding.

1. **Builtins stay Go; TypeScript as the execution IR is rejected.**
   Reimplementing the 14 builtin kinds in TS would move the hot path into
   the sobek interpreter (Go builtins: 5,000 files in ~92ms median, M3
   gate below) and make validation circular (the schema pipeline that
   validates YAML is Go-side). Nothing the IR would buy requires it.
2. **Adopted instead: one metadata record per rule TYPE** — the portable
   half of the idea. `internal/config/ruledoc.go` documents every builtin
   kind ({kind, where, clause, blame, summary, doc, example}); the same
   table patches `description` fields into the published JSON Schema
   (editor hovers), drives `arclint explain [kind]`, and will feed the
   M6 docs-site reference. Extension types self-describe through
   defineRule's new `description` field; `arclint explain` merges both
   sources. `ruledoc_test.go` asserts the table covers exactly the
   schema's kind enums — no drift in either direction.
3. **Module descriptions.** A `modules:` value is a glob list (terse
   form) or `{paths, description}`. `arclint module ls` and
   `arclint module info <name>` show description, member-file count, and
   the languages present among members, derived from the files through
   `lang.TargetOf` — now the single extension-to-target mapping, which
   the jsts/python analyzers also select through.
4. **ctx gap closed.** ImportInfo gains `targetFile` (file-granular
   resolution for JS/TS and Python, engine-side since M3 but previously
   dropped at the wire type); `imports()` is documented for every active
   target; ViolationInput accepts per-finding `contract`/`blame`
   overrides, validated host-side. This removes the "one contract per
   extension rule type" limit the PATTERN1 experiments documented (a
   consumes-typed extension had to blame consumers for provider-side
   findings).
5. `rules ls` gains a MODULE column; extension descriptions replace the
   generic "extension rule type X from Y" text in listings.
6. The differential oracle was not re-run for this milestone: it covers
   Go classification only, and M4 did not touch the Go analyzer; the
   jsts/python file-selection refactor is behavior-equal and covered by
   the extractor test matrices.

### M4 gate results (measured 2026-08-12, WSL2 Ubuntu 24.04, go1.26.4)

- `go vet ./...` and `go test ./...` green; self-hosting clean: 17 rules
  over 12 modules, 71 files in 5ms, with arclint's own modules now
  carrying descriptions (dogfooding the mapping form).
- e2e: `module info`, `explain` (builtin kind and extension type), and
  the per-finding contract/blame override asserted through the compiled
  binary's JSON output.
- Cold start median 13.19ms over 11 runs (bound 100ms); 5,000-file check
  median 75.9ms over 3 runs. Static builds 20.93 MB amd64 / 19.33 MB
  arm64 (CGO_ENABLED=0), in line with M3.

## M1 (2026-08-10)

1. **Module path** `github.com/wixregiga/arclint`, matching the `origin`
   remote of the repository.
2. **Graph-wide consumes clauses live in a top-level `dependencies:`
   list** (`layers`, `forbidden`, `independence`, `protected`, `acyclic`),
   because they span modules; per-module `consumes` carries the
   allow/deny, external, and stdlib policies. All report
   `contract: consumes`, `blame: consumer`.
3. **Module glob semantics**: a module glob matches files directly, and a
   glob that names a directory owns the whole subtree
   (`internal/features/*` covers `internal/features/alpha/alpha.go`),
   matching the proposal's `features: ["internal/features/*"]` shape.
   Membership is a set: overlapping module globs are legal.
4. **Internal allow-list semantics**: with `internal: []` (or any
   allow-list), an internal import must resolve to self or an allowed
   module; imports of internal code outside every declared module also
   violate. A `{deny: [...]}`-only mapping restricts nothing else.
5. **Protected is importer-centric**: a file is an allowed importer of a
   protected module when ANY of its modules is in the allow set, so
   overlapping umbrella modules do not create false violations
   (surfaced by self-hosting, fixed in the provider, kept as behavior).
6. **Classification** (also the oracle's ground-truth mapping): internal =
   under the owning module's path, or resolved through a local-directory
   `replace`, or a root `go.work` workspace member (go list equivalent:
   `.Module.Dir` inside the repo). Sibling modules in one repo without
   replace/workspace classify **external** (resolvable via require).
   Replace-to-local pointing outside the repo classifies internal with no
   tree directory. Version pinning on `replace old@v` is ignored at
   classification level.
7. **Unknown imports** (not stdlib, not internal, not covered by any
   require): repo-wide policy `scan.unknown_imports: warn|error|ignore`,
   default warn. Repos with no `go.mod` classify every non-stdlib import
   unknown.
8. **Files the go tool never considers** (path segments starting with `.`
   or `_`) are not import-analyzed; build-constrained files ARE analyzed,
   and the divergence from `go build` is asserted in the oracle as exactly
   the `IgnoredGoFiles` set. A second documented divergence class:
   testdata packages explicitly imported by tests (gin's protoexample) are
   covered by `go list` but excluded by arclint unless
   `scan.include_testdata` is set.
9. **Walker exclusions**: `.git`, `.hg`, `.svn`, `.arclint`, `vendor`,
   `node_modules` by name; `testdata` unless configured; symlinks never
   followed; `scan.exclude` adds doublestar globs.
10. **Rule ids**: explicit `id:` wins; defaults are
    `<module>.consumes.<aspect>`, `<module>.provides.<kind>[<i>]`,
    `<module>.invariants.<kind>[<i>]`, `dependencies.<kind>[<i>]`,
    `scan.unknown-imports`.
11. **Severity and exit codes**: severities `error|warn|info`, default
    error; `check` exits 1 only when an error-severity violation exists;
    2 for config/usage errors. Unknown-import warnings go to stderr, not
    the violation list.
12. **Cache** (`.arclint/cache.json`, written by `arclint load`): records
    the rules.yaml SHA-256 + arclint version; later commands skip schema
    and semantic validation on a fingerprint hit. Compiled artifacts are
    not serialized in M1 — regex/expr compilation is microseconds; the M2
    extension pipeline will extend the cache with transpile results where
    caching actually pays.
13. **`check [path]` selects the repository** (rules.yaml discovered
    upward from path, or `--rules`); it does not narrow checking to a
    subtree, because provides/graph contracts are repo-wide by nature.
14. **Correspondence sides derive over the whole tree** (the `files`
    regexes are anchored full-path matches and constrain scope
    themselves); `each` in registration rules is unanchored over the
    module's file paths and its full match (trailing `/` trimmed) anchors
    the violation. Match templates substitute regex-quoted capture
    values; value templates substitute raw values.
15. **Naming rules** apply to the file stem (base name minus final
    extension), ls-lint style; `regex:` alternatives are anchored.
16. **`runtime: [ts]`/`[py]` are schema-valid but rejected at load** until
    M3 implements those targets ("not supported yet" error, not silent).
17. **SARIF export deferred** (optional in the proposal, not in the M1 CLI
    contract); the `--format json` shape is the stable interchange.
18. **Oracle repo roster** (pinned SHAs in
    `internal/oracle/oracle_test.go`): spf13/cobra and gin-gonic/gin
    (single module), go-testfixtures/testfixtures (root go.work, 3
    members), opencontainers/runc (vendor/ + cgo), opentelemetry-go (28
    modules, sibling replaces). docker/cli was rejected because it has no
    go.mod (uses a custom `vendor.mod`), kubernetes/kubernetes because a
    multi-GB clone makes a poor recurring oracle.

## M3 (2026-08-10)

1. **Shared language model** (`internal/lang`): every target produces
   `lang.Import` with a Class and a tree-resolved target; the Go package
   aliases its old types onto it. File-granular languages set TargetFile;
   package-granular Go sets TargetDir; the engine's module lookup honors
   both.
2. **JS/TS extractor** (lexer-grade per research §4): comments blanked,
   string/template bodies masked, regex literals handled with the
   standard lexer heuristic; extracted forms are import/from,
   side-effect import, export-from (incl. `export type`), literal
   `import()` and literal `require()`. Documented false-negative classes,
   asserted by tests: computed `import(x)`/`require(v)`, template-literal
   specifiers. `import type` is extracted (runtime-vs-type distinction is
   below this tier); tsconfig `paths`/`baseUrl` aliases are not resolved
   (documented). `.d.ts` files are not scanned.
3. **JS/TS classification**: `node:` prefix and the embedded builtin
   table (generated from `require('module').builtinModules`, Node
   v24.17.0) → stdlib; relative specifiers resolve with extension probing
   (spec, spec+ext, spec/index+ext, directory) → internal; bare
   specifiers naming an in-repo package.json `name` → internal (workspace
   semantics, mirroring go.work); otherwise the NEAREST package.json's
   dependency sections decide external (nested-manifest ownership like
   nested go.mod); `#imports` aliases and absolute paths → unknown.
4. **Python extractor** (lexer-grade per research §4): `import` and
   `from ... import` at any indentation, semicolon statements, backslash
   continuations, parenthesized from-imports; #-comments and
   string/docstring bodies blanked by a triple-quote-aware line scanner.
   Documented false negatives, asserted: `importlib.import_module(name)`
   and `__import__(name)`.
5. **Python classification**: embedded stdlib table (from
   `sys.stdlib_module_names`, CPython 3.11.13) → stdlib by top-level
   module; source roots (repo root, src/, each pyproject.toml dir and its
   src/) resolve dotted modules to files or package directories,
   including PEP 420 namespace dirs → internal; pyproject.toml
   dependencies (PEP 621 [project], optional-dependencies, PEP 735
   dependency-groups, tool.poetry.dependencies) matched via PEP 503
   normalization (typing_extensions ↔ typing-extensions) → external;
   dist/module name mismatches (PyYAML→yaml) classify unknown, never
   silently external — the documented manifest limitation.
6. **Self-hosting caught the refactor**: the M3 dependency edges
   (engine→langs, golang→langs) violated the repo's own consumes
   contracts until rules.yaml legitimized them — recorded here as
   evidence the contracts bind.

### M3 gate results (measured 2026-08-10, WSL2 Ubuntu 24.04, go1.26.4)

- Extractor form matrices and false-negative classes green
  (internal/lang/jsts, internal/lang/python tests); engine fixtures
  ts-external-forbid and py-external-forbid flag the third-party import
  with blame=consumer at the exact line.
- Self-hosting clean: 17 rules over 12 modules, 65 files in 9ms.
- Cold start median ~10ms; 5,000-file check median 92.4ms.
- Size: 20.88 MB amd64 / 19.27 MB arm64 static builds.

## M2 (2026-08-10)

1. **Extension discovery is top-level only**: every `*.ts`/`*.js` directly
   under `.arclint/extensions/` is one extension entry, deduplicated by
   real path; shared helpers live in subdirectories and are bundled via
   relative imports. `*.d.ts` and dotfiles are skipped.
2. **Bundling**: esbuild Build (not Transform) with Bundle, CommonJS
   output, ES2017 target; `"arclint"` resolves to the embedded SDK source;
   bare npm specifiers are rejected with a designed error. TypeScript
   import elision applies (an import used only as a type vanishes before
   resolution).
3. **Rule instances** live in a top-level `rules:` list ({type, id?,
   severity?, params?}); the extension's defineRule declares the clause
   (`contract`, default invariant) and `blame` (default provider), keeping
   YAML instances pure data.
4. **Params schema**: the SDK's zod-style `s` builder produces plain JSON
   Schema inside defineRule at registration; the host compiles it
   (santhosh-tekuri) and validates YAML params BEFORE any extension code
   runs. Top-level `default:` values are host-applied (JSON Schema
   validators do not apply defaults).
5. **Two-phase lifecycle**: a frozen `__arclint` global exists only to
   turn runtime calls during the registration phase into a designed
   error; the functional runtime surface is exactly the ctx argument of
   check(), which does not exist outside the evaluation phase.
6. **Sandbox**: bare sobek runtime (ES built-ins only), host-injected
   read-only ctx (files/read/imports/modules/report), `Date.now` fixed and
   `Math.random` seeded host-side, interrupt-based 5s timeouts on both
   registration and each check() invocation. `new Date()` still reads the
   wall clock — the override covers exactly the documented determinism
   gap (Date.now/Math.random), recorded here as the residual.
7. **A crashing or timed-out rule** becomes an error-severity violation
   anchored at the extension file (CI fails visibly); an unknown rule
   type or bad params is a config error (exit 2).
8. **Transpile cache**: bundles cached under `.arclint/cache/extensions/`
   keyed by a recursive content hash of the extensions directory plus the
   build info (so upgrading arclint or esbuild invalidates).
9. **arclint.d.ts** is assembled from tygo-generated declarations of the
   Go host wire types (internal/ext/types.go) plus the hand-written SDK
   API surface; `arclint sdk init` writes it with a tsconfig.json wired
   via `paths`.

### M2 gate results (measured 2026-08-10, WSL2 Ubuntu 24.04, go1.26.4)

- Acceptance: the handler-naming TS rule loads, validates params against
  its schema, and reports a violation through `arclint check` running with
  a completely empty process environment (no PATH — nothing external can
  even be resolved), exit 1 with the stable JSON shape
  (cmd/arclint e2e test).
- Size: linux/amd64 20.65 MB, linux/arm64 19.07 MB stripped static
  builds; delta over the M1 binary (8.07 MB) is +12.59 MB, inside the
  ~15 MB esbuild+sobek budget from the research measurements.
- Cold start median 9.73ms over 11 runs; 5,000-file check median 96.4ms
  (both bounds hold with the embedded runtime).

### M1 gate results (measured 2026-08-10, WSL2 Ubuntu 24.04, go1.26.4)

- Gate 1: six fixture repos, tests written and failing before the engine
  existed; all green after.
- Gate 2: `make oracle` — 466 packages covered, 7,591 imports
  classification-asserted across the five repos, zero mismatches, zero
  crashes; divergences asserted: 35 build-constrained files, 1
  imported-testdata package.
- Gate 3: `make selfcheck` — 16 rules over 11 modules, clean, 45 files in
  5ms.
- Gate 4: `make bench` — cold start median 7.99ms over 11 runs (min 7.71,
  max 8.87; bound 100ms); 5,000-file synthetic repo median 79.6ms over 3
  runs (min 75.7, max 81.9; bound low single-digit seconds).
