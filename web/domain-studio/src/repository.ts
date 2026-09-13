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
}
export interface RepositoryContextResult { path: string; zone?: string; report: RepositoryContextReport; queriedAt: string }
export interface RepositoryCheckResult {
  exitCode: 0 | 1; diagnostics: RepositoryDiagnostic[]; checkedAt: string; stderr: string;
  /** True only when the CLI explicitly emits evaluation records with outcomes. */
  outcomesAvailable: boolean;
}
export class RepositoryError extends Error {
  constructor(message: string, public readonly code: string, public readonly status: number) { super(message); this.name = 'RepositoryError'; }
}
async function request<T>(path: string, method = 'GET', signal?: AbortSignal): Promise<T> {
  let response: Response;
  try { response = await fetch(`/api/arclint/${path}`, { method, signal, credentials: 'same-origin', headers: { Accept: 'application/json', 'X-Arclint-Studio': '1' } }); }
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
    throw new RepositoryError(failure.error?.message ?? 'The ArcLint request failed.', failure.error?.code ?? 'REQUEST_FAILED', response.status);
  }
  return value as T;
}
export const loadRepository = (signal?: AbortSignal) => request<RepositoryProject>('project', 'GET', signal);
export const queryRepositoryContext = (path: string, signal?: AbortSignal) => request<RepositoryContextResult>(`context?path=${encodeURIComponent(path)}`, 'GET', signal);
export const queryRepositoryZone = (zone: string, signal?: AbortSignal) => request<RepositoryContextResult>(`context?zone=${encodeURIComponent(zone)}`, 'GET', signal);
export const checkRepository = (signal?: AbortSignal) => request<RepositoryCheckResult>('check', 'POST', signal);

export function diagnosticLabel(diagnostic: RepositoryDiagnostic): string {
  // A finding is evidence about its subject, not an invented per-rule evaluation.
  return diagnostic.outcome ?? (diagnostic.kind === 'violation' ? 'Reported finding' : diagnostic.kind);
}
