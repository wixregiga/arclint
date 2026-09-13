import type { Concept, DomainContext, DomainProject, PatternSummary } from './contracts';
import { assertProject, createEmptyProject, makeId } from './domain';
import { sourceTerm } from './model-evidence';

/** The recorded model has no renderer coordinates or colours. */
export type SemanticContext = Omit<DomainContext, 'position' | 'color'>;
export type SemanticConcept = Omit<Concept, 'position'>;
export interface SemanticProject extends Omit<DomainProject, 'contexts' | 'concepts'> {
  contexts: SemanticContext[];
  concepts: SemanticConcept[];
}
export interface LayoutEntry { position: [number, number, number]; color?: string }
export type LayoutMap = Record<string, LayoutEntry>;
export interface ModelProjection { model: SemanticProject; layout: LayoutMap }
/** Compatibility adapter for callers comparing two DomainProjects. Not an ArcLint Baseline. */
export interface ModelSnapshot { name: string; capturedAt: string; project: DomainProject }
export interface StoredModelSnapshot extends ModelProjection { name: string; capturedAt: string }
export type Representation = 'spatial' | 'table';
export interface WorkspaceView {
  selectedId: string | null;
  scopeId: string | null;
  aggregateId: string | null;
  page: number;
  representation: Representation;
  lens: 'meaning' | 'governance';
}
/** Unassigned text is deliberately outside the canonical Domain Model. */
export interface NotebookDraft { id: string; text: string; createdAt: string; updatedAt: string }
export interface EditorDraft { subjectId: string; fields: Record<string, string>; updatedAt: string }
export interface WorkspaceEnvelope extends ModelProjection {
  version: 2;
  modelSnapshot: StoredModelSnapshot | null;
  patterns: PatternSummary[];
  view: WorkspaceView;
  notebook: NotebookDraft[];
  editorDrafts: Record<string, EditorDraft>;
}
export type WorkspaceCommandKind = 'model.edit' | 'layout.edit' | 'workspace.replace' | 'workspace.restore' | 'snapshot.capture' | 'patterns.change' | 'notebook.write' | 'notebook.remove';
export interface WorkspaceCommand { id: string; kind: WorkspaceCommandKind; label: string; createdAt: string; subjectId?: string }
export interface WorkspaceApplyOptions { consumeDraftId?: string; clearEditorDraftId?: string }
export interface Workspace {
  readonly project: DomainProject;
  readonly model: SemanticProject;
  readonly layout: LayoutMap;
  readonly modelSnapshot: ModelSnapshot | null;
  readonly patterns: PatternSummary[];
  readonly view: WorkspaceView;
  readonly notebook: NotebookDraft[];
  readonly canUndo: boolean;
  readonly canRedo: boolean;
  readonly history: WorkspaceCommand[];
  apply(label: string, mutation: (project: DomainProject) => void | DomainProject, options?: WorkspaceApplyOptions): void;
  replace(project: DomainProject, label?: string): void;
  restore(saved: unknown, label?: string): void;
  updateView(patch: Partial<WorkspaceView>): void;
  setModelSnapshot(snapshot: ModelSnapshot | null): void;
  setPatterns(patterns: PatternSummary[]): void;
  saveDraft(text: string, id?: string): NotebookDraft;
  removeDraft(id: string): void;
  saveEditorDraft(subjectId: string, fields: Record<string, string>): EditorDraft;
  getEditorDraft(subjectId: string): EditorDraft | null;
  removeEditorDraft(subjectId: string): void;
  undo(): boolean;
  redo(): boolean;
  snapshot(): WorkspaceEnvelope;
  serialize(): string;
}

const clone = <T>(value: T): T => structuredClone(value);
const defaultView = (): WorkspaceView => ({ selectedId: null, scopeId: null, aggregateId: null, page: 0, representation: 'spatial', lens: 'meaning' });
const same = (left: unknown, right: unknown): boolean => JSON.stringify(left) === JSON.stringify(right);
const mapping = (value: unknown, label: string): Record<string, unknown> => {
  if (!value || typeof value !== 'object' || Array.isArray(value)) throw new Error(`${label} must be an object.`);
  return value as Record<string, unknown>;
};
const text = (value: unknown, label: string): string => {
  if (typeof value !== 'string') throw new Error(`${label} must be text.`);
  return value;
};

