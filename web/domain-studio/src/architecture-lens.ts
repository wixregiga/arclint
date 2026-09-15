import type { DomainProject, SceneVisibility } from './contracts';
import type { ViewLens } from './view-state';
import { conceptContracts } from './model-evidence';
import { escapeHtml as esc } from './presentation';
import { loadRepository, queryRepositoryDependencies, type RepositoryProject, type RepositoryCheckResult } from './repository';
import { buildArchitectureEvidence, loadArchitecturePaths, architectureImportsForSelection, type ArchitectureEvidence, type ArchitecturePathResult, type ArchitectureDependencyInput } from './architecture-evidence';
import './architecture-lens.css';

export interface ArchitectureView {
  project: DomainProject; selectedId: string | null; contextId: string | null;
  lens: ViewLens; editorOpen: boolean; selectedLayerRule: string | null;
  visibility?: SceneVisibility; hiddenLayerZones?: string[]; layerSpread?: number;
  checking?: boolean; checkError?: string; watchingNote?: string;
}
interface Callbacks {
  changed(): void; layer(id: string | null): void;
  visibility(key: keyof SceneVisibility, enabled: boolean): void;
  layerZone(zone: string, visible: boolean): void; spread(value: number): void;
  loaded?(repository: RepositoryProject, report: RepositoryCheckResult | null): void;
  inspectPath(path: string): void; inspectZone(zone: string): void;
  inspectRule(id: string): void; report(): void; check(): void; locate(): void;
}
export interface ArchitectureLens {
  update(view: ArchitectureView): void;
  readonly evidence: ArchitectureEvidence | undefined;
  refresh(): Promise<void>;
  receive(repository: RepositoryProject | null, report: RepositoryCheckResult | null): void;
  openContract(subjectId: string, key: string, kind: 'invariant' | 'assertion'): void;
}

