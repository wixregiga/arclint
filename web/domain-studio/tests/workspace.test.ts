import { test } from 'node:test';
import assert from 'node:assert/strict';
import { readFileSync } from 'node:fs';
import { parse } from 'yaml';
import { createEmptyProject, createExampleProject } from '../src/domain';
import { exportDomainYaml, exportProject, importProject } from '../src/serialization';
import { assembleProject, createWorkspace, migrateWorkspace, projectModel } from '../src/workspace';

const source = readFileSync(new URL('../../../domain.arclint.yaml', import.meta.url), 'utf8');
const fixture = () => importProject(source);

test('v1 migration preserves the complete actual reference library and prior model snapshot', () => {
  const project = fixture();
  const previous = structuredClone(project); previous.name = 'Earlier model';
  const baseline = { name: 'Before review', capturedAt: '2026-09-12T12:30:00.000Z', project: previous };
  const patterns = [{ name: 'example/policy@1.0.0', description: 'A reference', rules: [{ id: 'example/policy:local', description: 'Recorded proposition' }], origin: { path: 'local/pattern.yaml' } }];
  const saved = JSON.stringify({ project, baseline, patterns });
  const workspace = createWorkspace(saved);
  assert.deepEqual(workspace.project, project);
  assert.deepEqual(workspace.project.sourceDocument, parse(source));
  assert.deepEqual(workspace.modelSnapshot, baseline);
  assert.deepEqual(workspace.patterns, patterns);
  assert.equal(workspace.snapshot().version, 2);
  assert.equal('baseline' in workspace.snapshot(), false);
  assert.deepEqual(parse(exportDomainYaml(workspace.project)), parse(source));
  assert.deepEqual(createWorkspace(workspace.serialize()).snapshot(), workspace.snapshot());
});

test('the semantic projection contains no layout fields and reconstructs the adapter losslessly', () => {
  const project = fixture();
  const projection = projectModel(project);
  for (const context of projection.model.contexts) {
    assert.equal('position' in context, false);
    assert.equal('color' in context, false);
  }
  for (const concept of projection.model.concepts) assert.equal('position' in concept, false);
  assert.deepEqual(projection.model.sourceDocument, project.sourceDocument);
  assert.deepEqual(assembleProject(projection), project);
  assert.deepEqual(importProject(exportProject(assembleProject(projection))), project);
});

test('representation, policy overlay and layout changes do not change canonical export', () => {
  const workspace = createWorkspace(undefined, fixture());
  const original = exportDomainYaml(workspace.project);
  const semantic = workspace.model;
  const concept = workspace.project.concepts[0];
  workspace.updateView({ selectedId: concept.id, scopeId: concept.contextId, page: 2 });
  workspace.apply('Arrange a concept', project => { project.concepts[0].position = [17, 2, -11]; project.contexts[0].color = '#123456'; });
  workspace.updateView({ representation: 'table', lens: 'governance' });
  assert.equal(exportDomainYaml(workspace.project), original);
  assert.deepEqual(workspace.model, semantic);
  assert.equal(workspace.view.selectedId, concept.id);
  assert.equal(workspace.view.scopeId, concept.contextId);
  assert.equal(workspace.view.page, 2);
  assert.equal(workspace.history.length, 1);
  assert.equal(workspace.history[0].kind, 'layout.edit');
  assert.equal(workspace.undo(), true);
  assert.equal(workspace.view.representation, 'table');
  assert.equal(workspace.view.lens, 'governance');
  assert.equal(exportDomainYaml(workspace.project), original);
});

test('unassigned notebook text survives switching, reload and model replacement exactly', () => {
  const workspace = createWorkspace(undefined, fixture());
  const before = exportDomainYaml(workspace.project);
  const value = '  What does “Book” mean here?\r\n\t配送 📚\n\n';
  const draft = workspace.saveDraft(value);
  workspace.updateView({ representation: 'table', lens: 'governance' });
  assert.equal(workspace.notebook[0].text, value);
  assert.equal(exportDomainYaml(workspace.project), before);
  const loaded = createWorkspace(workspace.serialize());
  assert.deepEqual(loaded.notebook, [draft]);
  assert.equal(loaded.project.contexts.some(context => /draft/i.test(context.name)), false);
  loaded.replace(createEmptyProject());
  assert.equal(loaded.notebook[0].text, value);
  assert.equal(loaded.project.contexts.length, 0);
});

