# arclint Rule Format Design

Status: draft for orchestrator review
Schema: [`schema/arclint-rules.schema.json`](../../schema/arclint-rules.schema.json) (JSON Schema draft 2020-12)

## 1. Goals and constraints

arclint is a very fast architecture linter written in Go. Rules live in exactly one place: `.arclint/rules.yaml`, a flat rule registry at the repo root. The format is fully declarative — there is no logic, no expressions, and no embedded code in the config. Everything is statically parseable so that a future VS Code extension (or any other tool) can consume the file with nothing more than a YAML parser and the published JSON Schema.

Implementation stack (locked): Go, `goccy/go-yaml` for parsing, `santhosh-tekuri/jsonschema/v6` for validating the parsed document against the schema, and `bmatcuk/doublestar` for glob matching.

Every rule has a stable, unique, user-chosen, kebab-case id (for example `no-utils-dir`). The id is the key in the rule registry, which makes uniqueness structural: a YAML mapping cannot carry two identical keys, and `goccy/go-yaml` is configured to reject duplicates. Ids are what baselines, `ignore` entries, and editor integrations reference, so they must never be renamed casually.

Violations are emitted as JSON objects with this exact shape:

```json
{
  "ruleId": "no-utils-dir",
  "category": "structure",
  "severity": "error",
  "path": "internal/utils/strings.go",
  "line": 1,
  "message": "File matches forbidden pattern **/utils/**",
  "fixHint": "Move helpers next to the code that uses them."
}
```

`category` is the rule's `type`. `line` is omitted when the violation is not tied to a line (structure and naming violations usually are not). `fixHint` is copied from the rule's optional `fixHint` field and is an empty string when the rule does not define one.

## 2. Category taxonomy

Two candidate taxonomies were on the table. The spec example proposed seven categories (structure, naming, dependencies, boundaries, files-required, files-forbidden, custom); the research input proposed four (structure, naming, boundaries, content). The reconciled set is five:

| Category | Absorbs | Rationale |
|---|---|---|
| `structure` | files-required, files-forbidden | Required and forbidden files are two sides of one concern — what the tree must and must not contain. One type with `require` and `forbid` params keeps the registry small without losing precision. |
| `naming` | — | ls-lint style conventions per glob. Distinct engine (basename matching), distinct category. |
| `dependencies` | boundaries | "Dependencies" and "boundaries" describe the same import-graph checks at different altitudes. Import-linter proves one category with several contract kinds is enough; splitting them would force users to guess which of two nearly identical categories to use. |
| `content` | — | Regex must/must-not-appear checks on file content. Cheap to implement, covers a large class of architectural conventions (no `fmt.Println` outside cmd, every file starts with a license header) that none of the other categories reach. |
| `custom` | — | Escape hatch: external command that returns violations as JSON. Keeps the core format closed while still being extensible. |

The category is a closed enum. Each category maps to exactly one rule `type`, and each type owns its own `params` block shape. Adding a future category means adding one enum value and one params definition to the schema — existing rules and consumers are unaffected.

## 3. File-level structure

```yaml
version: 1                     # required, const 1 for now
extends:                       # optional, applied in order before local rules
  - "arclint:recommended"      # embedded preset name — local file paths are rejected in v1
baseline: .arclint/baseline.json   # optional, grandfathered violations
exclude:                       # optional global excludes, on top of built-ins
  - "**/testdata/**"
ignore:                        # optional per-path suppressions
  - path: "legacy/**"
    rules: [no-utils-dir]      # omit `rules` to silence everything under the path
rules:                         # required, flat registry keyed by rule id
  <rule-id>: { ... }
```

Semantics:

