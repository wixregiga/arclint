import './style.css';
import { createIcons, ArrowUpLeft, Compass, BookOpen, Map, Menu, ArrowDown, ArrowDownToLine, ArrowUpDown, Box, Camera, Check, ChevronRight, CircleAlert, CircleCheck, CircleDashed, Copy, Download, File, FileCode, FolderOpen, HardDrive, Hexagon, History, Keyboard, Layers3, Minus, Network, Orbit, PanelLeftClose, Pencil, Plus, Redo2, Scan, Search, Sparkles, Undo2, Upload, View, Waypoints, X } from 'lucide';
const icons = { ArrowUpLeft, Compass, BookOpen, Map, Menu, ArrowDown, ArrowDownToLine, ArrowUpDown, Box, Camera, Check, ChevronRight, CircleAlert, CircleCheck, CircleDashed, Copy, Download, File, FileCode, FolderOpen, HardDrive, Hexagon, History, Keyboard, Layers3, Minus, Network, Orbit, PanelLeftClose, Pencil, Plus, Redo2, Scan, Search, Sparkles, Undo2, Upload, View, Waypoints, X };
import { createScene } from './scene';
import { createEmptyProject, createExampleProject, validateProject, diffProjects, assertProject, cloneProject, makeId } from './domain';
import { exportProject, importProject, exportDomainYaml, parsePattern } from './serialization';
import type { DomainProject, Baseline, StudioMode, PatternSummary, ConceptKind, Concept, SceneController } from './contracts';