test('editor fields persist across representations and overlays without entering semantic history', () => {
  const workspace = createWorkspace(undefined, fixture());
  const subject = workspace.project.concepts[0].id;
  const fields = { name: '  Unfinished name ', definition: '\nExact text\n  ', kind: '', invariants: 'First\r\nSecond' };
  workspace.saveEditorDraft(subject, fields);
  workspace.updateView({ selectedId: subject, representation: 'table', lens: 'governance' });
  assert.deepEqual(workspace.getEditorDraft(subject)?.fields, fields);
  assert.deepEqual(createWorkspace(workspace.serialize()).getEditorDraft(subject)?.fields, fields);
  assert.equal(workspace.history.length, 0);
  workspace.apply('Update a definition', project => { project.concepts[0].definition = 'Agreed definition.'; });
  workspace.removeEditorDraft(subject);
  assert.equal(workspace.history.length, 1);
  assert.equal(workspace.undo(), true);
  assert.equal(workspace.project.concepts[0].definition, fixture().concepts[0].definition);
  assert.equal(workspace.view.representation, 'table');
});

test('typed shared history restores model snapshots and Patterns across replacement', () => {
  const project = fixture();
  const baseline = { name: 'Reference', capturedAt: '2026-09-12T12:00:00Z', project: structuredClone(project) };
  const patterns = [{ name: 'local', description: 'Policy', rules: [{ id: 'local', description: 'Constraint' }] }];
  const workspace = createWorkspace({ project, baseline, patterns });
  workspace.replace(createEmptyProject(), 'Start another domain');
  assert.equal(workspace.modelSnapshot, null);
  assert.deepEqual(workspace.patterns, []);
  assert.equal(workspace.history.at(-1)?.kind, 'workspace.replace');
  workspace.updateView({ representation: 'table' });
  assert.equal(workspace.undo(), true);
  assert.deepEqual(workspace.project, project);
  assert.deepEqual(workspace.modelSnapshot, baseline);
  assert.deepEqual(workspace.patterns, patterns);
  assert.equal(workspace.view.representation, 'table');
  assert.equal(workspace.redo(), true);
  assert.deepEqual(workspace.project, createEmptyProject());
  assert.equal(workspace.modelSnapshot, null);
  assert.deepEqual(workspace.patterns, []);
});

test('notebook typing coalesces without losing exact undo/redo content', () => {
  const workspace = createWorkspace();
  const draft = workspace.saveDraft('A');
  workspace.saveDraft('A\n ', draft.id);
  workspace.saveDraft('A\n  finished\n', draft.id);
  assert.equal(workspace.history.length, 1);
  assert.equal(workspace.history[0].kind, 'notebook.write');
  workspace.updateView({ representation: 'table' });
  workspace.undo();
  assert.equal(workspace.notebook.length, 0);
  workspace.redo();
  assert.equal(workspace.notebook[0].text, 'A\n  finished\n');
  workspace.removeDraft(draft.id);
  assert.equal(workspace.notebook.length, 0);
  workspace.undo();
  assert.equal(workspace.notebook[0].text, 'A\n  finished\n');
});

test('adapter getters and persisted snapshots cannot mutate shared state by reference', () => {
  const workspace = createWorkspace(undefined, fixture());
  const name = workspace.project.name;
  workspace.project.name = 'Changed through a getter';
  workspace.model.name = 'Changed through semantic getter';
  workspace.snapshot().model.name = 'Changed through snapshot';
  const id = workspace.project.contexts[0].id;
  workspace.layout[id].position[0] = 999;
  assert.equal(workspace.project.name, name);
  assert.notEqual(workspace.layout[id].position[0], 999);
});

test('invalid mutations fail atomically and new edits discard only the redo branch', () => {
  const workspace = createWorkspace(undefined, createExampleProject());
  const before = workspace.snapshot();
  assert.throws(() => workspace.apply('Invalid owner', project => { project.concepts[0].contextId = 'missing'; }), /missing context/);
  assert.deepEqual(workspace.snapshot(), before);
  assert.equal(workspace.canUndo, false);
  workspace.apply('Rename project', project => { project.name = 'First edit'; });
  assert.equal(workspace.history.at(-1)?.kind, 'model.edit');
  workspace.undo();
  workspace.apply('Different edit', project => { project.name = 'Another edit'; return project; });
  assert.equal(workspace.canRedo, false);
  assert.equal(workspace.project.name, 'Another edit');
});

test('subject-like prototype keys remain ordinary data in layout and editor drafts', () => {
  const project = createEmptyProject();
  project.contexts.push({ id: '__proto__', name: 'special', description: 'A boundary.', position: [0, 0, 0], color: '#abcdef' });
  const workspace = createWorkspace(undefined, project);
  workspace.saveEditorDraft('__proto__', Object.fromEntries([['__proto__', 'Exact field text']]));
  const loaded = createWorkspace(workspace.serialize());
  assert.deepEqual(loaded.project, project);
  assert.equal(loaded.getEditorDraft('__proto__')?.fields.__proto__, 'Exact field text');
  assert.equal(loaded.getEditorDraft('constructor'), null);
});