- **version** — format version, currently the constant `1`. Bumped only on breaking changes to the format.
- **extends** — embedded preset names, merged in listed order, local file last. v1 resolves only presets compiled into the binary; the single shipped preset is `arclint:recommended` (contains `no-utils-dir` at `error` and `readme-required` at `warn`). A local file path as an extends target is rejected with a config error. Merge is per rule id and whole-rule: a local rule with the same id fully replaces the inherited rule (no deep merge of params). Setting a local rule's severity to `off` is the idiomatic way to disable an inherited rule.
- **baseline** — path to a machine-written file of existing violations (Deptrac-style). Violations that match a baseline entry are suppressed entirely: they are omitted from all output (there is no `baselined` marker) and do not affect the exit code. Entries match on `{ruleId, path, messageHash}` where `messageHash` is the first 16 hex characters of the SHA-256 of the violation message. There is no `--update-baseline` flag in v1. The baseline format itself is out of scope for this document (see section 9 for the decided matching key).
- **exclude** — global glob excludes applied before any rule runs. `node_modules/**` and `.git/**` are always excluded and need not be listed.
- **ignore** — per-path suppression. Each entry is a glob plus an optional list of rule ids; omitting the list silences all rules for matching paths. This is the coarse-grained counterpart to the baseline: `ignore` says "never check this", the baseline says "these specific existing violations are grandfathered".
- **rules** — mapping from kebab-case rule id to rule object. Flat: no groups, no nesting.

## 4. Rule shape (common fields)

```yaml
rules:
  some-rule:
    type: structure            # required — one of the five categories
    severity: error            # required — error | warn | off
    description: One sentence of architectural intent.   # required
    files:                     # optional targeting, defaults to whole tree
      include: ["internal/**"]
      exclude: ["**/*_test.go"]
    fixHint: What to do about it.   # optional, copied into violations
    params: { ... }            # required, shape depends on type
```

`severity: off` keeps the rule in the registry (id remains reserved and documented) without executing it. Severity `warn` never affects the exit code: warn violations are printed in full but only `error` violations produce exit 1 — a run with only warn findings exits 0. `files.include` defaults to everything; `files.exclude` is subtracted from the include set; global `exclude` and built-ins are always subtracted first.

YAML 1.1 note: a bare `off` is parsed as the boolean `false` by YAML 1.1 parsers, so write it quoted (`severity: "off"`). To be forgiving, arclint's loader normalizes a boolean `false` in the severity position to the string `"off"` before schema validation; the quoted form is still the documented spelling, and the schema (which sees post-normalization data) only accepts the three strings.

## 5. Rule shapes per category

### 5.1 structure

`require` globs must each match at least one existing file. `forbid` globs must match no file. `require` is evaluated against the whole tree (minus global excludes) because "this file must exist" is a repo-level assertion; `forbid` respects the rule's `files` targeting.

```yaml
  require-ci-config:
    type: structure
    severity: error
    description: Repo must carry CI and docs entry points.
    params:
      require:
        - ".github/workflows/*.yml"
        - "README.md"

  no-utils-dir:
    type: structure
    severity: error
    description: No grab-bag utility directories.
    params:
      forbid:
        - "**/utils/**"
        - "**/helpers/**"
    fixHint: Move helpers next to the code that uses them.
```

### 5.2 naming

ls-lint style pipe syntax: alternatives separated by `|`, name valid if any alternative matches. Tokens: `camelCase`, `PascalCase`, `snake_case`, `kebab-case`, `SCREAMING_SNAKE_CASE`, `lowercase`, `regex:<pattern>`. The regex pattern may not contain `|` (use a single `regex:` alternative with a group instead). The check applies to the basename with the extension stripped for files; `target: dir` checks directory basenames instead.

```yaml
  go-file-naming:
    type: naming
    severity: error
    description: Go source files are snake_case.
    files:
      include: ["**/*.go"]
    params:
      style: "snake_case"

  package-dir-naming:
    type: naming
    severity: warn
    description: Package directories are single lowercase words or kebab-case.
    files:
      include: ["internal/**", "pkg/**"]
    params:
      target: dir
      style: "lowercase | kebab-case | regex:v[0-9]+"
```

One rule per glob set. Needing different conventions for different trees means writing two rules with two ids — that is a feature: each convention gets its own id, severity, and baseline entries.

### 5.3 dependencies

Modules are named groups of path globs. Contracts reference module names only. Four contract kinds, taken from import-linter and go-arch-lint because they cover every boundary rule seen in practice while staying declarative:

