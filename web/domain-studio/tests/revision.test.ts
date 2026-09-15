import { test } from 'node:test';
import assert from 'node:assert/strict';
import { createServer, type Server } from 'node:http';
import { once } from 'node:events';
import { EventEmitter } from 'node:events';
import { mkdtemp, mkdir, writeFile, rename, rm, symlink } from 'node:fs/promises';
import { tmpdir } from 'node:os';
import { join } from 'node:path';
import type { AddressInfo } from 'node:net';
import { arclintBridgePlugin, createArclintBridge, createRepositoryRevisionMonitor, type RepositoryRevision, type RepositoryRevisionMonitor } from '../server/arclint_bridge';

const pause = (milliseconds: number) => new Promise(resolve => setTimeout(resolve, milliseconds));
async function fixture() {
  const root = await mkdtemp(join(tmpdir(), 'studio-revision-'));
  await mkdir(join(root, 'src')); await mkdir(join(root, '.arclint'));
  await writeFile(join(root, 'src', 'value.go'), 'package source\n');
  await writeFile(join(root, 'rules.arclint.yaml'), 'runtime: [go]\n');
  return root;
}
async function changed(monitor: RepositoryRevisionMonitor, before: string) {
  const deadline = Date.now() + 2500;
  while (Date.now() < deadline) {
    const result = await monitor.snapshot();
    if (result.revision !== before) return result;
    await pause(20);
  }
  assert.fail('The repository revision did not change after a source mutation.');
}
async function stop(server: Server) { server.closeAllConnections(); await new Promise<void>((resolve, reject) => server.close(error => error ? reject(error) : resolve())); }

test('revision endpoint is read-only and detects source, Rules, Domain, and Baseline changes without invoking CLI', async t => {
  const root = await fixture(); let commands = 0;
  const bridge = createArclintBridge({ root, run: async () => { commands++; throw new Error('Revision reads must not invoke ArcLint.'); } });
  const server = createServer((request, response) => bridge(request, response, () => { response.statusCode = 404; response.end(); }));
  server.listen(0, '127.0.0.1'); await once(server, 'listening');
  t.after(async () => { bridge.dispose(); await stop(server); await rm(root, { recursive: true }); });
  const url = `http://127.0.0.1:${(server.address() as AddressInfo).port}/api/arclint/revision`;
  const firstResponse = await fetch(url); assert.equal(firstResponse.status, 200); assert.equal(firstResponse.headers.get('cache-control'), 'no-store');
  let previous = await firstResponse.json() as RepositoryRevision;
  assert.equal(previous.watching, true); assert.equal(typeof previous.revision, 'string'); assert.ok(Date.parse(previous.changedAt));
  assert.deepEqual(await fetch(url).then(response => response.json()), previous, 'Reading the revision does not change it.');
  for (const [path, contents] of [
    ['src/value.go', 'package source\ntype Value int\n'],
    ['rules.arclint.yaml', 'runtime: [go, typescript]\n'],
    ['domain.arclint.yaml', 'version: 1\nproject: Current\ncontexts: {}\n'],
    ['.arclint/baseline.v2.json', '{"version":2}\n'],
  ]) {
    await writeFile(join(root, path), contents);
    const monitor = { snapshot: async () => fetch(url).then(response => response.json()) as Promise<RepositoryRevision>, dispose() {} };
    previous = await changed(monitor, previous.revision); assert.equal(previous.watching, true, path);
    await pause(30); previous = await monitor.snapshot();
  }
  assert.equal(commands, 0);
  assert.equal((await fetch(url, { method: 'POST', headers: { 'X-Arclint-Studio': '1' } })).status, 405);
  assert.equal((await fetch(url, { headers: { Origin: 'https://elsewhere.example' } })).status, 403);
});

