---
name: arclint-rule-creator
description: Translate a plain-language architectural constraint ("every service needs a README", "components can't import app layer", "test files end in _test.go") into a valid arclint rule appended to .arclint/rules.yaml. Use when the user asks to add, create, or write an arclint rule, or states an architecture/naming/dependency/content convention they want enforced.
---

# arclint Rule Creator

Turn a natural-language constraint into one rule in `.arclint/rules.yaml`. Ground truth: `docs/design/rules.md` and `schema/arclint-rules.schema.json` (read them when a shape below is not enough).

## Workflow

1. **Categorize** via the decision table. Category is a CLOSED enum: `structure | naming | dependencies | content | custom`. Never invent a new category or top-level rule field — everything must stay statically parseable (a future VS Code extension consumes this file with only a YAML parser + the JSON schema).
2. **Pick id**: kebab-case, short imperative statement of the constraint (`no-utils-dir`, `require-service-readme`, `domain-stays-pure`). Grep `.arclint/rules.yaml` for the id first — ids are stable references (baselines, ignores); never duplicate, never rename existing ones.
3. **Write rule** using the exact shape for the category (below). If `.arclint/rules.yaml` does not exist, create it with `version: 1` and a `rules:` map. Otherwise append under the existing `rules:` map, preserving all existing content untouched.
4. **Validate**: immediately run `arclint check` after appending. If it errors, the loader names the offending rule id — fix that rule's shape and re-run until clean. Then eyeball the flagged findings: do they match the constraint's intent (right files, right message)? If a JSON-schema validator is available, also check against `schema/arclint-rules.schema.json`.

## Decision table

| Constraint phrasing | Category | Params |
|---|---|---|
| "X must exist", "repo must have Y" (repo-wide, one match anywhere) | structure | `require: [globs]` |
| "no X directory/file allowed", "Y is forbidden" | structure | `forbid: [globs]` |
| "every <dir> must contain X" (per-directory, each instance checked) | custom | see Custom recipe below — `structure.require` is repo-wide and will NOT catch a missing file in one of many dirs |
| "files must be snake_case", "must end in _test", "dirs lowercase" | naming | `style` (+ `target: dir` for dirs), scope via `files.include` |
| "A can't import B", "layers", "features independent", "X may only depend on Y" | dependencies | `modules` + `contract: layers\|forbidden\|independence\|mayDependOn` + matching key |
| "files must/must not contain <text/pattern>" | content | `mustContain` / `mustNotContain` matchers |
| "run our script/tool to check" | custom | `command: [argv]` (+ `timeoutSeconds`) |

Suffix conventions ("_test.go") are naming rules only if every targeted file must carry the suffix — scope `files.include` tightly; to ban a pattern instead, use structure `forbid`.

## Rule shape (common fields)

```yaml
rules:
  <kebab-id>:
    type: <category>          # required
    severity: error           # required: error | warn | "off" (quote off!)
    description: One sentence of intent.   # required
    files: { include: ["internal/**"], exclude: ["**/*_test.go"] }  # optional targeting
    fixHint: What to do about it.          # optional
    params: { ... }           # required, shape per category below
```

Severity: `error` = hard invariant, fails run (exit 1). `warn` = convention, migration in progress, advisory — printed, exit 0. `"off"` = id reserved, not run. New rules: `error` when violation is unambiguous and tree is clean (or baselined); otherwise start `warn`.

## Params per category

structure — `require` checked repo-wide (files block ignored); `forbid` respects `files`:
```yaml
    params:
      require: [".github/workflows/*.yml", "README.md"]   # each must match >=1 file
      # or
      forbid: ["**/utils/**"]                              # must match 0 files
```

naming — ls-lint pipe: `camelCase|PascalCase|snake_case|kebab-case|SCREAMING_SNAKE_CASE|lowercase|regex:<p>` (regex alternative may not contain `|`). Checks basename, extension stripped; `target: dir` for directories:
```yaml
    params:
      style: "lowercase | kebab-case | regex:v[0-9]+"
      target: dir            # default: file
```

dependencies — named modules (globs), one contract. `layers`: ordered top→bottom, layer may import below never above. `forbidden`: deny edges. `independence`: listed modules never import each other. `mayDependOn`: whitelist (empty list = depends on nothing):
```yaml
    params:
      modules:
        app:    ["internal/app/**"]
        domain: ["internal/domain/**"]
      contract: layers
      layers: [app, domain]
      # forbidden: [{from: [domain], to: [app]}]   # contract: forbidden
      # among: [billing, search]                    # contract: independence
      # mayDependOn: {app: [domain]}                # contract: mayDependOn
```

content — line-oriented RE2 regexes. `mustNotContain`: no targeted file may match. `mustContain`: every targeted file must match each pattern at least once:
```yaml
    params:
      mustNotContain:
        - pattern: 'fmt\.Print(ln|f)?\('
          message: Use slog instead.
```

custom — external argv from repo root; gets `{"files": [...]}` on stdin, prints JSON array of `{path, message, fixHint?}`; exit 0 only on success. Editor tooling skips custom rules:
```yaml
    params:
      command: ["scripts/check-openapi.sh"]
      timeoutSeconds: 60     # default 30, max 600
```

### Custom recipe: "every service dir must contain X"

`scripts/check-service-readme.sh` (reads stdin, writes stdout, no argv parsing needed):
```sh
#!/bin/sh
python3 - <<'PY'
import json, sys, os
files = json.load(sys.stdin)["files"]
dirs = {os.path.dirname(f) for f in files if f.startswith("services/")}
out = []
for d in sorted(dirs):
    if not os.path.exists(f"{d}/README.md"):
        out.append({"path": d, "message": "missing README.md", "fixHint": f"add {d}/README.md"})
print(json.dumps(out))
PY
```
Matching rule:
```yaml
rules:
  require-service-readme:
    type: custom
    severity: error
    description: Every services/* dir must contain a README.md.
    params: { command: ["scripts/check-service-readme.sh"] }
```

## Hard constraints

- Rule object allows ONLY: `type, severity, description, files, fixHint, params` (`additionalProperties: false`). Params blocks are closed per category too.
- File top level requires `version: 1` and the `rules` map.
- YAML 1.1 gotcha: always write `severity: "off"` quoted.
- One convention per rule: different trees needing different styles = two rules, two ids.
