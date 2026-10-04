# Evaluating the reviewer

Build the current CLI with `make build`. Run the opt-in evaluation harness
against a fresh fixture, with an output directory outside the repository:

```sh
python3 .agents/domain-reviewer/evaluate.py "$PWD/arclint" /tmp/arclint-reviewer-evaluation
```

This uses an authenticated Codex process and installs the reviewer only into
its temporary fixture. It supplies the inputs from [cases.md](cases.md), with
expected outcomes withheld. Compare each returned response with its expected
behavior; process success alone does not establish semantic correctness.

If native custom-agent discovery is unavailable, the harness reports that
limit. `--direct-instructions` evaluates the authored instructions separately;
it does not establish native activation. The manifest distinguishes these modes
and records instruction hashes and observed host activity.

Deterministic tests cover the workflow review contract, event feedback,
collection limits, installation/status, preservation of edits, and linked
worktree launchers. Run `make check` and `make ci`. Representative semantic
review still needs separate evaluation, including unsupported changes, missing
enforcement, unjustified duplication, and reassessment of repairs and rebuttals.

Evaluation results apply to the instruction hash and supplied cases recorded
for that run. They do not establish the behavior of a different revision or
guarantee future judgments.
