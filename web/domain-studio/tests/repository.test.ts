import { test } from 'node:test';
import assert from 'node:assert/strict';
import { createServer, request as httpRequest, type Server } from 'node:http';
import { once } from 'node:events';
import { mkdtemp, writeFile, readFile, readdir, rm, symlink } from 'node:fs/promises';
import { tmpdir } from 'node:os';
import { join } from 'node:path';
import type { AddressInfo } from 'node:net';
import { createArclintBridge, runArclint, type CommandResult, type CommandRunner } from '../server/arclint_bridge';
import { diagnosticLabel, diagnosticCounts, filterDiagnostics } from '../src/repository';

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
  assert.equal(result.pathType, 'file');
  assert.deepEqual(seen, [['context', '--format', 'json', '--', 'price.go']]);
  const routeResponse = await api.call('context?path=src%2F%5Bid%5D.ts');
  assert.equal(routeResponse.status, 200, 'Square brackets in framework route filenames are ordinary path characters.');
  assert.equal((await routeResponse.json()).pathType, 'missing', 'A hypothetical path is not source evidence.');
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

test('Report filtering retains repeated baseline occurrences and never assumes missing status', () => {
  const evidence = { kind: 'violation', ruleId: 'model/no-panic', message: 'same evidence', path: 'price.go' };
  const records = [{ ...evidence, status: 'baselined', line: 3 }, { ...evidence, status: 'baselined', line: 20 }, { ...evidence, status: 'active', line: 40 }, evidence, { kind: 'coverage', message: 'analysis unavailable' }, { kind: 'operational', severity: 'error', message: 'extension failed' }];
  assert.deepEqual(diagnosticCounts(records), { all: 6, active: 1, baselined: 2, suppressed: 0, operational: 1, coverage: 1 });
  assert.equal(filterDiagnostics(records, 'baselined').length, 2);
  assert.equal(filterDiagnostics(records, 'active')[0].line, 40);
  assert.equal('status' in evidence, false); assert.equal('fingerprint' in evidence, false);
});

test('Rule detail accepts only an exact configured identity and preserves optional evidence', async t => {
  const root = await fixture(); const calls: string[][] = [];
  const detail = { summary: rules[0], zones: ['model'], files: ['*.go'], evidence: 'line matching' };
  const api = await start(root, async args => { calls.push([...args]); return answer(args.includes('--') ? detail : rules); });
  t.after(async () => { await stop(api.server); await rm(root, { recursive: true }); });
  const response = await api.call('rule?id=model%2Fno-panic'); assert.equal(response.status, 200); assert.deepEqual(await response.json(), detail);
  assert.deepEqual(calls, [['rules', '--format', 'json'], ['rules', '--format', 'json', '--', 'model/no-panic']]);
  for (const query of ['id=--write', 'id=model%2F', 'id=model%2Fno-panic&id=second', 'id=']) assert.equal((await api.call(`rule?${query}`)).status, 400, query);
  assert.equal(calls.some(args => args.includes('--write')), false);
});

test('Pattern catalog queries offline sources and retains exact identity', async t => {
  const root = await fixture(); const catalog = { patterns: [{ reference: 'acme/library@1.2.3', source: 'vendored', rules: 2 }] };
  const api = await start(root, async args => { assert.deepEqual(args, ['patterns', '--format', 'json']); return answer(catalog); });
  t.after(async () => { await stop(api.server); await rm(root, { recursive: true }); });
  const response = await api.call('patterns'); assert.equal(response.status, 200); const result = await response.json();
  assert.deepEqual(result.patterns, catalog.patterns); assert.ok(result.queriedAt);
});

test('directory listing does not follow symlinks or escape the repository', async t => {
  const root = await fixture(); const outside = await mkdtemp(join(tmpdir(), 'studio-directory-outside-'));
  await writeFile(join(outside, 'private.txt'), 'outside'); await symlink(outside, join(root, 'shortcut'));
  const api = await start(root, async () => { throw new Error('A directory listing does not invoke the CLI.'); });
  t.after(async () => { await stop(api.server); await rm(root, { recursive: true }); await rm(outside, { recursive: true }); });
  const result = await api.call('files?directory=.').then(r => r.json()); assert.equal(result.directory, '.'); assert.equal(result.truncated, false);
  assert.ok(result.entries.some((entry: { path: string; kind: string }) => entry.path === 'price.go' && entry.kind === 'file'));
  assert.equal(result.entries.some((entry: { path: string }) => ['shortcut', 'private.txt'].includes(entry.path)), false);
  for (const directory of ['../', '/tmp', 'shortcut', '--write']) assert.equal((await api.call(`files?directory=${encodeURIComponent(directory)}`)).status, 400, directory);
});