- `layers` — ordered list, top to bottom; a layer may import layers below it, never above (Tach/import-linter readability).
- `forbidden` — explicit deny edges (`from` modules may not import `to` modules).
- `independence` — the listed modules may not import each other in any direction.
- `mayDependOn` — whitelist per module (go-arch-lint shape); anything not listed is forbidden for that module. An empty list means the module may depend on nothing.

```yaml
  layered-architecture:
    type: dependencies
    severity: error
    description: Enforce the three-layer architecture.
    params:
      modules:
        infra:  ["internal/infra/**"]
        app:    ["internal/app/**"]
        domain: ["internal/domain/**"]
      contract: layers
      layers: [infra, app, domain]   # top may import lower, never the reverse

  features-independent:
    type: dependencies
    severity: error
    description: Feature packages must not import each other.
    params:
      modules:
        billing:  ["internal/feature/billing/**"]
        search:   ["internal/feature/search/**"]
        accounts: ["internal/feature/accounts/**"]
      contract: independence
      among: [billing, search, accounts]
```

A violation points at the importing file and the line of the offending import statement.

### 5.4 content

Line-oriented RE2 regexes over file content. `mustNotContain`: no targeted file may match. `mustContain`: every targeted file must match at least once. Each matcher can override the violation message.

```yaml
  no-println-outside-cmd:
    type: content
    severity: warn
    description: Use the structured logger, not fmt.Println.
    files:
      include: ["internal/**/*.go"]
      exclude: ["**/*_test.go"]
    params:
      mustNotContain:
        - pattern: 'fmt\.Print(ln|f)?\('
          message: Use slog instead of fmt printing.
    fixHint: Replace with slog calls.
```

### 5.5 custom

