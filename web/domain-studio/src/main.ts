import './style.css';
import './platform.css';
import { createWorkspace, type Representation, type WorkspaceApplyOptions } from './workspace';
import { DOMAIN_KINDS, domainTable, onyxAvatar } from './presentation';
import { loadRepository } from './repository';
import { createIcons, ArrowUpLeft, Compass, BookOpen, Map, Menu, ArrowDown, ArrowDownToLine, ArrowUpDown, Box, Camera, Check, ChevronRight, CircleAlert, CircleCheck, CircleDashed, Copy, Download, File, FileCode, FolderOpen, HardDrive, Hexagon, History, Keyboard, Layers3, Minus, Network, Orbit, PanelLeftClose, Pencil, Plus, Redo2, Scan, Search, Sparkles, Undo2, Upload, View, Waypoints, X } from 'lucide';
const icons = { ArrowUpLeft, Compass, BookOpen, Map, Menu, ArrowDown, ArrowDownToLine, ArrowUpDown, Box, Camera, Check, ChevronRight, CircleAlert, CircleCheck, CircleDashed, Copy, Download, File, FileCode, FolderOpen, HardDrive, Hexagon, History, Keyboard, Layers3, Minus, Network, Orbit, PanelLeftClose, Pencil, Plus, Redo2, Scan, Search, Sparkles, Undo2, Upload, View, Waypoints, X };
import { createScene } from './scene';
import { createWorkbench, type Workbench } from './workbench';
import { relationshipDescription } from './model-evidence';
import { projectView, keeperFor, type ViewLens } from './view-state';
import { createEmptyProject, createExampleProject, validateProject, diffProjects, assertProject, cloneProject, makeId } from './domain';
import { exportProject, importProject, exportDomainYaml, parsePattern } from './serialization';
import type { DomainProject, Baseline, StudioMode, PatternSummary, ConceptKind, Concept, SceneController } from './contracts';

