'use strict';
/* Tests of the map's model. The documents here have the shape
   `arclint domain overview --format json` prints; the Go test of the
   server runs this file against a real recording too, through
   ARCLINT_OVERVIEW_JSON. */
const test = require('node:test');
const assert = require('node:assert/strict');
const fs = require('node:fs');
const S = require('./studio.js');

function identityAccess() {
  return {
    source: 'domain.arclint.yaml', found: true, project: 'identityaccess', description: 'Identity and access.',
    counts: { contexts: 1, aggregates: 1, entities: 1, valueObjects: 1, invariants: 2, assertions: 2, specifications: 1, events: 1, services: 1, questions: 1, relations: 0 },
    contexts: [{
      name: 'identityaccess', definition: 'Identity and access for the tenants.',
      aggregates: [{
        name: 'Tenant', definition: 'A subscribing organisation.', identity: 'TenantID', aliases: ['tenancy'],
        entities: [{ name: 'RegistrationInvitation', definition: 'An invitation within its tenant.' }],
        invariants: [{ key: 'one-open-invitation-per-description', statement: 'No two open invitations share a description.', verifiedBy: ['tenant_test.go:TestOneOpenInvitation'] }],
        assertions: [{ key: 'invites-only-while-active', on: 'OfferRegistrationInvitation', statement: 'Only an active tenant offers an invitation.' }],
        repository: 'TenantRepository'
      }],
      valueObjects: [{ name: 'EmailAddress', definition: 'A well-formed email address.', invariants: [{ key: 'well-formed', statement: 'An email address parses as one mailbox address.' }] }],
      events: [{ name: 'TenantProvisioned', definition: 'A tenant came into existence.', raisedBy: 'Tenant' }],
      services: [{ name: 'TenantProvisioningService', definition: 'Brings a tenant into existence.', assertions: [{ key: 'tenant-active-on-completion', on: 'ProvisionTenant', statement: 'A provisioned tenant is active.' }] }],
      specifications: [{ name: 'ActiveTenant', definition: 'Whether a tenant is active.' }],
      questions: [{ key: 'who-deactivates', text: 'Who may deactivate a tenant?' }]
    }],
    relations: []
  };
}

test('conceptsOf flattens one context as recorded, each entry under its owner', () => {
  const ctx = S.context(identityAccess(), 'identityaccess');
  const cs = S.conceptsOf(ctx);
  assert.deepEqual(cs.map((c) => c.id), [
    'aggregate:Tenant',
    'entity:Tenant/RegistrationInvitation',
    'invariant:Tenant/one-open-invitation-per-description',
    'assertion:Tenant/invites-only-while-active',
    'value_object:EmailAddress',
    'invariant:EmailAddress/well-formed',
    'domain_event:TenantProvisioned',
    'domain_service:TenantProvisioningService',
    'assertion:TenantProvisioningService/tenant-active-on-completion',
    'specification:ActiveTenant',
    'question:who-deactivates'
  ]);
  assert.equal(S.byId(cs, 'entity:Tenant/RegistrationInvitation').owner, 'aggregate:Tenant');
  assert.equal(S.byId(cs, 'invariant:EmailAddress/well-formed').owner, 'value_object:EmailAddress');
  assert.equal(S.byId(cs, 'assertion:TenantProvisioningService/tenant-active-on-completion').owner, 'domain_service:TenantProvisioningService');
  const event = S.byId(cs, 'domain_event:TenantProvisioned');
  assert.equal(event.owner, null, 'an event is recorded under the context, not under the aggregate');
  assert.equal(event.raisedById, 'aggregate:Tenant');
  assert.deepEqual(S.raisedBy(cs, 'aggregate:Tenant').map((c) => c.name), ['TenantProvisioned']);
});

test('conceptsOf keeps an invariant key recorded under two owners apart', () => {
  const ctx = { name: 'c', aggregates: [{ name: 'A', invariants: [{ key: 'not-empty', statement: 'A is not empty.' }] }],
    valueObjects: [{ name: 'V', invariants: [{ key: 'not-empty', statement: 'V is not empty.' }] }] };
  const ids = S.conceptsOf(ctx).map((c) => c.id);
  assert.deepEqual(ids, ['aggregate:A', 'invariant:A/not-empty', 'value_object:V', 'invariant:V/not-empty']);
  assert.equal(new Set(ids).size, ids.length);
});

