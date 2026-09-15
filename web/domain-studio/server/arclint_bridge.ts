import { execFile } from 'node:child_process';
import { watch, type FSWatcher } from 'node:fs';
import { readFile, readdir, realpath, stat, lstat, mkdtemp, rm, symlink, writeFile, open, rename } from 'node:fs/promises';
import { createHash, randomUUID } from 'node:crypto';
import { tmpdir } from 'node:os';
import { isDeepStrictEqual } from 'node:util';
import { parse } from 'yaml';
import { basename, dirname, isAbsolute, relative, resolve, sep } from 'node:path';
import { fileURLToPath } from 'node:url';
import type { IncomingMessage, ServerResponse } from 'node:http';
import type { Plugin } from 'vite';
import type { RepositoryDependencies, RepositoryCheckResult, RepositoryContextReport, RepositoryDiagnostic, RepositoryProject, RepositoryRuleSummary, RepositoryDocument, RepositoryDocumentPreview, RepositoryBaselinePreview } from '../src/repository';

export interface CommandResult { stdout: string; stderr: string; exitCode: number }
export type CommandRunner = (args: readonly string[], cwd: string) => Promise<CommandResult>;
export interface BridgeOptions { root?: string; run?: CommandRunner }
export interface RepositoryRevision { revision: string; changedAt: string; watching: boolean; reason?: string }
export interface RepositoryRevisionMonitor { snapshot(): Promise<RepositoryRevision>; dispose(): void }
class BridgeError extends Error {
  constructor(public readonly status: number, public readonly code: string, message: string) { super(message); }
}
const defaultRoot = resolve(dirname(fileURLToPath(import.meta.url)), '../../..');
const allowedHosts = new Set(['localhost', '127.0.0.1', '[::1]']);
const MAX_OUTPUT = 12 * 1024 * 1024;
const documentFiles = { rules: 'rules.arclint.yaml', domain: 'domain.arclint.yaml' } as const;
const baselineFile = '.arclint/baseline.v2.json';
const digest = (content: string | null) => content === null ? null : createHash('sha256').update(content).digest('hex');
const revisionIgnoredDirectories = new Set(['.git', '.hg', '.svn', 'node_modules', 'dist', 'build', 'coverage', 'test-results', 'playwright-report', '.next', '.nuxt', '.turbo', '.cache', '.pytest_cache', '__pycache__', '.venv', 'venv', '.codex', '.claude', '.wixregiga-ignore']);
const revisionIgnoredName = (name: string) => revisionIgnoredDirectories.has(name) || name.startsWith('.arclint-studio-');

/** Observe source/configuration changes without running ArcLint or reading file contents.
 * One nonrecursive watcher per directory avoids traversing dependency/output trees.
 * The cap and filesystem failures are reported; incomplete coverage never claims to watch.
 */
export function createRepositoryRevisionMonitor(root: string, maxDirectories = 2048): RepositoryRevisionMonitor {
  const identity = randomUUID(); let sequence = 0, changedAt = new Date().toISOString();
  let disposed = false, failure = '', pending: Promise<void> | null = null, rescanNeeded = true;
  const watchers = new Map<string, FSWatcher>();
  const changed = () => { sequence++; changedAt = new Date().toISOString(); };
  const fail = (message: string) => { if (failure !== message) { failure = message; changed(); } };
  const reconcile = async () => {
    const previousFailure = failure; failure = '';
    const seen = new Set<string>(), queue = [root];
    while (queue.length && !disposed) {
      const directory = queue.shift()!;
      if (seen.size >= maxDirectories) { fail(`Automatic change detection reached its ${maxDirectories}-directory limit.`); break; }
      try {
        const entry = await lstat(directory);
        if (!entry.isDirectory() || entry.isSymbolicLink() || !inside(root, await realpath(directory))) continue;
      } catch (error) {
        if ((error as NodeJS.ErrnoException).code !== 'ENOENT') fail('A source directory could not be read for automatic change detection.');
        continue;
      }
      seen.add(directory);
      if (!watchers.has(directory)) {
        try {
          const watcher = watch(directory, { persistent: false }, (event, filename) => {
            if (disposed || filename && revisionIgnoredName(String(filename))) return;
            changed();
            if (event === 'rename' || filename === null) {
              rescanNeeded = true;
              // A directory can be atomically replaced under the same name;
              // its old watcher still refers to the former inode.
              const replaced = filename === null ? null : resolve(directory, String(filename));
              if (replaced && watchers.has(replaced)) { watchers.get(replaced)!.close(); watchers.delete(replaced); }
            }
          });
          watcher.on('error', () => {
            if (disposed) return;
            watcher.close(); watchers.delete(directory); rescanNeeded = true;
            fail('The filesystem could not keep every source directory under observation.');
          });
          watchers.set(directory, watcher);
        } catch (error) {
          if ((error as NodeJS.ErrnoException).code === 'ENOENT') { seen.delete(directory); continue; }
          fail('The filesystem could not watch every source directory.');
        }
      }
      try {
        const entries = await readdir(directory, { withFileTypes: true });
        // Dirent.isDirectory excludes symlinks, including links into another repository.
        for (const entry of entries) if (entry.isDirectory() && !revisionIgnoredName(entry.name)) queue.push(resolve(directory, entry.name));
      } catch (error) {
        if ((error as NodeJS.ErrnoException).code !== 'ENOENT') fail('A source directory could not be read for automatic change detection.');
      }
    }
    for (const [directory, watcher] of watchers) if (!seen.has(directory)) { watcher.close(); watchers.delete(directory); }
    if (previousFailure && !failure) changed();
  };
  const refresh = async () => {
    if (disposed) return;
    if (pending) { await pending; return; }
    if (!rescanNeeded) return;
    rescanNeeded = false;
    pending = reconcile();
    try { await pending; } finally { pending = null; }
  };
  return {
    async snapshot() {
      await refresh();
      return { revision: `${identity}:${sequence}`, changedAt, watching: !disposed && !failure,
        ...(disposed ? { reason: 'Automatic change detection has stopped.' } : failure ? { reason: failure } : {}) };
    },
    dispose() { if (disposed) return; disposed = true; changed(); for (const watcher of watchers.values()) watcher.close(); watchers.clear(); },
  };
}
function preserveRuleIdentity(before: string | null, after: string): void {
  if (before === null) return;
  let previous: unknown, candidate: unknown;
  try { previous = parse(before); candidate = parse(after); } catch { return; } // Actual CLI reports syntax errors.
  if (!isObject(previous) || !isObject(candidate) || !isObject(previous.rules) || !isObject(candidate.rules)) return;
  const keys = ['imports', 'layers', 'structure', 'naming', 'content', 'invariants', 'independent', 'acyclic', 'imported_by', 'uses'];
  for (const [id, original] of Object.entries(previous.rules)) {
    const next = candidate.rules[id];
    if (!isObject(original) || !isObject(next)) continue;
    const constraint = (rule: Record<string, unknown>) => Object.fromEntries(keys.filter(key => key in rule).map(key => [key, rule[key]]));
    if (!isDeepStrictEqual(constraint(original), constraint(next))) throw new BridgeError(400, 'RULE_ID_REQUIRES_CHANGE', `Rule ${id} changes its Constraint. Give the changed proposition a new Rule ID; keep existing identity for severity, Scope, and rationale edits.`);
  }
}

