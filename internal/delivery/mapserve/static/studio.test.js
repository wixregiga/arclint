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
  const entityLines = L.lines.filter((l) => l[3] === 170);
  assert.equal(entityLines.length, 1, 'one line from Tenant to its entity');
  [['aggregate:Tenant', 'invariant:Tenant/one-open-invitation-per-description'], ['aggregate:Tenant', 'assertion:Tenant/invites-only-while-active'],
    ['value_object:EmailAddress', 'invariant:EmailAddress/well-formed'], ['domain_service:TenantProvisioningService', 'assertion:TenantProvisioningService/tenant-active-on-completion']].forEach(([owner, member]) => {
    const o = L.positions[owner], m = L.positions[member];
    assert.ok(m, member + ' has a position');
    assert.equal(m.member, true, member + ' is drawn as a member row');
    assert.ok(L.lines.some((l) => l[0] === o.x - 6 && l[1] === o.y + 6), owner + ' has a branch from its dot');
    assert.ok(L.lines.some((l) => l[2] === m.x && l[3] === m.y + 6 && l[1] === l[3]), member + ' is connected at its dot');
  });
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

/* --- Editing: the transformation between recordings, drafts, and the form. --- */

test('transformation names what one recording changes against another, property by property', () => {
  const accepted = identityAccess();
  const doc = identityAccess();
  doc.contexts[0].valueObjects[0].definition = 'A mailbox address.';
  doc.contexts[0].aggregates[0].invariants.push({ key: 'named', statement: 'A tenant has a name.' });
  doc.contexts[0].specifications = [];
  doc.contexts[0].definition = 'Identity and access.';
  const t = S.transformation(accepted, doc);
  assert.deepEqual(t.map((x) => [x.kind, x.concept, x.owner, x.name]), [
    ['changed', 'bounded_context', '', 'identityaccess'],
    ['added', 'invariant', 'Tenant', 'named'],
    ['changed', 'value_object', '', 'EmailAddress'],
    ['removed', 'specification', '', 'ActiveTenant']
  ]);
  assert.deepEqual(t[2].fields, [{ field: 'definition', before: 'A well-formed email address.', after: 'A mailbox address.' }]);
  assert.deepEqual(t[1].fields, [{ field: 'statement', before: null, after: 'A tenant has a name.' }]);
  assert.deepEqual(t[3].fields, [{ field: 'definition', before: 'Whether a tenant is active.', after: null }]);
  assert.deepEqual(S.transformation(accepted, identityAccess()), [], 'the same recording transforms nothing');
  const standing = S.standing(accepted, doc);
  assert.equal(standing[S.key('identityaccess', 'value_object', '', 'EmailAddress')], 'Proposed');
  assert.equal(standing[S.key('identityaccess', 'aggregate', '', 'Tenant')], undefined, 'an untouched entry is accepted');
  assert.deepEqual(S.standing(null, doc), {}, 'without an accepted recording nothing stands out');
});

test('drafts merge by entry, a removal of a drafted creation drops it, and each posts as define or remove', () => {
  const k = S.key('identityaccess', 'value_object', '', 'EmailAddress');
  let drafts = S.drafted([], { key: k, action: 'define', context: 'identityaccess', concept: 'value_object', owner: '', name: 'EmailAddress', change: { definition: 'A mailbox address.' } });
  drafts = S.drafted(drafts, { key: k, action: 'define', context: 'identityaccess', concept: 'value_object', owner: '', name: 'EmailAddress', change: { aliases: ['email'] } });
  assert.equal(drafts.length, 1);
  assert.deepEqual(drafts[0].change, { definition: 'A mailbox address.', aliases: ['email'] });
  assert.deepEqual(S.requestOf(drafts[0]), { concept: 'value_object', context: 'identityaccess', owner: '', name: 'EmailAddress', change: { definition: 'A mailbox address.', aliases: ['email'] } });
  const n = S.key('identityaccess', 'invariant', 'Tenant', 'named');
  drafts = S.drafted(drafts, { key: n, action: 'define', context: 'identityaccess', concept: 'invariant', owner: 'Tenant', name: 'named', change: { statement: 'A tenant has a name.' }, created: true });
  assert.equal(drafts.length, 2);
  drafts = S.drafted(drafts, { key: n, action: 'remove', context: 'identityaccess', concept: 'invariant', owner: 'Tenant', name: 'named' });
  assert.equal(drafts.length, 1, 'removing a drafted creation leaves nothing to publish');
  drafts = S.drafted(drafts, { key: S.key('identityaccess', 'question', '', 'who-deactivates'), action: 'remove', context: 'identityaccess', concept: 'question', owner: '', name: 'who-deactivates' });
  assert.deepEqual(S.requestOf(drafts[1]), { concept: 'question', context: 'identityaccess', owner: '', name: 'who-deactivates' });
  const c = S.key('billing', 'bounded_context', '', 'billing');
  drafts = S.drafted(drafts, { key: c, action: 'define', context: 'billing', concept: 'bounded_context', owner: '', name: 'billing', change: { definition: 'Invoices.' }, created: true });
  assert.equal(S.requestOf(S.draftOf(drafts, c)).context, '', 'a bounded context is located by its name alone');
  assert.equal(S.withoutDraft(drafts, c).length, 2);
});

