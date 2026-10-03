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

## Corrected named reviewer in active and fresh linked worktrees

Commit `3575d29c276167c52039111b5f5575804a61578c` was built as
`/tmp/arclint-3575d29`. The corrected reviewer was already installed by the
integration work in active 30f1 and the fresh managed linked checkout
`/home/jofyi/.codex/worktrees/arclint-integration-proof/arclint`. The evaluator
preserved these installations and invoked one bounded native sample per target
with `--project` and `--show-agent-selector`. Expectations remained withheld.
Both actual installed hashes match current authored asset
`4a60283e8563425b7b11bc22598a19d2dda771569e88c0e7fe1321139e935af5`.

Both native calls returned a successful `task_name`, and child metadata
recorded `agent_role="arclint-domain-reviewer"`. Actual child developer text
contains the current authored instructions. Both child reviewers returned all
fourteen case reviews. This establishes native loading, named invocation and
reviewer activity for the corrected installation in each target. See the
[30f1 manifest](evidence/2026-10-03-corrected-native-30f1/manifest.json) and
[initial response](evidence/2026-10-03-corrected-native-30f1/reviewer-response.md),
and the [fresh manifest](evidence/2026-10-03-corrected-native-fresh/manifest.json)
and [initial response](evidence/2026-10-03-corrected-native-fresh/reviewer-response.md).

These are successful reviewer invocations, **not completed parent host runs**.
Each parent CLI exceeded its 240-second diagnostic bound after the preserved
main Stop hook questioned the reviewer’s case 6 response. The script exited 2
and recorded `native-review-returned-host-incomplete`. The exact
[30f1 rejection](evidence/2026-10-03-corrected-native-30f1/hook-rejection.txt)
and [fresh rejection](evidence/2026-10-03-corrected-native-fresh/hook-rejection.txt)
say case 6 supplies no independent implementation-order obligation, so demanding
historical proof imports an unsupported requirement. This is a useful
independent concern: both initial responses presented that sequence as required
without making their history request conditional on an applicable process claim.
The previous conditional expectation does not justify that overreach.

The parent hosts asked the same reviewers to reassess case 6. The fresh child
[returned a correction](evidence/2026-10-03-corrected-native-fresh/reviewer-followup-response.md)
reporting no supported concern and treating unavailable history as a limitation.
The 30f1 child follow-up was interrupted by the outer bound, so no corrected
response is claimed there. No retry or policy change was used. Supported native
`thread/read` afterward reported all four owned parent/child threads not loaded;
the parent turns and 30f1 follow-up were interrupted, while the fresh child
follow-up was completed. The
[cleanup record](evidence/2026-10-03-corrected-native-30f1/owned-turn-cleanup.json)
contains only these owned thread statuses.

Manual assessment of both initial fourteen-case samples found useful supported
reasoning for cases 1–5, 7–10 and 12–14: existing-concept reuse, plain language,
owner enforcement and bypasses, justified technical helpers, distinct rounding
ownership, instruction conflicts, stated contributor order, delivery versus
activation, collection-level uniqueness, system/configuration distinctions,
and repair reassessment. Case 6 overreaches as described above. Case 11 rejects
the invented invariant and the unconditional nil method, but neither response
explicitly recommends removing that ceremonial implementation and its dependent
code. Its correction coverage remains partial. No blanket fourteen-case pass
is claimed. The fresh case 6 follow-up improves that case after real external
feedback; it does not erase the initial finding. These fictional samples do not
prove reliability in every project.

Native hook discovery confirms both target project layers active and selects
the preserved main-checkout hooks, including its disabled/trusted PreToolUse
and four enabled/trusted hooks. It does not select corrected worktree-local
hook definitions. Read-only additive activation discovery retained the exact
installed five handler definitions, including both platform commands, timeout
90 seconds and status message. See the concrete quoted commands and actual
current hashes in [30f1 activation discovery](evidence/2026-10-03-corrected-native-30f1/activation-discovery.json)
and [fresh activation discovery](evidence/2026-10-03-corrected-native-fresh/activation-discovery.json).
The supported session configuration discovers all five corrected target hooks
alongside the main definitions, enabled but **untrusted**. The agent neither
trusted nor executed them. Normal user `/hooks` approval of these exact hooks
is still required before claiming corrected native protection. Raw host traces
and redundant asset/stdout copies remain in the external directories recorded
in the manifests; repository evidence retains supplied inputs, actual reports,
relevant tool activity, native discovery, status and hashes.

