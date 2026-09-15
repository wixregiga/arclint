import { parse, stringify } from 'yaml';
import type { Baseline, DomainProject, StudioMode } from './contracts';
import { importProject, exportDomainYaml } from './serialization';
import { conceptContracts, kindNames, planPosition, record, relationshipDescription } from './model-evidence';
import { loadRepository, queryRepositoryRevision, queryRepositoryContext, queryRepositoryZone, checkRepository, queryRepositoryRule, queryRepositoryPatterns, queryRepositoryDirectory, previewRepositoryDocument, applyRepositoryDocument, previewRepositoryBaseline, applyRepositoryBaseline, diagnosticCounts, filterDiagnostics, type DiagnosticFilter, type RepositoryDiagnostic, type RepositoryProject, type RepositoryContextReport, type RepositoryContextResult, type RepositoryCheckResult, type RepositoryRuleSummary, type RepositoryPatternCatalog, type RepositoryDocument, type RepositoryDocumentPreview, type RepositoryBaselinePreview } from './repository';
import './workbench.css';
import type { FocusView } from './view-state';
import { buildArchitectureEvidence } from './architecture-evidence';

const escape = (value: unknown) => String(value ?? '').replace(/[&<>"']/g, c => ({ '&': '&amp;', '<': '&lt;', '>': '&gt;', '"': '&quot;', "'": '&#39;' })[c]!);
type Representation = 'site' | 'plan' | 'matrix';
export type GovernanceSection = 'rules' | 'zones' | 'findings' | 'paths' | 'patterns';
interface Callbacks { inspectionInvalidated?(): void; select(id: string): void; replace(project: DomainProject): void; create(): void; notify(message: string, error?: boolean): void; evidenceChanged?(repository: RepositoryProject | null, report: RepositoryCheckResult | null): void; checkingChanged?(checking: boolean, error?: string): void }
export interface Workbench {
  update(project: DomainProject, selectedId: string | null, scopeId: string | null, mode: StudioMode, baseline: Baseline | null, view: FocusView): void;
  inspectSelection(id: string | null): void; selectionEvidence(id: string): string; bindSelection(): void;
  showGovernance(section?: GovernanceSection): void; checkCode(): void; checkQuietly(isCurrent?: () => boolean): Promise<void>; showRepository(): void;
  getRepository(): RepositoryProject | null; getReport(): RepositoryCheckResult | null; saveDomain(domainYaml: string): void;
  inspectPath(path: string): void; inspectZone(zone: string): void; showRule(id: string): void;
  initializeRepository(repository: RepositoryProject): boolean;
  receiveEvidence(repository: RepositoryProject | null, report: RepositoryCheckResult | null): void;
}
const sectionNames: Record<GovernanceSection, string> = { paths: 'Code paths', zones: 'Zones', rules: 'Rules', patterns: 'Patterns', findings: 'Report' };
type CheckEvaluation = { state: 'reported' | 'superseded' | 'stale' | 'failed' | 'busy'; message?: string };

export function createWorkbench(host: HTMLElement, callbacks: Callbacks): Workbench {
  let project: DomainProject;
  let repositoryOpening = false;
  let focusView: FocusView | null = null;
  let selected: string | null = null, scope: string | null = null;
  let mode: StudioMode = 'domain', baseline: Baseline | null = null;
  let representation: Representation = 'site';
  let matrixRowPage = 0, matrixColumnPage = 0;
  const matrixPageSize = 8;
  let repository: RepositoryProject | null = null;
  let run: RepositoryCheckResult | null = null;
  let patternCatalog: RepositoryPatternCatalog | null = null;
  let inspected: RepositoryContextResult | null = null;
  let activeSection: GovernanceSection = 'rules';
  let reportFilter: DiagnosticFilter = 'all';
  let ruleSearch = '', reportSearch = '', directory = '.';
  let requestGeneration = 0;
  // Repository evidence has its own lifetime; closing a reading does not end it.
  let evidenceGeneration = 0;
  let pendingRepository: { generation: number; promise: Promise<RepositoryProject> } | null = null;
  let pendingCheck: Promise<CheckEvaluation> | null = null;
  let pendingCheckGuards: (() => boolean)[] = [];
  let busy = false;
  let policyDraft: { content: string; expectedHash: string | null } | null = null;
  host.insertAdjacentHTML('beforeend', `
    <div class="workbench" aria-label="Architecture workbench">
      <form id="workbench-form"><label class="sr-only" for="workbench-input">Find a term or inspect a code path</label><span aria-hidden="true">⌕</span><input id="workbench-input" placeholder="A term or a repository path…" autocomplete="off" /><button id="workbench-submit" type="submit">Inspect ↗</button></form>
      <div class="drawing-controls"><div role="group" aria-label="Representation"><button id="view-site" aria-pressed="true">Map</button><button id="view-plan" aria-pressed="false">Plan</button><button id="view-matrix" aria-pressed="false">Matrix</button></div><button id="open-architecture">Zones & layers</button><button id="open-repository">Open repository</button><button id="run-repository-check">Check code</button></div>
      <div class="purpose-line"><button id="edit-purpose">Set the user and need for this model ↗</button><span id="connection-state">Model draft · code not inspected</span></div>
      <form id="purpose-form" hidden><label>User<input name="actor" required placeholder="Who needs this?" maxlength="100" /></label><label>Need<input name="need" required placeholder="What must they be able to do?" maxlength="240" /></label><button type="submit">Set anchor</button><button type="button" id="close-purpose">Cancel</button></form>
    </div>
    <section id="projection" aria-label="Alternative model representation" hidden></section>
    <section id="evidence-card" aria-label="ArcLint evidence" hidden></section>
  `);
  const $ = <T extends HTMLElement = HTMLElement>(selector: string) => host.querySelector<T>(selector)!;
  const card = $('#evidence-card');
  const closeTools = () => (host.querySelector('#tools-drawer') as HTMLDialogElement | null)?.close();
  host.querySelector('.tools-body')?.append(host.querySelector('.workbench')!);
  const closeEvidence = () => { requestGeneration++; card.hidden = true; };
  host.addEventListener('keydown', event => { if (event.key === 'Escape' && !card.hidden) { closeEvidence(); event.stopPropagation(); } });
  function bindEvidence() {
    const refresh = $('#refresh-repository');
    if (refresh) refresh.onclick = () => { clearRepositoryEvidence(); void showGovernance(activeSection).finally(() => callbacks.inspectionInvalidated?.()); };
    card.querySelectorAll<HTMLElement>('[data-inspect-path]').forEach(b => b.onclick = () => void inspect(b.dataset.inspectPath!));
    card.querySelectorAll<HTMLElement>('[data-inspect-zone]').forEach(b => b.onclick = () => void inspect(b.dataset.inspectZone!, true));
    card.querySelectorAll<HTMLElement>('[data-rule-detail]').forEach(b => b.onclick = () => void ruleDetail(b.dataset.ruleDetail!));
    card.querySelectorAll<HTMLElement>('[data-directory]').forEach(b => b.onclick = () => { directory = b.dataset.directory!; void showGovernance('paths'); });
    card.querySelectorAll<HTMLElement>('[data-copy]').forEach(b => b.onclick = async () => {
      try { await navigator.clipboard.writeText(b.dataset.copy!); callbacks.notify('Copied to clipboard.'); }
      catch { callbacks.notify('Clipboard unavailable. Select and copy the displayed text.', true); }
    });
  }
  function evidence(title: string, html: string) {
    closeTools(); card.hidden = false;
    card.innerHTML = `<div class="evidence-heading"><div><span>ARCLINT · REPOSITORY</span><h2>${escape(title)}</h2></div><button id="close-evidence" aria-label="Close evidence">×</button></div>
      <nav class="governance-nav" aria-label="Repository workspace">${(Object.keys(sectionNames) as GovernanceSection[]).map(section => `<button data-governance-section="${section}" aria-current="${activeSection === section ? 'page' : 'false'}">${sectionNames[section]}</button>`).join('')}</nav>
      <div class="evidence-body">${html}</div>`;
    $('#close-evidence').onclick = closeEvidence;
    card.querySelectorAll<HTMLElement>('[data-governance-section]').forEach(b => b.onclick = () => void showGovernance(b.dataset.governanceSection as GovernanceSection));
    bindEvidence();
  }
  const error = (failure: unknown) => escape(failure instanceof Error ? failure.message : String(failure));
  function clearRepositoryEvidence() {
    evidenceGeneration++; pendingRepository = null;
    repository = null; patternCatalog = null; inspected = null; run = null;
    markReportUpdating(); callbacks.evidenceChanged?.(null,null); status();
  }
  function clearReportEvidence() {
    evidenceGeneration++; pendingRepository = null; run = null;
    markReportUpdating(); callbacks.evidenceChanged?.(repository,null); status();
  }
  async function reloadRepositoryEvidence() {
    clearRepositoryEvidence();
    return ensureRepository();
  }
  async function ensureRepository(): Promise<RepositoryProject> {
    for (let attempt = 0; attempt < 3; attempt++) {
      if (repository) return repository;
      const generation = evidenceGeneration;
      if (!pendingRepository || pendingRepository.generation !== generation) pendingRepository = { generation, promise: loadRepository() };
      const pending = pendingRepository;
      try {
        const loaded = await pending.promise;
        if (generation !== evidenceGeneration) continue;
        if (!repository) { repository = loaded; callbacks.evidenceChanged?.(repository,run); }
        return repository;
      } catch (failure) {
        if (generation === evidenceGeneration) throw failure;
      } finally { if (pendingRepository === pending) pendingRepository = null; }
    }
    throw new Error('The repository is still changing. Open this reading again in a moment.');
  }
  async function readCurrentEvidence<T>(ticket: number, read: () => Promise<T>): Promise<T | undefined> {
    for (let attempt = 0; attempt < 3 && ticket === requestGeneration; attempt++) {
      await ensureRepository();
      if (ticket !== requestGeneration) return;
      const generation = evidenceGeneration;
      try {
        const result = await read();
        if (ticket !== requestGeneration) return;
        if (generation === evidenceGeneration) return result;
      } catch (failure) {
        if (ticket !== requestGeneration) return;
        if (generation === evidenceGeneration) throw failure;
      }
    }
    if (ticket === requestGeneration) throw new Error('The repository is still changing. Open this reading again in a moment.');
  }
  function status() {
    $('#connection-state').textContent = repository ? `${repository.repository.name} · on-disk evidence${run ? ` · checked ${new Date(run.checkedAt).toLocaleTimeString()}` : ' · not checked'}` : 'Model draft · code not inspected';
  }
  const boundSource = () => `<div class="repository-source"><span>${escape(repository?.repository.name ?? 'Local repository')}</span><code>${escape(repository?.repository.root ?? '')}</code><small>On disk${repository ? ` · loaded ${escape(new Date(repository.loadedAt).toLocaleTimeString())}` : ''}</small><button id="refresh-repository">Refresh evidence ↻</button></div>`;
  function focusBar() {
    const term = project?.concepts.find(c => c.id === selected);
    const context = project?.contexts.find(c => c.id === (term?.contextId ?? selected ?? scope));
    return `${boundSource()}${inspected ? `<div class="governance-focus"><span>Scope of inspection</span><b>${escape(inspected.zone ? `Zone ${inspected.zone}` : inspected.path)}</b><button id="clear-code-focus">Whole repository ×</button></div>` : ''}${term || context ? `<div class="governance-attention"><span>Model selection: ${escape(context?.name)}${term ? ` / ${escape(term.name)}` : ''}</span><button id="inspect-attention">Locate source ↗</button></div>` : ''}`;
  }
  function bindFocus() {
    $('#edit-repository-policy')?.addEventListener('click', () => void editPolicy());
    $('#clear-code-focus')?.addEventListener('click', () => { inspected = null; void showGovernance(activeSection); });
    $('#inspect-attention')?.addEventListener('click', () => void inspectSelection(selected ?? scope));
  }
  function ruleRows(report: RepositoryContextReport) {
    return (report.Rules ?? []).map(({ Summary: r, Reason }) => ruleRow({ id: r.ID, type: r.Type, severity: r.Severity, proposition: r.Proposition, rationale: r.Rationale, assurance: r.Assurance, provenance: r.Provenance, disabled: r.Disabled, disabledReason: r.DisabledReason }, Reason)).join('') || '<p class="evidence-empty">No governing Rules returned for this selection.</p>';
  }
  function ruleRow(rule: RepositoryRuleSummary, reason = '') {
    return `<article class="policy-row"><div><button data-rule-detail="${escape(rule.id)}">${escape(rule.id)} ↗</button><span>${escape(rule.disabled ? 'disabled' : rule.severity)}${rule.assurance ? ` · ${escape(rule.assurance)}` : ''}</span></div><p>${escape(rule.proposition)}</p>${reason ? `<small>${escape(reason)}</small>` : ''}${rule.provenance ? `<small>${escape(rule.provenance)}</small>` : ''}</article>`;
  }
  function contextBody(path: string, report: RepositoryContextReport) {
    return `${focusBar()}<p class="evidence-command">arclint context ${escape(path)}</p>${inspected?.pathType === 'missing' ? '<p class="evidence-meta">This path does not exist on disk. ArcLint returned the policy that would apply at this path, not observations of a file.</p>' : ''}<div class="workspace-section-heading"><h3>Zone memberships</h3><span>${report.Zones?.length ?? 0} selected Zones</span></div><p class="evidence-meta">Zones are overlapping file sets. Membership does not assign a term to a bounded context.</p>
      ${(report.Paths ?? []).map(p => `<div class="membership-row"><code>${escape(p.Path)}</code><span>${(p.Zones ?? []).map(zone => `<button data-inspect-zone="${escape(zone)}">${escape(zone)}</button>`).join('') || 'No declared Zone'}</span></div>`).join('')}
      ${(report.Zones ?? []).map(z => `<details class="zone-record"><summary>${escape(z.Name)}</summary><p>${escape(z.Description)}</p><code>${escape((z.Paths ?? []).join('\n'))}</code><p>Reported import contract: ${z.InternalRestricted ? escape(z.Internal?.join(', ') || 'no other declared Zone') : 'internal imports unrestricted by this contract'}. External: ${escape(z.External)}. Standard library: ${escape(z.Stdlib)}.</p></details>`).join('') || '<p>No declared Zone matches this path.</p>'}
      <h3>What governs it</h3><p class="evidence-meta">The reported import contract is declared policy. Layer Rules, incoming import restrictions, and other overlapping Zones may further restrict dependencies. Governing Rules remain inspectable below.</p>${ruleRows(report)}
      <h3>Recorded contract anchors</h3>${(report.domain?.contexts ?? []).flatMap(c => [...c.invariants ?? [], ...c.assertions ?? []].map(contract => `<article class="contract-record"><b>${escape(c.name)} / ${escape(contract.owner)} · ${escape(contract.key)}</b><p>${escape(contract.statement)}</p>${contract.on ? `<small>After ${escape(contract.on)}</small>` : ''}<small>${escape(contract.anchor ?? 'unknown')}${contract.reason ? ` · ${escape(contract.reason)}` : ''}</small>${contract.source ? `<button class="path-button" data-inspect-path="${escape(contract.source.replace(/:\d+$/, ''))}">${escape(contract.source)} ↗</button>` : '<small>No located source anchor</small>'}</article>`)).join('') || '<p>No contract anchors returned for this path.</p>'}`;
  }
  async function inspect(path: string, zone = false) {
    activeSection = zone ? 'zones' : 'paths';
    const ticket = ++requestGeneration;
    evidence('Inspecting code', `<p>Reading <code>${escape(path)}</code> from the bound repository…</p>`);
    try {
      const result = await readCurrentEvidence(ticket, () => zone ? queryRepositoryZone(path) : queryRepositoryContext(path));
      if (!result) return;
      inspected = result;
      evidence(zone ? `Zone: ${path}` : path, contextBody(result.path, result.report)); bindFocus(); status();
    } catch (failure) { if (ticket === requestGeneration) evidence('Code inspection unavailable', `<p role="alert">${error(failure)}</p>`); }
  }
  function architectureBody() {
    const repo = repository!;
    const architecture = buildArchitectureEvidence(project, repo);
    return `${focusBar()}<div class="policy-actions"><p>A Zone is a named file set. One file may belong to several Zones.</p><button id="edit-repository-policy">Edit Zones & Rules ↗</button></div>
      <div class="zone-directory">${(repo.context.Zones ?? []).map(zone => `<article><button data-inspect-zone="${escape(zone.Name)}"><b>${escape(zone.Name)}</b><span>Inspect →</span></button><p>${escape(zone.Description)}</p><code>${escape((zone.Paths ?? []).join('\n'))}</code></article>`).join('') || '<p>No Zones are declared.</p>'}</div>
      <h3>Declared dependency layers</h3><p class="evidence-meta">Only explicit layers Constraints determine order. These are locally authored Rules; distributed Rules remain inspectable in Rules.</p>${architecture.layers.map(rule => `<article class="layer-section"><button data-rule-detail="${escape(rule.id)}">${escape(rule.id)} ↗</button><p>${rule.disabled ? 'Disabled Layer Rule · this declared order is not being evaluated.' : 'This Rule forbids imports toward a higher layer. Other governing Rules can further restrict imports.'}</p><ol class="layer-stack">${rule.zones.map((zone, index) => `<li><span>${index === 0 ? 'HIGHEST' : 'LOWER'}</span><button data-inspect-zone="${escape(zone)}">${escape(zone)}</button><small>${rule.disabled ? 'Authored order · disabled' : 'Declared order under this Rule'}</small></li>`).join('')}</ol></article>`).join('') || '<p>No local layer order is declared.</p>'}${architecture.unavailableLayerRuleIds.length ? `<p class="evidence-meta">Structured order is unavailable for ${architecture.unavailableLayerRuleIds.map(id => `<button data-rule-detail="${escape(id)}">${escape(id)} ↗</button>`).join(', ')}.</p>` : ''}`;
  }
  function currentRules(): RepositoryRuleSummary[] {
    if (!inspected) return repository?.rules ?? [];
    return (inspected.report.Rules ?? []).map(({ Summary: r }) => ({ id: r.ID, type: r.Type, severity: r.Severity, proposition: r.Proposition, rationale: r.Rationale, assurance: r.Assurance, provenance: r.Provenance, disabled: r.Disabled, disabledReason: r.DisabledReason }));
  }
  function renderRuleList() {
    const all = currentRules();
    const matches = all.filter(r => `${r.id} ${r.type} ${r.proposition}`.toLocaleLowerCase().includes(ruleSearch.toLocaleLowerCase()));
    $('#rule-list').innerHTML = `<p class="evidence-meta">${matches.length} of ${all.length} Rules in this inspection.</p>${matches.map(r => ruleRow(r)).join('') || '<p class="evidence-empty">No Rules match this search.</p>'}`;
    bindEvidence();
  }
  async function ruleDetail(id: string) {
    activeSection = 'rules'; const ticket = ++requestGeneration;
    evidence(id, '<p>Reading the Rule’s complete contract…</p>');
    try {
      const detail = await readCurrentEvidence(ticket, () => queryRepositoryRule(id));
      if (!detail) return;
      const rule = detail.summary;
      const raw = record(record(parse(repository!.rulesYaml ?? '')).rules)[id];
      const paths = Array.isArray(detail.files) ? detail.files : detail.paths;
      evidence(id, `${focusBar()}<button id="back-rule-list" class="path-button">← Back to Rules</button><p class="rule-proposition">${escape(rule.proposition)}</p>
        <dl class="rule-facts"><div><dt>Constraint type</dt><dd>${escape(rule.type)}</dd></div><div><dt>Scope</dt><dd>${detail.entireRepository ? 'Entire repository' : detail.zones?.length ? detail.zones.map(zone => `<button data-inspect-zone="${escape(zone)}">${escape(zone)} ↗</button>`).join(' ') : 'Not supplied in this detail'}</dd></div>${Array.isArray(paths) ? `<div><dt>File narrowing</dt><dd><code>${escape(paths.join('\n'))}</code></dd></div>` : ''}<div><dt>Severity</dt><dd>${escape(rule.severity)}</dd></div><div><dt>Assurance</dt><dd>${escape(rule.assurance ?? 'Not supplied')}</dd></div><div><dt>Evidence method</dt><dd>${escape(detail.evidence ?? 'Not supplied')}</dd></div><div><dt>Origin</dt><dd>${escape(rule.provenance ?? (rule.builtIn ? 'Built into ArcLint' : 'Repository Rule'))}</dd></div></dl>
        ${rule.rationale ? `<h3>Rationale</h3><p>${escape(rule.rationale)}</p>` : '<p class="evidence-meta">No authored Rationale.</p>'}${rule.disabled ? `<p>Disabled${rule.disabledReason ? `: ${escape(rule.disabledReason)}` : ''}</p>` : ''}
        ${detail.limitations ? `<h3>Analysis limits</h3><p>${escape(Array.isArray(detail.limitations) ? detail.limitations.join('\n') : detail.limitations)}</p>` : ''}
        ${raw ? `<details><summary>Authored configuration · rules.arclint.yaml</summary><pre>${escape(stringify({ [id]: raw }))}</pre></details>` : ''}${detail.schema ? `<details><summary>Constraint shape and accepted Scope</summary><pre>${escape(detail.schema)}</pre></details>` : ''}
        <div class="evidence-actions"><button data-copy="${escape(id)}">Copy Rule ID</button><button data-copy="${escape(`arclint rules '${id.replace(/'/g, "'\\''")}'`)}">Copy inspect command</button></div>`);
      $('#back-rule-list').onclick = () => void showGovernance('rules'); bindFocus();
    } catch (failure) { if (ticket === requestGeneration) evidence('Rule unavailable', `<p role="alert">${error(failure)}</p>`); }
  }
  function renderReportRecords() {
    if (!run) return;
    const records = filterDiagnostics(run.diagnostics, reportFilter).filter(d => `${d.ruleId ?? ''} ${d.path ?? ''} ${d.message}`.toLocaleLowerCase().includes(reportSearch.toLocaleLowerCase()));
    $('#report-records').innerHTML = `<p class="evidence-meta">${records.length} returned record${records.length === 1 ? '' : 's'} shown. Repeated occurrences are preserved.</p>${records.map(d => `<article class="diagnostic-record" data-diagnostic-status="${escape(d.status ?? d.kind)}"><div>${d.ruleId ? `<button data-rule-detail="${escape(d.ruleId)}">${escape(d.ruleId)} ↗</button>` : `<b>${escape(d.kind)}</b>`}<span>${escape([d.kind, d.status, d.severity].filter(Boolean).join(' · '))}</span></div><p>${escape(d.message)}</p>${d.path ? `<button class="path-button" data-inspect-path="${escape(d.path)}">${escape(d.path)}${d.line ? `:${d.line}` : ''} ↗</button>` : ''}${d.remediation ? `<p><b>Suggested remediation.</b> ${escape(d.remediation)}</p>` : ''}${d.assurance ? `<small>Reported assurance: ${escape(d.assurance)}</small>` : ''}${typeof d.fingerprint === 'string' ? `<code>Fingerprint: ${escape(d.fingerprint)}</code>` : ''}<button class="copy-diagnostic" data-copy="${escape(JSON.stringify(d, null, 2))}">Copy evidence</button></article>`).join('') || '<p class="evidence-empty">No returned records match this filter.</p>'}`;
    bindEvidence();
  }
  function reportBody() {
    if (!run) return `${boundSource()}<div class="evidence-empty"><h3>No Report yet</h3><p>Inspection runs automatically against repository files and configured Rules. Results will appear here when it finishes. Unsaved browser drafts are not checked.</p><button id="check-from-report" class="primary">Check code →</button></div>`;
    const counts = diagnosticCounts(run.diagnostics);
    return `${boundSource()}<div class="report-summary"><div><h3>${run.exitCode === 0 ? 'Check completed' : 'Gate failed'}</h3><p>${escape(new Date(run.checkedAt).toLocaleString())} · exit ${run.exitCode}</p></div><button id="check-from-report">Run again ↻</button></div><p>${counts.active} active · ${counts.baselined} baselined · ${counts.suppressed} suppressed · ${counts.operational} operational · ${counts.coverage} coverage</p><div class="document-actions"><button id="review-baseline-adoption">Review Baseline adoption →</button><button id="download-report">Download Report JSON ↓</button></div><p class="evidence-meta">Baseline adoption acknowledges findings; it does not repair them. This check reads repository files on disk, excluding unsaved browser edits.</p>${!run.outcomesAvailable ? '<p class="evidence-meta">A complete per-Rule outcome table was not supplied. Zero active findings does not establish that every Rule passed. Assurance and fingerprints appear only when emitted.</p>' : ''}
      <div class="report-filters" role="group" aria-label="Report filter">${(Object.keys(counts) as DiagnosticFilter[]).map(filter => `<button data-report-filter="${filter}" aria-pressed="${filter === reportFilter}">${filter === 'all' ? 'All' : filter === 'baselined' ? 'Baselined' : filter[0].toUpperCase() + filter.slice(1)} <span>${counts[filter]}</span></button>`).join('')}</div><label class="evidence-search">Find in Report<input id="report-search" value="${escape(reportSearch)}" placeholder="Rule ID, path, or message" /></label><div id="report-records"></div>${run.stderr ? `<details><summary>CLI output</summary><pre>${escape(run.stderr)}</pre></details>` : ''}`;
  }
  function markReportUpdating() {
    const surface = $('#report-surface');
    if (!surface) return;
    surface.setAttribute('aria-busy', 'true');
    surface.querySelector<HTMLElement>('#report-updating')!.hidden = false;
    for (const id of ['download-report', 'review-baseline-adoption']) surface.querySelector<HTMLButtonElement>(`#${id}`)?.setAttribute('disabled', '');
  }
  function renderReportSurface() {
    const surface = $('#report-surface');
    if (!surface || card.hidden) return;
    const active = document.activeElement as HTMLInputElement | null;
    const focused = active && surface.contains(active) ? active : null;
    const caret = focused?.id === 'report-search' ? [focused.selectionStart, focused.selectionEnd] : null;
    const selector = focused?.id ? `#${focused.id}` : focused?.dataset.reportFilter ? `[data-report-filter="${focused.dataset.reportFilter}"]` : null;
    const scroll = card.querySelector('.evidence-body')!.scrollTop;
    surface.innerHTML = `<p id="report-updating" class="evidence-meta" ${busy ? '' : 'hidden'}>Inspection is updating. Any displayed findings belong to the previous completed Report.</p>${reportBody()}`;
    surface.setAttribute('aria-busy', String(busy));
    $('#check-from-report')?.addEventListener('click', () => void checkCode());
    if (run) {
      const source = repository, report = run;
      renderReportRecords();
      $('#review-baseline-adoption').onclick = () => void reviewBaselineAdoption();
      card.querySelectorAll<HTMLElement>('[data-report-filter]').forEach(b => b.onclick = () => { reportFilter = b.dataset.reportFilter as DiagnosticFilter; card.querySelectorAll<HTMLElement>('[data-report-filter]').forEach(item => item.setAttribute('aria-pressed', String(item === b))); renderReportRecords(); });
      $('#report-search').oninput = event => { reportSearch = (event.target as HTMLInputElement).value; renderReportRecords(); };
      $('#download-report').onclick = () => { const url = URL.createObjectURL(new Blob([JSON.stringify({ repository: source!.repository, ...report }, null, 2)], { type: 'application/json' })); const a = document.createElement('a'); a.href = url; a.download = 'arclint-report.json'; a.click(); setTimeout(() => URL.revokeObjectURL(url), 1000); };
    }
    bindEvidence();
    if (selector) {
      const next = surface.querySelector<HTMLInputElement>(selector);
      next?.focus({ preventScroll: true });
      if (caret) next?.setSelectionRange(caret[0], caret[1]);
    }
    card.querySelector('.evidence-body')!.scrollTop = scroll;
  }
  async function showGovernance(section: GovernanceSection = 'rules') {
    activeSection = section; const ticket = ++requestGeneration;
    evidence(sectionNames[section], '<p>Reading the bound repository…</p>');
    try {
      await ensureRepository(); if (ticket !== requestGeneration) return;
      if (section === 'zones') evidence('Zones & layers', architectureBody());
      if (section === 'rules') {
        evidence('Rules', `${focusBar()}<div class="policy-actions"><p>Each Rule states one Constraint over a compatible Scope.</p><button id="edit-repository-policy">Edit Rules, Zones & Bindings ↗</button></div><label class="evidence-search">Find a Rule<input id="governance-search" value="${escape(ruleSearch)}" placeholder="Rule ID, type, or proposition" /></label><div id="rule-list"></div>`);
        renderRuleList(); $('#governance-search').oninput = event => { ruleSearch = (event.target as HTMLInputElement).value; renderRuleList(); };
      }
      if (section === 'findings') {
        evidence('Report', '<section id="report-surface"></section>');
        renderReportSurface();
      }
      if (section === 'paths') {
        const listing = await readCurrentEvidence(ticket, () => queryRepositoryDirectory(directory));
        if (!listing) return;
        const parents = directory === '.' ? '' : `<button data-directory="${escape(directory.split('/').slice(0, -1).join('/') || '.')}">← Parent directory</button>`;
        evidence('Code paths', `${focusBar()}<form id="path-inspect-form" class="path-inspect-form"><label>Inspect a repository path<input id="path-inspect-input" placeholder="internal/example/root.go" required /></label><button type="submit">Inspect →</button></form><p class="evidence-meta">Files on disk, not a claim that every file was scanned. Select a file to ask ArcLint for all matching Zones and governing Rules.</p><div class="directory-heading"><code>${escape(listing.directory)}/</code>${parents}</div><div class="source-directory">${listing.entries.map(entry => entry.kind === 'directory' ? `<button data-directory="${escape(entry.path)}"><span aria-hidden="true">▱</span><span>${escape(entry.name)}/</span><small>Open →</small></button>` : `<div><button data-inspect-path="${escape(entry.path)}"><span aria-hidden="true">·</span><span>${escape(entry.name)}</span></button><button class="copy-path" data-copy="${escape(entry.path)}" aria-label="Copy path ${escape(entry.name)}">Copy</button></div>`).join('') || '<p>This directory contains no regular files or directories.</p>'}</div>${listing.truncated ? '<p>Showing the first 500 entries. Enter an exact path above to inspect another entry.</p>' : ''}`);
        $('#path-inspect-form').onsubmit = event => { event.preventDefault(); void inspect(($('#path-inspect-input') as HTMLInputElement).value.trim()); };
      }
      if (section === 'patterns') {
        const catalog = patternCatalog ?? await readCurrentEvidence(ticket, queryRepositoryPatterns);
        if (!catalog) return;
        patternCatalog = catalog;
        const yaml = record(parse(repository!.rulesYaml ?? '')); const installations = Array.isArray(yaml.extends) ? yaml.extends.map(record) : [];
        evidence('Patterns', `${boundSource()}<div class="policy-actions"><p>Patterns distribute Rules and Zone declarations. Bindings supply the local paths.</p><button id="edit-repository-policy">Edit extends & Bindings ↗</button></div><h3>Configured installations</h3>${installations.map(installation => `<article class="pattern-installation"><b>${escape(installation.pattern)}</b>${Object.entries(record(installation.bind)).map(([zone, paths]) => `<div class="binding-row"><button data-inspect-zone="${escape(zone)}">${escape(zone)} ↗</button><code>${escape(Array.isArray(paths) ? paths.join('\n') : paths)}</code></div>`).join('') || '<p>No Bindings recorded.</p>'}${repository!.rules.filter(rule => rule.provenance === installation.pattern).map(rule => ruleRow(rule)).join('')}</article>`).join('') || '<p>No Patterns are extended by this repository.</p>'}<h3>Available offline</h3>${patternCatalog.patterns.map(pattern => `<details class="pattern-catalog-entry"><summary><b>${escape(pattern.reference)}</b><span>${escape(pattern.source)}</span></summary><p>${escape(pattern.documentation)}</p><p>${pattern.rules ?? 'Unreported'} Rules · ${pattern.extensions ?? 'Unreported'} extensions${pattern.coverage?.length ? ` · ${escape(pattern.coverage.join(', '))}` : ''}</p>${pattern.digest ? `<code>${escape(pattern.digest)}</code>` : ''}<p class="evidence-meta">${installations.some(i => i.pattern === pattern.reference) ? 'Configured by extends in this repository.' : 'Available to resolve; not installed by viewing this entry.'}</p><button data-copy="${escape(pattern.reference)}">Copy exact reference</button></details>`).join('') || '<p>No offline Patterns returned.</p>'}<p class="evidence-meta">Edit extends and Bindings to adopt an available Pattern. Imported browser references remain separate from repository configuration.</p>`);
      }
      bindFocus(); status();
    } catch (failure) { if (ticket === requestGeneration) evidence(`${sectionNames[section]} unavailable`, `<p role="alert">${error(failure)}</p>`); }
  }
  /** Evaluation owns the evidence generation; presentation never decides whether a result is current. */
  function evaluateRepository(isCurrent?: () => boolean): Promise<CheckEvaluation> {
    if (pendingCheck) { if (isCurrent) pendingCheckGuards.push(isCurrent); return pendingCheck; }
    if (busy) return Promise.resolve({ state: 'busy', message: 'Repository loading is still in progress. Retry the check when it finishes.' });
    const guards = isCurrent ? [isCurrent] : [];
    pendingCheckGuards = guards;
    busy = true; $('#run-repository-check').setAttribute('disabled', '');
    // Assign the shared promise before callbacks can request another check.
    pendingCheck = Promise.resolve().then(async (): Promise<CheckEvaluation> => {
      let failureMessage: string | undefined;
      try {
        callbacks.checkingChanged?.(true);
        const before = await queryRepositoryRevision().catch(() => null);
        const source = await reloadRepositoryEvidence(), generation = evidenceGeneration;
        const result = await checkRepository();
        const after = before ? await queryRepositoryRevision().catch(() => null) : null;
        if (before && (!after || before.revision !== after.revision)) {
          failureMessage = 'Repository files changed during inspection. Updating the evidence.';
          return {state:'stale',message:failureMessage};
        }
        if (!guards.every(guard => guard())) return { state: 'superseded' };
        if (generation !== evidenceGeneration || source !== repository) {
          failureMessage = 'Repository evidence changed during this check. The result was discarded.';
          return { state: 'stale', message: failureMessage };
        }
        run = result; status(); callbacks.evidenceChanged?.(repository,run);
        return { state: 'reported' };
      } catch (failure) {
        failureMessage = failure instanceof Error ? failure.message : String(failure);
        return { state: 'failed', message: failureMessage };
      } finally {
        pendingCheck = null; pendingCheckGuards = []; busy = false; $('#run-repository-check').removeAttribute('disabled');
        callbacks.checkingChanged?.(false, failureMessage);
        if (run) renderReportSurface();
        else {
          const surface = $('#report-surface');
          surface?.setAttribute('aria-busy', 'false');
          const note = surface?.querySelector<HTMLElement>('#report-updating');
          if (note && failureMessage) { note.hidden = false; note.textContent = `Inspection unavailable: ${failureMessage} Retrying automatically.`; }
        }
      }
    });
    return pendingCheck;
  }
  async function checkQuietly(isCurrent?: () => boolean) {
    const result = await evaluateRepository(isCurrent);
    if (result.state === 'failed' || result.state === 'stale' || result.state === 'busy') throw new Error(result.message ?? 'The repository check could not complete.');
  }
  async function checkCode() {
    if (busy && !pendingCheck) return;
    activeSection = 'findings';
    const ticket = ++requestGeneration;
    evidence('Checking repository', '<p>ArcLint is observing code on disk and evaluating its configured Rules.</p><p>Browser model edits are not an input to this check.</p>');
    const result = await evaluateRepository();
    if (ticket !== requestGeneration) return;
    if (result.state === 'reported') await showGovernance('findings');
    else if (result.state === 'stale' || result.state === 'superseded') {
      evidence('Check needs refreshing', `<p>${escape(result.message ?? 'Source changed while checking. A current check is required.')}</p><button id="recheck-current-source">Check code →</button>`);
      $('#recheck-current-source').addEventListener('click', () => void checkCode());
    } else if (result.state === 'failed') evidence('Check could not complete', `<p role="alert">${escape(result.message)}</p>${run ? '<p>The previous completed Report remains available in Report; this failed attempt did not replace it.</p>' : ''}`);
  }
  async function reviewBaselineAdoption() {
    activeSection = 'findings'; const ticket = ++requestGeneration;
    evidence('Review Baseline adoption', '<p>Running a complete repository check without Baseline subtraction. This review does not acknowledge findings yet.</p>');
    try {
      const preview = await previewRepositoryBaseline(); if (ticket !== requestGeneration) return;
      const findings = preview.report.diagnostics.filter(d => d.status === 'active');
      const suppressed = preview.report.diagnostics.filter(d => d.status === 'suppressed').length;
      evidence('Review Baseline adoption', `${boundSource()}<h3>Acknowledge ${preview.findings} current findings</h3><p>${preview.action === 'refresh' ? 'Replace the existing Baseline with the current findings, dropping stale entries.' : 'Capture the current findings in the repository Baseline.'} Adoption acknowledges debt; it does not repair code or change domain meanings.</p><p class="evidence-meta">${preview.rules} exact-assurance Rules · ${suppressed} suppressed findings remain suppressed and are not adopted. Coverage and operational gaps block this action.</p><p class="evidence-meta">The server rechecks document hashes and repeats this assessment before invoking ArcLint. The native capture command then scans again; repository source is not frozen during that final command.</p><code>${escape(preview.filename)}</code><div class="baseline-review-findings">${findings.map(d => `<article class="diagnostic-record"><div><b>${escape(d.ruleId)}</b><span>${escape(d.severity)}</span></div><p>${escape(d.message)}</p>${d.path ? `<code>${escape(d.path)}${d.line ? `:${d.line}` : ''}</code>` : ''}</article>`).join('') || '<p>No active findings would be adopted.</p>'}</div><div class="document-actions"><button id="apply-baseline-adoption" class="primary">${preview.action === 'refresh' ? 'Replace Baseline' : 'Capture Baseline'} →</button><button id="cancel-baseline-adoption">Return to Report</button></div>`);
      $('#apply-baseline-adoption').onclick = () => void applyBaselineAdoption(preview);
      $('#cancel-baseline-adoption').onclick = () => void showGovernance('findings');
    } catch (failure) { if (ticket === requestGeneration) evidence('Baseline adoption unavailable', `<p role="alert">${error(failure)}</p><p>No Baseline action was performed. The existing Report remains available.</p>`); }
  }
  async function applyBaselineAdoption(preview: RepositoryBaselinePreview) {
    ($('#apply-baseline-adoption') as HTMLButtonElement).disabled = true; const ticket = ++requestGeneration;
    clearReportEvidence();
    try {
      const result = await applyRepositoryBaseline(preview.token);
      if (ticket === requestGeneration) {
        evidence('Baseline updated', `<h3>${result.findings} findings acknowledged</h3><p>ArcLint ${escape(result.action)} recorded ${result.rules} applied Rules${typeof result.removedStale === 'number' ? ` and removed ${result.removedStale} stale occurrences` : ''}.</p>${result.findings !== preview.findings ? '<p role="alert">The native command reported a different adopted count than the preview. Source may have changed during its final assessment; inspect the new Report.</p>' : ''}<p>Adoption changed the Baseline, not the code. Inspection updates automatically to show which findings remain active.</p><button id="check-after-adoption" class="primary">Check code →</button>`);
        $('#check-after-adoption').onclick = () => void checkCode();
      }
      callbacks.notify(`ArcLint acknowledged ${result.findings} findings in the Baseline.`);
    } catch (failure) {
      if (ticket === requestGeneration) { evidence('Baseline adoption not confirmed', `<p role="alert">${error(failure)}</p><button id="retry-baseline-review">Review current findings →</button>`); $('#retry-baseline-review').onclick = () => void reviewBaselineAdoption(); }
    } finally { clearReportEvidence(); callbacks.inspectionInvalidated?.(); }
  }
  function sourceHash(repo: RepositoryProject, document: RepositoryDocument): string | null {
    if (!repo.documentHashes) throw new Error('Reload the local server to use version-checked document saving.');
    return repo.documentHashes[document];
  }
  async function editPolicy(message = '') {
    activeSection = 'rules';
    try {
      const repo = await ensureRepository();
      policyDraft ??= { content: repo.rulesYaml ?? '', expectedHash: sourceHash(repo, 'rules') };
      evidence('Edit repository policy', `${boundSource()}<p>Edit <code>rules.arclint.yaml</code>: Rules, Zones, and the Bindings under <code>extends</code>. A material Constraint change needs a new Rule ID.</p>${message ? `<p class="form-error" role="alert">${escape(message)}</p>` : ''}<form id="policy-editor-form"><label for="policy-yaml">Rules YAML</label><textarea id="policy-yaml" spellcheck="false" aria-describedby="policy-save-help">${escape(policyDraft.content)}</textarea><p id="policy-save-help" class="evidence-meta">Review validates the candidate with ArcLint and shows the exact changes. Applying writes this file to the bound repository.</p><div class="document-actions"><button type="submit" class="primary">Review changes →</button><button type="button" id="discard-policy-draft">Discard draft</button><button type="button" id="review-latest-policy">Compare with current file</button></div></form>`);
      $('#policy-yaml').oninput = event => { policyDraft!.content = (event.target as HTMLTextAreaElement).value; };
      $('#policy-editor-form').onsubmit = event => { event.preventDefault(); void reviewDocument('rules', policyDraft!.content, policyDraft!.expectedHash); };
      $('#discard-policy-draft').onclick = () => { policyDraft = null; void showGovernance('rules'); };
      $('#review-latest-policy').onclick = () => void reviewCurrentDocument('rules', policyDraft!.content);
    } catch (failure) { evidence('Policy editing unavailable', `<p role="alert">${error(failure)}</p>`); }
  }
  function documentDiff(before: string | null, after: string) {
    const old = before === null ? [] : before.split('\n'), next = after.split('\n');
    let prefix = 0, suffix = 0;
    while (prefix < old.length && prefix < next.length && old[prefix] === next[prefix]) prefix++;
    while (suffix < old.length - prefix && suffix < next.length - prefix && old[old.length - suffix - 1] === next[next.length - suffix - 1]) suffix++;
    const start = Math.max(0, prefix - 3), removed = old.slice(prefix, old.length - suffix), added = next.slice(prefix, next.length - suffix);
    const rows = [...old.slice(start, prefix).map(line => `<span> ${escape(line)}</span>`), ...removed.map(line => `<span class="diff-remove">−${escape(line)}</span>`), ...added.map(line => `<span class="diff-add">+${escape(line)}</span>`), ...old.slice(old.length - suffix, old.length - suffix + 3).map(line => `<span> ${escape(line)}</span>`)];
    return { removed: removed.length, added: added.length, html: rows.join('\n') };
  }
  async function reviewCurrentDocument(document: RepositoryDocument, content: string) {
    try { const source = await reloadRepositoryEvidence(); const hash = sourceHash(source, document); if (document === 'rules' && policyDraft) policyDraft.expectedHash = hash; await reviewDocument(document, content, hash); }
    catch (failure) { evidence('Could not refresh the source', `<p role="alert">${error(failure)}</p>`); }
  }
  async function reviewDocument(document: RepositoryDocument, content: string, expectedHash: string | null) {
    const ticket = ++requestGeneration;
    evidence('Validating document', '<p>ArcLint is loading the candidate Rules and Domain in an isolated workspace. The repository documents have not changed.</p>');
    try {
      const preview = await previewRepositoryDocument(document, content, expectedHash);
      if (ticket !== requestGeneration) return;
      const diff = documentDiff(preview.before, preview.after);
      evidence(`Review ${preview.filename}`, `${boundSource()}<p class="document-validation">${escape(preview.validation.message)}</p><div class="document-review-heading"><span>${diff.added} added lines · ${diff.removed} removed lines</span><code>${escape(preview.filename)}</code></div><pre class="document-diff" aria-label="Proposed document changes">${diff.html || 'No content changes.'}</pre><details><summary>Complete proposed YAML</summary><pre>${escape(preview.after)}</pre></details><div class="document-actions"><button id="apply-document" class="primary" ${preview.before === preview.after ? 'disabled' : ''}>Apply to repository →</button><button id="back-document-editor">${document === 'rules' ? 'Continue editing' : 'Return to model'}</button></div><p class="evidence-meta">This writes ${escape(preview.filename)}. The server checks that Rules and Domain still match this preview. Inspection updates automatically after applying.</p>`);
      $('#back-document-editor').onclick = () => { if (document === 'rules') void editPolicy(); else closeEvidence(); };
      $('#apply-document').onclick = () => void applyDocument(preview);
    } catch (failure) {
      if (ticket !== requestGeneration) return;
      if (document === 'rules') await editPolicy(failure instanceof Error ? failure.message : String(failure));
      else { evidence('Domain could not be prepared', `<p role="alert">${error(failure)}</p><p>The model draft is retained. Resolve invalid entries in the model, or compare it with the current repository document.</p><button id="review-latest-domain">Compare with current file</button>`); $('#review-latest-domain').onclick = () => void reviewCurrentDocument('domain', content); }
    }
  }
  async function applyDocument(preview: RepositoryDocumentPreview) {
    const button = $('#apply-document') as HTMLButtonElement; button.disabled = true;
    const ticket = ++requestGeneration;
    clearRepositoryEvidence();
    try {
      const result = await applyRepositoryDocument(preview.token);
      if (result.document === 'rules') policyDraft = null;
      if (ticket === requestGeneration) evidence(`${result.filename} saved`, `<p>The reviewed document was applied to the bound repository.</p><p class="evidence-meta">Saved ${escape(new Date(result.savedAt).toLocaleString())}. Previous inspection and Report caches were cleared.</p><div class="document-actions"><button id="check-saved-document" class="primary">Check code →</button><button id="return-from-save">Return to model</button></div>`);
      $('#check-saved-document')?.addEventListener('click', () => void checkCode());
      $('#return-from-save')?.addEventListener('click', closeEvidence);
      callbacks.notify(`Saved ${result.filename} to the repository.`); status();
    } catch (failure) {
      if (ticket === requestGeneration) {
        const body = card.querySelector('.evidence-body')!;
        body.insertAdjacentHTML('beforeend', `<p class="form-error" role="alert">${error(failure)}</p><p>The model and editor draft are retained.</p><button id="review-latest-after-failure">Compare with current file</button>`);
        $('#review-latest-after-failure').onclick = () => void reviewCurrentDocument(preview.document, preview.after);
      }
    } finally { clearRepositoryEvidence(); callbacks.inspectionInvalidated?.(); }
  }
  async function saveDomain(domainYaml: string) {
    try { const repo = await ensureRepository(); await reviewDocument('domain', domainYaml, sourceHash(repo, 'domain')); }
    catch (failure) { evidence('Domain saving unavailable', `<p role="alert">${error(failure)}</p>`); }
  }
  async function showRepository() {
    closeTools(); if (repositoryOpening) return;
    repositoryOpening = true; $('#open-repository').setAttribute('disabled', '');
    const ticket = ++requestGeneration;
    try {
      // Foreground intent survives a quiet check. Invalidate its pending result,
      // then use the existing execution instead of starting a competing reload.
      const checking = pendingCheck;
      if (checking) { clearRepositoryEvidence(); await checking; }
      busy = true;
      const loaded = await reloadRepositoryEvidence();
      if (ticket !== requestGeneration) return;
      if (!loaded.domainYaml) throw new Error('This repository has no domain.arclint.yaml. Create a model here or import a domain file.');
      const next = importProject(loaded.domainYaml);
      closeEvidence(); callbacks.replace(next); status();
      callbacks.notify(`Loaded ${loaded.repository.name} from disk. Edits remain a browser draft; Undo restores the previous model.`);
    } catch (failure) { if (ticket === requestGeneration) evidence('Repository could not be opened', `<p role="alert">${error(failure)}</p>`); }
    finally { repositoryOpening = false; busy = false; $('#open-repository').removeAttribute('disabled'); callbacks.inspectionInvalidated?.(); }
  }
  $('#open-repository').onclick = () => void showRepository();
  $('#open-architecture').onclick = () => void showGovernance('zones');
  $('#run-repository-check').onclick = () => void checkCode();
  $('#workbench-form').onsubmit = event => {
    event.preventDefault(); closeTools();
    const text = ($('#workbench-input') as HTMLInputElement).value.trim(); if (!text) return;
    const matches = [...project.contexts, ...project.concepts].filter(item => item.name.toLocaleLowerCase() === text.toLocaleLowerCase());
    if (matches.length === 1) { closeEvidence(); callbacks.select(matches[0].id); return; }
    if (matches.length > 1) {
      evidence('Choose the context', matches.map(item => `<button class="path-button" data-match="${escape(item.id)}">${escape(item.name)} · ${escape('contextId' in item ? project.contexts.find(c => c.id === item.contextId)?.name : 'context')}</button>`).join(''));
      card.querySelectorAll<HTMLElement>('[data-match]').forEach(b => b.onclick = () => { closeEvidence(); callbacks.select(b.dataset.match!); }); return;
    }
    if (text.includes('/') || /\.[a-z\d]+$/i.test(text)) { void inspect(text); return; }
    evidence('No matching term', `<p>No recorded term is named <b>${escape(text)}</b>. Enter a repository path or add the missing meaning to the model.</p><button id="create-from-workbench">Add a term</button>`);
    $('#create-from-workbench').onclick = () => { closeEvidence(); callbacks.create(); };
  };
  function setRepresentation(next: Representation) {
    closeTools(); representation = next; host.dataset.representation = next;
    for (const item of ['site', 'plan', 'matrix']) $(`#view-${item}`).setAttribute('aria-pressed', String(item === next));
    $('#projection').hidden = next === 'site'; renderDrawing();
  }
  $('#view-site').onclick = () => setRepresentation('site'); $('#view-plan').onclick = () => setRepresentation('plan'); $('#view-matrix').onclick = () => setRepresentation('matrix');
  function renderDrawing() {
    if (!project || representation === 'site') return;
    const objects = [...project.contexts, ...project.concepts].filter(c => focusView?.ids.includes(c.id));
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
      panel.innerHTML = `<div class="matrix-drawing"><div class="drawing-heading"><h2>Relationships</h2><p>Read from row to column.</p><div class="matrix-pagination">${pager('row',matrixRowPage)}${pager('column',matrixColumnPage)}</div></div><table><caption>Same model · ${objects.length} objects · ${relations.length} relationships</caption><thead><tr><th scope="col">FROM ↓ / TO →</th>${columns.map(object => `<th scope="col"><button data-model-select="${escape(object.id)}">${escape(object.name)}</button></th>`).join('')}</tr></thead><tbody>${rows.map(source => `<tr><th scope="row"><button data-model-select="${escape(source.id)}">${escape(source.name)}</button></th>${columns.map(target => `<td ${source.id === target.id ? 'class="matrix-self"' : ''}>${(cells.get(JSON.stringify([source.id,target.id])) ?? []).map(r => `<button data-matrix-relation="${escape(r.id)}" aria-label="${escape(`${name(r.source)} ${r.label} ${name(r.target)}`)}">${escape(relationshipDescription(project, r).label)}</button>`).join('')}</td>`).join('')}</tr>`).join('')}</tbody></table></div>`;
    } else {
      const coordinates = new Map([...project.contexts, ...objects].map(object => [object.id, planPosition(object.position)]));
      const contexts = project.contexts.filter(c => ids.has(c.id) || objects.some(item => 'contextId' in item && item.contextId === c.id));
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
      panel.innerHTML = `<div class="drawing-heading"><h2>Plan</h2><p>Same place. Saved positions.</p></div><div class="plan-drawing" style="width:${width}px;height:${Math.max(height,...edgeLabels.map(p=>p.y+60))}px">${contexts.map(c => { const box=bounds.get(c.id)!, p=point(c.id); return `<div class="plan-context-boundary" style="left:${box.x-minX}px;top:${box.y-minY}px;width:${box.right-box.x}px;height:${box.bottom-box.y}px"></div>${ids.has(c.id) ? `<button class="plan-object context ${selected === c.id ? 'is-selected' : ''}" data-model-select="${escape(c.id)}" style="left:${p.x}px;top:${p.y}px"><b>${escape(c.name)}</b></button>` : ''}`; }).join('')}<svg class="plan-lines" width="${width}" height="${Math.max(height,...edgeLabels.map(p=>p.y+60))}" aria-hidden="true"><defs><marker id="plan-arrow" orient="auto-start-reverse" markerWidth="8" markerHeight="8" refX="7" refY="4"><path d="M0 0 8 4 0 8" fill="#526863"/></marker></defs>${relations.map(r => { const {a,b}=clipped(r); const direction=relationshipDescription(project,r).direction; return `<path d="M${a.x} ${a.y} L${b.x} ${b.y}" ${direction === 'none' ? 'stroke-dasharray="3 8"' : 'marker-end="url(#plan-arrow)"'} ${direction === 'both' ? 'marker-start="url(#plan-arrow)"' : ''}/>`; }).join('')}${edgeLabels.map(({r,x,y})=>{ const a=point(r.source),b=point(r.target); return `<path class="plan-leader" d="M${(a.x+b.x)/2} ${(a.y+b.y)/2} L${x} ${y}"/>`; }).join('')}${mode === 'baseline' && baseline ? conceptObjects.flatMap(c => { const old = baseline!.project.concepts.find(o => o.id === c.id); if (!old || JSON.stringify(old.position) === JSON.stringify(c.position)) return []; const a = planPosition(old.position), b = point(c.id); return [`<path class="movement-trace" d="M${a.x - minX} ${a.y - minY} L${b.x} ${b.y}" marker-end="url(#plan-arrow)"/>`]; }).join('') : ''}</svg>${conceptObjects.map(object => { const p=point(object.id), contracts=conceptContracts(project,object); return `<button class="plan-object ${object.kind} ${selected === object.id ? 'is-selected' : ''}" data-model-select="${escape(object.id)}" style="left:${p.x}px;top:${p.y}px"><b>${escape(object.name)}</b></button>`; }).join('')}${edgeLabels.map(({r,x,y}) => { const info=relationshipDescription(project,r); return `<button class="plan-relation" data-plan-relation="${escape(r.id)}" style="left:${x}px;top:${y}px" aria-label="${escape(`${name(r.source)} ${r.label} ${name(r.target)}`)}" title="${escape(info.meaning)}">${escape(info.label)} ${info.direction === 'both' ? '↔' : info.direction === 'none' ? '∥' : '→'}</button>`; }).join('')}</div>`;
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
  async function inspectSelection(id: string | null) {
    if (!id) { await showGovernance('rules'); return; }
    const concept = project.concepts.find(c => c.id === id);
    const context = project.contexts.find(c => c.id === (concept?.contextId ?? id));
    const ticket = ++requestGeneration;
    evidence(`Inspecting ${concept?.name ?? context?.name ?? 'code'}`, '<p>Reading the repository’s recorded source anchors…</p>');
    try {
      const repo = await ensureRepository();
      if (ticket !== requestGeneration) return;
      const association = buildArchitectureEvidence(project, repo).contexts.find(item => item.id === context?.id);
      const anchors = concept ? association?.subjects.find(item => item.id === concept.id)?.anchors ?? [] : association?.anchors ?? [];
      const paths = [...new Set(anchors.map(anchor => anchor.path))];
      if ((!concept && paths.length) || paths.length > 1) {
        evidence(`Located code for ${concept?.name ?? context?.name ?? 'context'}`, `<p>These are located contract anchors. They do not establish the entire code footprint.</p>${paths.map(path => `<button class="path-button" data-inspect-path="${escape(path)}">${escape(path)} ↗</button>`).join('')}`); return;
      }
      if (paths[0]) { await inspect(paths[0]); return; }
      evidence(`Inspect ${concept?.name ?? context?.name ?? 'code'}`, `<p>This model has no located code anchor for this selection. Enter the repository path you want ArcLint to inspect.</p><form id="selection-path-form"><label>Repository path<input id="selection-code-path" name="path" placeholder="internal/example/root.go" required /></label><button type="submit">Inspect path ↗</button></form>`);
      $('#selection-path-form').onsubmit = event => { event.preventDefault(); void inspect(($('#selection-code-path') as HTMLInputElement).value.trim()); };
      $('#selection-code-path').focus();
    } catch (failure) { if (ticket === requestGeneration) evidence('Inspection unavailable', `<p role="alert">${error(failure)}</p>`); }
  }
  function selectionEvidence(id: string): string {
    const concept = project?.concepts.find(c => c.id === id);
    if (!concept) return '';
    const contracts = conceptContracts(project, concept);
    const anchors = buildArchitectureEvidence(project, repository).contexts.find(context => context.id === concept.contextId)?.subjects.find(subject => subject.id === concept.id)?.anchors ?? [];
    return `<section class="recorded-contracts"><h3>Recorded contracts</h3><p class="field-help">These state what must hold. Code inspection supplies separate evidence.</p><details ${contracts.invariants.length ? 'open' : ''}><summary>Invariants · ${contracts.invariants.length}</summary>${contracts.invariants.map(i => `<article><b>╳ ${escape(i.key)}</b><p>${escape(i.statement)}</p></article>`).join('') || '<p>No invariants recorded.</p>'}</details><details ${contracts.assertions.length ? 'open' : ''}><summary>Assertions · ${contracts.assertions.length}</summary>${contracts.assertions.map(a => `<article><b>⊣ After ${escape(a.operation)}</b><code>${escape(a.key)}</code><p>${escape(a.statement)}</p></article>`).join('') || '<p>No operation post-conditions recorded.</p>'}</details><h3>Code evidence</h3>${anchors.map(a => `<button class="path-button" data-inspect-path="${escape(a.path)}">Contract anchor: ${escape(a.source)} ↗</button>`).join('') || '<p class="field-help">No located contract anchor for this term. Enter a repository path above to inspect its governing Rules.</p>'}</section>`;
  }
  function bindSelection() { document.querySelectorAll<HTMLElement>('#inspector [data-inspect-path]').forEach(b => b.onclick = () => void inspect(b.dataset.inspectPath!)); }
  return { update(next, id, contextId, nextMode, pointOfReference, view) { focusView = view; project = next; selected = id; scope = contextId; mode = nextMode; baseline = pointOfReference; purpose(); status(); renderDrawing(); }, inspectSelection: id => { void inspectSelection(id); }, selectionEvidence, bindSelection, showGovernance: section => { void showGovernance(section); }, checkCode: () => { void checkCode(); }, checkQuietly, showRepository: () => { void showRepository(); }, getRepository: () => repository ? structuredClone(repository) : null, getReport: () => run ? structuredClone(run) : null, saveDomain: yaml => { void saveDomain(yaml); }, inspectPath: path => { void inspect(path); }, inspectZone: zone => { void inspect(zone,true); }, showRule: id => { void ruleDetail(id); }, initializeRepository(repo) { if (repositoryOpening || busy || repository || pendingRepository || evidenceGeneration || requestGeneration) return false; evidenceGeneration++; repository = repo; status(); return true; }, receiveEvidence(repo,result) { evidenceGeneration++; pendingRepository = null; repository = repo; run = result; inspected = null; patternCatalog = null; status(); } };
}
