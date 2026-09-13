import { parse } from 'yaml';
import type { Baseline, DomainProject, StudioMode } from './contracts';
import { importProject } from './serialization';
import { conceptContracts, kindNames, planPosition, record, relationshipDescription } from './model-evidence';
import { loadRepository, queryRepositoryContext, queryRepositoryZone, checkRepository, type RepositoryProject, type RepositoryContextReport, type RepositoryCheckResult } from './repository';
import './workbench.css';

const escape = (value: unknown) => String(value ?? '').replace(/[&<>"']/g, c => ({ '&': '&amp;', '<': '&lt;', '>': '&gt;', '"': '&quot;', "'": '&#39;' })[c]!);
type Representation = 'site' | 'plan' | 'matrix';
interface Callbacks { select(id: string): void; replace(project: DomainProject): void; create(): void; notify(message: string, error?: boolean): void }
export interface Workbench { update(project: DomainProject, selectedId: string | null, scopeId: string | null, mode: StudioMode, baseline: Baseline | null): void; selectionEvidence(id: string): string; bindSelection(): void }

export function createWorkbench(host: HTMLElement, callbacks: Callbacks): Workbench {
  let project: DomainProject;
  let selected: string | null = null, scope: string | null = null;
  let mode: StudioMode = 'domain', baseline: Baseline | null = null;
  let representation: Representation = 'site';
  let matrixRowPage = 0, matrixColumnPage = 0;
  const matrixPageSize = 32;
  let repository: RepositoryProject | null = null;
  let run: RepositoryCheckResult | null = null;
  let requestGeneration = 0;
  let busy = false;
  host.insertAdjacentHTML('beforeend', `
    <div class="workbench" aria-label="Architecture workbench">
      <form id="workbench-form"><label class="sr-only" for="workbench-input">Find a concept or inspect a code path</label><span aria-hidden="true">⌕</span><input id="workbench-input" placeholder="A concept name or a code path…" autocomplete="off" /><button id="workbench-submit" type="submit">Inspect ↗</button></form>
      <div class="drawing-controls"><div role="group" aria-label="Representation"><button id="view-site" aria-pressed="true">Site</button><button id="view-plan" aria-pressed="false">Plan</button><button id="view-matrix" aria-pressed="false">Matrix</button></div><button id="open-architecture">Zones & layers</button><button id="open-repository">Open repository</button><button id="run-repository-check">Check code</button></div>
      <div class="purpose-line"><button id="edit-purpose">Set the user and need for this model ↗</button><span id="connection-state">Model draft · code not inspected</span></div>
      <form id="purpose-form" hidden><label>User<input name="actor" required placeholder="Who needs this?" maxlength="100" /></label><label>Need<input name="need" required placeholder="What must they be able to do?" maxlength="240" /></label><button type="submit">Set anchor</button><button type="button" id="close-purpose">Cancel</button></form>
    </div>
    <section id="projection" aria-label="Alternative model representation" hidden></section>
    <section id="evidence-card" aria-label="ArcLint evidence" hidden></section>
    <details class="drawing-key" aria-label="Map legend"><summary>READ THE DRAWING</summary><span>□ Unclassified · named foundation</span><span>▣ Aggregate · consistency boundary</span><span>◇ Value · identity-free object</span><span>╳ Invariant · must hold</span><span>⊣ Assertion · after an operation</span><span>→ Relationship · follow the named arrow</span></details>
    <button id="onyx-helper" aria-label="Ask Onyx for the next step"><svg viewBox="0 0 48 44" aria-hidden="true"><path d="M12 8 5 5 4 27 14 25M36 8 43 5 44 27 34 25" fill="#202a29"/><path d="M12 8Q24 0 36 8L35 29Q24 44 13 29Z" fill="#34413d"/><path d="m15 21 9 8 9-8-5 14h-8Z" fill="#64766b"/><circle cx="17" cy="17" r="2" fill="#eae4ca"/><circle cx="31" cy="17" r="2" fill="#eae4ca"/><path d="m20 25 4 5 4-5Z" fill="#14221f"/></svg><span><b>Onyx</b><small id="onyx-hint">Start with a need</small></span></button>
  `);
  const $ = <T extends HTMLElement = HTMLElement>(selector: string) => host.querySelector<T>(selector)!;
  const card = $('#evidence-card');
  host.addEventListener('keydown', event => { if (event.key === 'Escape' && !card.hidden) { requestGeneration++; card.hidden = true; event.stopPropagation(); } });
  ($('#workbench-form') as HTMLFormElement).autocomplete = 'off';
  ($('.drawing-key') as HTMLDetailsElement).open = window.innerWidth > 1100;
  function evidence(title: string, html: string) {
    card.hidden = false;
    card.innerHTML = `<div class="evidence-heading"><div><span>ARCLINT · EVIDENCE</span><h2>${escape(title)}</h2></div><button id="close-evidence" aria-label="Close evidence">×</button></div><div class="evidence-body">${html}</div>`;
    $('#close-evidence').onclick = () => { requestGeneration++; card.hidden = true; };
    card.querySelectorAll<HTMLElement>('[data-inspect-path]').forEach(b => b.onclick = () => void inspect(b.dataset.inspectPath!));
    card.querySelectorAll<HTMLElement>('[data-inspect-zone]').forEach(b => b.onclick = () => void inspect(b.dataset.inspectZone!, true));
  }
  const error = (failure: unknown) => escape(failure instanceof Error ? failure.message : String(failure));
  async function ensureRepository() { if (!repository) repository = await loadRepository(); return repository; }
  function status() {
    $('#connection-state').textContent = repository ? `${repository.repository.name} · on-disk evidence${run ? ` · checked ${new Date(run.checkedAt).toLocaleTimeString()}` : ' · not checked'}` : 'Model draft · code not inspected';
    $('#onyx-hint').textContent = !project?.contexts.length ? 'Lay out a context' : selected ? 'Inspect this selection' : run ? `${run.diagnostics.length} reported records` : 'Model → rules → code';
  }
  function ruleRows(report: RepositoryContextReport) {
    return (report.Rules ?? []).map(({ Summary: r, Reason }) => `<details class="evidence-rule"><summary><span>${escape(r.ID)}</span><em>${escape(r.Disabled ? 'disabled' : r.Severity)}</em></summary><p>${escape(r.Proposition)}</p>${r.Rationale ? `<p><b>Rationale.</b> ${escape(r.Rationale)}</p>` : ''}<p class="evidence-meta">${escape(Reason)}${r.Assurance ? ` · Assurance: ${escape(r.Assurance)}` : ''}</p>${r.DisabledReason ? `<p>Disabled: ${escape(r.DisabledReason)}</p>` : ''}</details>`).join('') || '<p>No governing Rules returned for this path.</p>';
  }
  function contextBody(path: string, report: RepositoryContextReport) {
    return `<p class="evidence-command">arclint context ${escape(path)}</p><p class="evidence-meta">Bound repository: ${escape(repository?.repository.root ?? 'local repository')}. This describes code on disk.</p>
      <h3>Where it belongs</h3>${(report.Zones ?? []).map(z => `<article class="zone-record"><b>${escape(z.Name)}</b><p>${escape(z.Description)}</p><code>${escape((z.Paths ?? []).join(', '))}</code><p>May import: ${z.InternalRestricted ? escape(z.Internal?.join(', ') || 'no other declared Zone') : 'not restricted by this Zone’s import contract'}. External: ${escape(z.External)}.</p></article>`).join('') || '<p>No declared Zone owns this path.</p>'}
      <h3>What governs it</h3><p class="evidence-meta">Import permissions describe code dependencies. They do not grant people permission to change files.</p>${ruleRows(report)}
      <h3>Recorded contract anchors</h3>${(report.domain?.contexts ?? []).flatMap(c => [...c.invariants ?? [], ...c.assertions ?? []].map(contract => `<article class="contract-record"><b>${escape(contract.owner)} · ${escape(contract.key)}</b><p>${escape(contract.statement)}</p><small>${escape(contract.source ?? contract.reason ?? 'No source anchor found')} · ${escape(contract.anchor ?? 'unknown')}</small></article>`)).join('') || '<p>No contract anchors returned for this path.</p>'}`;
  }
  async function inspect(path: string, zone = false) {
    const ticket = ++requestGeneration;
    evidence('Inspecting code', `<p>Reading <code>${escape(path)}</code> from the bound repository…</p>`);
    try {
      const [result] = await Promise.all([zone ? queryRepositoryZone(path) : queryRepositoryContext(path), ensureRepository()]);
      if (ticket !== requestGeneration) return;
      evidence(zone ? `Zone: ${path}` : path, contextBody(result.path, result.report)); status();
    } catch (failure) { if (ticket === requestGeneration) evidence('Code inspection unavailable', `<p role="alert">${error(failure)}</p>`); }
  }
  async function architecture() {
    const ticket = ++requestGeneration;
    evidence('Zones & layers', '<p>Reading the repository’s declared contracts…</p>');
    try {
      const repo = await ensureRepository(); if (ticket !== requestGeneration) return;
      const yaml = record(parse(repo.rulesYaml ?? ''));
      const layerRules = Object.entries(record(yaml.rules)).flatMap(([id, raw]) => {
        const rule = record(raw);
        return Array.isArray(rule.layers) && rule.layers.every(item => typeof item === 'string') ? [{ id, layers: rule.layers as string[] }] : [];
      });
      evidence('Zones & layers', `<p>Repository: <b>${escape(repo.repository.name)}</b>. Zones group paths; bounded contexts define where a model applies. A Zone may overlap other Zones.</p><h3>Declared dependency layers</h3><p class="evidence-meta">Highest layer first. A lower layer may not import a higher one. Only explicit <code>layers</code> constraints determine this order.</p>${layerRules.map(rule => `<article class="layer-section"><code>${escape(rule.id)}</code><ol class="layer-stack">${rule.layers.map((zone, index) => `<li><span>${index === 0 ? 'HIGHER' : 'LOWER'}</span><b>${escape(zone)}</b><small>${index ? 'Cannot import a layer above' : 'Dependencies may point down'}</small></li>`).join('')}</ol></article>`).join('') || '<p>No local layer order is declared. No height is inferred from a Zone’s name.</p>'}<h3>Declared Zones</h3>${(repo.context.Zones ?? []).map(zone => `<article class="zone-record"><b>${escape(zone.Name)}</b><p>${escape(zone.Description)}</p>${(zone.Paths ?? []).map(path => `<code>${escape(path)}</code>`).join('')}<button data-inspect-zone="${escape(zone.Name)}" class="path-button">Inspect Zone ${escape(zone.Name)} ↗</button><small>May import: ${zone.InternalRestricted ? escape(zone.Internal?.join(', ') || 'no other Zone') : 'not restricted by this Zone’s import contract'}</small></article>`).join('')}<h3>Repository-wide Rules</h3>${ruleRows(repo.context)}`); status();
    } catch (failure) { if (ticket === requestGeneration) evidence('Architecture unavailable', `<p role="alert">${error(failure)}</p>`); }
  }
  async function checkCode() {
    if (busy) return;
    busy = true; $('#run-repository-check').setAttribute('disabled', '');
    const ticket = ++requestGeneration;
    evidence('Checking repository', '<p>ArcLint is observing the bound repository and evaluating its configured Rules.</p><p>Unsaved browser model edits are not part of this check.</p>');
    try {
      await ensureRepository(); run = await checkRepository(); status();
      if (ticket !== requestGeneration) return;
      evidence('Repository check', `<p class="evidence-command">arclint check --format json</p><p><b>${run.diagnostics.length} reported records</b> · exit ${run.exitCode} · ${escape(new Date(run.checkedAt).toLocaleString())}</p><p>Checked code on disk in <code>${escape(repository!.repository.root)}</code>. Browser draft changes were not written there.</p>${!run.outcomesAvailable ? '<p class="evidence-meta">This CLI returns diagnostics, without a complete per-Rule outcome table. An empty list does not prove every Rule passed.</p>' : ''}${run.diagnostics.map(d => `<article class="diagnostic-record"><div><b>${escape(d.ruleId ?? d.kind)}</b><span>${escape(d.status ?? d.severity ?? d.kind)}</span></div><p>${escape(d.message)}</p>${d.path ? `<button class="path-button" data-inspect-path="${escape(d.path)}">${escape(d.path)}${d.line ? `:${d.line}` : ''} ↗</button>` : ''}${d.remediation ? `<p>${escape(d.remediation)}</p>` : ''}</article>`).join('') || '<p>No diagnostic records were emitted.</p>'}${run.stderr ? `<details><summary>CLI output</summary><pre>${escape(run.stderr)}</pre></details>` : ''}`);
    } catch (failure) { if (ticket === requestGeneration) evidence('Check could not complete', `<p role="alert">${error(failure)}</p>`); }
    finally { busy = false; $('#run-repository-check').removeAttribute('disabled'); }
  }
  $('#open-repository').onclick = async () => {
    if (busy) return;
    busy = true; $('#open-repository').setAttribute('disabled', '');
    try {
      const loaded = await loadRepository();
      if (!loaded.domainYaml) throw new Error('This repository has no domain.arclint.yaml. Create a model here or import a domain file.');
      const next = importProject(loaded.domainYaml);
      repository = loaded; run = null; callbacks.replace(next); status();
      callbacks.notify(`Loaded ${loaded.repository.name} from disk. Edits remain a browser draft; Undo restores the previous model.`);
    } catch (failure) { evidence('Repository could not be opened', `<p role="alert">${error(failure)}</p>`); }
    finally { busy = false; $('#open-repository').removeAttribute('disabled'); }
  };
  $('#open-architecture').onclick = () => void architecture();
  $('#run-repository-check').onclick = () => void checkCode();
  $('#workbench-form').onsubmit = event => {
    event.preventDefault();
    const value = $('#workbench-input') as HTMLInputElement;
    const text = value.value.trim(); if (!text) return;
    const matches = [...project.contexts, ...project.concepts].filter(item => item.name.toLocaleLowerCase() === text.toLocaleLowerCase());
    if (matches.length === 1) { card.hidden = true; callbacks.select(matches[0].id); return; }
    if (matches.length > 1) {
      evidence('Choose the context', matches.map(item => `<button class="path-button" data-match="${escape(item.id)}">${escape(item.name)} · ${escape('contextId' in item ? project.contexts.find(c => c.id === item.contextId)?.name : 'context')}</button>`).join(''));
      card.querySelectorAll<HTMLElement>('[data-match]').forEach(b => b.onclick = () => { card.hidden = true; callbacks.select(b.dataset.match!); }); return;
    }
    if (text.includes('/') || /\.[a-z\d]+$/i.test(text)) { void inspect(text); return; }
    evidence('No matching concept', `<p>No recorded concept is named <b>${escape(text)}</b>. Use an existing name, enter a repository path, or add a concept.</p><button id="create-from-workbench">Add concept</button>`);
    $('#create-from-workbench').onclick = () => { card.hidden = true; callbacks.create(); };
  };
  function setRepresentation(next: Representation) {
    representation = next; host.dataset.representation = next;
    for (const item of ['site', 'plan', 'matrix']) $(`#view-${item}`).setAttribute('aria-pressed', String(item === next));
    $('#projection').hidden = next === 'site'; renderDrawing();
  }
  $('#view-site').onclick = () => setRepresentation('site'); $('#view-plan').onclick = () => setRepresentation('plan'); $('#view-matrix').onclick = () => setRepresentation('matrix');
  function renderDrawing() {
    if (!project || representation === 'site') return;
    const objects = [...project.contexts.filter(c => !scope || c.id === scope), ...project.concepts.filter(c => !scope || c.contextId === scope)];
    const ids = new Set(objects.map(item => item.id));
    const relations = project.relationships.filter(r => ids.has(r.source) && ids.has(r.target));
    const name = (id: string) => objects.find(item => item.id === id)?.name ?? id;
    const panel = $('#projection');
    const scroll = { x: panel.scrollLeft, y: panel.scrollTop };
    if (representation === 'matrix') {
      const pageCount = Math.max(1, Math.ceil(objects.length / matrixPageSize));
      matrixRowPage = Math.min(matrixRowPage, pageCount - 1); matrixColumnPage = Math.min(matrixColumnPage, pageCount - 1);
      const rows = objects.slice(matrixRowPage * matrixPageSize, (matrixRowPage + 1) * matrixPageSize);
      const columns = objects.slice(matrixColumnPage * matrixPageSize, (matrixColumnPage + 1) * matrixPageSize);
      const cells = new Map<string, typeof relations>();
      const add = (source: string, target: string, relation: typeof relations[number]) => { const key=JSON.stringify([source,target]); cells.set(key,[...cells.get(key) ?? [],relation]); };
      for (const relation of relations) { add(relation.source,relation.target,relation); if (relationshipDescription(project,relation).direction === 'both') add(relation.target,relation.source,relation); }
      const pager = (axis: 'row'|'column', page: number) => `<div><button data-matrix-page="${axis}" data-step="-1" ${page ? '' : 'disabled'} aria-label="Previous ${axis}s">←</button><span>${axis === 'row' ? 'Rows' : 'Columns'} ${objects.length ? page*matrixPageSize+1 : 0}–${Math.min(objects.length,(page+1)*matrixPageSize)} of ${objects.length}</span><button data-matrix-page="${axis}" data-step="1" ${page+1 < pageCount ? '' : 'disabled'} aria-label="Next ${axis}s">→</button></div>`;
      panel.innerHTML = `<div class="matrix-drawing"><div class="drawing-heading"><h2>Relationship matrix</h2><p>Read from the row to the column. Each cell names the recorded relationship; blank means none recorded.</p><div class="matrix-pagination">${pager('row',matrixRowPage)}${pager('column',matrixColumnPage)}</div></div><table><caption>Same model · ${objects.length} objects · ${relations.length} relationships</caption><thead><tr><th scope="col">FROM ↓ / TO →</th>${columns.map(object => `<th scope="col"><button data-model-select="${escape(object.id)}">${escape(object.name)}</button></th>`).join('')}</tr></thead><tbody>${rows.map(source => `<tr><th scope="row"><button data-model-select="${escape(source.id)}">${escape(source.name)}</button></th>${columns.map(target => `<td ${source.id === target.id ? 'class="matrix-self"' : ''}>${(cells.get(JSON.stringify([source.id,target.id])) ?? []).map(r => `<button data-matrix-relation="${escape(r.id)}" aria-label="${escape(`${name(r.source)} ${r.label} ${name(r.target)}`)}">${escape(relationshipDescription(project, r).label)}</button>`).join('')}</td>`).join('')}</tr>`).join('')}</tbody></table></div>`;
    } else {
      const coordinates = new Map(objects.map(object => [object.id, planPosition(object.position)]));
      const contexts = project.contexts.filter(c => ids.has(c.id));
      const bounds = new Map(contexts.map(context => {
        const members = project.concepts.filter(c => c.contextId === context.id && ids.has(c.id)).map(c => coordinates.get(c.id)!);
        const anchor = coordinates.get(context.id)!;
        const xs = [anchor.x, ...members.map(p => p.x)], ys = [anchor.y, ...members.map(p => p.y)];
        return [context.id, { x: Math.min(...xs) - 115, y: Math.min(...ys) - 130, right: Math.max(...xs) + 115, bottom: Math.max(...ys) + 100 }];
      }));
      const minX = Math.min(0, ...[...bounds.values()].map(b => b.x)) - 40, minY = Math.min(0, ...[...bounds.values()].map(b => b.y)) - 40;
      const width = Math.max(1050, Math.max(0, ...[...bounds.values()].map(b => b.right)) - minX + 50), height = Math.max(700, Math.max(0, ...[...bounds.values()].map(b => b.bottom)) - minY + 50);
      const point = (id: string) => { const p = coordinates.get(id)!; const ctx = bounds.get(id); return ctx ? { x: (ctx.x + ctx.right) / 2 - minX, y: ctx.y - minY + 30 } : { x: p.x - minX, y: p.y - minY }; };
      const clipped = (r: typeof relations[number]) => { const a = point(r.source), b = point(r.target); const dx = b.x - a.x, dy = b.y - a.y; const inset = Math.min(.4, 1 / Math.max(Math.abs(dx) / 88, Math.abs(dy) / 55, 1)); return { a: { x: a.x + dx * inset, y: a.y + dy * inset }, b: { x: b.x - dx * inset, y: b.y - dy * inset } }; };
      const conceptObjects = objects.filter((item): item is DomainProject['concepts'][number] => 'kind' in item);
      const occupied = conceptObjects.map(c => { const p = point(c.id); return { x:p.x - 83, y:p.y - 55, width:166, height:110 }; });
      const edgeLabels = relations.map(r => {
        const a = point(r.source), b = point(r.target); let x = (a.x + b.x) / 2, y = (a.y + b.y) / 2;
        const collides = () => occupied.some(rect => x + 100 > rect.x && x - 100 < rect.x + rect.width && y + 16 > rect.y && y - 16 < rect.y + rect.height);
        for (let attempts = 0; attempts < 40 && collides(); attempts++) y += 35;
        occupied.push({x:x-100,y:y-16,width:200,height:32}); return {r,x,y};
      });
      panel.innerHTML = `<div class="drawing-heading"><h2>Model plan</h2><p>Scroll to explore · saved positions · select any object to edit its meaning. Position records arrangement, not evolutionary maturity.</p></div><div class="plan-drawing" style="width:${width}px;height:${Math.max(height,...edgeLabels.map(p=>p.y+60))}px">${contexts.map(c => { const box=bounds.get(c.id)!, p=point(c.id); return `<div class="plan-context-boundary" style="left:${box.x-minX}px;top:${box.y-minY}px;width:${box.right-box.x}px;height:${box.bottom-box.y}px"></div><button class="plan-object context ${selected === c.id ? 'is-selected' : ''}" data-model-select="${escape(c.id)}" style="left:${p.x}px;top:${p.y}px"><small>Bounded context</small><b>${escape(c.name)}</b></button>`; }).join('')}<svg class="plan-lines" width="${width}" height="${Math.max(height,...edgeLabels.map(p=>p.y+60))}" aria-hidden="true"><defs><marker id="plan-arrow" orient="auto-start-reverse" markerWidth="8" markerHeight="8" refX="7" refY="4"><path d="M0 0 8 4 0 8" fill="#526863"/></marker></defs>${relations.map(r => { const {a,b}=clipped(r); const direction=relationshipDescription(project,r).direction; return `<path d="M${a.x} ${a.y} L${b.x} ${b.y}" ${direction === 'none' ? 'stroke-dasharray="3 8"' : 'marker-end="url(#plan-arrow)"'} ${direction === 'both' ? 'marker-start="url(#plan-arrow)"' : ''}/>`; }).join('')}${edgeLabels.map(({r,x,y})=>{ const a=point(r.source),b=point(r.target); return `<path class="plan-leader" d="M${(a.x+b.x)/2} ${(a.y+b.y)/2} L${x} ${y}"/>`; }).join('')}${mode === 'baseline' && baseline ? conceptObjects.flatMap(c => { const old = baseline!.project.concepts.find(o => o.id === c.id); if (!old || JSON.stringify(old.position) === JSON.stringify(c.position)) return []; const a = planPosition(old.position), b = point(c.id); return [`<path class="movement-trace" d="M${a.x - minX} ${a.y - minY} L${b.x} ${b.y}" marker-end="url(#plan-arrow)"/>`]; }).join('') : ''}</svg>${conceptObjects.map(object => { const p=point(object.id), contracts=conceptContracts(project,object); return `<button class="plan-object ${object.kind} ${selected === object.id ? 'is-selected' : ''}" data-model-select="${escape(object.id)}" style="left:${p.x}px;top:${p.y}px"><small>${escape(kindNames[object.kind])}</small><b>${escape(object.name)}</b>${contracts.invariants.length || contracts.assertions.length ? `<span>╳ ${contracts.invariants.length} invariants · ⊣ ${contracts.assertions.length} assertions</span>` : ''}</button>`; }).join('')}${edgeLabels.map(({r,x,y}) => { const info=relationshipDescription(project,r); return `<button class="plan-relation" data-plan-relation="${escape(r.id)}" style="left:${x}px;top:${y}px" aria-label="${escape(`${name(r.source)} ${r.label} ${name(r.target)}`)}" title="${escape(info.meaning)}">${escape(info.label)} ${info.direction === 'both' ? '↔' : info.direction === 'none' ? '∥' : '→'}</button>`; }).join('')}</div>`;
    }
    panel.scrollLeft = scroll.x; panel.scrollTop = scroll.y;
    panel.querySelectorAll<HTMLElement>('[data-matrix-page]').forEach(b => b.onclick = () => { const step=Number(b.dataset.step); if (b.dataset.matrixPage === 'row') matrixRowPage += step; else matrixColumnPage += step; renderDrawing(); });
    panel.querySelectorAll<HTMLElement>('[data-model-select]').forEach(b => b.onclick = () => callbacks.select(b.dataset.modelSelect!));
    panel.querySelectorAll<HTMLElement>('[data-plan-relation],[data-matrix-relation]').forEach(b => b.onclick = () => callbacks.select(b.dataset.planRelation ?? b.dataset.matrixRelation!));
  }
  const briefKey = () => `arclint.studio.purpose:${project.name}`;
  function purpose() {
    try { const saved = JSON.parse(localStorage.getItem(briefKey()) ?? 'null'); $('#edit-purpose').textContent = saved?.actor && saved?.need ? `${saved.actor} → ${saved.need}` : 'Set the user and need for this model ↗'; }
    catch { $('#edit-purpose').textContent = 'Set the user and need for this model ↗'; }
  }
  $('#edit-purpose').onclick = () => { $('#purpose-form').hidden = !$('#purpose-form').hidden; if (!$('#purpose-form').hidden) $('#purpose-form input').focus(); };
  $('#close-purpose').onclick = () => { $('#purpose-form').hidden = true; };
  $('#purpose-form').onsubmit = event => { event.preventDefault(); const form = $('#purpose-form') as HTMLFormElement; const data = new FormData(form); try { localStorage.setItem(briefKey(), JSON.stringify({ actor: String(data.get('actor')).trim(), need: String(data.get('need')).trim() })); form.hidden = true; purpose(); } catch { callbacks.notify('Could not save the model’s user and need.', true); } };
  $('#onyx-helper').onclick = () => {
    const c = project.concepts.find(item => item.id === selected);
    evidence('Onyx · next step', `<p>${c ? `You selected <b>${escape(c.name)}</b>, ${escape(kindNames[c.kind].toLowerCase())} in ${escape(project.contexts.find(ctx => ctx.id === c.contextId)?.name)}.` : `This model records ${project.contexts.length} contexts and ${project.concepts.length} concepts.`}</p><ol class="inspection-steps"><li><b>Record the need.</b> Name the user and what the model must support.</li><li><b>Define the boundary.</b> Name the concepts, their identities, and the invariants the root must protect.</li><li><b>Inspect the governing Rules.</b> Use a real path to see its Zones, constraints, and Rationale.</li><li><b>Check the code.</b> Run ArcLint, then inspect each reported finding at its path.</li></ol><p class="evidence-meta">Onyx is a local guide. Recommendations come from the current selection and recorded evidence.</p><div class="guide-actions"><button id="onyx-zones">Inspect Zones</button><button id="onyx-check">Check repository</button></div>`);
    $('#onyx-zones').onclick = () => void architecture(); $('#onyx-check').onclick = () => void checkCode();
  };
  function selectionEvidence(id: string): string {
    const concept = project?.concepts.find(c => c.id === id);
    if (!concept) return '';
    const contracts = conceptContracts(project, concept);
    const contextName = project.contexts.find(c => c.id === concept.contextId)?.name;
    const sameSource = repository?.domainYaml && project.sourceDocument && JSON.stringify(parse(repository.domainYaml)) === JSON.stringify(project.sourceDocument);
    const anchors = sameSource ? (repository!.context.domain?.contexts ?? []).filter(c => c.name === contextName).flatMap(c => [...c.invariants ?? [], ...c.assertions ?? []]).filter(item => item.owner === concept.name && item.ownerConcept === concept.kind && item.source) : [];
    return `<section class="recorded-contracts"><h3>Recorded contracts</h3><p class="field-help">These state what must hold. Code inspection supplies separate evidence.</p><details ${contracts.invariants.length ? 'open' : ''}><summary>Invariants · ${contracts.invariants.length}</summary>${contracts.invariants.map(i => `<article><b>╳ ${escape(i.key)}</b><p>${escape(i.statement)}</p></article>`).join('') || '<p>No invariants recorded.</p>'}</details><details ${contracts.assertions.length ? 'open' : ''}><summary>Assertions · ${contracts.assertions.length}</summary>${contracts.assertions.map(a => `<article><b>⊣ After ${escape(a.operation)}</b><code>${escape(a.key)}</code><p>${escape(a.statement)}</p></article>`).join('') || '<p>No operation post-conditions recorded.</p>'}</details><h3>Code evidence</h3>${anchors.map(a => `<button class="path-button" data-inspect-path="${escape(a.source!.replace(/:\d+$/, ''))}">Contract anchor: ${escape(a.source)} ↗</button>`).join('') || '<p class="field-help">No located contract anchor for this concept. Enter a repository path above to inspect its governing Rules.</p>'}</section>`;
  }
  function bindSelection() { document.querySelectorAll<HTMLElement>('#inspector [data-inspect-path]').forEach(b => b.onclick = () => void inspect(b.dataset.inspectPath!)); }
  return { update(next, id, contextId, nextMode, pointOfReference) { project = next; selected = id; scope = contextId; mode = nextMode; baseline = pointOfReference; purpose(); status(); renderDrawing(); }, selectionEvidence, bindSelection };
}
