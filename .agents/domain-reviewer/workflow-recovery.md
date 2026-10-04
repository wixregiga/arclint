# Task-focused workflow review

The user's reset is: “just start from no hook no anything. make hooks that
guard against you not following the domain workflow. make it report things
based upon the thing you're working. that's all i ever wanted”. This replaces
the previous proposed guard integration. The separate installed main guard
remains untouched.

The user clarified: “arclint is a tool that will be used by others. arclint uses
itself as well.” This is a reusable feature for any adopting project. ArcLint's
own work uses the same public commands and contracts; it receives no separate
project policy or hard-coded source scope.

The hook reports specific workflow departures in the current task. The user
already established the workflow: obtain ArcLint context before reading source;
explain and record changed meanings before implementation; implement actual
enforcement; examine ownership and unjustified duplication; reassess repairs and
rebuttals. Earlier user messages `01a0fa3c-7443-7d63-ba1b-2e1eaffaa0fa` and
`01a0fcd7-9c24-7580-8ec3-f7a10acec6d6` establish these review responsibilities
and require their contracts where the feature meets its users and contributors.

## Domain and implementation responsibility

`WorkflowReview.Assess` owns acceptance of the assessment. This is a domain
service under `domain-service-when-no-owner`: assessing a task against supplied
meanings and implementation is a significant decision no existing aggregate
owns. `contract-of-a-service` places its guarantees under assertions. No Agent
entity, installation identity, consistency aggregate or comment policy follows.
Evidence, Finding and Assessment are supplied snapshots and review results whose
meaning depends on their contents, not an identity. Equal contents are
interchangeable under `value-test`; record those actual values without artificial
constructor invariants. No claim about Agent identity follows from them.

Semantic evaluation examines supplied task, established domain, affected work,
context/activity, earlier findings and repairs. It identifies concrete workflow
departures and suggestions. It must not infer missing chronology from a code
snapshot, invent additional work or treat an assistant's previous claim as user
approval. Missing evidence produces an explicit coverage limit. An advisory
about missing evidence is not a demonstrated violation. Its judgment is tested
with representative cases; exact quotes alone cannot prove that judgment right.

`Assess` deterministically requires a task and validates each candidate's named
evidence, exact quote, specific departure and correction. It returns accepted
findings and explicit missing-evidence limits. These checks establish grounding,
not semantic correctness. A malformed or unsupported assessment is reported as
unavailable, never as a successful review.

The implementation contract is `WorkflowReview.Assess(Evidence, Assessment)
(Assessment, error)`. Evidence contains `Task`, named `Passages` and `Limits`;
Finding contains `Evidence`, `Quote`, `Departure` and `Correction`; Assessment
contains `Findings` and `Limits`. Assess preserves evidence and candidate limits
and copies returned data. Missing or unreadable context is described as a limit;
model failure remains an unavailable error. There is no approval field.

The application collects the current task's available evidence, invokes semantic
evaluation and the domain operation, and returns the assessment. Host adapters
translate events and reports. They do not own an independent review policy.
Editable evaluator instructions are shipped with ArcLint and remain separate
from native host protocol and deterministic finding validation.

This supersedes the blanket claim in integration-decisions.md that all review
behavior is delivery or workflow plumbing. Reporting an assessment is delivery;
deciding what its findings can claim is domain behavior. The librarian's
`application-service-holds-no-rule` test distinguishes them.

## Deliberate limits and verification

The new hook reports advisories. It introduces no comment ban, language exclusion,
timer, approval cache or automatic blocking. This is the narrowest behavior
supported by the reset's request to report on current work. Installation and
native activation remain separate facts; neither establishes successful review.

Verify domain rejection of invented evidence, explicit coverage limits and
advisory-only output; exercise the actual hook protocol with task evidence; then
evaluate real semantic responses for context, chronology, enforcement, ownership,
duplication and repair/rebuttal cases. Test an unrelated task and missing history
to check that the hook does not invent work. Record limits when the host does not
supply enough task activity. Repository checks validate implementation structure,
not the semantic truth of feedback.

## Removing the rejected implementation

The reset also removes the earlier guard as a distributed product feature. The
old Go-only/comment policy and approval lifecycle were agent-authored choices,
not an approved alternative. Labelling those commands legacy does not justify
shipping them. Remove their `agents hooks`, `agents setup` and `agents status`
commands and unused implementation. Keep the separately named reviewer and the
new task-focused workflow feature. Preserve existing installed copies, especially
the protected main-repository guard, as the user explicitly required. Historical
failure evidence remains history and is not current product guidance.

Native installation and event transport ports belong to their delivery caller.
They are not application use cases merely because their methods cross an adapter
boundary. The semantic ReviewWorkflow application operation remains responsible
for evaluation and domain acceptance. This correction removes misplaced interface
ownership instead of adding empty forwarding use cases or weakening lint.

## Quoting native action evidence

A repeat of the installed protocol test against implementation commit 65691d9
failed at the proposed-action event: a model quote did not match the JSON-escaped
native action passage, so domain acceptance correctly returned unavailable.
Keep this failure as evidence. Present decoded tool string content as named text
passages with its event/tool provenance, including actual newlines and symbols,
so a reviewer can cite the supplied proposal directly. Preserve action chronology
and coverage limits. Do not relax the exact-quote assertion or count an unavailable
assessment as a successful report. This is evidence presentation, not a changed
domain rule or a new approval policy.

## Linked-worktree native trust identity

Actual Codex 0.159.2 trust inspection showed that session-config hook definitions
use the same /<session-flags>/config.toml event/group key in both worktrees.
Trusting one root's definition therefore marked the other root's definition
modified. This was not an old guard rejection, and repeatedly approving two
conflicting definitions is not a recovery.

For a linked checkout, installation prepares one deterministic inventory of the
exact installed workflow providers in that Git worktree family. Both launchers
present the same ordered native groups, so each root has its own stable group key
within that inventory. Include no uninstalled provider or invented padding. Each
provider already rejects events outside its root before collection or evaluation.
Inherited host hooks remain intact, and trust remains Codex's normal explicit
flow. Adding or removing a sibling installation requires refreshing the earlier
launchers and reviewing changed native definitions; no trust is auto-renewed.
This is host delivery identity, not a new domain concept or review policy.
