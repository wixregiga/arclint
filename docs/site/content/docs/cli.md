+++
title = "CLI"
description = "Every command, with what it reads and writes."
weight = 6
+++

| command | does |
|---|---|
| `arclint init` | draft a starter `rules.yaml`; `--pattern bare` (default) or a built-in Pattern (`vertical`), which also copies Pattern extension entries into `.arclint/extensions`; `--languages go,ts,py` selects runtime targets and `--force` permits replacing existing targets |
| `arclint patterns` | list visible Pattern distribution packages, including built-in packages such as `arclint/vertical@0.1.0` |
| `arclint check [path]` | evaluate configured Rules; accepts `--format human\|json`, `--no-baseline`, and `--only` / `--exclude` Rule selectors |
| `arclint baseline capture` | replace `.arclint/baseline.v2.json` with the active findings from one complete assessment |
| `arclint baseline refresh` | reassess and replace the Baseline, dropping stale entries |
| `arclint context [paths...]` | explain the repository or everything binding the selected paths; `--module` adds named Modules and `--format json` emits the machine form |
| `arclint domain` | shorthand for `arclint domain overview`; inspect and maintain the project's ubiquitous language |
| `arclint domain init` | create an empty, schema-hinted `ubiquitous-language.yaml` beside the resolved `rules.yaml`; leave an existing file unchanged |
| `arclint domain overview` | summarize the project's ubiquitous language for understanding |
| `arclint domain list [type]` | list domain definitions, optionally filtered to `entities`, `aggregates`, `value-objects`, `business-rules`, or `events` |
| `arclint domain show <type> <name>` | show one domain definition by singular type and canonical name |
| `arclint domain explain [type]` | explain ArcLint's supported domain concepts |
| `arclint domain define <type> <name>` | create or update a domain definition; `--guided` starts interactive authoring |
| `arclint domain remove <type> <name>` | remove a domain definition (`rm` alias); never touches source files |
| `arclint domain schema` | print the JSON Schema accepted for `ubiquitous-language.yaml` |
| `arclint agents` | print the generated `AGENTS.md` architecture block; `--write` installs or refreshes it between markers without changing surrounding text |
| `arclint rules [selector]` | list configured Rules, or show one complete Rule when the selector has one exact match; broader selectors produce a narrowed list |
| `arclint rules schema` | print the indented JSON Schema accepted for `rules.yaml` |
| `arclint rules test [name]` | run all Rule Tests under `.arclint/tests`, or one test selected by name |
| `arclint sdk init` | write `arclint.d.ts` and `tsconfig.json` under `.arclint/extensions` |

Exit codes are `0` for a clean command, `1` when the gate fails or a
Rule Test does not match its expectation, and `2` for configuration or
usage errors. A gate failure can come from an active error-severity
Violation or an error-severity operational Diagnostic.

Commands that use repository configuration accept `--rules <path>` or
`--rules=<path>` to select `rules.yaml` explicitly. Otherwise ArcLint
discovers it upward from the working directory. `check [path]` starts
discovery from the optional path. The directory containing `rules.yaml`
is the repository root and Extension root.

## Rule tests

A rule test is one YAML file under `.arclint/tests/` (stem = test name):
fixture files plus the complete expected Diagnostics for one rule id.
`arclint rules test` materializes the fixtures and runs them through the
real parsers and evaluators.

```yaml
# .arclint/tests/disallowed-adapters-import.yaml
rule: "t:core/consumes"
files:
  go.mod: |
    module example.com/app

    go 1.26
  core/a.go: |
    package core

    import _ "example.com/app/adapters"
  adapters/a.go: |
    package adapters
expect: []
```

Author expected findings from the CLI, not from evaluator source. Start
with `expect: []` and run `arclint rules test`. Failures print every
unexpected finding as a ready-to-paste `.arclint/tests` entry — `kind`,
`path`, optional `line`, and `message` — under `unexpected findings
(add intended ones to expect):`. Copy only the entries you intend; leave
the list empty when the case must produce no Diagnostics. An empty list
that stays empty asserts exactly that, not a stronger conformance outcome.

The `message` field is an exact Diagnostic contract. Paste the
CLI-emitted text verbatim (Go-quoted). Do not hand-format or reconstruct
it: consumes, for example, renders quoted Module lists as
`Module(s) ["adapters"]`, not bare `"adapters"`. A failure for the
fixture above looks like:

