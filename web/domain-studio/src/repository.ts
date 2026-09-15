/** Actual CLI report shapes. Optional evidence stays absent; it is never inferred from a clean list. */
export interface RepositoryRuleSummary {
  id: string; type: string; severity: string; proposition: string;
  rationale?: string; assurance?: string; provenance?: string; builtIn?: boolean;
  disabled?: boolean; disabledReason?: string;
}
export interface RepositoryZone {
  Name: string; Description: string; Paths: string[] | null; Internal: string[] | null;
  InternalRestricted: boolean; External: string; Stdlib: string;
}
export interface RepositoryContextRule {
  Summary: { ID: string; Type: string; Severity: string; Proposition: string; Rationale?: string;
    Assurance?: string; Provenance?: string; BuiltIn?: boolean; Disabled?: boolean; DisabledReason?: string };
  Reason: string; Via: string[] | null;
}
export interface RepositoryContract {
  key: string; statement: string; owner?: string; ownerConcept?: string;
  source?: string; anchor?: 'found' | 'missing' | 'unanchorable' | string; reason?: string; on?: string;
}
export interface RepositoryDomainContext {
  name: string; aggregates?: { name: string; identity?: string; entities?: string[] }[];
  valueObjects?: string[]; entities?: string[]; events?: string[]; services?: string[];
  invariants?: RepositoryContract[]; assertions?: RepositoryContract[]; specifications?: unknown[];
  [key: string]: unknown;
}
export interface RepositoryContextReport {
  Scope: string; Languages: string[]; RuleCount: number;
  Zones: RepositoryZone[] | null; Rules: RepositoryContextRule[] | null;
  Paths: { Path: string; Zones: string[] | null }[] | null;
  Kinds?: { Kind: string; Meaning: string }[] | null; UnknownImports?: string;
  domain?: {
    source?: string; counts?: Record<string, number>; scoped?: boolean; shown?: Record<string, number>; located?: boolean;
    contexts?: RepositoryDomainContext[];
    relations?: { from: string; to: string; kind: string; description?: string }[];
    unanchored?: { kind: string; context: string; owner?: string; key: string; statement?: string; expected?: string; reason?: string }[];
    [key: string]: unknown;
  };
}
export interface RepositoryDiagnostic {
  kind: string; message: string; ruleId?: string; path?: string; line?: number;
  severity?: string; status?: string; remediation?: string; rationale?: string;
  outcome?: string; assurance?: string;
  [key: string]: unknown;
}
export interface RepositoryProject {
  repository: { name: string; root: string }; domainYaml: string | null; rulesYaml: string | null;
  context: RepositoryContextReport; rules: RepositoryRuleSummary[]; loadedAt: string;
  documentHashes?: { rules: string | null; domain: string | null };
}
export interface RepositoryContextResult { path: string; zone?: string; pathType?: 'file' | 'directory' | 'missing'; report: RepositoryContextReport; queriedAt: string }
export interface RepositoryCheckResult {
  exitCode: 0 | 1; diagnostics: RepositoryDiagnostic[]; checkedAt: string; stderr: string;
  /** True only when the CLI explicitly emits evaluation records with outcomes. */
  outcomesAvailable: boolean;
}
/** Native parsed imports; directory targets are package-granular, never guessed files. */
export interface RepositoryDependencies {
  files: { path: string; zones: string[]; language: string; importsAvailable: boolean }[];
  edges: { sourcePath: string; targetPath: string; targetKind: 'file' | 'directory' | 'unresolved';
    specifier: string; line: number; classification: 'internal' | 'external' | 'stdlib' | 'unknown' | 'cgo';
    sourceZones: string[]; targetZones: string[] }[];
  coverage: { scope: 'repository'; languages: string[]; filesObserved: number; sourceFiles: number; filesWithImports: number; complete: boolean };
  diagnostics: { path?: string; code: string; message: string }[];
  limitations: string[];
  observedAt: string; revision: string; changedDuringObservation: boolean;
}
export interface RepositoryRevision { revision: string; changedAt: string; watching: boolean; reason?: string }
export interface RepositoryRuleDetail {
  summary: RepositoryRuleSummary; evidence?: string; languages?: string[]; facts?: string[];
  entireRepository?: boolean; zones?: string[]; paths?: string[]; limitations?: string[] | string;
  schema?: string; [key: string]: unknown;
}
export interface RepositoryPattern {
  reference: string; namespace: string; name: string; version: string; source: string;
  vendored?: boolean; authored?: boolean; digest?: string; documentation?: string;
  rules?: number; extensions?: number; coverage?: string[];
}
export interface RepositoryPatternCatalog { patterns: RepositoryPattern[]; queriedAt: string }
export interface RepositoryDirectory {
  directory: string; entries: { path: string; name: string; kind: 'file' | 'directory' }[];
  truncated: boolean; queriedAt: string;
}
export type DiagnosticFilter = 'all' | 'active' | 'baselined' | 'suppressed' | 'operational' | 'coverage';
export type RepositoryDocument = 'rules' | 'domain';
export interface RepositoryDocumentPreview {
  token: string; document: RepositoryDocument; filename: string; before: string | null; after: string;
  expectedHash: string | null; proposedHash: string; expiresAt: string;
  validation: { rules: number; domain: boolean; message: string };
}
export interface RepositoryDocumentApplied { document: RepositoryDocument; filename: string; hash: string; savedAt: string }
export interface RepositoryBaselinePreview {
  token: string; action: 'capture' | 'refresh'; filename: string; findings: number; rules: number;
  report: RepositoryCheckResult; expiresAt: string;
}
export interface RepositoryBaselineApplied { action: 'capture' | 'refresh'; findings: number; rules: number; removedStale?: number; savedAt: string }
export class RepositoryError extends Error {
  constructor(message: string, public readonly code: string, public readonly status: number) { super(message); this.name = 'RepositoryError'; }
}
async function request<T>(path: string, method = 'GET', signal?: AbortSignal, body?: unknown, attempt = 0): Promise<T> {
  let response: Response;
  try { response = await fetch(`/api/arclint/${path}`, { method, signal, credentials: 'same-origin', headers: { Accept: 'application/json', 'X-Arclint-Studio': '1', ...(body === undefined ? {} : { 'Content-Type': 'application/json' }) }, ...(body === undefined ? {} : { body: JSON.stringify(body) }) }); }
  catch (error) {
    if (signal?.aborted) throw error;
    throw new RepositoryError('The local ArcLint connection is unavailable. Start Domain Studio with its Vite dev or preview server.', 'CONNECTION_UNAVAILABLE', 0);
  }
  const raw = await response.text();
  let value: unknown;
  try { value = JSON.parse(raw); }
  catch { throw new RepositoryError('This server does not expose the local ArcLint API. Start Domain Studio with npm run dev or npm run preview.', 'API_UNAVAILABLE', response.status); }
  if (!response.ok) {
    const failure = value as { error?: { message?: string; code?: string } };
    // Several open Studio tabs share the bridge. Retry only its explicit busy
    // response to a read; writes always retain their original outcome.
    if (method === 'GET' && response.status === 429 && failure.error?.code === 'BUSY' && attempt < 4) {
      await new Promise<void>((resolve, reject) => {
        if (signal?.aborted) { reject(signal.reason); return; }
        const cancel = () => { clearTimeout(timer); reject(signal?.reason); };
        const timer = setTimeout(() => { signal?.removeEventListener('abort', cancel); resolve(); }, 250 * 2 ** attempt);
        signal?.addEventListener('abort', cancel, { once: true });
      });
      return request<T>(path, method, signal, body, attempt + 1);
    }
    throw new RepositoryError(failure.error?.message ?? 'The ArcLint request failed.', failure.error?.code ?? 'REQUEST_FAILED', response.status);
  }
  return value as T;
}
export const loadRepository = (signal?: AbortSignal) => request<RepositoryProject>('project', 'GET', signal);
export const queryRepositoryDependencies = (signal?: AbortSignal) => request<RepositoryDependencies>('dependencies', 'GET', signal);
export const queryRepositoryRevision = (signal?: AbortSignal) => request<RepositoryRevision>('revision', 'GET', signal);
export const queryRepositoryContext = (path: string, signal?: AbortSignal) => request<RepositoryContextResult>(`context?path=${encodeURIComponent(path)}`, 'GET', signal);
export const queryRepositoryZone = (zone: string, signal?: AbortSignal) => request<RepositoryContextResult>(`context?zone=${encodeURIComponent(zone)}`, 'GET', signal);
export const checkRepository = (signal?: AbortSignal) => request<RepositoryCheckResult>('check', 'POST', signal);
export const queryRepositoryRule = (id: string, signal?: AbortSignal) => request<RepositoryRuleDetail>(`rule?id=${encodeURIComponent(id)}`, 'GET', signal);
export const queryRepositoryPatterns = (signal?: AbortSignal) => request<RepositoryPatternCatalog>('patterns', 'GET', signal);
export const queryRepositoryDirectory = (directory = '.', signal?: AbortSignal) => request<RepositoryDirectory>(`files?directory=${encodeURIComponent(directory)}`, 'GET', signal);
export const previewRepositoryDocument = (document: RepositoryDocument, content: string, expectedHash: string | null) => request<RepositoryDocumentPreview>('documents/preview', 'POST', undefined, { document, content, expectedHash });
export const applyRepositoryDocument = (token: string) => request<RepositoryDocumentApplied>('documents/apply', 'POST', undefined, { token });
export const previewRepositoryBaseline = () => request<RepositoryBaselinePreview>('baseline/preview', 'POST', undefined, {});
export const applyRepositoryBaseline = (token: string) => request<RepositoryBaselineApplied>('baseline/apply', 'POST', undefined, { token });

/** Count returned occurrences, never unique messages or fingerprints. Missing status stays unknown. */
export function diagnosticCounts(diagnostics: readonly RepositoryDiagnostic[]): Record<DiagnosticFilter, number> {
  return Object.fromEntries((['all', 'active', 'baselined', 'suppressed', 'operational', 'coverage'] as const)
    .map(filter => [filter, filterDiagnostics(diagnostics, filter).length])) as Record<DiagnosticFilter, number>;
}
export function filterDiagnostics(diagnostics: readonly RepositoryDiagnostic[], filter: DiagnosticFilter): RepositoryDiagnostic[] {
  return diagnostics.filter(d => filter === 'all' || (filter === 'coverage' || filter === 'operational'
    ? d.kind === filter : d.kind === 'violation' && d.status === filter));
}

export function diagnosticLabel(diagnostic: RepositoryDiagnostic): string {
  // A finding is evidence about its subject, not an invented per-rule evaluation.
  return diagnostic.outcome ?? (diagnostic.kind === 'violation' ? 'Reported finding' : diagnostic.kind);
}
