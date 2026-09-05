+++
title = "CLI"
description = "Every command, with what it reads and writes."
weight = 6
+++

| command | does |
|---|---|
| `arclint init` | draft a starter `rules.arclint.yaml`; `--pattern bare` (default) or a Pattern to extend by exact reference or name (`arclint/vertical@0.1.0`, `vertical`), which drafts the `extends` block with the Pattern's suggested bindings; `--languages go,ts,py` selects runtime targets and `--force` permits replacing an existing file |
| `arclint patterns` | list the Patterns that resolve offline: those embedded in the binary (`arclint/vertical@0.1.0`, `arclint/domain-model@0.1.0`), then vendored and authored packages under `.arclint/patterns/<namespace>/<name>/`; `--remote` lists what the Registry publishes instead (`--registry <url>` or `ARCLINT_REGISTRY` names one; `file://` trees work) |
| `arclint patterns install <pattern>` | extend `rules.arclint.yaml` with one Pattern named by reference, `namespace/name`, or bare name, binding every Module it lists; vendors it first when it came from the Registry; drafts `rules.arclint.yaml` when none exists (`--languages`) |
| `arclint patterns vendor <pattern>` | copy one Pattern under `.arclint/patterns/<namespace>/<name>/` with its `manifest.json`, so every load verifies it and the Registry is never needed again |
| `arclint patterns export <pattern> --dir <tree>` | publish one offline Pattern into a Registry tree on disk: `<tree>/<namespace>/<name>/<version>/` plus `<tree>/index.json` |
| `arclint check [path]` | evaluate configured Rules; accepts `--no-baseline` and `--only` / `--exclude` Rule selectors |
| `arclint baseline capture` | replace `.arclint/baseline.v2.json` with the active findings from one complete assessment |
| `arclint baseline refresh` | reassess and replace the Baseline, dropping stale entries |
| `arclint context [paths...]` | explain the repository or everything binding the selected paths; `--module` adds named Modules, `--full` lists the whole recorded domain instead of the part anchored into the scope |
| `arclint domain` | shorthand for `arclint domain overview`; inspect and maintain the project's ubiquitous language |
| `arclint domain init` | create an empty, schema-hinted `domain.arclint.yaml` beside the resolved `rules.arclint.yaml`; leave an existing file unchanged |
| `arclint domain overview` | summarize the project's ubiquitous language for understanding |
| `arclint domain list [type]` | list domain definitions, optionally filtered to `entities`, `value_objects`, `invariants`, `assertions`, `specifications`, or `events` |
| `arclint domain show <type> <name>` | show one domain definition by singular type and canonical name |
| `arclint domain explain [type]` | explain ArcLint's supported domain concepts |
| `arclint domain define <type> <name>` | create or update a domain definition inside a bounded context; `--guided` starts interactive authoring |
| `arclint domain remove <type> <name>` | remove a domain definition (`rm` alias); never touches source files |
| `arclint domain schema` | print the JSON Schema accepted for `domain.arclint.yaml`; `--write` puts it at `.arclint/schemas/domain.arclint.schema.json` (or under `--dir`) so the file's modeline can name a local copy |
| `arclint agents` | command group for agent-facing artifacts |
| `arclint agents md` | print the generated `AGENTS.md` architecture block (`markdown`, `agentsmd` aliases); `--write` installs or refreshes it between markers without changing surrounding text |
| `arclint agents skill` | write generated `SKILL.md` and `VOCAB.yaml` to `--dir` (default `.agents/skills/domain-librarian/`), and the domain schema they point at to `.arclint/schemas/domain.arclint.schema.json` |
| `arclint rules [selector]` | list configured Rules, or show one complete Rule when the selector has one exact match; broader selectors produce a narrowed list |
| `arclint rules schema` | print the indented JSON Schema accepted for `rules.arclint.yaml`; `--write` puts it at `.arclint/schemas/rules.arclint.schema.json` (or under `--dir`) so the ruleset's modeline can name a local copy |
| `arclint rules test [name]` | run all Rule Tests under `.arclint/tests`, or one test selected by name |
| `arclint sdk init` | write `arclint.d.ts` and `tsconfig.json` under `.arclint/extensions` |

`--format human|json` selects one renderer for every semantic command
report. Human output is styled when stdout is a terminal. `--no-color`
or a non-empty `NO_COLOR` environment variable selects the byte-stable
plain renderer. Raw schema and generated markdown commands are unchanged.

Exit codes are `0` for a clean command, `1` when the gate fails or a
Rule Test does not match its expectation, and `2` for configuration or
usage errors. A gate failure can come from an active error-severity
Violation or an error-severity operational Diagnostic.