const STORAGE_KEY = 'arclint.domain-studio.v1';
const COLORS = ['#637966', '#a38a50', '#455c71', '#987568', '#75877b', '#977f9a'];
const KINDS: Record<ConceptKind, string> = { unclassified: 'Unclassified', aggregate: 'Aggregate', entity: 'Entity', value_object: 'Value object', domain_event: 'Domain event', domain_service: 'Domain service', specification: 'Specification', repository: 'Repository', factory: 'Factory' };
const escape = (value: unknown) => String(value ?? '').replace(/[&<>"']/g, c => ({ '&': '&amp;', '<': '&lt;', '>': '&gt;', '"': '&quot;', "'": '&#39;' })[c]!);
const icon = (name: string, size = 16) => `<i data-lucide="${name}" width="${size}" height="${size}" aria-hidden="true"></i>`;
let project = createExampleProject();
let baseline: Baseline | null = null;
let patterns: PatternSummary[] = [];
let selectedId: string | null = null;
let mode: StudioMode = 'domain';
let search = '';
let scopeId: string | null = null;
let aggregateId: string | null = null;
let saveError = '';
let startupNotice = '';
interface WorkspaceSnapshot { project: DomainProject; baseline: Baseline | null; patterns: PatternSummary[] }
const history: WorkspaceSnapshot[] = [];
const future: WorkspaceSnapshot[] = [];
const snapshot = (): WorkspaceSnapshot => structuredClone({ project, baseline, patterns });
let recoveryRaw: string | null = null;
try {
  const raw = localStorage.getItem(STORAGE_KEY);
  recoveryRaw = raw;
  if (raw) {
    const saved = JSON.parse(raw);
    project = assertProject(saved.project);
    if (saved.baseline) baseline = { name: String(saved.baseline.name), capturedAt: String(saved.baseline.capturedAt), project: assertProject(saved.baseline.project) };
    patterns = Array.isArray(saved.patterns) ? saved.patterns.filter((p: PatternSummary) => typeof p?.name === 'string' && typeof p?.description === 'string' && Array.isArray(p?.rules) && p.rules.every(r => typeof r.id === 'string' && typeof r.description === 'string')) : [];
  }
} catch { if (recoveryRaw) startupNotice = 'Your previous save could not be loaded. Download the saved data before replacing it.'; else saveError = 'Local storage unavailable'; }

const app = document.querySelector<HTMLDivElement>('#app')!;
app.innerHTML = `
  <main class="atlas-world" aria-label="Domain atlas">
    <div id="scene" data-testid="scene" aria-label="Interactive domain landscape"></div>
    <div class="paper-grain" aria-hidden="true"></div>
    <div class="atlas-wordmark"><span class="atlas-seal" aria-hidden="true">${icon('waypoints', 24)}</span><div><span class="edition">ARCLINT · A LIVING RECORD</span><span class="atlas-name">The Domain Atlas</span></div></div>
    <section class="locator" aria-label="Your place in the domain">
      <div class="locator-rule"></div><span class="locator-caption">YOU ARE HERE</span>
      <button id="edit-project" class="project-title" title="Edit domain details"><h1 id="project-name"></h1>${icon('pencil', 12)}</button>
      <nav id="breadcrumbs" aria-label="Domain hierarchy"></nav>
      <button id="ascend" class="ascend" aria-label="Ascend one level">${icon('arrow-up-left', 16)}<span>Return one level</span><kbd>esc</kbd></button>
      <span id="project-description" class="sr-only"></span><span id="model-stats" class="sr-only"></span>
    </section>
    <div class="workspace-access"><button id="workspace-menu-toggle" class="workspace-toggle" aria-label="Workspace menu" aria-expanded="false">${icon('menu', 18)}<span>Workspace</span></button><div id="workspace-menu" class="workspace-menu manuscript" hidden><p class="menu-caption">THIS DOMAIN</p><button id="new-domain">${icon('file')} New domain</button><button id="import">${icon('upload')} Import</button><button id="export">${icon('download')} Export</button><button id="load-example-menu">${icon('folder-open')} Load example domain</button><span class="menu-rule"></span><button id="menu-domain-details">${icon('pencil')} Domain details</button></div></div>
    <nav class="atlas-instruments" aria-label="Atlas instruments">
      <button id="open-index" aria-label="Find a concept"><span>${icon('search', 20)}</span><b>Index</b><kbd>/</kbd></button>
      <button data-mode="patterns" aria-label="Patterns"><span>${icon('layers-3', 20)}</span><b>Patterns</b></button>
      <button data-mode="baseline" aria-label="Baseline"><span>${icon('history', 20)}</span><b>Compare</b></button>
      <button id="field-guide" aria-label="Field guide"><span>${icon('book-open', 20)}</span><b>Field guide</b></button>
    </nav>
    <section id="inspector" class="field-manuscript" aria-label="Selection details" hidden></section>
    <div id="notice" class="notice" role="status" aria-live="polite"></div>
    <div class="creation-compass" aria-label="Create in the domain">
      <div id="creation-fan" class="creation-fan" hidden><button id="add-context">${icon('hexagon', 19)}<span>Add context</span></button><button id="connect" aria-label="Connect concepts">${icon('waypoints', 19)}<span>Connect</span></button></div>
      <button id="create-toggle" class="create-stamp" aria-label="Create" aria-expanded="false">${icon('plus', 27)}<span>CREATE</span></button>
      <button id="add-concept" class="create-concept">Add concept <span>↗</span></button>
    </div>
    <div class="view-instruments" role="toolbar" aria-label="Map tools"><button id="overview" aria-label="Frame whole domain" title="Frame whole domain">${icon('scan', 18)}</button><button id="top-view" aria-label="Overhead view" title="Overhead view">${icon('map', 18)}</button><span></span><button id="zoom-out" aria-label="Zoom out">${icon('minus', 17)}</button><button id="zoom-in" aria-label="Zoom in">${icon('plus', 17)}</button><span></span><button id="undo" aria-label="Undo" title="Undo">${icon('undo-2', 17)}</button><button id="redo" aria-label="Redo" title="Redo">${icon('redo-2', 17)}</button></div>
    <div class="map-caption"><span class="caption-rule"></span><p id="level-hint">Every model begins with a place.<br /><small>Drag to explore · Double-click a context to enter</small></p></div>
    <div class="field-status"><span id="issues-status"></span><span id="save-status"></span><button id="help" aria-label="Keyboard shortcuts">${icon('keyboard', 15)}</button></div>
  </main>
  <dialog id="navigator" class="index-sheet manuscript" aria-label="Domain index"><div class="index-heading"><div><span class="edition">THE RECORDED LANGUAGE</span><h2>Find your place.</h2></div><button id="close-index" class="icon-button" aria-label="Close index">${icon('x', 20)}</button></div><label class="index-search">${icon('search', 20)}<input id="search" placeholder="A concept, context, or meaning…" aria-label="Find a concept" autocomplete="off" /><kbd>esc</kbd></label><div id="model-tree" data-testid="model-tree"></div><p class="index-footer">Select a name to locate it in the atlas. <span id="context-count"></span> contexts.</p></dialog>
  <input type="file" id="import-file" accept=".json,.yaml,.yml" hidden />
  <input type="file" id="pattern-file" accept=".yaml,.yml,.json" hidden />
  <dialog id="modal" class="studio-dialog manuscript"></dialog>
`;
const $ = <T extends HTMLElement = HTMLElement>(selector: string) => document.querySelector<T>(selector)!;
let scene: SceneController;
const refreshIcons = () => createIcons({ icons, attrs: { 'stroke-width': 1.6 } });
function notify(message: string, error = false) {
  $('#notice').textContent = message;
  $('#notice').className = `notice visible${error ? ' error' : ''}`;
  window.clearTimeout(noticeTimer);
  noticeTimer = window.setTimeout(() => $('#notice').classList.remove('visible'), error ? 10000 : 4500);
}
let noticeTimer = 0;
function persist() {
  try { localStorage.setItem(STORAGE_KEY, JSON.stringify({ project, baseline, patterns })); saveError = ''; }
  catch { saveError = 'Save failed · export a backup'; }
}
function commit(next: DomainProject, message?: string, replaceWorkspace = false) {
  // Canonical imports project aggregate ownership into named graph edges.
  // Keep these projections consistent when ownership changes in the editor.
  next.relationships = next.relationships.flatMap(r => {
    const member = next.concepts.find(c => c.id.startsWith('yaml:') && r.id === `${c.id}:owner`);
    if (!member) return [r];
    if (!member.ownerId || !['entity', 'repository', 'factory'].includes(member.kind)) return [];
    return [{ ...r, source: member.kind === 'entity' ? member.ownerId : member.id, target: member.kind === 'entity' ? member.id : member.ownerId, label: member.kind === 'entity' ? 'owns member' : member.kind === 'repository' ? 'repository for' : 'creates' }];
  });
  next = assertProject(next);
  history.push(snapshot());
  if (history.length > 60) history.shift();
  future.length = 0;
  project = next;
  if (replaceWorkspace) { baseline = null; patterns = []; scopeId = null; aggregateId = null; }
  if (scopeId && !project.contexts.some(c => c.id === scopeId)) { scopeId = null; aggregateId = null; }
  if (aggregateId && !project.concepts.some(c => c.id === aggregateId && c.kind === 'aggregate')) aggregateId = null;
  if (selectedId && ![...project.concepts, ...project.contexts, ...project.relationships].some(x => x.id === selectedId)) selectedId = null;
  persist(); render();
  if (message) notify(message);
}
function undo() { const last = history.pop(); if (!last) return; future.push(snapshot()); ({ project, baseline, patterns } = last); selectedId = null; persist(); render(); scene.overview(); notify('Change undone'); }
function redo() { const next = future.pop(); if (!next) return; history.push(snapshot()); ({ project, baseline, patterns } = next); selectedId = null; persist(); render(); scene.overview(); notify('Change restored'); }
function select(id: string | null, focus = false) {
  ($('#navigator') as HTMLDialogElement).close();
  selectedId = id;
  if (focus && id) {
    const concept = project.concepts.find(c => c.id === id);
    if (scopeId && concept && concept.contextId !== scopeId) { scopeId = concept.contextId; aggregateId = null; }
    if (aggregateId && concept && concept.id !== aggregateId && concept.ownerId !== aggregateId) aggregateId = null;
  }
  render();
  if (id && focus) { scene.focus(id); positionManuscript(); }
}
function enter(id: string) {
  const context = project.contexts.find(c => c.id === id);
  const concept = project.concepts.find(c => c.id === id);
  if (context) { scopeId = context.id; aggregateId = null; }
  else if (concept?.kind === 'aggregate') { scopeId = concept.contextId; aggregateId = concept.id; }
  else { select(id, true); return; }
  selectedId = null; mode = 'domain'; render();
}
function ascend() {
  if (aggregateId) aggregateId = null;
  else scopeId = null;
  selectedId = null; mode = 'domain'; render();
  if (!scopeId) scene.overview();
}
function openIndex() {
  closeWorkspaceMenu(); closeCreationFan();
  renderTree(); refreshIcons();
  const navigator = $('#navigator') as HTMLDialogElement;
  if (!navigator.open) navigator.showModal();
  $('#search').focus();
}
function closeWorkspaceMenu() { $('#workspace-menu').hidden = true; $('#workspace-menu-toggle').setAttribute('aria-expanded', 'false'); }
function closeCreationFan() { $('#creation-fan').hidden = true; $('#create-toggle').setAttribute('aria-expanded', 'false'); }
function positionManuscript() {
  const sheet = $('#inspector');
  if (sheet.hidden || window.innerWidth < 760) return;
  const anchor = selectedId ? scene?.screenPosition?.(selectedId) : null;
  const width = Math.min(370, window.innerWidth - 64);
  const preferred = anchor ? (anchor.x > window.innerWidth * .57 ? anchor.x - width - 55 : anchor.x + 55) : window.innerWidth * .5 - width * .5;
  const x = Math.max(30, Math.min(window.innerWidth - width - 110, preferred));
  const y = anchor ? Math.max(80, Math.min(window.innerHeight - 410, anchor.y - 155)) : Math.max(90, window.innerHeight * .14);
  sheet.style.left = `${x}px`; sheet.style.top = `${y}px`;
}
function renderLocation() {
  const context = project.contexts.find(c => c.id === scopeId);
  const aggregate = project.concepts.find(c => c.id === aggregateId);
  $('#breadcrumbs').innerHTML = `<button id="realm-location" aria-label="Domain" class="${context ? '' : 'current'}"><span class="location-dot"></span>All contexts</button>${context ? `<button id="context-location" class="${aggregate ? '' : 'current'}"><span class="location-dot"></span>${escape(context.name)}</button>` : ''}${aggregate ? `<span class="current"><span class="location-dot"></span>${escape(aggregate.name)}</span>` : ''}`;
  $('#realm-location').onclick = () => { scopeId = null; aggregateId = null; selectedId = null; mode = 'domain'; render(); scene.overview(); };
  $('#context-location')?.addEventListener('click', () => { aggregateId = null; selectedId = null; mode = 'domain'; render(); });
  $('#ascend').toggleAttribute('disabled', !scopeId && !aggregateId);
  $('#scene').dataset.scope = aggregateId ?? scopeId ?? 'realm';
  $('#level-hint').innerHTML = aggregate ? `Within ${escape(aggregate.name)}.<br /><small>Its recorded members and relationships</small>` : context ? `Inside ${escape(context.name)}.<br /><small>A shared language, within a boundary</small>` : 'Every model begins with a place.<br /><small>Drag to explore · Double-click a context to enter</small>';
}
function entityName(id: string) { return [...project.concepts, ...project.contexts].find(c => c.id === id)?.name ?? 'Removed concept'; }
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
    form.querySelector('[data-identity-field]')?.toggleAttribute('hidden', !['aggregate', 'entity'].includes(kind));
    form.querySelector('[data-owner-field]')?.toggleAttribute('hidden', !['entity', 'repository', 'factory'].includes(kind));
    form.querySelector('[data-alias-field]')?.toggleAttribute('hidden', !['aggregate', 'entity', 'value_object'].includes(kind));
    form.querySelector('[data-metadata-empty]')?.toggleAttribute('hidden', kind !== 'unclassified');
    const owner = form.elements.namedItem('ownerId') as HTMLSelectElement;
    if (owner) { const current = owner.value; owner.innerHTML = '<option value="">Not yet decided</option>' + project.concepts.filter(c => c.kind === 'aggregate' && c.contextId === contextId && c.id !== editedId).map(c => `<option value="${escape(c.id)}" ${c.id === current ? 'selected' : ''}>${escape(c.name)}</option>`).join(''); }
  };
  (form.elements.namedItem('kind') as HTMLElement)?.addEventListener('change', () => { update(); const details = form.querySelector('details'); if (details) details.open = true; });
  (form.elements.namedItem('contextId') as HTMLElement)?.addEventListener('change', update);
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
  }).join('') || `<div class="empty-tree">${icon('orbit', 25)}<p>${q ? 'No matching concepts.' : 'Your domain starts here.'}</p>${!q ? '<span>Add a context, then give its concepts a name and meaning.</span>' : ''}</div>`;
  document.querySelectorAll<HTMLButtonElement>('[data-select]').forEach(button => button.onclick = () => { mode = 'domain'; select(button.dataset.select!, true); });
}
function relationshipRows(id: string) {
  const links = project.relationships.filter(r => r.source === id || r.target === id);
  return links.map(r => `<button class="relationship-row" data-link="${escape(r.id)}"><span>${r.source === id ? '→' : '←'}</span><span><b>${escape(entityName(r.source === id ? r.target : r.source))}</b><small>${escape(r.label)}</small></span>${icon('chevron-right', 14)}</button>`).join('') || '<p class="muted">No relationships yet.</p>';
}
function renderInspector() {
  const findings = validateProject(project);
  const concept = project.concepts.find(c => c.id === selectedId);
  const context = project.contexts.find(c => c.id === selectedId);
  const relation = project.relationships.find(c => c.id === selectedId);
  let content = '';
  if (mode === 'baseline') {
    const changes = baseline ? diffProjects(baseline.project, project) : [];
    content = `<div class="panel-heading"><h2>Baseline</h2>${icon('history')}</div><div class="inspector-content"><div class="section-eyebrow">A POINT OF REFERENCE</div><h3>${baseline ? escape(baseline.name) : 'See how your model evolves.'}</h3><p class="muted">Save where you are. Changes to the model appear here and in the map.</p><button class="primary full" id="capture-baseline">${icon('camera')} ${baseline ? 'Replace baseline' : 'Capture baseline'}</button>${baseline ? `<div class="baseline-date">Captured ${escape(new Date(baseline.capturedAt).toLocaleString())}</div><div class="section-label">CHANGES <span>${changes.length}</span></div>${changes.map(c => `<button class="change-row ${c.type}" data-change="${escape(c.id)}"><span>${c.type === 'added' ? '+' : c.type === 'removed' ? '−' : '~'}</span><span><b>${escape(c.name)}</b><small>${escape(c.detail)}</small></span><em>${c.type}</em></button>`).join('') || '<div class="empty-state">Your model matches this baseline.</div>'}<button class="quiet-button full" id="download-baseline">${icon('download')} Download baseline</button>` : '<div class="baseline-illustration"><span></span><span></span><span></span></div>'}</div>`;
  } else if (mode === 'patterns') {
    content = `<div class="panel-heading"><h2>Patterns & checks</h2>${icon('layers-3')}</div><div class="inspector-content"><div class="section-eyebrow">MAKE THE MODEL EXPLICIT</div><h3>Give the structure a reason.</h3><p class="muted">Inspect a Pattern’s rules alongside your domain. Imported Patterns are reference material; they are not executed here.</p><button class="primary full" id="import-pattern">${icon('upload')} Import Pattern</button>${patterns.map((p, index) => `<details class="pattern-card"><summary>${escape(p.name)}<span>${p.rules.length} rules</span></summary><p>${escape(p.description)}</p>${p.rules.map(r => `<div class="pattern-rule"><b>${escape(r.id)}</b><p>${escape(r.description)}</p></div>`).join('')}<button class="quiet-button" data-remove-pattern="${index}">Remove reference</button></details>`).join('')}<div class="section-label">MODEL CHECKS <span>${findings.length}</span></div><p class="small muted">Local completeness checks. Code conformance requires ArcLint.</p>${findingRows(findings)}</div>`;
  } else if (concept) {
    const ctx = project.contexts.find(c => c.id === concept.contextId)!;
    content = `<div class="panel-heading"><h2>Concept</h2><button class="icon-button" id="clear-selection" aria-label="Clear selection">${icon('x')}</button></div><div class="inspector-content"><div class="selection-kind">${icon('box', 16)} ${escape(KINDS[concept.kind])}<button id="learn-selection" class="learn-link">Learn</button></div>${concept.kind === 'aggregate' ? `<button class="enter-place" id="enter-aggregate">Explore aggregate ${icon('arrow-down')}</button>` : ''}<form id="concept-form">${field('Name', 'name', concept.name)}${field('Definition', 'definition', concept.definition, true, false)}<div class="field-pair"><label class="field"><span>Kind</span><select name="kind">${kindOptions(concept.kind)}</select></label><label class="field"><span>Context</span><select name="contextId">${contextOptions(concept.contextId)}</select></label></div>${metadataFields(concept)}${field('Invariants', 'invariants', concept.invariants.join('\n'), true, false)}<p class="field-help">One invariant per line. Record what must always hold.</p><p class="form-error" role="alert"></p><button class="primary full" type="submit">${icon('check', 16)} Save concept</button></form><div class="section-label">RELATIONSHIPS <button class="icon-button" id="connect-selected" aria-label="Add relationship">${icon('plus', 15)}</button></div>${relationshipRows(concept.id)}<div class="inspector-bottom"><button class="quiet-button full" id="prepare-ai">${icon('sparkles', 16)} Prepare AI request</button><button class="danger-link" id="delete-selection">Delete concept</button></div></div>`;
  } else if (context) {
    content = `<div class="panel-heading"><h2>Bounded context</h2><button class="icon-button" id="clear-selection" aria-label="Clear selection">${icon('x')}</button></div><div class="inspector-content"><div class="selection-kind">${icon('hexagon')} BOUNDED CONTEXT<button id="learn-selection" class="learn-link">Learn</button></div><button class="enter-place" id="enter-context">Enter context ${icon('arrow-down')}</button><form id="context-form">${field('Name', 'name', context.name)}${field('Definition', 'description', context.description, true, false)}<label class="field"><span>Map color</span><input name="color" type="color" value="${escape(context.color)}" /></label><p class="form-error" role="alert"></p><button class="primary full" type="submit">${icon('check')} Save context</button></form><div class="section-label">CONCEPTS <span>${project.concepts.filter(c => c.contextId === context.id).length}</span></div><button class="quiet-button full" id="add-in-context">${icon('plus')} Add concept here</button><div class="section-label">RELATIONSHIPS <button class="icon-button" id="connect-selected" aria-label="Add relationship">${icon('plus', 15)}</button></div>${relationshipRows(context.id)}<div class="inspector-bottom"><button class="quiet-button full" id="prepare-ai">${icon('sparkles')} Prepare AI request</button><button class="danger-link" id="delete-selection">Delete context</button></div></div>`;
  } else if (relation) {
    content = `<div class="panel-heading"><h2>Relationship</h2><button class="icon-button" id="clear-selection" aria-label="Clear selection">${icon('x')}</button></div><div class="inspector-content"><div class="connection-preview"><b>${escape(entityName(relation.source))}</b>${icon('arrow-down', 24)}<b>${escape(entityName(relation.target))}</b></div><form id="relationship-form">${field('Relationship', 'label', relation.label)}<p class="form-error" role="alert"></p><button class="primary full" type="submit">Save relationship</button></form><button class="quiet-button full" id="reverse-relationship">${icon('arrow-up-down')} Reverse direction</button><button class="danger-link" id="delete-selection">Delete relationship</button></div>`;
  } else {
    $('#inspector').hidden = true;
    $('#inspector').replaceChildren();
    return;
  }
  $('#inspector').hidden = false;
  $('#inspector').dataset.kind = mode === 'domain' ? 'object' : 'overlay';
  $('#inspector').innerHTML = content;
  if (mode !== 'domain') $('#inspector .panel-heading').insertAdjacentHTML('beforeend', `<button class="icon-button" id="close-sheet" aria-label="Close sheet">${icon('x', 18)}</button>`);
  bindInspector();
}
function findingRows(findings: ReturnType<typeof validateProject>) { return findings.map(f => `<button class="finding-row ${f.severity}" data-finding="${escape(f.subjectId)}">${icon('circle-alert', 15)}<span><b>${escape(f.title)}</b><small>${escape(f.message)}</small></span></button>`).join('') || '<div class="check-clear">No model completeness issues found.</div>'; }
function render() {
  $('#project-name').textContent = project.name;
  $('#project-description').textContent = project.description;
  $('#model-stats').textContent = `${project.contexts.length} CONTEXTS   /   ${project.concepts.length} CONCEPTS   /   ${project.relationships.length} RELATIONSHIPS`;
  const findings = validateProject(project);
  $('#issues-status').innerHTML = `<button id="show-checks">${icon(findings.length ? 'circle-dashed' : 'circle-check', 13)} ${findings.length ? `${findings.length} items to review` : 'Model checks clear'}</button>`;
  $('#show-checks').onclick = () => { mode = 'patterns'; render(); };
  $('#save-status').innerHTML = `<span class="status-light ${saveError ? 'warning' : ''}"></span>${escape(saveError || 'Saved locally')}`;
  $('#undo').toggleAttribute('disabled', !history.length);
  $('#redo').toggleAttribute('disabled', !future.length);
  document.querySelectorAll<HTMLElement>('[data-mode]').forEach(b => { b.classList.toggle('active', b.dataset.mode === mode); b.setAttribute('aria-pressed', String(b.dataset.mode === mode)); });
  renderTree(); renderInspector(); refreshIcons();
  renderLocation();
  scene?.update({ project, selectedId, mode, baseline, findings, search, scopeId, aggregateId });
  positionManuscript();
}
function safely(form: HTMLFormElement, action: () => void) { try { action(); } catch (error) { form.querySelector('.form-error')!.textContent = error instanceof Error ? error.message : String(error); } }
function bindInspector() {
  $('#clear-selection')?.addEventListener('click', () => select(null));
  $('#close-sheet')?.addEventListener('click', () => { mode = 'domain'; select(null); });
  $('#enter-context')?.addEventListener('click', () => { if (selectedId) enter(selectedId); });
  $('#enter-aggregate')?.addEventListener('click', () => { if (selectedId) enter(selectedId); });
  $('#learn-selection')?.addEventListener('click', showFieldGuide);
  $('#overview-add')?.addEventListener('click', () => conceptDialog());
  $('#add-in-context')?.addEventListener('click', () => conceptDialog(selectedId!));
  $('#all-checks')?.addEventListener('click', () => { mode = 'patterns'; render(); });
  $('#connect-selected')?.addEventListener('click', () => relationshipDialog());
  $('#prepare-ai')?.addEventListener('click', prepareAI);
  $('#delete-selection')?.addEventListener('click', deleteSelection);
  $('#capture-baseline')?.addEventListener('click', captureBaseline);
  $('#import-pattern')?.addEventListener('click', () => $('#pattern-file').click());
  $('#load-example')?.addEventListener('click', loadExample);
  $('#download-baseline')?.addEventListener('click', () => { if (baseline) download(`${slug(baseline.name)}.baseline.json`, exportProject(baseline.project)); });
  $('#reverse-relationship')?.addEventListener('click', () => { const next = cloneProject(project); const r = next.relationships.find(r => r.id === selectedId)!; [r.source, r.target] = [r.target, r.source]; commit(next, 'Relationship reversed'); });
  document.querySelectorAll<HTMLElement>('[data-link]').forEach(b => b.onclick = () => select(b.dataset.link!));
  document.querySelectorAll<HTMLElement>('[data-finding]').forEach(b => b.onclick = () => { mode = 'domain'; select(b.dataset.finding!, true); });
  document.querySelectorAll<HTMLElement>('[data-change]').forEach(b => b.onclick = () => { selectedId = b.dataset.change!; scene.focus(selectedId); scene.update({ project, selectedId, mode, baseline, findings: validateProject(project), search, scopeId, aggregateId }); });
  document.querySelectorAll<HTMLElement>('[data-remove-pattern]').forEach(b => b.onclick = () => { patterns.splice(Number(b.dataset.removePattern), 1); persist(); render(); });
  const cf = document.querySelector<HTMLFormElement>('#concept-form');
  if (cf) bindMetadata(cf, selectedId);
  if (cf) cf.onsubmit = event => { event.preventDefault(); safely(cf, () => { const data = new FormData(cf); const next = cloneProject(project); const c = next.concepts.find(c => c.id === selectedId)!; c.name = String(data.get('name')).trim(); c.definition = String(data.get('definition')).trim(); c.kind = String(data.get('kind')) as ConceptKind; const contextId = String(data.get('contextId')); if (contextId !== c.contextId) { const ctx = next.contexts.find(x => x.id === contextId)!; c.position = [ctx.position[0] + 3, 2, ctx.position[2] + 3]; } c.contextId = contextId; const invariantInput = String(data.get('invariants')); if (invariantInput !== c.invariants.join('\n')) c.invariants = invariantInput.split('\n').map(x => x.trim()).filter(Boolean); applyMetadata(c, data); next.concepts.forEach(member => { if (member.ownerId === c.id && (c.kind !== 'aggregate' || member.contextId !== c.contextId)) delete member.ownerId; }); commit(next, 'Concept saved'); }); };
  const ctxf = document.querySelector<HTMLFormElement>('#context-form');
  if (ctxf) ctxf.onsubmit = event => { event.preventDefault(); safely(ctxf, () => { const data = new FormData(ctxf); const next = cloneProject(project); const c = next.contexts.find(c => c.id === selectedId)!; c.name = String(data.get('name')).trim(); c.description = String(data.get('description')).trim(); c.color = String(data.get('color')); commit(next, 'Context saved'); }); };
  const rf = document.querySelector<HTMLFormElement>('#relationship-form');
  if (rf) rf.onsubmit = event => { event.preventDefault(); safely(rf, () => { const next = cloneProject(project); next.relationships.find(r => r.id === selectedId)!.label = String(new FormData(rf).get('label')).trim(); commit(next, 'Relationship saved'); }); };
}
function showDialog(title: string, contents: string, submitLabel: string, onSubmit: (data: FormData) => void) {
  closeWorkspaceMenu(); closeCreationFan();
  const modal = $('#modal') as HTMLDialogElement;
  modal.innerHTML = `<form id="dialog-form"><div class="dialog-heading"><div><div class="eyebrow">THE DOMAIN ATLAS</div><h2>${escape(title)}</h2></div><button type="button" class="icon-button" id="close-dialog" aria-label="Close dialog">${icon('x', 20)}</button></div><div class="dialog-content">${contents}<p class="form-error" role="alert"></p></div><div class="dialog-actions"><button class="quiet-button" type="button" id="cancel-dialog">Cancel</button><button class="primary" type="submit">${escape(submitLabel)}</button></div></form>`;
  $('#close-dialog').onclick = () => modal.close(); $('#cancel-dialog').onclick = () => modal.close();
  $('#dialog-form').onsubmit = event => { event.preventDefault(); safely($('#dialog-form') as HTMLFormElement, () => { onSubmit(new FormData($('#dialog-form') as HTMLFormElement)); modal.close(); }); };
  refreshIcons(); if (!modal.open) modal.showModal();
  setTimeout(() => modal.querySelector<HTMLInputElement>('input,textarea,select')?.focus(), 20);
}
function contextDialog() {
  showDialog('Add context', `${field('Name', 'name')}${field('Definition', 'description', '', true, false)}<p class="muted small">A boundary within which concepts have a shared meaning.</p>`, 'Create context', data => {
    const next = cloneProject(project); const index = next.contexts.length; const angle = index * 2.4;
    const id = makeId('context'); next.contexts.push({ id, name: String(data.get('name')).trim(), description: String(data.get('description')).trim(), color: COLORS[index % COLORS.length], position: index === 0 ? [0, 0, 0] : [Math.cos(angle) * (18 + index * 3), 0, Math.sin(angle) * (18 + index * 3)] }); selectedId = id; mode = 'domain'; commit(next, 'Context created'); scene.focus(id);
  });
}
function conceptDialog(contextId?: string) {
  if (!project.contexts.length) { contextDialog(); notify('Create a context first, then add its concepts.'); return; }
  const selectedContext = contextId || project.concepts.find(c => c.id === selectedId)?.contextId || project.contexts.find(c => c.id === selectedId)?.id || scopeId || project.contexts[0].id;
  showDialog('Add concept', `${field('Name', 'name')}${field('Definition', 'definition', '', true, false)}<div class="field-pair"><label class="field"><span>Kind</span><select name="kind">${kindOptions()}</select></label><label class="field"><span>Context</span><select name="contextId">${contextOptions(selectedContext)}</select></label></div>${metadataFields()}${field('Invariants', 'invariants', '', true, false)}<p class="muted small">Keep a concept unclassified until its identity and responsibilities are understood.</p>`, 'Create concept', data => {
    const next = cloneProject(project); const ctx = next.contexts.find(c => c.id === data.get('contextId'))!; const count = next.concepts.filter(c => c.contextId === ctx.id).length; const angle = count * 2.4; const radius = 4 + Math.floor(count / 5) * 4; const id = makeId('concept');
    next.concepts.push({ id, name: String(data.get('name')).trim(), definition: String(data.get('definition')).trim(), kind: String(data.get('kind')) as ConceptKind, contextId: ctx.id, invariants: String(data.get('invariants')).split('\n').map(x => x.trim()).filter(Boolean), position: [ctx.position[0] + Math.cos(angle) * radius, 1 + count % 3, ctx.position[2] + Math.sin(angle) * radius] }); applyMetadata(next.concepts.at(-1)!, data); selectedId = id; mode = 'domain'; commit(next, 'Concept created'); scene.focus(id);
  });
  bindMetadata($('#dialog-form') as HTMLFormElement);
}
function relationshipDialog() {
  const entities = [...project.contexts, ...project.concepts];
  if (entities.length < 2) { notify('Add at least two concepts or contexts to connect.'); return; }
  const opts = (selected?: string) => entities.map(c => `<option value="${escape(c.id)}" ${c.id === selected ? 'selected' : ''}>${escape(c.name)}${'contextId' in c ? '' : ' · context'}</option>`).join('');
  const first = entities.find(c => c.id === selectedId)?.id || entities[0].id;
  showDialog('Connect concepts', `<label class="field"><span>From</span><select name="source">${opts(first)}</select></label><div class="connection-arrow">${icon('arrow-down')}</div><label class="field"><span>To</span><select name="target">${opts(entities.find(c => c.id !== first)!.id)}</select></label>${field('Relationship', 'label')}<p class="muted small">Use your domain’s language: contains, depends on, publishes, or a more precise meaning.</p>`, 'Create relationship', data => {
    if (data.get('source') === data.get('target')) throw new Error('Choose two different objects.');
    const next = cloneProject(project); const id = makeId('relation'); next.relationships.push({ id, source: String(data.get('source')), target: String(data.get('target')), label: String(data.get('label')).trim() }); selectedId = id; mode = 'domain'; commit(next, 'Relationship created');
  });
}
function deleteSelection() {
  const object = [...project.contexts, ...project.concepts, ...project.relationships].find(c => c.id === selectedId); if (!object) return;
  const isContext = project.contexts.some(c => c.id === selectedId);
  showDialog('Delete from domain', `<p>Delete <strong>${escape('name' in object ? object.name : object.label)}</strong>${isContext ? ' and every concept inside it' : ''}? Its relationships will also be removed.</p><p class="muted small">You can undo this change.</p>`, 'Delete', () => {
    const next = cloneProject(project); const ids = new Set([selectedId!]); if (isContext) next.concepts.filter(c => c.contextId === selectedId).forEach(c => ids.add(c.id)); next.concepts = next.concepts.filter(c => !ids.has(c.id)); next.concepts.forEach(c => { if (c.ownerId && ids.has(c.ownerId)) delete c.ownerId; }); next.contexts = next.contexts.filter(c => !ids.has(c.id)); next.relationships = next.relationships.filter(r => !ids.has(r.id) && !ids.has(r.source) && !ids.has(r.target)); commit(next, 'Deleted from domain');
  });
}
function newDomain() {
  showDialog('New domain', `${field('Name', 'name')}${field('Description', 'description', '', true, false)}<p class="muted small">Start an empty model. Your current model can be recovered with Undo. Export it to keep a separate copy.</p>`, 'Create domain', data => {
    const next = createEmptyProject(); next.name = String(data.get('name')).trim(); next.description = String(data.get('description')).trim(); selectedId = null; mode = 'domain'; search = ''; ($('#search') as HTMLInputElement).value = ''; commit(next, 'New domain created', true); scene.overview();
  });
}
function loadExample() {
  showDialog('Load example domain', '<p>Open the bskilled draft to explore the editor. Its contexts and concept kinds are proposals.</p><p class="muted small">This replaces your current model. Undo will bring it back.</p>', 'Load example', () => { selectedId = null; mode = 'domain'; commit(createExampleProject(), 'Example domain loaded', true); scene.overview(); });
}
function captureBaseline() {
  showDialog('Capture baseline', `${field('Name', 'name', `${project.name} · starting point`)}<p class="muted small">Capture the current domain and layout.${baseline ? ' This replaces the previous baseline; download it first if you need to keep it.' : ''}</p>`, 'Save baseline', data => { const name = String(data.get('name')).trim(); if (!name) throw new Error('Give the baseline a name.'); baseline = { name, capturedAt: new Date().toISOString(), project: cloneProject(project) }; persist(); render(); notify('Baseline captured'); });
}
function slug(value: string) { return value.toLocaleLowerCase().replace(/[^a-z0-9]+/g, '-').replace(/^-|-$/g, '') || 'domain'; }
function download(name: string, content: string, type = 'application/json') { const url = URL.createObjectURL(new Blob([content], { type })); const link = document.createElement('a'); link.href = url; link.download = name; link.click(); setTimeout(() => URL.revokeObjectURL(url), 1000); notify('Download prepared'); }
function exportDialog() {
  closeWorkspaceMenu(); closeCreationFan();
  const modal = $('#modal') as HTMLDialogElement;
  modal.innerHTML = `<div class="dialog-heading"><div><div class="eyebrow">TAKE YOUR MODEL WITH YOU</div><h2>Export domain</h2></div><button class="icon-button" id="close-dialog" aria-label="Close dialog">${icon('x')}</button></div><div class="dialog-content"><button class="export-option" id="download-workspace">${icon('box', 24)}<span><b>Download workspace</b><small>JSON · complete model, relationships, and layout</small></span>${icon('arrow-down-to-line')}</button><button class="export-option" id="download-yaml">${icon('file-code', 24)}<span><b>Download domain YAML</b><small>ArcLint vocabulary · unresolved concepts become questions</small></span>${icon('arrow-down-to-line')}</button><p class="muted small">Workspace JSON preserves all visual edits. YAML export reports unsupported model changes instead of silently losing information.</p><p class="form-error" role="alert"></p></div>`;
  $('#close-dialog').onclick = () => { modal.close(); };
  $('#download-workspace').onclick = () => { download(`${slug(project.name)}.domain.json`, exportProject(project)); modal.close(); };
  $('#download-yaml').onclick = () => { try { download('domain.arclint.yaml', exportDomainYaml(project), 'application/yaml'); modal.close(); } catch (error) { modal.querySelector('.form-error')!.textContent = error instanceof Error ? error.message : String(error); } };
  refreshIcons(); modal.showModal();
}
function prepareAI() {
  const subject = [...project.contexts, ...project.concepts].find(c => c.id === selectedId);
  const concepts = project.concepts.filter(c => c.id === selectedId || c.contextId === selectedId);
  const ids = new Set([selectedId, ...concepts.map(c => c.id)]);
  const relationships = project.relationships.filter(r => ids.has(r.source) || ids.has(r.target));
  const request = `Help me review and improve the following part of the ${project.name} domain.\n\nSelected area: ${subject?.name ?? project.name}\n\nGoal: [Describe the change you want.]\n\nTreat this as a domain model, not proof about the code. Preserve confirmed vocabulary and boundaries. Identify missing evidence. Propose concrete changes before implementation.\n\nModel context:\n${JSON.stringify({ subject, concepts, relationships, connectedConcepts: project.concepts.filter(c => relationships.some(r => r.source === c.id || r.target === c.id)) }, null, 2)}`;
  showDialog('Prepare AI request', `<p class="muted">Edit this request, then use it in your AI harness. No AI or code execution is connected to this workspace.</p><label class="field"><span>Request</span><textarea name="request" rows="12">${escape(request)}</textarea></label><button type="button" class="quiet-button" id="copy-request">${icon('copy')} Copy request</button>`, 'Download request', data => download(`${slug(subject?.name || project.name)}-request.md`, String(data.get('request')), 'text/markdown'));
  $('#copy-request').onclick = async () => { try { await navigator.clipboard.writeText(String(new FormData($('#dialog-form') as HTMLFormElement).get('request'))); notify('AI request copied'); } catch { notify('Clipboard unavailable. Download the request instead.', true); } };
}
function showFieldGuide() {
  const concept = project.concepts.find(c => c.id === selectedId);
  const kind = concept?.kind ?? (project.contexts.some(c => c.id === selectedId) ? 'bounded_context' : 'domain');
  const entries: Record<string, [string, string, string]> = {
    domain: ['A model you can inhabit', 'A domain model records the concepts people use, their meanings, and the boundaries within which those meanings hold.', 'The atlas lets you author the model. Completeness checks here do not establish that code conforms to it.'],
    bounded_context: ['Bounded context', 'A boundary within which a model and its language have a consistent meaning. A word may mean something different in another context.', 'ArcLint can compare recorded context relations with imports when code zones and observations are available. This atlas has not scanned your code.'],
    aggregate: ['Aggregate', 'A consistency boundary with one root. Its members must respect the invariants that the root protects.', 'ArcLint has built-in checks for recorded aggregate roots and invariant enforcement methods. Their presence is structural evidence, not proof of every business behavior.'],
    entity: ['Entity', 'Something recognized by its continuing identity, even when its attributes change. A member entity belongs to an aggregate.', 'The atlas records identity and aggregate ownership. Runtime behavior and persistence are outside this local model view.'],
    value_object: ['Value object', 'A concept defined by its values. Two instances with equal values are interchangeable in the model.', 'ArcLint checks recorded declarations and selected structural conventions. Equal-value behavior still requires evidence from the implementation.'],
    unclassified: ['An open question', 'A useful name whose identity, equality, responsibilities, or ownership still needs to be established.', 'Unclassified concepts remain drafts. YAML export records them as questions, rather than assigning a domain kind on your behalf.'],
  };
  const entry = entries[kind] ?? [KINDS[concept!.kind], 'Use a precise definition and the domain evidence to decide this concept’s responsibilities.', 'The atlas records your model. Code conformance is evaluated separately by ArcLint.'];
  showDialog('Field guide', `<article class="guide-entry"><span class="section-eyebrow">UNDERSTANDING THE MODEL</span><h3>${escape(entry[0])}</h3><p>${escape(entry[1])}</p><div class="section-label">WHAT IS CHECKED</div><p class="muted">${escape(entry[2])}</p>${concept ? `<div class="section-label">YOUR RECORDED MEANING</div><p>${escape(concept.definition || 'No definition recorded.')}</p>` : ''}</article>`, 'Return to atlas', () => {});
}
function showHelp() { showDialog('Navigate your domain', '<div class="shortcut-list"><span>Orbit the map</span><kbd>Drag</kbd><span>Pan the camera</span><kbd>Right-drag</kbd><span>Zoom</span><kbd>Scroll</kbd><span>Arrange a concept</span><kbd>Shift + drag</kbd><span>Find a concept</span><kbd>/</kbd><span>Add a concept</span><kbd>C</kbd><span>Frame whole domain</span><kbd>F</kbd><span>Undo / redo</span><kbd>Ctrl + Z / Shift + Z</kbd></div><p class="muted small">Moving objects changes layout only. Change a concept’s Context field to change its ownership. Every object is also selectable from the Index using a keyboard.</p>', 'Got it', () => {}); }
$('#undo').onclick = undo; $('#redo').onclick = redo; $('#add-context').onclick = contextDialog; $('#new-domain').onclick = newDomain;
$('#add-concept').onclick = () => conceptDialog(); $('#connect').onclick = relationshipDialog; $('#export').onclick = exportDialog; $('#import').onclick = () => { closeWorkspaceMenu(); $('#import-file').click(); };
$('#overview').onclick = () => scene.overview(); $('#top-view').onclick = () => scene.topView(); $('#zoom-in').onclick = () => scene.zoom(1); $('#zoom-out').onclick = () => scene.zoom(-1); $('#help').onclick = showHelp;
$('#open-index').onclick = openIndex;
$('#close-index').onclick = () => ($('#navigator') as HTMLDialogElement).close();
$('#workspace-menu-toggle').onclick = () => { const hidden = !$('#workspace-menu').hidden; $('#workspace-menu').hidden = hidden; $('#workspace-menu-toggle').setAttribute('aria-expanded', String(!hidden)); closeCreationFan(); };
$('#create-toggle').onclick = () => { const hidden = !$('#creation-fan').hidden; $('#creation-fan').hidden = hidden; $('#create-toggle').setAttribute('aria-expanded', String(!hidden)); closeWorkspaceMenu(); };
$('#load-example-menu').onclick = loadExample;
$('#ascend').onclick = ascend;
$('#field-guide').onclick = showFieldGuide;
$('#menu-domain-details').onclick = () => $('#edit-project').click();
window.addEventListener('resize', positionManuscript);
document.addEventListener('pointerdown', e => { const target = e.target as HTMLElement; if (!target.closest('.workspace-access')) closeWorkspaceMenu(); if (!target.closest('.creation-compass')) closeCreationFan(); });
$('#edit-project').onclick = () => showDialog('Domain details', `${field('Name', 'name', project.name)}${field('Description', 'description', project.description, true, false)}`, 'Save domain', data => { const next = cloneProject(project); next.name = String(data.get('name')).trim(); next.description = String(data.get('description')).trim(); commit(next, 'Domain details saved'); });
$('#search').oninput = event => { search = (event.target as HTMLInputElement).value; renderTree(); refreshIcons(); scene.update({ project, selectedId, mode, baseline, findings: validateProject(project), search, scopeId, aggregateId }); };
document.querySelectorAll<HTMLElement>('[data-mode]').forEach(b => b.onclick = () => { closeWorkspaceMenu(); closeCreationFan(); mode = b.dataset.mode as StudioMode; if (mode === 'domain') selectedId = null; render(); });
$('#import-file').onchange = async event => { const input = event.target as HTMLInputElement; const file = input.files?.[0]; if (!file) return; try { if (file.size > 5_000_000) throw new Error('Choose a domain file smaller than 5 MB.'); const imported = importProject(await file.text()); selectedId = null; mode = 'domain'; search = ''; ($('#search') as HTMLInputElement).value = ''; commit(imported, `Imported ${file.name}`, true); scene.overview(); } catch (error) { notify(`Import failed: ${error instanceof Error ? error.message : String(error)}`, true); } finally { input.value = ''; } };
$('#pattern-file').onchange = async event => { const input = event.target as HTMLInputElement; const file = input.files?.[0]; if (!file) return; try { if (file.size > 2_000_000) throw new Error('Choose a Pattern smaller than 2 MB.'); const pattern = parsePattern(await file.text()); patterns.push(pattern); persist(); render(); notify(`Pattern reference imported: ${pattern.name}`); } catch (error) { notify(`Pattern import failed: ${error instanceof Error ? error.message : String(error)}`, true); } finally { input.value = ''; } };
document.addEventListener('keydown', event => { if (($('#modal') as HTMLDialogElement).open || ($('#navigator') as HTMLDialogElement).open || (event.target as HTMLElement).closest('input,textarea,select,[contenteditable]')) return; if ((event.ctrlKey || event.metaKey) && event.key.toLowerCase() === 'z') { event.preventDefault(); event.shiftKey ? redo() : undo(); } else if (event.key === '/') { event.preventDefault(); openIndex(); } else if (event.key.toLowerCase() === 'c') conceptDialog(); else if (event.key.toLowerCase() === 'f') scene.overview(); else if (event.key === 'Escape') { closeWorkspaceMenu(); closeCreationFan(); if (scopeId || aggregateId) ascend(); else { mode = 'domain'; select(null); } } });
scene = createScene($('#scene'), { select: id => { if (id) mode = 'domain'; select(id); }, enter, move: (id, position) => { const next = cloneProject(project); const c = next.concepts.find(c => c.id === id); if (c) { c.position = position; commit(next); } } });
if (!startupNotice) persist();
render();
if (startupNotice) {
  notify(startupNotice, true);
  const raw = recoveryRaw;
  if (raw) showDialog('Recover saved workspace', '<p>Your previous save could not be loaded. Download the original data before making edits so it can be recovered.</p>', 'Download saved data', () => download('domain-studio-recovery.json', raw));
}
window.addEventListener('beforeunload', () => scene.dispose());
