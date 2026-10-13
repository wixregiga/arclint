# Report shape

Five sections, in this order. Headers are allowed. Every `file:line` was read this session.

## 1. Scope read

One short paragraph: base ref, branch, whether uncommitted and untracked work was included, the evidence numbers fetched, the mode. Then what was not checked, and every worktree change made by someone else during the run.

## 2. Inventory

| id | statement | sources | placement | owner and key, or reason | rule | enforcement today |
|---|---|---|---|---|---|---|

- `id`: S1 onward, stable through the reply.
- `statement`: one rule, short, in the domain's voice.
- `sources`: every `file:line` that states it, docs first, then help text, code comments, tests, issues.
- `placement`: one of the placements in `gate.md`.
- `owner and key, or reason`: `Owner / kebab-key` for an entry, the no-entry reason otherwise, and `see Qn` for a question.
- `rule`: the protocol rule number or distillation rule id that decided.
- `enforcement today`: `file:line`, `none`, `misplaced: where`, or `split: a / b`.

After the table, list documentation discrepancies found on the way, one line each with both sources.

## 3. Entries to record

First the YAML fragment the recorded domain would hold, in the shape of `domain.arclint.yaml` (`version: 1`, `project`, `contexts`, then `value_objects`, `invariants`, `questions` under the context), limited to the entries that change. Then the exact commands that record them, one per entry:

```bash
arclint domain define value_object DomainRecording --context vocabulary --definition "..."
arclint domain define invariant spelled-with-slashes --owner DomainRecording --context vocabulary --statement "..."
arclint domain define question ruleset-names-the-recording-when --context vocabulary --text "..."
arclint domain remove invariant <key> --owner <Owner>
```

Under each entry: the evidence that decided it, quoted or cited.

## 4. Code changes each entry requires

One bullet per recorded entry: the owner, where enforcement must move or be added, which call sites go away, and which tests pin the behavior today. Name files and functions; write no code.

## 5. Questions for the owner

Numbered plain questions, one decision each, written so a yes or no or a short choice answers them. Include the questions recorded under `questions` in section 3 and any statement the gate could not place.

In mode `record`, append the output of the define and remove commands and of `arclint check .`.
