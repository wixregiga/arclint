/* The map's model: pure functions over the domain overview document
   `arclint domain overview --format json` prints, the drafts the page
   holds before it publishes them, and the transformation between two
   recordings. No DOM. Loads as a <script> (window.Studio) and under
   node (require) for its tests. */
(function (root, factory) {
  if (typeof module === 'object' && module.exports) module.exports = factory();
  else root.Studio = factory();
})(typeof self !== 'undefined' ? self : this, function () {
  'use strict';

  /* The concepts the domain file records, spelled as the recording spells them. */
  var CONCEPTS = ['bounded_context', 'aggregate', 'entity', 'value_object', 'domain_event', 'domain_service', 'invariant', 'assertion', 'specification', 'question'];
  var CONCEPT_LABEL = {
    bounded_context: 'Bounded context', aggregate: 'Aggregate', entity: 'Entity', value_object: 'Value object',
    domain_event: 'Domain event', domain_service: 'Domain service', invariant: 'Invariant', assertion: 'Assertion',
    specification: 'Specification', question: 'Question', relation: 'Relation'
  };

  function list(a) { return Array.isArray(a) ? a : []; }
  /* Recorded text on its way into HTML. */
  function esc(s) { return String(s == null ? '' : s).replace(/&/g, '&amp;').replace(/</g, '&lt;').replace(/>/g, '&gt;').replace(/"/g, '&quot;'); }
  function id(concept, owner, name) { return concept + ':' + (owner ? owner + '/' : '') + name; }

  /* Every entry one bounded context records, flat, each knowing the entry it is recorded under. */
  function conceptsOf(ctx) {
    var out = [];
    var aggregateIds = Object.create(null);
    list(ctx.aggregates).forEach(function (a) {
      var aid = id('aggregate', '', a.name);
      aggregateIds[a.name] = aid;
      out.push({ id: aid, concept: 'aggregate', name: a.name, owner: null, definition: a.definition || '', identity: a.identity || '',
        aliases: list(a.aliases), repository: a.repository || '', factory: a.factory || '' });
      list(a.entities).forEach(function (e) {
        out.push({ id: id('entity', a.name, e.name), concept: 'entity', name: e.name, owner: aid, definition: e.definition || '',
          identity: e.identity || '', aliases: list(e.aliases) });
      });
      list(a.invariants).forEach(function (i) { out.push(invariantOf(i, a.name, aid)); });
      list(a.assertions).forEach(function (s) { out.push(assertionOf(s, a.name, aid)); });
    });
    list(ctx.valueObjects).forEach(function (v) {
      var vid = id('value_object', '', v.name);
      out.push({ id: vid, concept: 'value_object', name: v.name, owner: null, definition: v.definition || '', aliases: list(v.aliases) });
      list(v.invariants).forEach(function (i) { out.push(invariantOf(i, v.name, vid)); });
    });
    list(ctx.events).forEach(function (e) {
      out.push({ id: id('domain_event', '', e.name), concept: 'domain_event', name: e.name, owner: null, definition: e.definition || '',
        raisedBy: e.raisedBy || '', raisedById: aggregateIds[e.raisedBy] || null });
    });
    list(ctx.services).forEach(function (s) {
      var sid = id('domain_service', '', s.name);
      out.push({ id: sid, concept: 'domain_service', name: s.name, owner: null, definition: s.definition || '' });
      list(s.assertions).forEach(function (a) { out.push(assertionOf(a, s.name, sid)); });
    });
    list(ctx.specifications).forEach(function (s) {
      out.push({ id: id('specification', '', s.name), concept: 'specification', name: s.name, owner: null, definition: s.definition || '' });
    });
    list(ctx.questions).forEach(function (q) {
      out.push({ id: id('question', '', q.key), concept: 'question', name: q.key, owner: null, text: q.text || '' });
    });
    return out;
  }
  function invariantOf(i, ownerName, ownerId) {
    return { id: id('invariant', ownerName, i.key), concept: 'invariant', name: i.key, owner: ownerId, statement: i.statement || '', verifiedBy: list(i.verifiedBy) };
  }
  function assertionOf(a, ownerName, ownerId) {
    return { id: id('assertion', ownerName, a.key), concept: 'assertion', name: a.key, owner: ownerId, on: a.on || '', statement: a.statement || '', verifiedBy: list(a.verifiedBy) };
  }

  function byId(concepts, cid) { for (var i = 0; i < concepts.length; i++) if (concepts[i].id === cid) return concepts[i]; return null; }
  function ofConcept(concepts, concept) { return concepts.filter(function (c) { return c.concept === concept; }); }
  function membersOf(concepts, ownerId, concept) {
    return concepts.filter(function (c) { return c.owner === ownerId && (!concept || c.concept === concept); });
  }
  function raisedBy(concepts, aggregateId) {
    return concepts.filter(function (c) { return c.concept === 'domain_event' && c.raisedById === aggregateId; });
  }
  /* The full text of an entry, which hovering a node shows: the statement
     a rule records, the text of a question, otherwise the name. */
  function label(c) { return c.concept === 'invariant' || c.concept === 'assertion' ? c.statement || c.name : c.concept === 'question' ? c.text || c.name : c.name; }

  function context(doc, name) {
    var contexts = list(doc && doc.contexts);
    for (var i = 0; i < contexts.length; i++) if (contexts[i].name === name) return contexts[i];
    return contexts[0] || null;
  }
  function relationsOf(doc, name) {
    return list(doc && doc.relations).filter(function (r) { return r.from === name || r.to === name; });
  }

  /* Navigator order: the recording's order, each entry under the one it is recorded under. */
  function navigator(concepts) {
    var rows = [];
    function member(c) { rows.push({ c: c, depth: 2 }); }
    ofConcept(concepts, 'aggregate').forEach(function (a) {
      rows.push({ c: a, depth: 1 });
      membersOf(concepts, a.id, 'entity').forEach(member);
      membersOf(concepts, a.id, 'invariant').forEach(member);
      membersOf(concepts, a.id, 'assertion').forEach(member);
    });
    ofConcept(concepts, 'value_object').forEach(function (v) {
      rows.push({ c: v, depth: 1 });
      membersOf(concepts, v.id, 'invariant').forEach(member);
    });
    ofConcept(concepts, 'domain_event').forEach(function (e) { rows.push({ c: e, depth: 1 }); });
    ofConcept(concepts, 'domain_service').forEach(function (s) {
      rows.push({ c: s, depth: 1 });
      membersOf(concepts, s.id, 'assertion').forEach(member);
    });
    ofConcept(concepts, 'specification').forEach(function (s) { rows.push({ c: s, depth: 1 }); });
    ofConcept(concepts, 'question').forEach(function (q) { rows.push({ c: q, depth: 1 }); });
    return rows;
  }

  function matches(c, query, concept) {
    if (concept && c.concept !== concept) return false;
    var q = String(query || '').trim().toLowerCase();
    if (!q) return true;
    var text = [c.name, c.definition, c.statement, c.text, c.identity, c.on, c.raisedBy].concat(c.aliases || []).join(' ').toLowerCase();
    return text.indexOf(q) >= 0;
  }

  /* Positions on the map, in canvas pixels. One column per aggregate: the
     aggregate at the top, its entities in a row below, its invariants and
     assertions down the left, the events it raises down the right. Then
     a column of value objects with their invariants, one of domain
     services with their assertions, and one of what remains: events no
     aggregate raises, specifications and questions.

     A column wraps into the next one when it reaches a wrap height. With
     a viewport ({width, height} of the visible canvas), the wrap height
     is the one whose map fits that viewport at the largest scale, so a
     long column neither towers nor flattens into a strip.

     An owner's members (an aggregate's invariants and assertions, a
     value object's invariants, a domain service's assertions) sit in
     rows down the left of its column, and a branch connects them to the
     owner: a line from the owner's dot to a spine, and from the spine
     to each member's dot. */
  var MARGIN = { width: 48, height: 96 };
  function fit(L, viewport) {
    return Math.min(1, (viewport.width - MARGIN.width) / L.width, (viewport.height - MARGIN.height) / L.height);
  }
  function layout(concepts, viewport) {
    var tallest = layoutWrapped(concepts, Infinity);
    if (!viewport || !(viewport.width > MARGIN.width) || !(viewport.height > MARGIN.height)) return tallest;
    var best = tallest, bestFit = fit(tallest, viewport);
    for (var wrap = 250; wrap < tallest.height; wrap += 40) {
      var L = layoutWrapped(concepts, wrap);
      var f = fit(L, viewport);
      if (f > bestFit) { best = L; bestFit = f; }
    }
    return best;
  }
  function layoutWrapped(concepts, columnHeight) {
    var positions = {}, lines = [], x = 0, height = 250;
    var aggregates = ofConcept(concepts, 'aggregate');
    var columnWidth = aggregates.length > 1 ? 300 : 440;
    var rowStep = 26;
    /* A member row is a dot beside its name; DOT is the dot's size, so a
       line meets a dot's edge at half of it. */
    var DOT = 12;
    function branch(ownerX, ownerY, left, members) {
      if (!members.length) return;
      var spine = left + 14, last = members[members.length - 1];
      lines.push([ownerX - DOT / 2, ownerY + DOT / 2, spine, ownerY + DOT / 2]);
      lines.push([spine, ownerY + DOT / 2, spine, positions[last.id].y + DOT / 2]);
      members.forEach(function (m) { lines.push([spine, positions[m.id].y + DOT / 2, left + 24, positions[m.id].y + DOT / 2]); });
    }
    aggregates.forEach(function (a) {
      var centre = x + columnWidth / 2;
      positions[a.id] = { x: centre, y: 46, align: 'centre' };
      var entities = membersOf(concepts, a.id, 'entity');
      entities.forEach(function (e, i) {
        var ex = centre + (i - (entities.length - 1) / 2) * Math.min(96, (columnWidth - 40) / Math.max(1, entities.length));
        positions[e.id] = { x: ex, y: 162, align: 'centre' };
        lines.push([centre, 70, ex, 170]);
      });
      var rules = membersOf(concepts, a.id, 'invariant').concat(membersOf(concepts, a.id, 'assertion'));
      rules.forEach(function (r, i) { positions[r.id] = { x: x + 24, y: 200 + i * rowStep, align: 'left', maxWidth: columnWidth / 2 - 30, member: true }; });
      branch(centre, 46, x, rules);
      var events = raisedBy(concepts, a.id);
      events.forEach(function (e, i) { positions[e.id] = { x: x + columnWidth - 24, y: 200 + i * rowStep, align: 'right', maxWidth: columnWidth / 2 - 30 }; });
      height = Math.max(height, 200 + Math.max(rules.length, events.length) * rowStep + 30);
      x += columnWidth;
    });
    function column(items, members, width) {
      if (!items.length) return;
      var y = 46;
      items.forEach(function (c) {
        var needed = 36 + members(c).length * rowStep + 10;
        if (y > 46 && y + needed > columnHeight) { x += width; y = 46; }
        var owner = { x: x + width / 2, y: y };
        positions[c.id] = { x: owner.x, y: y, align: 'centre', maxWidth: width - 40 };
        y += 36;
        members(c).forEach(function (m) {
          positions[m.id] = { x: x + 24, y: y, align: 'left', maxWidth: width - 48, member: true };
          y += rowStep;
        });
        branch(owner.x, owner.y, x, members(c));
        y += 10;
        height = Math.max(height, y + 20);
      });
      x += width;
    }
    column(ofConcept(concepts, 'value_object'), function (v) { return membersOf(concepts, v.id, 'invariant'); }, 220);
    column(ofConcept(concepts, 'domain_service'), function (s) { return membersOf(concepts, s.id, 'assertion'); }, 220);
    var loose = concepts.filter(function (c) {
      return (c.concept === 'domain_event' && !c.raisedById) || c.concept === 'specification' || c.concept === 'question';
    });
    column(loose, function () { return []; }, 220);
    return { width: Math.max(440, x), height: height, positions: positions, lines: lines };
  }

  /* What the inspector shows for a selection, as recorded: each row names
     the recorded field and lists its values; a required field with no value
     is marked missing. */
  function row(field, values, required) {
    var vs = list(values).filter(function (v) { return v !== '' && v != null; });
    return { field: field, values: vs, missing: !!required && !vs.length };
  }
  function names(cs) { return cs.map(function (c) { return c.name; }); }
  function inspectContext(doc, ctx, concepts) {
    var sections = [{
      heading: ctx.name, kind: CONCEPT_LABEL.bounded_context, rows: [
        row('Definition', [ctx.definition], true),
        row('Aggregates', names(ofConcept(concepts, 'aggregate'))),
        row('Value objects', names(ofConcept(concepts, 'value_object'))),
        row('Domain events', names(ofConcept(concepts, 'domain_event'))),
        row('Domain services', names(ofConcept(concepts, 'domain_service'))),
        row('Specifications', names(ofConcept(concepts, 'specification'))),
        row('Questions', names(ofConcept(concepts, 'question')))
      ]
    }];
    var relations = relationsOf(doc, ctx.name);
    if (relations.length) {
      sections.push({ heading: 'Context map', rows: relations.map(function (r) {
        var other = r.from === ctx.name ? 'to ' + r.to : 'from ' + r.from;
        return row(r.kind, [other].concat(r.description ? [r.description] : []));
      }) });
    }
    return sections;
  }
  function inspect(doc, ctx, concepts, c) {
    if (!c) return inspectContext(doc, ctx, concepts);
    var owner = c.owner && byId(concepts, c.owner);
    var rows = [], sections;
    switch (c.concept) {
      case 'aggregate':
        rows = [row('Definition', [c.definition], true), row('Identity', [c.identity], true), row('Aliases', c.aliases),
          row('Entities', names(membersOf(concepts, c.id, 'entity'))), row('Domain events', names(raisedBy(concepts, c.id))),
          row('Repository', [c.repository]), row('Factory', [c.factory])];
        sections = [{ heading: c.name, kind: CONCEPT_LABEL.aggregate, rows: rows }];
        sections.push(rulesSection('Invariants', membersOf(concepts, c.id, 'invariant')));
        sections.push(rulesSection('Assertions', membersOf(concepts, c.id, 'assertion')));
        return sections;
      case 'entity':
        rows = [row('Definition', [c.definition], true), row('Identity', [c.identity]), row('Aliases', c.aliases), row('Aggregate', [owner && owner.name], true)];
        return [{ heading: c.name, kind: CONCEPT_LABEL.entity, rows: rows }];
      case 'value_object':
        rows = [row('Definition', [c.definition], true), row('Aliases', c.aliases)];
        return [{ heading: c.name, kind: CONCEPT_LABEL.value_object, rows: rows }, rulesSection('Invariants', membersOf(concepts, c.id, 'invariant'))];
      case 'domain_event':
        rows = [row('Definition', [c.definition], true), row('Raised by', [c.raisedBy])];
        return [{ heading: c.name, kind: CONCEPT_LABEL.domain_event, rows: rows }];
      case 'domain_service':
        rows = [row('Definition', [c.definition], true)];
        return [{ heading: c.name, kind: CONCEPT_LABEL.domain_service, rows: rows }, rulesSection('Assertions', membersOf(concepts, c.id, 'assertion'))];
      case 'invariant':
        rows = [row('Statement', [c.statement], true), row('Key', [c.name], true), row('Owner', [owner && owner.name], true), row('Verified by', c.verifiedBy)];
        return [{ heading: c.name, kind: CONCEPT_LABEL.invariant, rows: rows }];
      case 'assertion':
        rows = [row('Statement', [c.statement], true), row('Key', [c.name], true), row('On', [c.on], true), row('Owner', [owner && owner.name], true), row('Verified by', c.verifiedBy)];
        return [{ heading: c.name, kind: CONCEPT_LABEL.assertion, rows: rows }];
      case 'specification':
        return [{ heading: c.name, kind: CONCEPT_LABEL.specification, rows: [row('Definition', [c.definition], true)] }];
      case 'question':
        return [{ heading: c.name, kind: CONCEPT_LABEL.question, rows: [row('Text', [c.text], true)] }];
    }
    return [];
  }
  function rulesSection(heading, rules) {
    return { heading: heading, rows: rules.length ? rules.map(function (r) { return row(r.on || r.name, [r.statement], true); }) : [row('none recorded', [])] };
  }

  /* The counts line, in the overview's words. */
  function countsLine(counts) {
    counts = counts || {};
    function phrase(n, one, many) { return n + ' ' + (n === 1 ? one : many); }
    var parts = [phrase(counts.contexts || 0, 'context', 'contexts')];
    [['aggregates', 'aggregate', 'aggregates'], ['entities', 'entity', 'entities'], ['valueObjects', 'value object', 'value objects'],
      ['invariants', 'invariant', 'invariants'], ['assertions', 'assertion', 'assertions'], ['specifications', 'specification', 'specifications'],
      ['events', 'event', 'events'], ['services', 'service', 'services'], ['questions', 'question', 'questions'], ['relations', 'relation', 'relations']
    ].forEach(function (t) { if (counts[t[0]] > 0) parts.push(phrase(counts[t[0]], t[1], t[2])); });
    return parts.join(' · ');
  }

  /* The concepts a context's map draws, in legend order. */
  function legend(concepts) {
    return CONCEPTS.filter(function (k) { return k !== 'bounded_context' && ofConcept(concepts, k).length > 0; });
  }

  /* The path under the map: context, the owner when there is one, the entry. */
  function path(ctx, concepts, c) {
    var parts = [ctx ? ctx.name : ''];
    if (c) {
      var owner = c.owner && byId(concepts, c.owner);
      if (owner) parts.push(owner.name);
      parts.push(c.name);
    }
    return parts;
  }

  /* --- Editing: the recorded properties of each concept, as the define
     flags spell them; the same names the define request carries. A list
     property is edited one value per line. --- */
  var PROPERTIES = {
    bounded_context: [{ name: 'definition', label: 'Definition', required: true, long: true }],
    aggregate: [{ name: 'definition', label: 'Definition', required: true, long: true }, { name: 'identity', label: 'Identity', required: true },
      { name: 'aliases', label: 'Aliases', list: true }, { name: 'repository', label: 'Repository' }, { name: 'factory', label: 'Factory' }],
    entity: [{ name: 'definition', label: 'Definition', required: true, long: true }, { name: 'identity', label: 'Identity' }, { name: 'aliases', label: 'Aliases', list: true }],
    value_object: [{ name: 'definition', label: 'Definition', required: true, long: true }, { name: 'aliases', label: 'Aliases', list: true }],
    domain_event: [{ name: 'definition', label: 'Definition', required: true, long: true }, { name: 'raisedBy', label: 'Raised by' }],
    domain_service: [{ name: 'definition', label: 'Definition', required: true, long: true }],
    invariant: [{ name: 'statement', label: 'Statement', required: true, long: true }, { name: 'verifiedBy', label: 'Verified by', list: true }],
    assertion: [{ name: 'on', label: 'On', required: true }, { name: 'statement', label: 'Statement', required: true, long: true }, { name: 'verifiedBy', label: 'Verified by', list: true }],
    specification: [{ name: 'definition', label: 'Definition', required: true, long: true }],
    question: [{ name: 'text', label: 'Text', required: true, long: true }]
  };
  /* The concept an entry of each nested concept is recorded under. */
  var OWNER_CONCEPTS = { entity: ['aggregate'], invariant: ['aggregate', 'value_object'], assertion: ['aggregate', 'domain_service'] };

  /* One entry's recorded properties, as a plain object of the property names above. */
  function propertiesOf(c) {
    var out = {};
    (PROPERTIES[c.concept] || []).forEach(function (p) { out[p.name] = p.list ? list(c[p.name]).slice() : String(c[p.name] == null ? '' : c[p.name]); });
    return out;
  }
  function sameValue(a, b) { return Array.isArray(a) || Array.isArray(b) ? list(a).join('\n') === list(b).join('\n') : String(a == null ? '' : a) === String(b == null ? '' : b); }

  /* The key that names one entry across recordings: its context, concept,
     owner and name, parted by a separator no recorded name holds. */
  function key(context, concept, owner, name) { return context + '\u001f' + concept + '\u001f' + owner + '\u001f' + name; }
  function ownerName(concepts, c) { var o = c.owner && byId(concepts, c.owner); return o ? o.name : ''; }

  /* Every entry a recording holds, keyed, each with its properties: the
     bounded contexts, then what each records, then the relations of the
     context map, one per pair of contexts. */
  function catalog(doc) {
    var out = {}, order = [];
    list(doc && doc.contexts).forEach(function (ctx) {
      var k = key(ctx.name, 'bounded_context', '', ctx.name);
      out[k] = { key: k, context: ctx.name, concept: 'bounded_context', owner: '', name: ctx.name, properties: { definition: ctx.definition || '' } };
      order.push(k);
      var concepts = conceptsOf(ctx);
      concepts.forEach(function (c) {
        var owner = ownerName(concepts, c);
        var ck = key(ctx.name, c.concept, owner, c.name);
        out[ck] = { key: ck, context: ctx.name, concept: c.concept, owner: owner, name: c.name, properties: propertiesOf(c) };
        order.push(ck);
      });
    });
    list(doc && doc.relations).forEach(function (r) {
      var rk = key(r.from, 'relation', '', r.to);
      out[rk] = { key: rk, context: '', concept: 'relation', owner: '', name: r.from + ' \u2192 ' + r.to, properties: { kind: r.kind || '', description: r.description || '' } };
      order.push(rk);
    });
    return { entries: out, order: order };
  }

  /* The transformation from one recording to another: each entry added,
     removed or changed, with every property that differs, before and
     after. Entries follow the later recording's order, the removed ones
     after it in the earlier's. */
  function transformation(before, after) {
    var a = catalog(before), b = catalog(after), out = [];
    b.order.forEach(function (k) {
      var was = a.entries[k], now = b.entries[k];
      if (!was) return out.push({ key: k, context: now.context, concept: now.concept, owner: now.owner, name: now.name, kind: 'added',
        fields: Object.keys(now.properties).filter(function (f) { return !sameValue(now.properties[f], ''); }).map(function (f) { return { field: f, before: null, after: now.properties[f] }; }) });
      var fields = Object.keys(now.properties).filter(function (f) { return !sameValue(was.properties[f], now.properties[f]); })
        .map(function (f) { return { field: f, before: was.properties[f], after: now.properties[f] }; });
      if (fields.length) out.push({ key: k, context: now.context, concept: now.concept, owner: now.owner, name: now.name, kind: 'changed', fields: fields });
    });
    a.order.forEach(function (k) {
      if (b.entries[k]) return;
      var was = a.entries[k];
      out.push({ key: k, context: was.context, concept: was.concept, owner: was.owner, name: was.name, kind: 'removed',
        fields: Object.keys(was.properties).filter(function (f) { return !sameValue(was.properties[f], ''); }).map(function (f) { return { field: f, before: was.properties[f], after: null }; }) });
    });
    return out;
  }

  /* The standing of every entry of a recording against the accepted one:
     Proposed for an entry the transformation touches, Accepted otherwise;
     nothing when no accepted recording is known. */
  function standing(accepted, doc) {
    var out = {};
    if (!accepted) return out;
    transformation(accepted, doc).forEach(function (t) { out[t.key] = 'Proposed'; });
    return out;
  }

  /* --- Drafts: edits the page holds until Publish. A define draft names
     an entry and the properties to set; a remove draft names the entry.
     Drafts are keyed, so a second edit of one entry merges into the first
     and keeps its place: an owner drafted before what is recorded under it
     still publishes first.
     --- */
  function drafted(drafts, draft) {
    var at = drafts.map(function (d) { return d.key; }).indexOf(draft.key);
    var prior = at >= 0 ? drafts[at] : null;
    var out = drafts.slice();
    if (draft.action === 'define' && prior && prior.action === 'define') {
      var change = {};
      Object.keys(prior.change).forEach(function (f) { change[f] = prior.change[f]; });
      Object.keys(draft.change).forEach(function (f) { change[f] = draft.change[f]; });
      draft = { key: draft.key, action: 'define', context: draft.context, concept: draft.concept, owner: draft.owner, name: draft.name, change: change, created: prior.created };
    }
    if (draft.action === 'remove' && prior && prior.created) { out.splice(at, 1); return out; }
    if (at >= 0) out[at] = draft; else out.push(draft);
    return out;
  }
  /* The drafts once one of them is published: that draft goes, and a newer
     draft of the same entry, saved while it was publishing, stays to be
     published next, now against an entry that exists after a define and
     does not after a remove. */
  function published(drafts, draft) {
    return drafts.filter(function (d) { return d !== draft; })
      .map(function (d) { return d.key === draft.key ? Object.assign({}, d, { created: draft.action === 'remove' }) : d; });
  }
  function withoutDraft(drafts, k) { return drafts.filter(function (d) { return d.key !== k; }); }
  function draftOf(drafts, k) { return drafts.filter(function (d) { return d.key === k; })[0] || null; }

  /* The body a define or remove draft posts: the entry as `arclint domain
     define <concept> <name>` names it, and for a define the properties it
     sets, a list as a list. */
  function requestOf(draft) {
    var body = { concept: draft.concept, context: draft.concept === 'bounded_context' ? '' : draft.context, owner: draft.owner || '', name: draft.name };
    if (draft.action === 'define') body.change = draft.change;
    return body;
  }

  function clone(v) { return JSON.parse(JSON.stringify(v)); }
  var SECTION = { aggregate: 'aggregates', value_object: 'valueObjects', domain_event: 'events', domain_service: 'services', specification: 'specifications', question: 'questions' };
  var NAME_FIELD = { invariant: 'key', assertion: 'key', question: 'key' };
  function nameOf(concept, item) { return item[NAME_FIELD[concept] || 'name']; }
  function findNamed(items, concept, name) { return list(items).filter(function (i) { return nameOf(concept, i) === name; })[0] || null; }
  /* Where an entry of a context lives: the array that holds it, in the owner when nested. */
  function sectionFor(ctx, concept, owner) {
    if (!OWNER_CONCEPTS[concept]) { ctx[SECTION[concept]] = ctx[SECTION[concept]] || []; return ctx[SECTION[concept]]; }
    var holder = null;
    OWNER_CONCEPTS[concept].forEach(function (oc) { holder = holder || findNamed(ctx[SECTION[oc]], oc, owner); });
    if (!holder) return null;
    var field = concept === 'entity' ? 'entities' : concept === 'invariant' ? 'invariants' : 'assertions';
    holder[field] = holder[field] || [];
    return holder[field];
  }
  function setProperty(item, field, value) {
    if (Array.isArray(value)) { if (value.length) item[field] = value.slice(); else delete item[field]; return; }
    if (value === '' && field !== 'definition' && field !== 'statement' && field !== 'text' && field !== 'on' && field !== 'identity') delete item[field]; else item[field] = value;
  }

  /* The recording with the drafts applied: what the domain file would
     record once every draft is published as drafted. A removal takes
     what is recorded under the entry with it, as remove does. */
  function applyDrafts(doc, drafts) {
    var out = clone(doc || {});
    out.contexts = list(out.contexts);
    drafts.forEach(function (d) {
      if (d.concept === 'bounded_context') {
        var i = out.contexts.map(function (c) { return c.name; }).indexOf(d.name);
        if (d.action === 'remove') {
          if (i >= 0) out.contexts.splice(i, 1);
          out.relations = list(out.relations).filter(function (r) { return r.from !== d.name && r.to !== d.name; });
          return;
        }
        if (i < 0) { out.contexts.push({ name: d.name, definition: '' }); i = out.contexts.length - 1; }
        Object.keys(d.change).forEach(function (f) { setProperty(out.contexts[i], f, d.change[f]); });
        return;
      }
      var ctx = out.contexts.filter(function (c) { return c.name === d.context; })[0];
      if (!ctx) return;
      var items = sectionFor(ctx, d.concept, d.owner);
      if (!items) return;
      var at = items.map(function (it) { return nameOf(d.concept, it); }).indexOf(d.name);
      if (d.action === 'remove') {
        if (at >= 0) items.splice(at, 1);
        if (d.concept === 'aggregate') list(ctx.events).forEach(function (e) { if (e.raisedBy === d.name) delete e.raisedBy; });
        return;
      }
      if (at < 0) { var fresh = {}; fresh[NAME_FIELD[d.concept] || 'name'] = d.name; items.push(fresh); at = items.length - 1; }
      Object.keys(d.change).forEach(function (f) { setProperty(items[at], f, d.change[f]); });
    });
    return out;
  }

  /* The edit form of one entry: each property with its current value, as
     text; a list one value per line. For a new entry, every property empty. */
  function form(concept, properties) {
    return (PROPERTIES[concept] || []).map(function (p) {
      var v = properties ? properties[p.name] : (p.list ? [] : '');
      return { name: p.name, label: p.label, required: !!p.required, long: !!p.long, list: !!p.list, value: p.list ? list(v).join('\n') : String(v == null ? '' : v) };
    });
  }
  /* What a filled form changes against the entry's properties: each
     property whose text differs, parsed back (a list one value per line,
     blank lines dropped). For a new entry every filled property. */
  function changeFrom(concept, properties, values) {
    var change = {};
    (PROPERTIES[concept] || []).forEach(function (p) {
      var text = String(values[p.name] == null ? '' : values[p.name]);
      var parsed = p.list ? text.split('\n').map(function (s) { return s.trim(); }).filter(Boolean) : text.trim();
      if (!properties) { if (p.list ? parsed.length : parsed !== '') change[p.name] = parsed; return; }
      if (!sameValue(properties[p.name], parsed)) change[p.name] = parsed;
    });
    return change;
  }
  /* The owners a nested concept may be recorded under in a context. */
  function ownersFor(concepts, concept) {
    var out = [];
    (OWNER_CONCEPTS[concept] || []).forEach(function (oc) { ofConcept(concepts, oc).forEach(function (c) { out.push({ concept: oc, name: c.name }); }); });
    return out;
  }

  return {
    CONCEPTS: CONCEPTS, CONCEPT_LABEL: CONCEPT_LABEL, PROPERTIES: PROPERTIES, OWNER_CONCEPTS: OWNER_CONCEPTS, esc: esc,
    key: key, catalog: catalog, transformation: transformation, standing: standing, propertiesOf: propertiesOf, ownerName: ownerName,
    drafted: drafted, published: published, withoutDraft: withoutDraft, draftOf: draftOf, requestOf: requestOf, applyDrafts: applyDrafts,
    form: form, changeFrom: changeFrom, ownersFor: ownersFor,
    fit: fit, conceptsOf: conceptsOf, byId: byId, ofConcept: ofConcept, membersOf: membersOf, raisedBy: raisedBy, label: label,
    context: context, relationsOf: relationsOf, navigator: navigator, matches: matches, layout: layout,
    inspect: inspect, countsLine: countsLine, legend: legend, path: path
  };
});