```
FAIL disallowed-adapters-import (t:core/consumes)
  unexpected findings (add intended ones to expect):
    - kind: violation
      path: core/a.go
      line: 3
      message: "import \"example.com/app/adapters\" resolves to Module(s) [\"adapters\"], not in the allow-list of Module \"core\""
0 passed · 1 failed
```

Adopt that block into `expect:` and re-run until the case passes.

## Context for agents

`arclint context <path|module>` answers "what is architecturally true
where I am about to edit?" without loading the whole ruleset into a
prompt: the modules owning the path, their descriptions, what they may
import, every rule binding them, and the command that verifies the
result. A file path, a directory, or a declared module name all
resolve; an exact module name wins when both match. `--format json`
emits the machine shape for coding agents.

`arclint agents --write` covers the prompt-time half: it compiles the
ruleset into a generated block inside `AGENTS.md` (modules, dependency
policy, repo-wide rules, query commands) so agents see the architecture
before writing code. The block sits between markers; hand-written
content around it survives regeneration, and the block never carries
timestamps, so regeneration is idempotent for an unchanged ruleset.

## Shell completion

`arclint completion bash|zsh|fish|powershell` emits the shell script.
Completion uses the resolved `rules.yaml` when available: Rule IDs for
`rules` and the `check --only` / `--exclude` selectors, Module names for
`context --module`, supported languages for `init --languages`, and the
closed `human` / `json` output-format values. Without a loadable
repository configuration, repository-derived candidates stay empty.

## JSON diagnostics

`arclint check --format json` emits an array containing every Diagnostic
in the complete Conformance Assessment. The stable shape is:

```json
[
  {
    "kind": "violation",
    "ruleId": "repo:dependencies/application-inward",
    "path": "internal/delivery/handler.go",
    "line": 3,
    "severity": "error",
    "status": "active",
    "message": "import resolves to Module \"infrastructure\", not allowed by Module \"delivery\"",
    "remediation": "depend on the inward-owned port"
  }
]
```

`kind` is `violation`, `operational`, or `coverage`. Fields that do not
apply are omitted: coverage Diagnostics have no Severity, operational
Diagnostics may have no Rule ID, and only Violation Diagnostics have a
status. `line` is omitted when the Diagnostic is not line-anchored, and
`remediation` is omitted when none was supplied.

The default human output prints active Violations and operational or
coverage Diagnostics, followed by counts for active, suppressed, and
baselined findings and applied Rules. Suppressed and baselined
Violations remain part of the Assessment and JSON output but do not
appear as active findings.

## Schema output

`arclint rules schema` always emits indented JSON. The same generated
schema is committed as `docs/rules.schema.json`; tests keep the command
output and committed file byte-for-byte identical.

## Project domain model

`arclint domain` inspects and maintains the project's Ubiquitous Language:
the shared language used by developers, domain experts, documentation,
tests, and code. ArcLint records that language as a structured Project
Domain Model in a committed `ubiquitous-language.yaml` beside the
resolved `rules.yaml`. ArcLint does not search for the model
independently; `--rules <path>` moves both files' project root together.

Running `arclint domain` with no subcommand is the same as
`arclint domain overview`. Subcommands cover file initialization
(`init`), summary (`overview`), inventory (`list`), one definition
(`show`), ArcLint-owned concept meanings (`explain`), create-or-update
(`define`, including `--guided`), removal (`remove` / `rm`), and the
published schema (`schema`). `domain init` is idempotent: it creates a
versioned, schema-hinted empty file when none exists and never replaces
an existing model.

Every domain subcommand except `init` and `schema` accepts `--format text|json`
(default `text`). Text is for humans; JSON is the stable machine shape
for agents and scripts. `domain schema` always emits indented JSON, the
same contract committed as `docs/ubiquitous-language.schema.json`.

Domain command exit codes:

| code | meaning |
|---|---|
| `0` | operation succeeded, including an unchanged `define` |
| `1` | operation could not be completed (missing definition, malformed model, write failure) |
| `2` | invalid command usage (unknown type, missing name, mutually exclusive flags, bad `--format`) |

Declaring knowledge never creates a Diagnostic by itself. Domain quality
findings remain the responsibility of enabled Rules under
`arclint check`. `arclint context` surfaces the recorded model when
present; focused path relevance stays with `context`, not `domain`.
