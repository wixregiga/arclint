# Corrective acceptance audit — 2026-10-03

This audits the current correction against user evidence in decisions.md.
The earlier issue checklist, completion report and green checks are not approval
of the rejected definitions. Status below separates behavior from mechanics.

| Requirement | Current implementation or evidence | Assessment |
| --- | --- | --- |
| Agent, Coding Agent, Agent Host, Client, LLM meanings | agent context; direct decisions and primary sources in decisions.md | System, role, runtime, user interface and model distinguished; no invented identity/equality claim. |
| Instructions, AGENTS.md, Skill, Plugin meanings | agent context and public reviewer documentation | Instructions, conventions and packages distinguished from runtime/configuration/session. |
| Remove unsupported model and dependencies | Deleted domain/agent package, 4 invariants, empty code zone and conformist relation | Actual removal, with delivery validation retained in application/adapter. |
| DR-1 through DR-7 | Authored reviewer instructions and cases 1–7, 11–14 | Pre/during grounding, language, enforcement, ownership, duplication and reassessment encoded; direct instruction sample met 13 case expectations; case 11 detected both defects but its suggested correction was less complete. Native named execution remains unverified. |
| DR-17, chronology | Instructions and case 6 | Missing history must be reported as unverified, not invented late recording. |
| DR-18, feedback-only | Instructions, configured sandbox default, case 8 | Behavioral obligation; parent host permission overrides mean TOML is not proof of isolation. |
| DR-8, editable separate instructions | Authored TOML under Codex assets; guard files unchanged | Reviewer stays separate from installed guard. |
| DR-9, DR-13, installable in Codex | Application use cases, real Codex adapter and CLI assembly tests | Installation/status tested; native attempts did not spawn the reviewer; evaluation.md records the missing selector and separate instruction evaluation. |
| DR-10, DR-11, DR-12, shared release | Embedded TOML and reviewer.json ArcLint version | No independent agent version; install, status and upgrade tests. |
| DR-14, meaningful verification | Application port isolation/error tests, real adapter and CLI tests, behavioral cases | File mechanics do not prove semantic judgment; see current evaluation. |
| DR-15, activity evidence | Reviewer returns scope, evidence, concerns and limits in host conversation | ArcLint owns no separate retention policy; actual direct-instruction response and host trace retained; named reviewer activity remains unverified. |
| DR-16, documentation | Public reviewer page, contributor guide, decisions, evaluation | Invocation, ownership and effective permission limits documented. |
| Supporting librarian and generated assets | Authored skill protocol/distillation; generated SKILL.md, VOCAB.yaml, AGENTS.md, classifying page | No forced tactical classification or universal must-to-invariant conversion. |
| Preserve scope and existing work | Before/after hashes of initial 8 files, including main guard and local installation | All 8 byte-identical on comparison after implementation. |
| No timers or automatic blocking | Explicit invocation; installer does not write hooks or host policy | Existing guard lifecycle stays separate. |
| Required checks | Repository finish gate and architecture/rule fixtures | make check, make ci (race suite), architecture check, 48 rule fixtures and documentation build passed; independent make check-ro passed. |
| Independent semantic/code review | Astra reviewed evidence, model, instructions, implementation and tests | Unsupported conformist relation found and removed; final review found no supported blocking defect. This is review evidence, not user approval. |
| PR #80 and issue #81 | Descriptions must reflect corrected implementation and measured limits | Update after final evidence. |

## Architecture limits

The architecture check has 26 existing baselined findings, seven stale baseline
entries, two unsupported infrastructure subjects and disabled Web launch-surface
coverage. Baselines and rule strength were not changed to hide findings.
Existing adopted findings concern rule/adoption context overlap and resulting
imports, plus five Rule invariant method findings. They are outstanding repair
work; this correction does not establish their semantics or enforcement.
No baseline finding selects the corrected reviewer delivery or skill files.

The context prose records shared meanings without tactical entries. ArcLint's
current schema does not machine-check those prose meanings or prove reviewers
follow them. Behavioral sampling is limited evidence, not a universal quality
claim. The historical e06a978 evaluation is not verification of current assets.

## Host invocation limit

Installation/status succeeded in fresh temporary projects, with the installed
TOML hash matching the authored asset. Two native attempts (default and an
invocation-only multi_agent_v2 feature override) returned without spawning the
named reviewer. A bounded diagnostic reported the exposed spawn fields as
`task_name`, `message`, `fork_turns`, `model`, `reasoning_effort`, with no named
custom-agent selector. The traces contain no named spawn. This establishes a
verification gap in this session interface, not a proven installer defect or a
universal statement about Codex support. No feature override is recommended by
this evidence. Named activation and a named review are not claimed.
