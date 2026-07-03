# arclint Templating Engine

Status: Draft design. Owner: templating subagent. Scope: the template format, manifest schema, rendering semantics, regeneration semantics for `arclint make`, and the Go engine implementation.

This document assumes the locked project conventions: templates live in `.arclint/templates/<thing>/`, one directory per thing type; manifests are declarative copier-style `template.yaml` files with no code; interpolation is mustache-style `{{ var }}` with a fixed filter set and no loops or conditionals in template content; answers are recorded sharded, one file per generated unit at `.arclint/answers/<unit-path-slug>.yaml`; input priority is flags > saved answers > defaults, with an interactive prompt firing only for a required variable (one with no default) that flags and saved answers left unresolved.

## 1. Template directory anatomy

A template is a directory under `.arclint/templates/` whose name is the thing type. Dropping a new directory with a `template.yaml` inside it is the entire registration process — arclint discovers thing types by listing that directory at startup. There is no registry file, no plugin hook, no config edit. This is the hygen folder-drop model.

```
.arclint/
  templates/
    repo/                  # built-in, shipped by `arclint init`
    service/               # built-in
    component/             # built-in
    docs-page/             # user-defined: exists because the directory exists
      template.yaml        # manifest: variables, prompts, metadata
      files/               # the render root; everything under here is rendered
        ...
  answers/                 # sharded saved answers, one file per generated unit (see section 4)
```

Only two entries are meaningful inside a template directory:

- `template.yaml` — the manifest. Required. Declares the template's variables, version, and destination root.
- `files/` — the render root. Required. Every file and directory under `files/` is rendered into the destination, with both content and path names interpolated. Anything else in the template directory (a `README.md`, notes, fixtures) is ignored by the renderer, so template authors can document their templates freely.

### Worked example: the built-in `service` template

Manifest, `.arclint/templates/service/template.yaml`:

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

Files tree, `.arclint/templates/service/files/`:

```
files/
  cmd/{{ name | kebab }}/main.go
  internal/handler.go
  internal/handler_test.go
  service.yaml
```

`files/cmd/{{ name | kebab }}/main.go`:

```go
package main

import (
	"log"

	"{{ repo_name }}/services/{{ name | kebab }}/internal"
)

func main() {
	log.Printf("starting {{ name | kebab }} ({{ transport }})")
	if err := internal.Run(); err != nil {
		log.Fatal(err)
	}
}
```

`files/internal/handler.go`:

```go
package internal

// {{ name | pascal }}Handler handles {{ transport }} requests for the
// {{ name }} service.
type {{ name | pascal }}Handler struct{}

// Run starts the {{ name | kebab }} service.
func Run() error {
	h := &{{ name | pascal }}Handler{}
	_ = h
	return nil
}
```

`files/internal/handler_test.go`:

```go
package internal

import "testing"

func Test{{ name | pascal }}Handler(t *testing.T) {
	t.Skip("scaffolded test for {{ name | kebab }} — implement me")
}
```

`files/service.yaml`:

```yaml
name: {{ name | kebab }}
transport: {{ transport }}
database: {{ db_name }}
```

Rendering `arclint new service --var name="payment gateway" --var transport=http --var with_db=true --var db_name=payments` produces:

```
services/payment-gateway/
  cmd/payment-gateway/main.go       # PaymentGatewayHandler, etc.
  internal/handler.go
  internal/handler_test.go
  service.yaml
```

Note the `{{ repo_name }}` variable in `main.go`: variables not declared in the manifest but provided by arclint itself are injected as built-ins. Built-ins are documented in section 3.

## 2. Manifest schema

The manifest is deliberately small. Every field, exhaustively:

### Top-level fields

| Field | Type | Required | Meaning |
|---|---|---|---|
| `version` | int | yes | Template version. Monotonically increasing integer, bumped by the author on any change to variables or files. Used by regeneration (section 4). |
| `description` | string | no | One-line human description, shown in `arclint new --list`. |
| `destination` | string | yes | Interpolated path, relative to repo root, where `files/` is rendered. This is also the identity of the generated unit (section 4). |
| `variables` | list | yes | Ordered list of variable declarations. Order matters: prompts appear in this order, and `when` conditions may only reference variables declared earlier. |

### Variable fields