A complete Rule (`arclint rules <id>`) prints the Claim the author
wrote and, beside it, `asserts`: the canonical statement of what the
parameters assert, in domain language. The two differ on purpose: the
Claim is the proposition, `asserts` is its operational content, such
as the layer order, the allow-list, the cycle scope, or the globs an
expanded structure Rule derived. JSON carries the same string as
`asserts`.

```
$ arclint rules acme/hexagonal:dependencies/acyclic
id:          acme/hexagonal:dependencies/acyclic
type:        acyclic
severity:    error
claim:       Module dependencies contain no cycle.
asserts:     dependencies among ["core", "ports", "adapters"] contain no cycle
applies to:  the entire repository
```

Commands that use repository configuration accept `--rules <path>` or
`--rules=<path>` to select `rules.arclint.yaml` explicitly. Otherwise ArcLint
discovers it upward from the working directory. `check [path]` starts
discovery from the optional path. The directory containing `rules.arclint.yaml`
is the repository root and Extension root. `patterns` commands compose
against the working directory instead, so `patterns install` can draft
a `rules.arclint.yaml` where none exists yet.

## Patterns

`arclint patterns` and its subcommands move Patterns between the three
places a Pattern resolves from: the binary, `.arclint/patterns`, and a
Registry. A check never reaches a Registry; `install` and `vendor` read
one only for a Pattern that is neither embedded nor under
`.arclint/patterns`, and `--remote` lists one from its index alone.

```
$ arclint patterns install acme/layers --registry file:///srv/patterns
installed acme/layers@1.0.0 (registry, 3fb2cbda1af6)
vendored to /work/shop/.arclint/patterns/acme/layers
extended /work/shop/rules.arclint.yaml
bound:
  app: internal/app/**
unbound (bind each under extends[].bind before the ruleset loads):
  domain
next: bind the unbound modules, then run `arclint check .`
```

`install` reports the source it resolved from (`embedded`, `local`, or
`registry`), the short digest of the exact files, where the vendored
copy went, whether `rules.arclint.yaml` was written or extended (and which
version an existing entry moved from), every binding it wrote, the
declared Modules it folded into bindings, and the Modules still to
bind. `vendor` reports the directory written or that an identical copy
was already there; `export` reports the version directory and the
index it updated.

Requests to an `https` Registry carry `Authorization: Bearer` with
`GITHUB_TOKEN`, else `GH_TOKEN`, when one is set. A `404` from the
Registry is reported as the Pattern not being published there; `401`
and `403` name the two variables.

With `--format json`:

```json
{
  "registry": "file:///srv/patterns",
  "patterns": [
    {
      "reference": "acme/layers@1.0.0",
      "namespace": "acme",
      "name": "layers",
      "version": "1.0.0",
      "source": "registry",
      "vendored": false,
      "authored": false,
      "digest": "sha256:3fb2cbda1af6...",
      "documentation": "Two layers: a domain that imports nothing, and an app above it.",
      "rules": 1,
      "extensions": 0,
      "coverage": ["go"]
    }
  ]
}
```

`registry` is present only for `--remote`. `source` is `embedded`,
`local`, or `registry`; `vendored` and `authored` say what the
repository carries under `.arclint/patterns` regardless of `source`.
`install` emits `{reference, digest, source, vendoredPath?,
vendorReplaced?, rulesetPath, rulesetCreated, rulesetReplaced?,
bound: [{module, paths}], unbound: [], adopted?}`; `vendor` emits
`{reference, digest, source, path?, replaced?, unchanged}`; `export`
emits `{reference, digest, versionDir, indexPath, replaced}`.

## Rule tests

A rule test is one YAML file under `.arclint/tests/` (stem = test name):
fixture files plus the complete expected Diagnostics for one rule id.
`arclint rules test` materializes the fixtures and runs them through the
real parsers and evaluators.

```yaml
# .arclint/tests/disallowed-adapters-import.yaml
rule: "core/consumes"
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
unexpected finding as a ready-to-paste `.arclint/tests` entry (`kind`,
`path`, optional `line`, and `message`) under `unexpected findings
(add intended ones to expect):`. Copy only the entries you intend; leave
the list empty when the case must produce no Diagnostics. An empty list
that stays empty asserts exactly that, not a stronger conformance outcome.

The `message` field is an exact Diagnostic contract. Paste the
CLI-emitted text verbatim (Go-quoted). Do not hand-format or reconstruct
it: consumes, for example, renders quoted Module lists as
`Module(s) ["adapters"]`, not bare `"adapters"`. With
`core: "core/**"` and `adapters: "adapters/**"` declared as Modules and
`core/consumes` asserting `imports: {internal: []}` on `core`, a failure
for the fixture above looks like:

```
FAIL disallowed-adapters-import (core/consumes)
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