test('new directories, atomic replacement, rename, and deletion remain observable', async t => {
  const root = await fixture(), monitor = createRepositoryRevisionMonitor(root);
  t.after(async () => { monitor.dispose(); await rm(root, { recursive: true }); });
  let previous = await monitor.snapshot();
  await mkdir(join(root, 'new', 'nested'), { recursive: true });
  await writeFile(join(root, 'new', 'nested', 'module.ts'), 'export const n = 1;\n');
  previous = await changed(monitor, previous.revision);
  await pause(40); previous = await monitor.snapshot();
  await writeFile(join(root, 'new', 'nested', 'module.ts'), 'export const n = 2;\n');
  previous = await changed(monitor, previous.revision);
  await rename(join(root, 'src'), join(root, 'old-src'));
  await mkdir(join(root, 'src')); await writeFile(join(root, 'src', 'replacement.go'), 'package replacement\n');
  previous = await changed(monitor, previous.revision);
  await pause(40); previous = await monitor.snapshot();
  await writeFile(join(root, 'src', 'replacement.go'), 'package replacement\ntype New int\n');
  previous = await changed(monitor, previous.revision);
  await pause(40); previous = await monitor.snapshot();
  await rm(join(root, 'new'), { recursive: true });
  assert.equal((await changed(monitor, previous.revision)).watching, true);
});

test('dependency/generated output and outside symlink targets never cause watch storms', async t => {
  const root = await fixture(), outside = await mkdtemp(join(tmpdir(), 'studio-revision-outside-'));
  const ignored = ['.git', 'node_modules', 'dist', 'build', 'test-results', 'playwright-report', 'coverage'];
  for (const directory of ignored) await mkdir(join(root, directory));
  await symlink(outside, join(root, 'external'));
  const monitor = createRepositoryRevisionMonitor(root);
  t.after(async () => { monitor.dispose(); await rm(root, { recursive: true }); await rm(outside, { recursive: true }); });
  const before = await monitor.snapshot();
  for (const directory of ignored) await writeFile(join(root, directory, 'generated.ts'), 'output');
  await writeFile(join(root, '.arclint-studio-generated.tmp'), 'temporary candidate');
  await writeFile(join(outside, 'source.go'), 'package elsewhere');
  await pause(120);
  assert.equal((await monitor.snapshot()).revision, before.revision);
});

test('directory cap and disposal report unavailable coverage explicitly', async t => {
  const root = await fixture(), capped = createRepositoryRevisionMonitor(root, 1), monitor = createRepositoryRevisionMonitor(root);
  t.after(async () => { capped.dispose(); monitor.dispose(); await rm(root, { recursive: true }); });
  const limited = await capped.snapshot(); assert.equal(limited.watching, false); assert.match(limited.reason!, /directory limit/);
  await monitor.snapshot(); monitor.dispose();
  const disposed = await monitor.snapshot(); assert.equal(disposed.watching, false); assert.match(disposed.reason!, /stopped/);
  await writeFile(join(root, 'src', 'value.go'), 'package changed'); await pause(80);
  assert.equal((await monitor.snapshot()).revision, disposed.revision);
});

test('both Vite dev and preview HTTP close dispose their revision monitors', async t => {
  const root = await fixture(); t.after(() => rm(root, { recursive: true }));
  for (const hookName of ['configureServer', 'configurePreviewServer'] as const) {
    const plugin = arclintBridgePlugin({ root }); let middleware: ReturnType<typeof createArclintBridge> | undefined;
    const events = new EventEmitter();
    const hook = plugin[hookName]; assert.equal(typeof hook, 'function');
    (hook as (server: unknown) => void)({ httpServer: events, middlewares: { use(value: ReturnType<typeof createArclintBridge>) { middleware = value; } } });
    assert.ok(middleware);
    const server = createServer((request, response) => middleware!(request, response, () => response.end()));
    server.listen(0, '127.0.0.1'); await once(server, 'listening');
    try {
      const url = `http://127.0.0.1:${(server.address() as AddressInfo).port}/api/arclint/revision`;
      assert.equal((await fetch(url).then(response => response.json())).watching, true);
      events.emit('close');
      const response = await fetch(url); assert.equal(response.status, 503);
      assert.equal((await response.json()).error.code, 'REVISION_MONITOR_STOPPED');
    } finally { middleware.dispose(); await stop(server); }
  }
});
