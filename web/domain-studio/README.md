# The Domain Atlas

A general-purpose domain editor built as an explorable, physical atlas. Contexts occupy terraced terrain; recorded concepts and relationships form its architecture and routes. The landscape fills the viewport. Selection opens a contextual manuscript; search and file commands appear only when invoked. The optional bskilled sample is an illustrative draft, not the product's domain or a confirmed architecture.

## Run locally

Requires Node.js 22.12+ (or 20.19+) and npm.

```sh
cd web/domain-studio
npm ci
npm run dev
```

Open http://localhost:5173. Model editing runs in your browser; no account or API key is needed. The Vite dev/preview server also exposes a local, read-only ArcLint connection. Code inspection sends a path to that server, which runs the installed `arclint` binary against one configured repository. Browser local storage saves the current model, one baseline, and imported Pattern references. The user/need brief is saved separately by model name in the same browser; it is not part of canonical YAML or a workspace download. Export JSON to keep a portable copy; local storage is specific to the browser and origin.

## Working with a domain

- **Workspace → New domain** starts an empty workspace. Use the creation compass for **Add context** and **Connect**, or the adjacent **Add concept** action. Definitions, kind, invariants, identity, aliases, and aggregate ownership can be edited in the inspector. Leave a kind unclassified while it is unresolved.
- **Connect** creates a directed, named relationship between concepts or contexts. Select a connection to rename, reverse, or delete it.
- Drag the background to orbit, right-drag to pan, scroll to zoom, and Shift-drag a concept to arrange it. Arrangement changes position only; the Context field changes ownership. The **Index** provides keyboard-accessible object selection. Double-click a context or use **Enter context** to descend; explore an aggregate only when that ownership is recorded. The locator shows the current level. Escape or **Return one level** ascends. The overhead and frame controls recover orientation.
- **Undo/Redo** reverses model changes, including replacing a workspace. History is kept for the current session. Starting or importing another domain clears the active baseline and Pattern references; Undo restores the previous workspace.
- **Workspace → Import** reads Studio JSON or canonical `domain.arclint.yaml`. Imported YAML retains its complete parsed source metadata even when the visual projection does not expose a field.
- **Workspace → Export → Download workspace** preserves model IDs, positions, arbitrary relationships, and retained source metadata as JSON. Baselines are downloaded separately from the Compare view. Pattern references remain in browser storage.
- **Workspace → Export → Download domain YAML** writes the ArcLint language record. Unclassified concepts and free-form relationships become open questions. Classification alone does not prove domain validity. Canonical export requires the metadata needed by the chosen kind. Unsupported edits to an imported canonical structure raise an explicit error; JSON always remains available. YAML does not preserve camera/layout and does not preserve comments or formatting.
- **Compare** captures a named point of reference. Added, changed, and removed concepts, contexts, and relationships appear in the comparison. Position-only changes are distinguished from changes to meaning. This is a model snapshot, separate from ArcLint's accepted-finding baseline.
- **Patterns** imports and displays real Pattern manifest definitions and rule IDs. Local completeness checks identify missing definitions, classifications, and ownership. Imported references are not automatically installed into a repository. **Check code** runs the Rules already configured in the bound repository and displays the real CLI diagnostic output.
- **Prepare AI request** creates an editable, contextual prompt for the selected area. Copy or download it for your harness. This version does not run an AI, edit a repository, or install skills. **Check code** executes the installed ArcLint linter separately from the AI request.

Keyboard: `/` searches, `C` adds a concept, `F` frames the model, `Ctrl/Cmd+Z` undoes, `Ctrl/Cmd+Shift+Z` redoes. The **Field guide** explains selected domain concepts and distinguishes model meaning from code checks. Native dialogs and model forms are keyboard accessible. Reduced motion is respected; a selectable list fallback remains available if WebGL cannot initialize.

## Inspect the architecture

Use the input above the drawing to select an exact concept name or inspect a relative code path. **Site**, **Plan**, and **Matrix** show the same model and preserve its IDs and positions. In the matrix, read from the row to the column; a bidirectional context relation appears in both directions. Plan boundaries reflect recorded membership. These are authored spatial positions, not inferred evolutionary maturity.

**Open repository** reads `domain.arclint.yaml` from the configured repository into the editor. Undo restores the previous model. **Zones & layers** displays explicit local layer order from `rules.arclint.yaml`, along with declared Zone paths and permissions. Select a Zone to request `arclint context --zone <name>`; enter a file path to request `arclint context <path>`. Imported YAML and live context results retain their actual meanings and source anchors. A contract anchor is labeled as such; it is not guessed to be a type declaration.

**Check code** runs `arclint check . --format json` against files on disk. Browser draft edits are not applied there. The result retains its exit code, timestamp, diagnostic paths, rule IDs, and statuses, including baselined or suppressed findings. The installed CLI's diagnostic list does not contain a complete per-Rule outcome table; an empty list is not displayed as proof that every Rule passed. Failures remain visible errors.

The server defaults to this checkout. To inspect another local repository, including a bskilled checkout with ArcLint configured, restart with:

```sh
ARCLINT_STUDIO_REPO=/absolute/path/to/repository npm run dev
```

The `arclint` executable must be on the server's PATH. Open the app through `localhost` or a loopback IP. API calls are restricted to the configured repository, use fixed subprocess argument lists, and reject path traversal, outside symlinks, and cross-origin requests. Commands have bounded execution time and output. Both Vite dev and Vite preview expose the API; static hosting alone supports model editing but cannot inspect local repositories.

The read-only bridge does not change Go source, domain records, Rule configuration, or the accepted-finding baseline. **Onyx** offers local next steps for modeling and inspection; it is an original stylized helper, not a connected AI.

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

Original procedural terrain, architecture, and CSS, drawing on the latest supplied direction: Japanese material landscapes, tactical selection and camera language, and contextual encyclopedia entries. The rejected dashboard composition is preserved only in checkpoint `247958f`; the current UI uses a full-viewport landscape with transient tools. No game assets are used. Three.js supplies the renderer; Vite builds the local app. See DESIGN.md for the governing design decisions.

- [Three.js OrbitControls](https://threejs.org/docs/pages/OrbitControls.html)
- [Vite setup](https://vite.dev/guide/)