When `domain.arclint.yaml` is present, `context` also carries the
recorded domain, disclosed progressively. A worksite lists only what
anchors into it: a term whose type declaration lies under a selected
path or inside a selected Module, an invariant or assertion whose
carrying declaration lies there, and a whole bounded context whenever a
selected Module is named for it. The headline counts the listing
against the whole model (`1 of 4 contexts, 3 of 25 invariants anchor
into this scope`) and names `--full`, which lists the whole recorded
model instead; bare `arclint context` always lists it whole. Each
invariant, assertion, and specification line ends with its anchor: the
`file:line` of the declaration that carries it, `missing` when the
declaration the recorded shape names (a value object's constructor, an
aggregate method named for the invariant id, a specification's
`SatisfiedBy`) is not declared, or `unanchorable` when the recorded
shape names no declaration at all. The listing closes with an
`unanchored contracts:` block that groups every missing and
unanchorable contract by owner and cause so an agent cannot skim past
them: an unanchorable contract needs its recording changed before any
source can carry it, and an `invariants` Rule on the owning Module
turns each missing contract into a Violation under `arclint check`.
The JSON shape carries the same facts under `domain.scoped`,
`domain.shown`, `domain.located`, per-contract `anchor` and `reason`,
and `domain.unanchored`.

`arclint agents md --write` covers the prompt-time half: it compiles the
ruleset, the recorded vocabulary, and the local extension registry into
a generated block inside `AGENTS.md`: an ask-arclint-first directive,
the full command surface with when-to-use guidance, a recorded-domain
snapshot, every module with its rule claims, repository-wide rules, and
the local extension inventory. Agents see the architecture before
writing code. The block sits between markers; hand-written content
around it survives regeneration, and the block never carries
timestamps, so regeneration is idempotent for an unchanged ruleset and
vocabulary.

## Shell completion

`arclint completion bash|zsh|fish|powershell` emits the shell script.
Completion uses the resolved `rules.arclint.yaml` when available: Rule IDs for
`rules` and the `check --only` / `--exclude` selectors, Module names for
`context --module`, supported languages for `init --languages` and
`patterns install --languages`, `bare` plus every visible Pattern
reference for `init --pattern`, every offline Pattern reference for the
`patterns install`, `vendor`, and `export` argument, and the closed
`human` / `json` output-format values. Without a loadable
repository configuration, repository-derived candidates stay empty.
Command aliases complete alongside canonical names (`agents mar<TAB>`
offers `markdown`, `domain r<TAB>` offers both `remove` and `rm`), each
carrying its command's description.

## JSON diagnostics

`arclint check --format json` emits an array containing every Diagnostic
in the complete Conformance Assessment. The stable shape is:

```json
[
  {
    "kind": "violation",
    "ruleId": "arclint/vertical:dependencies/inward",
    "pattern": "arclint/vertical@0.1.0",
    "path": "internal/orders/application/create_order.go",
    "line": 3,
    "severity": "error",
    "status": "active",
    "message": "import resolves to Module \"infra\", not allowed by Module \"application\"",
    "remediation": "depend on the inward-owned port"
  }
]
```

`kind` is `violation`, `operational`, or `coverage`. Fields that do not
apply are omitted: coverage Diagnostics have no Severity, operational
Diagnostics may have no Rule ID, and only Violation Diagnostics have a
status. `ruleId` is the qualified id, and `pattern` names the Pattern
that distributed the Rule; a local Rule has an unqualified `ruleId` and
no `pattern`. `line` is omitted when the Diagnostic is not line-anchored, and
`remediation` is omitted when none was supplied.

The default human output prints active Violations and operational or
coverage Diagnostics, followed by counts for active, suppressed, and
baselined findings and applied Rules. Suppressed and baselined
Violations remain part of the Assessment and JSON output but do not
appear as active findings.

## Schema output

`arclint rules schema` and `arclint domain schema` always emit indented
JSON. Every configuration file and schema follows one naming pattern:
configuration is `<name>.arclint.yaml` at the project root
(`rules.arclint.yaml`, `domain.arclint.yaml`), and its schema is
`<name>.arclint.schema.json`. `--write` puts the schema under
`.arclint/schemas/` (or the directory named by `--dir`) and reports
whether the copy changed, so a project can keep a local copy for its
editor modelines:

```yaml
# yaml-language-server: $schema=.arclint/schemas/rules.arclint.schema.json
```

arclint publishes its own release copies as
`docs/schemas/rules.arclint.schema.json` and
`docs/schemas/domain.arclint.schema.json`; each schema's `$id` is the
raw GitHub URL of that copy on `main`, and `arclint init` and
`arclint domain init` write that URL into a fresh file's modeline until
a local copy exists. `make schemas` regenerates every committed copy,
and drift tests keep the command output and the committed files
byte-for-byte identical.

## Project domain model

`arclint domain` inspects and maintains the project's Ubiquitous Language:
the shared language used by developers, domain experts, documentation,
tests, and code. ArcLint records that language as a structured Project
Domain Model in a committed `domain.arclint.yaml` beside the
resolved `rules.arclint.yaml`. ArcLint does not search for the model
independently; `--rules <path>` moves both files' project root together.

The file is organized by bounded context:

```yaml
version: 1
contexts:
  - name: billing
    entities:
      - name: Invoice
        definition: ...
        aggregate: true
        aliases: [bill]
    value_objects:
      - name: Money
        definition: ...
    invariants:
      - statement: An Invoice total is non-negative
        owner: Invoice
    assertions:
      - statement: "A processed Invoice marks payment received"
        owner: "Invoice"
        id: "payment-received"
        on: "Process"
    specifications:
      - name: "PaidInvoice"
        definition: "An invoice fully paid."
    events:
      - name: InvoiceIssued
        definition: ...
relations:
  - from: billing
    to: catalog
    kind: customer_supplier
```

Relation `kind` is one of `partnership`, `shared_kernel`,
`customer_supplier`, `conformist`, `anticorruption_layer`,
`open_host_service`, `published_language`, or `separate_ways`. Concept
spellings use underscores (`entity`, `value_object`, `invariant`,
`assertion`, `specification`, `aggregate`, `aggregate_root`, `domain_event`,
`bounded_context`, `business_rule`). An aggregate is a designation on an
entity (`aggregate: true`), not a separate stored object.
`business_rule` records as an invariant. `assertion` and `specification` record in their own separate collections. Defining a term targets a bounded context.

Fresh files get a YAML language-server modeline pointing at
`.arclint/schemas/domain.arclint.schema.json` when that path exists
under the project root, otherwise the schema's `$id`, the raw GitHub URL
of `docs/schemas/domain.arclint.schema.json` on `main`.

Running `arclint domain` with no subcommand is the same as
`arclint domain overview`. Subcommands cover file initialization
(`init`), summary (`overview`), inventory (`list`), one definition
(`show`), ArcLint-owned concept meanings (`explain`), create-or-update
(`define`, including `--guided`), removal (`remove` / `rm`), and the
published schema (`schema`). `domain init` is idempotent: it creates a
versioned, schema-hinted empty file when none exists and never replaces
an existing model.

The process-level `--format human|json` option applies to domain reports
as well as the other semantic command outputs. JSON is the stable machine
shape for agents and scripts. `domain schema` always emits indented JSON,
the same contract the binary writes to
`.arclint/schemas/domain.arclint.schema.json` with `--write` (also
written by `arclint agents skill`) and publishes as
`docs/schemas/domain.arclint.schema.json`.

Domain command exit codes:

| code | meaning |
|---|---|
| `0` | operation succeeded, including an unchanged `define` |
| `1` | operation could not be completed (missing definition, malformed model, write failure) |
| `2` | invalid command usage (unknown type, missing name, mutually exclusive flags, bad `--format`) |

Declaring knowledge never creates a Diagnostic by itself. Domain quality
findings remain the responsibility of enabled Rules under
`arclint check`. `arclint context` surfaces the recorded model when
present, scoped to what anchors into the worksite; focused path
relevance stays with `context`, not `domain`. `arclint domain` prints
the whole model with every contract's `source:` line: the carrying
declaration, `missing`, or `unanchorable` with the reason.

## Agent skill artifacts

`arclint agents skill` materializes the domain-librarian skill bundle the
binary owns: `SKILL.md` and `VOCAB.yaml` under `--dir` (default
`.agents/skills/domain-librarian/`), plus the domain schema the
vocabulary points at, which always lands at
`.arclint/schemas/domain.arclint.schema.json` under the project root.
Those committed files are generated outputs (and litmus fixtures the
generator must reproduce).
`arclint agents md` (aliases `markdown`, `agentsmd`) prints or installs
the AGENTS.md architecture block; it does not write skill artifacts.