/** Split at the adapter boundary; retained canonical source metadata is copied whole. */
export function projectModel(project: DomainProject): ModelProjection {
  const checked = assertProject(project);
  const layout: LayoutMap = Object.fromEntries([
    ...checked.contexts.map(({ id, position, color }) => [id, { position: clone(position), color }] as const),
    ...checked.concepts.map(({ id, position }) => [id, { position: clone(position) }] as const),
  ]);
  const model: SemanticProject = {
    ...checked,
    contexts: checked.contexts.map(({ position: _position, color: _color, ...context }) => context),
    concepts: checked.concepts.map(({ position: _position, ...concept }) => concept),
  };
  return { model, layout };
}

/** Existing renderers and serializers receive a fresh DomainProject adapter. */
export function assembleProject(projection: ModelProjection): DomainProject {
  const model = mapping(projection.model, 'Semantic model');
  const layout = mapping(projection.layout, 'Layout');
  if (!Array.isArray(model.contexts) || !Array.isArray(model.concepts)) throw new Error('Semantic contexts and concepts must be arrays.');
  const presentation = (id: unknown): Record<string, unknown> => {
    if (typeof id !== 'string' || !Object.hasOwn(layout, id)) throw new Error(`Layout is missing subject ${String(id)}.`);
    return mapping(layout[id], 'Subject layout');
  };
  return assertProject({
    ...model,
    contexts: model.contexts.map(value => { const c = mapping(value, 'Semantic context'); const p = presentation(c.id); return { ...c, position: p.position, color: p.color }; }),
    concepts: model.concepts.map(value => { const c = mapping(value, 'Semantic concept'); return { ...c, position: presentation(c.id).position }; }),
  });
}

