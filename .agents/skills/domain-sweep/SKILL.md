---
name: domain-sweep
description: Beta. Read every documentation, comment, help-text, test-name and domain-recording line a change adds or removes, and place each rule it states through the domain-librarian's gate as an invariant, assertion, specification, definition change, declined rule, question, or no entry, with the deciding rule cited. Use before every pull request on arclint, when a branch wrote code before recording meaning (AGENTS.md workflow step 3), when issue #94's domain process is requested for a change, or before recording a change's rules with arclint domain define. Read-only by default; mode record writes only through arclint.
---

# Domain sweep

**Beta.** The gate follows the decisions of issues #125, #128, #143 and #144, which arclint now carries, and of #148 and #151, which it does not yet; report where arclint does not do what the gate describes.

You run arclint's domain process over one change: find every statement of what must or never happen, decide where each belongs with the domain-librarian's gate, say where code enforces it and which test proves it, and hand the owner the entries, the code moves and the open questions. The domain-librarian skill records entries; this skill decides what there is to record and proves each decision with evidence.

No keyword decides what you read. A rule is often worded without one: PR #96's discovery order ("the first of these files that exists") had none, and a keyword pass missed it. You read every row the inventory lists. Keywords only order the reading.

## Parameters

Pass them as `key=value` in the invocation argument or in plain words. Each has a default.

| parameter | default | meaning |
|---|---|---|
| `base` | `main` | The change starts at the merge base of this ref and the head. |
| `head` | working tree | A commit to sweep instead of the working tree. Without it, the change is committed and uncommitted work together, plus untracked files. |
| `pr` | none | A pull request number. Sets `base` and `head`. For an open one, `gh pr view N --json baseRefOid,headRefOid`. For a merged one, `head` is its merge commit (`gh pr view N --json mergeCommit,commits`), and `base` depends on how it merged: a merge commit has two parents, and `base` is `<oid>^1`; a rebase put the pull request's `n` commits on the base branch, ending at `<oid>`, and `base` is `<oid>~n`; a squash put one commit there, and `base` is `<oid>^`. Tell a rebase from a squash by comparing `git log --format=%s -n <n> <oid>` with the pull request's commit headlines. |
| `scope` | `touched` | `changed`: lines the change adds or removes. `touched`: also the unchanged lines of touched files, for statements whose meaning the change moved or made enforceable. |
| `paths` | `*.md *.go *.ts *.tsx *.mts *.cts *.py` | Where statements live: Markdown docs, and Go, TypeScript and Python comments, help text, messages and test names. Domain recordings are always included. |
| `exclude` | the script's defaults | Files that state nothing of their own: `.claude/skills` copies, `testdata` directories, and the root `domain.arclint.cml` export. |
| `recording` | from `arclint domain` | The recording's path when it is not named `domain.arclint.yaml`, `.yml` or `.cml`; `arclint domain` prints it on its `Source:` line. |
| `candidates` | none | Statements the caller wants covered. Each becomes a row even when the answer is no entry. |
| `evidence` | none | Issue and pull request numbers whose text counts as the owner's words, for example `#95 #97 #98`. |
| `mode` | `report` | `report`: read-only, the reply is the output. `record`: also record undisputed entries through `arclint domain define` and `arclint domain remove`, then run `arclint check .`. |
| `arclint` | build | Path to the arclint to use. The default builds one from the head, because an installed arclint may be older than the change. |

## Procedure

Say in one line what you are starting before each step. Batch every independent read of a step into one response. `<skill dir>` is the directory holding this file, in the checkout you run from; a change to this skill's own files is swept like any other change. Every file you write goes in one run directory of its own, `run=$(mktemp -d /tmp/domain-sweep.XXXXXX)`, so two runs never share files.

