# Report shape

Five sections, in this order. Headers are allowed. Every `file:line` was read this session. Name the kind with every number ("12 statements", never a bare "12"). Label a claim you did not verify `[INFERENCE]`.

## 1. Scope read

One short paragraph: the run directory, base ref and merge base, head (a commit or the working tree), whether uncommitted and untracked work was included, the evidence numbers fetched, the mode, and the inventory's summary line (rows by kind and source, generated files skipped), and how the rows were read: by you, or in parts by readers, with the coverage check's counts. Then what was not checked, and every worktree change made by someone else during the run.

## 2. Inventory

| id | statement | sources | evidence | placement | owner and key, or reason | rule | enforcement today | proving test |
|---|---|---|---|---|---|---|---|---|

- `id`: S1 onward, stable through the reply.
- `statement`: one rule, short, in the domain's voice.
- `sources`: every `file:line` that states it, docs first, then help text, code comments, tests, issues. A `deleted` row is cited as `<merge base>:file:line`.
- `evidence`: the owner's words or docs that decide it, quoted short with their source, or `none` (then the placement is a question or a no entry).
- `placement`: one of the placements in `gate.md`.
- `owner and key, or reason`: `Owner / kebab-key` for an entry, the no-entry reason or the declined reason otherwise, and `see Qn` for a question.
- `rule`: the protocol rule number, distillation rule id or issue that decided.
- `enforcement today`: one of the enforcement labels in `gate.md`.
- `proving test`: `path:TestName` you read, or `none`.

Entries, changes to entries, questions and declined statements each get a row of their own. No-entry statements of one reason in one file may share one row, listing every source.

After the table, list documentation discrepancies found on the way, one line each with both sources, and every recorded text over its word limit with its count.

## 3. Entries to record

First the YAML fragment the recorded domain would hold, in the shape of `domain.arclint.yaml` (`version: 1`, `project`, `contexts`, then `value_objects`, `invariants`, `assertions`, `questions` under the context), limited to the entries that change. Give each proposed text its word count against its limit. Then the exact commands that record them, one per entry, using only flags that `arclint domain define --help` lists:

```bash
arclint domain define value_object DomainRecording --context vocabulary --definition "..."
arclint domain define invariant spelled-with-slashes --owner DomainRecording --context vocabulary --statement "..." --verified-by internal/domain/vocab/domain_recording_test.go:TestNewDomainRecordingStaysInsideTheRepository
arclint domain define question ruleset-names-the-recording-when --context vocabulary --text "..."
arclint domain remove invariant <key> --owner <Owner>
```

Under each entry: the evidence that decided it, quoted or cited, and each literature citation with its tier and whether you read it.

## 4. Code changes each entry requires

One bullet per recorded entry: the owner, where enforcement must move or be added, which call sites go away, and which tests pin the behavior today. For a guarantee of an operation, say which operation decides and which application step carries out the change. For a declared service, say which shape of `gate.md` (a check in the model, or a value) it takes. Name files and functions; write no code.

## 5. Questions for the owner

Numbered plain questions, one decision each, written so a yes or no or a short choice answers them. Include the questions recorded under `questions` in section 3 and any statement the gate could not place.

In mode `record`, append the output of the define and remove commands and of `arclint check .`.