| Field | Type | Required | Meaning |
|---|---|---|---|
| `name` | string | yes | Identifier. `^[a-z][a-z0-9_]*$`. Referenced in templates as `{{ name }}` and on the CLI via the generic `--var` flag (`--var db_name=payments`). |
| `description` | string | yes | The prompt text shown to the user. |
| `type` | enum | yes | `string`, `bool`, or `choice`. |
| `default` | scalar | no | Default value. May itself contain `{{ }}` interpolation referencing earlier variables (see the `db_name` example above). A variable is required iff it has no `default` — there is no separate `required:` field. A required variable with no flag and no saved answer is prompted when interactive; under `--no-input` it is an exit-2 error. |
| `choices` | list of strings | only for `choice` | Allowed values. The default, if given, must be a member. |
| `validate` | string (regex) | no, `string` only | Go `regexp` syntax (RE2). The full input must match (the engine anchors the pattern with `\A...\z`). On failure the prompt repeats with the pattern shown; a failing `--var` value is a hard error. |
| `when` | string | no | Condition on prior answers. If false, the variable is skipped: not prompted, not settable by flag, and rendered as the empty string. |

### `when` expression grammar

`when` is intentionally not a scripting language. The grammar is:

```
condition  := clause ( "&&" clause )*
clause     := ident op literal
op         := "==" | "!="
literal    := true | false | quoted string | bare word
```

Examples: `with_db == true`, `transport != worker`, `transport == grpc && with_db == true`. No `||`, no parentheses, no comparisons other than equality. If a condition needs more than that, the template is trying to be a program, and the answer is to split it into two thing types. Referencing a variable declared later in the list is a manifest validation error, which keeps evaluation single-pass and cycle-free.

### Full manifest example exercising everything

```yaml
version: 3
description: "Kitchen-sink demonstration manifest"
destination: "widgets/{{ name | kebab }}"

variables:
  - name: name
    description: "Widget name"
    type: string
    validate: "^[a-zA-Z][a-zA-Z0-9 -]{1,40}$"

  - name: flavor
    description: "Which flavor of widget?"
    type: choice
    choices: [basic, fancy, experimental]
    default: basic

  - name: with_docs
    description: "Generate a docs stub?"
    type: bool
    default: true

  - name: docs_title
    description: "Title for the docs page"
    type: string
    default: "{{ name | pascal }} Widget"
    when: "with_docs == true"

  - name: experimental_flag
    description: "Feature-flag key for experimental widgets"
    type: string
    validate: "^[a-z][a-z0-9_]*$"
    when: "flavor == experimental"
```

Manifest validation happens before any prompting: unknown fields, bad regexes, `choices` on non-choice types, defaults outside `choices`, forward references in `when`, and duplicate variable names are all hard errors with file-and-line positions. Because manifests contain no code, validating one is cheap and side-effect-free, which also means `arclint` can lint the templates directory itself.

## 3. Rendering semantics

### Interpolation

The only template syntax is `{{ expr }}` where `expr` is `var` followed by zero or more filters: `var`, `var | filter`, or a chain like `{{ x | snake | upper }}`. Filter chaining is allowed and filters apply left to right. Whitespace inside the braces is insignificant: `{{name}}`, `{{ name }}`, and `{{ name|kebab }}` are identical.

Filters, all pure string-to-string:

| Filter | Input `payment gateway` becomes |
|---|---|
| `pascal` | `PaymentGateway` |
| `camel` | `paymentGateway` |
| `snake` | `payment_gateway` |
| `kebab` | `payment-gateway` |
| `upper` | `PAYMENT GATEWAY` |
| `lower` | `payment gateway` |
| `plural` | `payment gateways` |

Case filters first split the input into words on spaces, hyphens, underscores, and lower-to-upper case boundaries, then reassemble. `plural` uses a small English rule table (trailing `s`/`x`/`z`/`ch`/`sh` → `+es`, consonant-`y` → `ies`, else `+s`) with an exceptions map for common irregulars; it is a convenience, not a linguistics engine.

Bool variables render as `true`/`false`. A variable skipped by `when` renders as the empty string. Referencing an undeclared, non-built-in variable is a render-time error naming the file and offset — never silently empty, because silent empties produce plausible-looking broken output.

Built-in variables injected by arclint (never declared in manifests): `repo_name` (the git root directory name), `year` (current year), `arclint_version`. There is no `module` built-in — a Go module path comes from `go.mod`, which is language-specific and would contradict arclint's language-agnostic positioning. Built-ins lose to manifest variables of the same name, and manifests shadowing a built-in get a lint warning.

