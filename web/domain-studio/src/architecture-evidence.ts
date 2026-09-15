import { parse } from 'yaml';
import type { Concept, DomainProject } from './contracts';
import { exportDomainYaml } from './serialization';
import { diagnosticCounts, queryRepositoryContext, type DiagnosticFilter, type RepositoryCheckResult, type RepositoryContextResult, type RepositoryContextRule, type RepositoryDiagnostic, type RepositoryDomainContext, type RepositoryProject, type RepositoryDependencies } from './repository';

/** A located contract is source evidence, not proof that its invariant holds. */
export interface ArchitectureAnchor {
  path: string; line?: number; source: string; key: string; statement: string;
  kind: 'invariant' | 'assertion' | 'specification'; owner?: string;
  membership: 'unqueried' | 'reported' | 'unavailable' | 'missing';
  zones: string[]; reason?: string; rules: RepositoryContextRule[];
}
export type ArchitectureLocationState = 'located' | 'unlocated' | 'unlinked' | 'unavailable';
export interface ArchitectureSubject {
  id: string; name: string; kind: Concept['kind']; ownerId?: string;
  state: ArchitectureLocationState; anchors: ArchitectureAnchor[]; zones: string[];
  /** Returned diagnostic occurrences at these anchor paths, not a per-term outcome. */
  diagnostics: RepositoryDiagnostic[];
}
export interface ArchitectureContext {
  id: string; name: string; state: ArchitectureLocationState;
  subjects: ArchitectureSubject[]; anchors: ArchitectureAnchor[]; zones: string[];
  /** Each returned occurrence appears once even when subjects share an anchor file. */
  diagnostics: RepositoryDiagnostic[];
}
export interface ArchitectureZone {
  name: string; description: string; paths: string[]; internal: string[];
  /** One import contract reported by context, not combined permission across every Rule. */
  internalRestricted: boolean; external: string; stdlib: string;
  /** All observed file memberships, including overlaps; null means not observed. */
  observedPaths: string[] | null;
}
export interface ArchitectureLayerRule {
  id: string; zones: string[]; severity: string; disabled: boolean;
  proposition: string; source: 'rules.arclint.yaml';
}
export interface ArchitectureEvaluation {
  state: 'not-run' | 'reported'; checkedAt?: string;
  diagnostics: RepositoryDiagnostic[]; counts: Record<DiagnosticFilter, number>;
  outcomesAvailable: boolean;
}
export interface ArchitectureEvidence {
  linked: boolean; linkReason: string; contexts: ArchitectureContext[]; zones: ArchitectureZone[];
  layers: ArchitectureLayerRule[]; unavailableLayerRuleIds: string[];
  evaluation: ArchitectureEvaluation;
  observedImports: ArchitectureObservedImports;
}
export type ArchitectureObservedImports = ({ state: 'reported'; reason: string } & RepositoryDependencies)
  | { state: 'unavailable' | 'loading'; reason: string; files: []; edges: [] };
export type ArchitectureDependencyInput = RepositoryDependencies | { state: 'unavailable' | 'loading'; reason: string } | null;
export type ArchitecturePathResult = RepositoryContextResult | { path: string; error: string };

const record = (value: unknown): Record<string, unknown> => value && typeof value === 'object' && !Array.isArray(value) ? value as Record<string, unknown> : {};
const unique = (values: readonly string[]): string[] => [...new Set(values)];
function ordered(value: unknown): unknown {
  if (Array.isArray(value)) return value.map(ordered);
  if (value && typeof value === 'object') return Object.fromEntries(Object.entries(value).sort(([a], [b]) => a.localeCompare(b)).map(([key, entry]) => [key, ordered(entry)]));
  return value;
}

/** Compare the complete current domain export; canvas positions are not domain data. */
export function matchesArchitectureSource(project: DomainProject, repository: RepositoryProject | null): boolean {
  if (!repository?.domainYaml) return false;
  try { return JSON.stringify(ordered(parse(repository.domainYaml))) === JSON.stringify(ordered(parse(exportDomainYaml(project)))); }
  catch { return false; }
}

