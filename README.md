# arclint

arclint is a very fast, language-agnostic architecture linter and template-repository creator. It enforces required and forbidden files, naming conventions, dependency boundaries, and content rules from a single declarative YAML file, and it scaffolds new services, packages, and pages from drop-in templates whose saved answers double as drift-lint input. It is a single static Go binary with a cold start under 50ms, so it runs invisibly on every save, pre-commit hook, and CI job. Rules are pure data — YAML validated against a published JSON Schema — with stable, user-chosen rule IDs, which makes the whole rule surface consumable by editors, CI pipelines, and AI agents without executing any code.

## Install

```
go install github.com/wixregiga/arclint/cmd/arclint@latest
```

Or build from source:

```
git clone https://github.com/wixregiga/arclint
cd arclint
go build ./cmd/arclint
```

## Quickstart

### arclint init

Run `arclint init` in your repo root to create the `.arclint/` configuration layout:

```
$ arclint init
created .arclint/
  rules.yaml              # default ruleset, heavily commented
  answers/                # sharded saved answers, one file per generated unit
  templates/component/    # example template
  templates/repo/         # example template
  templates/service/      # example template

try: arclint new service my-service
try: arclint check
```

Pass `--bare` to skip the example templates, or `--force` to overwrite an existing `.arclint/` directory. Without `--force`, init refuses to touch an existing setup.

### arclint check

`arclint check` lints the architecture: it runs every rule in `rules.yaml` against the tree, grouped by category with the rule ID always visible and a fix hint on its own line:

```
$ arclint check
structure (2)
  services-need-dockerfile  services/users-api  missing required file: Dockerfile
          fix: add a Dockerfile, or run `arclint make services/users-api --apply`
  no-utils-dir              pkg/util            package name "util" is on the forbidden-names list
          fix: rename to something domain-specific; see the no-utils-dir rule in rules.yaml

dependencies (1)
  no-cross-service-internals  services/billing/main.go:14  imports internal package of another service: services/users-api/internal/db
          fix: depend on the service API, not its internals

3 violations (2 structure, 1 dependencies) in 412 files, 38ms
```

`--format json` emits a stable machine-readable report on stdout, so `arclint check --format json | jq` always works:

```json
{
  "violations": [
    {
      "ruleId": "services-need-dockerfile",
      "category": "structure",
      "severity": "error",
      "path": "services/users-api",
      "message": "missing required file: Dockerfile",
      "fixHint": "add a Dockerfile, or run `arclint make services/users-api --apply`"
    }
  ],
  "summary": { "total": 1, "filesScanned": 412, "durationMs": 38 }
}
```

Narrow a run with `--rules <ids>` or `--skip <ids>` (comma-separated rule IDs), and tune parallelism with `--jobs <n>`. Exit codes are uniform: 0 for a clean run (or one with only warn-severity findings), 1 when any error-severity violation is found, 2 for a config or usage error. The 1/2 split lets CI distinguish "the repo has problems" from "the invocation is broken". `check` never prompts and is CI-safe by construction.

### arclint new

`arclint new <thing> [name]` generates a unit from a template. A thing exists because a directory exists under `.arclint/templates/` — no registration:

```
$ arclint new service payment-gateway
```

Any variable the template declares can be supplied with the repeatable `--var name=value` flag. If a required variable (one with no manifest default) is still unresolved after flags and saved answers, and you are on a TTY, arclint prompts for it interactively — then prints the equivalent non-interactive command so you graduate to scripting naturally:

```
tip: next time, skip the prompts with:
  arclint new service payment-gateway --var transport=http --var with_db=true
```

In CI, pass `--no-input`: prompting is disabled entirely and any missing required input exits 2 with the exact flag to pass. Use `--dry-run` to see the file list and diffs without writing, `--out <dir>` to override the manifest's `destination:`, and `--list` to see all available things with their descriptions. If the destination already exists, `new` hard-refuses and points you at `arclint make` — regenerating an existing unit is make's job, and `new` has no `--force`.

Troubleshooting: if the prompt TUI misrenders (screen readers, `TERM=dumb`, or a pty-piped stdin), set `ARCLINT_ACCESSIBLE=1` (or run under `TERM=dumb`) to fall back to plain line-based prompting.

Every successful generation records the resolved variables in `.arclint/answers/<unit-path-slug>.yaml`, which is what powers drift detection.

### arclint make

`arclint make [paths...]` re-renders generated units from their saved answers and reports drift. The default is a dry-run — nothing is written:

```
$ arclint make
drift: services/users-api (template: service, 3 files)
  modified  services/users-api/main.go
  modified  services/users-api/Dockerfile
  created   services/users-api/healthz.go
drift: services/billing (template: service, 1 file)
  modified  services/billing/Dockerfile

2 units drifted, 4 files affected. run with --apply to write.
```

