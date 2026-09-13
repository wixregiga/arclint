# ArcLint Studio

A domain modeling and repository governance workspace. Bounded contexts are regions of local meaning. Aggregate enclosures show recorded ownership. Zones are independent, overlapping file sets. Spatial and Table presentations share the same editing commands, drafts, selection, and history. The optional bskilled example is one test domain; the application works with other projects and canonical ArcLint domain files.

## Run locally

Requires Node.js 22.12+ (or 20.19+), npm, and an installed `arclint` executable for repository operations.

```sh
cd web/domain-studio
npm ci
npm run dev
```

Open http://localhost:5173. The server binds this checkout by default. To work with another repository:

```sh
ARCLINT_STUDIO_REPO=/absolute/path/to/repository npm run dev
```

On first use, Studio opens the bound repository’s actual domain when available. A late response cannot replace work you have started typing. Without a connection, an empty local domain remains usable. Saved workspaces always take priority; the bskilled example is an explicit menu action.

The binding is explicit and fixed for that server. Importing a domain file never silently changes which repository a check or write targets. Static hosting supports local modeling; repository operations require the local dev/preview server.

## Model the domain

- **Build:** start typing a definition, question, or promise immediately. Multiline input survives reload. Build retains the exact text in the Notebook until you assign it to a bounded context and a domain kind. Unassigned notes stay outside canonical YAML.
- **Enter and leave:** select a bordered context region to enter its language. Select an aggregate or another domain entry to inspect its recorded relationships. Back goes up one level; the project breadcrumb returns to the project. Find reaches every entry. Spatial pages contain at most eight subjects, including external references; Table uses its own forty-row pages.
- **Read the drawing:** overview region area follows the full recorded entry count, labeled explicitly. Its contour follows saved arrangement; these are presentation properties, not evidence of runtime population, maturity, or importance. Within a context, aggregate boundaries contain actual members. Other domain kinds use distinct marks. Context influence and observed imports remain different relationships.
- **Author:** choose Bounded context, Aggregate, Entity, Value object, Domain event, Domain service, Specification, Repository, Factory, Open question, Invariant, or Assertion. Invariants attach to aggregates or values; assertions attach to an aggregate operation. Identity and aggregate ownership are explicit. Unfinished editor text survives presentation changes, governance inspection, and reload.
- **Arrange:** drag to orbit, right-drag to pan, scroll to zoom, and Shift-drag an entry to change its saved position. Frame, Plan, and Zoom controls stay visible. Arrangement never changes canonical meaning. The Context field changes semantic membership.
- **Switch presentation:** Spatial and Table use one semantic model and editing desk. A renderer switch preserves selections, unfinished input, and history. Layout coordinates and colors are stored separately from semantic records.
- **Compare:** Tools → Review → Model snapshot captures the model and layout. The comparison distinguishes arrangement changes from semantic changes. This is separate from the ArcLint Baseline used to acknowledge findings.

Onyx is one persistent female dog support avatar, independent of the scene and current context. The avatar opens a support conversation with local guidance, direct repository actions, and an editable AI request for another harness. No live AI provider is connected; free-form questions outside the local guide's supported actions are retained as notebook drafts rather than answered with invented analysis.

## Inspect and apply architecture

**Rules**, **Findings**, **Paths**, and **Check code** are available from the main view. The repository workspace provides Code paths, Zones, Rules, Patterns, and Report sections.

