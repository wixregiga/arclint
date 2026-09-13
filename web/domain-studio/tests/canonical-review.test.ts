import { test } from 'node:test';
import assert from 'node:assert/strict';
import { parse } from 'yaml';
import { importProject, exportDomainYaml } from '../src/serialization';
import { diffProjects } from '../src/domain';

const order = `version: 1
project: shop
contexts:
  orders:
    definition: Ordering language.
    aggregates:
      Order:
        definition: An accepted purchase.
        identity: OrderID
        aliases: [Purchase]
        assertions:
          confirmation-ready:
            on: Confirm
            statement: Every line has an accepted price.
    events:
      OrderConfirmed:
        definition: An order was confirmed.
        raised_by: Order.Confirm
`;

test('model snapshot comparison reports removal of optional semantic fields', () => {
  const before = importProject(order), current = structuredClone(before);
  const root = current.concepts.find(concept => concept.kind === 'aggregate')!;
  delete root.aliases;
  const change = diffProjects(before, current).find(change => change.id === root.id);
  assert.ok(change, 'Removing an authored alias must change the model snapshot.');
  assert.match(change.detail, /aliases/);
});

test('reclassifying an event as unresolved cannot silently discard its recorded source', () => {
  const project = importProject(order);
  const event = project.concepts.find(concept => concept.kind === 'domain_event')!;
  event.kind = 'unclassified';
  let exported: string;
  try { exported = exportDomainYaml(project); }
  catch (error) { assert.match(String(error), /raised.by|event|metadata/i); return; }
  assert.match(JSON.stringify(parse(exported)), /Order\.Confirm/, 'Preserve the proposed event source, or explicitly reject this conversion.');
});

test('reclassifying a legacy aggregate retains source-only operation assertions or fails explicitly', () => {
  const project = importProject(order);
  const aggregate = project.concepts.find(concept => concept.kind === 'aggregate')!;
  // Older saved workspaces retained assertions only in sourceDocument.
  delete aggregate.assertions;
  aggregate.kind = 'unclassified';
  let exported: string;
  try { exported = exportDomainYaml(project); }
  catch (error) { assert.match(String(error), /assertion|contract|metadata/i); return; }
  assert.match(JSON.stringify(parse(exported)), /Every line has an accepted price\./, 'A soft conversion to an open question must not erase the old operation contract.');
});

test('reclassifying an event as a repository cannot bypass its recorded source contract', () => {
  const project = importProject(order);
  const event = project.concepts.find(concept => concept.kind === 'domain_event')!;
  event.kind = 'repository';
  event.ownerId = project.concepts.find(concept => concept.kind === 'aggregate')!.id;
  let exported: string;
  try { exported = exportDomainYaml(project); }
  catch (error) { assert.match(String(error), /raised.by|event|metadata|contract/i); return; }
  assert.match(JSON.stringify(parse(exported)), /Order\.Confirm/, 'Repository serialization must not discard metadata that another term kind cannot represent.');
});

test('reclassifying a service as a repository retains its authored definition as an explicit question', () => {
  const source = order.replace('    events:', '    services:\n      OrderPricing:\n        definition: Determines accepted prices for the ordering model.\n    events:');
  const project = importProject(source);
  const service = project.concepts.find(concept => concept.kind === 'domain_service')!;
  service.kind = 'repository';
  service.ownerId = project.concepts.find(concept => concept.kind === 'aggregate')!.id;
  const exported = exportDomainYaml(project);
  assert.match(JSON.stringify(parse(exported)), /Determines accepted prices for the ordering model\./, 'Changing classification must not erase the unchanged authored definition.');
});
