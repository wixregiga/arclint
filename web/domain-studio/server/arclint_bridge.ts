import { execFile } from 'node:child_process';
import { readFile, realpath, stat } from 'node:fs/promises';
import { basename, dirname, isAbsolute, relative, resolve, sep } from 'node:path';
import { fileURLToPath } from 'node:url';
import type { IncomingMessage, ServerResponse } from 'node:http';
import type { Plugin } from 'vite';
import type { RepositoryCheckResult, RepositoryContextReport, RepositoryDiagnostic, RepositoryProject, RepositoryRuleSummary } from '../src/repository';

export interface CommandResult { stdout: string; stderr: string; exitCode: number }
export type CommandRunner = (args: readonly string[], cwd: string) => Promise<CommandResult>;
export interface BridgeOptions { root?: string; run?: CommandRunner }
class BridgeError extends Error {
  constructor(public readonly status: number, public readonly code: string, message: string) { super(message); }
}
const defaultRoot = resolve(dirname(fileURLToPath(import.meta.url)), '../../..');
const allowedHosts = new Set(['localhost', '127.0.0.1', '[::1]']);
const MAX_OUTPUT = 12 * 1024 * 1024;

export const runArclint: CommandRunner = (args, cwd) => new Promise((resolveResult, reject) => {
  execFile('arclint', [...args], { cwd, timeout: 45_000, maxBuffer: MAX_OUTPUT, encoding: 'utf8', windowsHide: true, shell: false }, (error, stdout, stderr) => {
    if (error && typeof error.code !== 'number') {
      const missing = error.code === 'ENOENT';
      reject(new BridgeError(missing ? 503 : 502, missing ? 'ARCLINT_UNAVAILABLE' : 'COMMAND_FAILED', missing ? 'ArcLint is not installed on the server PATH.' : `ArcLint could not complete the read-only command: ${error.message}`));
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
  if (request.method === 'POST' && request.headers['x-arclint-studio'] !== '1') throw new BridgeError(403, 'REQUEST_HEADER_REQUIRED', 'Repository checks require the Domain Studio request header.');
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
  // Initialization errors become a useful API response, not an unhandled startup rejection.
  const root = async () => {
    try { const value = await realpath(configuredRoot); if (!(await stat(value)).isDirectory()) throw new Error('not a directory'); return value; }
    catch { throw new BridgeError(503, 'REPOSITORY_UNAVAILABLE', 'The configured ArcLint repository directory is unavailable. Check ARCLINT_STUDIO_REPO.'); }
  };
  const running = new Map<string, Promise<CommandResult>>();
  const execute = async (cwd: string, args: readonly string[]) => {
    const key = JSON.stringify(args);
    let pending = running.get(key);
    if (!pending) {
      if (running.size >= 4) throw new BridgeError(429, 'BUSY', 'ArcLint is handling other repository requests. Try again shortly.');
      pending = run(args, cwd); running.set(key, pending);
      void pending.finally(() => { if (running.get(key) === pending) running.delete(key); }).catch(() => {});
    }
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
  return (request: IncomingMessage, response: ServerResponse, next: () => void) => {
    let url: URL;
    try { url = new URL(request.url ?? '/', 'http://localhost'); } catch { next(); return; }
    if (!url.pathname.startsWith('/api/arclint/')) { next(); return; }
    void (async () => {
      authorize(request);
      const routes = new Map([['/api/arclint/project', 'GET'], ['/api/arclint/context', 'GET'], ['/api/arclint/check', 'POST']]);
      const expected = routes.get(url.pathname);
      if (!expected) throw new BridgeError(404, 'NOT_FOUND', 'Unknown ArcLint endpoint.');
      if (request.method !== expected) throw new BridgeError(405, 'METHOD_NOT_ALLOWED', `Use ${expected} for this endpoint.`);
      const cwd = await root();
      if (url.pathname === '/api/arclint/project') {
        const [domainYaml, rulesYaml, rawContext, rawRules] = await Promise.all([
          readDocument(cwd, 'domain.arclint.yaml'), readDocument(cwd, 'rules.arclint.yaml'),
          checkedCommand(cwd, ['context', '--full', '--format', 'json']), checkedCommand(cwd, ['rules', '--format', 'json']),
        ]);
        if (!Array.isArray(rawRules) || rawRules.some(rule => !isObject(rule) || typeof rule.id !== 'string' || typeof rule.proposition !== 'string')) throw new BridgeError(502, 'UNSUPPORTED_RULES_REPORT', 'This ArcLint rules JSON shape is not supported.');
        const result: RepositoryProject = { repository: { name: basename(cwd), root: cwd }, domainYaml, rulesYaml, context: contextReport(rawContext), rules: rawRules as RepositoryRuleSummary[], loadedAt: new Date().toISOString() };
        send(response, 200, result);
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
          send(response, 200, { path, report, queriedAt: new Date().toISOString() });
        }
      } else send(response, 200, checkResult(await execute(cwd, ['check', '.', '--format', 'json'])));
    })().catch(error => {
      const failure = error instanceof BridgeError ? error : new BridgeError(500, 'REPOSITORY_ERROR', 'The local repository request could not be completed.');
      send(response, failure.status, { error: { code: failure.code, message: failure.message } });
    });
  };
}

export function arclintBridgePlugin(options: BridgeOptions = {}): Plugin {
  const middleware = createArclintBridge(options);
  return { name: 'arclint-local-read-only-bridge', configureServer(server) { server.middlewares.use(middleware); }, configurePreviewServer(server) { server.middlewares.use(middleware); } };
}