test('applyDrafts shows the recording as the drafts would leave it', () => {
  const doc = identityAccess();
  const drafts = [
    { key: S.key('identityaccess', 'value_object', '', 'EmailAddress'), action: 'define', context: 'identityaccess', concept: 'value_object', owner: '', name: 'EmailAddress', change: { definition: 'A mailbox address.', aliases: [] } },
    { key: S.key('identityaccess', 'invariant', 'Tenant', 'named'), action: 'define', context: 'identityaccess', concept: 'invariant', owner: 'Tenant', name: 'named', change: { statement: 'A tenant has a name.' } },
    { key: S.key('identityaccess', 'assertion', 'TenantProvisioningService', 'idempotent'), action: 'define', context: 'identityaccess', concept: 'assertion', owner: 'TenantProvisioningService', name: 'idempotent', change: { on: 'ProvisionTenant', statement: 'Provisioning twice provisions once.' } },
    { key: S.key('identityaccess', 'domain_event', '', 'TenantProvisioned'), action: 'define', context: 'identityaccess', concept: 'domain_event', owner: '', name: 'TenantProvisioned', change: { raisedBy: '' } },
    { key: S.key('identityaccess', 'specification', '', 'ActiveTenant'), action: 'remove', context: 'identityaccess', concept: 'specification', owner: '', name: 'ActiveTenant' },
    { key: S.key('billing', 'bounded_context', '', 'billing'), action: 'define', context: 'billing', concept: 'bounded_context', owner: '', name: 'billing', change: { definition: 'Invoices and payments.' } },
    { key: S.key('billing', 'aggregate', '', 'Invoice'), action: 'define', context: 'billing', concept: 'aggregate', owner: '', name: 'Invoice', change: { definition: 'A bill.', identity: 'InvoiceID' } }
  ];
  const after = S.applyDrafts(doc, drafts);
  assert.equal(doc.contexts[0].valueObjects[0].definition, 'A well-formed email address.', 'the recording itself is untouched');
  const ctx = S.context(after, 'identityaccess');
  assert.equal(ctx.valueObjects[0].definition, 'A mailbox address.');
  assert.deepEqual(ctx.aggregates[0].invariants.map((i) => i.key), ['one-open-invitation-per-description', 'named']);
  assert.deepEqual(ctx.services[0].assertions.map((a) => a.on), ['ProvisionTenant', 'ProvisionTenant']);
  assert.equal(ctx.events[0].raisedBy, undefined, 'an empty raised_by records none');
  assert.deepEqual(ctx.specifications, []);
  const billing = S.context(after, 'billing');
  assert.equal(billing.definition, 'Invoices and payments.');
  assert.deepEqual(billing.aggregates, [{ name: 'Invoice', definition: 'A bill.', identity: 'InvoiceID' }]);
  const t = S.transformation(doc, after);
  assert.deepEqual(t.map((x) => x.kind + ' ' + x.concept + ' ' + x.name), [
    'added invariant named', 'changed value_object EmailAddress', 'changed domain_event TenantProvisioned', 'added assertion idempotent',
    'added bounded_context billing', 'added aggregate Invoice', 'removed specification ActiveTenant'
  ]);
  const gone = S.applyDrafts(doc, [{ key: S.key('identityaccess', 'aggregate', '', 'Tenant'), action: 'remove', context: 'identityaccess', concept: 'aggregate', owner: '', name: 'Tenant' }]);
  assert.deepEqual(gone.contexts[0].aggregates, []);
  assert.equal(gone.contexts[0].events[0].raisedBy, undefined, 'an event the removed aggregate raised no longer names it');
  const noContext = S.applyDrafts(doc, [{ key: S.key('identityaccess', 'bounded_context', '', 'identityaccess'), action: 'remove', context: 'identityaccess', concept: 'bounded_context', owner: '', name: 'identityaccess' }]);
  assert.deepEqual(noContext.contexts, []);
});

