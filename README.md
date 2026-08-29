# arclint

A repository linter and architectural-conformance system.

**Problem:** llm code is super long and annoying to read, and after a
few updates over a couple of weeks it's unmaintainable. You end up
asking yourself, "what does this thing even do?"

Current solutions:

- Throw your hands up
- Give up
- Read a million lines of code a day
- Pray

A good senior dev is unreasonable and lazy. They want the codebase a
certain way, and they know not everyone shares their passion for
vertically sliced, GoF, DDD, dependency inverted, Dijkstra approved,
dodecahedron architectural patterns. Especially not the agents.

So don't hope everyone's on the same page. Check for it. Throw an error
when it's not up to your standards. And when you find a pattern worth
keeping, write a rule for it.

Use arclint for that check. Or not. It's up to you.

- Already use linters? Sick. This is one. Config it up in `rules.yaml`.
- Got weird house rules? Dope. Write a small TypeScript extension. No
  npm, no Node, no toolchain to install; the binary transpiles and
  sandboxes it.
- Want to know which rules govern a file or directory? Tubular. Run
  `arclint context <path>`.
- DDD crazy? Rad. Keep your terms in a committed
  `ubiquitous-language.yaml`, then write rules like "each aggregate
  lives in `internal/<snake_case>/`, imports no third-party code, and
  declares a repository interface."
- Want vertically sliced hexagons? Write a rule for it.
- Want features to live in a certain file? Write a rule for it.
- Want to test your rule out first? Write a test for it in
  `.arclint/tests/name-features-the-way-i-want.yaml` and run
  `arclint rules test`.
- It does other things too. They're below.

## Install