async function requestBody(request: IncomingMessage): Promise<Record<string, unknown>> {
  if (!request.headers['content-type']?.startsWith('application/json')) throw new BridgeError(415, 'JSON_REQUIRED', 'Send application/json for a document action.');
  let size = 0; const chunks: Buffer[] = [];
  for await (const chunk of request) {
    const bytes = Buffer.isBuffer(chunk) ? chunk : Buffer.from(chunk);
    size += bytes.length;
    if (size > 2_000_000) throw new BridgeError(413, 'DOCUMENT_TOO_LARGE', 'Document actions accept at most 2 MB of JSON.');
    chunks.push(bytes);
  }
  let body: unknown;
  try { body = JSON.parse(Buffer.concat(chunks).toString('utf8')); }
  catch { throw new BridgeError(400, 'INVALID_JSON', 'The document action is not valid JSON.'); }
  if (!isObject(body)) throw new BridgeError(400, 'INVALID_DOCUMENT_ACTION', 'A document action must be an object.');
  return body;
}

export const runArclint: CommandRunner = (args, cwd) => new Promise((resolveResult, reject) => {
  execFile('arclint', [...args], { cwd, timeout: 45_000, maxBuffer: MAX_OUTPUT, encoding: 'utf8', windowsHide: true, shell: false }, (error, stdout, stderr) => {
    if (error && typeof error.code !== 'number') {
      const missing = error.code === 'ENOENT';
      reject(new BridgeError(missing ? 503 : 502, missing ? 'ARCLINT_UNAVAILABLE' : 'COMMAND_FAILED', missing ? 'ArcLint is not installed on the server PATH.' : `ArcLint could not complete the command: ${error.message}`));
      return;
    }
    resolveResult({ stdout, stderr, exitCode: error ? error.code as number : 0 });
  });
});
function inside(root: string, target: string): boolean {
  const path = relative(root, target);
  return path === '' || (!isAbsolute(path) && path !== '..' && !path.startsWith(`..${sep}`));
}
/** Reject option injection, traversal and symlink escapes, including paths not yet created. */
export async function resolveRepositoryPath(root: string, input: string): Promise<string> {
  if (!input || input.length > 2000 || input.trim() !== input || input.startsWith('-') || /[\x00-\x1f\\:*?{}]/.test(input) || isAbsolute(input) || input.split('/').includes('..')) throw new BridgeError(400, 'INVALID_PATH', 'Choose a relative repository path, without traversal, globs, or command options.');
  const target = resolve(root, input);
  if (!inside(root, target)) throw new BridgeError(400, 'INVALID_PATH', 'The path must stay inside the configured repository.');
  let existing = target;
  for (;;) {
    try {
      const actual = await realpath(existing);
      if (!inside(root, actual)) throw new BridgeError(400, 'INVALID_PATH', 'The path resolves outside the configured repository.');
      break;
    } catch (error) {
      if (error instanceof BridgeError) throw error;
      const code = (error as NodeJS.ErrnoException).code;
      if (code !== 'ENOENT' && code !== 'ENOTDIR') throw new BridgeError(400, 'INVALID_PATH', 'The repository path cannot be read.');
      const parent = dirname(existing);
      if (parent === existing) throw new BridgeError(400, 'INVALID_PATH', 'The repository path cannot be resolved.');
      existing = parent;
    }
  }
  return relative(root, target).split(sep).join('/') || '.';
}
function authorize(request: IncomingMessage): void {
  const host = request.headers.host;
  let address: URL;
  try { address = new URL(`http://${host ?? ''}`); }
  catch { throw new BridgeError(403, 'LOCAL_ONLY', 'ArcLint repository access is local only.'); }
  if (!host || !allowedHosts.has(address.hostname) || address.host !== host.toLowerCase()) throw new BridgeError(403, 'LOCAL_ONLY', 'Open Domain Studio through localhost or a loopback IP.');
  const origin = request.headers.origin;
  if (origin && origin !== `http://${address.host}`) throw new BridgeError(403, 'FOREIGN_ORIGIN', 'Cross-origin repository access is not allowed.');
  if (request.headers['sec-fetch-site'] === 'cross-site') throw new BridgeError(403, 'FOREIGN_ORIGIN', 'Cross-site repository access is not allowed.');
  if (request.method === 'POST' && request.headers['x-arclint-studio'] !== '1') throw new BridgeError(403, 'REQUEST_HEADER_REQUIRED', 'Repository actions require the Domain Studio request header.');
}
function parseJson(result: CommandResult, operation: string): unknown {
  try { return JSON.parse(result.stdout); }
  catch { throw new BridgeError(502, 'INVALID_CLI_OUTPUT', `ArcLint ${operation} returned an unsupported non-JSON response. ${result.stderr.trim()}`.trim()); }
}
function isObject(value: unknown): value is Record<string, unknown> { return !!value && typeof value === 'object' && !Array.isArray(value); }
function contextReport(value: unknown): RepositoryContextReport {
  if (!isObject(value) || typeof value.Scope !== 'string' || !Array.isArray(value.Languages) || typeof value.RuleCount !== 'number' || (value.Zones !== null && !Array.isArray(value.Zones))) throw new BridgeError(502, 'UNSUPPORTED_CONTEXT_REPORT', 'This ArcLint context JSON shape is not supported. No model evidence was inferred.');
  return value as unknown as RepositoryContextReport;
}
function dependencyReport(value: unknown): Omit<RepositoryDependencies, 'observedAt' | 'revision' | 'changedDuringObservation'> {
  const fail = () => { throw new BridgeError(502, 'DEPENDENCIES_UNAVAILABLE', 'This ArcLint CLI does not provide a supported observed-dependency report. Rebuild the current checkout CLI and restart Studio; other repository evidence remains available.'); };
  if (!isObject(value) || !isObject(value.dependencies)) return fail();
  const d = value.dependencies;
  const strings = (v: unknown): v is string[] => Array.isArray(v) && v.every(item => typeof item === 'string');
  const relativePath = (v: unknown): v is string => typeof v === 'string' && !!v && !v.includes('\\') && !v.startsWith('/') && !/^[A-Za-z]:/.test(v) && !v.split('/').includes('..');
  if (!Array.isArray(d.files) || d.files.some(file => !isObject(file) || !relativePath(file.path) || !strings(file.zones) || typeof file.language !== 'string' || typeof file.importsAvailable !== 'boolean')) return fail();
  if (!Array.isArray(d.edges) || d.edges.some(edge => !isObject(edge) || !relativePath(edge.sourcePath) || !['file', 'directory', 'unresolved'].includes(String(edge.targetKind)) || (edge.targetKind === 'unresolved' ? edge.targetPath !== '' : !relativePath(edge.targetPath)) || typeof edge.specifier !== 'string' || !Number.isInteger(edge.line) || !['internal', 'external', 'stdlib', 'unknown', 'cgo'].includes(String(edge.classification)) || !strings(edge.sourceZones) || !strings(edge.targetZones))) return fail();
  const coverage = d.coverage;
  if (!isObject(coverage) || coverage.scope !== 'repository' || !strings(coverage.languages) || typeof coverage.complete !== 'boolean' || !['filesObserved', 'sourceFiles', 'filesWithImports'].every(key => Number.isInteger(coverage[key]) && Number(coverage[key]) >= 0)) return fail();
  if (!strings(d.limitations) || !Array.isArray(d.diagnostics) || d.diagnostics.some(row => !isObject(row) || typeof row.code !== 'string' || typeof row.message !== 'string' || (row.path !== undefined && !relativePath(row.path)))) return fail();
  return d as unknown as Omit<RepositoryDependencies, 'observedAt' | 'revision' | 'changedDuringObservation'>;
}
function checkResult(result: CommandResult): RepositoryCheckResult {
  if (result.exitCode !== 0 && result.exitCode !== 1) throw new BridgeError(502, 'CHECK_FAILED', `ArcLint check failed (exit ${result.exitCode}). ${result.stderr.trim() || result.stdout.trim()}`.trim());
  const value = parseJson(result, 'check');
  if (!Array.isArray(value) || value.some(row => !isObject(row) || typeof row.kind !== 'string' || typeof row.message !== 'string')) throw new BridgeError(502, 'UNSUPPORTED_CHECK_REPORT', 'This ArcLint check JSON shape is not supported. No conformance result was inferred.');
  return { exitCode: result.exitCode, diagnostics: value as RepositoryDiagnostic[], stderr: result.stderr, checkedAt: new Date().toISOString(), outcomesAvailable: value.some(row => row.kind === 'evaluation' && typeof row.outcome === 'string') };
}
function send(response: ServerResponse, status: number, data: unknown): void {
  response.statusCode = status;
  response.setHeader('Content-Type', 'application/json; charset=utf-8');
  response.setHeader('Cache-Control', 'no-store');
  response.setHeader('X-Content-Type-Options', 'nosniff');
  response.end(JSON.stringify(data));
}

