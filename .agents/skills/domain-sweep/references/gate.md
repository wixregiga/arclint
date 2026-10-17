# Placement gate

Apply after the owner evidence is quoted. Rule ids are the protocol numbers in `.agents/skills/domain-librarian/SKILL.md` and the `distillation_rules` ids in its `VOCAB.yaml`. Each decision below names the issue that records it and, once arclint carries it, the rule or the `VOCAB.yaml` section that does. Issue #151 is still open: its decision already binds placement, but arclint does not enforce it yet. Where a command or flag a decision describes does not exist in the arclint you built, say so in the report and do not write the command.

## Order of checks

For each statement, in this order:

1. **Declined rules.** Compare it with `declined_rules` in the librarian's `VOCAB.yaml` (protocol rule 11). A match is placed `declined`, with the entry's id, reason and basis. Do not propose the rule again.
2. **Disagreement.** When two owner sources state it differently, it is a question, whatever row it would otherwise fit. Record the discrepancy with both sources.
3. **Placement.** Otherwise the first row of the placement table whose conditions hold decides.
4. **Decide, then carry out.** For a guarantee of an operation, apply the section of that name before choosing the owner.
5. **Declared services.** When the owner is a declared domain service, apply the shapes of that section.
6. **Proving test.** Name the test that proves the entry, or `none`.
7. **Word limits.** Count the words of every text you propose. Over the limit is not a proposal.
8. **Sources.** Every literature citation states its tier, and a rule needs a tier 1 source or an owner decision.

## What counts as evidence

- **The recorded language** is the recording at the merge base. What the change itself records is a candidate like any other row: its recording is a claim (Evidence, rule 1), and "already recorded" never applies to it.
- **The owner's docs** are the docs as they stand at the merge base. Docs the change adds or edits are claims until the owner approves the change.
- **The owner's words** are issue and pull request text the owner's account wrote. Text an agent generated is a claim, wherever it is posted: a body that ends "Generated with Claude Code", a bot's review, an issue title over a bot's body.
- **An owner decision** for step 8 is the owner's words or the owner's docs above.

## Placement table

