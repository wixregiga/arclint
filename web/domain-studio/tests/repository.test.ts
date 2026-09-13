import { test } from 'node:test';
import assert from 'node:assert/strict';
import { createServer, request as httpRequest, type Server } from 'node:http';
import { once } from 'node:events';
import { mkdtemp, writeFile, rm, symlink } from 'node:fs/promises';
import { tmpdir } from 'node:os';
import { join } from 'node:path';
import type { AddressInfo } from 'node:net';
import { createArclintBridge, runArclint, type CommandResult, type CommandRunner } from '../server/arclint_bridge';
import { diagnosticLabel } from '../src/repository';

const context = { Scope: 'repository', Languages: ['go'], RuleCount: 1, Zones: [], Rules: null, Paths: null };
const rules = [{ id: 'model/no-panic', type: 'content', severity: 'error', proposition: 'No panic', rationale: 'Keep the model total.', assurance: 'exact' }];
const diagnostics = [{ kind: 'violation', ruleId: 'model/no-panic', severity: 'error', path: 'price.go', line: 3, status: 'active', message: 'forbidden content matching /panic/' }];
const answer = (value: unknown, exitCode = 0, stderr = ''): CommandResult => ({ stdout: JSON.stringify(value), exitCode, stderr });
async function fixture() {
  const root = await mkdtemp(join(tmpdir(), 'domain-studio-api-'));
  await writeFile(join(root, 'rules.arclint.yaml'), 'runtime: [go]\nzones:\n  model:\n    paths: "*.go"\nrules:\n  model/no-panic:\n    on: model\n    rationale: Model code never panics.\n    content:\n      forbid: panic\n');
  await writeFile(join(root, 'domain.arclint.yaml'), 'version: 1\nproject: Fixture\ncontexts:\n  model:\n    definition: The fixture model.\n    value_objects:\n      Price:\n        definition: An amount represented as a value.\n');
  await writeFile(join(root, 'go.mod'), 'module example.com/studiofixture\n\ngo 1.23\n');
  await writeFile(join(root, 'price.go'), 'package fixture\n\ntype Price int\n\nfunc Fail() { panic("bad") }\n');
  return root;
}
async function start(root: string, run?: CommandRunner) {
  const bridge = createArclintBridge({ root, run });
  const server = createServer((request, response) => bridge(request, response, () => { response.statusCode = 404; response.end(); }));
  server.listen(0, '127.0.0.1'); await once(server, 'listening');
  const base = `http://127.0.0.1:${(server.address() as AddressInfo).port}`;
  return { server, base, call: (path: string, init?: RequestInit) => fetch(`${base}/api/arclint/${path}`, init) };
}
async function stop(server: Server) { server.closeAllConnections(); await new Promise<void>((resolve, reject) => server.close(error => error ? reject(error) : resolve())); }
const post = { method: 'POST', headers: { 'X-Arclint-Studio': '1' } };

test('project supplies actual document bytes and raw CLI evidence from one bound cwd', async t => {
  const root = await fixture(); const commands: [readonly string[], string][] = [];
  const api = await start(root, async (args, cwd) => { commands.push([args, cwd]); return answer(args[0] === 'context' ? context : rules); });
  t.after(async () => { await stop(api.server); await rm(root, { recursive: true }); });
  const response = await api.call('project'); assert.equal(response.status, 200); const project = await response.json();
  assert.equal(project.repository.root, root); assert.match(project.domainYaml, /project: Fixture/);
  assert.match(project.rulesYaml, /forbid: panic/); assert.deepEqual(project.context, context); assert.deepEqual(project.rules, rules);
  assert.deepEqual(commands.map(([args]) => args).sort(), [['context', '--full', '--format', 'json'], ['rules', '--format', 'json']].sort());
  assert.ok(commands.every(([, cwd]) => cwd === root));
  assert.equal(response.headers.get('cache-control'), 'no-store');
});