test('navigator lists the recording in its order, members under their owner', () => {
  const cs = S.conceptsOf(S.context(identityAccess(), 'identityaccess'));
  assert.deepEqual(S.navigator(cs).map((r) => r.depth + ' ' + r.c.name), [
    '1 Tenant', '2 RegistrationInvitation', '2 one-open-invitation-per-description', '2 invites-only-while-active',
    '1 EmailAddress', '2 well-formed',
    '1 TenantProvisioned',
    '1 TenantProvisioningService', '2 tenant-active-on-completion',
    '1 ActiveTenant',
    '1 who-deactivates'
  ]);
});

test('label speaks the recorded statement for a rule and the name otherwise', () => {
  const cs = S.conceptsOf(S.context(identityAccess(), 'identityaccess'));
  assert.equal(S.label(S.byId(cs, 'aggregate:Tenant')), 'Tenant');
  assert.equal(S.label(S.byId(cs, 'invariant:Tenant/one-open-invitation-per-description')), 'No two open invitations share a description.');
  assert.equal(S.label(S.byId(cs, 'question:who-deactivates')), 'Who may deactivate a tenant?');
});

test('matches searches the recorded words and narrows by concept', () => {
  const cs = S.conceptsOf(S.context(identityAccess(), 'identityaccess'));
  const tenant = S.byId(cs, 'aggregate:Tenant');
  assert.equal(S.matches(tenant, 'tenancy', ''), true, 'an alias is searched');
  assert.equal(S.matches(tenant, 'organisation', ''), true, 'the definition is searched');
  assert.equal(S.matches(tenant, 'TENANTID', ''), true, 'the identity is searched, case aside');
  assert.equal(S.matches(tenant, '', 'entity'), false);
  assert.equal(S.matches(tenant, 'zzz', ''), false);
  const rule = S.byId(cs, 'assertion:Tenant/invites-only-while-active');
  assert.equal(S.matches(rule, 'OfferRegistrationInvitation', 'assertion'), true, 'the operation an assertion is on is searched');
});

test('layout places every concept inside the enclosure and draws aggregate to entity lines', () => {
  const cs = S.conceptsOf(S.context(identityAccess(), 'identityaccess'));
  const L = S.layout(cs);
  cs.forEach((c) => {
    const p = L.positions[c.id];
    assert.ok(p, c.id + ' has a position');
    assert.ok(p.x >= 0 && p.x <= L.width, c.id + ' x within width');
    assert.ok(p.y >= 0 && p.y <= L.height, c.id + ' y within height');
  });
  assert.equal(L.lines.length, 1, 'one line from Tenant to its entity');
  assert.equal(L.positions['invariant:Tenant/one-open-invitation-per-description'].align, 'left');
  assert.equal(L.positions['domain_event:TenantProvisioned'].align, 'right', 'an event sits beside the aggregate that raises it');
  assert.ok(L.positions['value_object:EmailAddress'].x > L.positions['aggregate:Tenant'].x, 'value objects follow the aggregates');
});

function manyValueObjects(n) {
  const ctx = { name: 'c', aggregates: [{ name: 'A', identity: 'AId' }], valueObjects: [] };
  for (let i = 0; i < n; i++) ctx.valueObjects.push({ name: 'V' + i, definition: 'v', invariants: [{ key: 'k' + i, statement: 's' }] });
  return S.conceptsOf(ctx);
}

test('layout without a viewport keeps every column whole', () => {
  const cs = manyValueObjects(13);
  const L = S.layout(cs);
  const xs = new Set(S.ofConcept(cs, 'value_object').map((v) => L.positions[v.id].x));
  assert.equal(xs.size, 1, 'one column of value objects');
});

test('layout wraps a long column to the height that fits the viewport best', () => {
  const cs = manyValueObjects(30);
  const viewport = { width: 740, height: 730 };
  const tall = S.layout(cs);
  const L = S.layout(cs, viewport);
  const xs = new Set(S.ofConcept(cs, 'value_object').map((v) => L.positions[v.id].x));
  assert.ok(xs.size >= 2, 'value objects spread over several columns: ' + xs.size);
  assert.ok(S.fit(L, viewport) > S.fit(tall, viewport), 'wrapping fits the viewport larger than one tall column');
  cs.forEach((c) => { const p = L.positions[c.id]; assert.ok(p.x <= L.width && p.y <= L.height, c.id + ' inside'); });
});