test('real CLI read paths retain overlapping memberships and expose complete Rule contracts', async t => {
  try { await runArclint(['--help'], process.cwd()); } catch { t.skip('ArcLint executable is not installed on PATH'); return; }
  const root = await fixture();
  await writeFile(join(root, 'rules.arclint.yaml'), 'runtime: [go]\nzones:\n  model: "*.go"\n  audited: "price.go"\nrules:\n  model/no-panic:\n    on: model\n    content:\n      forbid: panic\n');
  const api = await start(root); t.after(async () => { await stop(api.server); await rm(root, { recursive: true }); });
  const query = await api.call('context?path=price.go').then(r => r.json()); assert.deepEqual(query.report.Paths[0].Zones.sort(), ['audited', 'model']);
  const detailResponse = await api.call('rule?id=model%2Fno-panic'); assert.equal(detailResponse.status, 200); const detail = await detailResponse.json();
  assert.equal(detail.summary.id, 'model/no-panic'); assert.deepEqual(detail.zones, ['model']); assert.equal(typeof detail.evidence, 'string');
  const catalogResponse = await api.call('patterns'); assert.equal(catalogResponse.status, 200); assert.ok(Array.isArray((await catalogResponse.json()).patterns));
});

const documentAction = (body: unknown): RequestInit => ({ method: 'POST', headers: { 'X-Arclint-Studio': '1', 'Content-Type': 'application/json' }, body: JSON.stringify(body) });

test('real candidate policy validation is isolated, and explicit apply writes only the reviewed file', async t => {
  const root = await fixture(); const api = await start(root);
  t.after(async () => { await stop(api.server); await rm(root, { recursive: true }); });
  const project = await api.call('project').then(r => r.json());
  const next = project.rulesYaml.replace('    on: model', '    on: model\n    severity: warning');
  const response = await api.call('documents/preview', documentAction({ document: 'rules', content: next, expectedHash: project.documentHashes.rules }));
  const preview = await response.json(); assert.equal(response.status, 200, JSON.stringify(preview));
  assert.equal(preview.before, project.rulesYaml); assert.equal(preview.after, next); assert.ok(preview.validation.rules >= 1);
  assert.equal(await readFile(join(root, 'rules.arclint.yaml'), 'utf8'), project.rulesYaml);
  const applied = await api.call('documents/apply', documentAction({ token: preview.token, content: 'unreviewed payload is ignored' }));
  assert.equal(applied.status, 200, await applied.text());
  assert.equal(await readFile(join(root, 'rules.arclint.yaml'), 'utf8'), next);
  assert.equal(await readFile(join(root, 'domain.arclint.yaml'), 'utf8'), project.domainYaml);
  assert.equal((await api.call('documents/apply', documentAction({ token: preview.token }))).status, 409);
  assert.equal((await readdir(root)).some(name => name.startsWith('.arclint-studio-')), false);
});

test('invalid policy and reused IDs for changed Constraints are rejected without touching repository files', async t => {
  const root = await fixture(); const api = await start(root);
  t.after(async () => { await stop(api.server); await rm(root, { recursive: true }); });
  const project = await api.call('project').then(r => r.json());
  const reused = await api.call('documents/preview', documentAction({ document: 'rules', content: project.rulesYaml.replace('forbid: panic', 'forbid: println'), expectedHash: project.documentHashes.rules }));
  assert.equal(reused.status, 400); assert.equal((await reused.json()).error.code, 'RULE_ID_REQUIRES_CHANGE');
  const invalid = await api.call('documents/preview', documentAction({ document: 'rules', content: project.rulesYaml.replace('on: model', 'on: missing'), expectedHash: project.documentHashes.rules }));
  assert.notEqual(invalid.status, 200); assert.match((await invalid.json()).error.message, /missing/);
  assert.equal(await readFile(join(root, 'rules.arclint.yaml'), 'utf8'), project.rulesYaml);
});

