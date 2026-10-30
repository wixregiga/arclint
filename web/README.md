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
a gateway, API base and memory. Styles remain inside the mount's shadow root;
importing the mount never starts it or registers a worker.

## Current scope

Domain authoring includes project descriptions, recorded entries, aliases,
connections, draft forms, deliberate saving/removal and comparison with the
remote default branch when available. Draft forms persist in the current
browser session; the recording changes only on save. Context relations can be
viewed but are edited through the existing CLI. Feature prose and ticket export
are the next absorption unit. Architecture rule authoring stays outside Studio.

The presentation adapts BWrite's light authoring workspace and kind badges;
its repository remains untouched. Map layout derives from ArcLint's prior
Studio, with provenance retained at the adapted implementation.
