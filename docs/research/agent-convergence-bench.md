# Agent convergence bench

The feedback-loop claim behind arclint's agent-facing surface, measured
instead of assumed: given a repository with architecture violations and
arclint's diagnostics, does a coding agent converge to a conforming
implementation, and does prompt-time context change convergence?

The published evidence supports deterministic external feedback
(STALL+, arXiv 2406.10018; CoCoGen, arXiv 2403.16792; Static Analysis
as a Feedback Loop, arXiv 2508.14419) and prompt-time structural
context (STALL+ prompting-phase results; deterministic anchoring,
arXiv 2606.26979) for code generation and repair. None of it measures
architectural conformance. This bench closes that gap for arclint's own
diagnostic and context surface.

## Method

Harness: `internal/agentbench` (build tag `agentbench`), run via
`make agentbench`. Each trial:

1. `arclint init --pattern <p> --runtimes go --force` into a fresh
   temp repo, then overlay violating Go code (the pattern's template
   rules.yaml stays authoritative).
2. Loop up to 3 iterations: run `arclint check .`; hand the findings to
   a real coding agent CLI working in the repo; re-check.
3. Success requires all three: `arclint check .` clean, `go build ./...`
   green, and the contract surface untouched (rules.yaml, `.arclint/`)
   — a "repair" that edits the contract is recorded as gamed.

Scenarios:

- `feature-slice-dirty` — the feature-slice pattern over
  `testdata/fixtures/pattern-feature-slice-dirty`: 6 violations across
  builtin kinds (banned bucket files, protected-module imports,
  cross-feature dependencies) and the pattern's TS extension rules.
- `goprod-fat-interface` — the go-prod pattern over
  `testdata/agentbench/goprod-fat`: one `goprod:small-interfaces`
  violation on a 6-method interface whose every method has call sites,
  so the repair must restructure consumers, not delete capability.

Conditions:

- `diag` — the prompt carries the task, hard constraints, and the
  human-readable `arclint check .` output (messages plus fix hints).
- `diag+context` — additionally, `arclint agents --write` has installed
  the generated AGENTS.md block (the agent CLI reads it), and the
  prompt carries `arclint context <path>` output for the violating
  locations.

The agent command is `AGENTBENCH_AGENT_CMD` (default `codex exec
--sandbox workspace-write --skip-git-repo-check`). Results are
environment- and model-dependent measurements, not CI assertions: the
test fails on harness errors, never on an agent losing. Caveats: n is
small, one agent family per run, and the agent's global instruction
files (for codex, `~/.codex/AGENTS.md`) apply identically to both
conditions.

## Results

### 2026-08-15 — codex-cli 0.144.3, gpt-5.6-sol, reasoning effort low

| scenario | condition | initial | iterations | final | success | gamed | wall |
|---|---|---|---|---|---|---|---|
| feature-slice-dirty | diag | 6 | 1 | 0 | yes | no | 59s |
| feature-slice-dirty | diag+context | 6 | 1 | 0 | yes | no | 100s |
| goprod-fat-interface | diag | 1 | 1 | 0 | yes | no | 78s |
| goprod-fat-interface | diag+context | 1 | 1 | 0 | yes | no | 55s |

Reading. Every trial converged in one iteration with the contract
surface untouched and the build green, including the six-violation
multi-kind repair. Two conclusions, one honest limitation:

1. arclint's current diagnostic surface (rule id, path, line, message,
   fix hint) was sufficient for one-shot convergence by a current
   frontier agent at low reasoning effort. No trial showed the agent
   needing per-rule intent or rationale to repair correctly.
2. The diag versus diag+context comparison did not separate: both
   conditions sat at the one-iteration ceiling. This run therefore
   shows prompt-time context was not *needed* for convergence at this
   difficulty; it does not show context adds nothing. Discriminating
   the conditions needs harder scenarios (larger repos, conflicting
   plausible repairs) or weaker agents, where the ceiling lifts.

Consequence for schema decisions: as of this run there is no measured
convergence deficit motivating per-instance intent/rationale/prompt
fields or richer per-finding semantic diagnostics. The next
discriminating experiment, if wanted: scenario difficulty scaling on a
real mid-size repository, or generation-time (not repair-time)
measurement, where the anchoring literature predicts context matters
most.