test('the form edits the properties the concept records, and a filled form changes only what differs', () => {
  const cs = S.conceptsOf(S.context(identityAccess(), 'identityaccess'));
  const tenant = S.byId(cs, 'aggregate:Tenant');
  const props = S.propertiesOf(tenant);
  assert.deepEqual(props, { definition: 'A subscribing organisation.', identity: 'TenantID', aliases: ['tenancy'], repository: 'TenantRepository', factory: '' });
  assert.deepEqual(S.form('aggregate', props).map((f) => [f.name, f.value, f.required, f.list]), [
    ['definition', 'A subscribing organisation.', true, false], ['identity', 'TenantID', true, false], ['aliases', 'tenancy', false, true],
    ['repository', 'TenantRepository', false, false], ['factory', '', false, false]
  ]);
  assert.deepEqual(S.changeFrom('aggregate', props, { definition: 'A subscribing organisation.', identity: 'TenantID', aliases: 'tenancy\n\n org ', repository: 'TenantRepository', factory: '' }),
    { aliases: ['tenancy', 'org'] });
  assert.deepEqual(S.changeFrom('aggregate', props, { definition: 'A subscribing organisation.', identity: 'TenantID', aliases: '', repository: 'TenantRepository', factory: '' }),
    { aliases: [] }, 'clearing a list records it empty, deliberately');
  assert.deepEqual(S.changeFrom('value_object', null, { definition: 'A thing.', aliases: '' }), { definition: 'A thing.' }, 'a new entry records what is filled');
  assert.deepEqual(S.ownersFor(cs, 'invariant').map((o) => o.name), ['Tenant', 'EmailAddress']);
  assert.deepEqual(S.ownersFor(cs, 'assertion').map((o) => o.name), ['Tenant', 'TenantProvisioningService']);
  assert.deepEqual(S.ownersFor(cs, 'entity').map((o) => o.name), ['Tenant']);
  assert.equal(S.ownerName(cs, S.byId(cs, 'invariant:EmailAddress/well-formed')), 'EmailAddress');
  assert.deepEqual(S.form('assertion', S.propertiesOf(S.byId(cs, 'assertion:Tenant/invites-only-while-active'))).map((f) => f.value), ['OfferRegistrationInvitation', 'Only an active tenant offers an invitation.', '']);
});

test('a real recording round-trips through the catalog and transforms into itself', { skip: !process.env.ARCLINT_OVERVIEW_JSON && 'ARCLINT_OVERVIEW_JSON is not set' }, () => {
  const doc = JSON.parse(fs.readFileSync(process.env.ARCLINT_OVERVIEW_JSON, 'utf8'));
  const cat = S.catalog(doc);
  assert.equal(new Set(cat.order).size, cat.order.length, 'keys are unique');
  assert.deepEqual(S.transformation(doc, JSON.parse(JSON.stringify(doc))), []);
  const ctx = doc.contexts[0];
  const cs = S.conceptsOf(ctx);
  const first = cs[0];
  const k = S.key(ctx.name, first.concept, S.ownerName(cs, first), first.name);
  const field = S.PROPERTIES[first.concept][0].name;
  const change = {}; change[field] = 'Changed in the page.';
  const after = S.applyDrafts(doc, [{ key: k, action: 'define', context: ctx.name, concept: first.concept, owner: S.ownerName(cs, first), name: first.name, change: change }]);
  const t = S.transformation(doc, after);
  assert.equal(t.length, 1);
  assert.deepEqual([t[0].kind, t[0].key, t[0].fields[0].field, t[0].fields[0].after], ['changed', k, field, 'Changed in the page.']);
});

test('a second edit of a drafted owner keeps its place ahead of what is drafted under it', () => {
  const a = S.key('billing', 'aggregate', '', 'Invoice');
  const i = S.key('billing', 'invariant', 'Invoice', 'positive');
  let drafts = S.drafted([], { key: a, action: 'define', context: 'billing', concept: 'aggregate', owner: '', name: 'Invoice', change: { definition: 'A bill.', identity: 'InvoiceID' }, created: true });
  drafts = S.drafted(drafts, { key: i, action: 'define', context: 'billing', concept: 'invariant', owner: 'Invoice', name: 'positive', change: { statement: 'An invoice total is positive.' }, created: true });
  drafts = S.drafted(drafts, { key: a, action: 'define', context: 'billing', concept: 'aggregate', owner: '', name: 'Invoice', change: { definition: 'A bill a customer owes.' } });
  assert.deepEqual(drafts.map((d) => d.key), [a, i]);
  assert.deepEqual(drafts[0].change, { definition: 'A bill a customer owes.', identity: 'InvoiceID' });
  assert.equal(drafts[0].created, true, 'still a creation');
});

