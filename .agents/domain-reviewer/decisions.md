# Corrected agent meanings — 2026-10-03

This decision is recorded before corrective implementation. It supersedes the
Agent and AgentHost value-object choices in e06a978, not unrelated domain entries.

## Evidence and authority

The current user request rejects defining Client by ArcLint consumption and
silently equating Agent with its installation file. It explicitly authorizes
removing unsupported classes, value objects, aggregates and invariants.

In the original chat `01a0f90b-eeab-7d52-8073-8d840c2978ec`:

- Message `01a0fcd7-9c24-7580-8ec3-f7a10acec6d6` says “Yes, Coding Agent,
  Agent Host, LLM, Agent, Client, Skill, Plugin” belong in the common language.
  It requires CLI commands, versioning with ArcLint, embedded/generated delivery,
  tests, logging and documentation; different host routes are a delivery concern.
- Message `01a1002f-6c77-7bc0-8226-e5d76cefb5d1` asks “are these requirements
  for the end user or for you writing the the agent? those are two separate contexts”.
- Messages `01a0fa3e-4608-7930-a1d8-26350f6a6421` and
  `01a0fa41-6191-79d3-b0e2-a6386f32f341` request a separate reviewer and editable
  agent files instead of repeatedly editing Go to change review instructions.
- Message `01a0fa3c-7443-7d63-ba1b-2e1eaffaa0fa` requests questions and suggestions
  before/during work, human-readable definitions, and examination of enforcement,
  duplication and misplaced responsibilities afterward.

The critique is defect evidence. Issue #81, the earlier checklist and evaluation
are implementation claims, not approval of meanings. The chat “Domain Reviewer
Purpose” contains an assistant interpretation, not an additional user decision.

## Primary specifications and boundaries

- [OpenAI agent loop](https://openai.com/index/unrolling-the-codex-agent-loop/):
  the runtime coordinates model responses and tool execution. The Agent/LLM
  distinction below is an inference from this separation, not an identity model.

- [ACP client and agent roles](https://agentclientprotocol.com/protocol/v1/overview):
  the client interfaces users with agents. We use this role distinction without
  claiming that ArcLint implements ACP or that all protocols use Client this way.
- [Codex custom agents](https://learn.chatgpt.com/docs/agent-configuration/subagents):
  TOML supplies a configuration layer for spawned sessions. The running system,
  configuration and conversation are distinct. Parent runtime permission
  overrides can take precedence over the file's sandbox default.
- [AGENTS.md](https://agents.md/): repository context and instructions.
- [Agent Skills specification](https://agentskills.io/specification): a directory
  with SKILL.md and optional resources; not an independently running agent.
- [Codex skills and plugins](https://learn.chatgpt.com/docs/skills-and-plugins):
  plugins package capabilities under a host contract; a lone custom-agent TOML
  is not a plugin. This feature selects no plugin packaging route.

Agent, Coding Agent, Agent Host and LLM name the system, its task specialization,
its runtime provider and its model component respectively. These are working
meanings for this context, not claims that a protocol standard mandates a
universal ontology. The concise definitions live in domain.arclint.yaml.

## Classification and ownership decisions

The librarian's `value-test` is NOT ESTABLISHED for Agent or Agent Host: equal
configuration text does not establish interchangeable running systems. The
previous model tested a different thing (a configuration), then named it Agent.
The `identity-test` and `transaction-boundary` supply no evidence that ArcLint
needs to track an agent lifecycle or an aggregate. No replacement tactical
classification is made. The existing schema permits context definitions without
tactical entries; these meanings are explicit prose, not machine-checked terms.

Under `language-not-guards` and `not-a-domain-rule`, TOML required fields, a
supported host option and the delivery filename are installation validation.
They are not invariants of all Agents or Agent Hosts. Remove the unsupported
Agent/Host types, four invariants and their tests. Keep validation at the real
application/adapter boundaries: the application rejects unsupported host
requests before invoking a port, and the Codex adapter validates its authored
configuration. Remove the now-empty agent code zone; preserve the agent context.
Reuse of classification guidance does not establish a conformist context-map
relationship. Remove vocabulary-to-agent rather than claim the reviewer accepts
recordings unchanged; it must challenge unsupported recordings.

Reviewer obligations remain in requirements.md; contributor duties remain in
development.md; host and installer guarantees are documented separately.
Instructions implement reviewer behavior and deserve behavioral evaluation,
but their existence or a matching method name does not prove correctness.
Do not add timers, blocking, plugin delivery or an invented lifecycle model.

## Required verification

Check real CLI installation/status, invalid host before writes, malformed
configuration, preservation of edits and separate guard assets. Evaluate the
current instructions on cases that challenge unsupported recordings and
ceremonial enforcement, with expected outcomes withheld. Record exact inputs,
responses, instruction hash and limitations. Run required repository checks and
inspect the baseline without treating green results as semantic approval.
