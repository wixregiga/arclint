# Integration audit — 2026-10-03

> Historical record. The user reset this work to task-focused workflow hooks.
> Current decisions and evidence are in [workflow-recovery.md](workflow-recovery.md)
> and [workflow-audit.md](workflow-audit.md). Rejected guard behavior below is
> not a current product contract or evidence of user approval.

Audit target: implementation commit `5db6d0b06d4b518c1da09721725395aaf2506ea2`.
The current named reviewer has successfully completed native reviews in both
requested checkouts. The whole integration remains incomplete: autonomous repair
under the corrected native protection hooks has not been observed in either
checkout. Installation, protocol fixtures and subprocess review are distinct
from that missing evidence.

## Meanings and responsibilities

[decisions.md](decisions.md) traces Agent, Coding Agent, Agent Host, Client, LLM,
Agent Instructions, AGENTS.md, Skill and Plugin to direct user statements and
primary specifications. Agent is the performing system; Client mediates the
user's interaction; the Agent Host supplies runtime and tools; the LLM supplies
the model component. Instructions, packaging, host configuration and running
instances have distinct responsibilities. These are contextual working meanings,
not a claim that one specification mandates a universal ontology.

The rejected Agent/AgentHost value objects, installation-derived invariants,
Go implementation and unjustified conformist relationship were removed. No
replacement tactical objects were manufactured. Established meanings are recorded
in context prose; that prose is not machine-checked tactical vocabulary.

[integration-decisions.md](integration-decisions.md) records repair, containment,
setup, output, reviewer calibration and independent-provider state ownership before
implementation. Existing inward dependency and renderer boundaries suffice; no
rule or baseline relaxation is justified. Reviewer obligations are in
[requirements.md](requirements.md), contributor duties in [development.md](development.md),
and installer/host behavior in the public documentation. Typed installation results
carry observations and introduce no domain identity, aggregate or invariant.

## Reviewer and delivery requirement audit

Every DR obligation in the current requirements and contributor document is
accounted for below. Labels and prior checklists remain claims to audit; they
are not independent user approval.

| Obligation | Current evidence | Result and limit |
| --- | --- | --- |
| DR-1: keep proposals grounded before/during implementation | Current instructions and native cases 1, 4, 8 and 13 | Both current samples question unsupported meanings and accept justified implementation details without inventing objects. Future invocations are not guaranteed. |
| DR-2: examine whether proposed meanings were explained | Decision records and native case 1 | Both samples question the unexplained second Scope. Supplied fictional evidence does not establish every real discussion. |
| DR-3: examine recording before changed-meaning implementation | Instructions and native cases 1 and 9 | Both distinguish current agreement from actual sequence evidence. They do not manufacture chronology violations from missing history. |
| DR-4 and DR-7: evidence, understandable correction and uncertainty | Native cases 2, 6, 10, 11 and 13 | Both distinguish unsupported claims, conflicts and coverage limits; recording and green checks do not establish approval. |
| DR-5: actual enforcement, capable owners and unjustified duplication | Native cases 3, 5, 7, 11 and 12 | Both identify bypasses, collection-level ownership and ceremonial methods; different rounding decisions justify separate code. Unsupported recording and solely dependent implementation are both removed in the recommendations. |
| DR-6: reassess repairs and rebuttals | Native cases 5 and 14 | Both withdraw unsupported duplication concerns and identify the remaining CLI bypass after the HTTP repair. |
| DR-17: qualify unavailable history | Current native case 6 and calibrated instruction change | Both report coverage limits without demanding proof of an unstated obligation. Case 9 separately questions an actual chronology claim. |
| DR-18: feedback without editing or deciding for the user | Current instructions, native case 8 and retained child activity | Feedback-only behavior is observed for supplied reviews. TOML defaults and instructions alone do not guarantee effective host isolation. |
| DR-15: identify evidence and return findings/activity | Native child responses, role metadata, parent results and retained activity excerpts | Both current reviewers returned findings to the caller and both parent processes completed with exit 0. Codex owns conversation retention; ArcLint does not provide an additional log service. |
| DR-8 and DR-12: editable instructions shipped separately in ArcLint | Authored embedded TOML, independent reviewer installer, generated librarian assets and drift tests | Reviewer instructions are editable and separate from the existing guard. No Go model exists solely to hold the agent vocabulary. |
| DR-9: CLI installation | Real CLI/application/adapter tests and target status records | Separate reviewer installation is verified. Installation does not imply native protection-hook activation. |
| DR-10 and DR-11: shared release identity | Installed metadata/status and actual CLI version tests | Reviewer shares ArcLint `0.1.0-beta.1`; no separate reviewer release was invented. |
| DR-13: native host installation with Codex first | Installed configuration, successful named spawn, child role and exact loaded instructions in both targets | Current reviewer native loading, invocation and completed feedback are verified in Codex 0.159.2. Host selection visibility was explicitly exposed; no trust was granted. |
| DR-14: installation and behavior verification | Real assembly tests, native samples, unchanged cases and withheld expectations | Both current samples meet all fourteen expected outcomes. This is sample evidence, not a review of the entire implementation or universal reliability. |
| DR-16: installation, invocation and limits documented | Public documentation, requirements/development records and evaluation evidence | Documentation distinguishes installed files, host loading, invocation, review judgment and protection. |