test('apply detects edits to either source document after review and retains the external change', async t => {
  const root = await fixture(); const api = await start(root);
  t.after(async () => { await stop(api.server); await rm(root, { recursive: true }); });
  const project = await api.call('project').then(r => r.json());
  const candidate = `${project.rulesYaml}\n# reviewed change\n`;
  const preview = await api.call('documents/preview', documentAction({ document: 'rules', content: candidate, expectedHash: project.documentHashes.rules })).then(r => r.json());
  assert.ok(preview.token, JSON.stringify(preview));
  const externalDomain = `${project.domainYaml}\n# external edit\n`; await writeFile(join(root, 'domain.arclint.yaml'), externalDomain);
  const apply = await api.call('documents/apply', documentAction({ token: preview.token }));
  assert.equal(apply.status, 409); assert.equal((await apply.json()).error.code, 'DOCUMENT_CHANGED');
  assert.equal(await readFile(join(root, 'rules.arclint.yaml'), 'utf8'), project.rulesYaml);
  assert.equal(await readFile(join(root, 'domain.arclint.yaml'), 'utf8'), externalDomain);
  const stale = await api.call('documents/preview', documentAction({ document: 'domain', content: project.domainYaml, expectedHash: project.documentHashes.domain }));
  assert.equal(stale.status, 409); assert.equal((await stale.json()).error.code, 'DOCUMENT_CHANGED');
});

test('domain saving uses canonical CLI validation and preserves the unchanged ruleset', async t => {
  const root = await fixture(); const api = await start(root);
  t.after(async () => { await stop(api.server); await rm(root, { recursive: true }); });
  const project = await api.call('project').then(r => r.json());
  const candidate = project.domainYaml.replace('An amount represented as a value.', 'The immutable amount of a library charge.');
  const response = await api.call('documents/preview', documentAction({ document: 'domain', content: candidate, expectedHash: project.documentHashes.domain }));
  const preview = await response.json(); assert.equal(response.status, 200, JSON.stringify(preview)); assert.equal(preview.validation.domain, true);
  assert.equal(await readFile(join(root, 'domain.arclint.yaml'), 'utf8'), project.domainYaml);
  assert.equal((await api.call('documents/apply', documentAction({ token: preview.token }))).status, 200);
  assert.equal(await readFile(join(root, 'domain.arclint.yaml'), 'utf8'), candidate);
  assert.equal(await readFile(join(root, 'rules.arclint.yaml'), 'utf8'), project.rulesYaml);
  const invalid = await api.call('documents/preview', documentAction({ document: 'domain', content: `${candidate}\nunrecognized: true\n`, expectedHash: preview.proposedHash }));
  assert.notEqual(invalid.status, 200); assert.equal(await readFile(join(root, 'domain.arclint.yaml'), 'utf8'), candidate);
});

test('write routes require local action headers, fixed document identity, and a fresh review token', async t => {
  const root = await fixture(); const api = await start(root);
  t.after(async () => { await stop(api.server); await rm(root, { recursive: true }); });
  assert.equal((await api.call('documents/preview', { method: 'POST', body: '{}' })).status, 403);
  const foreign = documentAction({ token: 'anything' }); foreign.headers = { ...foreign.headers, Origin: 'https://attacker.example' };
  assert.equal((await api.call('documents/apply', foreign)).status, 403);
  assert.equal((await api.call('documents/apply', documentAction({ token: 'not-reviewed' }))).status, 409);
  assert.equal((await api.call('documents/preview', documentAction({ document: '../other', content: 'anything', expectedHash: null }))).status, 400);
  const original = await readFile(join(root, 'domain.arclint.yaml'), 'utf8');
  await writeFile(join(root, 'original-domain.yaml'), original); await rm(join(root, 'domain.arclint.yaml')); await symlink('original-domain.yaml', join(root, 'domain.arclint.yaml'));
  const project = await api.call('project').then(r => r.json());
  const symlinked = await api.call('documents/preview', documentAction({ document: 'domain', content: original, expectedHash: project.documentHashes.domain }));
  assert.equal(symlinked.status, 400); assert.equal((await symlinked.json()).error.code, 'DOCUMENT_NOT_REGULAR');
});

