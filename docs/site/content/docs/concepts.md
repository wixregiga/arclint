+++
title = "Concepts"
description = "Modules, contracts, blame, and what internal/external/stdlib mean."
weight = 2
+++

## Modules

A module is a named set of files, defined by path globs in rules.yaml.
Modules are the vocabulary of every other rule: contracts, layers, and
protections refer to module names, never raw paths.

```yaml
modules:
  entities:
    paths: ["internal/entities/**"]
    description: "Domain types and invariants; depends on nothing."
  features: ["internal/features/*"]   # terse form
```

A glob matches files directly, and a glob naming a directory owns the
whole subtree. Overlapping modules are legal: a file can belong to
several modules, which makes umbrella modules (`src: ["internal/**"]`)
cheap for repo-wide invariants.

## Import classes

Every import in every scanned file is classified before any rule runs.
The class names appear throughout the rule surface, so their exact
meaning matters:

| class | meaning |
|---|---|
| `internal` | resolves to a file inside this repository: another module, or undeclared internal code |
| `external` | a third-party dependency declared in your manifest: `go.mod` require, `package.json` dependencies, `pyproject.toml` |
| `stdlib` | the language's standard library (embedded tables generated from each toolchain) |
| `unknown` | none of the above; governed by `scan.unknown_imports: warn/error/ignore` |

So in a contract, `internal: [app]` means "may import the app module and
nothing else internal", and `external: forbid` means "no third-party
libraries here at all". Go classification is exact and proven against
`go list` over pinned real repositories. TypeScript and Python are
lexer-grade with documented limits: computed specifiers like
`import(x)` or `importlib.import_module(name)` are invisible by design.

## Contracts

A module's contract has three clause kinds, borrowed from design by
contract:

- **consumes** (preconditions): what the module may depend on. Per-module
  allow and deny lists plus third-party and stdlib policy. Graph-wide
  clauses live under top-level `dependencies:` because they span modules:
  `layers`, `forbidden`, `independence`, `protected`, `acyclic`.
- **provides** (postconditions): what the module must supply.
  `registration` says every instance of a shape registers itself
  somewhere; `correspondence` says a value set derived from one side of
  the tree must exist on the other side.
- **invariants**: properties that always hold. Naming conventions,
  required and forbidden paths, content rules, and `expr` predicates
  type-checked at load time.

Run `arclint explain <kind>` for any of these; the
[rule reference](/docs/rules/) is the same text.

## Blame

Every violation carries a blame side, and it is checkable output, not
vocabulary:

- a **consumes** break blames the **consumer**: the importing file broke
  its own precondition.
- a **provides** break blames the **provider**: the module failed a
  promise it made to the rest of the repository.
- invariants blame the module that holds the property.

TypeScript extension rules declare a default contract and blame once and
may override both per finding, so a rule that checks two sides of one
contract labels each finding truthfully.

## Capability labels

Every rule type states how it enforces its claim, and every finding
carries the label:

| label | basis |
|---|---|
| `exact` | the classified import graph or parsed syntax facts |
| `structural` | paths, shapes, and declaration placement |
| `heuristic` | names, regexes over text, or complexity signals |
| `advisory` | guidance; reports without claiming proof |

Builtin dependency rules are `exact`; naming and structure rules are
`structural`; content regexes are `heuristic`. Extensions declare their
own tier in `defineRule` and default to `heuristic`, the conservative
claim. The label prevents false confidence: a rule that matches names
cannot present its findings as proven semantics.

## Rule identity

Rule ids are stable strings, and several clauses may share one explicit
id to form one requirement (a layering rule plus a protected rule both
carrying `ddd:ARCH-002`, for example). Patterns prefix their rule ids
with a short namespace (`slice:`, `layers:`), so `arclint rules show
slice` lists a pattern's whole rule set and `arclint rules test
--pattern` proves it against fixtures.

## Exceptions

Sometimes a rule is right and one file is still allowed to break it.
Every clause kind accepts an `except` list; a finding is suppressed
when its anchor path matches, and the rule keeps firing everywhere
else:

```yaml
contracts:
  domain:
    consumes:
      internal: []
      external: forbid
      except:
        - paths: ["internal/reports/bridge.go"]
          reason: "grandfathered direct DB access; remove with the reports rewrite"
```

The globs use the same doublestar dialect as module paths, the shape is
identical on `dependencies` rules, invariants, and extension instances,
and `reason` is required: an exception is policy, and the YAML is its
audit trail. Suppressed findings are counted in check output
(`2 suppressed by except`), never silently dropped. `arclint explain
except` has the full story.

## Validation layers

rules.yaml passes three gates before anything runs: YAML syntax, the
published JSON Schema (the same file that powers editor completion), and
semantic validation (module references, regex compilation, expr type
checking). Extension rule params are validated against each extension's
declared schema before a line of extension code executes.