| placement | conditions | deciding rules | example |
|---|---|---|---|
| declined | the statement matches an entry of VOCAB's `declined_rules` | Declined rules (rule 11), issue #144 | `factory/required-for-every-aggregate` (Evans 2015, p. 18) |
| already recorded | the project's recording at the merge base states it, under the right owner, in equivalent words | Evidence (rule 1) | a statement the change only repeats |
| change to an existing entry | an entry recorded at the merge base is wrong, incomplete, over its word limit, or silent on a case the change now covers | Precedence (rule 9), Description style (rule 16), `domain_recording/text-within-word-limit` | an entry that says nothing about the file a symlink reaches |
| invariant under a value object | value integrity, holds at every observable time, one violation, enforceable in the constructor, owner is the subject | Invariant gate (rule 10), invariant-ownership, value-test | PR #96: `DomainRecording / spelled-with-slashes` (the owner's docs: a domain file path is "written with slashes") |
| invariant under an aggregate | a cluster rule that must stay consistent in one transaction, owned by the root | rule 10, invariant-ownership, transaction-boundary | candidate `Rule / pattern never chooses the domain file` (needed the owner's decision) |
| assertion | a guarantee of one named operation of the root or of a domain service, with `on`; the operation is the one that decides (see below) | precondition-as-assertion, contract-of-a-service, issue #128 | an assertion on the service operation that returns the verdict, never on the step that writes |
| specification | experts pass the predicate around as a thing | specification-as-thing | none in the PR #96 run |
| definition only | the meaning is established but it restates or orders facts, forbids nothing a naive implementation could do, or has no capable owner; and the definition can hold it within its word limit | rule 10 ("restating a definition is not an invariant"), Litmus first (rule 2) | PR #96: a domain file the ruleset names is the project's own choice |
| question | a product decision the evidence leaves open, two owner sources that disagree, or a meaning its owner's definition cannot hold within its limit | Ask or record (rule 8), minimal-evidence | PR #96: which file is the domain file when the ruleset names none (issue #95 and the docs disagree); when the ruleset carries `domain.file` |
| no entry | one of the reasons below | the rule named beside the reason | PR #96: a translation target that cannot be read is never overwritten (delivery promise of an adapter) |

Split an "and" of independently violable clauses into one candidate per clause before placing it; the split applies to entries, not to no-entry rows. A clause the owner cannot check, such as where a link on disk leads, keeps its entry, is labelled `split` under enforcement, and names the test that proves the rest.

A rule found inside a recorded definition is a statement like any other. Wording alone does not make a rule: "must-always/must-never wording alone does not establish one" (invariant-ownership, `internal/domain/vocab/distillation.go:32`). A definition is read for what it forbids, whatever its words.

## Declined rules

Recorded in issue #144, and carried in the librarian's `VOCAB.yaml` under `declined_rules`: the rules arclint deliberately does not enforce, each with an id, the statement declined, the reason, and its basis (a tier and page of literature, or an owner decision's issue). Read that list; do not copy it here, so it cannot drift. When a candidate states what an entry declines, place it `declined` and cite the entry's id, reason and basis. As the librarian's rule 11 says, new tier 1 or 2 literature or a new owner decision in an issue reopens an entry; a request alone does not, so say which of the two would.

## Decide, then carry out

Recorded in issue #128, and in the librarian's rule 13 (Moves between layers). Record a service's assertion on the operation that makes the decision. That operation receives what it needs as values, returns its verdict, and changes nothing. The application gathers those values, asks for the verdict, and makes the changes only when the verdict allows them; it holds no business rule of its own (application-service-holds-no-rule).

- The verdict is a value. A verdict that carries a rule ("approves only when no participant refused") is a value object with that rule as its invariant, so `value_object/constructed-through-one-door` applies.
- An effect the domain cannot see ("a refusal changes nothing on disk") is proved by an automated test (Evans 2015, p. 22), not by moving the workflow into the domain.
- A fact the model cannot observe, such as whether a file exists or whether two paths are one file, is gathered by the application through a port and passed to the deciding operation as a value.
- Before you recommend moving code between layers, for example a workflow into a domain service to satisfy `assertion/checked-by-its-operation`, state the strongest case against the move with sources from the meta-model's bibliography. Then recommend the placement where the decision goes to the domain and the execution stays in the application.
- A business decision found in an application service is placed under its owner and reported as a finding. "No entry" is never the answer for a product decision.

## Declared services

Recorded in issue #148, carried by the distillation rule `guarantee-has-one-checked-home` and enforced by `assertion/checked-by-its-operation`. A domain service declared the canonical way is "a standalone interface declared as a service" (Evans 2015, p. 14): a Go interface, a TypeScript interface or abstract class, a Python ABC or Protocol. Its assertion is checked once, in the model, by one of two shapes:

- **(a) a check in the model:** a function of the context spelled from the key (`AssertNeverExceedsTotal`, `assertNeverExceedsTotal`, `assert_never_exceeds_total`);
- **(b) a value:** the operation returns a recorded value object of the same context whose invariant has the same key, enforced by its constructor (Evans 2015, p. 21: "moving complex logic into value objects").

Never an interface method, a callback, or prose. Only what neither shape can state is verified by the tests under its `verified_by`, which `assertion/verifying-tests-declared` checks exist. When neither shape exists and no test is named, report the missing check on the model side. An aggregate root declared as an interface or an abstract class needs those tests; a concrete owner (struct, class) keeps the checking method `assertion/checked-by-its-operation` asks for.

A recorded domain service records at least one assertion (`domain_service/records-an-assertion`). A statement that justifies a service but names no guarantee of its operations is a question for the expert's words, not a service without a contract.

## Proving tests

Recorded in issue #143. For every entry, name the test that proves it as `path:TestName` from the repository root (Go `func Test...`, Python `def test_...` or `Class.test_...`, the TypeScript `test("...")` name). Read the test before you name it. This is verification of enforcement, not classification, so the librarian's Economy section does not limit it. A test that exists shows the behavior is pinned; whether it proves the statement is your judgment, and you say which. Record it with `arclint domain define invariant|assertion ... --verified-by <path>:<test name>`, repeated for several; the flag replaces the recorded tests, and `--clear-verified-by` records none. `invariant/verifying-tests-declared` and `assertion/verifying-tests-declared` then fail the check when a named test is missing. For an owner declared as an interface, the proving test is the expected evidence, not an empty `Assert` method.

Tests pin behavior; they do not approve it. A rule only tests state, with no doc and no owner's words, needs the owner's yes before it is recorded.

## Word limits

Recorded in issue #125 and enforced by `domain_recording/text-within-word-limit`: `arclint domain define` refuses a longer text, and `arclint check` reports one already recorded as an error. Every text you propose fits:

| text | limit |
|---|---|
| an entry's definition (aggregate, entity, value object, domain service, domain event, specification) | 40 words |
| a bounded context's definition | 60 words |
| an invariant's or assertion's statement | 40 words |
| the project description | 100 words |

A definition says what a word means. When the meaning will not fit, keep what the word means, record each rule it states as an invariant or assertion, and put promises that are not domain rules in tests. A fact that is neither, such as an order of discovery, and that the definition cannot hold within its limit, is a question for the owner. An entry recorded at the merge base over its limit is a change to an existing entry, with its word count. No limit is recorded for a question's text; keep each to one decision.

## Sources

The owner's source standard, recorded in issue #151 (still open; the librarian's `VOCAB.yaml` carries the tiers under `source_tiers`):