function checkedPatterns(value: unknown = []): PatternSummary[] {
  if (!Array.isArray(value)) throw new Error('Pattern references must be an array.');
  for (const raw of value) {
    const pattern = mapping(raw, 'Pattern reference');
    text(pattern.name, 'Pattern name'); text(pattern.description, 'Pattern description');
    if (!Array.isArray(pattern.rules)) throw new Error('Pattern Rules must be an array.');
    for (const entry of pattern.rules) {
      const rule = mapping(entry, 'Pattern Rule'); text(rule.id, 'Rule ID'); text(rule.description, 'Rule description');
    }
  }
  return clone(value as PatternSummary[]);
}
function checkedView(value: unknown = {}, project: DomainProject): WorkspaceView {
  const raw = mapping(value, 'Workspace view');
  const view = { ...defaultView(), ...raw };
  for (const key of ['selectedId', 'scopeId', 'aggregateId'] as const) if (view[key] !== null && typeof view[key] !== 'string') throw new Error(`${key} must be a subject ID or null.`);
  if (view.representation !== 'spatial' && view.representation !== 'table') throw new Error('Unknown workspace representation.');
  if (view.lens !== 'meaning' && view.lens !== 'governance') throw new Error('Unknown workspace task.');
  if (!Number.isSafeInteger(view.page) || view.page < 0) throw new Error('View page must be a nonnegative integer.');
  const nodes = [...project.contexts, ...project.concepts, ...project.relationships];
  if (view.selectedId && !nodes.some(item => item.id === view.selectedId)) view.selectedId = null;
  if (view.scopeId && !project.contexts.some(item => item.id === view.scopeId)) view.scopeId = null;
  if (view.aggregateId && !project.concepts.some(item => item.id === view.aggregateId && item.kind === 'aggregate')) view.aggregateId = null;
  return { selectedId: view.selectedId, scopeId: view.scopeId, aggregateId: view.aggregateId, page: view.page, representation: view.representation, lens: view.lens };
}
function checkedNotebook(value: unknown = []): NotebookDraft[] {
  if (!Array.isArray(value)) throw new Error('Notebook drafts must be an array.');
  const drafts = value.map(entry => {
    const d = mapping(entry, 'Notebook draft');
    return { id: text(d.id, 'Draft ID'), text: text(d.text, 'Draft text'), createdAt: text(d.createdAt, 'Draft creation time'), updatedAt: text(d.updatedAt, 'Draft update time') };
  });
  if (new Set(drafts.map(d => d.id)).size !== drafts.length) throw new Error('Notebook draft IDs must be unique.');
  return drafts;
}
function checkedFields(value: unknown): Record<string, string> {
  return Object.fromEntries(Object.entries(mapping(value, 'Editor fields')).map(([key, value]) => [key, text(value, `Editor field ${key}`)]));
}
function checkedEditorDrafts(value: unknown = {}): Record<string, EditorDraft> {
  return Object.fromEntries(Object.entries(mapping(value, 'Editor drafts')).map(([key, entry]) => {
    const draft = mapping(entry, 'Editor draft');
    if (draft.subjectId !== key) throw new Error('Editor draft key must match its subject.');
    return [key, { subjectId: key, fields: checkedFields(draft.fields), updatedAt: text(draft.updatedAt, 'Editor draft update time') }];
  }));
}
/** Older adapters retained these contracts only in the canonical source document. */
function upgradeContractProjection(project: DomainProject): DomainProject {
  const next = clone(project);
  for (const concept of next.concepts) {
    const source = sourceTerm(next, concept.id);
    // An explicit array, including [], is an authored edit and takes precedence.
    if (concept.kind === 'aggregate' && concept.assertions === undefined && source.assertions !== undefined) {
      concept.assertions = Object.entries(mapping(source.assertions, `${concept.name} assertions`)).map(([key, value]) => {
        const assertion = mapping(value, `${concept.name} assertion ${key}`);
        return { key, on: text(assertion.on, 'Assertion operation'), statement: text(assertion.statement, 'Assertion statement') };
      });
    }
    if (concept.kind === 'domain_event' && concept.raisedBy === undefined && source.raised_by !== undefined) concept.raisedBy = text(source.raised_by, 'Event source');
  }
  return assertProject(next);
}
function storeSnapshot(value: unknown): StoredModelSnapshot | null {
  if (value === null || value === undefined) return null;
  const raw = mapping(value, 'Model snapshot');
  const project = 'project' in raw ? assertProject(raw.project) : assembleProject(raw as unknown as ModelProjection);
  return { name: text(raw.name, 'Model snapshot name'), capturedAt: text(raw.capturedAt, 'Model snapshot capture time'), ...projectModel(upgradeContractProjection(project)) };
}

/** Accepts the previous browser envelope without renaming or truncating its canonical library. */
export function migrateWorkspace(saved?: unknown, initial: DomainProject = createEmptyProject()): WorkspaceEnvelope {
  if (typeof saved === 'string') saved = JSON.parse(saved);
  if (saved === null || saved === undefined) return { version: 2, ...projectModel(upgradeContractProjection(initial)), modelSnapshot: null, patterns: [], view: defaultView(), notebook: [], editorDrafts: {} };
  const raw = mapping(saved, 'Saved workspace');
  if (raw.version === 2) {
    const project = upgradeContractProjection(assembleProject(raw as unknown as ModelProjection));
    return { version: 2, ...projectModel(project), modelSnapshot: storeSnapshot(raw.modelSnapshot), patterns: checkedPatterns(raw.patterns), view: checkedView(raw.view, project), notebook: checkedNotebook(raw.notebook), editorDrafts: checkedEditorDrafts(raw.editorDrafts) };
  }
  if (raw.version !== undefined && raw.version !== 1) throw new Error('Unsupported workspace version.');
  const project = upgradeContractProjection(assertProject(raw.project ?? raw));
  return { version: 2, ...projectModel(project), modelSnapshot: storeSnapshot(raw.baseline ?? raw.modelSnapshot), patterns: checkedPatterns(raw.patterns), view: checkedView(raw.view, project), notebook: checkedNotebook(raw.notebook), editorDrafts: checkedEditorDrafts(raw.editorDrafts) };
}

