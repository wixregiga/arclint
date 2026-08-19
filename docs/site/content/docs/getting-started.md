+++
title = "Getting started"
description = "Install, init, check: contracts enforced in two minutes."
weight = 1
+++

## Install

arclint is one static binary (CGO disabled). Build it from source with
Go:

```bash
git clone https://github.com/wixregiga/arclint && cd arclint
make build   # produces ./arclint
```

Or run it as a container. The image is built from `scratch` (the binary
is self-contained; the TypeScript extension runtime is in-process), and
the default command checks the mounted repository:

```bash
make docker                                  # builds arclint:<version>
docker run --rm -v "$PWD":/repo arclint:0.1.0
```

`VERSION` defaults to `0.1.0` in the Makefile; tag the run with the same
value you built.

## Initialize a repository

```bash
cd ~/projects/your-repo
arclint init
```

`init` drafts a commented starter `rules.yaml` from explicit language
choices. It does not scan the tree and does not select a Pattern.
Default languages is `go`. Pass others with `--languages`:

```bash
arclint init --languages go,ts
```

Accepted values are `go`, `ts`, and `py` (comma-separated). An existing
`rules.yaml` is refused unless you pass `--force`.

On success:

```
wrote /path/to/rules.yaml
next: declare your modules, then run `arclint check .`
```

The draft sets `runtime`, one Module `source` covering `**`, and one
vacuously satisfied `consumes` Rule so the file loads and `check` stays
clean until you split Modules:

```yaml
# ArcLint architecture contracts.
# Grow this file module by module: declare real Modules under `modules`,
# then state what each may import under `contracts`.
# Query commands: arclint rules [selector] · arclint context <path>

runtime: [go]

scan:
  # error | warn | ignore for imports that classify neither stdlib,
  # internal, nor declared in the dependency manifest.
  unknown_imports: warn

modules:
  source:
    paths: ["**"]
    description: "Every file. Split into real modules as the architecture takes shape."

contracts:
  source:
    consumes:
      id: "repo:source/dependencies"
      # An allow-list of other declared modules. Empty means this module
      # may import no other declared module; with one module this is
      # vacuously true — it starts binding the moment you split modules.
      internal: []
```

When you add TypeScript Extensions later, `arclint sdk init` writes
`arclint.d.ts` and `tsconfig.json` under `.arclint/extensions`.

## Check

```bash
arclint check .
arclint check . --format json   # Diagnostic array for CI
```

Human output lists active Violations and operational or coverage
Diagnostics, then a summary line:

```
0 active finding(s) · 0 suppressed · 0 baselined · 1 rule(s) applied
```

Exit codes: `0` clean, `1` when the gate fails (an active error-severity
Violation or an error-severity operational Diagnostic), `2`
configuration or usage error.

Optional flags: `--only` / `--exclude` (Rule ids, prefixes, or patterns)
and `--no-baseline` to evaluate without subtracting the committed
baseline.

## Adopt an existing repository

A repo with history usually has Violations on day one. Capture them
instead of fixing everything first:

```bash
arclint baseline capture   # writes .arclint/baseline.v2.json
arclint check .
```

`baseline capture` reports how many findings it adopted, for example
`baseline captured: 1 finding(s) across 1 applied rule(s)`. After
capture, a clean human summary looks like
`0 active finding(s) · 0 suppressed · 1 baselined · 1 rule(s) applied`.

Commit the baseline file. From then on only new findings fail the run;
baselined findings stay counted. When adopted findings disappear, check
prints a coverage note so you can shrink the snapshot:

```
coverage: baseline: 1 adopted finding(s) no longer occur; refresh the baseline
```

```bash
arclint baseline refresh   # drops stale entries after comparison
```

## Adjust the modules

Module boundaries are yours. Edit the `modules:` block in `rules.yaml`,
then inspect what binds where:

```bash
arclint context                  # repository scope
arclint context path/to/file.go  # everything binding a path
arclint context --module source  # one declared Module
```

`context` reports scope, languages, configured Rule count, each Module's
paths and import allow-list, and the Rules that apply. Re-run
`arclint check .` when the Modules look right.

## Inspect Rules and schema

```bash
arclint rules                          # one line per configured Rule
arclint rules repo:source/dependencies # full detail for one Rule
arclint rules schema                   # JSON Schema for rules.yaml
```

For editor completion, point a YAML language server at the committed
schema (kept identical to `arclint rules schema` by a drift test):

```yaml
# yaml-language-server: $schema=https://raw.githubusercontent.com/wixregiga/arclint/main/docs/rules.schema.json
```

If that URL does not resolve yet, use `docs/rules.schema.json` from an
arclint checkout, or the live output of `arclint rules schema`.