## Original request and whole-integration audit

| Requested outcome or constraint | Current evidence | Result and limit |
| --- | --- | --- |
| Inspect worktree, PR #80, critique, requirements and relevant history; audit inherited claims | Decision records cite direct message IDs and primary specifications; critique retained; independent implementation/semantic review | Unsupported prior definitions, issue #81 and completion claims were examined rather than accepted as approval. PR #80 and issue #81 track the corrected result and remaining native-protection gap; this audit supplies the evidence for their update. |
| Follow AGENTS.md, context-first workflow and domain-librarian; record justified decisions before corrections | Recorded decisions, generated guidance/skill, source history and required checks | Corrections use established boundaries. Classification no longer forces understood vocabulary or all promises into tactical objects/invariants. |
| Keep reviewer promises, contributor duties and actual delivery clear | Separate requirements/development documents, installation status, named invocation and host discovery | These are separate evidence obligations; a matching instruction file or status result closes only its own claim. |
| Preserve separate reviewer, editable versioned assets, install/status, docs, activity and meaningful tests | DR audit above; native manifests and CLI tests | Observed for the current reviewer. Reviewer use is explicitly invoked and is separate from guard approval. |
| Question meanings before/during work and examine enforcement/ownership/duplication afterward; reassess repairs/rebuttals | Current native fourteen-case reports in both targets | All fourteen expected outcomes observed in each sample; remaining domain correctness requires evidence on each real review. |
| Preserve `/home/jofyi/ai/arclint/.codex/hooks/arclint-domain-guard/guard.py` and its policies | Initial preservation manifest, provider proof and current direct SHA256 comparison | Original remains `e5e271821d73723a48db33af21421a22b12ae123e1c54db459704ecd1cb0c34e`. Worktree assets were not substituted for it. |
| Preserve unrelated/user-owned changes and active checkout | Initial eight-file preservation record and current tracked status/HEAD observation | 30f1 remains at `cf40d1b5a36ec0d9598d14210074c10a48d0157f` with no tracked changes. Requested new integration files were installed separately. The final manifest rehashes all eight initially protected/local files and confirms each unchanged. |
| Legitimate governance rejection and fresh-session recovery | Earlier live hook proof restores protected bytes, starts a new session and obtains fresh review | Demonstrated through real subprocess hook/reviewer activity. No endless rejection retries, bypass flag or installed-main policy weakening. The first 5db attempt correctly rejected contradictory fixture meanings at final review. After removing the unsupported fixture classification, the same binary passed all repair/restoration/governance/fresh-session steps. Both attempts are retained. |
| Actual finding repair and missing scoped-file restoration | Python 41 tests, OMP 34 tests, earlier real reviewer subprocess activity | Permission precedes mutation, missing material remains required and post/final review is fresh. Harness fixture mutation is verified; autonomous repair through corrected native hooks in both target checkouts is unverified. |
| Protection during repairs | Protected/mixed/outside edits, traversal/symlink/stale-result tests and governance invalidation | Permission is not acceptance of resulting content. Dedicated inspection cannot renew invalidated approval. Native corrected-hook protection remains unverified. |
| Worktree routing and independent provider state | Event-cwd/scope tests and provider-isolation subprocess proof | Target checkout owns scope/state; absent target configuration does not fall back across a repository boundary. Copied preserved and corrected providers retain independent snapshots in either start order. |
| Native Windows containment | Earlier native Windows Go five scope tests and OMP 34 tests, zero skips | Files/OMP sources are unchanged from verified 3575d29 to 5db6d0b. This is unchanged-source evidence, not a claim of an exact-5db Windows rerun. Native Windows Python/Codex remains unsupported; Linux/WSL is supported. |
| Setup validates before writes and preserves existing material | Full-tree before/after failure tests, missing scope/empty globs, external files, managed conflicts, AGENTS markers and unsupported hosts | Preflight precedes setup writes and adapters recheck. No crash-atomic multi-file guarantee. |
| JSON reports actual facts | Complete stdout parsed for real setup/status/hooks/reviewer commands and all renderer tests | Empty/missing/invalid receipts, omitted required assets and unrelated hook registrations cannot certify intact installation. No activation/trust inference. |
| Corrected native protection in active and fresh checkout | Exact 5db installed hashes and additive host discovery in both targets | Files are installed; five corrected additive definitions are enabled but untrusted. Neither autonomous protected repair nor native corrected-hook activation is verified in either target. |
| No unagreed timers, automatic reviewer blocking or unrelated features | Reviewer instructions/install separation and scoped diff | Named reviewer runs when invoked. Existing guard timeout/retry policies remain; no reviewer timer, plugin route, unrelated feature or trust bypass added. |
| Required checks and independent review | Exact 5db CI, Python, fixture and docs logs; previous check/review records | Final `make ci` passed; Python 41, rule fixtures 48 and docs passed. Green checks do not establish meanings or native hook activation. The final bundle retains the legitimate failed fixture review and successful corrected-fixture exercise separately. The latter does not establish native host activation. |
| Update PR #80 and supporting issue with corrected result | Current remote metadata inspected during this audit | Final descriptions link this audit and evidence and retain native protected repair as open. Earlier issue/checklist text remains no independent proof of approval. No merge. |
| Audit every requirement before claiming completion | This document and linked evidence | Reviewer requirements have current sampled support; the whole requested integration is not complete while the native protected repair remains unobserved. |

