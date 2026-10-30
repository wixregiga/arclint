# Feature writing inside Studio

Unit 1 is complete at 79913e36: domain authoring and the map ship in the single
binary, make check and make ci pass, 17 browser tests execute without skips,
and the original BWrite checkout is unchanged. This is the second complete unit.

## Product behavior

Write freely with the project's actual language visible. Recognised words and
aliases are cues, not declarations of the author's intent. Inspect a cue to see
its definition, context and connections. Deliberately associate an occurrence
with a recorded entry, or mark it plain. Shared spellings across contexts remain
ambiguous until chosen; there is no mandatory feature context. Language and Map
remain available without losing feature text. Language edits use the existing
validated domain operations, and the editor reads the current recording again.

Save feature drafts in the adopting project and reopen them in another browser
session. Copy as ticket exports the title and prose plus the definitions of the
terms used, with context names and a clear distinction between deliberately tied
uses and automatic cues. Unresolved references are visible; missing or changed
meaning is never silently assigned elsewhere. No architecture rule UI, mandatory
Gherkin, proof fields, invented brief or issue-tracker integration is needed.

## DDD review and classification

The user asks for intent expressed with domain language at hand, including when
language emerges during iterative feature design. The vocabulary context's
meaning was updated through domain define before implementation. Classification
of the feature document as an aggregate is NOT ESTABLISHED: the owner's evidence
requires writing and contextual references, not a business identity or an atomic
consistency invariant. Its file address is a storage address. By the librarian's
litmus-first and structure-follows-classification rules, no tactical type is
invented merely to store the document. The known meaning is in the context
definition. Basis: Evans 2015 pp.12,16 (tier 1, read through the verified meta-model).
Opposite: BWrite makes Feature an aggregate because offsets must always fit its
text. We preserve unresolved annotations instead of asserting that invariant over
external edits, so that inherited consistency boundary is not imported.

Feature artifact loading/saving is application coordination over an inward-owned
port, implemented by an infrastructure file adapter and wired in composition.
HTTP and Web translate and render. Vocabulary validity remains with the existing
model; feature persistence creates no second vocabulary. Verdict: conforms.
Basis: application-service-holds-no-rule; Evans 2015 p.10 (tier 1, read through the
verified meta-model). Opposite: put persistence in delivery; rejected because it
would bypass the existing adapter boundary. Reference resolution in the editor
indicates what is unresolved, not whether feature prose is correct.

Automatic recognition never creates a rule or contract. Explicit reference
selection does not assert correctness, and Copy as ticket carries no completion
status. Verdict: conforms; declined normative-wording-is-a-rule and the recorded
structural-checks-are-not-proof remain intact. File concurrency and refusal with
no writes are adapter effects verified by tests, not invented domain invariants.

## Fixed HTTP contract for the two implementation agents

All paths are relative to the injected API base and keep the existing origin
checks. Only the composition root imports the new infrastructure adapter.

- GET api/features returns `{features: [{id,title,revision}]}` in a deterministic
  order. No directory or files returns an empty list.
- GET api/feature?id=ID returns `{feature: Document}` or a 404 error.
- POST api/feature accepts `{id?,revision?,title,body,references}` and returns
  `{feature: Document}`. No id creates one; an existing id requires the revision
  last read. A stale revision returns 409 and writes nothing. Creation never
  overwrites a supplied address; an unknown supplied id returns 404.
- DELETE api/feature?id=ID takes the last-read revision through the If-Match
  header and returns `{removed: ID}`. Stale revisions return 409 with no write.

Document is `{id,revision,title,body,references}`. The server computes revisions
from persisted content; they are opaque strings, not timestamps. References are
`{id,from,to,quote,reading,term?}`. Positions use the editor's UTF-16 text units;
quote retains the selected text. Reading is `names`, `value`, `rule` or `plain`.
A non-plain reference carries term `{concept,context,owner,name}`; plain carries
none. This metadata is an annotation of the same saved body. References that do
not fit a later body, or whose target is gone, remain saved and are shown as
unresolved with a reason. No guessed retargeting or automatic semantic acceptance.

The infrastructure adapter stores versioned JSON documents under
`.arclint/studio/features/<server-generated-id>.json`, relative to the selected
project. It limits and validates its own file identifiers and request shape,
uses atomic replacement, and refuses stale saves/deletes under a lock. It does
not accept arbitrary paths, follow links outside that folder or write the domain
recording. It rejects malformed stored files visibly rather than hiding them.
The application port carries typed document data, not an opaque mutable domain.
Error responses remain `{error: string}`; bad transport input is 400, missing
artifact 404, stale write 409, and read/write failure 422. No registered operation
may lack implementation and tests in the same unit.

The browser's draft transport is separately injected from StudioGateway so hosts
can supply it explicitly; standalone uses the same API base. Existing mounts
remain usable for language without a feature store. The feature view loads its
store when opened. Failed and stale saves keep all typed text and offer recovery.

## Draft storage integration

Astra's typed draft operations and HTTP routes are integrated. The file adapter
uses the same rooted-filesystem and advisory-lock approach as the existing agent
progress store: directory handles confine file access, and one lock covers the
revision comparison and atomic replacement across Studio processes. JSON files
remain versioned; non-document metadata such as .gitignore is not a draft.

Verification includes two compiled Studio servers over each of the repository's
real YAML and CML recordings. It exercises concurrent saves, reopening through
another process, stale deletion, foreign-origin refusal, Unicode round-trips,
and byte-for-byte preservation of the domain recording. Filesystem tests also
cover a replaced storage directory, outside-file links, malformed documents,
and missing required stored text fields. The browser editor remains a separate
integration step; passing these checks does not establish that the full feature
writing flow works.