test('path context passes a validated literal argument after -- and retains report casing', async t => {
  const root = await fixture(); const seen: readonly string[][] = [];
  const api = await start(root, async args => { (seen as string[][]).push([...args]); return answer({ ...context, Scope: 'price.go' }); });
  t.after(async () => { await stop(api.server); await rm(root, { recursive: true }); });
  const response = await api.call('context?path=price.go'); assert.equal(response.status, 200);
  const result = await response.json(); assert.equal(result.path, 'price.go'); assert.equal(result.report.Scope, 'price.go');
  assert.deepEqual(seen, [['context', '--format', 'json', '--', 'price.go']]);
  const routeResponse = await api.call('context?path=src%2F%5Bid%5D.ts');
  assert.equal(routeResponse.status, 200, 'Square brackets in framework route filenames are ordinary path characters.');
});

test('path traversal, absolute paths, option injection and symlink escape never reach CLI', async t => {
  const root = await fixture(); const outside = await mkdtemp(join(tmpdir(), 'studio-outside-')); let calls = 0;
  await symlink(outside, join(root, 'escape'));
  const api = await start(root, async () => { calls++; return answer(context); });
  t.after(async () => { await stop(api.server); await rm(root, { recursive: true }); await rm(outside, { recursive: true }); });
  for (const path of ['../outside.go', '/etc/passwd', '--write', 'C:\\private', 'escape/missing.go', 'src/../../outside', 'foo\u0000bar', '*.go']) {
    const response = await api.call(`context?path=${encodeURIComponent(path)}`); assert.equal(response.status, 400, path); assert.equal((await response.json()).error.code, 'INVALID_PATH');
  }
  assert.equal(calls, 0);
});

test('foreign hosts, origins and cross-site requests cannot query repository', async t => {
  const root = await fixture(); let calls = 0;
  const api = await start(root, async () => { calls++; return answer(context); });
  t.after(async () => { await stop(api.server); await rm(root, { recursive: true }); });
  const foreignHeaders: Record<string, string>[] = [{ Host: 'attacker.example' }, { Origin: 'https://attacker.example' }, { Origin: 'null' }, { 'Sec-Fetch-Site': 'cross-site' }];
  for (const headers of foreignHeaders) {
    const status = await new Promise<number | undefined>((resolve, reject) => {
      const request = httpRequest(`${api.base}/api/arclint/project`, { headers }, response => { response.resume(); response.on('end', () => resolve(response.statusCode)); });
      request.on('error', reject); request.end();
    });
    assert.equal(status, 403, JSON.stringify(headers));
  }
  assert.equal((await api.call('check', { method: 'POST' })).status, 403);
  assert.equal(calls, 0);
});

test('exit one is a completed check with findings; no outcomes or green conformance are invented', async t => {
  const root = await fixture();
  const api = await start(root, async args => { assert.deepEqual(args, ['check', '.', '--format', 'json']); return answer(diagnostics, 1); });
  t.after(async () => { await stop(api.server); await rm(root, { recursive: true }); });
  const response = await api.call('check', post); assert.equal(response.status, 200); const result = await response.json();
  assert.equal(result.exitCode, 1); assert.deepEqual(result.diagnostics, diagnostics); assert.equal(result.outcomesAvailable, false);
  assert.equal(result.diagnostics[0].outcome, undefined); assert.equal(diagnosticLabel(result.diagnostics[0]), 'Reported finding');
});

test('configuration failure and malformed CLI response remain failures, never empty findings', async t => {
  const root = await fixture(); let result: CommandResult = { stdout: '', stderr: 'invalid rules configuration', exitCode: 2 };
  const api = await start(root, async () => result);
  t.after(async () => { await stop(api.server); await rm(root, { recursive: true }); });
  let response = await api.call('check', post); assert.equal(response.status, 502); assert.equal((await response.json()).error.code, 'CHECK_FAILED');
  result = { stdout: 'not JSON', stderr: '', exitCode: 0 };
  response = await api.call('check', post); assert.equal(response.status, 502); assert.equal((await response.json()).error.code, 'INVALID_CLI_OUTPUT');
  result = answer({ ok: true }); response = await api.call('check', post); assert.equal(response.status, 502); assert.equal((await response.json()).error.code, 'UNSUPPORTED_CHECK_REPORT');
});

test('empty successful check stays an empty diagnostic list without inferred rule outcomes', async t => {
  const root = await fixture(); const api = await start(root, async () => answer([]));
  t.after(async () => { await stop(api.server); await rm(root, { recursive: true }); });
  const response = await api.call('check', post); const result = await response.json();
  assert.equal(response.status, 200); assert.equal(result.exitCode, 0); assert.equal(result.outcomesAvailable, false); assert.deepEqual(result.diagnostics, []);
});

