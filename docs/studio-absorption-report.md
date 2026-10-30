# BWrite absorption into ArcLint Studio

## Result

Studio primarily writes the adopting project's domain language. Its Language,
Map and Features views use that project's configured recording. The Web build
is embedded in the Go binary; installed users need no Node service. The original
BWrite checkout at c75ead5f remains untouched. Work started from origin/main at
5a01c469, on the existing feat/explore-feature-writing branch.

## Working flow

1. Open `./arclint map serve <project-path>`. A missing domain file can begin in
   Studio; a malformed existing recording still reports its error.
2. Write and refine project descriptions, contexts and recorded entries in
   Language. Search names, aliases or definitions. Map shows their recorded
   connections and opens the same inspector. Available default-branch data
   distinguishes Accepted and Proposed entries; unsaved forms stay drafts.
3. Open Features and describe behavior freely. Recorded names and aliases cue
   inline, with definitions and connections beside the prose. An ambiguous
   spelling offers its contexts. Tie an occurrence deliberately or mark it plain.
4. Open a meaning in Language, edit it through the existing validated operation,
   then return to the feature with text intact and the current definition shown.
5. Save the feature into `.arclint/studio/features/`. Reopen it in another session.
   A stale revision preserves typed text and offers an explicit recovery choice.
6. Copy as ticket for Jira, Linear or an agent. Markdown carries the title,
   prose and current contextual definitions, distinguishing deliberate ties from
   automatic cues. Denied clipboard access offers manual copying.

## Implementation and review

| Responsibility | Owner | Existing mechanism used |
| --- | --- | --- |
| Project and empty-recording HTTP path | Astra | Existing domain overview and validated define operations |
| Feature persistence | Astra, completed in integration | Inward application port, infrastructure adapter, mapserve, composition |
| Language, Map and feature editor | Fable | Reserved Web FSD structure, BWrite presentation and CodeMirror behavior |
| Integration and adversarial review | Codex | Embedded assets, real browser/server tests, repository gates |

Review fixed late save responses discarding newer typing, displaced annotations
breaking later edits, and workspace upgrades discarding unsaved Language forms.
The file adapter also gained rooted filesystem access and cross-process locking.
The final browser suite uses real Go feature routes and project files; its store
stub was removed. DDD placement and classification decisions are recorded in
[the design contract](studio-feature-writing.md).

## Verification

- 74 Web unit tests cover recognition, aliases, ambiguity, annotations, undo,
  transport, export, mounting/disposal, workspace persistence and pending saves.
- 29 Chromium browser tests cover the embedded build at root and nested bases,
  YAML and CML recordings, empty and absent recordings, actual file persistence,
  a new browser session, language edits, ticket copying and narrow screens.
- Two compiled Studio processes verify competing saves and stale deletion over
  both real recordings, with unchanged recording bytes and Unicode round-trips.
- Architecture: zero active findings; the existing 38 baseline entries remain.
  All 53 architecture fixtures pass. No rule, baseline or exclusion was weakened.
- Embedded build freshness is checked against a fresh temporary Vite build.

## Practical limits

Browser verification uses Chromium; Firefox, WebKit, installed-PWA behavior and
offline workflows are unverified. Context relations are visible and remain
editable through the CLI. Copy as ticket is the Jira/Linear handoff; there is no
automatic publishing or synchronization. Saved JSON documents are shared through
the adopting project's usual workflow; unsaved text remains session-local.

These checks establish the authoring behavior and integration. They do not
measure whether agents produce better implementations; that remains a product
hypothesis to evaluate with real adopting teams. The implementation completed
locally before the separate review and PR publication step. See the
[local PR review](studio-local-review.md) for findings and resolutions.

## Final gates

`make check` and `make ci` both exited zero on the integrated implementation.
The final CI output includes:

```text
Tests  74 passed (74)
Embedded Studio matches the current Web build.
29 passed
INF no leaks found
```

The generated Web bundle currently triggers Vite's advisory chunk-size warning;
the build and freshness check succeed. The source and embedded build are committed
together. No runtime Node process is added to installed ArcLint.