export function createArclintBridge(options: BridgeOptions = {}) {
  const configuredRoot = options.root ?? process.env.ARCLINT_STUDIO_REPO ?? defaultRoot;
  const run = options.run ?? runArclint;
  let revisionMonitor: RepositoryRevisionMonitor | null = null, revisionRoot = '';
  let disposed = false;
  const previews = new Map<string, { preview: RepositoryDocumentPreview; hashes: { rules: string | null; domain: string | null }; root: string }>();
  const baselinePreviews = new Map<string, { preview: RepositoryBaselinePreview; hashes: { rules: string | null; domain: string | null; baseline: string | null }; root: string }>();
  let applying = false;
  // Initialization errors become a useful API response, not an unhandled startup rejection.
  const root = async () => {
    try { const value = await realpath(configuredRoot); if (!(await stat(value)).isDirectory()) throw new Error('not a directory'); return value; }
    catch { throw new BridgeError(503, 'REPOSITORY_UNAVAILABLE', 'The configured ArcLint repository directory is unavailable. Check ARCLINT_STUDIO_REPO.'); }
  };
  const running = new Map<string, Promise<CommandResult>>();
  const waiting: { key: string; start(): void; reject(error: Error): void }[] = [];
  let activeCommands = 0;
  const drain = () => {
    while (!disposed && activeCommands < 4 && waiting.length) waiting.shift()!.start();
  };
  const execute = async (cwd: string, args: readonly string[]) => {
    if (disposed) throw new BridgeError(503, 'BRIDGE_STOPPED', 'The repository connection has stopped.');
    const key = JSON.stringify([cwd, args]);
    // Read requests share active and queued commands across tabs. Mutation
    // commands retain immediate admission: never defer or retry an adoption
    // after its final input checks while other reads take their turns.
    const mutation = args[0] === 'baseline' && ['capture', 'refresh'].includes(args[1]);
    const existing = mutation ? undefined : running.get(key);
    if (existing) return existing;
    if (activeCommands >= 4 && (mutation || waiting.length >= 32)) throw new BridgeError(429, 'BUSY', 'ArcLint is handling other repository requests. Try again shortly.');
    let succeed!: (result: CommandResult) => void, fail!: (error: Error) => void;
    const pending = new Promise<CommandResult>((resolve, reject) => { succeed = resolve; fail = reject; });
    if (!mutation) running.set(key, pending);
    const start = () => {
      activeCommands++;
      void Promise.resolve().then(() => run(args, cwd)).then(succeed, fail).finally(() => {
        activeCommands--;
        if (running.get(key) === pending) running.delete(key);
        drain();
      });
    };
    waiting.push({ key, start, reject: fail });
    drain();
    return pending;
  };
  const readDocument = async (cwd: string, filename: string) => {
    await resolveRepositoryPath(cwd, filename);
    try {
      const path = resolve(cwd, filename);
      if ((await stat(path)).size > 5_000_000) throw new BridgeError(413, 'DOCUMENT_TOO_LARGE', `${filename} exceeds the 5 MB read limit.`);
      return await readFile(path, 'utf8');
    } catch (error) { if ((error as NodeJS.ErrnoException).code === 'ENOENT') return null; throw error; }
  };
  const checkedCommand = async (cwd: string, args: readonly string[]) => {
    const result = await execute(cwd, args);
    if (result.exitCode !== 0) throw new BridgeError(502, 'QUERY_FAILED', `ArcLint ${args[0]} failed (exit ${result.exitCode}). ${result.stderr.trim() || result.stdout.trim()}`.trim());
    return parseJson(result, args[0]);
  };
  const documents = async (cwd: string) => {
    const [rules, domain] = await Promise.all([readDocument(cwd, documentFiles.rules), readDocument(cwd, documentFiles.domain)]);
    return { rules, domain, hashes: { rules: digest(rules), domain: digest(domain) } };
  };
  const regularDocument = async (cwd: string, document: RepositoryDocument) => {
    const path = resolve(cwd, documentFiles[document]);
    const info = await lstat(path).catch(error => { if (error.code === 'ENOENT') return null; throw error; });
    if (info && !info.isFile()) throw new BridgeError(400, 'DOCUMENT_NOT_REGULAR', `Saving requires ${documentFiles[document]} to be a regular file, not a symlink or directory.`);
    return info;
  };
  const validateCandidate = async (cwd: string, candidate: { rules: string | null; domain: string | null }) => {
    if (!candidate.rules) throw new BridgeError(400, 'RULESET_REQUIRED', 'A repository rules.arclint.yaml is required to validate this document.');
    const temporary = await mkdtemp(resolve(tmpdir(), 'arclint-studio-validate-'));
    try {
      // The CLI has no separate domain-file flag. Both documents live in this
      // isolated root; existing support files are linked for offline resolution.
      // Only read-only listing commands run here, never check/install/define.
      const entries = await readdir(cwd, { withFileTypes: true });
      await Promise.all(entries.filter(entry => !['.git', documentFiles.rules, documentFiles.domain].includes(entry.name) && !entry.name.startsWith('.arclint-studio-')).map(entry => symlink(resolve(cwd, entry.name), resolve(temporary, entry.name), entry.isDirectory() ? 'dir' : 'file')));
      const rulesPath = resolve(temporary, documentFiles.rules);
      await writeFile(rulesPath, candidate.rules, { mode: 0o600 });
      if (candidate.domain !== null) await writeFile(resolve(temporary, documentFiles.domain), candidate.domain, { mode: 0o600 });
      const validatedRules = await checkedCommand(temporary, ['rules', '--rules', rulesPath, '--format', 'json']);
      if (!Array.isArray(validatedRules)) throw new BridgeError(502, 'INVALID_VALIDATION_OUTPUT', 'ArcLint did not return a validated Rule list.');
      if (candidate.domain !== null) {
        const validatedDomain = await checkedCommand(temporary, ['domain', '--rules', rulesPath, '--format', 'json']);
        if (!isObject(validatedDomain) || validatedDomain.found !== true) throw new BridgeError(400, 'DOMAIN_VALIDATION_FAILED', 'ArcLint did not accept the candidate domain document.');
      }
      return { rules: validatedRules.length, domain: candidate.domain !== null, message: 'ArcLint loaded the candidate Rules and Domain in an isolated workspace. This validates configuration and recorded language; no code check was run.' };
    } finally { await rm(temporary, { recursive: true, force: true }); }
  };
  const baselineState = async (cwd: string) => {
    // Capture creates this directory. A symlink at either level could redirect
    // the CLI's own write, so even in-repository symlinks are refused here.
    for (const [path, directory] of [['.arclint', true], [baselineFile, false]] as const) {
      const info = await lstat(resolve(cwd, path)).catch(error => { if (error.code === 'ENOENT') return null; throw error; });
      if (info && (info.isSymbolicLink() || (directory ? !info.isDirectory() : !info.isFile()))) throw new BridgeError(400, 'UNSAFE_BASELINE_PATH', 'Baseline adoption requires a regular .arclint directory and baseline.v2.json file, without symlinks.');
    }
    const [current, baseline] = await Promise.all([documents(cwd), readDocument(cwd, baselineFile)]);
    return { ...current.hashes, baseline: digest(baseline) };
  };
  const adoptionAssessment = async (cwd: string) => {
    const rules = await checkedCommand(cwd, ['rules', '--format', 'json']);
    if (!Array.isArray(rules) || rules.some(rule => !isObject(rule) || !rule.disabled && rule.assurance !== 'exact')) throw new BridgeError(409, 'BASELINE_EVIDENCE_INCOMPLETE', 'This CLI omits the full outcome table. Baseline adoption here requires exact-assurance Rules; an unknown, partial, heuristic, or advisory Rule cannot establish complete evaluation through this API.');
    const report = checkResult(await execute(cwd, ['check', '.', '--no-baseline', '--format', 'json']));
    if (report.diagnostics.some(d => d.kind !== 'violation' || !['active', 'suppressed'].includes(d.status ?? ''))) throw new BridgeError(409, 'BASELINE_EVIDENCE_INCOMPLETE', 'The unbaselined assessment contains coverage, operational, or unsupported records. Resolve these evaluation gaps before adopting findings.');
    return { report, rules: rules.filter(rule => !rule.disabled).length };
  };
  const middleware = (request: IncomingMessage, response: ServerResponse, next: () => void) => {
    let url: URL;
    try { url = new URL(request.url ?? '/', 'http://localhost'); } catch { next(); return; }
    if (!url.pathname.startsWith('/api/arclint/')) { next(); return; }
    void (async () => {
      authorize(request);
      const routes = new Map([['/api/arclint/dependencies', 'GET'], ['/api/arclint/revision', 'GET'], ['/api/arclint/project', 'GET'], ['/api/arclint/context', 'GET'], ['/api/arclint/check', 'POST'], ['/api/arclint/rule', 'GET'], ['/api/arclint/patterns', 'GET'], ['/api/arclint/files', 'GET'], ['/api/arclint/documents/preview', 'POST'], ['/api/arclint/documents/apply', 'POST'], ['/api/arclint/baseline/preview', 'POST'], ['/api/arclint/baseline/apply', 'POST']]);
      const expected = routes.get(url.pathname);
      if (!expected) throw new BridgeError(404, 'NOT_FOUND', 'Unknown ArcLint endpoint.');
      if (request.method !== expected) throw new BridgeError(405, 'METHOD_NOT_ALLOWED', `Use ${expected} for this endpoint.`);
      const cwd = await root();
      if (url.pathname === '/api/arclint/revision' || url.pathname === '/api/arclint/dependencies') {
        if (disposed) throw new BridgeError(503, 'REVISION_MONITOR_STOPPED', 'The repository change monitor has stopped.');
        if (!revisionMonitor || revisionRoot !== cwd) {
          revisionMonitor?.dispose(); revisionRoot = cwd;
          revisionMonitor = createRepositoryRevisionMonitor(cwd);
        }
        const before = await revisionMonitor.snapshot();
        if (url.pathname === '/api/arclint/revision') { send(response, 200, before); return; }
        const command = await execute(cwd, ['context', '--dependencies', '--format', 'json']);
        if (command.exitCode !== 0) throw new BridgeError(502, 'DEPENDENCIES_UNAVAILABLE', `Observed imports could not be read. Rebuild the current checkout CLI if --dependencies is unavailable. ${command.stderr.trim() || command.stdout.trim()}`);
        const observed = dependencyReport(parseJson(command, 'context --dependencies'));
        const after = await revisionMonitor.snapshot();
        const changedDuringObservation = before.revision !== after.revision;
        const diagnostics = [...observed.diagnostics];
        if (changedDuringObservation) diagnostics.push({ code: 'REPOSITORY_CHANGED', message: 'Repository files changed while imports were observed. Refresh to obtain current evidence.' });
        if (!after.watching) diagnostics.push({ code: 'REVISION_UNAVAILABLE', message: after.reason ?? 'Repository changes cannot be monitored completely.' });
        send(response, 200, { ...observed, diagnostics, observedAt: new Date().toISOString(), revision: before.revision, changedDuringObservation });
      } else if (url.pathname === '/api/arclint/project') {
        const [domainYaml, rulesYaml, rawContext, rawRules] = await Promise.all([
          readDocument(cwd, 'domain.arclint.yaml'), readDocument(cwd, 'rules.arclint.yaml'),
          checkedCommand(cwd, ['context', '--full', '--format', 'json']), checkedCommand(cwd, ['rules', '--format', 'json']),
        ]);
        if (!Array.isArray(rawRules) || rawRules.some(rule => !isObject(rule) || typeof rule.id !== 'string' || typeof rule.proposition !== 'string')) throw new BridgeError(502, 'UNSUPPORTED_RULES_REPORT', 'This ArcLint rules JSON shape is not supported.');
        const result: RepositoryProject = { repository: { name: basename(cwd), root: cwd }, domainYaml, rulesYaml, documentHashes: { rules: digest(rulesYaml), domain: digest(domainYaml) }, context: contextReport(rawContext), rules: rawRules as RepositoryRuleSummary[], loadedAt: new Date().toISOString() };
        send(response, 200, result);
      } else if (url.pathname === '/api/arclint/baseline/preview') {
        await requestBody(request);
        const hashes = await baselineState(cwd);
        const { report, rules } = await adoptionAssessment(cwd);
        if (!isDeepStrictEqual(hashes, await baselineState(cwd))) throw new BridgeError(409, 'BASELINE_INPUTS_CHANGED', 'Rules, Domain, or Baseline changed while assessment ran. Review again.');
        const preview: RepositoryBaselinePreview = { token: randomUUID(), action: hashes.baseline === null ? 'capture' : 'refresh', filename: baselineFile, findings: report.diagnostics.filter(d => d.status === 'active').length, rules, report, expiresAt: new Date(Date.now() + 15 * 60_000).toISOString() };
        for (const [key, value] of baselinePreviews) if (Date.parse(value.preview.expiresAt) < Date.now()) baselinePreviews.delete(key);
        if (baselinePreviews.size >= 8) baselinePreviews.delete(baselinePreviews.keys().next().value!);
        baselinePreviews.set(preview.token, { preview, hashes, root: cwd }); send(response, 200, preview);
      } else if (url.pathname === '/api/arclint/baseline/apply') {
        const body = await requestBody(request);
        const stored = typeof body.token === 'string' ? baselinePreviews.get(body.token) : undefined;
        if (!stored || stored.root !== cwd || Date.parse(stored.preview.expiresAt) < Date.now()) throw new BridgeError(409, 'PREVIEW_REQUIRED', 'Review a fresh Baseline adoption before applying.');
        if (applying) throw new BridgeError(409, 'WRITE_IN_PROGRESS', 'Another repository change is being applied.');
        applying = true;
        try {
          if (!isDeepStrictEqual(stored.hashes, await baselineState(cwd))) throw new BridgeError(409, 'BASELINE_INPUTS_CHANGED', 'Rules, Domain, or Baseline changed after review. No adoption was performed.');
          const repeated = await adoptionAssessment(cwd);
          if (repeated.rules !== stored.preview.rules || !isDeepStrictEqual(repeated.report.diagnostics, stored.preview.report.diagnostics)) throw new BridgeError(409, 'BASELINE_REPORT_CHANGED', 'The unbaselined findings changed after review. Review the new findings before adoption.');
          if (!isDeepStrictEqual(stored.hashes, await baselineState(cwd))) throw new BridgeError(409, 'BASELINE_INPUTS_CHANGED', 'Repository inputs changed during assessment. No adoption was performed.');
          // The CLI owns fingerprint/count serialization. Never manufacture a
          // Baseline from the UI diagnostic projection.
          baselinePreviews.delete(stored.preview.token);
          const captured = await checkedCommand(cwd, ['baseline', stored.preview.action, '--format', 'json']);
          if (!isObject(captured) || typeof captured.findings !== 'number' || typeof captured.rules !== 'number') throw new BridgeError(502, 'BASELINE_RESULT_UNAVAILABLE', 'ArcLint ran the adoption command, but its summary could not be read. Inspect the Baseline before trying again.');
          send(response, 200, { ...captured, action: stored.preview.action, savedAt: new Date().toISOString() });
        } finally { applying = false; }
      } else if (url.pathname === '/api/arclint/documents/preview') {
        const body = await requestBody(request);
        if ((body.document !== 'rules' && body.document !== 'domain') || typeof body.content !== 'string' || !body.content.trim() || Buffer.byteLength(body.content) > 1_000_000 || !(body.expectedHash === null || typeof body.expectedHash === 'string' && /^[a-f0-9]{64}$/.test(body.expectedHash))) throw new BridgeError(400, 'INVALID_DOCUMENT_ACTION', 'Choose rules or domain, supply YAML under 1 MB, and include the source hash returned by this server.');
        await regularDocument(cwd, body.document);
        const current = await documents(cwd);
        if (current.hashes[body.document] !== body.expectedHash) throw new BridgeError(409, 'DOCUMENT_CHANGED', 'The source document changed on disk. Refresh it and review your changes against the current version.');
        if (body.document === 'rules') preserveRuleIdentity(current.rules, body.content);
        const candidate = { rules: current.rules, domain: current.domain, [body.document]: body.content };
        const validation = await validateCandidate(cwd, candidate);
        const token = randomUUID();
        const preview: RepositoryDocumentPreview = { token, document: body.document, filename: documentFiles[body.document], before: current[body.document], after: body.content, expectedHash: body.expectedHash, proposedHash: digest(body.content)!, expiresAt: new Date(Date.now() + 15 * 60_000).toISOString(), validation };
        for (const [key, stored] of previews) if (Date.parse(stored.preview.expiresAt) < Date.now()) previews.delete(key);
        if (previews.size >= 16) previews.delete(previews.keys().next().value!);
        previews.set(token, { preview, hashes: current.hashes, root: cwd }); send(response, 200, preview);
      } else if (url.pathname === '/api/arclint/documents/apply') {
        const body = await requestBody(request);
        const stored = typeof body.token === 'string' ? previews.get(body.token) : undefined;
        if (!stored || stored.root !== cwd || Date.parse(stored.preview.expiresAt) < Date.now()) throw new BridgeError(409, 'PREVIEW_REQUIRED', 'Review a fresh preview before applying this document.');
        if (applying) throw new BridgeError(409, 'WRITE_IN_PROGRESS', 'Another document is being applied. Try again after it completes.');
        applying = true;
        let temporary: string | undefined;
        try {
          const { preview } = stored;
          const current = await documents(cwd);
          if (JSON.stringify(current.hashes) !== JSON.stringify(stored.hashes)) throw new BridgeError(409, 'DOCUMENT_CHANGED', 'Rules or Domain changed after this preview. Review a fresh preview; nothing was written.');
          const info = await regularDocument(cwd, preview.document);
          await validateCandidate(cwd, { rules: current.rules, domain: current.domain, [preview.document]: preview.after });
          temporary = resolve(cwd, `.arclint-studio-${randomUUID()}.tmp`);
          const output = await open(temporary, 'wx', info ? info.mode & 0o777 : 0o644);
          try { await output.writeFile(preview.after, 'utf8'); await output.sync(); } finally { await output.close(); }
          if (JSON.stringify((await documents(cwd)).hashes) !== JSON.stringify(stored.hashes)) throw new BridgeError(409, 'DOCUMENT_CHANGED', 'The documents changed while validation ran. Nothing was applied; review a fresh preview.');
          await regularDocument(cwd, preview.document);
          await rename(temporary, resolve(cwd, preview.filename)); temporary = undefined;
          previews.delete(preview.token);
          send(response, 200, { document: preview.document, filename: preview.filename, hash: preview.proposedHash, savedAt: new Date().toISOString() });
        } finally { if (temporary) await rm(temporary, { force: true }); applying = false; }
      } else if (url.pathname === '/api/arclint/rule') {
        const ids = url.searchParams.getAll('id');
        if (ids.length !== 1 || !ids[0] || ids[0].length > 500 || ids[0].startsWith('-') || /[\x00-\x20]/.test(ids[0])) throw new BridgeError(400, 'INVALID_RULE', 'Choose one exact configured Rule ID.');
        const rules = await checkedCommand(cwd, ['rules', '--format', 'json']);
        if (!Array.isArray(rules) || !rules.some(rule => isObject(rule) && rule.id === ids[0])) throw new BridgeError(400, 'INVALID_RULE', 'This Rule ID is not configured in the bound repository.');
        const detail = await checkedCommand(cwd, ['rules', '--format', 'json', '--', ids[0]]);
        if (!isObject(detail) || !isObject(detail.summary) || detail.summary.id !== ids[0]) throw new BridgeError(502, 'UNSUPPORTED_RULE_DETAIL', 'ArcLint did not return the requested Rule detail.');
        send(response, 200, detail);
      } else if (url.pathname === '/api/arclint/patterns') {
        const catalog = await checkedCommand(cwd, ['patterns', '--format', 'json']);
        if (!isObject(catalog) || !Array.isArray(catalog.patterns) || catalog.patterns.some(pattern => !isObject(pattern) || typeof pattern.reference !== 'string')) throw new BridgeError(502, 'UNSUPPORTED_PATTERN_CATALOG', 'ArcLint returned an unsupported Pattern catalog.');
        send(response, 200, { ...catalog, queriedAt: new Date().toISOString() });
      } else if (url.pathname === '/api/arclint/files') {
        const directories = url.searchParams.getAll('directory');
        if (directories.length !== 1) throw new BridgeError(400, 'INVALID_PATH', 'Choose one relative repository directory.');
        const directory = await resolveRepositoryPath(cwd, directories[0]);
        let entries;
        try { entries = await readdir(resolve(cwd, directory), { withFileTypes: true }); }
        catch { throw new BridgeError(400, 'DIRECTORY_UNAVAILABLE', 'This repository directory cannot be listed.'); }
        // One level only. Never follow symlinks or expose Git's object database.
        const visible = entries.filter(entry => entry.name !== '.git' && (entry.isFile() || entry.isDirectory()))
          .sort((a, b) => Number(b.isDirectory()) - Number(a.isDirectory()) || a.name.localeCompare(b.name));
        send(response, 200, { directory, entries: visible.slice(0, 500).map(entry => ({ name: entry.name, path: directory === '.' ? entry.name : `${directory}/${entry.name}`, kind: entry.isDirectory() ? 'directory' : 'file' })), truncated: visible.length > 500, queriedAt: new Date().toISOString() });
      } else if (url.pathname === '/api/arclint/context') {
        const paths = url.searchParams.getAll('path');
        const zones = url.searchParams.getAll('zone');
        if (paths.length + zones.length !== 1) throw new BridgeError(400, 'INVALID_CONTEXT_REQUEST', 'Supply exactly one relative path or one declared Zone name.');
        if (zones.length) {
          const zone = zones[0];
          if (!zone || zone.trim() !== zone || zone.length > 200 || zone.startsWith('-') || /[\x00-\x1f]/.test(zone)) throw new BridgeError(400, 'INVALID_ZONE', 'Choose one exact declared Zone name.');
          const repository = contextReport(await checkedCommand(cwd, ['context', '--full', '--format', 'json']));
          if (!repository.Zones?.some(candidate => candidate.Name === zone)) throw new BridgeError(400, 'INVALID_ZONE', `No configured Zone is named ${zone}.`);
          const report = contextReport(await checkedCommand(cwd, ['context', '--zone', zone, '--format', 'json']));
          send(response, 200, { path: `--zone ${zone}`, zone, report, queriedAt: new Date().toISOString() });
        } else {
          const path = await resolveRepositoryPath(cwd, paths[0]);
          const report = contextReport(await checkedCommand(cwd, ['context', '--format', 'json', '--', path]));
          const subject = await stat(resolve(cwd, path)).catch(() => null);
          send(response, 200, { path, pathType: subject ? subject.isDirectory() ? 'directory' : 'file' : 'missing', report, queriedAt: new Date().toISOString() });
        }
      } else send(response, 200, checkResult(await execute(cwd, ['check', '.', '--format', 'json'])));
    })().catch(error => {
      const failure = error instanceof BridgeError ? error : new BridgeError(500, 'REPOSITORY_ERROR', 'The local repository request could not be completed.');
      send(response, failure.status, { error: { code: failure.code, message: failure.message } });
    });
  };
  return Object.assign(middleware, { dispose() {
    disposed = true; revisionMonitor?.dispose();
    for (const command of waiting.splice(0)) {
      running.delete(command.key);
      command.reject(new BridgeError(503, 'BRIDGE_STOPPED', 'The repository connection has stopped.'));
    }
  } });
}

export function arclintBridgePlugin(options: BridgeOptions = {}): Plugin {
  const attach = (server: { middlewares: { use: (middleware: ReturnType<typeof createArclintBridge>) => unknown }; httpServer: { once(event: 'close', listener: () => void): unknown } | null }) => {
    const middleware = createArclintBridge(options);
    server.middlewares.use(middleware); server.httpServer?.once('close', () => middleware.dispose());
  };
  return { name: 'arclint-local-repository-bridge', configureServer: attach, configurePreviewServer: attach };
}
