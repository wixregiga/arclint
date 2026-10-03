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

The session must load the project's agent configuration and expose named
custom-agent invocation. Untrusted project configuration is not loaded.
Codex CLI 0.159.2 can hide the selector in its multi-agent tool schema. For that
version, a session-only configuration option exposes it:

```sh
codex -c features.multi_agent_v2.hide_spawn_agent_metadata=false
```

This controls tool metadata; it does not grant project or hook trust. Native
activity must show a spawn with `agent_type = "arclint-domain-reviewer"` and the
corresponding child role. A task name alone is insufficient. If the selector is
still absent, report the failure; do not claim a generic substitute was the
installed reviewer. Instruction behavior and native loading need separate evidence.

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

An Agent carries out a task through decisions and actions. A Coding Agent does
software work, including review. Its Agent Host provides the runtime and tools;
a Client is the interface through which the user interacts with it. An LLM is
the model component, not the whole running agent. A product may provide both
client and host roles. This distinction follows the
[ACP client role](https://agentclientprotocol.com/protocol/v1/overview#client);
ArcLint does not implement ACP.

Agent Instructions guide behavior. [AGENTS.md](https://agents.md/) carries
repository context and instructions. A [Skill](https://agentskills.io/specification)
packages task instructions in SKILL.md and optional resources. A
[Plugin](https://learn.chatgpt.com/docs/skills-and-plugins) packages capabilities
under a host's installation contract. This delivery is a Codex custom-agent
configuration, not a plugin.

[Codex custom-agent files](https://learn.chatgpt.com/docs/agent-configuration/subagents)
configure spawned sessions. They are not the running agent or its history.
Codex can apply parent runtime permission overrides over custom-agent defaults;
the read-only instruction and sandbox setting are not a security guarantee.

These meanings are recorded in the agent context of `domain.arclint.yaml`.
They do not require Agent or AgentHost value objects. Codex required fields
are checked by the adapter, and the supported host is checked by the installation
use case. Those delivery restrictions do not define what an Agent is.

## What a useful review establishes

A recording is a claim to examine, not proof that a person approved it. The
reviewer should trace a disputed meaning to the request or a decision, then
trace its obligation into code paths that can enforce it. For example, a
method named `EnsureUniqueID` on one Rule cannot establish uniqueness among
all Rules unless it actually has access to the required collection or boundary.

A useful response identifies the unsupported decision or bypass, explains the
consequence, proposes a correction, and revisits it when given a repair or
rebuttal. Matching names and green tests alone are not enough. The reviewer
can also conclude that no concern is supported and explain its coverage limits.