function sourceLocation(source: unknown): { path: string; line?: number; source: string } | null {
  if (typeof source !== 'string' || !source || source.trim() !== source) return null;
  const match = /^(.*):([1-9]\d*)$/.exec(source);
  const path = match?.[1] ?? source;
  // Match the existing path endpoint's literal repository path boundary.
  if (path.startsWith('/') || path.startsWith('-') || path.split('/').includes('..') || /[\x00-\x1f\\:*?{}]/.test(path)) return null;
  const line = match ? Number(match[2]) : undefined;
  if (line !== undefined && !Number.isSafeInteger(line)) return null;
  return { path, ...(line === undefined ? {} : { line }), source };
}

type LocatedContract = { path: string; line?: number; source: string; key: string; statement: string; kind: ArchitectureAnchor['kind']; owner?: string };
function locatedContracts(context: RepositoryDomainContext, subject?: Concept): LocatedContract[] {
  const contracts: LocatedContract[] = [];
  const append = (raw: unknown, kind: ArchitectureAnchor['kind']) => {
    const item = record(raw);
    if (item.anchor !== 'found') return;
    const location = sourceLocation(item.source);
    if (!location) return;
    if (subject) {
      if (kind === 'specification') { if (subject.kind !== 'specification' || item.name !== subject.name) return; }
      else if (item.owner !== subject.name || (kind === 'assertion' ? subject.kind !== 'aggregate' : item.ownerConcept !== subject.kind)) return;
    }
    contracts.push({ ...location, kind, key: String(item.key ?? item.name ?? ''), statement: typeof item.statement === 'string' ? item.statement : '', ...(typeof item.owner === 'string' ? { owner: item.owner } : {}) });
  };
  for (const item of context.invariants ?? []) append(item, 'invariant');
  for (const item of context.assertions ?? []) append(item, 'assertion');
  for (const item of context.specifications ?? []) append(item, 'specification');
  return contracts;
}

/** These are CLI-located contract paths, not a recursive listing of context code. */
export function architectureAnchorPaths(repository: RepositoryProject): string[] {
  if (repository.context.domain?.located !== true) return [];
  return unique((repository.context.domain?.contexts ?? []).flatMap(context => locatedContracts(context).map(anchor => anchor.path))).sort();
}

/** Reserve two bridge slots for foreground actions, including the two-command repository load. */
export async function loadArchitecturePaths(repository: RepositoryProject, signal?: AbortSignal,
  query: (path: string, signal?: AbortSignal) => Promise<RepositoryContextResult> = queryRepositoryContext,
  options: { progress?(results: ArchitecturePathResult[]): void; priority?(): string[] } = {}): Promise<ArchitecturePathResult[]> {
  const paths = architectureAnchorPaths(repository), results: ArchitecturePathResult[] = new Array(paths.length);
  const remaining = paths.map((_,index) => index);
  const worker = async () => {
    while (remaining.length) {
      const priority = options.priority?.() ?? [];
      const urgent = priority.map(path => remaining.findIndex(index => paths[index] === path)).find(index => index !== -1);
      const index = remaining.splice(urgent ?? 0,1)[0], path = paths[index];
      if (signal?.aborted) throw signal.reason ?? new Error('Architecture inspection was cancelled.');
      try { results[index] = await query(path, signal); }
      catch (error) {
        if (signal?.aborted) throw error;
        results[index] = { path, error: error instanceof Error ? error.message : String(error) };
      }
      options.progress?.(results.filter(Boolean));
    }
  };
  await Promise.all(Array.from({ length: Math.min(2, paths.length) }, worker));
  return results;
}