test('publishing one draft keeps a newer draft of the same entry saved meanwhile', () => {
  const k = S.key('identityaccess', 'value_object', '', 'EmailAddress');
  const sent = { key: k, action: 'define', context: 'identityaccess', concept: 'value_object', owner: '', name: 'EmailAddress', change: { definition: 'A.' }, created: true };
  const other = { key: S.key('identityaccess', 'question', '', 'q'), action: 'remove', context: 'identityaccess', concept: 'question', owner: '', name: 'q' };
  assert.deepEqual(S.published([sent, other], sent), [other], 'the published draft goes');
  const newer = S.drafted([sent, other], { key: k, action: 'define', context: 'identityaccess', concept: 'value_object', owner: '', name: 'EmailAddress', change: { definition: 'B.' } });
  const left = S.published(newer, sent);
  assert.equal(left.length, 2, 'the newer draft stays');
  assert.equal(left[0].change.definition, 'B.');
  assert.equal(left[0].created, false, 'the entry exists once its define is published');
});

test('transformation shows a relation that changes, and one a removed context takes with it', () => {
  const doc = identityAccess();
  doc.contexts.push({ name: 'billing', definition: 'Invoices.' });
  doc.relations = [{ from: 'identityaccess', to: 'billing', kind: 'conformist', description: 'Billing reads tenants.' }];
  const changed = JSON.parse(JSON.stringify(doc));
  changed.relations[0].kind = 'partnership';
  const t = S.transformation(doc, changed);
  assert.deepEqual(t.map((x) => [x.kind, x.concept, x.name]), [['changed', 'relation', 'identityaccess → billing']]);
  assert.deepEqual(t[0].fields, [{ field: 'kind', before: 'conformist', after: 'partnership' }]);
  const gone = S.applyDrafts(doc, [{ key: S.key('billing', 'bounded_context', '', 'billing'), action: 'remove', context: 'billing', concept: 'bounded_context', owner: '', name: 'billing' }]);
  assert.deepEqual(S.transformation(doc, gone).map((x) => [x.kind, x.concept]), [['removed', 'bounded_context'], ['removed', 'relation']]);
});

test('withdrawing a drafted creation takes what is drafted under it', () => {
  const v = { key: S.key('c', 'value_object', '', 'Money'), action: 'define', context: 'c', concept: 'value_object', owner: '', name: 'Money', change: { definition: 'An amount.' }, created: true };
  const inv = { key: S.key('c', 'invariant', 'Money', 'positive'), action: 'define', context: 'c', concept: 'invariant', owner: 'Money', name: 'positive', change: { statement: 'Money is positive.' }, created: true };
  const other = { key: S.key('c', 'invariant', 'Price', 'positive'), action: 'define', context: 'c', concept: 'invariant', owner: 'Price', name: 'positive', change: { statement: 'A price is positive.' }, created: true };
  const remove = { key: v.key, action: 'remove', context: 'c', concept: 'value_object', owner: '', name: 'Money' };
  assert.deepEqual(S.drafted([v, inv, other], remove).map((d) => d.key), [other.key], 'removing the drafted owner takes its invariant, not another owner\'s');
  assert.deepEqual(S.withoutDraft([v, inv, other], v.key).map((d) => d.key), [other.key], 'discarding it does too');
  const recorded = Object.assign({}, v, { created: false });
  assert.deepEqual(S.withoutDraft([recorded, inv], v.key).map((d) => d.key), [inv.key], 'an owner already recorded keeps what is drafted under it');
  const ctx = { key: S.key('billing', 'bounded_context', '', 'billing'), action: 'define', context: 'billing', concept: 'bounded_context', owner: '', name: 'billing', change: { definition: 'Bills.' }, created: true };
  const agg = { key: S.key('billing', 'aggregate', '', 'Invoice'), action: 'define', context: 'billing', concept: 'aggregate', owner: '', name: 'Invoice', change: { definition: 'A bill.' }, created: true };
  assert.deepEqual(S.withoutDraft([ctx, agg, v], ctx.key).map((d) => d.key), [v.key], 'a new context takes everything drafted in it');
});
