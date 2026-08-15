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

### 2026-08-15 — weak-model matrix, omp harness

Same scenarios and conditions, three deliberately weaker agents driven
through omp (`omp launch -p --auto-approve --no-session --model <m>`):
gpt-5.4-mini (thinking low, codex OAuth), grok-composer-2.5-fast (xai
OAuth), claude-haiku-4-5 (thinking low, Anthropic OAuth). 12/12 trials:
one-iteration convergence, zero gaming, build green. Combined with the
codex run: 16/16 across four model families.

Agent wall seconds on the six-violation scenario (n=1 per cell, and the
haiku run overlapped another bench, so times are indicative only):

| model | diag | diag+context |
|---|---|---|
| gpt-5.4-mini | 234s | 119s |
| grok-composer-2.5-fast | 202s | 222s |
| claude-haiku-4-5 | 262s | 136s |

Reading. The sufficiency conclusion strengthens: arclint's current
diagnostic surface produced one-shot architectural repair even from
small non-frontier models. Iterations still cannot separate the
conditions (ceiling everywhere), so the only visible signal is agent
time, where context roughly halved the hard-scenario repair for two of
three weak models. At n=1 per cell that is a hint to test, not a
finding. Separating the conditions now clearly requires harder
scenarios, not weaker agents.

### 2026-08-15 — repetition study, n=5 per cell, hard scenario only

Five repeats per condition on feature-slice-dirty, sequential (no
concurrent runs contending), two weak models via omp at thinking low.
All 20 trials: one-iteration convergence, zero gaming, build green —
the all-time tally is 36/36.

Agent seconds, sorted, with medians:

| model | diag | diag+context |
|---|---|---|
| gpt-5.4-mini | 91 115 146 149 203 → **146** | 85 89 92 102 145 → **92** |
| claude-haiku-4-5 | 126 158 185 211 336 → **185** | 106 118 179 181 207 → **179** |

Reading. The timing effect is model-dependent. For gpt-5.4-mini the
n=1 hint replicated: a 37% median reduction, with the context sample
beating the diag sample in 21 of 25 pairwise comparisons — a
consistent shift, borderline at this sample size. For haiku the
medians are flat (185 versus 179); the context sample looks better
only through diag's one 336s outlier, and wins just 18 of 25 pairwise
comparisons. Conclusion: prompt-time context never changed correctness
at this difficulty, sped one weak model up substantially and another
not measurably. The sufficiency verdict on the current diagnostic
surface is final for this scenario tier; any further case for richer
prompt-time or per-rule fields has to come from harder scenarios or
generation-time measurement, not from more repetitions here.
