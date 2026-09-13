import { test } from 'node:test';
import assert from 'node:assert/strict';
import { readFileSync } from 'node:fs';
import { parse } from 'yaml';
import { assertProject, cloneProject, createEmptyProject, createExampleProject, diffProjects, validateProject } from '../src/domain';
import { exportDomainYaml, exportProject, importProject, parsePattern } from '../src/serialization';
const source = readFileSync(new URL('../../../domain.arclint.yaml', import.meta.url), 'utf8');

test('JSON export preserves arbitrary domains, positions, metadata and Unicode', () => {
  const project = createExampleProject();
  project.name = 'Shipping / 配送'; project.concepts[0].name = 'Shipment';
  project.sourceDocument = { arbitrary: { aliases: ['配送'], external: true } };
  assert.deepEqual(importProject(exportProject(project)), project);
  const copied = cloneProject(project); copied.concepts[0].name = 'Other';
  assert.equal(project.concepts[0].name, 'Shipment');
});

test('actual repository canonical YAML survives roundtrip without losing metadata', () => {
  const project = importProject(source);
  assert.equal(project.name, 'arclint'); assert.equal(project.contexts.length, 5);
  assert.ok(project.concepts.some(c => c.kind === 'entity' && c.name === 'Zone'));
  assert.ok(project.concepts.some(c => c.kind === 'unclassified'));
  assert.deepEqual(parse(exportDomainYaml(project)), parse(source));
  assert.deepEqual(importProject(exportProject(project)), project);
});

test('canonical definitions/invariants edits preserve identity, aliases, assertions, and keys', () => {
  const input = `version: 1\nproject: shop\ncontexts:\n  orders:\n    definition: Orders\n    aggregates:\n      Order:\n        definition: Original\n        identity: OrderID\n        aliases: [Purchase]\n        invariants:\n          lines-present: Has lines\n        assertions:\n          ready:\n            on: Confirm\n            statement: Is ready\n        repository: Orders\n    value_objects:\n      OrderID:\n        definition: Identity\n`;
  const project = importProject(input); const order = project.concepts.find(c => c.name === 'Order')!;
  order.definition = 'Revised definition'; order.invariants[0] = 'Order has at least one line.';
  const result = parse(exportDomainYaml(project)).contexts.orders.aggregates.Order;
  assert.equal(result.identity, 'OrderID'); assert.deepEqual(result.aliases, ['Purchase']);
  assert.deepEqual(result.assertions.ready, { on: 'Confirm', statement: 'Is ready' });
  assert.equal(result.invariants['lines-present'], 'Order has at least one line.');
});

test('canonical renames preserve unrelated source metadata and remain editable', () => {
  const project = importProject(source); project.concepts[0].name = 'Renamed';
  const revised = importProject(exportDomainYaml(project));
  assert.ok(revised.concepts.some(c => c.name === 'Renamed'));
  assert.deepEqual(revised.sourceDocument?.relations, project.sourceDocument?.relations);
  assert.equal(importProject(exportProject(project)).concepts[0].name, 'Renamed');
});

test('adding and removing domain entries preserves unrelated assertions, relations and invariant keys', () => {
  const project = importProject(source);
  const before = parse(source);
  const context = project.contexts.find(c => c.name === 'rule')!;
  project.concepts.push({id:'new-policy-value',name:'PolicyRevision',kind:'value_object',contextId:context.id,definition:'The revision of an authored policy.',position:[1,0,1],invariants:['A revision is positive.']});
  const revised = parse(exportDomainYaml(project));
  assert.equal(revised.contexts.rule.value_objects.PolicyRevision.definition,'The revision of an authored policy.');
  assert.deepEqual(revised.contexts.rule.aggregates,before.contexts.rule.aggregates);
  assert.deepEqual(revised.relations,before.relations);
  project.concepts = project.concepts.filter(c => c.id !== 'new-policy-value');
  assert.deepEqual(parse(exportDomainYaml(project)),before);
});