function anchorEvidence(anchor: LocatedContract, paths: ReadonlyMap<string, ArchitecturePathResult>, observedFiles: ReadonlyMap<string, RepositoryDependencies['files'][number]>): ArchitectureAnchor {
  const result = paths.get(anchor.path);
  const base = { ...anchor, zones: [], rules: [] };
  if (!result) {
    const observed = observedFiles.get(anchor.path);
    if (observed) return { ...base, membership: 'reported', zones: unique(observed.zones), reason: 'File membership is reported by the repository observation; applicable Rules are still being queried.' };
    return { ...base, membership: 'unqueried', reason: 'Zone membership has not been queried for this located anchor.' };
  }
  if ('error' in result) return { ...base, membership: 'unavailable', reason: result.error };
  if (result.pathType === 'missing') return { ...base, membership: 'missing', reason: 'The located path is now missing. Its hypothetical policy does not establish file membership.' };
  if (result.pathType !== 'file') return { ...base, membership: 'unavailable', reason: result.pathType === 'directory' ? 'The located contract path is a directory, not a verified source file.' : 'This report does not establish that the located path is a source file.' };
  const binding = result.report.Paths?.find(binding => binding.Path === anchor.path);
  if (!binding) return { ...base, membership: 'unavailable', reason: 'ArcLint did not return Zone membership for this exact source path.' };
  return { ...anchor, membership: 'reported', zones: unique(binding.Zones ?? []), rules: [...result.report.Rules ?? []] };
}

function layers(repository: RepositoryProject | null): { layers: ArchitectureLayerRule[]; unavailableLayerRuleIds: string[] } {
  if (!repository) return { layers: [], unavailableLayerRuleIds: [] };
  let authored: Record<string, unknown> = {};
  try { authored = record(record(parse(repository.rulesYaml ?? '')).rules); } catch { /* Loaded CLI Rules remain inspectable; no order is inferred. */ }
  const declarations: ArchitectureLayerRule[] = [];
  for (const rule of repository.rules.filter(rule => rule.type === 'layers')) {
    const order = record(authored[rule.id]).layers;
    if (!Array.isArray(order) || !order.length || !order.every(zone => typeof zone === 'string')) continue;
    declarations.push({ id: rule.id, zones: [...order], severity: rule.severity, disabled: rule.disabled ?? false, proposition: rule.proposition, source: 'rules.arclint.yaml' });
  }
  return { layers: declarations, unavailableLayerRuleIds: repository.rules.filter(rule => rule.type === 'layers' && !declarations.some(declared => declared.id === rule.id)).map(rule => rule.id) };
}