test('layout of a short context is the same with or without a viewport', () => {
  const cs = S.conceptsOf(S.context(identityAccess(), 'identityaccess'));
  assert.deepEqual(S.layout(cs, { width: 740, height: 730 }), S.layout(cs));
});

test('fit never enlarges past full size', () => {
  assert.equal(S.fit({ width: 100, height: 100 }, { width: 2000, height: 2000 }), 1);
  assert.equal(S.fit({ width: 1388, height: 250 }, { width: 740, height: 730 }), (740 - 48) / 1388);
});

test('layout of an empty context still has an enclosure', () => {
  const L = S.layout([]);
  assert.deepEqual([L.width, L.height, L.lines], [440, 250, []]);
});

test('inspect names the recorded fields and marks a required field that is missing', () => {
  const doc = identityAccess();
  const ctx = S.context(doc, 'identityaccess');
  const cs = S.conceptsOf(ctx);
  const agg = S.inspect(doc, ctx, cs, S.byId(cs, 'aggregate:Tenant'));
  assert.equal(agg[0].kind, 'Aggregate');
  assert.deepEqual(agg[0].rows.map((r) => r.field), ['Definition', 'Identity', 'Aliases', 'Entities', 'Domain events', 'Repository', 'Factory']);
  assert.deepEqual(agg[0].rows[4].values, ['TenantProvisioned']);
  assert.deepEqual(agg[0].rows[1].values, ['TenantID']);
  assert.deepEqual(agg[1].rows[0], { field: 'one-open-invitation-per-description', values: ['No two open invitations share a description.'], missing: false });
  assert.deepEqual(agg[2].rows[0].field, 'OfferRegistrationInvitation');
  const inv = S.inspect(doc, ctx, cs, S.byId(cs, 'invariant:Tenant/one-open-invitation-per-description'))[0];
  assert.deepEqual(inv.rows.map((r) => r.field), ['Statement', 'Key', 'Owner', 'Verified by']);
  assert.deepEqual(inv.rows[2].values, ['Tenant']);
  assert.deepEqual(inv.rows[3].values, ['tenant_test.go:TestOneOpenInvitation']);
  const asrt = S.inspect(doc, ctx, cs, S.byId(cs, 'assertion:TenantProvisioningService/tenant-active-on-completion'))[0];
  assert.deepEqual(asrt.rows.map((r) => r.field + '=' + r.values.join(',')), [
    'Statement=A provisioned tenant is active.', 'Key=tenant-active-on-completion', 'On=ProvisionTenant', 'Owner=TenantProvisioningService', 'Verified by='
  ]);
  const event = S.inspect(doc, ctx, cs, S.byId(cs, 'domain_event:TenantProvisioned'))[0];
  assert.deepEqual(event.rows[1], { field: 'Raised by', values: ['Tenant'], missing: false });
  const bare = S.inspect(doc, ctx, cs, S.byId(S.conceptsOf({ name: 'c', aggregates: [{ name: 'Ledger' }] }), 'aggregate:Ledger'))[0];
  assert.equal(bare.rows[1].field, 'Identity');
  assert.equal(bare.rows[1].missing, true, 'an aggregate without an identity is marked missing');
});

test('inspect of the context lists what it records and its context map', () => {
  const doc = identityAccess();
  doc.contexts.push({ name: 'collaboration', definition: 'Forums and calendars.' });
  doc.relations = [{ from: 'identityaccess', to: 'collaboration', kind: 'customer_supplier', description: 'Collaboration conforms to identities.' }];
  const ctx = S.context(doc, 'identityaccess');
  const sections = S.inspect(doc, ctx, S.conceptsOf(ctx), null);
  assert.equal(sections[0].kind, 'Bounded context');
  assert.deepEqual(sections[0].rows.map((r) => r.field), ['Definition', 'Aggregates', 'Value objects', 'Domain events', 'Domain services', 'Specifications', 'Questions']);
  assert.deepEqual(sections[0].rows[1].values, ['Tenant']);
  assert.equal(sections[1].heading, 'Context map');
  assert.deepEqual(sections[1].rows[0], { field: 'customer_supplier', values: ['to collaboration', 'Collaboration conforms to identities.'], missing: false }, 'the relation kind is spelled as recorded');
  assert.deepEqual(S.relationsOf(doc, 'collaboration').length, 1);
});