1. **Orient.** When `head` is a commit, export it read-only into the run directory: `mkdir "$run/tree" && git archive <head> | tar -x -C "$run/tree"`, and build arclint from the checkout you run in, so the exported tree's tool configuration is never loaded: `go -C "$run/tree" build -buildvcs=false -o "$run/arclint" ./cmd/arclint`. An archive has no `.git`, hence `-buildvcs=false`. Without a `head`, build from the working tree: `go build -o "$run/arclint" ./cmd/arclint`. Run that arclint, from the exported tree or the working tree, for every arclint command below. Run `arclint domain` for the recorded language and its `Source:` path; when that path is not named `domain.arclint.yaml`, `.yml` or `.cml`, pass it as `--recording <path>` to every inventory command below. List the touched files once with `python3 -I -B <skill dir>/scripts/inventory.py --base <base> [--head <head>] [--recording <path>] --files` and run `arclint context` on that list before opening any of them. Read `.agents/skills/domain-librarian/SKILL.md` for the numbered protocol and the `distillation_rules` ids in `.agents/skills/domain-librarian/VOCAB.yaml`. Read `references/gate.md` in this skill. Note `git status --short` so you can tell at the end whether the worktree changed under you.
2. **Owner's words.** For each number in `evidence`, fetch the body and comments once (`gh pr view N --json title,body,comments`, `gh issue view N --json title,body,comments`) into a file under `"$run"`. Fetch the issues the gate cites when a statement turns on them. Only the recorded language, the owner's docs and the owner's words classify a statement, as "What counts as evidence" in `references/gate.md` defines them. Everything the change itself records or documents, tests, code and agent-written text are claims to examine.
3. **Inventory.** Run the script once:

   ```bash
   python3 -I -B <skill dir>/scripts/inventory.py --base <base> [--head <head>] [--recording <path>] --scope <scope> --out "$run/rows.tsv"
   ```

   Each row is `kind source hint file:line text`; `python3 -I <skill dir>/scripts/inventory.py --help` explains every column and what is listed. Read every `added` and `deleted` row, whole: a deleted statement may need its entry removed with `arclint domain remove`. Read the `existing` rows of each file whose changed rows moved or enforced a meaning. Rows come file by file, files with more hinted rows first; read each file's rows in order so a paragraph stays whole. When the rows are more than one reader can hold, split them by file into parts under `"$run/parts"` and read every part. A reader you delegate to gets this skill's gate, its rules and the no-toy rule verbatim, and writes two files beside its part:
   - `<part>.statements.tsv`, one statement per line: `statement`, `sources`, `placement`, `owner and key, or reason`, `evidence`, `enforcement today`, `proving test`, with the values `references/gate.md` and `references/report.md` define;
   - `<part>.coverage.tsv`, one line per row of the part: its `file:line`, then the number of the statement it maps to or the reason it states none.

   Before merging, check with a script that every row of `rows.tsv` appears in exactly one coverage file, and report the counts. A `file:line` a reader read counts as read this session; re-read yourself the lines behind every entry and every question. Merge rows that state one rule into one statement, numbered S1 onward, keeping every source. Add every caller candidate as its own statement.
4. **Place.** For each statement, in the order `references/gate.md` gives: check the declined rules; quote the owner evidence; apply the placement table and name the protocol rule or issue that decided; separate the decision from its execution; for a declared service, choose its shape; grep the code for where the rule is enforced and label it present with `file:line`, `none`, or `misplaced` with the reason; name the proving test you read, or `none`; count the words of every text you propose. Code never decides placement. Split an "and" of independently violable clauses into separate statements. When two sources disagree, record the discrepancy, and when the disagreement is a product decision, record it as a question instead of choosing.
5. **Verify your own citations.** Every `file:line` in the reply is a line you or a reader read in this session. Every literature citation states its tier and whether you read it. Say explicitly what you did not check.
6. **Re-check the worktree.** Run `git status --short` again. Report every change you did not make.
7. **Report** in the shape of `references/report.md`. In mode `report` the reply is the report; when the caller names a file outside the repository, also write it there if you can, and say so if you cannot. In mode `record`, after the report is drafted, run the define and remove commands for the entries no question blocks, run `arclint check .`, and add the command output to the report. Never hand-write the domain file.

## Rules

- Read-only in mode `report`: no file written inside the repository, no git command that changes state. Mode `record` writes only through arclint.
- Every row is read. A keyword, its absence, or a row's length never decides whether a line is read.
- Code is not domain evidence; it is enforcement evidence only. Tests pin behavior; they do not approve it.
- A row is a candidate, not a statement. A statement is one rule in the domain's voice with its owner as subject.
- One tool call that returns the whole list beats many greps. Keep the inventory in a file and filter it in the call.
- No toy implementations: every request is production-intent. Placeholder logic, hardcoded happy paths, fixture-only validation, and stubbed returns are defects, not drafts. A reader you delegate to receives this rule verbatim.
- No em dashes anywhere in the reply or in recorded text.
- Do not stop early. The reply is complete when every row was read, every statement has a row, every entry has evidence, a code consequence and a proving test or `none`, and every undecidable one is a question.
