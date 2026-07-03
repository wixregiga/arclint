# arclint CLI Surface Design

This document defines the complete command-line surface of arclint: every command, every flag, the interactive prompting model, output formats, exit codes, and the error message style guide. It is the contract between the CLI layer (cobra + charmbracelet/huh) and the engine underneath. Anything not specified here is an engine concern.

## Design Principles

1. **Fast by default.** Cold start under 50ms. No config loading, no filesystem walking, no template parsing before flag parsing completes. Help text and version output must never touch `.arclint/`.
2. **Every prompt has a flag.** Interactive prompts are a convenience layer over flags, never the only path. Anything the TUI can ask, a flag can answer. After every answered prompt, arclint prints the equivalent flag so users graduate to non-interactive usage naturally.
3. **Scriptable everywhere.** `--no-input` plus flags makes every command fully deterministic for CI. `--format json` makes every check result machine-readable.
4. **Folder-drop extensibility.** New template types require zero registration: drop a folder in `.arclint/templates/` and it exists.
5. **Errors teach.** Every error states what happened and how to fix it, in one line each.

## Input Resolution Order

For any value a command needs, arclint resolves in strict priority order:

1. **Explicit flag** (`--var name=users-api`, `--out`, etc.)
2. **Saved answers** (per-unit files under `.arclint/answers/` from a previous run, where applicable)
3. **Manifest defaults** (`default:` values in `template.yaml`)
4. **Interactive TUI prompt** — last resort, for required variables only, and only when stdin/stdout is a TTY and `--no-input` is not set.

A higher tier always wins outright; lower tiers are only consulted when higher tiers produce nothing. A variable is required iff it has no `default:` in the manifest — there is no separate `required:` field. A prompt therefore fires only when a required variable is still unresolved after flags and saved answers. With `--no-input`, tier 4 is removed entirely: if a required value is still missing after flags and saved answers, arclint exits with code 2 immediately, before doing any work.

## Global Flags

These flags are accepted by every command.

| Flag | Type | Default | Meaning |
|---|---|---|---|
| `--no-input` | bool | false | Never prompt. Missing required input is a hard error (exit 2) with the flag that would have supplied it. |
| `--config <path>` | string | `.arclint/` discovered upward from cwd | Use an explicit config root instead of discovery. |
| `--format <text\|json>` | string | `text` | Output format. `json` implies no color and no progress output. |
| `--quiet` | bool | false | Suppress everything except violations, diffs, and errors. |
| `--verbose` | bool | false | Show per-file progress, timing breakdown, rule evaluation detail. |
| `--help`, `-h` | bool | — | Command help. Never reads config. |
| `--version` | bool | — | Print version. Never reads config. |

`--quiet` and `--verbose` are mutually exclusive; passing both is a usage error (exit 2). `--format json` on commands that produce structured results (`check`, `make`) emits JSON to stdout; human commentary, progress, and prompts (when they occur) go to stderr, so `arclint check --format json | jq` always works.

Config discovery: arclint walks upward from the working directory looking for `.arclint/`, stopping at the git root or filesystem root. `--config` bypasses discovery. Discovery only runs when the command actually needs config — `--help` and `--version` never trigger it.

---

## Command Reference

### arclint init

Create the `.arclint/` configuration layout in the current directory.

```
arclint init [flags]
```

**Flags**

| Flag | Meaning |
|---|---|
| `--force` | Overwrite an existing `.arclint/` directory. Without it, init refuses when `.arclint/` exists. |
| `--bare` | Create `rules.yaml` and empty `templates/` only; skip example templates. |

**Behavior**

1. If `.arclint/` already exists and `--force` is absent, print `error: .arclint/ already exists — run with --force to overwrite, or edit .arclint/rules.yaml directly` and exit 2. Nothing is touched.
2. Otherwise create:

```
.arclint/
  rules.yaml              # default ruleset, heavily commented
  answers/                # sharded saved answers, one file per generated unit (empty at init)
  templates/
    service/              # example template (omitted with --bare)
      template.yaml       # manifest: variables, destination, metadata
      files/              # render root; content and path names are interpolated
        cmd/{{ name | kebab }}/main.go
        internal/handler.go
        service.yaml
    package/              # second example (omitted with --bare)
      template.yaml
      files/
        ...
```

3. Print a short next-steps message: the created paths, then `try: arclint new service my-service` and `try: arclint check`.

`init` never prompts, so `--no-input` is a no-op here. It is the one command that works without an existing `.arclint/`.

**Exit codes:** 0 created; 2 already exists without `--force`, or path not writable.

---

### arclint new

Generate a new unit (service, package, module — any "thing") from a template.

```
arclint new <thing> [name] [flags]
```

**Arguments**

- `<thing>` — template type; must match a directory name under `.arclint/templates/`. Required.
- `[name]` — name of the generated unit. Optional; prompted (or taken from the manifest's `name` variable default) when omitted.

**Flags**

| Flag | Meaning |
|---|---|
| `--var name=value` | Set a template variable. Repeatable. Highest-priority input source. |
| `--out <dir>` | Override the manifest's `destination:`. Without it, the interpolated `destination:` from `template.yaml` is used. |
| `--dry-run` | Render and show the file list plus diffs, write nothing. |
| `--list` | List available things (template directories) with their manifest descriptions, then exit 0. Ignores other args. |

**Behavior**

1. Discover things by listing directories in `.arclint/templates/` that contain a `template.yaml`. No registry, no config entry — the folder is the registration.
2. If `<thing>` is unknown: print the available things and the closest match by edit distance, exit 2:

```
error: unknown template "servce" — available: service, package, cli-tool (did you mean "service"?)
```

3. Load `template.yaml`. For each declared variable, resolve via the input order (flag → saved answer → manifest default → prompt, prompting only for required variables). Positional `[name]` is sugar for `--var name=<value>`.
4. Validate every resolved value against the manifest's validation rules (`validate` regex, `choices` membership). Flag-supplied values that fail validation are exit-2 errors naming the variable, the value, and the constraint. Prompt-supplied values are re-prompted inline (see TUI model).
5. Render templates into the destination. If the destination already exists, hard-refuse: abort with the colliding path listed and an error pointing the user to `arclint make` for regenerating an existing unit, exit 2, having written nothing (the existence check runs before the first write). There is no `--force` on `arclint new` — regeneration is `arclint make`'s job.
6. Record the resolved variable set into the unit's answers file at `.arclint/answers/<unit-path-slug>.yaml`, so `arclint make` can re-render later.
7. Print the created file list and, if any inputs came from prompts, the full equivalent non-interactive command line.

**Exit codes:** 0 generated (or dry-run clean); 2 unknown thing, validation failure, missing required input under `--no-input`, or destination already exists.

---

### arclint make

Re-render templates against saved answers and report or apply drift. This is the "keep generated units in sync with their template" command.

```
arclint make [paths...] [flags]
```

**Arguments**

- `[paths...]` — one folder, several folders, or nothing (repo root). Each path is matched against generated units recorded in `.arclint/answers/`; a parent path selects all units beneath it.

**Flags**

| Flag | Meaning |
|---|---|
| `--apply` | Write the re-rendered output. Default is a dry-run diff summary. |
| `--diff` | Show a unified diff per drifted file instead of the one-line-per-file summary. |
| `--var name=value` | Override a saved answer for this run. Repeatable. Overrides are persisted back to the unit's answers file only with `--apply`. |
| `--fail-on-drift` | Exit 1 when drift is found (for CI). Default exit is 0 for a pure report. |

**Behavior**

1. Resolve target units: paths given → units under those paths; no paths → every unit recorded in `.arclint/answers/`. A path that matches no recorded unit is an exit-2 error naming the path and suggesting `arclint new` (nothing recorded means nothing to re-render).
2. For each unit, re-render its template using the saved answers (plus any `--var` overrides), compare against the files on disk.
3. Default (dry-run): print a per-unit drift summary — files that would change or be created, with unified diffs under `--diff` and one-line-per-file otherwise. (`--verbose` is global logging only — progress and timing — and never changes what drift output is shown.)

```
drift: services/users-api (template: service, 3 files)
  modified  services/users-api/main.go
  modified  services/users-api/Dockerfile
  created   services/users-api/healthz.go
drift: services/billing (template: service, 1 file)
  modified  services/billing/Dockerfile

2 units drifted, 4 files affected. run with --apply to write.
```

4. With `--apply`: write the changes, update the affected units' answers files if overrides were given, print what was written. A drifted file the user edited (template unchanged) prints `restoring <path> (your edits replaced by template)` instead of the plain `wrote <path>`, since apply is discarding the user's edit; a file untouched since generation prints plain `wrote <path>`.
5. `--format json` emits a machine-readable drift report: a single object with a `units` array, one record per unit, each carrying its affected files. Per-unit `status` is one of `clean|drift|conflict|orphan`; per-file `status` is one of `added|changed|removed|conflict` (`removed` is reserved in the schema — nothing emits it in v1). Like check's JSON, the schema is stable: fields are added, never renamed or removed.

```json
{
  "units": [
    {
      "unit": "services/users-api",
      "template": "service",
      "status": "drift",
      "files": [
        { "path": "services/users-api/main.go", "status": "changed" },
        { "path": "services/users-api/Dockerfile", "status": "changed" },
        { "path": "services/users-api/healthz.go", "status": "added" }
      ]
    },
    {
      "unit": "services/billing",
      "template": "service",
      "status": "clean",
      "files": []
    }
  ]
}
```

`--apply` may overwrite rendered files — the template plus answers are the source of truth, and the default dry-run diff is the consent step. There are no protected regions inside rendered files in v1. One refinement (see the templating design's conflict policy): a file the user edited that the new render *also* changes is a conflict; `--apply` skips it and reports it, and only `--apply --take-template` overwrites the user's version. This is deliberate and stated in the command's help text.

**Exit codes:** 0 no drift, or drift reported/applied successfully; 1 drift found and `--fail-on-drift` set; 2 unknown path, corrupt answers file, or a `template.yaml` that is present but invalid. A recorded unit whose template directory has been deleted is not an error: the unit is reported with status `orphan` and the run exits 0. Drift has no severity concept: the exit code of `arclint make` is governed solely by `--fail-on-drift`.

---

### arclint check

Lint the architecture: run every rule in `rules.yaml` against the tree.

```
arclint check [paths...] [flags]
```

**Arguments**

- `[paths...]` — files or directories to check. Default: repo root (the directory containing `.arclint/`).

**Flags**

| Flag | Meaning |
|---|---|
| `--format text\|json` | Output format (global flag, most relevant here). |
| `--rules <ids>` | Comma-separated rule IDs to run exclusively. |
| `--skip <ids>` | Comma-separated rule IDs to skip. |
| `--jobs <n>` | Parallelism for the file walk. Default: number of CPUs. |

**Behavior**

1. Load `rules.yaml`, compile rules, walk the given paths in parallel (worker pool over the file walk; rules that operate on tree shape synchronize at the end).
2. Collect violations, group by category, sort within category by path then line.
3. Exit 0 if clean, 1 if any error-severity violation, 2 on config/usage errors (bad `rules.yaml`, unknown rule ID in `--rules`/`--skip`, nonexistent path). Severity `warn` never affects the exit code: warn violations are printed but a run with only warns exits 0.

`check` never prompts; it is read-only and CI-safe by construction. `--no-input` is accepted and has no effect.

**Text output** (default) — grouped by category, rule ID always visible, fix hint on its own indented line:

```
structure (2)
  services-need-dockerfile  services/*/Dockerfile  required pattern matched no files
          fix: add a Dockerfile, or run `arclint make --apply`
  no-utils-dir              pkg/util            package name "util" is on the forbidden-names list
          fix: rename to something domain-specific; see the no-utils-dir rule in rules.yaml

dependencies (1)
  no-cross-service-internals  services/billing/main.go:14  imports internal package of another service: services/users-api/internal/db
          fix: depend on the service API, not its internals

3 violations (2 structure, 1 dependencies) in 412 files, 38ms
```

With `--quiet`, only the violation lines and the summary line print. A clean run prints `0 violations in 412 files, 31ms` (nothing at all under `--quiet`).

**JSON output** — a single object on stdout; one record per violation:

```json
{
  "violations": [
    {
      "ruleId": "services-need-dockerfile",
      "category": "structure",
      "severity": "error",
      "path": "services/*/Dockerfile",
      "message": "required pattern matched no files",
      "fixHint": "add a Dockerfile, or run `arclint make --apply`"
    },
    {
      "ruleId": "no-cross-service-internals",
      "category": "dependencies",
      "severity": "error",
      "path": "services/billing/main.go",
      "line": 14,
      "message": "imports internal package of another service: services/users-api/internal/db",
      "fixHint": "depend on the service API, not its internals"
    }
  ],
  "summary": { "total": 2, "filesScanned": 412, "durationMs": 38 }
}
```

`line` is present only when the violation is line-anchored. For structure `require` violations, `path` is the unmatched glob pattern itself — arclint does not resolve the pattern to per-unit paths. The JSON schema is stable: fields are added, never renamed or removed.

**Exit codes:** 0 clean (or only warn-severity violations); 1 one or more error-severity violations; 2 config or usage error.

---

## TUI Prompting Model

### When a prompt fires

A prompt fires if and only if all three hold:

1. A required variable (one with no `default:` in the manifest) is still unresolved after checking explicit flags and saved answers.
2. stdin and stdout are both TTYs.
3. `--no-input` is not set.

If condition 2 or 3 fails and a required variable remains unresolved, the command exits 2 with a message naming the exact flag to pass. Nothing is half-done: the missing-input check runs before any filesystem writes.

```
error: missing required input "name" — pass --var name=<value> or run without --no-input
```

### What a prompt looks like

Prompts are huh forms, one field per unresolved variable, batched into a single form per command so the user answers everything in one pass. Each field shows:

- **Title** — the variable's human label from the manifest (`description:` field), falling back to the variable name.
- **Input** — empty; a prompt fires only when no flag or saved answer supplied the value and the variable has no manifest default, so there is nothing to prefill.
- **Validation** — the manifest's constraints (`validate` regex, `choices` membership) run inline via huh's validator; a failing value shows the constraint message under the field and keeps focus there. `choice` variables render as a select list rather than free text; `bool` variables as a yes/no toggle.

```
┃ Service name
┃ > users-api
┃   must match ^[a-z][a-z0-9-]*$
```

Prompts render to stderr, keeping stdout clean for `--format json` and pipes.

### Prompts teach flags

Every prompt maps 1:1 to a flag. After a form completes, arclint prints the non-interactive equivalent of what was just answered, dimmed, before doing the work:

```
tip: next time, skip the prompts with:
  arclint new service users-api --var port=8080 --var owner=platform-team
```

This line is suppressed by `--quiet` and appears only when at least one value actually came from a prompt (not when everything resolved from flags or defaults). It is the single most important DX touch in the CLI: interactive use continuously trains users toward scriptable use.

---

## Template Manifest Contract (CLI-relevant subset)

`arclint new` and `arclint make` read `.arclint/templates/<thing>/template.yaml`. The fields the CLI surface depends on:

```yaml
version: 1
description: A Go microservice with Dockerfile and health endpoint   # shown in --list and unknown-thing errors
destination: "services/{{ name | kebab }}"   # interpolated path where files/ renders, relative to repo root; --out overrides
variables:
  - name: name             # bound to the positional [name] argument
    description: Service name
    type: string
    validate: "^[a-z][a-z0-9-]*$"
  - name: port
    description: HTTP port
    type: string
    default: "8080"
  - name: with_db
    description: Does this service own a database?
    type: bool
    default: false
  - name: tier
    description: Service tier
    type: choice
    choices: [critical, standard, experimental]
    default: standard
```

A variable is required iff it has no `default:`; there is no separate `required:` field. Above, `name` is required (no default), while `port` and `tier` are optional. A prompt fires only when a required variable is unresolved (no flag, no saved answer) and the session is interactive; under `--no-input` an unresolved required variable is an exit-2 error.

The variable list is the single source for flag parsing (`--var`), prompt generation, and validation — one definition, three consumers, no drift possible between them.

## Saved Answers

Saved answers are sharded: one file per generated unit at `.arclint/answers/<unit-path-slug>.yaml`, where the slug is the unit's destination path with `/` replaced by `-`. Each file records the template that produced the unit and the fully-resolved variable set. For example, `.arclint/answers/services-users-api.yaml`:

```yaml
template: service
destination: services/users-api
answers:
  name: users-api
  port: 8080
  tier: standard
```

Written by `arclint new`, read by `arclint make`, updated by `arclint make --apply --var ...`. Answers files mutate only on apply: `arclint new` writes immediately because generation is itself the apply step; `arclint make` records overrides and newly-prompted variables (for example, after a template version bump) only with `--apply`; a dry-run never touches `.arclint/answers/`. The directory is committed to the repo — it is the record of what exists and why, and drift detection depends on it. Sharding (rather than one central answers.yaml) means parallel `arclint new` runs on different branches never merge-conflict.

---

## Exit Codes

Uniform across all commands:

| Code | Meaning |
|---|---|
| 0 | Success. Check clean, generation done, drift report ran (no `--fail-on-drift`), init created. |
| 1 | Findings. Error-severity check violations (warn-severity violations are printed but never affect the exit code), or drift found under `make --fail-on-drift`. |
| 2 | Usage or config error. Unknown command/flag/thing, invalid `rules.yaml` or `template.yaml`, missing required input under `--no-input`, destination already exists for `arclint new`, `.arclint/` exists without `--force`, conflicting flags. |

The 1/2 split matters for CI: 1 means "the tool ran and the repo has problems", 2 means "the invocation or configuration is broken". Pipelines can treat them differently.

## Error Message Style Guide

Every error is one line to stderr, prefixed `error: `, in two halves: **what happened** — **how to fix it**, joined by an em dash. No stack traces, no "unexpected error", no vagueness. Rules:

1. Name the exact thing: the path, the flag, the rule ID, the variable. Never "invalid input".
2. The fix half is an action the user can take right now: a flag to pass, a file to edit, a command to run.
3. Quote user-supplied values back so typos are visible: `unknown template "servce"`.
4. When the fix is a command, print it verbatim in backticks so it can be copy-pasted.
5. Multiple independent errors (e.g. several invalid `--var` values) print one line each, then a single exit.
6. Suggestions use closest-match (edit distance) whenever the input comes from a known finite set: template names, rule IDs, flag names.

Examples:

```
error: .arclint/ already exists — run with --force to overwrite, or edit .arclint/rules.yaml directly
error: unknown template "servce" — available: service, package, cli-tool (did you mean "service"?)
error: variable "name" value "Users API" fails pattern ^[a-z][a-z0-9-]*$ — use lowercase letters, digits, and hyphens
error: missing required input "port" — pass --var port=<value> or run without --no-input
error: destination services/users-api already exists — regenerate an existing unit with `arclint make services/users-api`
error: .arclint/rules.yaml line 12: unknown rule kind "forbid-imprts" — did you mean "forbid-imports"?
error: no .arclint/ found from /home/x/repo upward — run `arclint init` in the repo root
error: path "services/ghosts" has no recorded unit in .arclint/answers/ — generate it first with `arclint new`
```

## Performance Ground Rules

- Cold start budget: under 50ms to first useful output. The cobra command tree is constructed statically; no `init()`-time filesystem or network access anywhere.
- Config (`rules.yaml`, answers files under `.arclint/answers/`, manifests) loads lazily, after flag parse, only for commands that need it.
- `check` walks files with a worker pool (`--jobs`); rule compilation happens once, before the walk.
- `--help` and `--version` complete without touching the disk beyond the binary itself.
