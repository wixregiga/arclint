# ArcLint Places

A general-purpose domain editor with three spatial viewing levels: the whole domain, one bounded context, and a selected subject with its recorded connections. Every view contains at most eight model subjects. Architectural forms, shared ground, and a physical keeper orient the view; definitions and evidence appear when you ask for them. The optional bskilled sample remains an illustrative draft, not the product's scope.

## Run locally

Requires Node.js 22.12+ (or 20.19+) and npm.

```sh
cd web/domain-studio
npm ci
npm run dev
```

Open http://localhost:5173. Model editing runs in your browser; no account or API key is needed. The Vite dev/preview server also exposes a local, read-only ArcLint connection. Code inspection sends a path to that server, which runs the installed `arclint` binary against one configured repository. Browser local storage saves the current model, one baseline, and imported Pattern references. The user/need brief is saved separately by model name in the same browser; it is not part of canonical YAML or a workspace download. Export JSON to keep a portable copy; local storage is specific to the browser and origin.

## Working with a domain

- **Tools → Files & workspace → New domain** starts an empty workspace. Use **Tools** for **Add context**, **Connect concepts**, and **Add concept**. Definitions, kind, invariants, identity, aliases, and aggregate ownership can be edited after choosing **Edit meaning**. Leave a kind unclassified while it is unresolved.
- **Connect** creates a directed, named relationship between concepts or contexts. Select a connection to rename, reverse, or delete it.
- Click a context to enter it; click a concept to approach its detail view. **Back** returns one level. **Find** searches every subject, including those outside the current page. The pager exposes additional subjects without exceeding eight at once; selected subjects remain present on their detail pages. Saved model coordinates retain their meaning across viewing levels and reloads.
- Drag the background to orbit, right-drag to pan, scroll to zoom, and Shift-drag a concept to arrange it. Arrangement changes position only; the Context field changes ownership. **Tools → View & history** contains overhead, framing, zoom, and Undo/Redo controls.
- **Meaning** supports recording and editing language. **Governance** offers code inspection through the real ArcLint connection. **Edit meaning** opens a dedicated work surface; closing it retains your place and selection. The keeper is an interface guide: Onyx at the world level, a Steward inside a context, and an Inspector at detail. Clicking a keeper opens the corresponding local action, without a connected AI or invented inspection result.
- **Undo/Redo** reverses model changes, including replacing a workspace. History is kept for the current session. Starting or importing another domain clears the active baseline and Pattern references; Undo restores the previous workspace.
- **Tools → Files & workspace → Import** reads Studio JSON or canonical `domain.arclint.yaml`. Imported YAML retains its complete parsed source metadata even when the visual projection does not expose a field.
- **Tools → Files & workspace → Export → Download workspace** preserves model IDs, positions, arbitrary relationships, and retained source metadata as JSON. Baselines are downloaded separately from the Compare view. Pattern references remain in browser storage.
- **Tools → Files & workspace → Export → Download domain YAML** writes the ArcLint language record. Unclassified concepts and free-form relationships become open questions. Classification alone does not prove domain validity. Canonical export requires the metadata needed by the chosen kind. Unsupported edits to an imported canonical structure raise an explicit error; JSON always remains available. YAML does not preserve camera/layout and does not preserve comments or formatting.
- **Compare** captures a named point of reference. Added, changed, and removed concepts, contexts, and relationships appear in the comparison. Position-only changes are distinguished from changes to meaning. Close the comparison work surface to see changes in place; **Return to present** clears the comparison overlay. This is a model snapshot, separate from ArcLint's accepted-finding baseline.
- **Patterns** imports and displays real Pattern manifest definitions and rule IDs. Local completeness checks identify missing definitions, classifications, and ownership. Imported references are not automatically installed into a repository. **Check code** runs the Rules already configured in the bound repository and displays the real CLI diagnostic output.
- **Prepare AI request** creates an editable, contextual prompt for the selected area. Copy or download it for your harness. This version does not run an AI, edit a repository, or install skills. **Check code** executes the installed ArcLint linter separately from the AI request.

Keyboard: `/` searches, `C` adds a concept, `F` frames the model, `Ctrl/Cmd+Z` undoes, `Ctrl/Cmd+Shift+Z` redoes. The **Field guide** explains selected domain concepts and distinguishes model meaning from code checks. Native dialogs and model forms are keyboard accessible. Reduced motion is respected; a selectable list fallback remains available if WebGL cannot initialize.

## Inspect the architecture

Use the input in **Tools** to select an exact concept name or inspect a relative code path. **Site**, **Plan**, and **Matrix** show the same model and preserve its IDs and positions. In the matrix, read from the row to the column; a bidirectional context relation appears in both directions. Plan boundaries reflect recorded membership. These are authored spatial positions, not inferred evolutionary maturity.

**Open repository** reads `domain.arclint.yaml` from the configured repository into the editor. Undo restores the previous model. **Zones & layers** displays explicit local layer order from `rules.arclint.yaml`, along with declared Zone paths and permissions. Select a Zone to request `arclint context --zone <name>`; enter a file path to request `arclint context <path>`. Imported YAML and live context results retain their actual meanings and source anchors. A contract anchor is labeled as such; it is not guessed to be a type declaration.

**Check code** runs `arclint check . --format json` against files on disk. Browser draft edits are not applied there. The result retains its exit code, timestamp, diagnostic paths, rule IDs, and statuses, including baselined or suppressed findings. The installed CLI's diagnostic list does not contain a complete per-Rule outcome table; an empty list is not displayed as proof that every Rule passed. Failures remain visible errors.

The server defaults to this checkout. To inspect another local repository, including a bskilled checkout with ArcLint configured, restart with:

```sh
ARCLINT_STUDIO_REPO=/absolute/path/to/repository npm run dev
```

The `arclint` executable must be on the server's PATH. Open the app through `localhost` or a loopback IP. API calls are restricted to the configured repository, use fixed subprocess argument lists, and reject path traversal, outside symlinks, and cross-origin requests. Commands have bounded execution time and output. Both Vite dev and Vite preview expose the API; static hosting alone supports model editing but cannot inspect local repositories.

The read-only bridge does not change Go source, domain records, Rule configuration, or the accepted-finding baseline. The keepers offer actions appropriate to the current layer; their forms and roles are original interface guides.

## Verify

```sh
npm test
npm run build
npx playwright install chromium
npm run test:browser
```

The browser suite uses a separate Library domain to verify creation, editing, relationships, persistence, baseline comparison, undo/redo, and JSON export/import. It also exercises desktop, tablet, and mobile layouts; interchangeable representations; directional relationships; invariant/assertion meaning; and actual CLI context/check responses. Bridge tests cover failing checks, Zone queries, preserved diagnostic status, and repository boundaries. Canonical import/export tests include this repository's actual domain YAML and an embedded Pattern manifest.

Run `make check` from the repository root for the repository finish gate. No Go domain, CLI behavior, or root ruleset was changed by this standalone editor.

## Design references

The [shared model](model/README.md) records the subsequent model-first review, its [interactive example](model/index.html), and the requirements for equivalent spatial and conventional presentations. Those changes are proposals; the descriptions above describe the currently implemented application.

Original procedural architecture and keepers on a continuous chalk ground. The latest direction was developed through a simulated adversarial review using industrial design restraint, architectural clarity, and spatial continuity as lenses inspired by the designers named in the brief. They did not participate or endorse the result. No game assets are used. See DESIGN.md for the decisions and rejected alternatives.

- [Three.js OrbitControls](https://threejs.org/docs/pages/OrbitControls.html)
- [Vite setup](https://vite.dev/guide/)