const STORAGE_KEY = 'arclint.domain-studio.v2';
const LEGACY_STORAGE_KEY = 'arclint.domain-studio.v1';
const COLORS = ['#637966', '#a38a50', '#455c71', '#987568', '#75877b', '#977f9a'];
const KINDS = DOMAIN_KINDS;
const escape = (value: unknown) => String(value ?? '').replace(/[&<>"']/g, c => ({ '&': '&amp;', '<': '&lt;', '>': '&gt;', '"': '&quot;', "'": '&#39;' })[c]!);
const icon = (name: string, size = 16) => `<i data-lucide="${name}" width="${size}" height="${size}" aria-hidden="true"></i>`;
let project = createEmptyProject();
let baseline: Baseline | null = null;
let patterns: PatternSummary[] = [];
let selectedId: string | null = null;
let mode: StudioMode = 'domain';
let search = '';
let scopeId: string | null = null;
let aggregateId: string | null = null;
let viewPage = 0;
let lens: ViewLens = 'meaning';
let representation: Representation = 'spatial';
let tablePage = 0;
let tableQuery = '';
let activeNotebookDraftId: string | undefined;
let workspace = createWorkspace(undefined, project);
let editorOpen = false;
let saveError = '';
let startupNotice = '';
let recoveryRaw: string | null = null;
try {
  const raw = localStorage.getItem(STORAGE_KEY) ?? localStorage.getItem(LEGACY_STORAGE_KEY);
  recoveryRaw = raw;
  if (raw) workspace = createWorkspace(JSON.parse(raw));
  project = workspace.project; baseline = workspace.modelSnapshot; patterns = workspace.patterns;
  ({selectedId,scopeId,aggregateId,lens,representation} = workspace.view); viewPage = workspace.view.page;
} catch { if (recoveryRaw) startupNotice = 'Your previous save could not be loaded. Download the saved data before replacing it.'; else saveError = 'Local storage unavailable'; }

const app = document.querySelector<HTMLDivElement>('#app')!;
app.innerHTML = `
  <main class="atlas-world" aria-label="ArcLint Studio" data-lens="meaning">
    <div id="scene" data-testid="scene" aria-label="Interactive domain landscape"></div>
    <header class="place-header"><button id="home-world" class="place-brand" aria-label="Whole domain">arclint<span>studio</span></button><h1 id="project-name"></h1><div class="representation-switch" role="group" aria-label="Application presentation"><button id="representation-spatial" aria-pressed="true">Spatial</button><button id="representation-table" aria-pressed="false">Table</button></div><div class="place-header-actions"><button id="open-index" aria-label="Find in the domain">${icon('search', 18)} Find</button><button id="tools-toggle" aria-label="Tools">Tools ${icon('plus', 16)}</button></div></header>
    <section class="place-orientation" aria-label="Your place in the domain"><button id="ascend" aria-label="Ascend one level">${icon('arrow-up-left', 16)} Back</button><nav id="breadcrumbs" aria-label="Domain hierarchy"></nav><h2 id="place-name">The whole domain</h2><p id="place-summary">Choose a context to enter.</p></section>
    <nav class="governance-access" aria-label="Repository governance"><button id="governing-rules">${icon('layers-3')} Rules</button><button id="repository-findings">${icon('circle-alert')} Findings</button><button id="source-paths">${icon('file-code')} Paths</button><button id="check-code-now">${icon('check')} Check code</button></nav>
    <section id="meaning-table" aria-label="Domain table" hidden><div class="table-heading"><label>Find in this view<input id="table-query" placeholder="Filter names and meanings…" /></label><span id="table-count"></span></div><div id="domain-table-content"></div><nav class="table-pages" aria-label="Table pages"><button id="table-prev">← Previous</button><span id="table-page"></span><button id="table-next">Next →</button></nav></section>
    <div class="focus-controls"><button id="exit-comparison" hidden>Model snapshot · return to present ×</button><div class="lens-switch" role="group" aria-label="Your goal"><button id="lens-meaning" aria-pressed="true">Meaning</button><button id="lens-governance" aria-pressed="false">Governance</button></div><button id="primary-action">Add context ${icon('plus', 15)}</button><button id="open-editor" hidden>Edit meaning ${icon('arrow-up-left', 15)}</button></div>
    <nav class="view-pages" aria-label="Visible subjects"><button id="view-prev" aria-label="Previous subjects">←</button><span id="view-count"></span><button id="view-next" aria-label="Next subjects">→</button></nav>
    <form id="build-bar" aria-label="Start writing your domain"><label class="sr-only" for="build-input">Describe a meaning or a question</label><textarea id="build-input" rows="1" placeholder="Describe a meaning, a question, a promise…" spellcheck="true"></textarea><button type="submit" id="build-draft">Build ${icon('arrow-up-left')}</button><button type="button" id="open-notebook" aria-label="Open notebook">${icon('book-open')}</button></form>
    <div class="map-navigation" role="toolbar" aria-label="Spatial navigation"><button id="map-frame" aria-label="Frame this view">${icon('scan')}</button><button id="map-top" aria-label="Plan view">${icon('map')}</button><button id="map-minus" aria-label="Zoom out">${icon('minus')}</button><button id="map-plus" aria-label="Zoom in">${icon('plus')}</button></div>
    <button id="keeper-action" class="keeper-action" aria-label="Talk to Onyx" aria-expanded="false">${onyxAvatar()}<span id="keeper-name">Onyx</span><span id="keeper-prompt" class="sr-only">Your domain guide</span></button>
    <section id="onyx-support" aria-label="Onyx support" hidden><header><div><b>Onyx</b><small>Local domain guide</small></div><button id="close-onyx" aria-label="Close Onyx">${icon('x')}</button></header><p id="onyx-place"></p><div id="onyx-conversation" role="log" aria-live="polite"></div><div class="onyx-actions"><button id="onyx-define">Work on this meaning</button><button id="onyx-rules">What governs this?</button><button id="onyx-paths">Find its source</button><button id="onyx-request">Prepare AI request ↗</button></div><form id="onyx-form"><label class="sr-only" for="onyx-input">Ask Onyx</label><input id="onyx-input" placeholder="Ask about this domain…" required /><button type="submit" aria-label="Send to Onyx">↑</button></form></section>
    <section id="inspector" class="field-manuscript" aria-label="Selection details" hidden></section>
    <div id="notice" class="notice" role="status" aria-live="polite"></div>
    <span id="project-description" class="sr-only"></span><span id="model-stats" class="sr-only"></span><span id="issues-status" class="sr-only"></span><span id="save-status" class="save-status"></span><span id="level-hint" hidden></span>
    <dialog id="tools-drawer" class="tools-drawer" aria-label="Tools"><header><span>WORK WITH THIS PLACE</span><button id="close-tools" aria-label="Close tools">${icon('x',22)}</button></header><h2>Tools</h2><div class="tools-body">
      <section class="tool-group"><h3>Shape the model</h3><button id="add-context">${icon('hexagon',16)} Add context</button><button id="add-concept">${icon('plus',16)} Add to domain</button><button id="connect" aria-label="Connect domain entries">${icon('waypoints',16)} Connect domain entries</button><button id="edit-project">${icon('pencil',16)} Domain details</button></section>
      <section class="tool-group workspace-access"><button id="workspace-menu-toggle" aria-label="Workspace menu" aria-expanded="false">Files & workspace ${icon('chevron-right',16)}</button><div id="workspace-menu" hidden><button id="new-domain">New domain</button><button id="import">Import</button><button id="export">Export</button><button id="save-domain-repository">Save domain to repository</button><button id="load-example-menu">Load example domain</button><button id="menu-domain-details">Domain details</button></div></section>
      <section class="tool-group"><h3>Review</h3><button data-mode="patterns" aria-label="Patterns">Patterns</button><button data-mode="baseline" aria-label="Model snapshot">Compare model snapshot</button><button id="field-guide" aria-label="Field guide">Recorded language guide</button></section>
      <section class="tool-group"><h3>View & history</h3><div class="view-instruments" role="toolbar" aria-label="Map tools"><button id="overview" aria-label="Frame whole domain">${icon('scan',18)}</button><button id="top-view" aria-label="Overhead view">${icon('map',18)}</button><button id="zoom-out" aria-label="Zoom out">${icon('minus',18)}</button><button id="zoom-in" aria-label="Zoom in">${icon('plus',18)}</button><button id="undo" aria-label="Undo">${icon('undo-2',18)}</button><button id="redo" aria-label="Redo">${icon('redo-2',18)}</button></div><button id="help" aria-label="Keyboard shortcuts">Keyboard shortcuts</button></section>
    </div></dialog>
  </main>
  <dialog id="navigator" class="index-sheet" aria-label="Domain index"><div class="index-heading"><div><span>EVERY PLACE REMAINS REACHABLE</span><h2>Find</h2></div><button id="close-index" class="icon-button" aria-label="Close index">${icon('x', 20)}</button></div><label class="index-search">${icon('search', 20)}<input id="search" placeholder="A name, definition, or question…" aria-label="Find in the domain" autocomplete="off" /></label><div id="model-tree" data-testid="model-tree"></div><p class="index-footer">Select a name to go there. <span id="context-count"></span> contexts.</p></dialog>
  <input type="file" id="import-file" accept=".json,.yaml,.yml" hidden />
  <input type="file" id="pattern-file" accept=".yaml,.yml,.json" hidden />
  <dialog id="modal" class="studio-dialog"></dialog>
`;
const $ = <T extends HTMLElement = HTMLElement>(selector: string) => document.querySelector<T>(selector)!;
let scene: SceneController;
let workbench: Workbench;
const currentView = () => projectView(project, { scopeId, selectedId, page: viewPage });
const sceneState = () => ({ project, selectedId, mode, baseline, findings: validateProject(project), search, scopeId, aggregateId, view: currentView(), lens });
function closeTools() { ($('#tools-drawer') as HTMLDialogElement).close(); }
function openTools() { const tools = $('#tools-drawer') as HTMLDialogElement; if (!tools.open) tools.showModal(); }
function openEditor() { closeTools(); editorOpen = true; mode = 'domain'; render(); }
const refreshIcons = () => createIcons({ icons, attrs: { 'stroke-width': 1.6 } });
function notify(message: string, error = false) {
  $('#notice').textContent = message;
  $('#notice').className = `notice visible${error ? ' error' : ''}`;
  window.clearTimeout(noticeTimer);
  noticeTimer = window.setTimeout(() => $('#notice').classList.remove('visible'), error ? 10000 : 4500);
}
let noticeTimer = 0;
function persist() {
  if (startupNotice) return;
  try {
    workspace.updateView({selectedId,scopeId,aggregateId,page:viewPage,representation,lens});
    if (JSON.stringify(workspace.modelSnapshot) !== JSON.stringify(baseline)) workspace.setModelSnapshot(baseline);
    if (JSON.stringify(workspace.patterns) !== JSON.stringify(patterns)) workspace.setPatterns(patterns);
    localStorage.setItem(STORAGE_KEY, workspace.serialize()); saveError = '';
  } catch { saveError = 'Save failed · export a backup'; }
}
function captureEditorDraft() {
  const form = $('#inspector')?.querySelector<HTMLFormElement>('form');
  if (!form) return;
  const fields = Object.fromEntries([...new FormData(form)].map(([key,value]) => [key,String(value)]));
  workspace.saveEditorDraft(`editor:${selectedId ?? scopeId}`,fields);
}
function restoreEditorDraft() {
  const form = $('#inspector').querySelector<HTMLFormElement>('form');
  if (!form) return;
  const draft = workspace.getEditorDraft(`editor:${selectedId ?? scopeId}`);
  if (draft) {
    for (const name of ['kind','contextId']) { const field = form.elements.namedItem(name); if (field instanceof HTMLSelectElement && draft.fields[name] !== undefined) field.value = draft.fields[name]; }
    (form.elements.namedItem('kind') as HTMLSelectElement | null)?.dispatchEvent(new Event('change'));
  }
  if (draft) for (const [name,value] of Object.entries(draft.fields)) {
    const input = form.elements.namedItem(name); if (input instanceof HTMLInputElement || input instanceof HTMLTextAreaElement || input instanceof HTMLSelectElement) input.value = value;
  }
  form.addEventListener('input', () => { captureEditorDraft(); persist(); });
  form.addEventListener('change', () => { captureEditorDraft(); persist(); });
}
function syncWorkspace() {
  project = workspace.project; baseline = workspace.modelSnapshot; patterns = workspace.patterns;
  ({selectedId,scopeId,aggregateId,lens,representation} = workspace.view); viewPage = workspace.view.page;
  activeNotebookDraftId = undefined;
  const composer = $('#build-input') as HTMLTextAreaElement;
  composer.value = workspace.getEditorDraft('composer')?.fields.text ?? '';
  sizeComposer();
}

function commit(next: DomainProject, message?: string, replaceWorkspace = false, options: WorkspaceApplyOptions = {}) {
  // Canonical imports project aggregate ownership into named graph edges.
  // Keep these projections consistent when ownership changes in the editor.
  next.relationships = next.relationships.flatMap(r => {
    const member = next.concepts.find(c => c.id.startsWith('yaml:') && r.id === `${c.id}:owner`);
    if (!member) return [r];
    if (!member.ownerId || !['entity', 'repository', 'factory'].includes(member.kind)) return [];
    return [{ ...r, source: member.kind === 'entity' ? member.ownerId : member.id, target: member.kind === 'entity' ? member.id : member.ownerId, label: member.kind === 'entity' ? 'owns member' : member.kind === 'repository' ? 'repository for' : 'creates' }];
  });
  next = assertProject(next);
  if (replaceWorkspace) workspace.replace(next,message); else workspace.apply(message ?? 'Edit domain or layout', () => next, options);
  project = workspace.project;
  if (replaceWorkspace) { baseline = null; patterns = []; scopeId = null; aggregateId = null; viewPage = 0; editorOpen = false; }
  if (scopeId && !project.contexts.some(c => c.id === scopeId)) { scopeId = null; aggregateId = null; }
  if (aggregateId && !project.concepts.some(c => c.id === aggregateId && c.kind === 'aggregate')) aggregateId = null;
  if (selectedId && ![...project.concepts, ...project.contexts, ...project.relationships].some(x => x.id === selectedId)) selectedId = null;
  persist(); render();
  if (message) notify(message);
}
function undo() { captureEditorDraft(); if (!workspace.undo()) return; syncWorkspace(); editorOpen = false; persist(); render(); scene.overview(); notify('Change undone'); }
function redo() { if (!workspace.redo()) return; syncWorkspace(); editorOpen = false; persist(); render(); scene.overview(); notify('Change restored'); }

function select(id: string | null, focus = false, preserveScope = false) {
  captureEditorDraft();
  ($('#navigator') as HTMLDialogElement).close(); closeTools();
  if (id && project.contexts.some(c => c.id === id)) { enter(id); return; }
  selectedId = id; viewPage = 0; editorOpen = false;
  const concept = project.concepts.find(c => c.id === id);
  if (concept && (!preserveScope || !project.contexts.some(context => context.id === scopeId))) scopeId = concept.contextId;
  render();
  if (id && focus) scene.focus(id);
}
function enter(id: string) {
  captureEditorDraft();
  const context = project.contexts.find(c => c.id === id);
  const concept = project.concepts.find(c => c.id === id);
  if (context) { scopeId = context.id; aggregateId = null; selectedId = null; }
  else if (concept?.kind === 'aggregate') { scopeId = concept.contextId; aggregateId = concept.id; selectedId = concept.id; }
  else { select(id, true); return; }
  editorOpen = false; viewPage = 0; if (mode !== 'baseline') mode = 'domain'; closeTools(); render();
}
function ascend() {
  captureEditorDraft();
  if (editorOpen) { editorOpen = false; if (mode !== 'baseline') mode = 'domain'; render(); return; }
  if (selectedId) { selectedId = null; aggregateId = null; }
  else { scopeId = null; aggregateId = null; }
  viewPage = 0; if (mode !== 'baseline') mode = 'domain'; render();
}
function openIndex() {
  closeTools();
  closeWorkspaceMenu(); closeCreationFan();
  renderTree(); refreshIcons();
  const navigator = $('#navigator') as HTMLDialogElement;
  if (!navigator.open) navigator.showModal();
  $('#search').focus();
}
function closeWorkspaceMenu() { $('#workspace-menu').hidden = true; $('#workspace-menu-toggle').setAttribute('aria-expanded', 'false'); }
function closeCreationFan() { /* Creation actions live in the summoned Tools surface. */ }
function positionManuscript() { /* Editing owns a full work surface, never a floating map card. */ }
function renderLocation() {
  const view = currentView();
  viewPage = view.page;
  const context = project.contexts.find(c => c.id === view.contextId);
  const concept = project.concepts.find(c => c.id === selectedId);
  const relation = project.relationships.find(c => c.id === selectedId);
  const place = concept?.name ?? (relation ? relationshipDescription(project,relation).label : context?.name ?? project.name);
  $('#place-name').textContent = place;
  $('#place-summary').textContent = concept ? concept.definition.slice(0,180) : relation ? relationshipDescription(project,relation).meaning : context ? context.description.slice(0,120) : `${project.contexts.length} bounded contexts · ${project.concepts.length} domain entries`;
  $('#breadcrumbs').innerHTML = `<button id="realm-location">${escape(project.name)}</button>${context ? `<span>/</span><button id="context-location">${escape(context.name)}</button>` : ''}${view.level === 'detail' ? `<span>/</span><span>${escape(concept ? KINDS[concept.kind] : 'Relationship')}</span>` : ''}`;
  $('#realm-location').onclick = () => { captureEditorDraft(); scopeId = null; aggregateId = null; selectedId = null; viewPage = 0; editorOpen = false; if (mode !== 'baseline') mode = 'domain'; render(); };
  $('#context-location')?.addEventListener('click', () => { captureEditorDraft(); selectedId = null; aggregateId = null; editorOpen = false; viewPage = 0; if (mode !== 'baseline') mode = 'domain'; render(); });
  $('#ascend').toggleAttribute('disabled', view.level === 'world');
  $('#scene').dataset.scope = scopeId ?? 'realm';
  $('#scene').dataset.depth = view.level;
  $('#scene').dataset.comparison = mode === 'baseline' ? 'baseline' : 'current';
  $('#exit-comparison').hidden = mode !== 'baseline';
  $('#scene').dataset.selectedId = selectedId ?? '';
  $('.atlas-world').dataset.depth = view.level;
  $('.atlas-world').dataset.lens = lens;
  const keeper = keeperFor(view,lens);
  $('#keeper-name').textContent = 'Onyx';
  $('#keeper-prompt').textContent = keeper.action;
  $('#keeper-action').dataset.keeper = 'Onyx';
  $('#keeper-action').setAttribute('aria-label', 'Talk to Onyx');
  $('#onyx-place').textContent = `Here with you · ${context ? context.name : project.name}${concept ? ` / ${concept.name}` : ''}`;
  $('#open-editor').hidden = view.level === 'world';
  $('#open-editor').textContent = concept ? `Edit ${KINDS[concept.kind].toLowerCase()} ↗` : 'Edit context ↗';
  $('#primary-action').hidden = false;
  $('#primary-action').textContent = lens === 'governance' ? 'Inspect governing Rules ↗' : context ? 'Add to this context +' : 'Add a context +';
  $('#view-count').textContent = `${view.ids.length} of ${view.total}`;
  $('.view-pages').hidden = view.pages <= 1;
  $('#view-prev').toggleAttribute('disabled', view.page === 0);
  $('#view-next').toggleAttribute('disabled', view.page + 1 >= view.pages);
  $('#lens-meaning').setAttribute('aria-pressed', String(lens === 'meaning'));
  $('#lens-governance').setAttribute('aria-pressed', String(lens === 'governance'));
}
function entityName(id: string) { return [...project.concepts, ...project.contexts].find(c => c.id === id)?.name ?? 'Removed domain entry'; }
function contextOptions(selected?: string) { return project.contexts.map(c => `<option value="${escape(c.id)}" ${c.id === selected ? 'selected' : ''}>${escape(c.name)}</option>`).join(''); }
function kindOptions(selected: ConceptKind = 'unclassified') { return Object.entries(KINDS).map(([key, label]) => `<option value="${key}" ${key === selected ? 'selected' : ''}>${label}</option>`).join(''); }
function field(label: string, name: string, value = '', multiline = false, required = true) { return `<label class="field"><span>${label}</span>${multiline ? `<textarea name="${name}" rows="3" ${required ? 'required' : ''}>${escape(value)}</textarea>` : `<input name="${name}" value="${escape(value)}" ${required ? 'required' : ''} maxlength="120" />`}</label>`; }
function metadataFields(concept?: Concept) {
  const kind = concept?.kind ?? 'unclassified';
  const ctxId = concept?.contextId ?? project.contexts[0]?.id;
  return `<details class="model-metadata" ${kind !== 'unclassified' ? 'open' : ''}><summary>Identity & ownership</summary><div class="metadata-fields"><div data-identity-field ${['aggregate', 'entity'].includes(kind) ? '' : 'hidden'}>${field('Identity', 'identity', concept?.identity ?? '', false, false)}<p class="field-help">Name of the identity, such as BookID.</p></div><label class="field" data-owner-field ${['entity', 'repository', 'factory'].includes(kind) ? '' : 'hidden'}><span>Owning aggregate</span><select name="ownerId"><option value="">Not yet decided</option>${project.concepts.filter(c => c.kind === 'aggregate' && c.contextId === ctxId && c.id !== concept?.id).map(c => `<option value="${escape(c.id)}" ${c.id === concept?.ownerId ? 'selected' : ''}>${escape(c.name)}</option>`).join('')}</select></label><div data-alias-field ${['aggregate', 'entity', 'value_object'].includes(kind) ? '' : 'hidden'}>${field('Aliases', 'aliases', concept?.aliases?.join(', ') ?? '', false, false)}<p class="field-help">Alternative names, separated by commas.</p></div><p class="small muted" data-metadata-empty ${kind === 'unclassified' ? '' : 'hidden'}>Choose a kind to record identity or ownership.</p></div></details>`;
}
function bindMetadata(form: HTMLFormElement, editedId: string | null = null) {
  const update = () => {
    const kind = (form.elements.namedItem('kind') as HTMLSelectElement).value;
    const contextId = (form.elements.namedItem('contextId') as HTMLSelectElement).value;
    form.querySelector('[data-invariant-field]')?.toggleAttribute('hidden', !['aggregate','value_object'].includes(kind));
    form.querySelector('[data-identity-field]')?.toggleAttribute('hidden', !['aggregate', 'entity'].includes(kind));
    form.querySelector('[data-owner-field]')?.toggleAttribute('hidden', !['entity', 'repository', 'factory'].includes(kind));
    form.querySelector('[data-alias-field]')?.toggleAttribute('hidden', !['aggregate', 'entity', 'value_object'].includes(kind));
    form.querySelector('[data-metadata-empty]')?.toggleAttribute('hidden', kind !== 'unclassified');
    const owner = form.elements.namedItem('ownerId') as HTMLSelectElement;
    if (owner) { const current = owner.value; owner.innerHTML = '<option value="">Not yet decided</option>' + project.concepts.filter(c => c.kind === 'aggregate' && c.contextId === contextId && c.id !== editedId).map(c => `<option value="${escape(c.id)}" ${c.id === current ? 'selected' : ''}>${escape(c.name)}</option>`).join(''); }
  };
  (form.elements.namedItem('kind') as HTMLElement)?.addEventListener('change', () => { update(); const details = form.querySelector('details'); if (details) details.open = true; });
  (form.elements.namedItem('contextId') as HTMLElement)?.addEventListener('change', update);
  update();
}
function applyMetadata(concept: Concept, data: FormData) {
  const originalAliases = concept.aliases;
  delete concept.identity; delete concept.ownerId; delete concept.aliases;
  const identity = String(data.get('identity') ?? '').trim();
  const owner = String(data.get('ownerId') ?? '');
  const aliasInput = String(data.get('aliases') ?? '');
  const aliases = originalAliases && aliasInput === originalAliases.join(', ') ? originalAliases : [...new Set(aliasInput.split(',').map(a => a.trim()).filter(Boolean))];
  if (['aggregate', 'entity'].includes(concept.kind) && identity) concept.identity = identity;
  if (['entity', 'repository', 'factory'].includes(concept.kind) && owner) concept.ownerId = owner;
  if (['aggregate', 'entity', 'value_object'].includes(concept.kind) && aliases.length) concept.aliases = aliases;
}
function renderTree() {
  const q = search.toLocaleLowerCase();
  $('#context-count').textContent = String(project.contexts.length).padStart(2, '0');
  $('#model-tree').innerHTML = project.contexts.map(ctx => {
    const concepts = project.concepts.filter(c => c.contextId === ctx.id && (!q || `${c.name} ${c.definition} ${ctx.name}`.toLocaleLowerCase().includes(q)));
    if (q && !concepts.length && !ctx.name.toLocaleLowerCase().includes(q)) return '';
    return `<section class="tree-context" style="--context-color:${escape(ctx.color)}"><button class="tree-context-title ${selectedId === ctx.id ? 'selected' : ''}" data-select="${escape(ctx.id)}">${icon('hexagon', 15)}<span>${escape(ctx.name)}</span><small>${concepts.length}</small></button><div class="tree-children">${concepts.map(c => `<button class="tree-concept ${selectedId === c.id ? 'selected' : ''}" data-select="${escape(c.id)}"><span class="concept-dot ${c.kind}"></span><span>${escape(c.name)}</span>${c.kind === 'unclassified' ? '<span class="draft-dot" title="Kind not yet classified">○</span>' : ''}</button>`).join('')}</div></section>`;
  }).join('') || `<div class="empty-tree">${icon('orbit', 25)}<p>${q ? 'No matching domain entries.' : 'Your domain starts here.'}</p>${!q ? '<span>Add a context, then record its language.</span>' : ''}</div>`;
  document.querySelectorAll<HTMLButtonElement>('[data-select]').forEach(button => button.onclick = () => { if (mode !== 'baseline') mode = 'domain'; select(button.dataset.select!, true); });
}
function relationshipRows(id: string) {
  const links = project.relationships.filter(r => r.source === id || r.target === id);
  return links.map(r => `<button class="relationship-row" data-link="${escape(r.id)}"><span>${r.source === id ? '→' : '←'}</span><span><b>${escape(entityName(r.source === id ? r.target : r.source))}</b><small>${escape(r.label)}</small></span>${icon('chevron-right', 14)}</button>`).join('') || '<p class="muted">No relationships yet.</p>';
}
function renderInspector() {
  if (!editorOpen) { $('#inspector').hidden = true; $('#inspector').replaceChildren(); return; }
  const findings = validateProject(project);
  const concept = project.concepts.find(c => c.id === selectedId);
  const context = project.contexts.find(c => c.id === selectedId || (!selectedId && c.id === scopeId));
  const relation = project.relationships.find(c => c.id === selectedId);
  let content = '';
  if (mode === 'baseline') {
    const changes = baseline ? diffProjects(baseline.project, project) : [];
    content = `<div class="panel-heading"><h2>Model snapshot</h2>${icon('history')}</div><div class="inspector-content"><div class="section-eyebrow">A POINT OF REFERENCE</div><h3>${baseline ? escape(baseline.name) : 'See how your model evolves.'}</h3><p class="muted">Save where you are. Changes to the model appear here and in the map.</p><button class="primary full" id="capture-baseline">${icon('camera')} ${baseline ? 'Replace model snapshot' : 'Capture model snapshot'}</button>${baseline ? `<div class="baseline-date">Captured ${escape(new Date(baseline.capturedAt).toLocaleString())}</div><div class="section-label">CHANGES <span>${changes.length}</span></div>${changes.map(c => `<button class="change-row ${c.type}" data-change="${escape(c.id)}"><span>${c.type === 'added' ? '+' : c.type === 'removed' ? '−' : '~'}</span><span><b>${escape(c.name)}</b><small>${escape(c.detail)}</small></span><em>${c.type}</em></button>`).join('') || '<div class="empty-state">Your model matches this snapshot.</div>'}<button class="quiet-button full" id="download-baseline">${icon('download')} Download model snapshot</button>` : '<div class="baseline-illustration"><span></span><span></span><span></span></div>'}</div>`;
  } else if (mode === 'patterns') {
    content = `<div class="panel-heading"><h2>Patterns & checks</h2>${icon('layers-3')}</div><div class="inspector-content"><div class="section-eyebrow">MAKE THE MODEL EXPLICIT</div><h3>Rules and their constraints.</h3><p class="muted">Inspect a Pattern’s rules alongside your domain. Imported Patterns are reference material; they are not executed here.</p><button class="primary full" id="import-pattern">${icon('upload')} Import Pattern</button>${patterns.map((p, index) => `<details class="pattern-card"><summary>${escape(p.name)}<span>${p.rules.length} rules</span></summary><p>${escape(p.description)}</p>${p.rules.map(r => `<div class="pattern-rule"><b>${escape(r.id)}</b><p>${escape(r.description)}</p></div>`).join('')}<button class="quiet-button" data-remove-pattern="${index}">Remove reference</button></details>`).join('')}<div class="section-label">MODEL CHECKS <span>${findings.length}</span></div><p class="small muted">Local draft checks. Check code evaluates the configured Rules in the bound repository.</p>${findingRows(findings)}</div>`;
  } else if (concept) {
    const ctx = project.contexts.find(c => c.id === concept.contextId)!;
    content = `<div class="panel-heading"><h2>${escape(KINDS[concept.kind])}</h2><button class="icon-button" id="clear-selection" aria-label="Clear selection">${icon('x')}</button></div><div class="inspector-content"><div class="selection-kind">${icon('box', 16)} ${escape(KINDS[concept.kind])}<button id="learn-selection" class="learn-link">Learn</button></div>${concept.kind === 'aggregate' ? `<button class="enter-place" id="enter-aggregate">Explore aggregate ${icon('arrow-down')}</button>` : ''}<form id="concept-form">${field('Name', 'name', concept.name)}${field('Definition', 'definition', concept.definition, true, false)}<div class="field-pair"><label class="field"><span>Kind</span><select name="kind">${kindOptions(concept.kind)}</select></label><label class="field"><span>Context</span><select name="contextId">${contextOptions(concept.contextId)}</select></label></div>${metadataFields(concept)}<div data-invariant-field ${['aggregate','value_object'].includes(concept.kind) ? '' : 'hidden'}>${field('Invariants', 'invariants', concept.invariants.join('\n'), true, false)}<p class="field-help">One invariant per line. Record what must always hold.</p></div><p class="form-error" role="alert"></p><button class="primary full" type="submit">${icon('check', 16)} Save definition</button></form>${['aggregate','value_object'].includes(concept.kind) ? `<section class="recorded-contracts"><div class="section-label">CONTRACTS</div><button id="add-invariant">Add invariant +</button>${concept.kind === 'aggregate' ? `<button id="add-assertion">Add assertion +</button>${(concept.assertions ?? []).map(a => `<button class="assertion-entry" data-edit-assertion="${escape(a.key)}"><b>After ${escape(a.on)}</b><span>${escape(a.statement)}</span><small>${escape(a.key)} ↗</small></button>`).join('')}` : ''}</section>` : ''}<div class="section-label">RELATIONSHIPS <button class="icon-button" id="connect-selected" aria-label="Add relationship">${icon('plus', 15)}</button></div>${relationshipRows(concept.id)}${workbench?.selectionEvidence(concept.id) ?? ''}<div class="inspector-bottom"><button class="quiet-button full" id="prepare-ai">${icon('sparkles', 16)} Prepare AI request</button><button class="danger-link" id="delete-selection">Delete domain entry</button></div></div>`;
  } else if (context) {
    content = `<div class="panel-heading"><h2>Bounded context</h2><button class="icon-button" id="clear-selection" aria-label="Clear selection">${icon('x')}</button></div><div class="inspector-content"><div class="selection-kind">${icon('hexagon')} BOUNDED CONTEXT<button id="learn-selection" class="learn-link">Learn</button></div><form id="context-form">${field('Name', 'name', context.name)}${field('Definition', 'description', context.description, true, false)}<label class="field"><span>Map color</span><input name="color" type="color" value="${escape(context.color)}" /></label><p class="form-error" role="alert"></p><button class="primary full" type="submit">${icon('check')} Save context</button></form><div class="section-label">DOMAIN ENTRIES <span>${project.concepts.filter(c => c.contextId === context.id).length}</span></div><button class="quiet-button full" id="add-in-context">${icon('plus')} Add to this context</button><div class="section-label">RELATIONSHIPS <button class="icon-button" id="connect-selected" aria-label="Add relationship">${icon('plus', 15)}</button></div>${relationshipRows(context.id)}<div class="inspector-bottom"><button class="quiet-button full" id="prepare-ai">${icon('sparkles')} Prepare AI request</button><button class="danger-link" id="delete-selection">Delete context</button></div></div>`;
  } else if (relation) {
    content = `<div class="panel-heading"><h2>Relationship</h2><button class="icon-button" id="clear-selection" aria-label="Clear selection">${icon('x')}</button></div><div class="inspector-content"><div class="connection-preview"><b>${escape(entityName(relation.source))}</b>${icon('arrow-down', 24)}<b>${escape(entityName(relation.target))}</b></div><p class="relationship-meaning">${escape(relationshipDescription(project, relation).meaning)}</p><form id="relationship-form">${field('Relationship', 'label', relation.label)}<p class="form-error" role="alert"></p><button class="primary full" type="submit">Save relationship</button></form><button class="quiet-button full" id="reverse-relationship">${icon('arrow-up-down')} Reverse direction</button><button class="danger-link" id="delete-selection">Delete relationship</button></div>`;
  } else {
    $('#inspector').hidden = true;
    $('#inspector').replaceChildren();
    return;
  }
  $('#inspector').hidden = false;
  $('#inspector').dataset.kind = mode === 'domain' ? 'object' : 'overlay';
  $('#inspector').innerHTML = content;
  if (mode === 'domain') $('#inspector .inspector-content')?.insertAdjacentHTML('beforeend','<div class="editor-repository-save"><button id="review-domain-save">Review repository changes ↗</button><small>Apply the saved domain after reviewing its diff.</small></div>');
  if (mode !== 'domain') $('#inspector .panel-heading').insertAdjacentHTML('beforeend', `<button class="icon-button" id="close-sheet" aria-label="Close sheet">${icon('x', 18)}</button>`);
  bindInspector();
  workbench?.bindSelection();
}
function findingRows(findings: ReturnType<typeof validateProject>) { return findings.map(f => `<button class="finding-row ${f.severity}" data-finding="${escape(f.subjectId)}">${icon('circle-alert', 15)}<span><b>${escape(f.title)}</b><small>${escape(f.message)}</small></span></button>`).join('') || '<div class="check-clear">No model completeness issues found.</div>'; }
function render() {
  workbench?.update(project, selectedId, scopeId, mode, baseline, currentView());
  $('#project-name').textContent = project.name;
  $('#project-description').textContent = project.description;
  $('#model-stats').textContent = `${project.contexts.length} CONTEXTS   /   ${project.concepts.length} DOMAIN ENTRIES   /   ${project.relationships.length} RELATIONSHIPS`;
  const findings = validateProject(project);
  $('#issues-status').innerHTML = `<button id="show-checks">${icon(findings.length ? 'circle-dashed' : 'circle-check', 13)} ${findings.length ? `${findings.length} items to review` : 'Model checks clear'}</button>`;
  $('#show-checks').onclick = () => { editorOpen = true; mode = 'patterns'; render(); };
  $('#save-status').innerHTML = `<span class="status-light ${saveError ? 'warning' : ''}"></span>${escape(saveError || 'Saved locally')}`;
  $('#undo').toggleAttribute('disabled', !workspace.canUndo);
  $('#redo').toggleAttribute('disabled', !workspace.canRedo);
  document.querySelectorAll<HTMLElement>('[data-mode]').forEach(b => { b.classList.toggle('active', b.dataset.mode === mode); b.setAttribute('aria-pressed', String(b.dataset.mode === mode)); });
  renderTree(); renderInspector(); restoreEditorDraft(); renderConventional(); refreshIcons();
  renderLocation();
  scene?.update(sceneState());
  positionManuscript();
  persist();
}
function safely(form: HTMLFormElement, action: () => void) { try { action(); } catch (error) { form.querySelector('.form-error')!.textContent = error instanceof Error ? error.message : String(error); } }
function bindInspector() {
  $('#clear-selection')?.addEventListener('click', () => { captureEditorDraft(); editorOpen = false; mode = 'domain'; render(); });
  $('#close-sheet')?.addEventListener('click', () => { captureEditorDraft(); editorOpen = false; if (mode !== 'baseline') mode = 'domain'; render(); });
  $('#enter-aggregate')?.addEventListener('click', () => { if (selectedId) enter(selectedId); });
  $('#learn-selection')?.addEventListener('click', showFieldGuide);
  $('#overview-add')?.addEventListener('click', () => conceptDialog());
  $('#add-in-context')?.addEventListener('click', () => conceptDialog(selectedId ?? scopeId!));
  $('#all-checks')?.addEventListener('click', () => { editorOpen = true; mode = 'patterns'; render(); });
  $('#connect-selected')?.addEventListener('click', () => relationshipDialog());
  $('#prepare-ai')?.addEventListener('click', prepareAI);
  $('#review-domain-save')?.addEventListener('click', reviewDomainSave);
  $('#add-invariant')?.addEventListener('click', () => contractDialog('invariant'));
  $('#add-assertion')?.addEventListener('click', () => contractDialog('assertion'));
  document.querySelectorAll<HTMLElement>('[data-edit-assertion]').forEach(button => button.onclick = () => contractDialog('assertion',button.dataset.editAssertion));
  $('#delete-selection')?.addEventListener('click', deleteSelection);
  $('#capture-baseline')?.addEventListener('click', captureBaseline);
  $('#import-pattern')?.addEventListener('click', () => $('#pattern-file').click());
  $('#load-example')?.addEventListener('click', loadExample);
  $('#download-baseline')?.addEventListener('click', () => { if (baseline) download(`${slug(baseline.name)}.model-snapshot.json`, exportProject(baseline.project)); });
  $('#reverse-relationship')?.addEventListener('click', () => { const next = cloneProject(project); const r = next.relationships.find(r => r.id === selectedId)!; [r.source, r.target] = [r.target, r.source]; commit(next, 'Relationship reversed'); });
  document.querySelectorAll<HTMLElement>('[data-link]').forEach(b => b.onclick = () => select(b.dataset.link!));
  document.querySelectorAll<HTMLElement>('[data-finding]').forEach(b => b.onclick = () => { mode = 'domain'; select(b.dataset.finding!, true); });
  document.querySelectorAll<HTMLElement>('[data-change]').forEach(b => b.onclick = () => { const id = b.dataset.change!; if (![...project.contexts, ...project.concepts, ...project.relationships].some(item => item.id === id)) { notify('Removed from the current model. Download the model snapshot to keep its earlier record.'); return; } select(id, true); editorOpen = false; mode = 'baseline'; render(); });
  document.querySelectorAll<HTMLElement>('[data-remove-pattern]').forEach(b => b.onclick = () => { patterns.splice(Number(b.dataset.removePattern), 1); persist(); render(); });
  const cf = document.querySelector<HTMLFormElement>('#concept-form');
  if (cf) bindMetadata(cf, selectedId);
  if (cf) cf.onsubmit = event => { event.preventDefault(); safely(cf, () => { const data = new FormData(cf); const next = cloneProject(project); const c = next.concepts.find(c => c.id === selectedId)!; c.name = String(data.get('name')).trim(); c.definition = String(data.get('definition')).trim(); c.kind = String(data.get('kind')) as ConceptKind; const contextId = String(data.get('contextId')); if (contextId !== c.contextId) { const ctx = next.contexts.find(x => x.id === contextId)!; c.position = [ctx.position[0] + 3, 2, ctx.position[2] + 3]; } c.contextId = contextId; const invariantInput = String(data.get('invariants')); if (invariantInput !== c.invariants.join('\n')) c.invariants = invariantInput.split('\n').map(x => x.trim()).filter(Boolean); applyMetadata(c, data); next.concepts.forEach(member => { if (member.ownerId === c.id) { if (c.kind !== 'aggregate') throw new Error('Reassign aggregate members before changing its kind.'); member.contextId = c.contextId; } }); assertProject(next); commit(next, 'Definition saved', false, {clearEditorDraftId: `editor:${selectedId ?? scopeId}`}); }); };
  const ctxf = document.querySelector<HTMLFormElement>('#context-form');
  if (ctxf) ctxf.onsubmit = event => { event.preventDefault(); safely(ctxf, () => { const data = new FormData(ctxf); const next = cloneProject(project); const c = next.contexts.find(c => c.id === (selectedId ?? scopeId))!; c.name = String(data.get('name')).trim(); c.description = String(data.get('description')).trim(); c.color = String(data.get('color')); assertProject(next); commit(next, 'Context saved', false, {clearEditorDraftId: `editor:${selectedId ?? scopeId}`}); }); };
  const rf = document.querySelector<HTMLFormElement>('#relationship-form');
  if (rf) rf.onsubmit = event => { event.preventDefault(); safely(rf, () => { const next = cloneProject(project); next.relationships.find(r => r.id === selectedId)!.label = String(new FormData(rf).get('label')).trim(); assertProject(next); commit(next, 'Relationship saved', false, {clearEditorDraftId: `editor:${selectedId ?? scopeId}`}); }); };
}
function showDialog(title: string, contents: string, submitLabel: string, onSubmit: (data: FormData) => void) {
  closeTools(); closeWorkspaceMenu(); closeCreationFan();
  const modal = $('#modal') as HTMLDialogElement;
  modal.innerHTML = `<form id="dialog-form"><div class="dialog-heading"><div><div class="eyebrow">ARCLINT STUDIO</div><h2>${escape(title)}</h2></div><button type="button" class="icon-button" id="close-dialog" aria-label="Close dialog">${icon('x', 20)}</button></div><div class="dialog-content">${contents}<p class="form-error" role="alert"></p></div><div class="dialog-actions"><button class="quiet-button" type="button" id="cancel-dialog">Cancel</button><button class="primary" type="submit">${escape(submitLabel)}</button></div></form>`;
  const cancel = () => { activeNotebookDraftId = undefined; modal.close(); };
  $('#close-dialog').onclick = cancel; $('#cancel-dialog').onclick = cancel; modal.oncancel = () => { activeNotebookDraftId = undefined; };
  $('#dialog-form').onsubmit = event => { event.preventDefault(); safely($('#dialog-form') as HTMLFormElement, () => { onSubmit(new FormData($('#dialog-form') as HTMLFormElement)); modal.close(); }); };
  refreshIcons(); if (!modal.open) modal.showModal();
  modal.querySelector<HTMLInputElement>('input,textarea,select')?.focus();
}
function contextDialog() {
  showDialog('Add context', `${field('Name', 'name')}${field('Definition', 'description', draftText(), true, false)}<p class="muted small">A boundary within which a language and model apply.</p>`, 'Create context', data => {
    const next = cloneProject(project); const index = next.contexts.length; const angle = index * 2.4;
    const id = makeId('context'); next.contexts.push({ id, name: String(data.get('name')).trim(), description: String(data.get('description')).trim(), color: COLORS[index % COLORS.length], position: index === 0 ? [0, 0, 0] : [Math.cos(angle) * (18 + index * 3), 0, Math.sin(angle) * (18 + index * 3)] }); assertProject(next); selectedId = null; scopeId = id; editorOpen = true; mode = 'domain'; commitAssignment(next, 'Context created'); scene.focus(id);
  });
}
function conceptDialog(contextId?: string, kind?: ConceptKind) {
  if (!kind) { domainPicker(contextId); return; }
  if (!project.contexts.length) { contextDialog(); return; }
  const selectedContext = contextId || project.concepts.find(c => c.id === selectedId)?.contextId || project.contexts.find(c => c.id === selectedId)?.id || scopeId || project.contexts[0].id;
  showDialog(`Add ${KINDS[kind].toLowerCase()}`,  `${field('Name', 'name')}${field('Definition', 'definition', draftText(), true, false)}<div class="field-pair"><label class="field"><span>Kind</span><select name="kind">${kindOptions(kind)}</select></label><label class="field"><span>Context</span><select name="contextId">${contextOptions(selectedContext)}</select></label></div>${metadataFields()}<div data-invariant-field ${['aggregate','value_object'].includes(kind) ? '' : 'hidden'}>${field('Invariants', 'invariants', '', true, false)}</div><p class="muted small">Use an open question while identity and responsibilities are unresolved.</p>`, 'Save definition', data => {
    const next = cloneProject(project); const ctx = next.contexts.find(c => c.id === data.get('contextId'))!; const count = next.concepts.filter(c => c.contextId === ctx.id).length; const angle = count * 2.4; const radius = 4 + Math.floor(count / 5) * 4; const id = makeId('concept');
    next.concepts.push({ id, name: String(data.get('name')).trim(), definition: String(data.get('definition')).trim(), kind: String(data.get('kind')) as ConceptKind, contextId: ctx.id, invariants: String(data.get('invariants')).split('\n').map(x => x.trim()).filter(Boolean), position: [ctx.position[0] + Math.cos(angle) * radius, 1 + count % 3, ctx.position[2] + Math.sin(angle) * radius] }); applyMetadata(next.concepts.at(-1)!, data); assertProject(next); selectedId = id; scopeId = ctx.id; editorOpen = true; mode = 'domain'; commitAssignment(next, 'Definition recorded'); scene.focus(id);
  });
  bindMetadata($('#dialog-form') as HTMLFormElement);
}
function relationshipDialog() {
  const entities = [...project.contexts, ...project.concepts];
  if (entities.length < 2) { notify('Record two domain entries before connecting them.'); return; }
  const opts = (selected?: string) => entities.map(c => `<option value="${escape(c.id)}" ${c.id === selected ? 'selected' : ''}>${escape(c.name)}${'contextId' in c ? '' : ' · context'}</option>`).join('');
  const first = entities.find(c => c.id === (selectedId ?? scopeId))?.id || entities[0].id;
  showDialog('Connect domain entries', `<label class="field"><span>From</span><select name="source">${opts(first)}</select></label><div class="connection-arrow">${icon('arrow-down')}</div><label class="field"><span>To</span><select name="target">${opts(entities.find(c => c.id !== first)!.id)}</select></label>${field('Relationship', 'label')}<p class="muted small">Use your domain’s language: contains, depends on, publishes, or a more precise meaning.</p>`, 'Create relationship', data => {
    if (data.get('source') === data.get('target')) throw new Error('Choose two different objects.');
    const next = cloneProject(project); const id = makeId('relation'); next.relationships.push({ id, source: String(data.get('source')), target: String(data.get('target')), label: String(data.get('label')).trim() }); selectedId = id; editorOpen = true; mode = 'domain'; commit(next, 'Relationship created');
  });
}
function deleteSelection() {
  const subjectId = selectedId ?? scopeId;
  const object = [...project.contexts, ...project.concepts, ...project.relationships].find(c => c.id === subjectId); if (!object) return;
  const isContext = project.contexts.some(c => c.id === subjectId);
  showDialog('Delete from domain', `<p>Delete <strong>${escape('name' in object ? object.name : object.label)}</strong>${isContext ? ' and every domain entry inside it' : ''}? Its relationships will also be removed.</p><p class="muted small">You can undo this change.</p>`, 'Delete', () => {
    const next = cloneProject(project); const ids = new Set([subjectId!]); if (isContext) next.concepts.filter(c => c.contextId === subjectId).forEach(c => ids.add(c.id)); next.concepts = next.concepts.filter(c => !ids.has(c.id)); next.concepts.forEach(c => { if (c.ownerId && ids.has(c.ownerId)) delete c.ownerId; }); next.contexts = next.contexts.filter(c => !ids.has(c.id)); next.relationships = next.relationships.filter(r => !ids.has(r.id) && !ids.has(r.source) && !ids.has(r.target)); commit(next, 'Deleted from domain');
  });
}
function newDomain() {
  showDialog('New domain', `${field('Name', 'name')}${field('Description', 'description', '', true, false)}<p class="muted small">Start an empty model. Your current model can be recovered with Undo. Export it to keep a separate copy.</p>`, 'Create domain', data => {
    const next = createEmptyProject(); next.name = String(data.get('name')).trim(); next.description = String(data.get('description')).trim(); selectedId = null; mode = 'domain'; search = ''; ($('#search') as HTMLInputElement).value = ''; commit(next, 'New domain created', true); scene.overview();
  });
}
function loadExample() {
  showDialog('Load example domain', '<p>Open the bskilled draft to explore the editor. Its boundaries and classifications are proposals.</p><p class="muted small">This replaces your current model. Undo will bring it back.</p>', 'Load example', () => { selectedId = null; mode = 'domain'; commit(createExampleProject(), 'Example domain loaded', true); scene.overview(); });
}
function captureBaseline() {
  showDialog('Capture model snapshot', `${field('Name', 'name', `${project.name} · starting point`)}<p class="muted small">Capture the current domain and layout.${baseline ? ' This replaces the previous model snapshot; download it first if you need to keep it.' : ''}</p>`, 'Save snapshot', data => { const name = String(data.get('name')).trim(); if (!name) throw new Error('Give the snapshot a name.'); baseline = { name, capturedAt: new Date().toISOString(), project: cloneProject(project) }; persist(); render(); notify('Model snapshot captured'); });
}
function slug(value: string) { return value.toLocaleLowerCase().replace(/[^a-z0-9]+/g, '-').replace(/^-|-$/g, '') || 'domain'; }
function download(name: string, content: string, type = 'application/json') { const url = URL.createObjectURL(new Blob([content], { type })); const link = document.createElement('a'); link.href = url; link.download = name; link.click(); setTimeout(() => URL.revokeObjectURL(url), 1000); notify('Download prepared'); }
function exportDialog() {
  closeTools(); closeWorkspaceMenu(); closeCreationFan();
  const modal = $('#modal') as HTMLDialogElement;
  modal.innerHTML = `<div class="dialog-heading"><div><div class="eyebrow">TAKE YOUR MODEL WITH YOU</div><h2>Export domain</h2></div><button class="icon-button" id="close-dialog" aria-label="Close dialog">${icon('x')}</button></div><div class="dialog-content"><button class="export-option" id="download-workspace">${icon('box', 24)}<span><b>Download workspace</b><small>JSON · domain, layout, drafts, snapshot, and references</small></span>${icon('arrow-down-to-line')}</button><button class="export-option" id="download-yaml">${icon('file-code', 24)}<span><b>Download domain YAML</b><small>ArcLint vocabulary · unresolved meanings remain questions</small></span>${icon('arrow-down-to-line')}</button><p class="muted small">Workspace JSON preserves all visual edits. YAML export reports unsupported model changes instead of silently losing information.</p><p class="form-error" role="alert"></p></div>`;
  $('#close-dialog').onclick = () => { modal.close(); };
  $('#download-workspace').onclick = () => { download(`${slug(project.name)}.workspace.json`, workspace.serialize()); modal.close(); };
  $('#download-yaml').onclick = () => { try { download('domain.arclint.yaml', exportDomainYaml(project), 'application/yaml'); modal.close(); } catch (error) { modal.querySelector('.form-error')!.textContent = error instanceof Error ? error.message : String(error); } };
  refreshIcons(); modal.showModal();
}
function prepareAI() {
  const subjectId = selectedId ?? scopeId;
  const subject = [...project.contexts, ...project.concepts].find(c => c.id === subjectId);
  const concepts = project.concepts.filter(c => c.id === subjectId || c.contextId === subjectId);
  const ids = new Set([subjectId, ...concepts.map(c => c.id)]);
  const relationships = project.relationships.filter(r => ids.has(r.source) || ids.has(r.target));
  const request = `Help me review and improve the following part of the ${project.name} domain.\n\nSelected area: ${subject?.name ?? project.name}\n\nGoal: [Describe the change you want.]\n\nTreat this as a domain model, not proof about the code. Preserve confirmed vocabulary and boundaries. Identify missing evidence. Propose concrete changes before implementation.\n\nModel context:\n${JSON.stringify({ subject, concepts, relationships, connectedConcepts: project.concepts.filter(c => relationships.some(r => r.source === c.id || r.target === c.id)) }, null, 2)}`;
  showDialog('Prepare AI request', `<p class="muted">Edit this request, then use it in your AI harness. AI execution is not connected. The separate Check code action inspects the bound repository.</p><label class="field"><span>Request</span><textarea name="request" rows="12">${escape(request)}</textarea></label><button type="button" class="quiet-button" id="copy-request">${icon('copy')} Copy request</button>`, 'Download request', data => download(`${slug(subject?.name || project.name)}-request.md`, String(data.get('request')), 'text/markdown'));
  $('#copy-request').onclick = async () => { try { await navigator.clipboard.writeText(String(new FormData($('#dialog-form') as HTMLFormElement).get('request'))); notify('AI request copied'); } catch { notify('Clipboard unavailable. Download the request instead.', true); } };
}
function showFieldGuide() {
  const concept = project.concepts.find(c => c.id === selectedId);
  const kind = concept?.kind ?? (project.contexts.some(c => c.id === (selectedId ?? scopeId)) ? 'bounded_context' : 'domain');
  const entries: Record<string, [string, string, string]> = {
    domain: ['The recorded Ubiquitous Language', 'Record the project’s language in domain.arclint.yaml. Name the bounded contexts, their aggregates and values, and the invariants and operation post-conditions they carry.', 'A Rule states one Constraint over a Scope, with a Rationale when its author supplies one. ArcLint observes the repository and judges its code against the configured Rules. Source: domain.arclint.yaml; docs/site/content/docs/concepts.md.'],
    bounded_context: ['Bounded context', 'A boundary within which a model and its language have a consistent meaning. A word may mean something different in another context.', 'ArcLint locates a context’s code and checks its imports against recorded context relations. Zones name groups of paths; they are not interchangeable with bounded contexts.'],
    aggregate: ['Aggregate', 'A consistency boundary with one root. Its members must respect the invariants that the root protects.', 'ArcLint has built-in checks for recorded aggregate roots and invariant enforcement methods. Their presence is structural evidence, not proof of every business behavior.'],
    entity: ['Entity', 'Something recognized by its continuing identity, even when its attributes change. A member entity belongs to an aggregate.', 'The atlas records identity and aggregate ownership. Runtime behavior and persistence are outside this local model view.'],
    value_object: ['Value object', 'A concept defined by its values. Two instances with equal values are interchangeable in the model.', 'ArcLint checks recorded declarations and selected structural conventions. Equal-value behavior still requires evidence from the implementation.'],
    unclassified: ['An open question', 'A useful name whose identity, equality, responsibilities, or ownership still needs to be established.', 'Unresolved meanings remain open questions. YAML export records them as questions, rather than assigning a domain kind on your behalf.'],
  };
  const entry = entries[kind] ?? [KINDS[concept!.kind], 'Use a precise definition and the domain evidence to decide its responsibilities.', 'The atlas records your model. Code conformance is evaluated separately by ArcLint.'];
  showDialog('Field guide', `<article class="guide-entry"><span class="section-eyebrow">UNDERSTANDING THE MODEL</span><h3>${escape(entry[0])}</h3><p>${escape(entry[1])}</p><div class="section-label">WHAT IS CHECKED</div><p class="muted">${escape(entry[2])}</p>${concept ? `<div class="section-label">YOUR RECORDED MEANING</div><p>${escape(concept.definition || 'No definition recorded.')}</p>` : ''}</article>`, 'Return to domain', () => {});
}
function showHelp() { showDialog('Navigate your domain', '<div class="shortcut-list"><span>Orbit the map</span><kbd>Drag</kbd><span>Pan the camera</span><kbd>Right-drag</kbd><span>Zoom</span><kbd>Scroll</kbd><span>Arrange a domain entry</span><kbd>Shift + drag</kbd><span>Find in the domain</span><kbd>/</kbd><span>Build the domain</span><kbd>C</kbd><span>Frame whole domain</span><kbd>F</kbd><span>Undo / redo</span><kbd>Ctrl + Z / Shift + Z</kbd></div><p class="muted small">Moving objects changes layout only. Change the Context field to change its ownership. Every object is also selectable from the Index using a keyboard.</p>', 'Got it', () => {}); }
function reviewDomainSave() {
  const form = $('#inspector').querySelector<HTMLFormElement>('form');
  if (form) { if (!form.reportValidity()) return; form.requestSubmit(); if (form.querySelector('.form-error')?.textContent) return; }
  captureEditorDraft(); closeTools();
  try { workbench.saveDomain(exportDomainYaml(project)); } catch (error) { notify(error instanceof Error ? error.message : String(error),true); }
}
function renderConventional() {
  $('.atlas-world').dataset.representation = representation;
  $('#meaning-table').hidden = representation !== 'table';
  $('#scene').hidden = representation === 'table';
  $('#representation-spatial').setAttribute('aria-pressed', String(representation === 'spatial'));
  $('#representation-table').setAttribute('aria-pressed', String(representation === 'table'));
  const result = domainTable(project, scopeId, selectedId, tableQuery, tablePage); tablePage = result.page;
  $('#domain-table-content').innerHTML = result.html;
  $('#table-count').textContent = `${result.total} ${scopeId ? 'domain entries' : 'bounded contexts'}`;
  $('#table-page').textContent = `${result.page + 1} / ${result.pages}`;
  $('#table-prev').toggleAttribute('disabled', !result.page);
  $('#table-next').toggleAttribute('disabled', result.page + 1 >= result.pages);
  $('#meaning-table').querySelectorAll<HTMLElement>('[data-table-select]').forEach(button => button.onclick = () => { select(button.dataset.tableSelect!); if (project.concepts.some(c => c.id === selectedId)) openEditor(); });
  $('#meaning-table').querySelectorAll<HTMLElement>('[data-table-edit]').forEach(button => button.onclick = () => { select(button.dataset.tableEdit!); openEditor(); });
}
function setRepresentation(next: Representation) {
  captureEditorDraft(); representation = next; persist(); render();
}
function sizeComposer() { const input = $('#build-input') as HTMLTextAreaElement; input.style.height = '44px'; input.style.height = `${Math.min(124,Math.max(44,input.scrollHeight))}px`; }
function draftText() { return workspace.notebook.find(d => d.id === activeNotebookDraftId)?.text ?? ''; }
function commitAssignment(next: DomainProject, message: string) {
  const draftId = activeNotebookDraftId;
  const clearComposer = Boolean(draftId && workspace.getEditorDraft('composer')?.fields.text === draftText());
  commit(next, message, false, {consumeDraftId: draftId, ...(clearComposer ? {clearEditorDraftId: 'composer'} : {})});
  activeNotebookDraftId = undefined;
  if (clearComposer) { ($('#build-input') as HTMLTextAreaElement).value = ''; sizeComposer(); }
  persist();
}
function domainPicker(contextId?: string) {
  const choices = [['bounded_context','Bounded context','A boundary for a local language'], ...Object.entries(KINDS).map(([key,label]) => [key,label,key === 'unclassified' ? 'Keep an unresolved question' : key === 'aggregate' ? 'A root and its consistency boundary' : key === 'entity' ? 'An identity-bearing member' : key === 'value_object' ? 'A value and its invariants' : `Record a ${label.toLowerCase()}`]), ['invariant','Invariant','A promise an owner must always uphold'],['assertion','Assertion','What holds after an aggregate operation']];
  showDialog('Build the domain', `${activeNotebookDraftId ? `<p class="draft-excerpt">${escape(draftText())}</p><p class="muted small">Your text is saved in the notebook until you assign it.</p>` : '<p class="muted">Choose what you want to record. Keep uncertain meanings as open questions.</p>'}<div class="domain-kind-picker">${choices.map(([key,label,description]) => `<button type="button" data-create-kind="${key}"><b>${label}</b><span>${description}</span><em>↗</em></button>`).join('')}</div>`, 'Close', () => { activeNotebookDraftId = undefined; });
  document.querySelectorAll<HTMLElement>('[data-create-kind]').forEach(button => button.onclick = () => {
    ($('#modal') as HTMLDialogElement).close();
    const kind = button.dataset.createKind!;
    if (kind === 'bounded_context') contextDialog();
    else if (kind === 'invariant' || kind === 'assertion') contractDialog(kind);
    else conceptDialog(contextId,kind as ConceptKind);
  });
}
function contractDialog(kind: 'invariant' | 'assertion', existingKey?: string) {
  const owners = project.concepts.filter(c => (kind === 'assertion' ? c.kind === 'aggregate' : ['aggregate','value_object'].includes(c.kind)) && (!scopeId || c.contextId === scopeId));
  if (!owners.length) { notify(kind === 'assertion' ? 'Record an aggregate before attaching an operation assertion.' : 'Record an aggregate or value object before attaching an invariant.'); return; }
  const owner = owners.find(c => c.id === selectedId) ?? owners[0];
  const existing = owner.assertions?.find(a => a.key === existingKey);
  showDialog(`${existing ? 'Edit' : 'Add'} ${kind}`, `<label class="field"><span>Owner</span><select name="ownerId">${(existing ? [owner] : owners).map(c => `<option value="${escape(c.id)}" ${owner.id === c.id ? 'selected' : ''}>${escape(c.name)} · ${escape(KINDS[c.kind])}</option>`).join('')}</select></label>${kind === 'assertion' ? `${field('Key', 'key',existing?.key ?? '')}${field('After operation','on',existing?.on ?? '')}` : ''}${field('Statement','statement',existing?.statement ?? draftText(),true)}<p class="muted small">${kind === 'assertion' ? 'This postcondition belongs to the named operation. Code inspection is separate evidence.' : 'Record the condition that must always hold for this owner.'}</p>`, 'Save '+kind, data => {
    const next = cloneProject(project), target = next.concepts.find(c => c.id === data.get('ownerId'))!;
    if (kind === 'invariant') target.invariants.push(String(data.get('statement')).trim());
    else {
      const assertion = {key:String(data.get('key')).trim(),on:String(data.get('on')).trim(),statement:String(data.get('statement')).trim()};
      target.assertions = (target.assertions ?? []).filter(a => a.key !== existingKey);
      target.assertions.push(assertion);
    }
    assertProject(next);
    selectedId = target.id; scopeId = target.contextId; editorOpen = true;
    if (existing) commit(next, 'Assertion saved');
    else commitAssignment(next, `${kind === 'invariant' ? 'Invariant' : 'Assertion'} recorded`);
  });
}
function openNotebook() {
  const drafts = workspace.notebook;
  showDialog('Notebook', `<p class="muted">Unassigned text stays here until its meaning and context are known.</p><div class="notebook-entries">${drafts.map(d => `<article><p>${escape(d.text)}</p><small>${escape(new Date(d.updatedAt).toLocaleString())}</small><button type="button" data-assign-draft="${escape(d.id)}">Assign to the domain ↗</button><button type="button" data-remove-draft="${escape(d.id)}">Remove</button></article>`).join('') || '<p>No unassigned drafts. Start typing in the Build bar.</p>'}</div>`, 'Done', () => {});
  document.querySelectorAll<HTMLElement>('[data-assign-draft]').forEach(button => button.onclick = () => { activeNotebookDraftId = button.dataset.assignDraft; ($('#modal') as HTMLDialogElement).close(); domainPicker(scopeId ?? undefined); });
  document.querySelectorAll<HTMLElement>('[data-remove-draft]').forEach(button => button.onclick = () => { workspace.removeDraft(button.dataset.removeDraft!); persist(); openNotebook(); });
}
function toggleOnyx() {
  const support = $('#onyx-support'); support.hidden = !support.hidden;
  $('#keeper-action').setAttribute('aria-expanded',String(!support.hidden));
  if (!support.hidden) {
    if (!$('#onyx-conversation').children.length) onyxMessage('Onyx', 'I can help you record a meaning, find its source, or inspect the Rules that govern it. What are you working on?');
    $('#onyx-input').focus();
  }
}
function onyxMessage(speaker: string, text: string) {
  const message = document.createElement('p'); message.className = speaker === 'You' ? 'from-user' : 'from-onyx';
  const label = document.createElement('strong'); label.textContent = speaker;
  message.append(label,document.createTextNode(text)); $('#onyx-conversation').append(message); message.scrollIntoView({block:'nearest'});
}
function askOnyx(text: string) {
  onyxMessage('You',text);
  const subject = project.concepts.find(c => c.id === selectedId), context = project.contexts.find(c => c.id === (subject?.contextId ?? scopeId));
  if (/\b(rule|govern|policy|constraint)\b/i.test(text)) {
    onyxMessage('Onyx','The Rules workspace shows the actual propositions and their scopes. Select a path or Zone there to narrow what governs the code.');
    workbench.showGovernance('rules');
  } else if (/\b(fail|finding|check|violation|baseline)\b/i.test(text)) {
    const report = workbench.getReport();
    onyxMessage('Onyx',report ? `The last check returned ${report.diagnostics.length} Diagnostics. The findings view keeps active findings and Baseline acknowledgements distinct.` : 'No Report has been loaded in this session. Check code evaluates the bound repository on disk.');
    workbench.showGovernance('findings');
  } else if (/\b(path|source|file|code|zone)\b/i.test(text)) {
    onyxMessage('Onyx','I will open the source workspace. Located associations and matching Zones come from ArcLint; a matching name alone does not establish a source path.');
    workbench.showGovernance('paths');
  } else if (/\b(where|back|exit|inside)\b/i.test(text)) {
    onyxMessage('Onyx',`You are ${context ? `inside ${context.name}` : `at the ${project.name} project`}${subject ? `, looking at ${subject.name}` : ''}. Use the project name in the breadcrumb to leave the context, or Back to go up one level.`);
  } else if (/\b(define|meaning|what is|understand)\b/i.test(text) && (subject || context)) {
    onyxMessage('Onyx',subject ? `${subject.name} is recorded as ${KINDS[subject.kind].toLowerCase()} in ${context?.name}. ${subject.definition || 'Its definition is still open.'}` : context!.description || 'This context has no recorded definition yet.');
  } else {
    onyxMessage('Onyx','I can keep that exact text as a draft while you decide where it belongs. Use “Work on this meaning” to open the editor, or the Build bar to assign a new definition or question. For a free-form AI review, “Prepare AI request” includes your selected domain.');
    workspace.saveDraft(text); persist();
  }
}

$('#save-domain-repository').onclick = reviewDomainSave;
$('#representation-spatial').onclick = () => setRepresentation('spatial');
$('#representation-table').onclick = () => setRepresentation('table');
$('#table-query').oninput = event => { tableQuery = (event.target as HTMLInputElement).value; tablePage = 0; renderConventional(); };
$('#table-prev').onclick = () => { tablePage--; renderConventional(); };
$('#table-next').onclick = () => { tablePage++; renderConventional(); };
$('#build-input').oninput = () => { sizeComposer(); workspace.saveEditorDraft('composer',{text:($('#build-input') as HTMLInputElement).value}); persist(); };
$('#build-bar').onsubmit = event => { event.preventDefault(); const text = ($('#build-input') as HTMLInputElement).value; if (text.trim()) { activeNotebookDraftId = workspace.saveDraft(text,activeNotebookDraftId).id; persist(); } domainPicker(scopeId ?? undefined); };
$('#open-notebook').onclick = openNotebook;
$('#close-onyx').onclick = () => toggleOnyx();
$('#onyx-form').onsubmit = event => { event.preventDefault(); const input = $('#onyx-input') as HTMLInputElement; const text = input.value; input.value = ''; askOnyx(text); };
$('#onyx-define').onclick = () => { if (selectedId || scopeId) openEditor(); else domainPicker(); };
$('#onyx-rules').onclick = () => workbench.inspectSelection(selectedId ?? scopeId);
$('#onyx-paths').onclick = () => workbench.showGovernance('paths');
$('#onyx-request').onclick = prepareAI;
$('#governing-rules').onclick = () => { captureEditorDraft(); selectedId || scopeId ? workbench.inspectSelection(selectedId ?? scopeId) : workbench.showGovernance('rules'); };
$('#repository-findings').onclick = () => { captureEditorDraft(); workbench.showGovernance('findings'); };
$('#source-paths').onclick = () => { captureEditorDraft(); workbench.showGovernance('paths'); };
$('#check-code-now').onclick = () => { captureEditorDraft(); workbench.checkCode(); };
$('#map-frame').onclick = () => scene.overview();
$('#map-top').onclick = () => scene.topView();
$('#map-minus').onclick = () => scene.zoom(-1);
$('#map-plus').onclick = () => scene.zoom(1);

$('#undo').onclick = undo; $('#redo').onclick = redo; $('#add-context').onclick = contextDialog; $('#new-domain').onclick = newDomain;
$('#add-concept').onclick = () => conceptDialog(); $('#connect').onclick = relationshipDialog; $('#export').onclick = exportDialog; $('#import').onclick = () => { closeTools(); closeWorkspaceMenu(); $('#import-file').click(); };
$('#overview').onclick = () => { closeTools(); scene.overview(); }; $('#top-view').onclick = () => { closeTools(); scene.topView(); }; $('#zoom-in').onclick = () => { closeTools(); scene.zoom(1); }; $('#zoom-out').onclick = () => { closeTools(); scene.zoom(-1); }; $('#help').onclick = showHelp;
$('#open-index').onclick = openIndex;
$('#close-index').onclick = () => ($('#navigator') as HTMLDialogElement).close();
$('#workspace-menu-toggle').onclick = () => { const hidden = !$('#workspace-menu').hidden; $('#workspace-menu').hidden = hidden; $('#workspace-menu-toggle').setAttribute('aria-expanded', String(!hidden)); closeCreationFan(); };

$('#load-example-menu').onclick = loadExample;
$('#ascend').onclick = ascend;
$('#field-guide').onclick = showFieldGuide;
$('#menu-domain-details').onclick = () => $('#edit-project').click();
window.addEventListener('resize', positionManuscript);
document.addEventListener('pointerdown', e => { const target = e.target as HTMLElement; if (!target.closest('.workspace-access')) closeWorkspaceMenu(); if (!target.closest('.creation-compass')) closeCreationFan(); });
$('#edit-project').onclick = () => showDialog('Domain details', `${field('Name', 'name', project.name)}${field('Description', 'description', project.description, true, false)}`, 'Save domain', data => { const next = cloneProject(project); next.name = String(data.get('name')).trim(); next.description = String(data.get('description')).trim(); commit(next, 'Domain details saved'); });
$('#search').oninput = event => { search = (event.target as HTMLInputElement).value; renderTree(); refreshIcons(); scene.update(sceneState()); };
document.querySelectorAll<HTMLElement>('[data-mode]').forEach(b => b.onclick = () => { closeTools(); editorOpen = true; closeWorkspaceMenu(); closeCreationFan(); mode = b.dataset.mode as StudioMode; if (mode === 'domain') selectedId = null; render(); });
$('#import-file').onchange = async event => { const input = event.target as HTMLInputElement; const file = input.files?.[0]; if (!file) return; try { if (file.size > 5_000_000) throw new Error('Choose a domain file smaller than 5 MB.'); const contents = await file.text(); let saved: unknown; try { saved = JSON.parse(contents); } catch { /* Canonical YAML uses the domain importer. */ } if (saved && typeof saved === 'object' && (saved as {version?:number}).version === 2) { captureEditorDraft(); workspace.restore(saved,'Import complete workspace'); syncWorkspace(); editorOpen = false; persist(); render(); scene.overview(); notify(`Restored ${file.name}`); return; } const imported = importProject(contents); selectedId = null; mode = 'domain'; search = ''; ($('#search') as HTMLInputElement).value = ''; commit(imported, `Imported ${file.name}`, true); scene.overview(); } catch (error) { notify(`Import failed: ${error instanceof Error ? error.message : String(error)}`, true); } finally { input.value = ''; } };
$('#pattern-file').onchange = async event => { const input = event.target as HTMLInputElement; const file = input.files?.[0]; if (!file) return; try { if (file.size > 2_000_000) throw new Error('Choose a Pattern smaller than 2 MB.'); const pattern = parsePattern(await file.text()); patterns.push(pattern); persist(); render(); notify(`Pattern reference imported: ${pattern.name}`); } catch (error) { notify(`Pattern import failed: ${error instanceof Error ? error.message : String(error)}`, true); } finally { input.value = ''; } };
document.addEventListener('keydown', event => { if ($('#inspector').contains(event.target as Node)) { if (event.key === 'Escape') { event.preventDefault(); ascend(); } return; } if (document.querySelector('dialog[open]') || workSurface() || (event.target as HTMLElement).closest('input,textarea,select,[contenteditable]')) return; if ((event.ctrlKey || event.metaKey) && event.key.toLowerCase() === 'z') { event.preventDefault(); event.shiftKey ? redo() : undo(); } else if (event.key === '/') { event.preventDefault(); openIndex(); } else if (event.key.toLowerCase() === 'c') conceptDialog(); else if (event.key.toLowerCase() === 'f') scene.overview(); else if (event.key === 'Escape') { closeWorkspaceMenu(); closeCreationFan(); ascend(); } });
workbench = createWorkbench($('.atlas-world'), { select: id => { if (mode !== 'baseline') mode = 'domain'; select(id, true); }, replace: next => { selectedId = null; mode = 'domain'; search = ''; ($('#search') as HTMLInputElement).value = ''; commit(next, undefined, true); scene.overview(); }, create: () => conceptDialog(), notify });
scene = createScene($('#scene'), { keeper: () => $('#keeper-action').click(), select: id => { if (id && mode !== 'baseline') mode = 'domain'; select(id, false, true); }, enter, move: (id, position) => { const next = cloneProject(project); const c = next.concepts.find(c => c.id === id); if (c) { c.position = position; commit(next); } } });
$('#exit-comparison').onclick = () => { mode = 'domain'; editorOpen = false; render(); };
$('#tools-toggle').onclick = openTools;
$('#close-tools').onclick = closeTools;
$('#home-world').onclick = () => $('#realm-location').click();
$('#open-editor').onclick = openEditor;
$('#primary-action').onclick = () => { if (lens === 'meaning') scopeId ? domainPicker(scopeId) : contextDialog(); else workbench.inspectSelection(selectedId ?? scopeId); };
$('#keeper-action').onclick = toggleOnyx;
$('#lens-meaning').onclick = () => { lens = 'meaning'; render(); };
$('#lens-governance').onclick = () => { captureEditorDraft(); lens = 'governance'; render(); workbench.showGovernance('zones'); };
$('#view-prev').onclick = () => { viewPage--; render(); scene.overview(); };
$('#view-next').onclick = () => { viewPage++; render(); scene.overview(); };
// The focused work surface owns keyboard interaction until it is closed.
const worldElement = $('.atlas-world');
let activeSurface: HTMLElement | null = null;
let returnFocus: HTMLElement | null = null;
const workSurface = () => !$('#evidence-card').hidden ? $('#evidence-card') : null;
const surfaceFocusables = (surface: HTMLElement) => Array.from(surface.querySelectorAll<HTMLElement>('button:not(:disabled),input,textarea,select,summary,a[href],[tabindex="0"]')).filter(element => element.getClientRects().length && !element.closest('[hidden]'));
const syncWorkSurface = () => {
  const surface = workSurface();
  if (surface && !activeSurface) returnFocus = document.activeElement as HTMLElement;
  for (const child of Array.from(worldElement.children)) {
    if (child instanceof HTMLElement && !(child instanceof HTMLDialogElement)) child.inert = !!surface && child !== surface && child.id !== 'notice';
  }
  if (surface && !surface.contains(document.activeElement) && !document.querySelector('dialog[open]')) surfaceFocusables(surface)[0]?.focus();
  if (!surface && activeSurface && returnFocus?.isConnected && !returnFocus.closest('[inert]')) returnFocus.focus();
  activeSurface = surface;
};
new MutationObserver(syncWorkSurface).observe(worldElement, { subtree: true, attributes: true, attributeFilter: ['hidden'], childList: true });
document.addEventListener('keydown', event => {
  const surface = workSurface();
  if (!surface || document.querySelector('dialog[open]')) return;
  if (event.key === 'Escape') {
    event.preventDefault(); event.stopImmediatePropagation();
    surface.querySelector<HTMLElement>('#close-evidence,#clear-selection,#close-sheet')?.click();
  } else if (event.key === 'Tab') {
    const targets = surfaceFocusables(surface), first = targets[0], last = targets.at(-1);
    if (event.shiftKey && document.activeElement === first) { event.preventDefault(); last?.focus(); }
    else if (!event.shiftKey && document.activeElement === last) { event.preventDefault(); first?.focus(); }
  }
}, true);
($('#build-input') as HTMLInputElement).value = workspace.getEditorDraft('composer')?.fields.text ?? '';
sizeComposer();
if (!startupNotice) persist();
render();
if (startupNotice) {
  notify(startupNotice, true);
  const raw = recoveryRaw;
  if (raw) showDialog('Recover saved workspace', '<p>Your previous save could not be loaded. Download the original data before making edits so it can be recovered.</p>', 'Download saved data', () => { download('domain-studio-recovery.json', raw); startupNotice = ''; recoveryRaw = null; persist(); notify('Original save downloaded. New work can now be saved locally.'); });
}
// First use opens the bound project's actual language. A late response may never
// overwrite a draft, import, or interaction that the user has already started.
if (!recoveryRaw && !startupNotice) {
  const untouched = workspace.serialize();
  void loadRepository().then(repository => {
    if (workspace.serialize() !== untouched || document.querySelector('dialog[open]')) return;
    const next = repository.domainYaml ? importProject(repository.domainYaml) : {...createEmptyProject(),name:repository.repository.name};
    commit(next, `Opened ${repository.repository.name}`, true); scene.overview();
  }).catch(() => { /* Local modeling remains available without the repository bridge. */ });
}
window.addEventListener('beforeunload', () => scene.dispose());
