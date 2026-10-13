# Placement gate

Apply after the owner evidence is quoted. The first row whose conditions hold decides. Rule ids are the protocol numbers in `.agents/skills/domain-librarian/SKILL.md` and the `distillation_rules` ids in its `VOCAB.yaml`.

| placement | conditions | deciding rules | example from the PR #96 run |
|---|---|---|---|
| already recorded | the recorded language states it, under the right owner, in equivalent words | Evidence (rule 1) | `context_map/one-relation-per-pair` was already in the meta-model; the CML adapter only duplicates it |
| change to an existing entry | the recorded text is wrong, incomplete, or silent on a case the change now covers | Precedence (rule 9), Description style (rule 14) | `inside-the-repository` said nothing about the file a symlink reaches |
| invariant under a value object | value integrity, holds at every observable time, one violation, enforceable in the constructor, owner is the subject | Invariant gate (rule 10), invariant-ownership, value-test | `DomainRecording / format-named-by-extension` |
| invariant under an aggregate | a cluster rule that must stay consistent in one transaction, owned by the root | rule 10, invariant-ownership, transaction-boundary | candidate `Rule / pattern never chooses the domain file` (needed the owner's decision) |
| assertion | a guarantee of one named operation of the root or of a domain service, with `on` | precondition-as-assertion, contract-of-a-service | none in that run |
| specification | experts pass the predicate around as a thing | specification-as-thing | none in that run |
| definition only | the meaning is established but it restates or orders facts, forbids nothing a naive implementation could do, or has no capable owner | rule 10 ("restating a definition is not an invariant"), Litmus first (rule 2) | the discovery order (named file, then yaml, yml, cml) |
| question | a product decision the evidence leaves open, or two sources that disagree | Ask or record (rule 8), minimal-evidence | when does the ruleset carry `domain.file` |
| no entry | one of the reasons below | the rule named beside the reason | |

## Reasons for no entry

Write the reason in these words so reports compare.

| reason | rule | test |
|---|---|---|
| programming guard | language-not-guards | you would not say it to an expert who never saw the code (a nil check, a parse error) |
| delivery promise of an adapter or port | not-a-domain-rule | it describes how one format or one adapter writes, retries, formats, or warns |
| application precondition | not-a-domain-rule, application-service-holds-no-rule | it holds only when one use case runs and no business decision is lost by deleting the step |
| delivery validation | not-a-domain-rule | the software proves its own output (read back, round trip) |
| contributor duty | rule 12 (Completeness sweep) | it tells a contributor what to do, not what the domain holds |
| presentation | rule 14 | it is wording of help, warnings or docs |
| decision already recorded elsewhere | Evidence (rule 1) | the meta-model or another context already records it |

A business decision found inside an application service is placed under its owner and reported as a finding (application-service-holds-no-rule). "No entry" is never the answer for a product decision.

## Enforcement labels

| label | meaning |
|---|---|
| `file:line` | the owner's constructor or root enforces it there |
| `none` | nothing enforces it; prose or a test only |
| `misplaced: where` | enforced, but outside the owner's constructor or root, or spread over several callers |
| `split: a / b` | the owner enforces the shape and an adapter enforces the rest (name both) |

## Pitfalls seen

- An invariant whose text joins two clauses that fail separately is two entries. A backslash path can lie inside the repository.
- "Every command says so" is a promise of the binary, and it was false for hook events and init. Check each universal claim against the dispatch code before letting it stand in a definition.
- Tests pin behavior; they do not approve it. A refusal only tests pin, with no doc stating it, still needs the owner's yes before it is recorded.
- Docs, help text and code can disagree three ways on one rule. Report the disagreement, then ask one question.
- The worktree can change while you read it. Compare `git status --short` at start and end and report what moved.
- A candidate the caller names gets a row even when the answer is no entry, with the reason.
