# Task-focused workflow review design

The workflow hook reports specific departures from the domain workflow in the
current task. It is a reusable feature for any adopting project. ArcLint uses
the same public commands and contracts for its own development, without a
separate project policy or hard-coded source scope.

The workflow obtains ArcLint context before reading source, explains and records
changed meanings before implementation, verifies enforcement, examines ownership
and unjustified duplication, and reassesses repairs and rebuttals. Reports
provide evidence and suggestions. Independently installed guards retain their
own configuration and lifecycle.

## Domain and implementation responsibility

`WorkflowReview.Assess` owns acceptance of the assessment. Under
`domain-service-when-no-owner`, this is a domain service: assessing a task
against supplied meanings and implementation is a significant decision no
existing aggregate owns. `contract-of-a-service` places its guarantees under
assertions. This does not require an Agent entity, installation identity,
consistency aggregate or comment policy.

Evidence, Finding and Assessment are supplied snapshots and review results
whose meaning depends on their contents. Equal contents are interchangeable
under `value-test`; these values need no artificial constructor invariants.
They establish no identity model for Agent.

Semantic evaluation examines the supplied task, established domain, affected
work, context and activity, earlier findings and repairs. It identifies concrete
departures and suggestions. It cannot infer missing chronology from a code
snapshot, invent additional work or treat an agent's previous claim as user
approval. Missing evidence produces an explicit coverage limit, not a
demonstrated violation. Representative cases evaluate judgment; exact quotes
alone cannot establish that judgment is correct.

`Assess` deterministically requires a task and validates each candidate's named
evidence, exact quote, specific departure and correction. It returns accepted
findings and explicit missing-evidence limits. These checks establish grounding,
not semantic correctness. A malformed or unsupported assessment is reported as
unavailable, never as a successful review.

The contract is `WorkflowReview.Assess(Evidence, Assessment) (Assessment, error)`.
Evidence contains `Task`, named `Passages` and `Limits`; Finding contains
`Evidence`, `Quote`, `Departure` and `Correction`; Assessment contains `Findings`
and `Limits`. Assess preserves evidence and candidate limits and copies returned
data. Missing or unreadable context is a limit; model failure is an unavailable
error. There is no approval field.

The application collects available task evidence, invokes semantic evaluation
and the domain operation, and returns the assessment. Host adapters translate
events and reports. Editable evaluator instructions ship with ArcLint and remain
separate from native host protocol and deterministic finding validation.

Reporting an assessment is delivery; deciding what its findings can claim is
domain behavior. The librarian's `application-service-holds-no-rule` test
distinguishes them. Native installation and event transport ports belong to
their delivery caller. Crossing an adapter boundary does not make those ports
application use cases; ReviewWorkflow owns evaluation and domain acceptance.

## Limits and verification

Reports are advisory. The hook introduces no comment ban, language exclusion,
timer, approval cache or automatic blocking. Installation, native activation
and successful review require separate evidence.

Verify rejection of invented evidence, explicit coverage limits and advisory
output through the domain contract and actual hook protocol. Evaluate semantic
responses for context, chronology, enforcement, ownership, duplication and
repair or rebuttal cases. Include unrelated tasks and missing history to check
that the hook does not invent work. Record coverage limits when host activity
is incomplete. Repository checks validate structure, not the truth of feedback.

## Native action evidence

Native action passages include decoded tool string content with event and tool
provenance, preserving actual newlines and symbols. A quote of decoded action
text can differ from its JSON-escaped representation and fail exact-quote
validation. Named decoded passages let findings cite supplied actions directly
while preserving chronology and coverage limits. Exact-quote validation remains
required; an unavailable assessment is not a successful report.

## Linked-worktree host trust

Codex 0.159.2 identifies session-config hooks by a
`/<session-flags>/config.toml` event/group key. Different worktree definitions
under the same key can mark one another as modified.

For a linked checkout, installation prepares one deterministic inventory of
the exact installed workflow providers in its Git worktree family. Launchers
present the same ordered native groups, giving each root a stable group key
within that inventory. No uninstalled provider or padding is included. Each
provider rejects events outside its root before collection or evaluation.

Inherited host hooks remain intact. Adding or removing a sibling installation
requires refreshing earlier launchers and reviewing changed native definitions
through Codex's explicit trust flow; trust is never automatically renewed.
This is host delivery identity, not a domain concept or review policy.

## Configuration and installation validation

Installed hooks retain the rules file selected with `--rules`; the collector
supplies that guidance rather than assuming a conventional filename. Malformed
installation metadata returns a configuration error before writes, preserving
existing files. These are delivery validation duties, not domain invariants.
Regression tests cover both paths.