Pass `--diff` for unified diffs per drifted file, `--apply` to write the changes, and `--var name=value` to override a saved answer (persisted back only with `--apply`). `--apply` rewrites drifted files to match the template — including files you edited (the dry-run diff shown by default is your review step). A conflict — you edited a file AND the template changed — is skipped and reported; only `--apply --take-template` overwrites those. When your edits are overwritten, `--apply` says so by name: `restoring services/users-api/main.go (your edits replaced by template)`, so you don't lose that context in a wall of `wrote` lines. The dry-run exits 0 even when drift is found — it is a pure report; add `--fail-on-drift` to exit 1 on drift, which is how make slots into CI as a scaffolding-drift gate. `--format json` emits a stable per-unit drift report.

## Rules: one file, five categories

All rules live in `.arclint/rules.yaml`, a flat registry keyed by stable kebab-case rule IDs. The format is fully declarative — no expressions, no embedded code — and validates against the published JSON Schema, so editors get completion and inline validation for free. A minimal file with one rule per category:

```yaml
version: 1

rules:
  # structure: what the tree must and must not contain
  no-utils-dir:
    type: structure
    severity: error
    description: No grab-bag utility directories.
    params:
      forbid: ["**/utils/**", "**/helpers/**"]
    fixHint: Move helpers next to the code that uses them.

  # naming: ls-lint style conventions per glob
  go-file-naming:
    type: naming
    severity: error
    description: Go source files are snake_case.
    files:
      include: ["**/*.go"]
    params:
      style: "snake_case"

  # dependencies: import-graph contracts between named modules
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

  # content: regex must/must-not-appear checks on file content
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

  # custom: external command escape hatch, JSON in and out
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

The five categories are a closed enum. `structure` asserts which files must and must not exist. `naming` applies ls-lint style basename conventions per glob. `dependencies` enforces import-graph contracts (layers, forbidden edges, independence, mayDependOn whitelists) between named module groups. `content` runs line-oriented regex checks over file content. `custom` runs an external command that returns violations as JSON — the only place external code enters, and consumers that cannot execute commands simply skip it.

Severity is `error`, `warn`, or `"off"` (quoted — YAML parses a bare `off` as a boolean). Only `error` violations produce exit 1; `warn` findings are printed in full but a run with only warns exits 0, so you can introduce a rule as a warning without breaking CI. For existing repos, a baseline file grandfathers current violations so new code is held to the rules without a big-bang cleanup. Full details in [docs/design/rules.md](docs/design/rules.md).

## Templates: folder-drop scaffolding

A template is a directory under `.arclint/templates/` containing a `template.yaml` manifest and a `files/` render root. The manifest is pure data — copier-style variable declarations, no code:

```yaml
version: 1
description: "A backend service with handler, config, and test scaffolding"
destination: "services/{{ name | kebab }}"

variables:
  - name: name
    description: "Service name (human words, e.g. 'payment gateway')"
    type: string
    validate: "^[a-zA-Z][a-zA-Z0-9 _-]*$"

  - name: transport
    description: "How does this service listen?"
    type: choice
    choices: [http, grpc, worker]
    default: http

  - name: with_db
    description: "Does this service own a database?"
    type: bool
    default: false

  - name: db_name
    description: "Database name"
    type: string
    default: "{{ name | snake }}"
    when: "with_db == true"
```

Interpolation is mustache-style `{{ var }}` with optional filters chained left to right, applied to both file content and path names (`files/cmd/{{ name | kebab }}/main.go`). The filter set is fixed: `pascal`, `camel`, `snake`, `kebab`, `upper`, `lower`, `plural`. There are deliberately no loops and no conditionals in template content — anything conditional belongs in the manifest's `when` field or in a separate thing type. Built-in variables `repo_name`, `year`, and `arclint_version` are always available.

Adding a custom thing type takes three steps and no registration:

1. Make the directory: `mkdir -p .arclint/templates/docs-page/files`
2. Write `.arclint/templates/docs-page/template.yaml` declaring `destination` and `variables`.
3. Put template files under `files/` — then run `arclint new docs-page --var title="Getting Started"`.

The thing type exists the moment the directory does, and every generated unit immediately participates in `arclint make` drift checking. Full details in [docs/design/templating.md](docs/design/templating.md).

## Writing rules with Claude Code

The `arclint-rule-creator` skill lets you describe an architectural constraint in plain language inside Claude Code — "no service may import another service's internals", "every docs page needs front matter" — and get back a valid, correctly categorized rule that validates against the schema, ready to paste into `rules.yaml`.

## Further reading

- [docs/discovery.md](docs/discovery.md) — why arclint exists: the landscape of architecture linters and scaffolders, the gap none of them fill, and the build-vs-buy analysis.
- [docs/design/cli.md](docs/design/cli.md) — the complete CLI surface: every command, flag, exit code, and the prompting model.
- [docs/design/rules.md](docs/design/rules.md) — the rule format, category taxonomy, and validation pipeline.
- [docs/design/templating.md](docs/design/templating.md) — the template engine, manifest schema, and regeneration semantics.