test('migration rejects missing layout or malformed data instead of silently losing it', () => {
  const saved = migrateWorkspace(undefined, fixture());
  delete saved.layout[saved.model.contexts[0].id];
  assert.throws(() => createWorkspace(saved), /Layout is missing/);
  assert.throws(() => createWorkspace({ version: 3 }), /Unsupported workspace version/);
  assert.throws(() => createWorkspace({ project: fixture(), patterns: [{ name: 'bad', rules: [] }] }), /description/);
});

test('restoring a complete workspace is atomic and Undo restores the prior model, selection and exact drafts', () => {
  const workspace = createWorkspace(undefined, fixture());
  workspace.saveDraft('  Original unassigned text\n');
  const subject = workspace.project.concepts[0];
  workspace.saveEditorDraft(`editor:${subject.id}`, { definition: '  Not saved yet\n' });
  workspace.updateView({ selectedId: subject.id, scopeId: subject.contextId, page: 1 });
  const before = workspace.snapshot();
  const other = createWorkspace(undefined, createExampleProject());
  other.saveDraft('\tAnother notebook');
  other.saveEditorDraft('editor:skill', { definition: 'Other draft' });
  other.updateView({ representation: 'table', selectedId: 'skill', scopeId: 'context-content' });
  other.setModelSnapshot({ name: 'Other reference', capturedAt: '2026-09-13T00:00:00Z', project: other.project });
  const imported = other.snapshot();
  workspace.restore(imported, 'Import downloaded workspace');
  assert.deepEqual(workspace.snapshot(), imported);
  assert.equal(workspace.history.at(-1)?.kind, 'workspace.restore');
  workspace.undo();
  assert.deepEqual(workspace.snapshot(), before);
  workspace.redo();
  assert.deepEqual(workspace.snapshot(), imported);
  const invalid = other.snapshot(); delete invalid.layout['skill'];
  const history = workspace.history;
  assert.throws(() => workspace.restore(invalid), /Layout is missing/);
  assert.deepEqual(workspace.snapshot(), imported);
  assert.deepEqual(workspace.history, history);
});

test('replacing a model does not permanently discard its unfinished editor drafts', () => {
  const workspace = createWorkspace(undefined, fixture());
  const subject = workspace.project.concepts[0].id;
  workspace.saveEditorDraft(subject, { definition: ' An unfinished thought ' });
  workspace.replace(createEmptyProject());
  assert.equal(workspace.getEditorDraft(subject), null);
  workspace.undo();
  assert.equal(workspace.getEditorDraft(subject)?.fields.definition, ' An unfinished thought ');
});

test('assigning notebook text and creating its domain entry form one atomic undoable action', () => {
  const workspace = createWorkspace(undefined, createExampleProject());
  const draft = workspace.saveDraft('  A meaning to be assigned.\n');
  const previous = workspace.project;
  const contextId = previous.contexts[0].id;
  workspace.apply('Record the drafted meaning', project => {
    project.concepts.push({ id: 'new-meaning', name: 'NewMeaning', kind: 'unclassified', contextId, definition: draft.text.trim(), invariants: [], position: [1, 1, 1] });
  }, { consumeDraftId: draft.id });
  assert.equal(workspace.notebook.length, 0);
  assert.equal(workspace.history.at(-1)?.kind, 'model.edit');
  workspace.undo();
  assert.deepEqual(workspace.project, previous);
  assert.deepEqual(workspace.notebook, [draft]);
  assert.throws(() => workspace.apply('Invalid assignment', project => {
    project.concepts[0].contextId = 'missing';
  }, { consumeDraftId: draft.id }), /missing context/);
  assert.deepEqual(workspace.project, previous);
  assert.deepEqual(workspace.notebook, [draft]);
  workspace.redo();
  assert.equal(workspace.notebook.length, 0);
  assert.ok(workspace.project.concepts.some(concept => concept.id === 'new-meaning'));
});

