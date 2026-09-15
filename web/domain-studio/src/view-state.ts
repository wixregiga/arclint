import type { Concept, DomainContext, DomainProject } from './contracts';

export type ViewLevel = 'world' | 'context' | 'detail';
export type ViewLens = 'meaning' | 'structure' | 'inspection' | 'governance';
export interface FocusView {
  level: ViewLevel;
  /** The actual active context, retained when inspecting one of its external connections. */
  contextId: string | null;
  /** The selected concept's recorded home, which may differ from the active context. */
  subjectContextId: string | null;
  selectedId: string | null;
  /** A concept retained on every detail page; relationship details frame endpoints instead. */
  anchorId: string | null;
  /** Zero-based and clamped. An empty view has one empty page. */
  page: number;
  ids: string[];
  /** Eligible subjects across all pages, including the retained detail anchor. */
  total: number;
  pages: number;
  relatedCount: number;
  visibleRelatedCount: number;
  externalIds: string[];
}
export interface ViewRequest { scopeId?: string | null; selectedId?: string | null; page?: number }
export interface KeeperGuide { name: string; role: string; prompt: string; action: string }
export const MAX_VIEW_SUBJECTS = 8;
type Subject = Concept | DomainContext;

function paginate(ids: string[], requested: number | undefined, size = MAX_VIEW_SUBJECTS) {
  const pages = Math.max(1, Math.ceil(ids.length / size));
  const page = Math.max(0, Math.min(pages - 1, typeof requested === 'number' && Number.isFinite(requested) ? Math.floor(requested) : 0));
  return { ids: ids.slice(page * size, (page + 1) * size), pages, page };
}
function emptyView(level: ViewLevel, contextId: string | null, selectedId: string | null): FocusView {
  return { level, contextId, subjectContextId: null, selectedId, anchorId: null, page: 0, ids: [], total: 0, pages: 1, relatedCount: 0, visibleRelatedCount: 0, externalIds: [] };
}
/**
 * A bounded view of recorded objects, not a new domain hierarchy. Positions, names,
 * classifications, relationships and collection order are never changed here.
 * Offstage subjects remain reachable through paging or selecting them directly.
 */
export function projectView(project: DomainProject, request: ViewRequest = {}): FocusView {
  const contexts = new Map(project.contexts.map(context => [context.id, context]));
  const concepts = new Map(project.concepts.map(concept => [concept.id, concept]));
  const subjects = new Map<string, Subject>([...contexts, ...concepts]);
  const activeContext = request.scopeId && contexts.has(request.scopeId) ? request.scopeId : null;
  const selectedContext = request.selectedId ? contexts.get(request.selectedId) : undefined;
  const selectedConcept = request.selectedId ? concepts.get(request.selectedId) : undefined;
  const selectedRelationship = request.selectedId ? project.relationships.find(relationship => relationship.id === request.selectedId) : undefined;

  if (selectedConcept && contexts.has(selectedConcept.contextId)) {
    const contextId = activeContext ?? selectedConcept.contextId;
    const connected = new Set<string>();
    for (const relationship of project.relationships) {
      if (relationship.source === selectedConcept.id) connected.add(relationship.target);
      if (relationship.target === selectedConcept.id) connected.add(relationship.source);
    }
    if (selectedConcept.ownerId) connected.add(selectedConcept.ownerId);
    for (const concept of project.concepts) if (concept.ownerId === selectedConcept.id) connected.add(concept.id);
    connected.delete(selectedConcept.id);
    // Ancestors are ground and breadcrumb, not duplicate domain actors.
    connected.delete(contextId);
    connected.delete(selectedConcept.contextId);
    const candidates = [...project.concepts, ...project.contexts].filter(subject => connected.has(subject.id));
    const sameContext = (subject: Subject) => 'contextId' in subject && subject.contextId === selectedConcept.contextId;
    const related = [...candidates.filter(sameContext), ...candidates.filter(subject => !sameContext(subject))];
    const page = paginate(related.map(subject => subject.id), request.page, MAX_VIEW_SUBJECTS - 1);
    const ids = [selectedConcept.id, ...page.ids];
    return {
      ...emptyView('detail', contextId, selectedConcept.id), ...page, ids,
      subjectContextId: selectedConcept.contextId, anchorId: selectedConcept.id,
      total: related.length + 1, relatedCount: related.length, visibleRelatedCount: page.ids.length,
      externalIds: ids.filter(id => { const subject = subjects.get(id)!; return 'contextId' in subject ? subject.contextId !== contextId : subject.id !== contextId; }),
    };
  }

  if (selectedRelationship) {
    // No invented relationship-node or graph expansion: show the exact endpoints.
    const ids = [...new Set([selectedRelationship.source, selectedRelationship.target])].filter(id => subjects.has(id));
    if (ids.length) {
      const endpointConcept = ids.map(id => concepts.get(id)).find((concept): concept is Concept => !!concept && contexts.has(concept.contextId));
      const contextId = activeContext ?? endpointConcept?.contextId ?? null;
      return {
        ...emptyView('detail', contextId, selectedRelationship.id), ids, total: ids.length,
        subjectContextId: endpointConcept?.contextId ?? null, relatedCount: ids.length, visibleRelatedCount: ids.length,
        externalIds: contextId ? ids.filter(id => { const subject = subjects.get(id)!; return 'contextId' in subject ? subject.contextId !== contextId : subject.id !== contextId; }) : [],
      };
    }
  }

  const contextId = selectedContext?.id ?? activeContext;
  if (contextId) {
    const ids = project.concepts.filter(concept => concept.contextId === contextId).map(concept => concept.id);
    return { ...emptyView('context', contextId, selectedContext?.id ?? null), ...paginate(ids, request.page), total: ids.length };
  }
  const ids = project.contexts.map(context => context.id);
  return { ...emptyView('world', null, null), ...paginate(ids, request.page), total: ids.length };
}

/** These are interface-guide roles, never inferred domain actors or AI agents. */
export function keeperFor(view: FocusView, lens: ViewLens): KeeperGuide {
  if (view.level === 'world') return {
    name: 'Onyx', role: 'Project guide · interface role',
    prompt: lens === 'meaning' ? 'Choose a context to explore what this domain means, or add the first boundary.' : 'Inspect the repository’s declared architecture before judging its code.',
    action: lens === 'meaning' ? 'Find a context' : 'Inspect repository',
  };
  if (view.level === 'context') return {
    name: 'Onyx', role: 'Context guide · interface role',
    prompt: lens === 'meaning' ? 'Choose a domain entry to examine its meaning and recorded relationships.' : 'Select a recorded source anchor or Zone to request the real ArcLint context.',
    action: lens === 'meaning' ? 'Edit this context' : 'Inspect context',
  };
  return {
    name: 'Onyx', role: 'Detail guide · interface role',
    prompt: lens === 'meaning' ? 'Review this subject’s definition, ownership, and contracts before changing them.' : 'Inspect available source evidence. A browser draft is not the code checked on disk.',
    action: lens === 'meaning' ? 'Review meaning' : 'Inspect evidence',
  };
}