test('typed operation assertions can be added and edited without rewriting invariants', () => {
  const project = importProject(source);
  const root = project.concepts.find(c => c.kind === 'aggregate')!;
  const previous = [...root.invariants];
  root.assertions = [...root.assertions ?? [],{key:'review-complete',on:'Review',statement:'Every proposed edit is recorded.'}];
  const revised = importProject(exportDomainYaml(project));
  const written = revised.concepts.find(c => c.name === root.name)!;
  assert.deepEqual(written.assertions?.find(a => a.key === 'review-complete'),{key:'review-complete',on:'Review',statement:'Every proposed edit is recorded.'});
  assert.deepEqual(written.invariants,previous);
  written.assertions!.find(a => a.key === 'review-complete')!.statement = 'Every accepted edit is recorded.';
  assert.equal(importProject(exportDomainYaml(revised)).concepts.find(c => c.name === root.name)!.assertions!.find(a => a.key === 'review-complete')!.statement,'Every accepted edit is recorded.');
});

test('incompatible reclassification cannot silently erase operation contracts', () => {
  const project = importProject('version: 1\nproject: shop\ncontexts:\n  orders:\n    definition: Orders\n    aggregates:\n      Order:\n        definition: Order\n        identity: OrderID\n        assertions:\n          ready:\n            on: Confirm\n            statement: Is ready\n');
  project.concepts[0].kind = 'value_object';
  assert.throws(() => exportDomainYaml(project),/Assertions require an aggregate/);
});

test('unconfirmed sample concepts export as questions, never invented aggregate/value types', () => {
  const project = createExampleProject(); const exported = parse(exportDomainYaml(project));
  for (const context of Object.values(exported.contexts) as Record<string, unknown>[]) {
    assert.equal(context.aggregates, undefined); assert.equal(context.value_objects, undefined); assert.ok(context.questions);
  }
  assert.ok(importProject(exportDomainYaml(project)).concepts.every(c => c.kind === 'unclassified'));
});

test('invalid references, duplicate IDs, coordinates, and executable colors are rejected', () => {
  const original = createExampleProject();
  const duplicate = cloneProject(original); duplicate.concepts[1].id = duplicate.concepts[0].id;
  assert.throws(() => assertProject(duplicate), /Duplicate ID/);
  const missing = cloneProject(original); missing.concepts[0].contextId = 'missing';
  assert.throws(() => assertProject(missing), /missing context/);
  const dangling = cloneProject(original); dangling.relationships[0].target = 'missing';
  assert.throws(() => assertProject(dangling), /missing endpoint/);
  const coordinates = cloneProject(original); coordinates.concepts[0].position[0] = Infinity;
  assert.throws(() => assertProject(coordinates), /finite coordinates/);
  const color = cloneProject(original); color.contexts[0].color = 'url(javascript:evil)';
  assert.throws(() => assertProject(color), /hex color/);
});

test('malformed YAML is rejected rather than becoming an empty workspace', () => {
  for (const bad of ['null', 'hello', 'version: 8\nproject: test\ncontexts: {}', 'version: 1\nproject: test\ncontexts: []', 'version: 1\nproject: x\ncontexts:\n  bad:\n    definition: 42', 'version: 1\nversion: 2']) assert.throws(() => importProject(bad));
});

test('context relationships work and dangling canonical relationships are rejected', () => {
  const project = createExampleProject();
  project.relationships.push({ id: 'context-link', source: project.contexts[0].id, target: project.contexts[1].id, label: 'partnership' });
  assertProject(project);
  assert.deepEqual(parse(exportDomainYaml(project)).relations[0], { from: 'content', to: 'distribution', kind: 'partnership' });
  assert.throws(() => importProject('version: 1\nproject: x\ncontexts: {}\nrelations:\n - from: missing\n   to: other\n   kind: conformist'), /missing endpoint/);
});

