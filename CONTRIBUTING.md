# Contributing to arclint

Build the binary with `make build`; it produces `./arclint` with the grammar subset build tags.

Run the full check suite with `make ci`; it runs `go vet` (including the oracle, bench, and agentbench tagged packages), `go test ./...`, and a selfcheck that lints this repository with its own `rules.yaml`.

Rule behavior is verified by the rule-test harness: `arclint rules test [case-file|dir]` materializes each case into a fresh tree, runs the ruleset, and asserts the complete violation set; with no argument it reads cases from `.arclint/tests`.

Make one commit per change; do not bundle unrelated edits into a single commit.

Write commit messages whose body states the problem the change solves, not only what was edited.

Open a pull request with the template filled in; CI must pass before review.

## The agent convergence bench

`make agentbench` measures whether real coding agents repair
architecture violations from arclint's diagnostics (method and result
history: `docs/research/agent-convergence-bench.md`). It is a
measurement tool, not part of `make ci`: it needs network access, an
authenticated agent CLI, and it spends real tokens on that CLI's
account — roughly one to five minutes of agent time per trial, four
trials per default run. The test fails only on harness errors, never on
an agent losing.

Prerequisite: a headless agent CLI that accepts a prompt as its final
argument, works in the current directory, and auto-approves its own
file edits. The default is `codex exec --sandbox workspace-write
--skip-git-repo-check`; any equivalent works:

    # default (codex)
    make agentbench

    # any other agent CLI, e.g. a model through omp
    AGENTBENCH_AGENT_CMD='omp launch -p --auto-approve --no-session --model claude-haiku-4-5 --thinking low' \
      make agentbench

Environment knobs:

| variable | meaning |
|---|---|
| `AGENTBENCH_AGENT_CMD` | agent command line; the repair prompt is appended as one final argument |
| `AGENTBENCH_SCENARIOS` | comma-separated scenario names to run (default: all) |
| `AGENTBENCH_REPEATS` | run every (scenario, condition) cell N times, for medians instead of anecdotes (default: 1) |
| `AGENTBENCH_OUT` | write the full JSON trial records to this path |
| `ARCLINT_BIN` | set by the make target; point it at a compiled arclint when invoking `go test -tags agentbench` directly |

Each trial initializes a pattern into a temp repo, overlays violating
code, and loops (check, hand findings to the agent, re-check) up to
three iterations. Success requires all three: `arclint check .` clean,
`go build ./...` green, and the contract surface untouched — the
harness fingerprints `rules.yaml` and `.arclint/` and records a trial
as gamed if a "repair" edited them. Every cell runs twice, once with
diagnostics only and once with prompt-time context (the generated
AGENTS.md block plus `arclint context` output), so the two channels can
be compared.

Adding a scenario: add an entry to `scenarios` in
`internal/agentbench/agentbench_test.go` naming a builtin pattern and
an overlay directory (see `testdata/agentbench/goprod-fat`). The
overlaid tree must produce at least one violation and must build before
repair; the harness fails fast otherwise. Design overlays so that every
capability has call sites — a repair that deletes functionality should
break the build, not pass the bench. When you run the bench, append
your results (date, agent CLI, model, effort) to the research doc so
the history stays comparable.
