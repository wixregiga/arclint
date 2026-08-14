+++
title = "Getting started"
description = "Install, init, check: contracts enforced in two minutes."
weight = 1
+++

## Install

arclint is one static binary (CGO disabled, ~20 MB). Build it from
source with Go:

```bash
git clone https://github.com/wixregiga/arclint && cd arclint
make build   # produces ./arclint
```

Or run it as a container: the image is built from `scratch` (the
binary is self-contained, TypeScript extensions included), and the
default command checks the mounted repository:

```bash
make docker                                  # builds arclint:<version>
docker run --rm -v "$PWD":/repo arclint:0.1.0
```

## Initialize a repository

```bash
cd ~/projects/your-repo
arclint init
```

`init` detects the languages in your tree, asks which to analyze and
which architectural pattern to start from, then writes everything and
validates it:

```text
detected: go (142 files), ts (13 files)
runtimes to analyze [go,ts]:
patterns for go,ts:
  1. layers           Hexagonal layering: cmd composes, app orchestrates, ...
  2. starter          Minimal foundation: one module over the whole tree, ...
pattern [1]:
wrote rules.yaml
pattern "layers" ready for go, ts.
```

Non-interactive: `arclint init --runtimes go --pattern feature-slice`.
Patterns that ship TypeScript extensions also get
`.arclint/extensions/*.ts` and editor typings (`arclint.d.ts`,
`tsconfig.json`) written for free.

## Check

```bash
arclint check .
arclint check . --format json   # stable shape for CI
```

Exit codes: `0` clean, `1` at least one error-severity violation, `2`
configuration or usage error.

## Adopt an existing repository

A repo with history usually has violations on day one. Freeze them
instead of fixing everything first:

```bash
arclint baseline    # records current findings in .arclint/baseline.json
arclint check .     # clean: 0 violations · 12 baselined
```

Commit the baseline file. From now on only new findings fail the run,
the adopted count stays visible, and check warns when adopted findings
disappear so the file shrinks as debt is paid. The
[concepts page](/docs/concepts/#baseline) has the full story.

## Adjust the modules

Patterns ship sensible globs, but module boundaries are yours:

```bash
arclint module ls          # what each module matched, with file counts
arclint module info app    # one module: description, members, rules
```

Edit the `modules:` block in rules.yaml until `module ls` reflects your
tree, then run `arclint check .` again.

## Learn the vocabulary

```bash
arclint explain            # every rule kind, one line each
arclint explain consumes   # full explanation plus a paste-ready example
```

The same text powers editor hovers through the published JSON Schema.
Add this line at the top of rules.yaml for completion in editors with a
YAML language server:

```yaml
# yaml-language-server: $schema=https://raw.githubusercontent.com/wixregiga/arclint/main/docs/rules.schema.json
```

Until the repository is public that URL does not resolve; point at a
local copy instead (`docs/rules.schema.json` in the arclint checkout).
A drift test keeps the committed schema current, so a local copy is
never stale relative to its checkout.