External command escape hatch. arclint runs the argv from the repo root, writes `{"files": ["a.go", "b.go", ...]}` (the rule's targeted files) to stdin, and expects a JSON array of `{path, line?, message, fixHint?}` objects on stdout. Violations are conveyed entirely by the stdout array — an empty array means clean. Exit code 0 is the only success code: any non-zero command exit is a rule execution error, reported as such (never silently swallowed) and surfaced through arclint's exit-2 config/usage path. arclint injects `ruleId`, `category`, and `severity` from the rule's config; a violation-level `fixHint` overrides the rule-level one.

```yaml
  openapi-lint:
    type: custom
    severity: warn
    description: OpenAPI specs pass the in-house checker.
    files:
      include: ["api/**/*.yaml"]
    params:
      command: ["scripts/check-openapi.sh"]
      timeoutSeconds: 60
```

This is the only place external code enters the picture, and it is quarantined: the config itself remains data, and consumers that cannot execute commands (the VS Code extension) simply skip `custom` rules.

## 6. Full example (`arclint init` default)

This exact file ships as the `arclint init` template and validates against `schema/arclint-rules.schema.json`.

```yaml
# yaml-language-server: $schema=https://arclint.dev/schema/arclint-rules.schema.json
# arclint rule registry — the single canonical location is .arclint/rules.yaml.
# Docs: docs/design/rules.md

version: 1

# Inherit shared rule sets. Local rules with the same id win.
# extends:
#   - "arclint:recommended"

# Grandfather existing violations so new code is held to the rules
# without demanding a big-bang cleanup.
# baseline: .arclint/baseline.json

# Global excludes. node_modules/** and .git/** are always excluded.
exclude:
  - "**/testdata/**"
  - "dist/**"

# Per-path suppressions. Omit `rules` to silence everything under the path.
ignore:
  - path: "legacy/**"
    rules: [no-utils-dir, layered-architecture]

rules:
  # --- structure: what the tree must and must not contain -------------------

  require-ci-config:
    type: structure
    severity: "off"    # ships off so a fresh `arclint init` passes `arclint check` — enable once CI exists
    description: Repo must carry CI configuration and a README.
    params:
      require:
        - ".github/workflows/*.yml"
        - "README.md"
    fixHint: Add the missing file at the expected path.

  no-utils-dir:
    type: structure
    severity: error
    description: No grab-bag utility directories.
    params:
      forbid:
        - "**/utils/**"
        - "**/helpers/**"
    fixHint: Move helpers next to the code that uses them.

  # --- naming: ls-lint style conventions per glob ----------------------------

  go-file-naming:
    type: naming
    severity: error
    description: Go source files are snake_case.
    files:
      include: ["**/*.go"]
    params:
      style: "snake_case"

  package-dir-naming:
    type: naming
    severity: warn
    description: Package directories are lowercase, kebab-case, or version suffixes.
    files:
      include: ["internal/**", "pkg/**"]
    params:
      target: dir
      style: "lowercase | kebab-case | regex:v[0-9]+"

  # --- dependencies: import-graph contracts between named modules ------------

  layered-architecture:
    type: dependencies
    severity: error
    description: Infra may use app, app may use domain, never the reverse.
    params:
      modules:
        infra:  ["internal/infra/**"]
        app:    ["internal/app/**"]
        domain: ["internal/domain/**"]
      contract: layers
      layers: [infra, app, domain]
    fixHint: Depend inward — invert the dependency with an interface in the lower layer.

  domain-stays-pure:
    type: dependencies
    severity: error
    description: The domain layer may not import infrastructure concerns.
    params:
      modules:
        domain: ["internal/domain/**"]
        infra:  ["internal/infra/**"]
        cli:    ["cmd/**"]
      contract: forbidden
      forbidden:
        - from: [domain]
          to: [infra, cli]

  # --- content: regex checks on file content ---------------------------------

  no-println-outside-cmd:
    type: content
    severity: warn
    description: Use the structured logger, not fmt printing.
    files:
      include: ["internal/**/*.go"]
      exclude: ["**/*_test.go"]
    params:
      mustNotContain:
        - pattern: 'fmt\.Print(ln|f)?\('
          message: Use slog instead of fmt printing.
    fixHint: Replace with slog calls.

  # --- custom: external command escape hatch ----------------------------------

  openapi-lint:
    type: custom
    severity: "off"      # quoted — see the YAML note in section 4. Enable once the script exists.
    description: OpenAPI specs pass the in-house checker.
    files:
      include: ["api/**/*.yaml"]
    params:
      command: ["scripts/check-openapi.sh"]
      timeoutSeconds: 60
```

## 7. Validation pipeline

1. `goccy/go-yaml` parses `.arclint/rules.yaml` into an `any` tree (duplicate-key detection on).
2. The tree is validated against the embedded `schema/arclint-rules.schema.json` via `santhosh-tekuri/jsonschema/v6` (draft 2020-12, format assertions on). Schema errors are reported with YAML line numbers by mapping the JSON pointer of each error back through goccy's AST.
3. Only after schema validation does arclint decode into typed Go structs. Semantic checks that JSON Schema cannot express run here: contract module references must exist in `modules`, `ignore[].rules` and baseline entries must reference known rule ids, regexes must compile as RE2.

The same schema file is published so editors get completion and validation for free via the `yaml-language-server` header comment.

## 8. Prior art borrowed

- **dependency-cruiser** — publishing a real JSON Schema for the rule file, and per-rule severity naming.
- **ls-lint** — the pipe syntax for naming conventions.
- **import-linter / Tach** — contract kinds (`layers`, `forbidden`, `independence`) and the readable ordered-layer list.
- **go-arch-lint** — the `mayDependOn` whitelist shape.
- **Deptrac** — the baseline concept for grandfathering existing violations.

## 9. Resolved questions (orchestrator decisions)

1. **Import resolution for `dependencies` rules — decided: language-agnostic regex extractors.** Import statements are extracted via regex extractors keyed by file extension (go, js/ts, py initially). Approximate by design — this keeps arclint polyglot and avoids per-language AST engines; precision-critical single-language analysis remains the territory of tools like dependency-cruiser.
2. **Baseline file format and matching key — decided: accepted as proposed.** JSON array of `{ruleId, path, messageHash}` where `messageHash` is the SHA-256 of the message truncated to 16 hex characters; line numbers excluded because they drift. Path-keyed: a moved file invalidates its baseline entries.
3. **`extends` preset distribution — decided: binary-embedded presets only in v1.** `arclint:recommended` style presets are compiled into the binary; no registry or URL fetch. Local paths are rejected in v1 — an extends entry must name an embedded preset.