test('simultaneous identical checks share one bounded command execution', async t => {
  const root = await fixture(); let calls = 0; let release: (() => void) | undefined;
  const blocked = new Promise<void>(resolve => { release = resolve; });
  const api = await start(root, async () => { calls++; await blocked; return answer(diagnostics, 1); });
  t.after(async () => { release?.(); await stop(api.server); await rm(root, { recursive: true }); });
  const first = api.call('check', post); const second = api.call('check', post);
  await new Promise(resolve => setTimeout(resolve, 40)); assert.equal(calls, 1); release!();
  assert.equal((await first).status, 200); assert.equal((await second).status, 200);
});

test('actual installed CLI performs a failing check and path context through the HTTP bridge without source writes', async t => {
  try { await runArclint(['--help'], process.cwd()); } catch { t.skip('ArcLint executable is not installed on PATH'); return; }
  const root = await fixture(); const api = await start(root);
  t.after(async () => { await stop(api.server); await rm(root, { recursive: true }); });
  const before = await import('node:fs/promises').then(fs => fs.readFile(join(root, 'price.go'), 'utf8'));
  const project = await api.call('project').then(r => r.json()); assert.equal(project.repository.root, root); assert.equal(project.context.Zones[0].Name, 'model');
  const checkResponse = await api.call('check', post); assert.equal(checkResponse.status, 200); const checked = await checkResponse.json();
  assert.equal(checked.exitCode, 1); assert.ok(checked.diagnostics.some((d: { ruleId?: string }) => d.ruleId === 'model/no-panic'));
  const queried = await api.call('context?path=price.go').then(r => r.json());
  assert.equal(queried.report.Scope, 'price.go'); assert.equal(queried.report.Rules[0].Summary.Rationale, 'Model code never panics.');
  assert.equal(await import('node:fs/promises').then(fs => fs.readFile(join(root, 'price.go'), 'utf8')), before);
});

test('Zone queries validate an exact configured name and use --zone rather than glob paths', async t => {
  const root = await fixture(); const seen: string[][] = [];
  const zone = { Name: 'model', Description: 'The model.', Paths: ['*.go'], Internal: null, InternalRestricted: false, External: 'allow', Stdlib: 'allow' };
  const api = await start(root, async args => { seen.push([...args]); return answer({ ...context, Zones: [zone], Scope: args.includes('--zone') ? 'zone model' : 'repository' }); });
  t.after(async () => { await stop(api.server); await rm(root, { recursive: true }); });
  const response = await api.call('context?zone=model'); assert.equal(response.status, 200);
  const result = await response.json(); assert.equal(result.zone, 'model'); assert.equal(result.path, '--zone model');
  assert.deepEqual(result.report.Zones[0].Paths, ['*.go']);
  assert.deepEqual(seen, [['context', '--full', '--format', 'json'], ['context', '--zone', 'model', '--format', 'json']]);
  seen.length = 0;
  for (const query of ['path=price.go&zone=model', 'zone=--write', 'zone=model&zone=other', 'zone=']) {
    const rejected = await api.call(`context?${query}`); assert.equal(rejected.status, 400);
  }
  assert.equal(seen.length, 0);
  const unknown = await api.call('context?zone=missing'); assert.equal(unknown.status, 400); assert.equal((await unknown.json()).error.code, 'INVALID_ZONE');
  assert.equal(seen.length, 1); assert.equal(seen[0].includes('--zone'), false);
});

test('actual CLI Zone selection reports rules for glob-selected source files', async t => {
  try { await runArclint(['--help'], process.cwd()); } catch { t.skip('ArcLint executable is not installed on PATH'); return; }
  const root = await fixture(); const api = await start(root);
  t.after(async () => { await stop(api.server); await rm(root, { recursive: true }); });
  const response = await api.call('context?zone=model'); assert.equal(response.status, 200);
  const result = await response.json(); assert.equal(result.zone, 'model');
  assert.equal(result.report.Zones[0].Name, 'model');
  assert.ok(result.report.Rules.some((rule: { Summary: { ID: string } }) => rule.Summary.ID === 'model/no-panic'));
  assert.equal((await api.call('context?path=*.go')).status, 400, 'Zone support does not loosen the path boundary.');
});