/** Read-only, selection-relative architecture. Late evidence never changes model or navigation. */
export function createArchitectureLens(host: HTMLElement, callbacks: Callbacks): ArchitectureLens {
  let view: ArchitectureView | undefined;
  let repository: RepositoryProject | null = null, report: RepositoryCheckResult | null = null;
  let paths: ArchitecturePathResult[] = [], projection: ArchitectureEvidence | undefined;
  let dependencies: ArchitectureDependencyInput = null;
  let failure = '', generation = 0;
  let abort: AbortController | undefined;
  let readingExpanded = false, readingKey = '', importPage = 0, readingContents = '', renderedZoneOptions = '';
  let contract: {subjectId: string; key: string; kind: 'invariant' | 'assertion'} | undefined;
  host.insertAdjacentHTML('beforeend', `<div id="architecture-tools"><div class="architecture-visibility" role="group" aria-label="Show on the map"><button id="zone-overlay" data-visibility="layers" aria-pressed="false">Show Layers</button><button id="dependency-overlay" data-visibility="dependencies" aria-pressed="true">Hide Dependencies</button><button id="description-overlay" data-visibility="description" aria-pressed="true">Hide Description</button></div><button id="architecture-open-reading" aria-expanded="false" aria-controls="architecture-reading">Details ↗</button><div id="architecture-layer-controls" hidden><label for="architecture-layer" class="sr-only">Layer Rule</label><select id="architecture-layer" aria-label="Layer Rule"><option value="">No declared layer order</option></select><details id="architecture-layer-options"><summary>Adjust layers</summary><div class="architecture-layer-options-body"><div id="architecture-layer-zones"></div><label class="architecture-spread" for="architecture-layer-spread">Spread<input id="architecture-layer-spread" type="range" min="0" max="1" step="0.05" value="0.65" /></label></div></details><button id="architecture-read-layer">Read Rule ↗</button></div><span id="architecture-state" role="status" hidden></span></div><section id="architecture-reading" aria-label="Selected architecture" hidden></section>`);
  const tools = host.querySelector<HTMLElement>('#architecture-tools')!;
  const visibility = (): SceneVisibility => view?.visibility ?? { layers: false, dependencies: true, description: true };
  tools.querySelectorAll<HTMLButtonElement>('[data-visibility]').forEach(button => button.onclick = () => {
    const key = button.dataset.visibility as keyof SceneVisibility;
    callbacks.visibility(key, !visibility()[key]);
  });
  const details = host.querySelector<HTMLButtonElement>('#architecture-open-reading')!;
  details.onclick = () => { readingExpanded = !readingExpanded; draw(); callbacks.changed(); };
  const reading = host.querySelector<HTMLElement>('#architecture-reading')!;
  const layer = host.querySelector<HTMLSelectElement>('#architecture-layer')!;
  const layerControls = host.querySelector<HTMLElement>('#architecture-layer-controls')!;
  const layerZones = host.querySelector<HTMLElement>('#architecture-layer-zones')!;
  const spread = host.querySelector<HTMLInputElement>('#architecture-layer-spread')!;
  const state = host.querySelector<HTMLElement>('#architecture-state')!;
  layer.onchange = () => callbacks.layer(layer.value || null);
  spread.oninput = () => callbacks.spread(Number(spread.value));
  host.querySelector<HTMLButtonElement>('#architecture-read-layer')!.onclick = () => { if (layer.value) callbacks.inspectRule(layer.value); };
  const controls = host.querySelector<HTMLElement>('.focus-controls');
  const orientation = host.querySelector<HTMLElement>('.place-orientation');
  function reserveControls() {
    if (controls) host.style.setProperty('--reading-bottom', `${Math.max(0, host.getBoundingClientRect().bottom - controls.getBoundingClientRect().top) + 12}px`);
    if (orientation) host.style.setProperty('--architecture-top', `${Math.ceil(orientation.getBoundingClientRect().bottom - host.getBoundingClientRect().top + 12)}px`);
  }
  const layout = new ResizeObserver(reserveControls);
  layout.observe(host);
  if (controls) layout.observe(controls);
  if (orientation) layout.observe(orientation);
  function writeReading(contents: string) {
    if (readingContents === contents) return false;
    readingContents = contents;
    const priorSections = new Map([...reading.querySelectorAll<HTMLDetailsElement>('.architecture-section')].map(section => [section.querySelector('summary')?.textContent?.split(' · ')[0], section.open]));
    const scrollTop = reading.scrollTop;
    reading.innerHTML = `<button class="architecture-reading-toggle" aria-controls="architecture-reading-body" aria-expanded="${readingExpanded}">Close details ×</button><div id="architecture-reading-body">${contents}</div>`;
    for (const section of reading.querySelectorAll<HTMLDetailsElement>('.architecture-section')) {
      const name = section.querySelector('summary')?.textContent?.split(' · ')[0];
      if (priorSections.has(name)) section.open = priorSections.get(name)!;
    }
    reading.scrollTop = scrollTop;
    reading.dataset.expanded = String(readingExpanded);
    reading.querySelector<HTMLButtonElement>('.architecture-reading-toggle')!.onclick = event => {
      readingExpanded = !readingExpanded;
      reading.dataset.expanded = String(readingExpanded);
      const toggle = event.currentTarget as HTMLButtonElement;
      toggle.setAttribute('aria-expanded', String(readingExpanded));
      toggle.textContent = 'Close details ×';
      draw(); callbacks.changed();
    };
    requestAnimationFrame(reserveControls);
    return true;
  }

  function draw() {
    if (!view) return;
    projection = buildArchitectureEvidence(view.project, repository, paths, report, dependencies);
    const {project, selectedId, contextId} = view;
    const shown = visibility();
    const nextReadingKey = `${selectedId ?? contextId ?? 'world'}`;
    if (readingKey !== nextReadingKey) { readingExpanded = false; readingKey = nextReadingKey; importPage = 0; }
    tools.hidden = view.editorOpen;
    tools.querySelectorAll<HTMLButtonElement>('[data-visibility]').forEach(button => {
      const key = button.dataset.visibility as keyof SceneVisibility;
      button.setAttribute('aria-pressed', String(shown[key]));
      button.textContent = `${shown[key] ? 'Hide' : 'Show'} ${key[0].toUpperCase()}${key.slice(1)}`;
    });
    layerControls.hidden = !shown.layers;
    details.setAttribute('aria-expanded',String(readingExpanded));
    const chosenRule = projection.layers.find(rule => rule.id === view!.selectedLayerRule) ?? projection.layers[0];
    const options = projection.layers.map(rule => `<option value="${esc(rule.id)}">${esc(rule.id)}${rule.disabled ? ' · disabled' : ''}</option>`).join('') || `<option value="">${projection.unavailableLayerRuleIds.length ? 'Layer order unavailable' : 'No layers Rule configured'}</option>`;
    if (layer.innerHTML !== options) layer.innerHTML = options;
    layer.value = chosenRule?.id ?? '';
    layer.disabled = !chosenRule;
    host.querySelector<HTMLElement>('#architecture-layer-options')!.hidden = !chosenRule;
    host.querySelector<HTMLElement>('#architecture-read-layer')!.hidden = !chosenRule;
    const hiddenZones = view.hiddenLayerZones ?? [];
    const zoneOptions = chosenRule?.zones.map(zone => `<label><input type="checkbox" data-layer-zone="${esc(zone)}" ${hiddenZones.includes(zone) ? '' : 'checked'} /><span>${esc(zone)}</span></label>`).join('') ?? '';
    if (renderedZoneOptions !== zoneOptions) {
      const focusedZone = (document.activeElement as HTMLElement | null)?.dataset.layerZone;
      renderedZoneOptions = zoneOptions;
      layerZones.innerHTML = zoneOptions;
      layerZones.querySelectorAll<HTMLInputElement>('[data-layer-zone]').forEach(input => input.onchange = () => callbacks.layerZone(input.dataset.layerZone!, input.checked));
      if (focusedZone) [...layerZones.querySelectorAll<HTMLInputElement>('[data-layer-zone]')].find(input => input.dataset.layerZone === focusedZone)?.focus({ preventScroll: true });
    }
    spread.value = String(view.layerSpread ?? .65);
    const subject = project.concepts.find(item => item.id === selectedId);
    const context = projection.contexts.find(item => item.id === selectedId) ?? projection.contexts.find(item => item.id === (subject?.contextId ?? contextId));
    const located = context?.subjects.find(item => item.id === selectedId);
    const imports = projection.observedImports;
    const scopedImports = architectureImportsForSelection(projection, selectedId, contextId);
    state.textContent = view.checkError ? 'Inspection unavailable' : view.watchingNote ? 'Live updates paused'
      : (shown.dependencies || shown.layers) && imports.state === 'loading' ? 'Observing imports…'
      : shown.dependencies && imports.state === 'unavailable' ? 'Dependencies unavailable'
      : shown.dependencies && imports.state === 'reported' && (!imports.coverage.complete || imports.changedDuringObservation) ? 'Dependencies incomplete'
      : shown.dependencies && imports.state === 'reported' && imports.diagnostics.length > 0 ? 'Dependency observation has limits'
      : (shown.layers || shown.dependencies) && scopedImports.resolution === 'pending' ? 'Resolving source files…' : shown.layers && !chosenRule ? 'No layer order is available.' : '';
    state.title = view.checkError || view.watchingNote || imports.reason;
    if (shown.dependencies && imports.state === 'unavailable' && repository) {
      const retry = document.createElement('button'); retry.textContent = 'Retry'; retry.onclick = () => { if (repository) void observeImports(repository, generation, abort?.signal); };
      state.append(' · ', retry);
    }
    state.hidden = !state.textContent;
    reading.hidden = view.editorOpen || !readingExpanded;
    host.dataset.readingOpen = String(!reading.hidden);
    if (reading.hidden) { readingContents = ''; reading.replaceChildren(); return; }
    const anchors = located?.anchors ?? context?.anchors ?? [];
    const uniqueAnchors = [...new Map(anchors.map(anchor => [anchor.path, anchor])).values()];
    let contents = `<div class="architecture-reading-heading"><span>${esc(subject?.name ?? context?.name ?? project.name)}</span></div>`;
    let allContracts: {key:string;statement:string;kind:'invariant'|'assertion';operation:string}[] = [];
    if (subject) {
      const promises = conceptContracts(project, subject);
      allContracts = [...promises.invariants.map(item => ({...item,kind:'invariant' as const,operation:''})), ...promises.assertions.map(item => ({...item,kind:'assertion' as const}))];
      const active = allContracts.find(item => contract?.subjectId === subject.id && item.key === contract.key && item.kind === contract.kind) ?? allContracts[0];
      contents += `<details class="architecture-section" ${contract?.subjectId === subject.id || !readingExpanded ? 'open' : ''}><summary>Protection · ${promises.invariants.length} invariants, ${promises.assertions.length} assertions</summary>${active ? `<div class="architecture-contract"><small>${active.kind === 'assertion' ? `After ${esc(active.operation)}` : 'Must always hold'}</small><h3>${esc(active.key)}</h3><p>${esc(active.statement)}</p></div>${allContracts.length > 1 ? `<label class="contract-switch">Read a contract<select id="architecture-contract-select">${allContracts.map((item,index) => `<option value="${index}" ${item === active ? 'selected' : ''}>${item.kind} · ${esc(item.key)}</option>`).join('')}</select></label>` : ''}` : '<p>No invariants or operation assertions recorded.</p>'}<p class="architecture-caption">${subject.ownerId ? `Owned by ${esc(project.concepts.find(item => item.id === subject.ownerId)?.name)}. ` : subject.kind === 'aggregate' ? 'Aggregate root. ' : ''}Recorded promises; inspection supplies evidence of enforcement.</p></details>`;
    }
    const diagnostics = subject ? located?.diagnostics ?? [] : context ? context.diagnostics : selectedId ? [] : projection.evaluation.diagnostics;
    const inspectionLabel = view.checking ? 'Inspecting…' : view.checkError ? 'Unavailable' : report ? `Checked ${new Date(report.checkedAt).toLocaleTimeString()}` : 'Waiting for inspection';
    contents += `<details class="architecture-section"><summary>Inspection · ${esc(inspectionLabel)}</summary>`;
    if (view.checkError) contents += `<p role="alert">${esc(view.checkError)}</p>`;
    else if (view.checking) contents += '<p>The repository is being inspected. Evidence will appear here automatically.</p>';
    else if (!report) contents += '<p>Repository inspection starts automatically when the local connection is available.</p>';
    else {
      contents += `<div class="inspection-statuses">${['active','baselined','suppressed'].map(status => `<span data-finding-status="${status}">${diagnostics.filter(item => item.kind === 'violation' && item.status === status).length} ${status}</span>`).join('')}</div>${diagnostics.slice(0,3).map(item => `<button class="architecture-finding" data-architecture-rule="${esc(item.ruleId ?? '')}"><b>${esc(item.status ?? item.kind)}</b><span>${esc(item.ruleId ?? item.message)}</span>${item.path ? `<code>${esc(item.path)}${item.line ? `:${item.line}` : ''}</code>` : ''}</button>`).join('')}`;
      if (!projection.evaluation.outcomesAvailable) contents += '<p class="architecture-caption">Complete per-Rule outcomes were not supplied. An empty finding list does not establish that every Rule passed.</p>';
    }
    contents += '<button data-architecture-report>Open full Report ↗</button></details>';
    contents += `<details class="architecture-section" ${shown.layers ? 'open' : ''}><summary>Files & Zones${uniqueAnchors.length ? ` · ${uniqueAnchors.length} paths` : ''}</summary>`;
    if (failure) contents += `<p role="alert">${esc(failure)}</p>`;
    else if (!projection.linked) contents += `<p>${esc(projection.linkReason)}</p>`;
    else if (!subject && !context) contents += `<p>${projection.zones.length} declared Zones. Enter a context or inspect a path to see its memberships.</p>`;
    else if (!anchors.length) contents += '<p>No source association is available for this selection.</p><button data-architecture-locate>Locate source ↗</button>';
    else contents += `<div class="architecture-paths">${uniqueAnchors.slice(0,3).map(anchor => `<button data-architecture-path="${esc(anchor.path)}"><code>${esc(anchor.path)}${anchor.line ? `:${anchor.line}` : ''}</code></button><div class="architecture-zone-links">${anchor.zones.map(zone => `<button data-architecture-zone="${esc(zone)}">${esc(zone)}</button>`).join('') || `<span>${anchor.membership === 'reported' ? 'No matching Zone' : esc(anchor.reason ?? 'Membership pending')}</span>`}</div>`).join('')}</div>${uniqueAnchors.length > 3 ? '<button data-architecture-locate>Inspect all located paths ↗</button>' : ''}`;
    if (chosenRule) contents += `<div class="architecture-layer-law"><small>${chosenRule.disabled ? 'Disabled Layer Rule' : 'Declared layer order'}</small><p>${chosenRule.zones.map(esc).join(' → ')}</p><button data-architecture-rule="${esc(chosenRule.id)}">${esc(chosenRule.id)} ↗</button></div>`;
    if (projection.unavailableLayerRuleIds.length) contents += `<p class="architecture-caption">No structured layer order was supplied for these Rules:</p>${projection.unavailableLayerRuleIds.map(id => `<button data-architecture-rule="${esc(id)}">${esc(id)} ↗</button>`).join('')}`;
    contents += '</details>';
    if (shown.dependencies) {
      const scoped = scopedImports;
      const importPages = Math.max(1, Math.ceil(scoped.edges.length / 6));
      importPage = Math.min(importPage, importPages - 1);
      contents += `<details class="architecture-section"><summary>Dependencies${imports.state === 'reported' && scoped.resolution === 'known' ? ` · ${scoped.edges.length} import occurrences` : scoped.resolution === 'pending' ? ' · resolving source' : ''}</summary><p class="architecture-caption">${esc(imports.reason)}</p>`;
      if (imports.state === 'reported') {
        contents += `<p class="architecture-caption">${esc(scoped.reason)}</p>${scoped.edges.slice(importPage * 6, (importPage + 1) * 6).map(edge => `<div class="architecture-import"><button data-architecture-path="${esc(edge.sourcePath)}"><code>${esc(edge.sourcePath)}:${edge.line}</code></button><span aria-hidden="true">↓</span><button data-architecture-path="${esc(edge.targetPath)}"><code>${esc(edge.targetPath)}${edge.targetKind === 'directory' ? '/' : ''}</code></button></div>`).join('')}${importPages > 1 ? `<nav class="architecture-import-pages" aria-label="Import occurrences"><button data-import-page="${importPage - 1}" ${importPage === 0 ? 'disabled' : ''}>← Previous</button><span>${importPage + 1} / ${importPages}</span><button data-import-page="${importPage + 1}" ${importPage === importPages - 1 ? 'disabled' : ''}>Next →</button></nav>` : ''}`;
        if (imports.diagnostics.length || imports.limitations.length) contents += `<details><summary>Observation limits</summary>${imports.limitations.map(limit => `<p>${esc(limit)}</p>`).join('')}${imports.diagnostics.map(diagnostic => `<p>${diagnostic.path ? `<code>${esc(diagnostic.path)}</code> · ` : ''}${esc(diagnostic.message)}</p>`).join('')}</details>`;
      }
      contents += '</details>';
    }
    if (!writeReading(contents)) return;
    reading.querySelector<HTMLSelectElement>('#architecture-contract-select')?.addEventListener('change', event => {
      const chosen = allContracts[Number((event.target as HTMLSelectElement).value)];
      contract = {subjectId:subject!.id,key:chosen.key,kind:chosen.kind}; draw();
    });
    reading.querySelectorAll<HTMLElement>('[data-architecture-path]').forEach(button => button.onclick = () => callbacks.inspectPath(button.dataset.architecturePath!));
    reading.querySelectorAll<HTMLButtonElement>('[data-import-page]').forEach(button => button.onclick = () => { importPage = Number(button.dataset.importPage); draw(); });
    reading.querySelectorAll<HTMLElement>('[data-architecture-zone]').forEach(button => button.onclick = () => callbacks.inspectZone(button.dataset.architectureZone!));
    reading.querySelectorAll<HTMLElement>('[data-architecture-rule]').forEach(button => button.onclick = () => button.dataset.architectureRule ? callbacks.inspectRule(button.dataset.architectureRule) : callbacks.report());
    reading.querySelector<HTMLElement>('[data-architecture-report]')?.addEventListener('click', callbacks.report);
    reading.querySelector<HTMLElement>('[data-architecture-locate]')?.addEventListener('click', callbacks.locate);
  }

  async function observeImports(repo: RepositoryProject, ticket: number, signal?: AbortSignal) {
    if (dependencies && 'state' in dependencies && dependencies.state === 'loading') return;
    dependencies = { state: 'loading', reason: 'Observing repository imports.' }; draw(); callbacks.changed();
    try {
      const result = await queryRepositoryDependencies(signal);
      if (ticket === generation && repository?.loadedAt === repo.loadedAt) dependencies = result;
    } catch (error) {
      if (ticket === generation && !signal?.aborted) dependencies = { state: 'unavailable', reason: error instanceof Error ? error.message : String(error) };
    } finally { if (ticket === generation && !signal?.aborted) { draw(); callbacks.changed(); } }
  }
  function requestImports(repo: RepositoryProject, ticket: number, signal?: AbortSignal) {
    if (visibility().layers || visibility().dependencies) return observeImports(repo, ticket, signal);
    return Promise.resolve();
  }

  async function locatePaths(repo: RepositoryProject, ticket: number, signal: AbortSignal) {
    try {
      const result = await loadArchitecturePaths(repo, signal, undefined, {
        progress: partial => { if (ticket === generation) { paths = partial; draw(); callbacks.changed(); } },
        priority: () => {
          const selected = projection?.contexts.flatMap(context => context.subjects).find(subject => subject.id === view?.selectedId);
          const context = projection?.contexts.find(context => context.id === view?.contextId);
          return (selected?.anchors ?? context?.anchors ?? []).map(anchor => anchor.path);
        },
      });
      if (ticket === generation) paths = result;
    }
    catch (error) { if (ticket === generation && !signal.aborted) failure = error instanceof Error ? error.message : String(error); }
    finally { if (ticket === generation) { draw(); callbacks.changed(); } }
  }
  async function refresh() {
    abort?.abort(); abort = new AbortController(); const ticket = ++generation;
    failure = ''; paths = []; dependencies = null; draw();
    try {
      const repo = await loadRepository(abort.signal); if (ticket !== generation) return;
      repository = repo; report = null; callbacks.loaded?.(repo,null); draw(); callbacks.changed();
      await Promise.all([locatePaths(repo,ticket,abort.signal), requestImports(repo,ticket,abort.signal)]);
    } catch (error) { if (ticket === generation) { failure = error instanceof Error ? error.message : String(error); draw(); callbacks.changed(); } }
  }
  return {
    update(next) { view = next; draw(); if (repository && dependencies === null) void requestImports(repository,generation,abort?.signal); }, get evidence() { return projection; }, refresh,
    receive(repo, result) {
      report = result;
      if (repo && repo.loadedAt !== repository?.loadedAt) {
        repository = repo; paths = []; dependencies = null; failure = '';
        abort?.abort(); abort = new AbortController(); const ticket = ++generation;
        void locatePaths(repo,ticket,abort.signal);
        void requestImports(repo,ticket,abort.signal);
      } else if (!repo) { repository = null; paths = []; dependencies = null; abort?.abort(); generation++; }
      draw(); callbacks.changed();
    },
    openContract(subjectId,key,kind) { contract = {subjectId,key,kind}; readingContents = ''; reading.replaceChildren(); draw(); readingExpanded = true; draw(); },
  };
}
