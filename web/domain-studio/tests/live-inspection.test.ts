import { test } from 'node:test';
import assert from 'node:assert/strict';
import { startLiveInspection } from '../src/live-inspection';

const pause = (ms: number) => new Promise(resolve => setTimeout(resolve, ms));
async function until(predicate: () => boolean) {
  const end = Date.now() + 2000;
  while (!predicate() && Date.now() < end) await pause(5);
  assert.ok(predicate(), 'Expected lifecycle transition did not occur');
}
const snapshot = (revision = '1') => ({revision, changedAt:'2026-09-14T12:00:00Z', watching:true});

test('opens with one automatic check and leaves an unchanged repository alone', async () => {
  let checks = 0, polls = 0;
  const live = startLiveInspection({pollMs:10,settleMs:5,revision:async () => { polls++; return snapshot(); },check:async valid => { assert.ok(valid()); checks++; },watchState() {}});
  try { await until(() => polls >= 8); assert.equal(checks,1); } finally { live.dispose(); }
});

test('a source change during inspection invalidates its result and coalesces a burst into a fresh check', async () => {
  let token = '1', observed = '', checks = 0, concurrent = 0, peak = 0;
  const publications: string[] = [];
  let release!: () => void;
  const waiting = new Promise<void>(resolve => release = resolve);
  const live = startLiveInspection({pollMs:10,settleMs:20,revision:async () => { observed = token; return snapshot(token); },
    check:async valid => { const current = token; checks++; concurrent++; peak = Math.max(peak,concurrent); if (checks === 1) await waiting; if (valid()) publications.push(current); concurrent--; },watchState() {}});
  try {
    await until(() => checks === 1); token = '2'; await until(() => observed === '2');
    token = '3'; await until(() => observed === '3'); release();
    await until(() => publications.length === 1); assert.deepEqual(publications,['3']); assert.equal(checks,2); assert.equal(peak,1);
  } finally { release(); live.dispose(); }
});

test('disposing cancels polling and prevents a pending result from publishing', async () => {
  let validResult = true, polls = 0, started = false;
  let release!: () => void;
  const waiting = new Promise<void>(resolve => release = resolve);
  const live = startLiveInspection({pollMs:10,settleMs:5,revision:async () => { polls++; return snapshot(); },check:async valid => { started = true; await waiting; validResult = valid(); },watchState() {}});
  await until(() => started); live.dispose(); const before = polls; release(); await pause(40);
  assert.equal(validResult,false); assert.equal(polls,before);
});

test('repository bootstrap finishes before automatic inspection can begin', async () => {
  let release!: () => void, checks = 0;
  const ready = new Promise<void>(resolve => release = resolve);
  const live = startLiveInspection({ready,pollMs:10,settleMs:5,revision:async () => snapshot(),check:async () => { checks++; },watchState() {}});
  try { await pause(30); assert.equal(checks,0); release(); await until(() => checks === 1); } finally { live.dispose(); }
});

test('an unavailable watcher still checks initially and exposes the coverage gap once per state', async () => {
  let checks = 0, note = '';
  const live = startLiveInspection({pollMs:10,settleMs:5,revision:async () => { throw new Error('offline'); },check:async () => { checks++; },watchState(value) { note = value; }});
  try { await until(() => checks === 1); await pause(50); assert.equal(checks,1); assert.match(note,/unavailable/); } finally { live.dispose(); }
});

test('a valid unavailable-watcher response does not immediately repeat an unchanged initial check', async () => {
  let checks = 0, polls = 0;
  const live = startLiveInspection({pollMs:10,settleMs:5,revision:async () => { polls++; return {...snapshot(),watching:false,reason:'Directory limit reached.'}; },check:async () => { checks++; },watchState() {}});
  try { await until(() => polls >= 8); assert.equal(checks,1); } finally { live.dispose(); }
});

test('explicit invalidation schedules one fresh check despite an unchanged repository token', async () => {
  let checks = 0;
  const live = startLiveInspection({pollMs:10,settleMs:15,revision:async () => snapshot(),check:async () => { checks++; },watchState() {}});
  try {
    await until(() => checks === 1);
    live.invalidate(); live.invalidate(); live.invalidate();
    await until(() => checks === 2); await pause(60); assert.equal(checks,2);
    live.dispose(); live.invalidate(); await pause(30); assert.equal(checks,2);
  } finally { live.dispose(); }
});

class VisibilityDocument extends EventTarget {
  visibilityState: 'visible' | 'hidden' = 'visible';
  show(visibility: 'visible' | 'hidden') { this.visibilityState = visibility; this.dispatchEvent(new Event('visibilitychange')); }
}
function fakeDocument() {
  const previous = Object.getOwnPropertyDescriptor(globalThis, 'document');
  const value = new VisibilityDocument();
  Object.defineProperty(globalThis, 'document', { value, configurable: true });
  return { value, restore() { if (previous) Object.defineProperty(globalThis, 'document', previous); else Reflect.deleteProperty(globalThis, 'document'); } };
}

test('hiding stops revision polls, invalidates an unfinished check, and polls before resuming', async () => {
  const document = fakeDocument();
  let checks = 0, polls = 0, token = '1';
  const publications: string[] = [];
  let release!: () => void;
  const waiting = new Promise<void>(resolve => release = resolve);
  const live = startLiveInspection({pollMs:10,settleMs:5,revision:async () => { polls++; return snapshot(token); },
    check:async valid => { const checked = token; checks++; if (checks === 1) await waiting; if (valid()) publications.push(checked); },watchState() {}});
  try {
    await until(() => checks === 1); document.value.show('hidden'); const before = polls;
    token = '2'; release(); await pause(60);
    assert.equal(polls,before); assert.equal(checks,1); assert.deepEqual(publications,[]);
    document.value.show('visible'); await until(() => publications.length === 1);
    assert.deepEqual(publications,['2']); assert.equal(checks,2);
  } finally { release(); live.dispose(); document.restore(); }
});

test('visibility resume cannot bypass the repository bootstrap barrier', async () => {
  const document = fakeDocument(); document.value.show('hidden');
  let polls = 0, checks = 0, release!: () => void;
  const ready = new Promise<void>(resolve => release = resolve);
  const live = startLiveInspection({ready,pollMs:10,settleMs:5,revision:async () => { polls++; return snapshot(); },check:async () => { checks++; },watchState() {}});
  try {
    document.value.show('visible'); await pause(30); assert.equal(polls,0); assert.equal(checks,0);
    release(); await until(() => checks === 1);
  } finally { release(); live.dispose(); document.restore(); }
});

test('a delayed revision from before hiding cannot schedule a check on resume', async () => {
  const document = fakeDocument();
  let polls = 0, checked = '', token = '1', release!: () => void;
  const waiting = new Promise<void>(resolve => release = resolve);
  const live = startLiveInspection({pollMs:10,settleMs:5,
    revision:async () => { const observed = token; polls++; if (polls === 1) await waiting; return snapshot(observed); },
    check:async () => { checked = token; },watchState() {}});
  try {
    await until(() => polls === 1); document.value.show('hidden'); token = '2'; document.value.show('visible'); release();
    await until(() => checked !== ''); assert.equal(checked,'2'); assert.ok(polls >= 2);
  } finally { release(); live.dispose(); document.restore(); }
});
