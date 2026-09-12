# Domain Studio implementation brief

Prompt workflow: mode=run runtime=codex layer=user task=A,D. The user has ended the interview, explicitly authorized implementation and subagents, and requested minimal discussion. This brief is the concrete revised prompt being executed.

Build a working spatial domain editor. Prioritize creating, defining, grouping, connecting, revising, saving, and loading domain concepts. Use bskilled only as an illustrative draft domain, never as a UI reference or a settled domain classification. Record uncertain kinds as unclassified. Keep ArcLint's existing domain and CLI behavior intact.

Art direction: Homeworld tactical navigation, No Man's Sky atmospheric depth, Gurren Lagann angular silhouettes and controlled energy. Design a world with readable labels and direct actions, not a conventional website. No game objectives, forced travel, invented metrics, or decorative fake controls.

Tokens: deep-space #080f18, hull #14242f, text #e5ecee, subdued #91a6b5, amber #f6b85c, ion #66d8df; warning #ef9377. Typography: Bahnschrift/Arial for headings and body, ui-monospace for coordinates and utility labels. Full-viewport scene, compact command bar, collapsible model tree, context inspector, bottom tactical controls. Single dark theme is intentional for this visual world. Respect reduced motion and provide keyboard-accessible model navigation and a top view.

Implement real local persistence and import/export, reversible editing, validation, baseline capture and structural comparison. Patterns must be presented honestly: imported Pattern definitions can be inspected, while model checks are not represented as full ArcLint conformance. AI work is a prepared request for the selected model area; do not pretend to execute an AI or refactor code without a connected runtime.

Acceptance: a user can create a context and concept, edit their meaning, connect concepts, save/reload without losing data, export/import, capture a baseline and see subsequent changes, inspect model issues, and navigate the scene. Validate with meaningful unit tests and actual browser interactions, inspect screenshots, run the repository make check gate. Deliver a running local preview with concise limitations.