test('esc keeps recorded text out of the markup', () => {
  assert.equal(S.esc('<b onmouseover="x">&</b>'), '&lt;b onmouseover=&quot;x&quot;&gt;&amp;&lt;/b&gt;');
  assert.equal(S.esc(null), '');
  assert.equal(S.esc(42), '42');
});

test('an event raised_by a name that is not a recorded aggregate is loose, even a prototype name', () => {
  const ctx = { name: 'c', aggregates: [{ name: 'A' }], events: [{ name: 'E1', raisedBy: 'constructor' }, { name: 'E2', raisedBy: 'Missing' }, { name: 'E3', raisedBy: 'A' }] };
  const cs = S.conceptsOf(ctx);
  assert.equal(S.byId(cs, 'domain_event:E1').raisedById, null);
  assert.equal(S.byId(cs, 'domain_event:E2').raisedById, null);
  assert.equal(S.byId(cs, 'domain_event:E3').raisedById, 'aggregate:A');
  const L = S.layout(cs);
  ['E1', 'E2', 'E3'].forEach((n) => assert.ok(L.positions['domain_event:' + n], n + ' is drawn'));
  assert.equal(L.positions['domain_event:E3'].align, 'right');
  assert.equal(L.positions['domain_event:E1'].align, 'centre');
});

test('countsLine spells the counts as the overview does', () => {
  assert.equal(S.countsLine(identityAccess().counts), '1 context · 1 aggregate · 1 entity · 1 value object · 2 invariants · 2 assertions · 1 specification · 1 event · 1 service · 1 question');
  assert.equal(S.countsLine({ contexts: 2, relations: 1 }), '2 contexts · 1 relation');
  assert.equal(S.countsLine(undefined), '0 contexts');
});

test('legend and path follow the active context and selection', () => {
  const doc = identityAccess();
  const ctx = S.context(doc, 'identityaccess');
  const cs = S.conceptsOf(ctx);
  assert.deepEqual(S.legend(cs), ['aggregate', 'entity', 'value_object', 'domain_event', 'domain_service', 'invariant', 'assertion', 'specification', 'question']);
  assert.deepEqual(S.path(ctx, cs, null), ['identityaccess']);
  assert.deepEqual(S.path(ctx, cs, S.byId(cs, 'entity:Tenant/RegistrationInvitation')), ['identityaccess', 'Tenant', 'RegistrationInvitation']);
  assert.equal(S.context(doc, 'missing').name, 'identityaccess', 'an unknown name falls back to the first context');
  assert.equal(S.context({ contexts: [] }, 'x'), null);
});

test('a real recording, when the Go test lends one', { skip: !process.env.ARCLINT_OVERVIEW_JSON && 'ARCLINT_OVERVIEW_JSON is not set' }, () => {
  const doc = JSON.parse(fs.readFileSync(process.env.ARCLINT_OVERVIEW_JSON, 'utf8'));
  assert.equal(doc.found, true);
  assert.ok(doc.contexts.length >= 1);
  let total = 0;
  doc.contexts.forEach((ctx) => {
    const cs = S.conceptsOf(ctx);
    const ids = cs.map((c) => c.id);
    assert.equal(new Set(ids).size, ids.length, ctx.name + ': ids are unique');
    const rows = S.navigator(cs);
    assert.equal(rows.length, cs.length, ctx.name + ': the navigator lists every concept once');
    const L = S.layout(cs);
    cs.forEach((c) => {
      assert.ok(L.positions[c.id], ctx.name + ': ' + c.id + ' is drawn');
      assert.ok(S.inspect(doc, ctx, cs, c).length >= 1, ctx.name + ': ' + c.id + ' is inspectable');
      if (c.owner) assert.ok(S.byId(cs, c.owner), ctx.name + ': ' + c.id + ' owner is recorded');
    });
    total += cs.length;
  });
  const counted = doc.counts.aggregates + doc.counts.entities + doc.counts.valueObjects + doc.counts.invariants + doc.counts.assertions +
    doc.counts.specifications + doc.counts.events + doc.counts.services + doc.counts.questions;
  assert.equal(total, counted, 'the map shows exactly what the overview counts');
});
