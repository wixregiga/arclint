/*
 * ArcLint Studio's stylesheet, held as a string so the mount entry can
 * place it inside Studio's shadow root at mount time rather than letting
 * a stylesheet import touch the host document. BWrite's quiet light
 * authoring workspace carried over: palette, radii, shadows, kind badges.
 */
export const studioStylesheet: string = String.raw`
:host {
  display: block;
  height: 100%;
  min-height: 0;
}

.st-root {
  --bg: #f6f6f3;
  --surface: #ffffff;
  --surface-sunken: #fafaf8;
  --ink: #1f2430;
  --ink-soft: #4a5160;
  --ink-muted: #737a87;
  --line: #e4e4df;
  --line-strong: #d3d3cc;
  --focus: #3b5bdb;
  --accent: #2f7d72;
  --accent-ink: #1f5f56;
  --accent-wash: rgba(47, 125, 114, 0.08);
  --accent-wash-strong: rgba(47, 125, 114, 0.16);
  --proposed: #8a5a00;
  --proposed-wash: #fbf1dc;
  --danger: #b42318;
  --danger-wash: #fdecea;
  --success-wash: #e8f4f1;
  --radius-lg: 16px;
  --radius-md: 10px;
  --radius-sm: 6px;
  --shadow-card: 0 1px 2px rgba(31, 36, 48, 0.04), 0 8px 24px -6px rgba(31, 36, 48, 0.1), 0 24px 48px -24px rgba(31, 36, 48, 0.12);
  --shadow-pop: 0 2px 4px rgba(31, 36, 48, 0.06), 0 12px 32px -8px rgba(31, 36, 48, 0.22);
  --font-ui: "Inter", ui-sans-serif, system-ui, -apple-system, "Segoe UI", Roboto, "Helvetica Neue", Arial, sans-serif;
  --font-mono: ui-monospace, "JetBrains Mono", "SF Mono", "Cascadia Code", Menlo, Consolas, monospace;

  /* Concept colors: BWrite's kinds, plus ArcLint's rules and questions. Each reads
     as white-on-color at 4.5:1 and as text on the page. */
  --kind-bounded_context: #493a89;
  --kind-aggregate: #166eb0;
  --kind-entity: #067031;
  --kind-value_object: #843f01;
  --kind-domain_event: #cf1033;
  --kind-domain_service: #c01790;
  --kind-specification: #776b1b;
  --kind-invariant: #5a4fcf;
  --kind-assertion: #8a3ab9;
  --kind-question: #4b4e4c;

  height: 100%;
  min-height: 0;
  font-family: var(--font-ui);
  font-size: 14px;
  line-height: 1.5;
  color: var(--ink);
  background-color: var(--bg);
  background-position: -1px -1px;
  -webkit-font-smoothing: antialiased;
  color-scheme: light;
}

.st-root *,
.st-root *::before,
.st-root *::after {
  box-sizing: border-box;
}

.st-root button,
.st-root input,
.st-root select,
.st-root textarea {
  font: inherit;
  color: inherit;
}

.st-root :focus-visible {
  outline: 2px solid var(--focus);
  outline-offset: 2px;
}

.st-root kbd {
  font-family: var(--font-mono);
  font-size: 11px;
  padding: 1px 5px;
  border: 1px solid var(--line-strong);
  border-bottom-width: 2px;
  border-radius: 4px;
  background: var(--surface);
  color: var(--ink-muted);
}

/* Layout: a top bar over navigator, main and, in map mode, an inspector */

.st-app {
  height: 100%;
  width: 100%;
  min-width: 0;
  grid-template-columns: minmax(0, 1fr);
  display: grid;
  grid-template-rows: auto minmax(0, 1fr) auto;
}

.st-topbar {
  display: flex;
  align-items: center;
  gap: 12px;
  padding: 8px 16px;
  border-bottom: 1px solid var(--line);
  background: rgba(250, 250, 248, 0.9);
}

.st-brand {
  display: flex;
  align-items: center;
  gap: 8px;
  min-width: 0;
  flex: 0 1 auto;
}

.st-brand__app {
  font-weight: 650;
  letter-spacing: -0.01em;
  white-space: nowrap;
}

.st-brand__divider {
  color: var(--line-strong);
  font-size: 18px;
  font-weight: 300;
}

.st-brand__project {
  border: 0;
  background: none;
  padding: 2px 6px;
  border-radius: var(--radius-sm);
  font-weight: 600;
  white-space: nowrap;
  overflow: hidden;
  text-overflow: ellipsis;
  max-width: 240px;
  cursor: pointer;
}

.st-brand__project:hover {
  background: var(--accent-wash);
}

.st-brand__project.is-empty {
  color: var(--ink-muted);
  font-weight: 500;
  font-style: italic;
}

.st-brand__source {
  color: var(--ink-muted);
  font-size: 12px;
  font-family: var(--font-mono);
  white-space: nowrap;
}

.st-topbar__modes {
  display: inline-flex;
  padding: 2px;
  border: 1px solid var(--line);
  border-radius: 999px;
  background: var(--surface);
}

.st-mode {
  border: 0;
  background: none;
  padding: 4px 12px;
  border-radius: 999px;
  font-size: 13px;
  font-weight: 550;
  color: var(--ink-soft);
  cursor: pointer;
}

.st-mode[aria-selected="true"] {
  background: var(--accent);
  color: #fff;
}

.st-accepted {
  font-size: 12.5px;
  color: var(--ink-soft);
  white-space: nowrap;
}

.st-accepted--unavailable {
  color: var(--ink-muted);
  font-style: italic;
}

.st-body {
  display: grid;
  grid-template-columns: 272px minmax(0, 1fr);
  min-height: 0;
}

.st-body--map {
  grid-template-columns: 272px minmax(0, 1fr) 400px;
}

.st-body--failed,
.st-body--loading {
  display: flex;
  justify-content: center;
  align-items: flex-start;
  padding: 48px 24px;
}

.st-main {
  min-width: 0;
  min-height: 0;
  overflow-y: auto;
  display: flex;
  justify-content: center;
  align-items: flex-start;
  padding: 28px 24px 72px;
}

.st-main--map {
  padding: 0;
  display: block;
  overflow: hidden;
}

.st-inspector {
  min-height: 0;
  overflow-y: auto;
  border-left: 1px solid var(--line);
  background: rgba(250, 250, 248, 0.86);
  padding: 16px;
}

.st-inspector .st-card {
  padding: 20px 22px;
  box-shadow: none;
}

/* Navigator */

.st-navigator {
  min-height: 0;
  overflow-y: auto;
  padding: 12px 10px 24px;
  border-right: 1px solid var(--line);
  background: rgba(250, 250, 248, 0.86);
  font-size: 13.5px;
}

.st-navigator__head {
  display: flex;
  align-items: center;
  justify-content: space-between;
  gap: 8px;
  padding: 0 0 8px 8px;
}

.st-navigator__heading {
  margin: 0;
  font-size: 11px;
  font-weight: 600;
  letter-spacing: 0.06em;
  text-transform: uppercase;
  color: var(--ink-muted);
}

.st-navigator__empty {
  margin: 4px 8px 8px;
  color: var(--ink-muted);
  font-size: 13px;
  list-style: none;
}

.st-nav-context {
  margin-top: 10px;
  padding-top: 8px;
  border-top: 1px solid var(--line);
}

.st-nav-list {
  list-style: none;
  margin: 0;
  padding: 0;
  display: grid;
  gap: 1px;
}

.st-nav-item {
  min-width: 0;
}

.st-nav-row {
  display: flex;
  align-items: center;
  min-width: 0;
  border-radius: 8px;
  padding-left: 8px;
}

.st-nav-row:hover {
  background: rgba(31, 36, 48, 0.04);
}

.st-nav-row.is-current {
  background: var(--surface);
  box-shadow: 0 0 0 1px var(--line), 0 1px 2px rgba(31, 36, 48, 0.05);
}

.st-nav-row--context .st-nav-row__main {
  font-weight: 600;
}

.st-nav-row__main {
  flex: 1;
  min-width: 0;
  display: flex;
  align-items: center;
  gap: 7px;
  padding: 5px 8px 5px 2px;
  border: 0;
  border-radius: 8px;
  background: none;
  text-align: left;
  cursor: pointer;
}

.st-nav-row__main[aria-current="true"] {
  font-weight: 600;
}

.st-nav-row__name {
  flex: 1;
  min-width: 0;
  white-space: nowrap;
  overflow: hidden;
  text-overflow: ellipsis;
}

.st-nav-fold {
  flex: none;
  width: 18px;
  height: 22px;
  border: 0;
  background: none;
  color: var(--ink-muted);
  font-size: 10px;
  cursor: pointer;
  padding: 0;
}

.st-nav-fold--none {
  display: inline-block;
}

.st-nav-add {
  flex: none;
  width: 24px;
  height: 24px;
  margin-right: 2px;
  border: 0;
  border-radius: var(--radius-sm);
  background: none;
  color: var(--ink-muted);
  font-size: 16px;
  line-height: 1;
  cursor: pointer;
}

.st-nav-add:hover,
.st-nav-add[aria-expanded="true"] {
  background: var(--accent-wash);
  color: var(--accent-ink);
}

.st-nav-menu {
  list-style: none;
  margin: 4px 6px 8px;
  padding: 6px;
  background: var(--surface);
  border: 1px solid var(--line);
  border-radius: var(--radius-md);
  box-shadow: var(--shadow-pop);
  display: grid;
  gap: 2px;
}

.st-nav-menu button {
  width: 100%;
  display: flex;
  align-items: flex-start;
  gap: 8px;
  padding: 6px 8px;
  border: 0;
  border-radius: var(--radius-sm);
  background: none;
  text-align: left;
  cursor: pointer;
}

.st-nav-menu button:hover {
  background: var(--accent-wash);
}

.st-nav-menu button b {
  display: block;
  font-weight: 600;
  font-size: 13px;
}

.st-nav-menu button small {
  display: block;
  color: var(--ink-muted);
  font-size: 11.5px;
  line-height: 1.35;
}

/* Cards */

.st-card {
  width: 100%;
  max-width: 880px;
  background: var(--surface);
  border: 1px solid var(--line);
  border-radius: var(--radius-lg);
  box-shadow: var(--shadow-card);
  padding: 28px 36px 26px;
}

.st-card--failure {
  max-width: 520px;
}

.st-card__head {
  display: flex;
  flex-direction: column;
  gap: 6px;
  padding-bottom: 16px;
  border-bottom: 1px solid var(--line);
  margin-bottom: 16px;
}

.st-eyebrow {
  font-size: 11.5px;
  font-weight: 600;
  letter-spacing: 0.06em;
  text-transform: uppercase;
  color: var(--ink-muted);
}

.st-card__title-row {
  display: flex;
  align-items: center;
  flex-wrap: wrap;
  gap: 10px;
}

.st-card__title {
  margin: 0;
  font-size: 24px;
  font-weight: 650;
  letter-spacing: -0.015em;
  line-height: 1.25;
  overflow-wrap: anywhere;
}

.st-card__title.is-empty {
  color: var(--ink-muted);
  font-weight: 500;
  font-style: italic;
}

.st-card__kind {
  color: var(--ink-muted);
  font-size: 13px;
}

.st-aliases {
  display: flex;
  flex-wrap: wrap;
  align-items: center;
  gap: 6px;
  font-size: 13px;
}

.st-aliases__label {
  color: var(--ink-muted);
}

.st-chip {
  display: inline-flex;
  align-items: center;
  padding: 0 8px;
  height: 20px;
  border: 1px solid var(--line-strong);
  border-radius: 999px;
  color: var(--ink-soft);
  font-size: 12px;
  font-weight: 550;
  white-space: nowrap;
}

.st-reading {
  margin: 0 0 16px;
  font-size: 16.5px;
  line-height: 1.6;
  color: var(--ink);
  overflow-wrap: anywhere;
}

.st-reading--missing {
  color: var(--danger);
  font-size: 14.5px;
  font-style: italic;
}

.st-properties {
  margin: 0 0 16px;
  display: grid;
  gap: 6px;
}

.st-property {
  display: grid;
  grid-template-columns: 120px 1fr;
  gap: 8px;
  align-items: baseline;
}

.st-property dt {
  color: var(--ink-muted);
  font-size: 12.5px;
  font-weight: 600;
}

.st-property dd {
  margin: 0;
  display: flex;
  flex-wrap: wrap;
  gap: 6px;
}

.st-value {
  overflow-wrap: anywhere;
}

.st-value--code {
  font-family: var(--font-mono);
  font-size: 12.5px;
  background: var(--surface-sunken);
  padding: 1px 6px;
  border-radius: 4px;
}

.st-missing {
  color: var(--danger);
  font-style: italic;
  font-size: 13px;
}

.st-section {
  margin: 18px 0 0;
  padding-top: 14px;
  border-top: 1px solid var(--line);
}

.st-section__heading {
  margin: 0 0 8px;
  font-size: 11.5px;
  font-weight: 600;
  letter-spacing: 0.06em;
  text-transform: uppercase;
  color: var(--ink-muted);
}

.st-connections {
  list-style: none;
  margin: 0;
  padding: 0;
  display: grid;
  gap: 4px;
}

.st-connections li {
  display: flex;
  flex-wrap: wrap;
  align-items: center;
  gap: 8px;
  min-width: 0;
}

.st-connections__label {
  min-width: 120px;
  color: var(--ink-muted);
  font-size: 12.5px;
  font-weight: 600;
}

.st-connections__target {
  display: inline-flex;
  align-items: center;
  gap: 6px;
  padding: 3px 8px 3px 4px;
  border: 1px solid transparent;
  border-radius: 999px;
  background: var(--surface-sunken);
  cursor: pointer;
  max-width: 100%;
  min-width: 0;
  text-align: left;
}

.st-connections__target > span {
  overflow-wrap: anywhere;
}

.st-inspector .st-connections__label {
  min-width: 96px;
}

.st-inspector .st-connections__target small {
  display: none;
}

.st-inspector .st-connections__note {
  padding-left: 0;
}

.st-connections__target:hover {
  border-color: var(--line-strong);
  background: var(--accent-wash);
}

.st-connections__target small {
  color: var(--ink-muted);
  overflow: hidden;
  white-space: nowrap;
  text-overflow: ellipsis;
  max-width: 320px;
}

.st-connections__note {
  flex-basis: 100%;
  padding-left: 128px;
  color: var(--ink-soft);
  font-size: 13px;
}

.st-group {
  margin-top: 8px;
}

.st-group__heading {
  margin: 0 0 4px;
  font-size: 12.5px;
  font-weight: 600;
  color: var(--ink-soft);
}

.st-group__items {
  list-style: none;
  margin: 0;
  padding: 0;
  display: grid;
  gap: 3px;
}

.st-group__items li {
  display: flex;
  align-items: baseline;
  gap: 8px;
  min-width: 0;
}

.st-group__items small {
  color: var(--ink-muted);
  overflow: hidden;
  white-space: nowrap;
  text-overflow: ellipsis;
  flex: 1;
  min-width: 0;
}

.st-counts {
  margin: 0 0 8px;
  color: var(--ink-soft);
  font-size: 13px;
}

.st-note {
  margin: 8px 0;
  color: var(--ink-muted);
  font-size: 13px;
}

.st-differences {
  list-style: none;
  margin: 0;
  padding: 0;
  display: grid;
  gap: 4px;
}

.st-difference {
  display: flex;
  align-items: center;
  gap: 8px;
  flex-wrap: wrap;
}

.st-difference small {
  color: var(--ink-muted);
}

.st-card__actions {
  display: flex;
  flex-wrap: wrap;
  align-items: center;
  gap: 8px;
  margin-top: 20px;
  padding-top: 16px;
  border-top: 1px solid var(--line);
}

.st-card__actions-grow {
  flex: 1;
}

.st-unsaved {
  display: flex;
  flex-wrap: wrap;
  align-items: center;
  gap: 10px;
  margin: 0 0 16px;
  padding: 10px 12px;
  border-radius: var(--radius-md);
  background: var(--proposed-wash);
  font-size: 13px;
}

/* Buttons and badges */

.st-button {
  display: inline-flex;
  align-items: center;
  gap: 8px;
  padding: 7px 14px;
  border: 1px solid var(--line-strong);
  border-radius: 8px;
  background: var(--surface);
  font-size: 13.5px;
  font-weight: 550;
  cursor: pointer;
  white-space: nowrap;
}

.st-button:hover {
  background: var(--surface-sunken);
  border-color: #c4c4bc;
}

.st-button:disabled {
  color: var(--ink-muted);
  background: transparent;
  cursor: default;
  opacity: 0.6;
}

.st-button--small {
  padding: 4px 10px;
  font-size: 12.5px;
}

.st-button--quiet {
  background: rgba(255, 255, 255, 0.7);
  border-color: var(--line);
}

.st-button--primary {
  background: var(--accent);
  border-color: var(--accent);
  color: #fff;
}

.st-button--primary:hover {
  background: var(--accent-ink);
  border-color: var(--accent-ink);
}

.st-button--primary:disabled {
  background: var(--accent);
  color: #fff;
}

.st-button--danger {
  background: var(--danger);
  border-color: var(--danger);
  color: #fff;
}

.st-button--danger:hover {
  background: #8f1c13;
  border-color: #8f1c13;
}

.st-link-button {
  display: inline-flex;
  align-items: center;
  gap: 4px;
  padding: 2px 0;
  border: 0;
  background: none;
  color: var(--focus);
  font-weight: 550;
  font-size: inherit;
  cursor: pointer;
  text-align: left;
}

.st-link-button:hover {
  text-decoration: underline;
}

.st-badge {
  display: inline-flex;
  align-items: center;
  padding: 0 8px;
  height: 20px;
  border-radius: 999px;
  font-size: 11.5px;
  font-weight: 600;
  line-height: 1;
  white-space: nowrap;
}

.st-badge--accepted {
  background: var(--success-wash);
  color: var(--accent-ink);
}

.st-badge--proposed {
  background: var(--proposed-wash);
  color: var(--proposed);
}

.st-badge--draft {
  background: var(--proposed-wash);
  color: var(--proposed);
  border: 1px dashed var(--proposed);
}

.st-badge--removed {
  background: var(--danger-wash);
  color: var(--danger);
}

.st-badge--muted {
  background: var(--surface-sunken);
  color: var(--ink-muted);
}

.st-kind-badge {
  --kind-color: var(--kind-question);
  display: inline-flex;
  align-items: center;
  justify-content: center;
  flex: none;
  width: 20px;
  height: 20px;
  border-radius: 50%;
  background-color: var(--kind-color);
  color: #fff;
  font-size: 11px;
  font-weight: 700;
  line-height: 1;
  user-select: none;
}

.st-kind-badge--small {
  width: 16px;
  height: 16px;
  font-size: 9px;
}

.st-kind-badge--large {
  width: 28px;
  height: 28px;
  font-size: 14px;
}

.st-kind-badge--proposed {
  outline: 2px dashed var(--kind-color);
  outline-offset: 1px;
}

.st-kind-badge--bounded_context { --kind-color: var(--kind-bounded_context); }
.st-kind-badge--aggregate { --kind-color: var(--kind-aggregate); }
.st-kind-badge--entity { --kind-color: var(--kind-entity); }
.st-kind-badge--value_object { --kind-color: var(--kind-value_object); }
.st-kind-badge--domain_event { --kind-color: var(--kind-domain_event); }
.st-kind-badge--domain_service { --kind-color: var(--kind-domain_service); }
.st-kind-badge--specification { --kind-color: var(--kind-specification); }
.st-kind-badge--invariant { --kind-color: var(--kind-invariant); }
.st-kind-badge--assertion { --kind-color: var(--kind-assertion); }
.st-kind-badge--question { --kind-color: var(--kind-question); }

/* Forms */

.st-entry-form {
  display: grid;
  gap: 14px;
}

.st-field {
  display: grid;
  gap: 4px;
}

.st-field label {
  font-size: 13px;
  font-weight: 600;
}

.st-field__required {
  color: var(--ink-muted);
  font-weight: 500;
  font-size: 12px;
}

.st-field__optional {
  color: var(--ink-muted);
  font-weight: 400;
  font-size: 12px;
}

.st-field input,
.st-field textarea,
.st-field select {
  width: 100%;
  padding: 8px 10px;
  border: 1px solid var(--line-strong);
  border-radius: 8px;
  background: var(--surface);
  font-size: 14px;
}

.st-field textarea {
  resize: vertical;
  min-height: 64px;
  line-height: 1.5;
}

.st-field input:focus-visible,
.st-field textarea:focus-visible,
.st-field select:focus-visible {
  outline-offset: 0;
}

.st-field [aria-invalid="true"] {
  border-color: var(--proposed);
}

.st-field__hint {
  margin: 0;
  font-size: 12px;
  color: var(--ink-muted);
}

.st-advanced {
  display: grid;
  gap: 12px;
}

.st-form-error {
  margin: 0;
  padding: 10px 12px;
  border-radius: var(--radius-md);
  background: var(--danger-wash);
  color: var(--danger);
  font-size: 13.5px;
  overflow-wrap: anywhere;
}

.st-form-note {
  margin: 0;
  font-size: 13px;
  color: var(--proposed);
}

.st-form-actions {
  display: flex;
  flex-wrap: wrap;
  align-items: center;
  gap: 8px;
}

.st-form-keys {
  margin-left: auto;
  color: var(--ink-muted);
  font-size: 12px;
}

.st-confirm {
  padding: 12px 14px;
  border-radius: var(--radius-md);
  background: var(--danger-wash);
  flex-basis: 100%;
}

.st-confirm p {
  margin: 0 0 10px;
}

/* Search */

.st-search {
  position: relative;
  flex: 1 1 320px;
  min-width: 180px;
  max-width: 520px;
}

.st-search input {
  width: 100%;
  padding: 7px 12px;
  border: 1px solid var(--line-strong);
  border-radius: 8px;
  background: var(--surface);
  font-size: 13.5px;
}

.st-search__hits {
  position: absolute;
  top: calc(100% + 6px);
  left: 0;
  right: 0;
  z-index: 30;
  margin: 0;
  padding: 6px;
  list-style: none;
  max-height: min(60vh, 520px);
  overflow: auto;
  background: var(--surface);
  border: 1px solid var(--line-strong);
  border-radius: var(--radius-md);
  box-shadow: var(--shadow-pop);
}

.st-search__hit {
  display: flex;
  align-items: flex-start;
  gap: 8px;
  padding: 7px 8px;
  border-radius: var(--radius-sm);
  cursor: pointer;
}

.st-search__hit.is-active {
  background: var(--accent-wash-strong);
  outline: 1px solid var(--accent);
  outline-offset: -1px;
}

.st-search__hit-main {
  display: grid;
  gap: 1px;
  min-width: 0;
}

.st-search__hit-name {
  font-weight: 600;
  font-size: 13.5px;
}

.st-search__hit-alias {
  font-weight: 400;
  color: var(--ink-soft);
}

.st-search__hit-where {
  color: var(--ink-muted);
  font-size: 12px;
}

.st-search__hit-snippet {
  color: var(--ink-soft);
  font-size: 12.5px;
}

.st-search__empty {
  padding: 10px;
  color: var(--ink-muted);
  font-size: 13.5px;
}

/* Map */

.st-map {
  height: 100%;
  display: grid;
  grid-template-rows: auto minmax(0, 1fr);
}

.st-map__contexts {
  padding: 12px 16px 8px;
  border-bottom: 1px solid var(--line);
  overflow: auto;
  background: rgba(250, 250, 248, 0.6);
  max-height: 320px;
}

.st-context-map {
  display: block;
  overflow: visible;
}

.st-context-map__context rect {
  fill: var(--surface);
  stroke: var(--line-strong);
  stroke-width: 1.5;
  cursor: pointer;
}

.st-context-map__context text {
  font-size: 14px;
  font-weight: 600;
  fill: var(--ink);
  pointer-events: none;
}

.st-context-map__context.is-active rect {
  stroke: var(--accent);
  stroke-width: 2;
}

.st-context-map__context.is-selected rect {
  fill: var(--accent-wash);
}

.st-context-map__context.is-proposed rect {
  stroke-dasharray: 6 4;
  stroke: var(--proposed);
}

.st-context-map__context:hover rect {
  fill: var(--surface-sunken);
}

.st-context-map__relation line {
  stroke: var(--ink-soft);
  stroke-width: 1.5;
  color: var(--ink-soft);
}

.st-context-map__relation text {
  font-size: 11px;
  fill: var(--ink-muted);
}

.st-map__context {
  display: grid;
  grid-template-rows: auto minmax(0, 1fr);
  min-height: 0;
}

.st-map__bar {
  display: flex;
  align-items: center;
  gap: 16px;
  padding: 10px 16px 6px;
}

.st-map__bar .st-section__heading {
  margin: 0;
}

.st-map__legend {
  display: inline-flex;
  align-items: center;
  gap: 6px;
  font-size: 12px;
  color: var(--ink-muted);
}

.st-map__legend-line {
  display: inline-block;
  width: 22px;
  height: 0;
  border-top: 2px solid var(--ink-soft);
  margin-left: 8px;
}

.st-map__legend-line--raises {
  border-top-color: var(--kind-domain_event);
}

.st-map__legend-line--identity {
  border-top-style: dashed;
  border-top-color: var(--kind-value_object);
}

.st-zoom {
  margin-left: auto;
  display: inline-flex;
  border: 1px solid var(--line);
  border-radius: 999px;
  background: var(--surface);
}

.st-zoom button {
  border: 0;
  background: none;
  padding: 3px 10px;
  color: var(--ink-soft);
  font-size: 13px;
  cursor: pointer;
}

.st-map__stage {
  min-height: 0;
  overflow: auto;
  padding: 8px 16px 24px;
}

.st-concept-map {
  display: block;
}

.st-concept-map__line {
  stroke: var(--line-strong);
  stroke-width: 1.5;
}

.st-concept-map__line--raises {
  stroke: var(--kind-domain_event);
  stroke-opacity: 0.6;
}

.st-concept-map__line--identity {
  stroke: var(--kind-value_object);
  stroke-dasharray: 4 3;
  stroke-opacity: 0.8;
}

.st-concept-map__node {
  cursor: pointer;
}

.st-concept-map__hit {
  fill: transparent;
  pointer-events: all;
}

.st-concept-map__node:hover .st-concept-map__hit {
  fill: var(--accent-wash);
}

.st-concept-map__node circle {
  fill: var(--kind-question);
  stroke: #fff;
  stroke-width: 1.5;
}

.st-concept-map__node--bounded_context circle { fill: var(--kind-bounded_context); }
.st-concept-map__node--aggregate circle { fill: var(--kind-aggregate); }
.st-concept-map__node--entity circle { fill: var(--kind-entity); }
.st-concept-map__node--value_object circle { fill: var(--kind-value_object); }
.st-concept-map__node--domain_event circle { fill: var(--kind-domain_event); }
.st-concept-map__node--domain_service circle { fill: var(--kind-domain_service); }
.st-concept-map__node--specification circle { fill: var(--kind-specification); }
.st-concept-map__node--invariant circle { fill: var(--kind-invariant); }
.st-concept-map__node--assertion circle { fill: var(--kind-assertion); }
.st-concept-map__node--question circle { fill: var(--kind-question); }

.st-concept-map__node.is-selected circle {
  stroke: var(--focus);
  stroke-width: 3;
}

.st-concept-map__node.is-proposed circle {
  stroke: var(--proposed);
  stroke-dasharray: 2 2;
  stroke-width: 2;
}

.st-concept-map__mark {
  font-size: 7px;
  font-weight: 700;
  fill: #fff;
  pointer-events: none;
}

.st-concept-map__label {
  font-size: 12px;
  fill: var(--ink);
}

.st-concept-map__node:hover .st-concept-map__label {
  text-decoration: underline;
}

.st-map__empty {
  padding: 24px;
}

/* Status line */

.st-status {
  display: flex;
  align-items: center;
  gap: 12px;
  padding: 4px 16px;
  border-top: 1px solid var(--line);
  background: rgba(250, 250, 248, 0.9);
  font-size: 12.5px;
  color: var(--ink-soft);
  min-height: 28px;
}

.st-status__grow {
  flex: 1;
}

.st-status__error {
  color: var(--danger);
}

.st-status__counts {
  color: var(--ink-muted);
  white-space: nowrap;
}

/* Features: navigator, the feature card with its editor, the language at hand */

.st-body--features {
  grid-template-columns: 272px minmax(0, 1fr) 380px;
}

.st-main--feature {
  align-items: stretch;
  padding: 20px 24px 72px;
}

.st-card--feature {
  width: 100%;
  max-width: 860px;
  margin: 0 auto;
  display: grid;
  gap: 14px;
}

.st-card--feature-empty {
  max-width: 560px;
  margin: 28px auto 0;
}

.st-feature-head {
  display: grid;
  gap: 4px;
}

.st-feature-title {
  width: 100%;
  border: 0;
  padding: 2px 0;
  background: none;
  font-size: 24px;
  font-weight: 650;
  letter-spacing: -0.015em;
  line-height: 1.25;
}

.st-feature-title::placeholder {
  color: var(--ink-muted);
  font-weight: 500;
}

.st-feature-title:focus-visible {
  outline: none;
  box-shadow: inset 0 -2px 0 var(--accent);
}

.st-editor-host {
  border: 1px solid var(--line);
  border-radius: var(--radius-md);
  background: var(--surface-sunken);
  transition: border-color 120ms ease, box-shadow 120ms ease;
}

.st-editor-host:focus-within {
  border-color: var(--line-strong);
  box-shadow: 0 0 0 3px rgba(59, 91, 219, 0.12);
}

.st-root .st-editor-host .cm-editor {
  border-radius: inherit;
  font-family: var(--font-ui);
  font-size: 15px;
}

.st-root .st-editor-host .cm-editor.cm-focused {
  outline: none;
}

.st-root .st-editor-host .cm-scroller {
  font-family: inherit;
  line-height: 1.75;
  min-height: 320px;
}

.st-root .st-editor-host .cm-content {
  padding: 16px 0;
  caret-color: var(--ink);
}

.st-root .st-editor-host .cm-line {
  padding: 0 18px;
}

.st-root .st-editor-host .cm-placeholder {
  color: var(--ink-muted);
}

.st-root .st-editor-host .cm-selectionBackground,
.st-root .st-editor-host .cm-content ::selection {
  background: rgba(59, 91, 219, 0.16) !important;
}

.st-root .st-editor-host .cm-focused .cm-selectionBackground {
  background: rgba(59, 91, 219, 0.22) !important;
}

/* Uses of the language in the prose. A cue is quiet; a tie is marked. */

.st-use {
  --use-color: var(--kind-question);
  border-radius: 3px;
  text-decoration-line: underline;
  text-decoration-color: var(--use-color);
  text-decoration-thickness: 1.5px;
  text-underline-offset: 3px;
  cursor: help;
}

.st-use--bounded_context { --use-color: var(--kind-bounded_context); }
.st-use--aggregate { --use-color: var(--kind-aggregate); }
.st-use--entity { --use-color: var(--kind-entity); }
.st-use--value_object { --use-color: var(--kind-value_object); }
.st-use--domain_event { --use-color: var(--kind-domain_event); }
.st-use--domain_service { --use-color: var(--kind-domain_service); }
.st-use--specification { --use-color: var(--kind-specification); }
.st-use--invariant { --use-color: var(--kind-invariant); }
.st-use--assertion { --use-color: var(--kind-assertion); }

.st-use--cue {
  text-decoration-style: dotted;
}

.st-use--tied {
  text-decoration-style: solid;
  text-decoration-thickness: 2px;
  background: color-mix(in srgb, var(--use-color) 10%, transparent);
}

.st-use--ambiguous {
  --use-color: var(--proposed);
  text-decoration-style: double;
}

.st-use--plain {
  text-decoration: none;
  cursor: default;
}

.st-use--unresolved {
  --use-color: var(--danger);
  text-decoration-style: dashed;
  background: var(--danger-wash);
}

.st-use-mark {
  display: inline-flex;
  align-items: center;
  gap: 4px;
  margin-inline-end: 4px;
  line-height: 1;
  vertical-align: middle;
}

.st-use-mark__word {
  font-size: 10.5px;
  font-weight: 650;
  letter-spacing: 0.02em;
  color: var(--ink-soft);
  user-select: none;
}

.st-root .st-editor-host .cm-tooltip {
  border: 0;
  background: transparent;
}

.st-use-card,
.st-insert {
  font-family: var(--font-ui);
  background: var(--surface);
  border: 1px solid var(--line);
  border-radius: var(--radius-md);
  box-shadow: var(--shadow-pop);
}

.st-use-card {
  width: max-content;
  max-width: min(340px, calc(100vw - 32px));
  padding: 12px 14px 10px;
  font-size: 13px;
  line-height: 1.45;
}

.st-use-card__eyebrow {
  font-size: 11px;
  font-weight: 600;
  letter-spacing: 0.06em;
  text-transform: uppercase;
  color: var(--accent-ink);
}

.st-use-card__name {
  margin-top: 2px;
  font-size: 15px;
  font-weight: 650;
}

.st-use-card__where {
  font-size: 12px;
  color: var(--ink-muted);
}

.st-use-card__reading,
.st-use-card__note {
  margin: 6px 0 0;
  color: var(--ink-soft);
}

.st-insert {
  min-width: 260px;
  max-width: 380px;
  max-height: 260px;
  overflow-y: auto;
  padding: 4px;
  font-size: 13.5px;
}

.st-insert__option {
  display: flex;
  align-items: center;
  gap: 8px;
  padding: 6px 8px;
  border-radius: var(--radius-sm);
  cursor: pointer;
}

.st-insert__option[aria-selected="true"] {
  background: var(--accent-wash-strong);
}

.st-insert__name {
  font-weight: 600;
}

.st-insert__where,
.st-insert__empty {
  color: var(--ink-muted);
}

.st-insert__empty {
  padding: 8px;
}

.st-conflict {
  display: grid;
  gap: 8px;
}

.st-confirm {
  display: inline-flex;
  align-items: center;
  gap: 8px;
  font-size: 13px;
}

/* The language at hand */

.st-feature-inspector {
  display: grid;
  gap: 12px;
}

.st-card--inspector {
  padding: 14px 16px;
  box-shadow: none;
  display: grid;
  gap: 8px;
}

.st-card__title--inspector {
  font-size: 18px;
}

.st-navigator__heading--sub {
  padding: 0 8px 6px;
}

.st-selection__text {
  margin: 0;
  font-size: 15px;
  font-weight: 600;
  overflow-wrap: anywhere;
}

.st-tie-options {
  list-style: none;
  margin: 0;
  padding: 0;
  display: grid;
  gap: 1px;
  max-height: 220px;
  overflow-y: auto;
}

.st-tie-option {
  display: flex;
  align-items: center;
  gap: 8px;
  padding: 5px 8px;
  border-radius: var(--radius-sm);
  cursor: pointer;
  font-size: 13.5px;
}

.st-tie-option:hover {
  background: rgba(31, 36, 48, 0.04);
}

.st-tie-option.is-active {
  background: var(--accent-wash-strong);
}

.st-tie-option__name {
  font-weight: 600;
}

.st-tie-option small {
  color: var(--ink-muted);
  margin-left: auto;
  white-space: nowrap;
}

.st-tie-reading {
  display: grid;
  gap: 4px;
  font-size: 13px;
}

.st-tie-reading__choice {
  display: flex;
  align-items: center;
  gap: 6px;
}

.st-candidates {
  list-style: none;
  margin: 0;
  padding: 0;
  display: grid;
  gap: 8px;
}

.st-candidates li {
  display: flex;
  align-items: flex-start;
  gap: 8px;
}

.st-candidates li > span {
  flex: 1;
  min-width: 0;
  display: grid;
}

.st-candidates__reading {
  color: var(--ink-muted);
}

.st-unresolved {
  list-style: none;
  margin: 0;
  padding: 0;
  display: grid;
  gap: 10px;
  font-size: 13px;
}

.st-unresolved li {
  display: grid;
  gap: 2px;
}

.st-unresolved__quote {
  font-weight: 600;
}

.st-unresolved__reason {
  color: var(--danger);
}

.st-unresolved__actions {
  display: flex;
  gap: 12px;
}

.st-copy-ticket {
  display: grid;
  gap: 8px;
}

.st-copy-ticket__note {
  font-size: 12.5px;
  color: var(--accent-ink);
}

.st-copy-ticket__fallback {
  display: grid;
  gap: 6px;
}

.st-copy-ticket__text {
  width: 100%;
  font-family: var(--font-mono);
  font-size: 12px;
  padding: 8px;
  border: 1px solid var(--line-strong);
  border-radius: var(--radius-sm);
  background: var(--surface);
}

@media (max-width: 960px) {
  .st-body,
  .st-body--map,
  .st-body--features {
    grid-template-columns: 220px minmax(0, 1fr);
  }
  .st-body--map .st-inspector,
  .st-body--features .st-inspector {
    grid-column: 1 / -1;
    border-left: 0;
    border-top: 1px solid var(--line);
    max-height: 45%;
  }
  .st-brand__source {
    display: none;
  }
}
@media (max-width: 640px) {
  .st-topbar { flex-wrap: wrap; gap: 8px; padding: 8px 12px; }
  .st-brand { flex: 1 1 100%; }
  .st-brand__project { max-width: 190px; }
  .st-search { flex: 1 1 100%; max-width: none; order: 4; }
  .st-accepted { flex: 1 1 auto; font-size: 11px; white-space: normal; }
  .st-mode { min-height: 36px; padding: 4px 10px; }
  .st-body { grid-template-columns: minmax(0, 1fr); grid-template-rows: minmax(100px, 22vh) minmax(0, 1fr); }
  .st-body--map, .st-body--features { grid-template-rows: minmax(80px, 16vh) minmax(170px, 1fr) minmax(180px, 1fr); }
  .st-navigator { border-right: 0; border-bottom: 1px solid var(--line); padding-bottom: 8px; }
  .st-main { padding: 12px 10px 32px; }
  .st-main--map { padding: 0; }
  .st-body--map .st-inspector, .st-body--features .st-inspector { max-height: none; }
  .st-card, .st-inspector .st-card { padding: 18px; overflow-wrap: anywhere; }
  .st-status { flex-wrap: wrap; }
  .st-status__counts { overflow: hidden; text-overflow: ellipsis; white-space: nowrap; }
}
`;