export function buildArchitectureEvidence(project: DomainProject, repository: RepositoryProject | null,
  pathResults: readonly ArchitecturePathResult[] = [], check: RepositoryCheckResult | null = null,
  dependencies: ArchitectureDependencyInput = null): ArchitectureEvidence {
  const linked = matchesArchitectureSource(project, repository);
  const located = repository?.context.domain?.located === true;
  const baseState: ArchitectureLocationState = !repository ? 'unavailable' : !linked ? 'unlinked' : !located ? 'unavailable' : 'unlocated';
  const paths = new Map(pathResults.map(result => [result.path, result]));
  const diagnostics = [...check?.diagnostics ?? []];
  const observedImports: ArchitectureObservedImports = !dependencies ? { state: 'unavailable', reason: 'Repository imports have not been observed.', files: [], edges: [] }
    : 'state' in dependencies ? { ...dependencies, files: [], edges: [] }
    : { ...dependencies, state: 'reported', reason: dependencies.changedDuringObservation ? 'Repository files changed during observation. Refresh before comparing dependencies.' : !dependencies.coverage.complete ? 'Some source imports could not be observed. This graph is incomplete.' : 'Observed imports within the configured scan scope; runtime dependencies are not included.' };
  const observedFiles = new Map(observedImports.files.map(file => [file.path, file]));
  const contexts: ArchitectureContext[] = project.contexts.map(context => {
    const reported = linked && located ? repository?.context.domain?.contexts?.find(candidate => candidate.name === context.name) : undefined;
    const subjects: ArchitectureSubject[] = project.concepts.filter(subject => subject.contextId === context.id).map(subject => {
      const anchors = reported ? locatedContracts(reported, subject).map(anchor => anchorEvidence(anchor, paths, observedFiles)) : [];
      const sourcePaths = new Set(anchors.filter(anchor => anchor.membership === 'reported').map(anchor => anchor.path));
      return { id: subject.id, name: subject.name, kind: subject.kind, ...(subject.ownerId ? { ownerId: subject.ownerId } : {}),
        state: anchors.length ? 'located' : baseState, anchors, zones: unique(anchors.flatMap(anchor => anchor.zones)),
        diagnostics: diagnostics.filter(diagnostic => diagnostic.path !== undefined && sourcePaths.has(diagnostic.path)) };
    });
    const anchors = subjects.flatMap(subject => subject.anchors);
    const sourcePaths = new Set(anchors.filter(anchor => anchor.membership === 'reported').map(anchor => anchor.path));
    return { id: context.id, name: context.name, state: anchors.length ? 'located' : baseState, subjects, anchors, zones: unique(anchors.flatMap(anchor => anchor.zones)),
      diagnostics: diagnostics.filter(diagnostic => diagnostic.path !== undefined && sourcePaths.has(diagnostic.path)) };
  });
  return {
    linked, linkReason: !repository ? 'Repository evidence has not been loaded.' : linked ? 'The current domain export matches the repository domain document.' : 'The current model differs from the repository domain document. Repository anchors are not associated with this model.',
    contexts,
    zones: (repository?.context.Zones ?? []).map(zone => ({ name: zone.Name, description: zone.Description, paths: [...zone.Paths ?? []], internal: [...zone.Internal ?? []], internalRestricted: zone.InternalRestricted, external: zone.External, stdlib: zone.Stdlib,
      observedPaths: observedImports.state === 'reported' ? unique(observedImports.files.filter(file => file.zones.includes(zone.Name)).map(file => file.path)).sort() : null })),
    ...layers(repository),
    evaluation: { state: check ? 'reported' : 'not-run', ...(check ? { checkedAt: check.checkedAt } : {}), diagnostics, counts: diagnosticCounts(diagnostics), outcomesAvailable: check?.outcomesAvailable ?? false },
    observedImports,
  };
}

/** Scope by exact, located file paths. An anchor does not establish ownership of its directory. */
export function architectureImportsForSelection(evidence: ArchitectureEvidence, selectedId: string | null, contextId: string | null) {
  const source = evidence.observedImports;
  const internal = source.edges.filter(edge => edge.classification === 'internal' && edge.targetPath !== '');
  if (!selectedId && !contextId) return { edges: internal, anchorPaths: [], scope: 'repository' as const, resolution: source.state === 'reported' ? 'known' as const : source.state === 'loading' ? 'pending' as const : 'unavailable' as const, reason: source.reason };
  const selectedContext = evidence.contexts.find(context => context.id === selectedId);
  const selected = evidence.contexts.flatMap(context => context.subjects).find(subject => subject.id === selectedId);
  const context = selectedContext ?? evidence.contexts.find(context => context.id === contextId);
  const anchors = selectedId && !selectedContext ? selected?.anchors ?? [] : context?.anchors ?? [];
  const anchorPaths = unique(anchors.filter(anchor => anchor.membership === 'reported').map(anchor => anchor.path));
  const pending = evidence.linked && (source.state === 'loading' || anchors.some(anchor => anchor.membership === 'unqueried'));
  const unresolved = anchors.some(anchor => anchor.membership === 'unavailable' || anchor.membership === 'missing');
  const resolution = pending ? 'pending' as const : evidence.linked && !unresolved && anchorPaths.length > 0 && source.state === 'reported' ? 'known' as const : 'unavailable' as const;
  const paths = new Set(anchorPaths);
  return { edges: internal.filter(edge => paths.has(edge.sourcePath) || edge.targetKind === 'file' && paths.has(edge.targetPath)), anchorPaths, scope: 'located-files' as const, resolution,
    reason: !evidence.linked ? evidence.linkReason : pending ? 'Resolving source files for this selection.' : unresolved ? 'Some located source files could not be resolved.' : !anchorPaths.length ? 'No located source files connect this selection to the observed imports.' : source.state !== 'reported' ? source.reason : 'Imports at the located source files. Directory targets remain directories; domain ownership is not inferred.' };
}