test('saving definition fields clears only their draft atomically and Undo retains other unfinished forms', () => {
  const workspace = createWorkspace(undefined, fixture());
  const root = workspace.project.concepts[0];
  const key = `editor:${root.id}`;
  const fields = { definition: ' A proposed revision. ' };
  workspace.saveEditorDraft(key, fields);
  assert.throws(() => workspace.apply('Invalid save', project => { project.concepts[0].contextId = 'missing'; }, { clearEditorDraftId: key }), /missing context/);
  assert.deepEqual(workspace.getEditorDraft(key)?.fields, fields);
  workspace.apply('Save definition', project => { project.concepts[0].definition = fields.definition.trim(); }, { clearEditorDraftId: key });
  assert.equal(workspace.getEditorDraft(key), null);
  workspace.saveEditorDraft('another-form', { definition: 'Do not discard this work.' });
  workspace.undo();
  assert.equal(workspace.project.concepts[0].definition, root.definition);
  assert.deepEqual(workspace.getEditorDraft(key)?.fields, fields);
  assert.equal(workspace.getEditorDraft('another-form')?.fields.definition, 'Do not discard this work.');
  workspace.redo();
  assert.equal(workspace.getEditorDraft(key), null);
  assert.equal(workspace.getEditorDraft('another-form')?.fields.definition, 'Do not discard this work.');
});

test('legacy source-only operation contracts become editable without losing earlier assertions', () => {
  const canonical = `version: 1
project: shop
contexts:
  orders:
    definition: Ordering language.
    aggregates:
      Order:
        definition: An accepted purchase.
        identity: OrderID
        assertions:
          confirmation-ready:
            on: Confirm
            statement: Every line has an accepted price.
    events:
      OrderConfirmed:
        definition: An order was confirmed.
        raised_by: Order.Confirm
`;
  const project = importProject(canonical);
  const aggregate = project.concepts.find(concept => concept.kind === 'aggregate')!;
  const event = project.concepts.find(concept => concept.kind === 'domain_event')!;
  const originalAssertion = aggregate.assertions![0];
  delete aggregate.assertions;
  delete event.raisedBy;
  const snapshot = { name: 'Legacy reference', capturedAt: '2026-09-12T12:00:00Z', project };
  const workspace = createWorkspace({ project, baseline: snapshot });
  assert.deepEqual(workspace.project.sourceDocument, project.sourceDocument);
  assert.deepEqual(workspace.project.concepts.map(concept => [concept.id, concept.kind, concept.position]), project.concepts.map(concept => [concept.id, concept.kind, concept.position]));
  assert.deepEqual(workspace.project.concepts.find(concept => concept.id === aggregate.id)?.assertions, [originalAssertion]);
  assert.equal(workspace.project.concepts.find(concept => concept.id === event.id)?.raisedBy, 'Order.Confirm');
  assert.deepEqual(workspace.modelSnapshot?.project, workspace.project);
  assert.deepEqual(parse(exportDomainYaml(workspace.project)), parse(canonical));
  assert.deepEqual(workspace.history, []);
  const imported = createWorkspace();
  imported.replace(importProject(exportProject(project)), 'Import legacy Studio JSON');
  assert.deepEqual(imported.project.concepts.find(concept => concept.id === aggregate.id)?.assertions, [originalAssertion]);
  assert.equal(imported.project.concepts.find(concept => concept.id === event.id)?.raisedBy, 'Order.Confirm');
  workspace.apply('Add another operation assertion', next => {
    next.concepts.find(concept => concept.id === aggregate.id)!.assertions!.push({ key: 'cancellation-complete', on: 'Cancel', statement: 'The order cannot be confirmed.' });
  });
  const saved = parse(exportDomainYaml(workspace.project));
  assert.deepEqual(saved.contexts.orders.aggregates.Order.assertions, {
    'confirmation-ready': { on: 'Confirm', statement: 'Every line has an accepted price.' },
    'cancellation-complete': { on: 'Cancel', statement: 'The order cannot be confirmed.' },
  });
  workspace.undo();
  assert.deepEqual(parse(exportDomainYaml(workspace.project)), parse(canonical));

  // An older v2 envelope can also carry the source-only adapter. Explicit edits
  // in a current envelope must take precedence over the retained source record.
  const olderV2 = workspace.snapshot();
  delete olderV2.model.concepts.find(concept => concept.id === aggregate.id)!.assertions;
  assert.deepEqual(createWorkspace(olderV2).project.concepts.find(concept => concept.id === aggregate.id)?.assertions, [originalAssertion]);
  workspace.apply('Remove operation assertions', next => { next.concepts.find(concept => concept.id === aggregate.id)!.assertions = []; });
  workspace.apply('Record the event source', next => { next.concepts.find(concept => concept.id === event.id)!.raisedBy = 'Order.Reconfirm'; });
  const restored = createWorkspace(workspace.serialize());
  assert.deepEqual(restored.project.concepts.find(concept => concept.id === aggregate.id)?.assertions, []);
  assert.equal(restored.project.concepts.find(concept => concept.id === event.id)?.raisedBy, 'Order.Reconfirm');
  assert.deepEqual(restored.project.sourceDocument, project.sourceDocument);
});