There are deliberately no loops and no conditionals in template content. A template file is either rendered whole or not present; anything conditional belongs in the manifest (`when` on a variable) or in a separate thing type. This keeps every template file valid enough to read, diff, and lint, and keeps rendering a single linear pass.

### Paths

File and directory names under `files/` go through exactly the same interpolation as content — `files/cmd/{{ name | kebab }}/main.go` is the canonical example. After interpolation, every rendered path is verified to be relative and free of `..` segments; a path that interpolates to something escaping the destination root is a hard error (path traversal guard). Two files whose names interpolate to the same destination path is also a hard error, reported before anything is written.

### Escaping literal `{{`

To emit a literal `{{`, write `{{{{`. To emit a literal `}}`, write `}}}}`. The tokenizer treats the doubled forms as escapes and emits the single form. This matters for templates that generate other templates, GitHub Actions files (`${{ ... }}`), or Helm charts. A lone `{{` that never closes before end of file is a render error, not silently passed through — templates targeting `{{`-heavy formats must escape, which keeps the grammar unambiguous.

### Binary files

Before rendering, each file is sniffed with the same heuristic Git uses: if the first 8000 bytes contain a NUL byte, the file is binary. Binary files are copied verbatim — no interpolation of content, though their *names* are still interpolated. This lets templates ship icons, fonts, or fixture archives without corruption. There is no marker or manifest field for this; the sniff is automatic and cheap.

### Empty directories

Git does not track empty directories, so a template that wants to scaffold one (e.g. an empty `migrations/` folder) contains a `.gitkeep` file in it. The renderer treats `.gitkeep` as ordinary content: it is copied through, so the generated project also carries the `.gitkeep` and survives commits. No special casing; the convention is documented, not enforced.

### Write behavior

`arclint new` renders everything to memory first, then writes. If the destination directory already exists, generation hard-refuses before writing anything, with an error pointing the user to `arclint make` for regenerating an existing unit — `arclint new` has no `--force`. Rendering is all-or-nothing: a render error anywhere means zero files are written.

### Destination collision across templates

If the interpolated destination is already claimed by a recorded unit generated from a *different* template, `arclint new` hard-refuses before writing anything. The error names both templates and the contested destination:

```
error: destination "services/payment-gateway" is already claimed by template "service" — template "docs-page" cannot generate there; pick a different destination or delete the existing unit first
```

There is no last-write-wins and no warning-and-continue path. One destination, one unit, one template is an invariant.

## 4. Regeneration: `arclint make`

### The answers files

Answers storage is sharded: every successful `arclint new` writes (or updates) one file per generated unit at `.arclint/answers/<unit-path-slug>.yaml`, where the slug is the unit's destination path with `/` replaced by `-` (e.g. `services/payment-gateway` → `services-payment-gateway.yaml`). There is no single central answers file — sharding means two branches each running `arclint new` touch different files and never merge-conflict. The files are committed to the repo; they are the durable link between generated code and the template that produced it — the copier pattern.

`.arclint/answers/services-payment-gateway.yaml`:

```yaml
version: 1
template: service
template_version: 1
destination: services/payment-gateway
generated_at: "2026-07-03T14:12:09Z"
answers:
  name: "payment gateway"
  transport: http
  with_db: true
  db_name: payments
files:
  cmd/payment-gateway/main.go: "9c4f1e0a2b7d5f6e8a1c3b5d7f9e0a2c4e6f8a0b2c4d6e8f0a1b3c5d7e9f1a3b"
  internal/handler.go: "5d7e9f1a3b5c7d9e1f3a5b7c9d1e3f5a7b9c1d3e5f7a9b1c3d5e7f9a1b3c5d7e"
  internal/handler_test.go: "1f3a5b7c9d1e3f5a7b9c1d3e5f7a9b1c3d5e7f9a1b3c5d7e9f1a3b5c7d9e1f3a"
  service.yaml: "7b9c1d3e5f7a9b1c3d5e7f9a1b3c5d7e9f1a3b5c7d9e1f3a5b7c9d1e3f5a7b9c"
```

The `files:` map records the SHA-256 of every rendered file at generation (or last apply) time, keyed by path relative to the destination. It is what classifies a difference during regeneration: a disk hash that no longer matches the recorded hash means the user edited the file; a new render that no longer matches the recorded hash means the template changed; both at once is a conflict (section 4, conflict policy).

`.arclint/answers/docs-pages-getting-started.yaml`:

