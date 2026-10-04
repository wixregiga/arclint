# Agent language and reviewer design

## Product requirements

ArcLint provides a separately invoked domain reviewer and task-focused workflow
hooks. Reviewer instructions are editable assets shipped with the CLI, rather
than policies that require Go changes. Installation commands, release versioning,
tests and documentation support their delivery through a host contract.

The reviewer questions proposed meanings before and during implementation, then
examines enforcement, duplication and responsibility after implementation. Its
feedback identifies evidence and returns to the caller through the host.
Reviewer obligations, contributor duties and host behavior have separate
contracts in [requirements.md](requirements.md) and
[development.md](development.md).

## Reference specifications and boundaries

- [OpenAI agent loop](https://openai.com/index/unrolling-the-codex-agent-loop/):
  the runtime coordinates model responses and tool execution. The Agent/LLM
  distinction below is an inference from this separation, not an identity model.
- [ACP client and agent roles](https://agentclientprotocol.com/protocol/v1/overview):
  the client interfaces users with agents. ArcLint uses this role distinction
  without implementing ACP or assuming all protocols use Client this way.
- [Codex custom agents](https://learn.chatgpt.com/docs/agent-configuration/subagents):
  TOML configures spawned sessions. The running system, configuration and
  conversation are distinct. Parent runtime permissions can override the file's
  sandbox default.
- [AGENTS.md](https://agents.md/): repository context and instructions.
- [Agent Skills specification](https://agentskills.io/specification): a directory
  with SKILL.md and optional resources; not an independently running agent.
- [Codex skills and plugins](https://learn.chatgpt.com/docs/skills-and-plugins):
  plugins package capabilities under a host contract. A custom-agent TOML file
  alone is not a plugin; this feature uses custom-agent configuration.

Agent, Coding Agent, Agent Host and LLM name the system, its task specialization,
its runtime provider and its model component respectively. Client names the
interface through which a user interacts with an agent, independently of
whether it calls ArcLint. These are working meanings for this context, not a
universal ontology. Their definitions live in `domain.arclint.yaml`.

## Classification and ownership

Equal configuration text does not establish interchangeable running agents or
hosts. The librarian's `value-test` is therefore not established for Agent or
Agent Host. Neither `identity-test` nor `transaction-boundary` establishes a
need for ArcLint to track their lifecycle or model them as aggregates. Their
meanings remain context definitions without tactical entries or corresponding
Go types; they are explicit prose, not machine-checked terms.

Under `language-not-guards` and `not-a-domain-rule`, required TOML fields,
supported host options and delivery filenames are installation validation,
not invariants of all Agents or Agent Hosts. The application rejects unsupported
host requests before invoking a port; the Codex adapter validates its authored
configuration. The agent context has no code zone. Reusing classification
guidance does not establish a conformist vocabulary-to-agent relationship:
the reviewer must challenge unsupported recordings rather than accept them
unchanged.

Instructions implement reviewer behavior and require behavioral evaluation.
Their existence or a matching method name does not prove correctness. The
feature introduces no timers, automatic blocking, plugin packaging or agent
lifecycle model. The workflow service and delivery boundaries are described in
[workflow-design.md](workflow-design.md).

## Verification

Verify CLI installation and status, rejection of an invalid host before writes,
malformed configuration, preservation of local edits and independently installed
guard assets. Evaluate the current instructions on unsupported recordings and
ceremonial enforcement, withholding expected outcomes. Record inputs, responses,
instruction hashes and limitations. Repository checks and baseline results
provide structural evidence; they do not establish semantic correctness.