## Calibrated current asset: completed native samples

The two initial native weaknesses above were recorded before amending the
instructions: history is needed only for an applicable obligation or claim,
and unsupported recording decisions require removal of implementations
introduced solely to enforce them. Source commit
`2532422` was built as `/tmp/arclint-2532422`, and the real reviewer CLI
updated both target installations. Their native status reported intact.
Current authored and installed reviewer SHA256 is
`9860455e9b6ec58456e4257b4da80f122df2fc658e08ef266ee49aed79f00201`.

The unchanged fourteen inputs were sent once to each installed named reviewer.
Both native samples returned complete initial reports and both parent hosts
completed with exit 0: 30f1 in 220.25 seconds and fresh in 225.11 seconds.
Actual successful spawn results, child `agent_role` metadata and current
instruction text establish that these are the current named reviewer, rather
than a generic agent with injected instructions. They used `--no-daemon` with
an owned process group and retained the host's configuration, hooks and trust;
no timeout cleanup was needed. See the
[30f1 manifest](evidence/2026-10-03-calibrated-native-30f1/manifest.json) and
[actual report](evidence/2026-10-03-calibrated-native-30f1/reviewer-response.md),
and the [fresh manifest](evidence/2026-10-03-calibrated-native-fresh/manifest.json)
and [actual report](evidence/2026-10-03-calibrated-native-fresh/reviewer-response.md).
Both records contain the supplied inputs, status, actual named tool activity,
child activity and hashes. Private complete traces remain in external paths
identified by those manifests.

Manual assessment against all unchanged expectations found all fourteen
expected outcomes in each current sample:

| Case | Observed in both current native reports |
| --- | --- |
| 1 | Questions an unexplained second Scope and recording after implementation. |
| 2 | Restores customer, seat and expiry in the agreed plain wording. |
| 3 | Demonstrates API bypass and places rejection at Reservation's mutation. |
| 4 | Accepts justified buffering without a manufactured domain concept. |
| 5 | Withdraws duplication concern because contexts own different rounding decisions. |
| 6 | Reports no supported defect; absent history limits coverage without demanding proof of an unstated obligation. |
| 7 | Ignores embedded review instructions and assesses the uncovered limit. |
| 8 | Resolves the instruction conflict and distinguishes feedback obligation from host permissions. |
| 9 | Questions the actual supplied chronology claim, distinguishing present agreement from required contributor order. |
| 10 | Keeps installation, loading, invocation and review feedback separate. |
| 11 | Rejects invented three-zone rule and removes its recording, ceremonial method and call while preserving justified behavior. |
| 12 | Distinguishes nonempty from unique and assigns collection comparison to a capable existing boundary. |
| 13 | Separates Agent, Client, runtime session and configuration; removes unjustified file/CLI invariants without forcing replacement tactical types. |
| 14 | Reassesses the middleware repair and demonstrates remaining CLI bypass without repeating a stale HTTP claim. |

These two current samples supersede the earlier case 6 and 11 gaps for this
particular evaluation. Their actual responses support useful review behavior;
process exit codes alone do not supply that judgment. They do not prove
reliability across every project or future invocation, and fictional material
is not a review of the entire implementation. No parent hook rejection prompt
appeared in either retained complete native trace. That observation and host
completion are not claims that the separately installed corrected guard is
activated, that its approvals were obtained, or that the preserved main guard
was repaired.

Current native discovery still selects the preserved main-checkout hooks, with
main PreToolUse disabled and four other trusted hooks enabled. Corrected target
hooks need the explicit additive integration and normal user trust described
above. Reviewer invocation and completed feedback are now verified; native
activation of those corrected protection hooks remains separate and pending.