```yaml
version: 1
template: docs-page
template_version: 2
destination: docs/pages/getting-started
generated_at: "2026-07-01T09:30:00Z"
answers:
  title: "Getting Started"
  section: guides
files:
  index.md: "3e5f7a9b1c3d5e7f9a1b3c5d7e9f1a3b5c7d9e1f3a5b7c9d1e3f5a7b9c1d3e5f"
```

A generated unit is identified by its `destination` path. One destination, one unit, one template, one answers file. Answers are stored post-resolution (after defaults and `when` were applied), so a re-render is fully deterministic without re-prompting. Variables skipped by `when` are stored as absent, not empty, so a later template version that changes a `when` condition re-prompts correctly.

### Drift detection

`arclint make` (no arguments) walks every unit file in `.arclint/answers/` and, for each:

1. Loads the current template and the saved answers.
2. Renders the entire unit **to memory** — nothing touches disk.
3. Diffs the in-memory render against the files currently on disk under `destination`.
4. Reports one of: `clean` (identical), `drift` (differences exist), `conflict` (a drifted file is both user-edited and template-changed), or `orphan` (template directory no longer exists, or destination was deleted). `orphan` is a report, not an error — the run still exits 0; only a `template.yaml` that is present but invalid is an exit-2 config error.

Output is a summary table plus, with `--diff`, a unified diff per drifted file. The default dry-run always exits 0, even when drift is found — it is a pure report. Passing `--fail-on-drift` makes drift exit 1, which is how `arclint make --fail-on-drift` slots into CI as a scaffolding-drift gate. Drift has no severity concept: the exit code of `arclint make` is governed solely by `--fail-on-drift`.

Drift is not inherently bad — users are expected to edit generated files. Drift reporting answers "what has diverged from the template," which becomes actionable the moment the template changes.

### Template version bumps

When a template author bumps `version` in `template.yaml`, every unit recorded with an older `template_version` is flagged `outdated` by `arclint make` regardless of drift. Bringing a unit up to date:

```
arclint make services/payment-gateway
```

This re-renders with saved answers against the new template. If the new template declares variables that have no saved answer (new variable, or a `when` that now activates), arclint prompts for just those — flags can pre-supply them following the usual priority. The result is diffed against disk and shown; nothing is written yet. Newly-prompted answers are recorded into the unit's answers file only when `--apply` runs — a dry-run never mutates `.arclint/answers/`.

### Conflict policy

arclint never overwrites user edits silently — the default dry-run diff is the consent step. Classification uses the recorded `files:` hashes (disk vs. recorded = user edit; new render vs. recorded = template change). The rules, in order:

- A file the user never touched (disk matches the recorded hash) whose new render differs: shown in the diff as a clean template update.
- A file the user edited whose template output is unchanged (new render still matches the recorded hash): this is drift. Template plus answers are the source of truth, so `--apply` restores the file from the template — user edits do not survive an apply. Changes meant to persist belong in the template or its variables.
- A file the user edited **and** whose template output changed: this is a conflict. The diff shows both sides (disk vs. new render). arclint does not attempt three-way merge in v1 — the diff is the deliverable, and the user reconciles by hand or accepts the template side.

No write happens without the explicit `--apply` flag:

```
arclint make services/payment-gateway --apply
```

`--apply` writes clean updates, updates `template_version` and any new answers in the unit's answers file, and still refuses to touch conflicted files unless `--apply --take-template` is also given (which overwrites the user's version — the one intentionally destructive path, and it says so loudly). There is no `--take-mine` because keeping the disk version is what happens by default when a conflicted file is skipped; the unit's recorded `template_version` still advances, with the skipped files listed in the command output.

Deleting a unit is manual: remove the files and the unit's answers file under `.arclint/answers/`. `arclint make` flags the half-deleted states (`orphan`) but never deletes user files.

There are no protected regions or edit markers inside rendered files in v1: the dry-run diff (default, no `--apply`) is the consent step, and `--apply` may overwrite rendered files subject to the conflict policy above.

## 5. Engine implementation notes (Go)

### Why not text/template

`text/template` was evaluated and rejected. Its syntax admits `{{if}}`, `{{range}}`, `{{with}}`, function calls, and pipeline chaining into arbitrary registered functions — exactly the logic leak the design forbids. Restricting it means parsing templates twice (once to reject constructs, once to execute) and its error positions are notoriously vague. A hand-rolled renderer for the grammar above is roughly 200 lines, is strict by construction (anything that is not `var | filter` is a parse error with byte-exact position), allocates less, and has no API surface that future contributors can quietly grow logic through. The grammar is small enough that the tokenizer *is* the specification.