| tier | works | may |
|---|---|---|
| 1, canonical | Evans 2003 and 2015, Vernon 2013 and his papers and sample code, Evans and Fowler 1997 | ground rules |
| 2, published authority | Fowler's PoEAA (Stafford's patterns included), Meyer's OOSC, Wlaschin's book, Fowler's bliki | ground guidance |
| 3, practitioner blogs | | only illustrate |
| excluded | Goodreads, unknown Medium authors, Stack Overflow, AI summaries | never be used |

Every citation states its tier and whether you read the page this session. A rule with neither a tier 1 source nor an owner decision is a question, not an entry. Canonical references are never removed or demoted: a proposal that edits a recorded tier 1 citation is refused.

## Reasons for no entry

Write the reason in these words so reports compare.

| reason | rule | test |
|---|---|---|
| programming guard | language-not-guards | you would not say it to an expert who never saw the code (a nil check, a parse error) |
| delivery promise of an adapter or port | not-a-domain-rule | it describes how one format or one adapter writes, retries, formats, or warns, or how a command controls its transaction (all or nothing) |
| application precondition | not-a-domain-rule, application-service-holds-no-rule | it holds only when one use case runs and no business decision is lost by deleting the step |
| application coordination | Behavior (rule 12), application-service-holds-no-rule | it loads, invokes, commits, publishes, notifies or translates, and deleting it loses no business decision |
| delivery validation | not-a-domain-rule | the software proves its own output (read back, round trip) |
| contributor duty | rule 14 (Completeness sweep) | it tells a contributor what to do, or what a contributor's tool (a skill, a script, a make target) promises, not what the domain holds |
| presentation | Description style (rule 16) | it is wording of help, warnings or docs |
| decision already recorded elsewhere | Evidence (rule 1) | the meta-model, another context, or an owner decision that placed it outside the domain already records it (PR #96: `context_map/one-relation-per-pair` is in the meta-model; the CML adapter repeats it) |

A no-entry statement that is still a promise names the test that pins it, or `none`.

## Enforcement labels

| label | meaning |
|---|---|
| `file:line` | the owner's constructor, root, or model-side check enforces it there |
| `none` | nothing enforces it; prose or a test only |
| `misplaced: where` | enforced, but outside the owner's constructor or root, or spread over several callers |
| `split: a / b` | the owner enforces the shape and an adapter enforces the rest (name both) |
| `adapter: file:line` | for a no-entry promise or a rule recorded elsewhere: the adapter or port that keeps it |
| `hollow: file:line` | the code arclint reports as the check, which checks nothing; report it as a finding |

## Pitfalls seen

- A rule worded without a keyword is still a rule. PR #96's discovery order ("the first of these files that exists") carried no keyword; a keyword pass missed it, so every row is read.
- An invariant whose text joins two clauses that fail separately is two entries. A backslash path can lie inside the repository.
- "Every command says so" is a promise of the binary, and it was false for hook events and init. Check each universal claim against the dispatch code before letting it stand in a definition.
- Docs, help text and code can disagree three ways on one rule. Report the disagreement, then ask one question.
- Satisfying a finding by moving a workflow into the domain hides the decision in callbacks. Separate the decision from its execution first.
- The worktree can change while you read it. Compare `git status --short` at start and end and report what moved.
- A candidate the caller names gets a row even when the answer is no entry, with the reason.