## Current native reviewer evidence

The current authored and installed reviewer SHA256 is
`9860455e9b6ec58456e4257b4da80f122df2fc658e08ef266ee49aed79f00201`.
Its source is byte-identical from 2532422 to 5db6d0b. See the
[30f1 manifest](evidence/2026-10-03-calibrated-native-30f1/manifest.json) and
[actual report](evidence/2026-10-03-calibrated-native-30f1/reviewer-response.md),
and [fresh manifest](evidence/2026-10-03-calibrated-native-fresh/manifest.json)
and [actual report](evidence/2026-10-03-calibrated-native-fresh/reviewer-response.md).
Both show successful native named spawn, child role, current instructions loaded,
complete reviewer feedback and parent exit 0 without timeout or hook-rejection
prompts. Expected outcomes were withheld. Manual assessment against unchanged
cases found 14/14 in each; the reports themselves support that assessment.

Earlier 12/14 initial samples and their timed-out parent processes remain retained.
They exposed the chronology overreach and incomplete ceremonial-code correction;
those decisions were recorded before instruction repair. The current successful
samples supersede those gaps for this particular evaluation, without erasing
historical failures or guaranteeing every future review.

## Guard state and exact implementation evidence

The [provider-isolation proof](evidence/2026-10-03-provider-isolation/proof.json)
executes copied preserved and corrected scripts as real subprocesses, in both
start orders. Their state paths are distinct, unchanged PostToolUse does not
invalidate either snapshot, and a corrected-script content update retains the
same cache path while invalidating prior approval. Original/corrected file hashes
are unchanged after that proof. This uses protocol fixture evidence, not a model
review, native hook activation or a changed preserved installation.