### Tokenizer and renderer

Single pass over the input `[]byte`, tracking only an offset and a small state:

1. Scan for `{{`. Everything before it is a literal chunk, appended to the output as-is.
2. On `{{{{`, emit literal `{{`, advance, continue scanning. Same for `}}}}` → `}}` in literal context.
3. On `{{`, scan to the matching `}}`. Unterminated tag at EOF is an error carrying the opening offset.
4. Parse the tag body: trim spaces, split on `|`, first segment must be a valid identifier, remaining segments must each be a key in the filter table. Empty body, bad identifier, or unknown filter are errors with the tag's file/line/column.
5. Look up the variable in the answer map (manifest answers overlaid on built-ins). Missing variable is an error. Apply filters left to right. Append result.

The same function renders paths and content; a path is just a short template. The whole engine is:

```go
type Renderer struct {
	vars    map[string]string          // resolved answers + built-ins
	filters map[string]func(string) string
}

func (r *Renderer) Render(src []byte, origin string) ([]byte, error)
```

No interfaces, no options struct, no reflection. Errors wrap a `Position{Origin string; Line, Col int}` so every failure points at a file location.

### Filter table

A package-level `map[string]func(string) string` with exactly seven entries. `pascal`, `camel`, `snake`, `kebab` share one word-splitting helper (`splitWords(s string) []string` handling spaces, `-`, `_`, and case boundaries); `upper`/`lower` are `strings.ToUpper`/`ToLower`; `plural` is the rule table described in section 3. The table is not exported and there is no registration function — adding a filter is a code change with a doc change, on purpose. Property tests pin the round-trip identities (`kebab(pascal(x)) == kebab(x)` for word-clean inputs).

### Performance envelope

Rendering is I/O bound. A template with a few dozen files renders to memory in well under a millisecond of CPU; the drift check for a whole repo is bounded by reading the generated files once. No caching layer is needed or wanted at this scale. The binary-sniff (first 8000 bytes, NUL check) happens on the same read used for rendering, so no extra I/O.

### Package layout

```
internal/template/
  manifest.go        # template.yaml parsing + validation
  renderer.go        # tokenizer + Render (the ~200 lines)
  filters.go         # filter table + splitWords + plural rules
  answers.go         # sharded answers files (.arclint/answers/*.yaml) load/save
  engine.go          # orchestration: discover, resolve inputs, render tree, diff
```

`goccy/go-yaml` (the locked-stack YAML library used everywhere in arclint) handles the manifest and answers files; the feature adds no new dependency.

## 6. Extension story: a custom thing type in under two minutes

Goal: the user wants a `docs-page` thing so every documentation page starts from the same skeleton.

Step 1 — make the directory (10 seconds):

```
mkdir -p .arclint/templates/docs-page/files
```

Step 2 — write the manifest, `.arclint/templates/docs-page/template.yaml` (45 seconds):

```yaml
version: 1
description: "A documentation page with front matter"
destination: "docs/pages/{{ title | kebab }}"

variables:
  - name: title
    description: "Page title"
    type: string

  - name: section
    description: "Which docs section?"
    type: choice
    choices: [guides, reference, tutorials]
    default: guides
```

Step 3 — write one template file, `.arclint/templates/docs-page/files/index.md` (30 seconds):

```markdown
---
title: "{{ title }}"
section: {{ section }}
slug: {{ title | kebab }}
---

# {{ title }}

Write the {{ section }} content for {{ title }} here.
```

Step 4 — use it (15 seconds):

```
arclint new docs-page --var title="Getting Started" --var section=guides
```

That renders `docs/pages/getting-started/index.md` and records the unit in `.arclint/answers/docs-pages-getting-started.yaml`, which means the new page immediately participates in `arclint make` drift checking. No registration, no config edit, no restart: the thing type existed the moment the directory did. Total elapsed: well under two minutes, and most of that is deciding on the variable names.

## Resolved questions (orchestrator decisions)

1. Filter chaining: **allowed**. `{{ x | snake | upper }}` applies filters left to right (section 3).
2. Answers storage: **sharded**. One file per generated unit at `.arclint/answers/<unit-path-slug>.yaml`; no central answers.yaml, so parallel `arclint new` runs on different branches never merge-conflict (section 4).
3. Destination collision across templates: **hard refuse**. `arclint new` errors before writing, naming both templates (section 3, write behavior). No last-write-wins.
