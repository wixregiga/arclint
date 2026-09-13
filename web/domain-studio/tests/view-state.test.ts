import { test } from 'node:test';
import assert from 'node:assert/strict';
import type { Concept, DomainContext, DomainProject } from '../src/contracts';
import { projectView, keeperFor } from '../src/view-state';
const context = (id: string): DomainContext => ({ id, name: id, description: id, color: '#aabbcc', position: [0, 0, 0] });
const concept = (id: string, contextId: string, extra: Partial<Concept> = {}): Concept => ({ id, name: id, definition: id, kind: 'unclassified', contextId, position: [0, 1, 0], invariants: [], ...extra });
const project = (contexts: DomainContext[], concepts: Concept[] = []): DomainProject => ({ version: 1, name: 'General model', description: '', contexts, concepts, relationships: [] });

test('world shows only actual contexts, with bounded pages reaching every context', () => {
  const model = project(Array.from({ length: 19 }, (_, i) => context(`c-${i}`)), [concept('not-world', 'c-0')]);
  const first = projectView(model); assert.equal(first.level, 'world'); assert.equal(first.total, 19); assert.equal(first.pages, 3);
  const pages = Array.from({ length: first.pages }, (_, page) => projectView(model, { page }));
  assert.ok(pages.every(view => view.ids.length <= 8));
  assert.deepEqual(pages.flatMap(view => view.ids), model.contexts.map(c => c.id));
});

test('context pages contain actual members only, without ancestor nodes or external crowds', () => {
  const model = project([context('a'), context('b')], [...Array.from({ length: 12 }, (_, i) => concept(`a-${i}`, 'a')), concept('external', 'b')]);
  const view = projectView(model, { scopeId: 'a', page: 1 });
  assert.equal(view.level, 'context'); assert.equal(view.selectedId, null); assert.equal(view.contextId, 'a');
  assert.deepEqual(view.ids, ['a-8', 'a-9', 'a-10', 'a-11']); assert.equal(view.total, 12);
  const selected = projectView(model, { scopeId: 'a', selectedId: 'b' });
  assert.equal(selected.level, 'context'); assert.equal(selected.contextId, 'b'); assert.deepEqual(selected.ids, ['external']);
});

test('detail includes only direct relations or recorded ownership, with same-context authored order first', () => {
  const model = project([context('a'), context('b'), context('c')], [concept('far', 'b'), concept('unrelated', 'a'), concept('member', 'a', { ownerId: 'root' }), concept('peer', 'a'), concept('root', 'a', { kind: 'aggregate' }), concept('owner', 'a', { kind: 'aggregate' })]);
  model.relationships = [
    { id: 'r1', source: 'root', target: 'far', label: 'uses' }, { id: 'r2', source: 'peer', target: 'root', label: 'calls' },
    { id: 'r3', source: 'root', target: 'c', label: 'publishes' }, { id: 'r4', source: 'root', target: 'a', label: 'belongs here' },
    { id: 'r5', source: 'root', target: 'peer', label: 'also uses' }, { id: 'r6', source: 'root', target: 'owner', label: 'references' },
  ];
  const view = projectView(model, { scopeId: 'a', selectedId: 'root' });
  assert.equal(view.level, 'detail'); assert.equal(view.anchorId, 'root');
  assert.deepEqual(view.ids, ['root', 'member', 'peer', 'owner', 'far', 'c']); assert.equal(view.relatedCount, 5);
  assert.deepEqual(view.externalIds, ['far', 'c']);
  assert.deepEqual(projectView(model, { scopeId: 'a', selectedId: 'member' }).ids, ['member', 'root']);
});

test('detail retains the selected anchor while all neighbors remain reachable through seven-slot pages', () => {
  const nodes = Array.from({ length: 25 }, (_, i) => concept(`n-${i}`, i < 15 ? 'a' : 'b'));
  const model = project([context('a'), context('b')], [concept('anchor', 'a'), ...nodes]);
  model.relationships = nodes.map(node => ({ id: `r-${node.id}`, source: 'anchor', target: node.id, label: 'relates' }));
  const first = projectView(model, { scopeId: 'a', selectedId: 'anchor' }); assert.equal(first.total, 26); assert.equal(first.pages, 4);
  const pages = Array.from({ length: first.pages }, (_, page) => projectView(model, { scopeId: 'a', selectedId: 'anchor', page }));
  for (const view of pages) { assert.equal(view.ids[0], 'anchor'); assert.ok(view.ids.length <= 8); assert.equal(new Set(view.ids).size, view.ids.length); assert.equal(view.visibleRelatedCount, view.ids.length - 1); }
  assert.deepEqual(pages.flatMap(view => view.ids.slice(1)), nodes.map(n => n.id));
  assert.equal(projectView(model, { selectedId: 'anchor', page: 999 }).page, 3);
});