Beta releases currently provide static Linux binaries for amd64 and arm64.
Choose a version from [GitHub Releases](https://github.com/wixregiga/arclint/releases),
then download and verify the archive for the current machine:

```bash
version="0.1.0-beta.1"
case "$(uname -m)" in
  x86_64) arch="amd64" ;;
  aarch64 | arm64) arch="arm64" ;;
  *) echo "unsupported architecture: $(uname -m)" >&2; exit 1 ;;
esac

base="https://github.com/wixregiga/arclint/releases/download/v${version}"
archive="arclint_${version}_linux_${arch}.tar.gz"
curl -fLO "${base}/${archive}"
curl -fLO "${base}/checksums.txt"
sha256sum --ignore-missing -c checksums.txt

mkdir -p "$HOME/.local/bin"
tar -xzf "$archive"
install -m 0755 arclint "$HOME/.local/bin/arclint"
rm arclint "$archive" checksums.txt
arclint --version
```

`$HOME/.local/bin` must be on `PATH`. Beta releases are prereleases and may
change before arclint reaches a stable version.

## Quickstart

```bash
arclint init       # draft a commented starter rules.yaml
arclint check .    # evaluate it
```

Grow the starter module by module. A real contract set looks like:

```yaml
runtime: [go]

modules:
  domain:
    paths: ["internal/domain/**"]
    description: "Aggregates and domain values; stdlib-only."
  application:
    paths: ["internal/application/**"]
    description: "Use cases coordinating domain objects through ports."

contracts:
  domain:
    consumes:
      id: "repo:domain/stdlib-only"
      internal: []        # may import no other declared module
      external: forbid
  application:
    consumes:
      id: "repo:application/inward"
      internal: [domain]
      external: forbid

dependencies:
  - id: "repo:dependencies/acyclic"
    kind: acyclic
```

## Rule types

The finite, arclint-owned set — extensions plug into it, they never
extend it:

| kind | claim shape |
|---|---|
| `consumes` | what a module may import: internal allow-list, external and stdlib policy |
| `structure` | files a module must contain or must not contain (globs) |
| `naming` | file-name case vocabulary (`snake_case`, `kebab-case`, `camelCase`, `PascalCase`, `regex:`) |
| `layers` | modules ordered highest first; imports go same-or-lower only |
| `protected` | who may import one module |
| `acyclic` | no dependency cycles among declared modules |
| `extension` | enforcement supplied by a TypeScript extension via `uses` + `with` |

Import analysis is exact, never heuristic: Go classification follows
the toolchain (embedded `go list std` table, module-path ownership,
replace directives, go.work), TypeScript and Python follow their
manifests, and each shipped language adapter embeds a generated stdlib
table — an obligation the ruleset itself enforces.

## Authoring rules

The loop is schema-guided, trialed live, then pinned:

1. **Edit with the schema.** [docs/rules.schema.json](docs/rules.schema.json)
   describes the complete accepted rules.yaml grammar — also printed by
   `arclint rules schema`, and drift-tested so runtime validation and
   the published schema accept the same documents. Point your editor at
   it:

   ```yaml
   # yaml-language-server: $schema=https://raw.githubusercontent.com/wixregiga/arclint/main/docs/rules.schema.json
   ```

2. **Trial it on the real repository.** `arclint check --only <id>`
   evaluates just that rule; patterns like `arclint:domain/*` work,
   and observation narrows to the facts the selected rules declare.

3. **Write a test for it.** A rule test is one YAML file under
   `.arclint/tests/`: a set of inline example files plus the complete
   findings you expect the rule to produce on them, run through the
   real parsers by `arclint rules test`. Start with an empty
   `expect: []` — failures print ready-to-paste entries; adopt the
   intended ones. An empty list that stays empty asserts complete
   conformance. This repository's own tests under
   [.arclint/tests](.arclint/tests) were authored exactly this way:

   ```yaml
   # .arclint/tests/sole-aggregate-forbids-second.yaml
   rule: "arclint:domain/rule-is-sole-aggregate"
   files:
     internal/domain/rule/root.go: |
       package rule
     internal/domain/pattern/pattern.go: |
       package pattern
   expect:
     - kind: violation
       path: internal/domain/pattern/pattern.go
       message: "path forbidden by structure rule \"internal/domain/pattern/**\" of Module \"domain\""
   ```

## Extensions

Custom enforcement is TypeScript under `.arclint/extensions/`,
transpiled in-process by esbuild and executed on a deterministic
sandbox (sobek): no npm, no Node, no filesystem or network access, and
host-controlled clock and randomness. Parameters are validated
host-side against the schema the extension publishes, before any
extension code runs. The complete showcase lives in this repository —
[.arclint/extensions/forbid_content.ts](.arclint/extensions/forbid_content.ts)
enforces `arclint:domain/no-panic` from rules.yaml:

```yaml
- id: "arclint:domain/no-panic"
  kind: extension
  files: "internal/domain/**/*.go"
  uses: forbid-content
  with:
    pattern: '\bpanic\('
```

Extensions see exactly the subjects the rule's applicability selects;
findings outside that scope are rejected. The capability surface is
one interface for every language: `ctx.files`, `ctx.read`,
`ctx.imports`, and `ctx.facts` — the same normalized shapes for Go,
TypeScript, and Python. Declarations use a closed cross-language
vocabulary: go/parser exact for Go, pinned tree-sitter grammars for
TypeScript and Python (embedded per the Makefile's grammar-subset
tags), deterministic everywhere. Extension evidence is treated as heuristic:
findings gate as suspected violations, and absence of findings is
undetermined, never proof of conformance.

## Working with agents

Three commands do the agent-facing work:

- `arclint context <paths...>` — give it the files an agent touched
  (and/or `--module <names>`) and one payload answers what governs the
  set: each path mapped to its owning modules, each involved module
  once, and the union of applicable rules with the scope parts that
  pulled them in, boundary rules included. Bare `arclint context`
  explains the repository instead: every module and its import policy,
  the rule kinds in use with their meanings, and the unknown-imports
  posture.
- `arclint agents --write` — generates the AGENTS.md block from the
  ruleset (this repository's [AGENTS.md](AGENTS.md) is produced this
  way), so the agent-facing context cannot drift from the enforced
  contract. `arclint agents skill` emits a skill bundle.
- `arclint domain` — inspects and maintains the project's ubiquitous
  language in a committed `ubiquitous-language.yaml`: bounded contexts,
  entities, aggregates, value objects, invariants, and events, with a
  published JSON Schema (`arclint domain schema`).

## Outcomes and exit codes

A good linter answers some basic questions about its rules:

- Can I ignore this? Severity is configured per rule and decides
  whether a finding gates: `error`, `warning`, or `info`.
- Is this a false positive? Every rule type states its assurance.
  Builtin import and tree rules are `exact`. Extension findings are
  always treated as `heuristic` and gate as suspected violations.
- Does it check what it says it checks? Every evaluation ends in
  exactly one outcome: `conforms`, `violates`, `suspected_violation`,
  `undetermined`, `unsupported`, `not_applicable`, or `failed`. What
  wasn't checked shows up as `undetermined` or `unsupported` instead
  of passing quietly.

These questions are answered on every run, for every rule. Severity
and assurance are independent.

Exit codes: `0` clean, `1` error-severity findings, `2` configuration
or usage error.

## Baseline

Adopted debt lives in a committed, reviewable baseline
(`.arclint/baseline.v2.json`): `baseline capture` adopts current
findings, `check` reports only new ones and counts the covered rest,
`baseline refresh` drops entries that no longer occur.

## Commands

```
check [path]        evaluate the repository (--format human|json, --no-baseline, --only/--exclude <selectors>)
rules [selector]    list the configured rules; one match shows the complete rule
rules schema        print the JSON Schema for rules.yaml (committed at docs/rules.schema.json)
rules test [name]   run the rule tests under .arclint/tests; failures exit 1
context [paths...]  the architecture, or everything binding the given paths (--module, --format json)
domain              inspect and maintain the project's ubiquitous language (init/overview/list/show/explain/define/remove/schema)
agents              AGENTS.md block (--write); skill bundle (skill); SKILL.md only (md|agentmd|markdown)
baseline capture    adopt current findings   ·  baseline refresh: drop stale entries
patterns            list local pattern packages (.arclint/patterns/<name>/pattern.yaml)
sdk init            write arclint.d.ts + tsconfig.json for extension authors
init                draft a starter rules.yaml (--languages go,ts,py --force)
completion <shell>  shell completion with live rule ids and module names (bash|zsh|fish|powershell)
```

A selector is an exact rule id, an id prefix, or a `path.Match`
pattern (`arclint:domain/*`); an exact id wins over expansion, and a
selector matching nothing is a loud error, never a silent no-op.
`--only` and `--exclude` take several, comma or space separated, with
exclusion winning.

## Developing

```bash
make ci                  # format + lint + vet + tests + selfcheck
go test -short ./...     # the quick loop, network suite skipped
make bench               # cold start and large-repo timings
```

This repository is checked by its own ruleset on every CI run
(`make selfcheck`), so the architecture below is enforced, not just
described. The codebase follows the rules it ships:

- `internal/domain` — the Rule aggregate, conformance check, and
  baseline model; stdlib-only, invalid values unconstructible.
- `internal/application` — action-named use cases behind ports.
- `internal/infrastructure` — outbound adapters: ruleset loading,
  observation, language facts, baselines, patterns, the extension
  runtime.
- `internal/delivery` — the framework-neutral CLI; Cobra is sealed
  inside one adapter behind a factory.
- `cmd/arclint` — the composition root.
- Verification lives beside what it verifies: the Go adapter carries
  a toolchain suite proving classification against `go list` over
  pinned real repositories (part of the normal test run, skipped under
  `-short`), and the domain glob matcher carries a committed reference
  truth table.

Product documentation lives under [docs/site](docs/site). The generated
[AGENTS.md](AGENTS.md) carries the architecture contracts for agents.
