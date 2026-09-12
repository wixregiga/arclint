# Domain Studio

A general-purpose spatial editor for domain models. The 3D map is the workspace: select concepts, edit definitions, connect ideas, and inspect changes. The optional bskilled sample is an illustrative draft, not the product's domain or a confirmed architecture.

## Run locally

Requires Node.js 22.12+ (or 20.19+) and npm.

```sh
cd web/domain-studio
npm ci
npm run dev
```

Open http://localhost:5173. Everything runs in your browser; no account or API key is needed. No application data is sent to a server. Browser local storage saves the current model, one baseline, and imported Pattern references. Export JSON to keep a portable copy; local storage is specific to the browser and origin.

## Working with a domain

- **New domain** starts an empty workspace. Add contexts, then concepts. Definitions, kind, invariants, identity, aliases, and aggregate ownership can be edited in the inspector. Leave a kind unclassified while it is unresolved.
- **Connect** creates a directed, named relationship between concepts or contexts. Select a connection to rename, reverse, or delete it.
- Drag the background to orbit, right-drag to pan, scroll to zoom, and Shift-drag a concept to arrange it. Arrangement changes position only; the Context field changes ownership. The Explorer provides keyboard-accessible object selection. The overhead and frame controls recover orientation.
- **Undo/Redo** reverses model changes, including replacing a workspace. History is kept for the current session. Starting or importing another domain clears the active baseline and Pattern references; Undo restores the previous workspace.
- **Import** reads Studio JSON or canonical `domain.arclint.yaml`. Imported YAML retains its complete parsed source metadata even when the visual projection does not expose a field.
- **Export → Download workspace** preserves model IDs, positions, arbitrary relationships, and retained source metadata as JSON. Baselines are downloaded separately from the Baseline view. Pattern references remain in browser storage.
- **Export → Download domain YAML** writes the ArcLint language record. Unclassified concepts and free-form relationships become open questions. Classification alone does not prove domain validity. Canonical export requires the metadata needed by the chosen kind. Unsupported edits to an imported canonical structure raise an explicit error; JSON always remains available. YAML does not preserve camera/layout and does not preserve comments or formatting.
- **Baseline** captures a named point of reference. Added, changed, and removed concepts, contexts, and relationships appear in the comparison. Position-only changes are distinguished from changes to meaning. This is a model snapshot, separate from ArcLint's accepted-finding baseline.
- **Patterns** imports and displays real Pattern manifest definitions and rule IDs. Local completeness checks identify missing definitions, classifications, and ownership. This version does not execute Pattern rules against code or claim ArcLint conformance.
- **Prepare AI request** creates an editable, contextual prompt for the selected area. Copy or download it for your harness. This version does not run an AI, edit a repository, install skills, or execute code.

Keyboard: `/` searches, `C` adds a concept, `F` frames the model, `Ctrl/Cmd+Z` undoes, `Ctrl/Cmd+Shift+Z` redoes. Native dialogs and model forms are keyboard accessible. Reduced motion is respected; a selectable list fallback remains available if WebGL cannot initialize.

## Verify

```sh
npm test
npm run build
npx playwright install chromium
npm run test:browser
```

The browser suite uses a separate Library domain to verify creation, editing, relationships, persistence, baseline comparison, undo/redo, and JSON export/import. It also exercises desktop, tablet, and mobile layouts. Canonical import/export tests include this repository's actual domain YAML and an embedded Pattern manifest.

Run `make check` from the repository root for the repository finish gate. No Go domain, CLI behavior, or root ruleset was changed by this standalone editor.

## Design references

Original procedural geometry and CSS, with direction drawn from the user's No Man's Sky, Homeworld, and Gurren Lagann references. No game assets are used. Three.js supplies the spatial renderer; Vite builds the local app.

- [Three.js OrbitControls](https://threejs.org/docs/pages/OrbitControls.html)
- [Vite setup](https://vite.dev/guide/)
