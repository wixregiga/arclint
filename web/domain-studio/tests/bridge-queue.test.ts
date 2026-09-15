import { test, type TestContext } from 'node:test';
import assert from 'node:assert/strict';
import { createServer } from 'node:http';
import { once } from 'node:events';
import { mkdtemp, mkdir, writeFile, rm, symlink, unlink } from 'node:fs/promises';
import { tmpdir } from 'node:os';
import { join } from 'node:path';
import type { AddressInfo } from 'node:net';
import { createArclintBridge, type CommandResult, type CommandRunner } from '../server/arclint_bridge';

const pause = (milliseconds: number) => new Promise(resolve => setTimeout(resolve, milliseconds));
const context = { Scope: 'repository', Languages: ['go'], RuleCount: 0, Zones: [], Rules: null, Paths: null };
const answer = (args: readonly string[]): CommandResult => ({ stdout: JSON.stringify(args[0] === 'context' ? context : []), stderr: '', exitCode: 0 });
const post = { method: 'POST', headers: { 'X-Arclint-Studio': '1' } };
function deferred() { let release!: () => void; const promise = new Promise<void>(resolve => { release = resolve; }); return { promise, release }; }
async function until(predicate: () => boolean) {
  const deadline = Date.now() + 2500;
  while (!predicate() && Date.now() < deadline) await pause(5);
  assert.ok(predicate(), 'The expected command state was not reached.');
}
async function fixture(t: TestContext, run: CommandRunner, configuredRoot?: string) {
  const root = configuredRoot ?? await mkdtemp(join(tmpdir(), 'studio-queue-'));
  if (!configuredRoot) {
    await writeFile(join(root, 'rules.arclint.yaml'), 'runtime: [go]\n');
    t.after(() => rm(root, { recursive: true, force: true }));
  }
  const bridge = createArclintBridge({ root, run });
  const server = createServer((request, response) => bridge(request, response, () => response.end()));
  server.listen(0, '127.0.0.1'); await once(server, 'listening');
  t.after(async () => { bridge.dispose(); server.closeAllConnections(); await new Promise<void>(resolve => server.close(() => resolve())); });
  return { bridge, call: (path: string, init?: RequestInit) => fetch(`http://127.0.0.1:${(server.address() as AddressInfo).port}/api/arclint/${path}`, init) };
}

test('queued project queries get their turn before later path queries and never exceed four active commands', async t => {
  const gate = deferred(); t.after(gate.release);
  const started: string[][] = []; let active = 0, peak = 0;
  const api = await fixture(t, async args => {
    started.push([...args]); peak = Math.max(peak, ++active);
    try { await gate.promise; return answer(args); } finally { active--; }
  });
  const initial = Array.from({ length: 4 }, (_, index) => api.call(`context?path=initial-${index}.go`));
  await until(() => started.length === 4);
  let projectSettled = false;
  const project = api.call('project').then(response => { projectSettled = true; return response; });
  await pause(40);
  assert.equal(projectSettled, false, 'Saturation must queue project loading rather than return an immediate BUSY.');
  const later = Array.from({ length: 6 }, (_, index) => api.call(`context?path=later-${index}.go`));
  await pause(40); assert.equal(started.length, 4);
  gate.release();
  for (const response of await Promise.all([...initial, project, ...later])) assert.equal(response.status, 200);
  assert.equal(peak, 4); assert.equal(started.length, 12);
  assert.deepEqual(started.slice(4, 6).map(args => args.join(' ')).sort(), ['context --full --format json', 'rules --format json']);
});

test('identical queued reads and checks share one execution, including after a command fails', async t => {
  const gate = deferred(); t.after(gate.release);
  const started: string[][] = [];
  const api = await fixture(t, async args => {
    started.push([...args]); await gate.promise;
    if (args.at(-1) === 'failed.go') throw new Error('Command failed.');
    return answer(args);
  });
  const initial = ['failed.go', 'second.go', 'third.go', 'fourth.go'].map(path => api.call(`context?path=${path}`));
  await until(() => started.length === 4);
  const checks = [api.call('check', post), api.call('check', post)];
  const paths = [api.call('context?path=shared.go'), api.call('context?path=shared.go')];
  await pause(40); assert.equal(started.length, 4);
  gate.release();
  assert.equal((await initial[0]).status, 500);
  for (const response of await Promise.all([...initial.slice(1), ...checks, ...paths])) assert.equal(response.status, 200);
  assert.equal(started.filter(args => args[0] === 'check').length, 1);
  assert.equal(started.filter(args => args.at(-1) === 'shared.go').length, 1);
});

test('only requests beyond the bounded queue return BUSY and an existing queued request still shares', async t => {
  const gate = deferred(); t.after(gate.release); let started = 0;
  const api = await fixture(t, async args => { started++; await gate.promise; return answer(args); });
  const initial = Array.from({ length: 4 }, (_, index) => api.call(`context?path=active-${index}.go`));
  await until(() => started === 4);
  const queued = Array.from({ length: 32 }, (_, index) => api.call(`context?path=queued-${index}.go`));
  await pause(100);
  const overflow = await api.call('context?path=overflow.go');
  assert.equal(overflow.status, 429); assert.equal((await overflow.json()).error.code, 'BUSY');
  const duplicate = api.call('context?path=queued-0.go');
  await pause(20); gate.release();
  for (const response of await Promise.all([...initial, ...queued, duplicate])) assert.equal(response.status, 200);
  assert.equal(started, 36);
});

test('command sharing includes the resolved repository root', async t => {
  const parent = await mkdtemp(join(tmpdir(), 'studio-queue-roots-'));
  t.after(() => rm(parent, { recursive: true, force: true }));
  const firstRoot = join(parent, 'first'), secondRoot = join(parent, 'second'), link = join(parent, 'current');
  await mkdir(firstRoot); await mkdir(secondRoot); await symlink(firstRoot, link);
  const gate = deferred(); t.after(gate.release); const roots: string[] = [];
  const api = await fixture(t, async (args, cwd) => { roots.push(cwd); await gate.promise; return answer(args); }, link);
  const first = api.call('check', post); await until(() => roots.length === 1);
  await unlink(link); await symlink(secondRoot, link);
  const second = api.call('check', post); await until(() => roots.length === 2);
  gate.release(); assert.equal((await first).status, 200); assert.equal((await second).status, 200);
  assert.deepEqual(roots, [firstRoot, secondRoot]);
});

test('disposal rejects queued requests without starting them', async t => {
  const gate = deferred(); t.after(gate.release); let started = 0;
  const api = await fixture(t, async args => { started++; await gate.promise; return answer(args); });
  const initial = Array.from({ length: 4 }, (_, index) => api.call(`context?path=active-${index}.go`));
  await until(() => started === 4);
  const queued = api.call('project'); await pause(40); api.bridge.dispose();
  const stopped = await queued; assert.equal(stopped.status, 503); assert.equal((await stopped.json()).error.code, 'BRIDGE_STOPPED');
  gate.release(); for (const response of await Promise.all(initial)) assert.equal(response.status, 200);
  assert.equal(started, 4);
});