test('real Baseline preview leaves files untouched; native capture adopts findings and refresh drops stale debt', async t => {
  const root = await fixture(); const api = await start(root);
  t.after(async () => { await stop(api.server); await rm(root, { recursive: true }); });
  const previewResponse = await api.call('baseline/preview', documentAction({})); const preview = await previewResponse.json();
  assert.equal(previewResponse.status, 200, JSON.stringify(preview)); assert.equal(preview.action, 'capture'); assert.equal(preview.findings, 1);
  await assert.rejects(readFile(join(root, '.arclint/baseline.v2.json')), { code: 'ENOENT' });
  assert.ok(preview.report.diagnostics.some((d: { ruleId: string; status: string }) => d.ruleId === 'model/no-panic' && d.status === 'active'));
  const response = await api.call('baseline/apply', documentAction({ token: preview.token })); const applied = await response.json();
  assert.equal(response.status, 200, JSON.stringify(applied)); assert.equal(applied.findings, 1); assert.equal(applied.action, 'capture');
  const checked = await api.call('check', post).then(r => r.json()); assert.equal(checked.exitCode, 0);
  assert.ok(checked.diagnostics.some((d: { status: string }) => d.status === 'baselined'));
  assert.equal((await api.call('baseline/apply', documentAction({ token: preview.token }))).status, 409);
  await writeFile(join(root, 'price.go'), 'package fixture\ntype Price int\nfunc Fail() {}\n');
  const refresh = await api.call('baseline/preview', documentAction({})).then(r => r.json());
  assert.equal(refresh.action, 'refresh'); assert.equal(refresh.findings, 0);
  const refreshed = await api.call('baseline/apply', documentAction({ token: refresh.token })).then(r => r.json());
  assert.equal(refreshed.findings, 0); assert.equal(refreshed.removedStale, 1);
});

test('Baseline apply rejects changed diagnostic occurrences and never invokes capture for stale review', async t => {
  const root = await fixture(); const api = await start(root);
  t.after(async () => { await stop(api.server); await rm(root, { recursive: true }); });
  const preview = await api.call('baseline/preview', documentAction({})).then(r => r.json()); assert.ok(preview.token, JSON.stringify(preview));
  await writeFile(join(root, 'price.go'), 'package fixture\ntype Price int\nfunc Fail() {}\n');
  const response = await api.call('baseline/apply', documentAction({ token: preview.token }));
  assert.equal(response.status, 409); assert.equal((await response.json()).error.code, 'BASELINE_REPORT_CHANGED');
  await assert.rejects(readFile(join(root, '.arclint/baseline.v2.json')), { code: 'ENOENT' });
});

test('Baseline adoption refuses evaluation gaps and nonexact Rules instead of assuming completed outcomes', async t => {
  const root = await fixture(); const calls: string[][] = []; let heuristic = true;
  const api = await start(root, async args => { calls.push([...args]); return answer(args[0] === 'rules' ? [{ ...rules[0], assurance: heuristic ? 'heuristic' : 'exact' }] : [{ kind: 'coverage', message: 'one subject unsupported' }]); });
  t.after(async () => { await stop(api.server); await rm(root, { recursive: true }); });
  let response = await api.call('baseline/preview', documentAction({})); assert.equal(response.status, 409); assert.equal((await response.json()).error.code, 'BASELINE_EVIDENCE_INCOMPLETE');
  heuristic = false; response = await api.call('baseline/preview', documentAction({})); assert.equal(response.status, 409);
  assert.equal((await response.json()).error.code, 'BASELINE_EVIDENCE_INCOMPLETE'); assert.equal(calls.some(args => args[0] === 'baseline'), false);
  await assert.rejects(readFile(join(root, '.arclint/baseline.v2.json')), { code: 'ENOENT' });
});

test('Baseline hashes and parent path protections prevent overwriting a changed or redirected adoption file', async t => {
  const root = await fixture(); const api = await start(root);
  t.after(async () => { await stop(api.server); await rm(root, { recursive: true }); });
  let preview = await api.call('baseline/preview', documentAction({})).then(r => r.json());
  assert.equal((await api.call('baseline/apply', documentAction({ token: preview.token }))).status, 200);
  preview = await api.call('baseline/preview', documentAction({})).then(r => r.json());
  const filename = join(root, '.arclint/baseline.v2.json'); const external = `${await readFile(filename, 'utf8')}\n`;
  await writeFile(filename, external);
  const response = await api.call('baseline/apply', documentAction({ token: preview.token })); assert.equal(response.status, 409);
  assert.equal((await response.json()).error.code, 'BASELINE_INPUTS_CHANGED'); assert.equal(await readFile(filename, 'utf8'), external);
  await rm(join(root, '.arclint'), { recursive: true });
  const outside = await mkdtemp(join(tmpdir(), 'studio-baseline-outside-')); t.after(async () => rm(outside, { recursive: true }));
  await symlink(outside, join(root, '.arclint'));
  const refused = await api.call('baseline/preview', documentAction({})); assert.equal(refused.status, 400); assert.equal((await refused.json()).error.code, 'UNSAFE_BASELINE_PATH');
  assert.deepEqual(await readdir(outside), []);
});
