# arclint Discovery — Gate 1

Status: Gate 1 candidate. Date: 2026-07-03.

arclint is a language-agnostic project-structure linter and scaffolder: it enforces required/forbidden files, naming conventions, and dependency boundaries from declarative YAML, emits machine-readable violations with stable rule IDs, and scaffolds new code from drop-in templates whose saved answers double as drift-lint input.

This document records the discovery research: what already exists, what those tools do better, why building still wins, and the locked technology stack.

---

## 1. Landscape: Architecture and Structure Linters

| Tool | Language scope | Speed profile | What it checks | Config model | Notable ideas worth stealing | Source |
|---|---|---|---|---|---|---|
| [ArchUnit](https://www.archunit.org/) | Java/JVM only | Slow (JVM startup + classpath analysis) | Package/class dependency rules, naming, layering | Rules as Java code (tests) | Fluent rules-as-code expressiveness | [github.com/TNG/ArchUnit](https://github.com/TNG/ArchUnit) |
| [dependency-cruiser](https://github.com/sverweij/dependency-cruiser) | JavaScript/TypeScript only | Node startup ~100ms+, slower on large graphs | Module dependency rules (from/to matchers) | JS/JSON config with published JSON Schema | Rule schema `{name, severity, comment, from/to matchers}`; multi-format output (json, err, mermaid) | [github.com/sverweij/dependency-cruiser](https://github.com/sverweij/dependency-cruiser) |
| [eslint-plugin-boundaries](https://github.com/javierbrea/eslint-plugin-boundaries) | JavaScript/TypeScript only | Bound to ESLint run cost | Element-type boundaries between folders | ESLint config | Element-type definitions + capture-group matchers | [github.com/javierbrea/eslint-plugin-boundaries](https://github.com/javierbrea/eslint-plugin-boundaries) |
| [ls-lint](https://ls-lint.org/) | Language-agnostic (filenames) | Milliseconds on thousands of files (Go) | File/directory naming only | Single YAML | Closest speed peer; proves Go can do this fast. No rule IDs, no structure/boundary rules | [github.com/loeffel-io/ls-lint](https://github.com/loeffel-io/ls-lint) |
| [Nx conformance](https://nx.dev/nx-enterprise/powerpack/conformance) | TS-centric monorepos | Node/Nx startup cost | Custom workspace conformance rules | TypeScript rule authoring; paid (Powerpack), Nx lock-in | Check-vs-fix split in rule lifecycle | [nx.dev](https://nx.dev/nx-enterprise/powerpack/conformance) |
| [Sonar architecture](https://www.sonarsource.com/) | Multi-language | Minutes (server-side analysis) | Architecture constraints as part of quality gate | Server product configuration | Validates market demand for architecture-as-quality-gate | [sonarsource.com](https://www.sonarsource.com/) |
| [Tach](https://github.com/gauge-sh/tach) | Python only | Fast (Rust core) | Module boundaries and interfaces | TOML | Rust core for speed on an interpreted-language ecosystem | [github.com/gauge-sh/tach](https://github.com/gauge-sh/tach) |
| [import-linter](https://github.com/seddonym/import-linter) | Python only | Slow (imports the codebase) | Import contracts | INI/TOML contracts | Contract taxonomy: forbidden / independence / layers | [github.com/seddonym/import-linter](https://github.com/seddonym/import-linter) |
| [arch-go](https://github.com/arch-go/arch-go) | Go only | Fast | Package dependency, naming, content rules | YAML | Compliance threshold percentage + `describe` command | [github.com/arch-go/arch-go](https://github.com/arch-go/arch-go) |
| [go-arch-lint](https://github.com/fe3dback/go-arch-lint) | Go only | Fast | Component dependency rules | YAML | Clean `mayDependOn` YAML shape | [github.com/fe3dback/go-arch-lint](https://github.com/fe3dback/go-arch-lint) |
| [Deptrac](https://github.com/qossmic/deptrac) | PHP only | Moderate | Layer dependency rules | YAML | Baseline file for gradual adoption in legacy codebases | [github.com/qossmic/deptrac](https://github.com/qossmic/deptrac) |

### The gap

Every tool above misses at least one of arclint's four pillars. Nothing on the market combines:

1. **Language-agnostic structure linting** — required/forbidden files, naming conventions, and directory boundaries that work identically in a Go repo, a Python repo, or a polyglot monorepo. Every boundary tool above is single-ecosystem; ls-lint is language-agnostic but naming-only.
2. **Sub-50ms cold start** — ls-lint is the only peer in this class; everything else pays JVM, Node, or interpreter startup.
3. **Machine-readable rules with stable IDs** — dependency-cruiser comes closest with its JSON Schema, but no tool emits violations keyed to stable rule identifiers suitable for suppression, baselining, and AI-agent consumption.
4. **Scaffolding integrated with linting** — no linter scaffolds, and no scaffolder lints. arclint's `new` command generates from templates, and the saved answers become drift-lint input for the `make` regeneration command.

## 2. Landscape: Templating and Scaffolding Tools

| Tool | Discovery model | Variable model | Regeneration | Weakness for arclint's use case | Source |
|---|---|---|---|---|---|
| [cookiecutter](https://github.com/cookiecutter/cookiecutter) | Whole-project template repos | JSON prompts | None | Whole-project only; no validation; no regen | [github.com/cookiecutter/cookiecutter](https://github.com/cookiecutter/cookiecutter) |
| [copier](https://github.com/copier-org/copier) | Template repos | Best-in-class declarative: type/help/default/choices/validator/when | Answers file + `copier update` | Jinja-heavy; project-level, not per-thing scaffolds | [copier.readthedocs.io](https://copier.readthedocs.io/) |
| [yeoman](https://yeoman.io/) | Generators published as npm packages | JS prompt code | None | Opposite of drop-in; heavy authoring ceremony | [yeoman.io](https://yeoman.io/) |
| [degit](https://github.com/Rich-Harris/degit) | Bare repo clone | None | None | Fast but no variables at all | [github.com/Rich-Harris/degit](https://github.com/Rich-Harris/degit) |
| [plop](https://plopjs.com/) | Local plopfile | Manifest is JS code | None | Config-as-code, not data | [plopjs.com](https://plopjs.com/) |
| [scaffdog](https://github.com/scaffdog/scaffdog) | Markdown file per template | Front-matter prompts | None | Drop-in-ish but no regeneration story | [github.com/scaffdog/scaffdog](https://github.com/scaffdog/scaffdog) |
| [hygen](https://www.hygen.io/) | Folder-drop: `_templates/<generator>/` = command, zero registration | `prompt.js` (code, not data) | None | Prompt definition is code; Node dependency | [github.com/jondot/hygen](https://github.com/jondot/hygen) |
| [cargo-generate](https://github.com/cargo-generate/cargo-generate) | Single-template repos | Declarative placeholders with regex validation | None | One template per repo; Rust-ecosystem framing | [github.com/cargo-generate/cargo-generate](https://github.com/cargo-generate/cargo-generate) |

### Ideas arclint adopts

- **hygen's folder-drop discovery**: dropping a directory under `.arclint/templates/<thing>/` makes `arclint new <thing>` exist. Zero registration.
- **copier's question schema**, but as pure data in `template.yaml` (type, help, default, choices, validator, when) instead of Jinja/code.
- **copier's answers-file regeneration**, repurposed: saved answers let `arclint make` re-render and diff, so scaffolds double as drift lint. No existing tool connects regeneration to linting — this is arclint's differentiator.
- **Mustache-style interpolation** `{{ var | filter }}` with case helpers (camel, snake, kebab, pascal) and left-to-right filter chaining (`{{ x | snake | upper }}`), avoiding a full template language.
- **dependency-cruiser's rule shape and published JSON Schema** for the rules file.
- **Deptrac's baseline file** and **arch-go's compliance threshold** for gradual adoption.
- **import-linter's contract taxonomy** (forbidden / independence / layers) as the vocabulary for boundary rules.
- **Nx's check-vs-fix split** in the rule lifecycle.

## 3. Build vs Buy

### What existing tools do better — honestly

- **dependency-cruiser** has years of hardening on JS module-graph resolution (tsconfig paths, webpack aliases, dynamic imports). arclint will not match its depth of *import-level* analysis in JavaScript, and should not try.
- **ArchUnit** offers expressiveness that declarative YAML cannot reach: arbitrary predicates over the type system. Teams needing "no class annotated X may call methods returning Y" are better served there.
- **copier** has a mature update/merge story including conflict handling on regeneration. arclint's first regeneration pass will be diff-and-report, not three-way merge.
- **Sonar** provides dashboards, history, and organizational governance that a CLI tool does not.
- **ls-lint** is already the fastest naming linter and needs nothing from us if naming is the only requirement.

### Why buying (adopting) still loses

Adopting means stitching three or four tools per language: ls-lint for naming, dependency-cruiser or Tach or import-linter for boundaries (one per ecosystem), a scaffolder, plus glue scripts to unify output. That stack:

- has no shared rule IDs, so suppression and baselining are inconsistent per tool;
- has no unified machine-readable output, which matters increasingly because AI agents are first-class consumers of lint output;
- pays multiple runtime startups (Node + Python) where arclint pays one 5-10ms binary start;
- cannot express arclint's core rules at all — no existing tool checks "every directory matching `internal/*/` must contain a `doc.go`" or "no `testdata/` outside `*_test` packages" language-agnostically;
- has zero integration between scaffolding and linting, which is the product thesis.

The gap is structural, not incremental. No vendor is positioned to close it: linter vendors are single-ecosystem by architecture, and scaffolder vendors have no lint engine. Building is justified.

## 4. Recommendation: Build, with a Fable 5 Orchestrator Development Model

Build arclint. Develop it with an orchestrator-and-subagents model on Fable 5: the orchestrator decomposes the roadmap into bounded work packages, delegates implementation to parallel subagents, and judges results against acceptance criteria before merging.

Honest justification:

- **The problem decomposes cleanly.** Walker, rule engine, config loader, template renderer, output formatters, and CLI wiring have narrow interfaces. That is exactly the shape where parallel subagent implementation works well and merge conflicts stay rare.
- **Go amplifies the model.** Fast compile, `go vet`, `gofmt`, and table-driven tests give subagents a tight, objective verification loop; the orchestrator can judge on compile-plus-test evidence rather than prose claims.
- **The honest risks**: orchestration adds coordination overhead on small tasks, subagents can drift from conventions without a locked spec, and judging quality is only as good as the acceptance criteria. Mitigation: this document plus the locked conventions in section 6 act as the shared contract; the orchestrator enforces rule-schema and violation-shape stability as non-negotiable interfaces; single-file tasks skip delegation.
- **Alternative rejected**: a single-agent serial build would produce the same code with less coordination cost but roughly 3-4x the wall-clock time on the parallelizable middle phase (rules engine + templates + formatters). Since the interfaces are locked before implementation starts, parallelism is cheap here.

## 5. Locked Stack Decision

| Concern | Choice | Version/pin |
|---|---|---|
| Language | Go | 1.26.4 |
| CLI framework | [spf13/cobra](https://github.com/spf13/cobra) | latest stable |
| TUI prompts | [charmbracelet/huh](https://github.com/charmbracelet/huh) | latest stable |
| Glob matching | [bmatcuk/doublestar/v4](https://github.com/bmatcuk/doublestar) | v4 |
| YAML parsing | [goccy/go-yaml](https://github.com/goccy/go-yaml) | latest stable |
| Schema validation | [santhosh-tekuri/jsonschema/v6](https://github.com/santhosh-tekuri/jsonschema) | v6 |
| Filesystem walk | parallel `filepath.WalkDir` / [charlievieth/fastwalk](https://github.com/charlievieth/fastwalk) | latest stable |

### Rationale

- **Cold start**: a static Go binary starts in 5-10ms. This is the difference between a linter that runs on every save/pre-commit invisibly and one developers disable. Node (100ms+) and JVM (seconds) competitors lose here; ls-lint proves the Go number is real.
- **Walk throughput**: parallel `WalkDir`/fastwalk sweeps thousands of files in well under a second, keeping full-repo lint interactive even on large monorepos.
- **AI-agent iteration loop**: Go's compile speed, single toolchain, and deterministic formatting give the fastest feedback cycle for the orchestrator/subagent development model — subagents get compiler-verified truth in seconds.
- **Distribution**: one static binary per platform, no runtime dependency. Drop into any CI image or `go install` it.
- **cobra** is the de facto Go CLI standard (kubectl, gh, hugo) with completions and nested commands for `lint` / `make` / `describe`.
- **huh** provides declarative form prompts that map one-to-one onto the copier-style question schema in `template.yaml`.
- **doublestar** supplies `**` globs matching the semantics users know from gitignore and editors, which stdlib `path.Match` lacks.
- **goccy/go-yaml** offers precise error positions (needed for pointing at the offending line in `rules.yaml`) and is actively maintained.
- **santhosh-tekuri/jsonschema/v6** validates `rules.yaml` and `template.yaml` against published schemas, mirroring the dependency-cruiser steal.

### Runner-up: Rust

Rust would match or beat Go on cold start and walk speed, and its ecosystem (ignore/globset crates, tree-sitter) is stronger for source parsing. It loses today on iteration-loop speed (compile times slow the agent feedback cycle) and on team/agent familiarity with the chosen libraries. Rust wins only if AST-heavy parsing becomes core to arclint — that is, if boundary rules move from path/glob-level matching to real import-graph extraction across many languages via tree-sitter. That is explicitly out of scope for v1; if it becomes core, revisit this decision then.

## 6. Locked arclint Conventions (recorded for downstream gates)

- **Config root**: `.arclint/`
- **Rules file**: `.arclint/rules.yaml`
- **Templates**: `.arclint/templates/<thing>/template.yaml` (folder-drop discovery; directory name is the `new` subcommand)
- **Saved answers**: sharded, one file per generated unit at `.arclint/answers/<unit-path-slug>.yaml` — no central answers.yaml
- **Interpolation**: mustache-style `{{ var | filter }}`; filter chaining allowed, applied left to right (`{{ x | snake | upper }}`)
- **Input priority** for template variables: explicit flags > saved answers > manifest defaults; interactive prompt fires only as last resort for required variables (those without a default) that remain unresolved — under `--no-input` that prompt becomes exit 2
- **Violation JSON shape**: `{ruleId, category, severity, path, line?, message, fixHint}` — stable rule IDs, machine-readable, schema-published.

## 7. Decision Log

Orchestrator verdicts binding on all design docs (cli.md, templating.md, rules.md), one line each:

- **D1 — Answers storage sharded.** One file per generated unit at `.arclint/answers/<unit-path-slug>.yaml`; no single central answers.yaml.
- **D2 — Filter chaining allowed.** `{{ x | snake | upper }}` applies filters left to right.
- **D3 — Destination collision across templates: hard refuse.** `arclint new` errors before writing, naming both templates; no last-write-wins.
- **D4 — YAML library is goccy/go-yaml everywhere** (locked stack); gopkg.in/yaml.v3 is not used.
- **D5 — `make --apply` may overwrite rendered files;** the default dry-run diff is the consent step; no protected regions in v1; conflicted files (user-edited and template-changed) require `--apply --take-template`.
- **D6 — Severity `warn` never affects exit code:** `error` → exit 1, `warn` printed only.
- **D7a — Dependency import resolution:** language-agnostic regex extractors keyed by file extension (go, js/ts, py initially), approximate by design.
- **D7b — Baseline matching key:** `{ruleId, path, messageHash}` accepted.
- **D7c — `extends`:** binary-embedded presets only in v1, no URL fetch.
- **D8 — Stack locked:** Go 1.26.4.
- **D9 — Category enum locked:** `structure | naming | dependencies | content | custom`.
- **D10 — Severity `"off"` YAML 1.1 gotcha:** loader coerces a boolean `false` in the severity position to the string `"off"`; quoted form is the documented spelling.
- **D11 — Command split:** `arclint new <thing>` is the only generation/scaffolding command; `arclint make` is regeneration and drift detection only.
- **D12 — Drift exit code gated:** default `arclint make` dry-run exits 0 even when drift is found; exit 1 only with `--fail-on-drift`.
- **D13 — Variable and diff flags:** template variables are passed only via the generic repeatable `--var name=value` flag (no per-variable flags); unified diffs on `arclint make` are shown via the dedicated `--diff` flag (`--verbose` is global logging only).
- **D14 — `make --format json` schema locked:** `{"units": [{unit, template, status: clean|drift|conflict|orphan, files: [{path, status: added|changed|removed|conflict}]}]}`.
- **D15 — Drift has no severity:** the `arclint make` exit code is governed solely by `--fail-on-drift`.
- **D16 — `arclint new` has no `--force`:** an existing destination is a hard refusal pointing the user to `arclint make`; `init` keeps its `--force`.
- **D17 — Required iff no default:** no `required:` manifest field exists; a prompt fires only for a required variable unresolved by flag or saved answer, on a TTY, without `--no-input`; `--no-input` plus an unresolved required variable is exit 2.
- **D18 — Check output categories from the locked enum only:** `structure | naming | dependencies | content | custom` ("layout" removed from examples).
- **D19 — No `module` built-in variable:** built-ins are `repo_name` (git root dir name), `year`, `arclint_version`; go.mod is language-specific and contradicts language-agnostic positioning.
- **D20 — Custom rule command contract:** stdin `{files: [paths]}`, stdout JSON array of `{path, line?, message, fixHint?}`; arclint injects ruleId/category/severity from rule config; any non-zero command exit is a rule execution error (exit-2 path).
- **D21 — Saved answers mutate only on apply:** `new` writes immediately (generation is the apply step); `make` records overrides and newly-prompted variables only with `--apply`; dry-run never mutates `.arclint/answers/`.
- **D22 — Doc authority split:** templating.md is authoritative for manifests (`destination:` field, typed variables `string|bool|choice`, `files/` render root, mustache-only interpolation); rules.md is authoritative for rule format (user-chosen kebab-case rule ids); cli.md reconciled to both.
- **D23 — `extends` embedded presets only:** local file paths as extends targets are rejected in v1; the single shipped preset `arclint:recommended` carries `no-utils-dir` (error) and `readme-required` (warn).
- **D24 — Baseline suppresses fully:** matched violations are omitted from output (no `baselined` flag); matching key is `{ruleId, path, sha256-hex-16(message)}`; `--update-baseline` does not exist in v1.
- **D25 — Structure `require` violation path is the pattern:** an unmatched require glob is reported with the glob pattern itself as `path`, never resolved to per-unit paths.
- **D26 — Deleted template dir is `orphan`, exit 0:** a recorded unit whose template directory is gone is reported as `orphan` and `arclint make` exits 0; a present-but-invalid `template.yaml` remains exit 2.
- **D27 — Answers `files:` map drives drift:** each shard records path → sha256 of the rendered files; template plus answers are the source of truth — a user-edited file with an unchanged template is drift and `--apply` restores it; the `removed` file status is reserved in the schema and never emitted in v1.
- **D28 — `init` ships `require-ci-config` severity off:** a fresh `arclint init` passes `arclint check` out of the box; users enable the rule once CI exists.
