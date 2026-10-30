# ArcLint Studio

Studio writes the adopting project's domain language. The Language view and
Map read the same configured recording through `arclint map serve`; every save
uses ArcLint's existing validated operations. No Node service runs in the product.

## Development

From the repository root, run `make web-install`. Install Chromium once with
`cd web && npx playwright install chromium`. In `web/`, `npm run typecheck`,
`npm test` and `npm run test:browser` run the focused checks. Browser tests build
the current Go binary and load its embedded assets against temporary copies of
the identity/access YAML example, boxoffice CML recording, an empty recording
and a project with no domain file, at root and nested paths.

After changing source, run `make generate ONLY=web` from the repository root.
Commit the generated `internal/delivery/mapserve/static/web/` files alongside
source. `make check` rebuilds in temporary space and refuses stale embedded
assets; `make ci` also exercises the browser flows. Plain `go build` uses the
committed assets without a JavaScript toolchain.

## Hosting

`src/app/entrypoints/start-web.ts` owns standalone startup. Its API URLs and
manifest remain relative to the page directory. `mount-web.ts` exports
`mountStudio(container, dependencies)`, which returns `dispose()`. A host can lend
a language gateway, a separately injected `featureStore`, API base and memory. Styles remain inside the mount's shadow root;
importing the mount never starts it or registers a worker.

## Current scope

Domain authoring includes project descriptions, recorded entries, aliases,
connections, draft forms, deliberate saving/removal and comparison with the
remote default branch when available. Draft forms persist in the current
browser session; the recording changes only on save. Context relations can be
viewed but are edited through the existing CLI. Architecture rule authoring
stays outside Studio.

Features provides freeform writing with inline language cues, aliases,
definitions and connections. Authors can deliberately tie an occurrence to a
recorded entry or mark it plain; ambiguous spellings remain unchosen. Missing
or displaced references stay visible with their reason. Ctrl-Space inserts a
term and Ctrl-I inspects the cursor's use. Open in Language edits a definition
through the same validated operations, then Features shows its current meaning.

Saved features live in `.arclint/studio/features/` under the adopting project.
They reopen in another session or checkout when those JSON files are shared.
On the first successful save, the adapter creates a `.gitignore` for local
`.lock` and `.write-*` metadata if none exists there. An existing ignore file
is preserved; its owner can add those patterns. Unsaved text survives mode switches and reloads
in the current browser session. Stale saves offer an explicit choice while
retaining typed text; no other editor's work is overwritten silently.

Copy as ticket produces Markdown for Jira, Linear or an agent: title, prose and
current contextual definitions. Deliberate uses are distinguished from automatic
cues, and unresolved or ambiguous uses are named. When clipboard access is denied,
the text remains available for manual copying. There is no automatic issue-tracker
publication or two-way synchronization.

The presentation adapts BWrite's light authoring workspace and kind badges;
its repository remains untouched. Map layout derives from ArcLint's prior
Studio, with provenance retained at the adapted implementation.