test('baseline distinguishes movement, meanings, and deleted relationships', () => {
  const original = createExampleProject(); const changed = cloneProject(original);
  changed.concepts[0].position[0] += 5; changed.concepts[1].definition = 'Revised meaning'; changed.relationships.shift();
  const changes = diffProjects(original, changed); assert.equal(changes.length, 3);
  assert.match(changes.find(c => c.id === original.concepts[0].id)!.detail, /meaning unchanged/);
  assert.equal(changes.find(c => c.id === original.relationships[0].id)!.type, 'removed');
  assert.equal(diffProjects(original, cloneProject(original)).length, 0);
});

test('draft checks report undefined meanings and duplicate context-local terms', () => {
  const project = createExampleProject(); project.concepts[1].name = project.concepts[0].name; project.concepts[0].definition = '';
  const findings = validateProject(project);
  assert.ok(findings.some(f => f.title === 'Repeated term' && f.severity === 'error'));
  assert.ok(findings.some(f => f.title === 'Meaning is undefined'));
  assert.ok(findings.some(f => f.title === 'Classification is open'));
  assert.deepEqual(validateProject(createEmptyProject()), []);
});

test('real embedded Pattern exposes qualified IDs and readable constraints', () => {
  const input = readFileSync(new URL('../../../internal/infrastructure/pattern/embedded/vertical/pattern.yaml', import.meta.url), 'utf8');
  const pattern = parsePattern(input); assert.equal(pattern.name, 'arclint/vertical@0.1.0'); assert.ok(pattern.rules.length > 10);
  assert.ok(pattern.rules.some(r => r.id === 'arclint/vertical:domain/stdlib-only' && r.description.includes('external: forbid')));
  assert.throws(() => parsePattern('rules: {}'), /pattern must be a mapping/);
});

test('canonical imports reject unsupported fields and misplaced invariants', () => {
  assert.throws(() => importProject('version: 1\nproject: x\ncontexts: {}\nmade_up: true'), /unsupported field/);
  assert.throws(() => importProject('version: 1\nproject: x\ncontexts:\n  main:\n    definition: Model\n    events:\n      Happened:\n        definition: Past\n        invariants: {}'), /unsupported field invariants/);
});

test('canonical export retains object-prototype-looking term names as data', () => {
  const project = createEmptyProject();
  project.contexts.push({ id: 'a', name: 'main', description: 'The model.', color: '#abcdef', position: [0, 0, 0] });
  project.concepts.push({ id: 'v', name: '__proto__', kind: 'value_object', contextId: 'a', definition: 'An intentionally unusual value name.', invariants: [], position: [1, 1, 1] });
  const exported = parse(exportDomainYaml(project));
  assert.equal(Object.hasOwn(exported.contexts.main.value_objects, '__proto__'), true);
  assert.equal(importProject(exportDomainYaml(project)).concepts[0].name, '__proto__');
});

