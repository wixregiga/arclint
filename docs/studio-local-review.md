# Studio absorption: local PR review

The branch was reviewed locally before publication against `origin/main` at
`5a01c469c2b1`. Astra reviewed Web correctness and its domain statements; Fable
reviewed Go persistence, composed HTTP behavior and its domain statements.
The integrating agent reviewed build, delivery, documentation and recording changes.
BWrite was read as source material and remains untouched.

## Findings and resolutions

| Finding | Resolution and evidence |
| --- | --- |
| P1: a delayed save or delete conflict could appear on another feature and offer to discard its text. | Feedback is held per feature. Two real-server browser regressions failed before the fix and pass afterward, retaining both features' text. |
| P2: deletion confirmation could follow the author to a different feature. | The feature panel remounts on selection changes. A regression first loads both features, reproduces the old confirmation leak, then proves the deliberate delete targets the selected feature only. |
| P2: lock and abandoned write files appeared beside committable drafts. | A successful save creates default ignore patterns if no ignore file exists. Tests inspect Git's actual untracked-file list and preserve an adopter-authored ignore file byte for byte. |
| P2: negative or reversed annotation ranges were persisted. | Shared application document validation rejects impossible ranges. Filesystem and composed HTTP tests verify refusals leave the saved body and revision unchanged. Collapsed and out-of-body stale annotations remain retained. |
| P2: an obsolete Node test exercised the retired frontend. | The old HTML, script and test were removed. Web unit tests and browser tests exercise the source and embedded Studio delivered by Go. |
| Relationship labels implied one-way influence for every context relation. | Partnership and shared kernel show mutual influence; separate ways has no arrow. Model tests cover all recorded kinds and unknown kinds; a rendering test supplies relation variants over a real overview response. |
| Help text narrowed assertions to postconditions, and several comments described retired behavior. | Assertion and specification wording follows the existing meta-model. Route, build, launch-rule and persistence documentation describe the implementation. |

## DDD review

| Decision and home | Verdict | Grounds and strongest opposite |
| --- | --- | --- |
| Vocabulary defines the language and validates language edits; Web renders and coordinates them. | Conforms | Existing vocabulary model and inward layer contracts. Keeping validation in the browser would make another authority. |
| A feature draft is an application document carried through an application port, not a new domain aggregate. | Conforms | Librarian protocol 2, 6 and 10 require classification evidence. Persistence and revision comparison alone do not establish a business consistency boundary. |
| Document shape guards live beside the application contract; files, locking and revisions stay in infrastructure. | Conforms | These are programmer and delivery guards, not business decisions. A Feature aggregate would add an unsupported classification. |
| Recognition cues and deliberate ties do not create domain entries or prove requirements satisfied. | Conforms | The existing declined wording-to-rule check does not prohibit explicit language edits; structural coverage is not behavioral proof. |
| Features retain stale references as unresolved rather than silently retargeting them. | Conforms | Delivery preserves author input. Impossible negative and reversed ranges are input errors, distinct from valid annotations displaced by later edits. |

The layer basis is Evans, *DDD Reference* (2015), p. 10, tier 1, read through
the meta-model's text-verified passage. Classification and declined-rule verdicts
also use the repository's domain-librarian protocol and sourced meta-model.
They establish placement, not the product's effect on coding-agent performance.

## Sweep and engineering decisions

The original touched-file inventory contained 4,655 rows. Every row was read
and classified across three partitions: Web 2,501 rows, backend 465 rows and
integration 1,689 rows. A duplicate-preserving coverage check found no missing,
extra or unverified rows. The fixes and this report received a final delta sweep.
The only recording correction was the vocabulary context definition, written
through `arclint domain define`, then exported to CML. It has 53 of 60 words.
No new tactical entries or domain invariants were justified.

The review's engineering questions are resolved as follows:

1. The file adapter owns default local metadata ignore patterns and preserves
   any adopter-authored ignore file.
2. A corrupt or unexpected JSON file visibly fails the list operation. It is
   not silently omitted; direct reads of intact drafts remain available.
3. Draft shape validation is shared by the application operation and file
   adapter. It neither resolves references nor checks domain meaning.

The documented local API retains 422 for storage failures and 400 for invalid
or oversized request bodies. Changing that error contract is outside this slice.
No product decision remains awaiting an answer. Existing baseline findings,
including overlong definitions outside the changed vocabulary definition, remain
reported by ArcLint; no baseline or rule was weakened.

## Follow-up review

The automated review after the CI synchronization fix found that JSON property
order made saved annotations appear unsaved. The page model compares annotation
fields by value. A reducer regression and a real-store browser assertion failed
before the fix and pass afterward; edits made during a save remain drafts.

The earlier global-navigation finding is also fixed: search results and the
project name open Language when invoked from Features. Real-store browser
regressions cover both actions, preserve the feature draft, and confirm that
those actions keep Map mode when invoked there.

## Validation limits

The integrated checks cover 74 Web unit tests, 29 Chromium browser tests,
compiled two-process persistence tests against real YAML and CML projects,
architecture fixtures, embedded-asset freshness and the repository gates.
The relation rendering test supplies response variants; persistence workflows
use the actual Go routes and project files. Firefox, WebKit, Windows runtime,
NFS locking, installed-PWA and offline behavior were not verified.