test('external detail preserves active scope and records actual membership separately', () => {
  const model = project([context('a'), context('b')], [concept('external', 'b')]);
  const view = projectView(model, { scopeId: 'a', selectedId: 'external' });
  assert.equal(view.contextId, 'a'); assert.equal(view.subjectContextId, 'b'); assert.deepEqual(view.externalIds, ['external']);
  assert.equal(projectView(model, { selectedId: 'external' }).contextId, 'b');
});

test('relationship detail displays exact real endpoints without manufacturing a relationship node', () => {
  const model = project([context('a'), context('b')], [concept('one', 'a'), concept('two', 'b'), concept('unrelated', 'a')]);
  model.relationships = [{ id: 'r', source: 'one', target: 'two', label: 'uses' }, { id: 'cr', source: 'a', target: 'b', label: 'conformist' }];
  const view = projectView(model, { scopeId: 'a', selectedId: 'r', page: 99 });
  assert.equal(view.level, 'detail'); assert.equal(view.selectedId, 'r'); assert.equal(view.anchorId, null);
  assert.deepEqual(view.ids, ['one', 'two']); assert.equal(view.page, 0); assert.equal(view.pages, 1);
  assert.deepEqual(projectView(model, { selectedId: 'cr' }).ids, ['a', 'b']);
});

test('stale selection/scope and invalid pages safely fall back to actual current model', () => {
  const model = project([context('a')], [concept('one', 'a')]);
  assert.deepEqual(projectView(model, { selectedId: 'missing', scopeId: 'a' }).ids, ['one']);
  assert.equal(projectView(model, { selectedId: 'missing', scopeId: 'a' }).level, 'context');
  assert.equal(projectView(model, { selectedId: 'missing', scopeId: 'missing' }).level, 'world');
  assert.equal(projectView(model, { selectedId: 'one', scopeId: 'missing' }).contextId, 'a');
  const empty = projectView(project([])); assert.deepEqual(empty.ids, []); assert.equal(empty.total, 0); assert.equal(empty.pages, 1);
  for (const page of [-9, NaN, Infinity]) assert.equal(projectView(model, { page }).page, 0);
});

test('projection is deterministic and pure; unrelated additions and labels/positions do not reorder the working set', () => {
  const model = project([context('a'), context('b')], Array.from({ length: 15 }, (_, i) => concept(`n-${i}`, 'a')));
  const before = structuredClone(model); const view = projectView(model, { scopeId: 'a', page: 1 });
  assert.deepEqual(model, before); assert.deepEqual(projectView(JSON.parse(JSON.stringify(model)), { scopeId: 'a', page: 1 }), view);
  model.concepts.push(concept('irrelevant', 'b')); model.concepts[0].name = 'No inferred priority'; model.concepts[0].position = [500, 50, 20];
  assert.deepEqual(projectView(model, { scopeId: 'a', page: 1 }).ids, view.ids);
  view.ids.push('fake'); assert.equal(projectView(model, { scopeId: 'a', page: 1 }).ids.includes('fake'), false);
});

test('forty contexts and five hundred concepts remain bounded in every level and every detail page', () => {
  const model = project(Array.from({ length: 40 }, (_, i) => context(`c-${i}`)), Array.from({ length: 500 }, (_, i) => concept(`n-${i}`, `c-${i % 40}`)));
  model.relationships = model.concepts.slice(1).map(n => ({ id: `r-${n.id}`, source: 'n-0', target: n.id, label: 'related' }));
  for (const request of [{}, { scopeId: 'c-0' }, { scopeId: 'c-0', selectedId: 'n-0' }]) {
    const first = projectView(model, request);
    for (let page = 0; page < first.pages; page++) assert.ok(projectView(model, { ...request, page }).ids.length <= 8);
  }
});

test('Onyx remains the same interface guide across every view and task', () => {
  const model = project([context('a')], [concept('one', 'a')]);
  const views = [projectView(model), projectView(model, { scopeId: 'a' }), projectView(model, { selectedId: 'one' })];
  assert.deepEqual(views.map(view => keeperFor(view, 'meaning').name), ['Onyx', 'Onyx', 'Onyx']);
  for (const view of views) {
    const a = keeperFor(view, 'meaning'), b = keeperFor(view, 'governance');
    assert.match(a.role, /interface role/); assert.notEqual(a.prompt, b.prompt); assert.notEqual(a.action, b.action);
    assert.equal(model.concepts.some(n => n.name === a.name), false);
  }
});