test('new models export aggregates, entities, repositories and factories with explicit identity and ownership', () => {
  const project = createEmptyProject();
  project.name = 'Shop';
  project.contexts.push({ id: 'orders', name: 'orders', description: 'The ordering model.', color: '#abcdef', position: [0, 0, 0] });
  const base = { contextId: 'orders', position: [0, 1, 0] as [number, number, number], invariants: [] };
  project.concepts.push(
    { ...base, id: 'order', name: 'Order', kind: 'aggregate', definition: 'An order with its lines.', identity: 'OrderID', aliases: ['Purchase'], invariants: ['Order has at least one line.'] },
    { ...base, id: 'line', name: 'OrderLine', kind: 'entity', definition: 'One line within an order.', ownerId: 'order', aliases: ['Line'] },
    { ...base, id: 'id', name: 'OrderID', kind: 'value_object', definition: 'The identity value of an order.' },
    { ...base, id: 'repo', name: 'Orders', kind: 'repository', definition: '', ownerId: 'order' },
    { ...base, id: 'factory', name: 'OrderFactory', kind: 'factory', definition: '', ownerId: 'order' },
  );
  const exported = exportDomainYaml(project);
  const order = parse(exported).contexts.orders.aggregates.Order;
  assert.equal(order.identity, 'OrderID');
  assert.deepEqual(order.aliases, ['Purchase']);
  assert.equal(order.entities.OrderLine.definition, 'One line within an order.');
  assert.equal(order.entities.OrderLine.identity, undefined);
  assert.deepEqual(order.entities.OrderLine.aliases, ['Line']);
  assert.equal(order.repository, 'Orders'); assert.equal(order.factory, 'OrderFactory');
  const loaded = importProject(exported);
  const loadedOrder = loaded.concepts.find(c => c.name === 'Order')!;
  assert.equal(loadedOrder.identity, 'OrderID');
  assert.equal(loaded.concepts.find(c => c.name === 'OrderLine')!.ownerId, loadedOrder.id);
  assert.equal(loaded.concepts.find(c => c.name === 'Orders')!.ownerId, loadedOrder.id);
  assert.deepEqual(parse(exportDomainYaml(loaded)), parse(exported));
});

test('aggregate and owner omissions stay editable drafts but canonical export explains what is missing', () => {
  const project = createExampleProject();
  project.concepts[0].kind = 'aggregate';
  assertProject(project);
  assert.ok(validateProject(project).some(f => f.subjectId === project.concepts[0].id && f.title === 'Aggregate identity is open'));
  assert.throws(() => exportDomainYaml(project), /aggregate identity/);
  project.concepts[0].identity = 'SkillID';
  project.concepts[1].kind = 'entity';
  assert.ok(validateProject(project).some(f => f.subjectId === project.concepts[1].id && f.title === 'Aggregate owner is open'));
  assert.throws(() => exportDomainYaml(project), /aggregate owner/);
  project.concepts[1].ownerId = project.concepts[0].id;
  assert.doesNotThrow(() => exportDomainYaml(project));
  project.concepts[1].ownerId = 'missing';
  assert.throws(() => assertProject(project), /incompatible aggregate owner/);
});

test('source-backed identity and alias edits preserve unrelated metadata', () => {
  const project = importProject(source);
  const root = project.concepts.find(c => c.kind === 'aggregate')!;
  root.identity = 'UpdatedRuleID'; root.aliases = ['Policy'];
  const exported = parse(exportDomainYaml(project));
  assert.equal(exported.contexts.rule.aggregates.Rule.identity, 'UpdatedRuleID');
  assert.deepEqual(exported.contexts.rule.aggregates.Rule.aliases, ['Policy']);
  assert.deepEqual(exported.contexts.rule.aggregates.Rule.entities, parse(source).contexts.rule.aggregates.Rule.entities);
  assert.deepEqual(exported.relations, parse(source).relations);
});

test('editing invariant lines retains surviving keys and cannot overwrite a generated-looking key', () => {
  const project = importProject('version: 1\nproject: shop\ncontexts:\n  orders:\n    definition: Orders\n    value_objects:\n      Price:\n        definition: A monetary amount.\n        invariants:\n          amount-positive: Price is positive.\n          recorded-rule-3: Price has a currency.\n');
  project.concepts[0].invariants = ['Price has a currency.', 'Price is positive.', 'Price has finite precision.'];
  const result = parse(exportDomainYaml(project));
  assert.deepEqual(result.contexts.orders.value_objects.Price.invariants, { 'recorded-rule-3': 'Price has a currency.', 'amount-positive': 'Price is positive.', 'recorded-rule-1': 'Price has finite precision.' });
  project.concepts[0].invariants = ['Price has a currency.'];
  assert.deepEqual(parse(exportDomainYaml(project)).contexts.orders.value_objects.Price.invariants, { 'recorded-rule-3': 'Price has a currency.' });
});
