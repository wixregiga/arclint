---
title: Domain Reviewer
weight: 10
---

ArcLint Domain Reviewer helps people and coding agents keep proposed changes
connected to the project's domain. It can review the plan before work, revisit
new decisions during work, and look for missing enforcement after work.

Install it in the project:

```sh
arclint agents reviewer install --host codex
arclint agents reviewer status
```

The first delivery supports Codex. The installation writes
`.codex/agents/arclint-domain-reviewer.toml` and records its ArcLint release in
`.arclint/reviewer.json`. The reviewer shares the CLI's release version.
Existing local edits are preserved. Installation status reports the files and
version; it does not establish that Codex has loaded the agent or that a review
has passed. Start a fresh Codex session after installation so it discovers the
project's agent configuration.

Ask Codex to use the named agent:

> Use arclint-domain-reviewer to review this change before implementation.
> The request is … The affected paths are … These are the decisions we have
> discussed …

Use the same request during or after implementation, stating the review stage
and supplying the current plan or changes. Include earlier feedback and any
repair or rebuttal when asking for reassessment. The reviewer runs when invoked.
This installation does not add an automatic hook or reminder.

Each review names what it examined and returns warnings, questions or
suggestions with evidence and a small next step. It distinguishes an observed
conflict from uncertainty or missing material. Those responses form a review
record in the Codex conversation; Codex owns that session history. ArcLint does
not write a separate review log or promise how long the host keeps it.

People decide disputed meanings. The instructions require the reviewer to
give feedback without editing files or approving new meanings on their behalf.
The configuration requests Codex's read-only sandbox; the host controls the
effective tools and permissions. Installing this file does not by itself prove
that a particular session enforces file isolation. A matching domain file and implementation do not
prove that the domain was discussed and recorded first. Tests and architecture
checks provide their own evidence; the reviewer should state what they do and
do not establish.

The existing domain guard remains independently installed. The guard's hooks
and review state are separate from this named reviewer.

## The shared language

An Agent is an assistant configured to carry out a stated responsibility. A
Coding Agent performs work on software. An Agent Host loads its configuration
and provides its tools and execution environment. A Client is the software a
person uses to interact with a capability; it can also be an Agent Host. An LLM
is the language model an agent uses.

Agent Instructions tell an agent how to work. `AGENTS.md` carries project
instructions, including ArcLint's generated architecture guidance and the
project's maintained contributor guidance. A Skill packages instructions and
supporting material for a task. A Plugin packages capabilities according to a
host's specification. These words describe different responsibilities; a
provider's filenames do not make them interchangeable.

An instruction file can implement obligations that matter to people. Those
meanings still belong in the recorded domain. Recording them does not require
inventing a Go class for every word in this vocabulary.
