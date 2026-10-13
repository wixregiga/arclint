---
name: domain-sweep
description: Inventory every must, must not, never, always, only, refused or rejected statement a change adds or touches, and place each through the domain-librarian's invariant gate as an invariant, assertion, specification, definition change, or no entry with the protocol rule cited. Use when a branch or pull request wrote code before recording meaning (AGENTS.md workflow step 3), when issue #94's domain process is requested for a change, or before recording a change's rules with arclint domain define. Read-only by default; mode record writes only through arclint.
---

# Domain sweep

You run arclint's domain process over one change: find every statement of what must or never happen, decide where each belongs with the domain-librarian's gate, say where code enforces it today, and hand the owner the entries, the code moves and the open questions. The domain-librarian skill records entries; this skill decides what there is to record and proves each decision with evidence.

## Parameters

Pass them as `key=value` in the invocation argument or in plain words. Each has a default.

| parameter | default | meaning |
|---|---|---|
| `base` | `main` | The ref the change is measured against. The change is the working tree against this ref, committed and uncommitted work together, plus untracked files. |
| `scope` | `both` | `added`: lines the change adds. `touched`: every line of a file the change touches. `both`: added lines first, existing lines of touched files for statements whose meaning the change moved or made enforceable. |
| `paths` | `docs/site/content/docs/*.md README.md CONTRIBUTING.md AGENTS.md *.go` | Where statements live: docs, CLI help text, Go doc comments, test names and assertions. |
| `candidates` | none | Statements the caller wants covered. Each becomes a row even when the answer is no entry. |
| `evidence` | none | Issue and pull request numbers whose text counts as the owner's words, for example `#95 #97 #98`. |
| `mode` | `report` | `report`: read-only, the reply is the output. `record`: also record undisputed entries through `arclint domain define` and `arclint domain remove`, then run `arclint check .`. |
| `arclint` | build | Path to the arclint to use. The default builds a fresh one with `go build -o /tmp/arclint-sweep ./cmd/arclint`, because an installed arclint may be older than the branch. |

## Procedure

Say in one line what you are starting before each step. Batch every independent read of a step into one response.

1. **Orient.** Build arclint. List the touched files once with `python3 -I <skill dir>/scripts/inventory.py --base <base> --files` and run `arclint context` on that list before opening any of them. Run `arclint domain` for the recorded language. Read `.agents/skills/domain-librarian/SKILL.md` for the numbered protocol and the `distillation_rules` ids in `.agents/skills/domain-librarian/VOCAB.yaml`. Read `references/gate.md` in this skill. Note `git status --short` so you can tell at the end whether the worktree changed under you.
2. **Owner's words.** For each number in `evidence`, fetch the body and comments once (`gh pr view N --json title,body`, `gh issue view N --json title,body,comments`) into a file under `/tmp/domain-sweep/`. Only the recorded language, the owner's docs and the owner's words in issues and pull requests classify a statement. Recordings, tests, code and assistant text are claims to examine.
3. **Inventory.** Run the script once:

   ```bash
   python3 -I <skill dir>/scripts/inventory.py --base <base> --scope <scope> --paths <paths> --out /tmp/domain-sweep/<branch>.tsv
   ```

   Each row is `kind match file:line text`. Rules such as discovery orders and defaults are often worded without a keyword, so the script lists every added line of prose whole (Markdown, Go comments, and Go raw strings, where the CLI help text lives) as `prose`, and lines of Go code only on a keyword or a rule-like test name, as `keyword`. Read every `prose` row, the `keyword` rows of Go code filtered by file, `existing` rows only for files whose added rows moved a meaning, and every `deleted` row, because a statement the change removes may need its entry removed with `arclint domain remove`. Merge rows that state one rule into one statement, numbered S1 onward, keeping every source. Add every caller candidate as its own statement.
4. **Place.** For each statement, in this order: quote the owner evidence; apply the gate in `references/gate.md` and name the protocol rule that decided; then grep the code for where the rule is enforced and label it present with `file:line`, `none`, or `misplaced` with the reason. Code never decides placement. Split an "and" of independently violable clauses into separate statements. When two sources disagree, record the discrepancy, and when the disagreement is a product decision, record it as a question instead of choosing.
5. **Verify your own citations.** Every `file:line` in the reply is a line you read in this session. Say explicitly what you did not check.
6. **Re-check the worktree.** Run `git status --short` again. Report every change you did not make.
7. **Report** in the shape of `references/report.md`. In mode `record`, after the report is drafted, run the define and remove commands for the entries no question blocks, run `arclint check .`, and add the command output to the report. Never hand-write the domain file.

## Rules

- Read-only in mode `report`: no file written inside the repository, no git command that changes state. Mode `record` writes only through arclint.
- Code is not domain evidence; it is enforcement evidence only.
- A row is a candidate, not a statement. A statement is one rule in the domain's voice with its owner as subject.
- One tool call that returns the whole list beats many greps. Keep the inventory in a file and filter it in the call.
- No em dashes anywhere in the reply or in recorded text.
- Do not stop early. The reply is complete when every statement has a row, every entry has evidence and a code consequence, and every undecidable one is a question.