type WorkspaceContent = Pick<WorkspaceEnvelope, 'model' | 'layout' | 'modelSnapshot' | 'patterns' | 'notebook'>;
type ReplacementState = Pick<WorkspaceEnvelope, 'editorDrafts'> & Partial<Pick<WorkspaceEnvelope, 'view'>>;
interface EditorDraftChange { subjectId: string; before: EditorDraft | null; after: EditorDraft | null }
interface HistoryEntry { command: WorkspaceCommand; before: WorkspaceContent; after: WorkspaceContent; beforeReplacement?: ReplacementState; afterReplacement?: ReplacementState; editorDraftChange?: EditorDraftChange }
const content = ({ model, layout, modelSnapshot, patterns, notebook }: WorkspaceEnvelope): WorkspaceContent => clone({ model, layout, modelSnapshot, patterns, notebook });

/** One command/state owner shared by the spatial and conventional renderers. */
export function createWorkspace(saved?: unknown, initial?: DomainProject): Workspace {
  let state = migrateWorkspace(saved, initial);
  const past: HistoryEntry[] = [];
  const future: HistoryEntry[] = [];
  const currentProject = () => assembleProject(state);
  const record = (kind: WorkspaceCommandKind, label: string, change: (next: WorkspaceEnvelope) => void, subjectId?: string, replacement?: 'drafts' | 'workspace', editorDraftId?: string): void => {
    const next = clone(state);
    change(next);
    const before = content(state), after = content(next);
    const replacementState = (value: WorkspaceEnvelope): ReplacementState => clone({ editorDrafts: value.editorDrafts, ...(replacement === 'workspace' ? { view: value.view } : {}) });
    const sidecars = replacement ? { beforeReplacement: replacementState(state), afterReplacement: replacementState(next) } : {};
    const editorDraftChange = editorDraftId ? { subjectId: editorDraftId, before: clone(Object.hasOwn(state.editorDrafts, editorDraftId) ? state.editorDrafts[editorDraftId] : null), after: clone(Object.hasOwn(next.editorDrafts, editorDraftId) ? next.editorDrafts[editorDraftId] : null) } : undefined;
    if (same(before, after) && (!replacement || same(sidecars.beforeReplacement, sidecars.afterReplacement)) && (!editorDraftChange || same(editorDraftChange.before, editorDraftChange.after))) return;
    const command: WorkspaceCommand = { id: makeId('command'), kind, label, createdAt: new Date().toISOString(), ...(subjectId ? { subjectId } : {}) };
    const previous = past.at(-1);
    // Consecutive typing belongs to one notebook edit, not one command per keystroke.
    if (!future.length && kind === 'notebook.write' && previous?.command.kind === kind && previous.command.subjectId === subjectId) {
      previous.after = after;
    } else {
      past.push({ command, before, after, ...sidecars, ...(editorDraftChange ? { editorDraftChange } : {}) });
      if (past.length > 60) past.shift();
    }
    future.length = 0;
    state = next;
    state.view = checkedView(state.view, currentProject());
  };
  const restoreEditorChange = (change: EditorDraftChange | undefined, direction: 'before' | 'after') => {
    if (!change) return;
    const entries = Object.entries(state.editorDrafts).filter(([id]) => id !== change.subjectId);
    const value = change[direction];
    state.editorDrafts = Object.fromEntries(value ? [...entries, [change.subjectId, clone(value)]] : entries);
  };
  return {
    get project() { return currentProject(); },
    get model() { return clone(state.model); },
    get layout() { return clone(state.layout); },
    get modelSnapshot() { return state.modelSnapshot ? { name: state.modelSnapshot.name, capturedAt: state.modelSnapshot.capturedAt, project: assembleProject(state.modelSnapshot) } : null; },
    get patterns() { return clone(state.patterns); },
    get view() { return clone(state.view); },
    get notebook() { return clone(state.notebook); },
    get canUndo() { return past.length > 0; },
    get canRedo() { return future.length > 0; },
    get history() { return past.map(entry => clone(entry.command)); },
    apply(label, mutation, options = {}) {
      const draft = currentProject();
      const projection = projectModel(mutation(draft) ?? draft);
      const kind = same(state.model, projection.model) ? 'layout.edit' : 'model.edit';
      record(kind, label, next => {
        Object.assign(next, projection);
        if (options.consumeDraftId) next.notebook = next.notebook.filter(entry => entry.id !== options.consumeDraftId);
        if (options.clearEditorDraftId) delete next.editorDrafts[options.clearEditorDraftId];
      }, undefined, undefined, options.clearEditorDraftId);
    },
    replace(project, label = 'Replace working model') {
      const projection = projectModel(upgradeContractProjection(project));
      record('workspace.replace', label, next => {
        Object.assign(next, projection); next.modelSnapshot = null; next.patterns = [];
        next.view = { ...next.view, selectedId: null, scopeId: null, aggregateId: null, page: 0 };
        next.editorDrafts = {};
      }, undefined, 'drafts');
    },
    restore(saved, label = 'Restore workspace') {
      // Validate before touching live state or history. Unlike constructing a new
      // store, importing a workspace remains recoverable through the same Undo.
      const imported = migrateWorkspace(saved);
      record('workspace.restore', label, next => Object.assign(next, imported), undefined, 'workspace');
    },
    updateView(patch) { state.view = checkedView({ ...state.view, ...patch }, currentProject()); },
    setModelSnapshot(snapshot) { const stored = storeSnapshot(snapshot); record('snapshot.capture', snapshot ? `Capture model snapshot: ${snapshot.name}` : 'Clear model snapshot', next => { next.modelSnapshot = stored; }); },
    setPatterns(patterns) { const checked = checkedPatterns(patterns); record('patterns.change', 'Change Pattern references', next => { next.patterns = checked; }); },
    saveDraft(value, id) {
      text(value, 'Draft text');
      const previous = id ? state.notebook.find(draft => draft.id === id) : undefined;
      const now = new Date().toISOString();
      const draft = { id: id ?? makeId('draft'), text: value, createdAt: previous?.createdAt ?? now, updatedAt: previous?.text === value ? previous.updatedAt : now };
      record('notebook.write', 'Write notebook draft', next => { const index = next.notebook.findIndex(item => item.id === draft.id); if (index < 0) next.notebook.push(draft); else next.notebook[index] = draft; }, draft.id);
      return clone(draft);
    },
    removeDraft(id) { record('notebook.remove', 'Remove notebook draft', next => { next.notebook = next.notebook.filter(draft => draft.id !== id); }, id); },
    saveEditorDraft(subjectId, fields) {
      const draft = { subjectId: text(subjectId, 'Editor subject'), fields: checkedFields(fields), updatedAt: new Date().toISOString() };
      // Object.fromEntries also supports legitimate IDs such as "__proto__" safely.
      state.editorDrafts = Object.fromEntries([...Object.entries(state.editorDrafts).filter(([key]) => key !== subjectId), [subjectId, draft]]);
      return clone(draft);
    },
    getEditorDraft(subjectId) { return Object.hasOwn(state.editorDrafts, subjectId) ? clone(state.editorDrafts[subjectId]) : null; },
    removeEditorDraft(subjectId) { delete state.editorDrafts[subjectId]; },
    undo() { const entry = past.pop(); if (!entry) return false; future.push(entry); state = { ...state, ...clone(entry.before), ...clone(entry.beforeReplacement ?? {}) }; restoreEditorChange(entry.editorDraftChange, 'before'); state.view = checkedView(state.view, currentProject()); return true; },
    redo() { const entry = future.pop(); if (!entry) return false; past.push(entry); state = { ...state, ...clone(entry.after), ...clone(entry.afterReplacement ?? {}) }; restoreEditorChange(entry.editorDraftChange, 'after'); state.view = checkedView(state.view, currentProject()); return true; },
    snapshot() { return clone(state); },
    serialize() { return JSON.stringify(state); },
  };
}