- Browse real repository directories, copy relative paths, and request `arclint context <path>` or `arclint context --zone <name>`. A missing path is identified as hypothetical policy, not an observed file. A file can match several Zones. Source associations come from actual CLI evidence; matching display names alone do not establish them.
- Inspect exact configured Rule IDs, Constraints, Scopes, authored Rationale, provenance, and available assurance. Zone paths and local layer ordering are shown separately from bounded-context membership.
- Inspect offline Patterns and configured Bindings. Imported Pattern files remain references until their policy is applied. No skill-manager installation UI or runtime is assumed.
- Run the actual configured check against files on disk. Retain returned paths, Rule IDs, statuses, multiplicity, timestamps, and exit code. Report filters distinguish active findings, baselined findings, suppressions, and other Diagnostics. Missing assurance or fingerprints remain absent; an empty diagnostic list is not a complete table of passed Rules.
- **Review Baseline adoption** assesses real unbaselined findings before offering Capture or Replace. The bridge checks Rules, Domain, Baseline, and returned diagnostics again before invoking native adoption. Because this CLI omits its full outcome table, the web action requires exact-assurance enabled Rules and refuses coverage or operational gaps. The native command scans again; its final source inputs are not frozen. The review states this limit. Adoption acknowledges findings and preserves their native fingerprints; it does not repair code.
- **Edit policy** prepares `rules.arclint.yaml`, including Rules, Zones, and Pattern Bindings. Review changes validates the candidate with ArcLint in an isolated workspace and shows its exact diff. A changed Constraint requires a new Rule ID.
- **Review repository changes** in the domain editor saves the current definition locally and prepares canonical `domain.arclint.yaml`. The same action is available from Tools → Files & workspace → Save domain to repository.
- **Apply to repository** is an explicit action after preview. The server rechecks source hashes, validates again, and atomically replaces only the reviewed document. If either companion document changed, the preview is refused and the draft remains. Applying clears stale repository and Report caches; Check code evaluates the new on-disk inputs.

Repository reads and writes are limited to the configured local repository. The bridge rejects cross-origin access, traversal, option injection, and escaping symlinks. Document writes require a short-lived preview token, unchanged source versions, and regular destination files. Commands have time and output limits. Tests execute writes only in temporary fixture repositories.

## Keep and exchange work

V1 saves migrate into a version-2 envelope containing separate semantic/layout records, Model snapshot, Pattern references, view state, exact notebook text, and editor drafts. The legacy save is retained. Undo/Redo spans both presentations and reverses semantic edits, arrangement, snapshots, and workspace replacement within the current session.

Tools → Files & workspace → Export provides:

- **Workspace JSON:** the complete version-2 envelope, including unfinished drafts, snapshot, and references. Import restores the complete workspace.
- **Domain YAML:** canonical ArcLint language, preserving assertion keys, invariant keys, identities, aliases, ownership, event metadata, and context relation descriptions. Adding, removing, and renaming entries in imported domains is supported. Incompatible edits fail explicitly rather than silently erasing metadata. Layout, browser drafts, comments, and formatting are not canonical language.

The optional user/need brief in Tools remains browser-local by model name. It is not part of canonical YAML. Local storage is specific to browser and origin; download a workspace to keep a portable copy.

Keyboard: `/` opens Find; `C` opens the domain builder; `F` frames the view; Ctrl/Cmd+Z and Ctrl/Cmd+Shift+Z undo and redo. Focused editors suppress background authoring shortcuts. Reduced motion is respected. A selectable fallback remains available if WebGL cannot initialize.

## Verify

```sh
npm test
npm run build
npm run test:browser
```

Unit coverage exercises canonical metadata and structural edits, separate layout, complete-save migration, shared history, exact draft retention, actual CLI reports, repository boundaries, and reviewed write conflicts. Browser coverage uses the real ArcLint domain and an unrelated Library domain, checks both presentations and mobile navigation, and exercises authoring, typed contracts, Onyx, snapshots, full-workspace exchange, and real repository evidence.

Run `make check` from the repository root before completing work. The web implementation does not change Go domain or CLI semantics.

## Model and design

The user accepted the [shared model](model/README.md). [DESIGN.md](DESIGN.md) records its implemented visual and interaction decisions. The [model board](model/index.html) remains an explanatory example, separate from the application. Existing ArcLint terminology governs both presentations; unresolved product decisions remain explicit in the model.