Current corrected guard SHA256 is
`2b36529d5e10eab15dcc1c7c926889318cde1ff306583a9aa3cb99fdad3861cd`.
The [30f1 discovery](evidence/2026-10-03-integration/activation-30f1.json)
and [fresh discovery](evidence/2026-10-03-integration/activation-fresh.json)
identify source 5db6d0b, installed guard/reviewer bytes and exact additive hook
configuration. Discovery attempted no execution and modified no trust.
The host's normalized handler hashes do not hash Python script bytes: host trust
and ArcLint asset integrity are different checks.

The [3575d29 manifest](evidence/2026-10-03-integration/manifest.json) and linked
logs describe the earlier exact implementation checks and real-reviewer live
hook subprocess proof. They still record the earlier reviewer hash and Python 40;
they are not evidence of a final-5db rerun. The [final manifest](evidence/2026-10-03-final-integration/manifest.json) retains
exact 5db CI, Python 41, 48 fixtures, docs and 6ecaa2f finish-gate logs. Product
implementation is unchanged from 5db6d0b to 6ecaa2f; the latter corrects only the
probe fixture and records its reason.

The [first final attempt](evidence/2026-10-03-final-integration/live-hook-inconsistent-fixture/manifest.json)
correctly rejected contradictory fixture meanings at fresh-session Stop: Room was
both a named bookable space and the name of that space. Its earlier comment repair
and missing-file restoration each obtained fresh post-tool and Stop passes.
Independent review confirmed the contradiction. The fixture's tactical
classification was unnecessary for the exercised behavior and unsupported by any
equality/identity decision; it was removed while retaining context meaning and
implementation. No guard rule, policy, scope or expected outcome changed.

The [corrected-fixture attempt](evidence/2026-10-03-final-integration/live-hook-corrected-fixture/manifest.json)
uses the same binary and guard bytes. It passed the entire sequence: finding,
permitted comment repair, fresh post/final pass, missing-file rejection, permitted
restoration, fresh post/final pass, governance invalidation, restored original
bytes, fresh session and final pass. Every event, response and state is retained.
This is actual production semantic review through installed hook subprocesses;
the harness makes fixture edits. It remains distinct from an autonomous agent
repairing through corrected native outer-host hooks.

## Remaining host step and limits

The domain-librarian says “Do not grant host trust yourself.” No trust was granted
or bypassed. Ordinary user trust through `/hooks`, followed by a fresh session,
is required before testing the corrected additive hooks in the native host.
The preserved main provider remains separately installed; its PreToolUse definition
is disabled/trusted and four other definitions are enabled/trusted. Those settings
were not changed to obtain a pass.

Installed target scope is explicitly `domain.arclint.yaml`, with no source
patterns. Do not claim whole-repository code protection. Autonomous protected
repair in both requested checkouts remains open pending normal host trust. Native reviewer invocation and completed
feedback are verified independently of those open items.

Architecture retains 26 baselined findings, seven stale entries, two unsupported
infrastructure subjects and disabled Web launch coverage. Existing rule/adoption
overlaps, imports and five Rule invariant findings remain outstanding. Sampling,
read-only instructions and sandbox defaults alone establish neither universal
review correctness nor effective runtime security.
