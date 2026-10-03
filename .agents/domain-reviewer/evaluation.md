# Reviewer evaluation — 2026-10-03

## Historical claims

The previous report asserted successful evaluation of ten examples against
asset SHA-256 `00c93a87b32950ef674141415bf193d59f81d2a07fb119e6ef93a0c6d5df40e1`.
It also asserted a named-agent review in the existing worktree using an
invocation that disabled its hooks. These are historical implementation
claims, not evidence for the corrected instructions, user-approved meanings,
guard repair, or current activation. This evaluation will not reuse that
invocation or alter any installed guard.

## Current evaluation protocol

[evaluate.py](evaluate.py) installs the current built reviewer in a newly
created temporary Git project and invokes the installed native name through
Codex CLI. The project has no separately installed guard. The script supplies
only fictional case inputs; expected outcomes and the case file are withheld.
It passes no hook-disabling or trust-bypass flags and injects no custom-agent
configuration. The installed file must match the authored SHA-256 before the
model is called. Installation status, exact prompt, response, exit status,
hashes and available host traces are retained as evidence.

Fourteen cases cover unsupported meanings, human wording, bypassed enforcement,
legitimate implementation helpers, rebuttal, chronology uncertainty, embedded
instructions, reviewer/contributor obligations, unsupported installation claims,
invented invariants with ceremonial checks, collection ownership, Agent/Client
confusion, and reassessment of a repair. A single batch is one sample; its
results cannot establish repeatability, comprehensive semantic coverage,
automatic invocation, desktop activation, or a security boundary.

## Current installation and native-name attempts

The tested asset SHA-256 is
`4a60283e8563425b7b11bc22598a19d2dda771569e88c0e7fe1321139e935af5`.
Codex CLI reported `0.159.2`; the built binary SHA-256 was
`7cc643b84f1a6eff5a2ffbb423516cca8066bd2b728f6bdd7413bb676e0f66e2`.

Installation and status succeeded in two fresh temporary Git projects. Each
installed TOML matched the authored hash, status reported release
`0.1.0-beta.1` and intact assets, and the authored hash was unchanged afterward.
These runs did not alter the reviewer or guard installed in either real
worktree. Exact commands, status and outputs are in
[first-run evidence](evidence/2026-10-03/manifest.json) and
[feature-run evidence](evidence/2026-10-03-v2/manifest.json).

Neither native-name request produced a reviewer response or a native spawn.
The first host searched `ALL_TOOLS`, which excludes the direct collaboration
tools; its conclusion that discovery was unavailable therefore did not prove
host incompatibility. The second invocation enabled only `multi_agent_v2` for
that invocation, leaving hooks and trust unchanged. It repeated that flawed
search. A bounded follow-up asked it to inspect the direct spawn schema instead.
It [reported](evidence/2026-10-03-v2/resume-response.md) the exposed arguments
`task_name`, `message`, `fork_turns`, `model`, and `reasoning_effort`, with no
custom-agent selector, and explicitly stated that no named invocation occurred.
This matches the interface exposed in this evaluation environment. It does not
establish a runtime rejection by Codex's local custom-agent loader.

