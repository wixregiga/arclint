# ArcLint Domain Reviewer requirements

These are requirements for people using the reviewer. They describe what the
reviewer owes its caller. They are separate from a feature's requirements or a
Jira/Linear ticket. The DR labels identify this reviewer’s requirements.

ArcLint Domain Reviewer helps people and coding agents keep a change grounded
in the project's domain. It reviews supplied requests, discussions, domain
recordings and implementation. People decide disputed meanings; the builder
makes the changes. The reviewer gives evidence and suggestions.

A request is the caller's desired outcome or instruction. Behavior is what the
reviewer does; an obligation says what it must do. A requirement records an
accepted behavior or constraint, and a case supplies concrete conditions and
an expected result. These words do not introduce ticket identities or objects
that ArcLint stores.

## Review behavior

DR-1. When invoked before or during implementation, the reviewer shall check
that proposed concepts and behavior follow the request and established domain.

DR-2. When reviewing a proposed domain recording, the reviewer shall check that
its intended meaning has been explained for discussion.

DR-3. When reviewing implementation of new or changed domain meaning, the
reviewer shall check that the corresponding decision was recorded first.

DR-4. When a decision lacks support or wording obscures a meaning, the reviewer
shall give a warning, question or suggestion with evidence in plain language.
A recording, checklist or previous agent report is a claim to examine, not
independent evidence that the user approved the meaning.

DR-5. When invoked after implementation, the reviewer shall examine supplied
code for missing enforcement, unjustified duplicate decisions and misplaced
domain responsibilities.

DR-6. When given a repair or rebuttal, the reviewer shall reassess the concern
against the current evidence.

DR-7. The reviewer shall distinguish demonstrated conflicts, suspected
problems and missing evidence.

DR-17. When evidence cannot establish the order of discussion, recording and
implementation, the reviewer shall state that the order is unverified.

DR-18. The reviewer shall give feedback without editing the reviewed files or
making a disputed domain decision on the user's behalf.

DR-15. Each review shall identify its evidence and return its findings to the
caller, so the host's conversation records its activity.

## Installer and host behavior

DR-9. ArcLint shall provide a CLI command to install the reviewer.

DR-10. The installed reviewer shall identify the ArcLint release that shipped it.

DR-11. The reviewer shall share ArcLint's release version.

DR-13. ArcLint shall deliver the reviewer through a host installation contract,
with Codex as the first supported Agent Host.

DR-16. ArcLint shall document installation, invocation and review limitations.

## Scope of this delivery

The reviewer runs when explicitly invoked. The caller supplies the request,
review stage, affected paths, decisions and any earlier feedback or rebuttal.
Its feedback belongs to the host conversation. Dedicated log files, automatic
invocation, timed reminders, blocking hooks and task creation are future
choices; this delivery does not claim those behaviors.

The installed domain guard continues to have its own lifecycle. Installing or
invoking this reviewer does not replace the guard or its review state. Codex
host discovery and activation are separate from writing installation files.

Agent instructions can implement product obligations. Their meaning belongs
in the shared domain language even when their implementation is a text file.
The contributor obligations for authoring, shipping and verifying that file
are in [development.md](development.md). Behavioral examples are in
[cases.md](cases.md).

## Basis of these requirements

[decisions.md](decisions.md) traces the retained behavior to direct user
statements and primary specifications. DR labels identify these obligations;
they do not establish approval by themselves. The current correction removes
unsupported tactical classifications while preserving the requested behavior.
The host may override a custom agent's permission defaults, so feedback-only
behavior is an obligation to evaluate, not a security guarantee from TOML.
