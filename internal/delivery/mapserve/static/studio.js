/* The map's model: pure functions over the domain overview document
   `arclint domain overview --format json` prints. No DOM. Loads as a
   <script> (window.Studio) and under node (require) for its tests. */
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
    specification: 'Specification', question: 'Question'
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
  /* What a node says on the map: the name, or the statement a rule records. */
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
     long column neither towers nor flattens into a strip. */
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
      rules.forEach(function (r, i) { positions[r.id] = { x: x + 24, y: 200 + i * rowStep, align: 'left', maxWidth: columnWidth / 2 - 30 }; });
      var events = raisedBy(concepts, a.id);
      events.forEach(function (e, i) { positions[e.id] = { x: x + columnWidth - 24, y: 200 + i * rowStep, align: 'right', maxWidth: columnWidth / 2 - 30 }; });
      height = Math.max(height, 200 + Math.max(rules.length, events.length) * rowStep + 30);
      x += columnWidth;
    });
    function column(items, members, width) {
      if (!items.length) return;
      var y = 46;
      items.forEach(function (c) {
        var needed = 30 + members(c).length * rowStep + 10;
        if (y > 46 && y + needed > columnHeight) { x += width; y = 46; }
        positions[c.id] = { x: x + width / 2, y: y, align: 'centre', maxWidth: width - 40 };
        y += 30;
        members(c).forEach(function (m) {
          positions[m.id] = { x: x + 24, y: y, align: 'left', maxWidth: width - 48 };
          y += rowStep;
        });
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

  return {
    CONCEPTS: CONCEPTS, CONCEPT_LABEL: CONCEPT_LABEL, esc: esc,
    fit: fit, conceptsOf: conceptsOf, byId: byId, ofConcept: ofConcept, membersOf: membersOf, raisedBy: raisedBy, label: label,
    context: context, relationsOf: relationsOf, navigator: navigator, matches: matches, layout: layout,
    inspect: inspect, countsLine: countsLine, legend: legend, path: path
  };
});