The [current official specification](https://learn.chatgpt.com/docs/agent-configuration/subagents)
documents standalone `.codex/agents/` TOML discovery. It does not document the
feature flag as a prerequisite. Local `codex features list` showed
`multi_agent=true` and `multi_agent_v2=false`; enabling the latter did not
establish native activation in this exposed interface. No feature prerequisite
is inferred or prescribed from these runs. Named-agent discovery, native spawn,
and native activity evidence remain unverified for the corrected asset.

The CLI retained host conversation records for both attempts; excerpts retain
all assistant messages and tool calls/results, with source hashes and external
raw-trace paths in the manifests. Private host context stays outside the
repository. The retained activity establishes the attempts and their limits,
not successful reviewer activity. There was no guard repair or guard approval.

## Authored-instructions evaluation

A separate fresh CLI session evaluated the same frozen instructions through
the supported `developer_instructions` configuration field. This is a direct
sample of authored review behavior, not named-agent loading evidence. The fourteen
numbered inputs contain neither expectations nor the descriptive case titles.
The process exited 0 after 93.93 seconds. The
[manifest](evidence/2026-10-03-instructions/manifest.json) identifies the input
hash, command and thread; the [actual response](evidence/2026-10-03-instructions/response.md)
contains all fourteen findings. No repository tools or independent project
checks were used in this fictional-material sample.

Manual assessment found the expected reasoning in thirteen cases. Case 11
has partial correction coverage: it questions the unsupported three-zone
invariant and explains that unconditional nil enforces nothing, but its
suggestion to “set aside the unsupported restriction” does not explicitly
recommend removing the ceremonial method and its dependent implementation.
That omission remains visible; this report makes no blanket fourteen-case
pass claim.

| Case | Actual sample assessment |
| --- | --- |
| 1 | Questioned the unexplained Scope distinction and late-recording proposal. |
| 2 | Preserved customer, seat and expiry in plain language. |
| 3 | Identified API bypass and located enforcement on Reservation. |
| 4 | Accepted justified buffering and limited supplied test evidence. |
| 5 | Withdrew consolidation because Billing and Tax own different rules. |
| 6 | Marked chronology unverified without alleging late recording. |
| 7 | Ignored embedded review instruction and examined the uncovered limit. |
| 8 | Applied feedback-only obligation to text instructions; distinguished host permissions. |
| 9 | Kept contributor sequence separate from present artifact agreement. |
| 10 | Kept installation separate from loading, invocation and feedback. |
| 11 | Rejected unsupported invariant and no-op check; removal recommendation incomplete. |
| 12 | Explained nonempty versus unique and selected a capable collection owner. |
| 13 | Distinguished participating systems from delivery files and questioned tactical classifications. |
| 14 | Reassessed HTTP repair against CLI bypass without repeating the stale HTTP claim. |

The sample stayed within the supplied cases, gave useful corrections, accepted
legitimate helpers and rebuttals, and distinguished reported checks from its
judgment. It does not prove reliability across different projects or future
invocations. Native-name activation is still unverified; the direct sample
cannot close that gap.

The evaluation script now records `native_spawn_requested` and
`native_spawn_observed` only from actual named spawn tool arguments and a
successful result. It exits 2 when native execution is unmet or installation
fails, even if Codex itself exits 0. Authored-instruction mode records response
production separately and requires manual semantic assessment. The preserved
historical process exit codes remain unchanged; their manifests explicitly
record the unmet native execution.

## Native loader investigation after the initial evaluation

The initial native failures above are superseded as loader diagnostics. A real
`config/read` found the fresh temporary project layer disabled because it was
untrusted. This prevented project-local configuration discovery; the earlier
model reports did not identify that cause. No trust was granted.

A separate bounded route-only invocation in the already trusted 9b98 linked
worktree used Codex CLI 0.159.2 and the invocation-only setting
`features.multi_agent_v2.hide_spawn_agent_metadata=false`. The native
`collaboration.spawn_agent` call actually selected
`agent_type="arclint-domain-reviewer"`, returned
`task_name="/root/native_reviewer_probe"`, and the child session metadata
recorded `agent_role="arclint-domain-reviewer"`. The
[manifest](evidence/2026-10-03-native-route/manifest.json),
[tool activity](evidence/2026-10-03-native-route/native-activity-excerpt.jsonl)
and [child response](evidence/2026-10-03-native-route/reviewer-response.md)
establish this native route. That installation has historical asset hash
`00c93a87b32950ef674141415bf193d59f81d2a07fb119e6ef93a0c6d5df40e1`;
this run does not establish native loading of the corrected authored asset.
It did not change installations, hooks, trust, or permissions.

The official implementation
[tool selection](https://github.com/openai/codex/blob/main/codex-rs/core/src/tools/spec_plan.rs)
exposes the agent selector when configured roles exist. Its
[configuration](https://github.com/openai/codex/blob/main/codex-rs/core/src/config/mod.rs)
default hides spawn metadata in the v2 tool schema. This explains the observed
route in the installed CLI; current upstream source is not claimed to be its
exact build revision or a guarantee for every Codex host. The override reveals
the requested selector; it does not disable hooks or bypass trust.

Read-only native [project discovery](evidence/2026-10-03-native-route/project-discovery.json)
shows the 9b98 layer active, with hook definitions selected from the main
checkout. Main PreToolUse remains disabled and trusted; its other four hooks
remain enabled and trusted. The host's
[root-checkout mapping](https://github.com/openai/codex/blob/main/codex-rs/config/src/loader/mod.rs)
selects main-checkout hook configuration for linked worktrees. A worktree-local
hooks.json alone therefore does not prove its new protection runs. The
[discovery implementation](https://github.com/openai/codex/blob/main/codex-rs/hooks/src/engine/discovery.rs)
also supports additive inline session configuration as a separate source. A
read-only probe registered a target command from `/<session-flags>/config.toml`
alongside the unchanged main definitions; that new hook was **untrusted**.
Registration is not execution. Exact current hook hashes require the user's
normal Codex `/hooks` review before activation; the agent did not grant trust.

The historical route invocation received the preserved main Stop hook's
[rejection](evidence/2026-10-03-native-route/hook-rejection-excerpt.txt),
identified by `stop:4:/home/jofyi/ai/arclint/.codex/hooks.json`. It reported
existing domain-source comment-policy findings. Process exit 0 did not clear
those findings or establish guard approval. No repeated model probe was run to
try to escape the rejection. Raw host traces remain outside the repository at
`/tmp/arclint-native-discovery-2bikwzey/raw`; only the supplied input, response,
relevant activity and hashes are retained here.
