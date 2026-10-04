# Workflow recovery audit — 2026-10-04

This is the current audit following the user's reset to hooks that report whether
agents follow the domain workflow for the task being performed. Earlier guard
acceptance reports are historical claims, not approval or current contracts.

## Corrected result and domain justification

The rejected Go-only/comment policy, approval state and old `agents hooks`,
`agents setup` and `agents status` routes have been removed from distributed
source. Existing installed guards are preserved. The independent
`arclint-domain-reviewer` remains installable and versioned with ArcLint.

The new public route is `arclint agents workflow install|status|review|event`.
Any adopting project uses it; ArcLint uses the same route. Installation writes
`.codex/hooks/arclint-workflow-guard/` and adds five native event definitions.
The instructions are editable and embedded in the same release. The launcher
handles ordinary projects and linked-worktree config discovery without replacing
inherited hook definitions or granting trust.

The meanings of Agent, Coding Agent, Agent Host, Client, LLM, Agent Instructions,
AGENTS.md, Skill and Plugin remain distinguished in the agent context. Their
justification is the direct decisions and primary specifications in decisions.md;
none receives an invented tactical type merely to represent a vocabulary word.
Client denotes user interaction, not any caller of ArcLint. Installed instructions
and packaging are distinct from a running agent and its history.

The reset decision was recorded in workflow-recovery.md before implementation.
WorkflowReview is a service because assessing supplied work is behavior without
an existing aggregate owner. Evidence, Finding and Assessment are actual supplied
snapshots/results, without identity or invented consistency boundaries. Its four
operation assertions have executable checks: task required, exact named evidence,
explained finding and correction, retained coverage limits. Those deterministic
checks do not prove the semantic judgment. The application invokes an evaluator
and domain acceptance; CLI/host ports own installation and event transport.

## Requirement evidence

- Before/during work: native task and proposed-action evidence reaches the
  reviewer; the invoice sequence identified an invented category and pricing
  change before execution and distinguished proposed from applied work.
- After work: actual model samples identified absent mutation enforcement and
  duplicated policy in adapters, with owner-local repairs that reused an existing
  concept. These are bounded synthetic samples, not universal accuracy claims.
- Repairs/rebuttals: the installed protocol sequence withdrew findings only after
  repaired source and action evidence; a separate rebuttal sample accepted existing
  enforcement without asking for duplicated validation.
- Evidence/uncertainty: exact quotes and input limits were retained. Missing
  chronology remained uncertainty, rather than a fabricated workflow violation.
  Invalid/missing model JSON and process failures report unavailable review.
- Current task scope: collector tests cover steering, same-session resume,
  commits, unborn Git, preexisting unrelated changes, selected files, nested
  guidance, bounded history, truncation and escaping paths. Repeated file paths
  are deduplicated in persisted state. Arbitrary text languages are collected.
- Reviewer/contributor/host obligations are distinct in the domain recording,
  editable instructions, generated librarian skill and public documentation.
  The domain and generated AGENTS guidance were updated through their generators.
- No new approval cache, periodic timer, automatic blocking or comment policy is
  present. Findings are advisory. Process timeouts are host execution limits.
- Existing unrelated assets and the main installed guard are checked against
  preserved hashes. No rule strength or baseline was relaxed.

## Actual behavior and activation evidence

See evidence/2026-10-04-workflow-recovery/ for full supplied inputs, observed
outputs, independent assessments and executable provenance. The installed invoice
sequence used actual public CLI subprocesses and an authenticated model. The
harness performed fixture mutations; it was not native outer-host dispatch or an
autonomous coding-agent repair. Three later semantic samples and the actual ArcLint
assessment implementation were reviewed through the public review command.
Each sample records the binary hash it actually used; historical samples are not
misrepresented as an exact final release run.

Installation/status in 30f1 and the integration-proof worktree reported intact
files. The user explicitly approved the new hooks, and they were trusted through
Codex's normal UI. A discovered session-key collision between sibling launchers
was corrected with identical ordered provider definitions; both native hooks/list
results now show the same ten enabled trusted definitions, five per root. Existing
main hook trust and enabled flags remain unchanged.

The fresh-worktree native smoke executed both requested commands. All five native
lifecycle types ran, producing seven reports and six semantic reviews with no
unavailable review. The working agent received feedback; Stop assessed its final
response. This proves Linux/WSL CLI dispatch and sampled semantic feedback, not an
autonomous defect repair or Windows desktop activation. The active checkout's final native run also completed, covering all five
lifecycle types with agent-visible feedback and no unavailable review. Its
installed binary came from the exact 4ba7815 source archive.

The final installed binary is built from an archive of commit 4ba7815, excluding
uncommitted source. Earlier sample binaries retain their actual hashes and build
limits. A repeat against 65691d9 failed exact-quote acceptance; decoded native text
passages repaired the evidence presentation without weakening domain checks. The
subsequent nine-event protocol run found the departures and withdrew them after
repair. Both the failed and successful runs are retained.

## Remaining limits

Semantic responses can be wrong; samples and exact quotes are evidence, not a
certification of all domain meanings. Bounded collection cannot reconstruct
missing history. Existing main-repository hook behavior remains separate and was
not repaired, disabled or overwritten. Native Windows desktop and native resume remain unverified; tested envelopes
and sampled reviews do not establish those outcomes.

The final staged `make check`, full `make ci` race suite, 48 rule fixtures and
documentation build passed. Architecture reports zero active findings and 26
existing baselined findings; seven baseline entries are stale. The baseline and
rule strength were not relaxed. These preexisting findings remain outstanding
work, not proof of corrected semantics. Results and installation evidence are
recorded in the adjacent evidence manifest. PR #80 and issue #81 describe this corrected feature
and its measured limits rather than the rejected guard policy.
