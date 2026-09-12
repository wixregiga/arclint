import type { Change, ConceptKind, DomainProject, Finding } from './contracts';

const kinds: ConceptKind[] = ['unclassified', 'aggregate', 'entity', 'value_object', 'domain_event', 'domain_service', 'specification', 'repository', 'factory'];
export function makeId(prefix = 'node'): string {
  return `${prefix}-${globalThis.crypto?.randomUUID?.() ?? `${Date.now().toString(36)}-${Math.random().toString(36).slice(2)}`}`;
}
export function cloneProject(project: DomainProject): DomainProject { return structuredClone(project); }
export function createEmptyProject(): DomainProject {
  return { version: 1, name: 'Untitled domain', description: '', contexts: [], concepts: [], relationships: [] };
}
export function createExampleProject(): DomainProject {
  const project: DomainProject = {
    version: 1, name: 'bskilled · exploration',
    description: 'An illustrative draft based on your skills-manager idea. Context boundaries, relationships, and concept classifications are proposals, not a settled model. Replace this example with any domain.',
    contexts: [
      { id: 'context-content', name: 'content', description: 'Proposed area for the skills, plugins, and commands people manage. This boundary needs confirmation.', color: '#66d8df', position: [-17, 0, -6] },
      { id: 'context-distribution', name: 'distribution', description: 'Proposed area for installing the same skills across harnesses and packaging them for desktop clients. This boundary needs confirmation.', color: '#f6b85c', position: [13, 0, -5] },
      { id: 'context-activation', name: 'activation', description: 'Proposed area for turning installed skills on and off. This boundary needs confirmation.', color: '#b1a1ee', position: [1, 0, 15] },
    ], concepts: [], relationships: [],
  };
  const entries: [string, string, string, string, [number, number, number]][] = [
    ['skill', 'Skill', 'content', 'A skill people manage and install across different harnesses. Its identity and classification remain open.', [-23, 2, -10]],
    ['plugin', 'Plugin', 'content', 'Something people can install alongside skills. Its relationship to skills and commands needs confirmation.', [-12, 3, -11]],
    ['command', 'Command', 'content', 'A command people can install and manage. Its identity and ownership remain open.', [-18, 1, 0]],
    ['harness', 'Harness', 'distribution', 'An environment to which people install skills. Its identity and classification remain open.', [8, 2, -10]],
    ['package', 'Package', 'distribution', 'A portable grouping prepared so skills can be uploaded to desktop clients. Its contents and lifecycle remain open.', [19, 4, -8]],
    ['installation', 'Installation', 'distribution', 'A candidate term for placing a skill into a harness. Whether this is a lasting record or an operation remains open.', [14, 1, 2]],
    ['enablement', 'Enablement', 'activation', 'A candidate term for a skill being turned on or off. Whether this belongs to an installation remains open.', [-5, 2, 15]],
    ['selection', 'Selection', 'activation', 'A candidate term for the skills a person chooses to use. Whether a separate concept is needed remains open.', [6, 3, 12]],
    ['client', 'DesktopClient', 'activation', 'A desktop client that can receive a packaged skill. Its placement in this context is provisional.', [3, 1, 22]],
  ];
  project.concepts = entries.map(([id, name, context, definition, position]) => ({ id, name, contextId: `context-${context}`, definition, kind: 'unclassified', invariants: [], position }));
  project.relationships = [
    { id: 'rel-1', source: 'installation', target: 'skill', label: 'installs (proposed)' },
    { id: 'rel-2', source: 'installation', target: 'harness', label: 'into (proposed)' },
    { id: 'rel-3', source: 'package', target: 'skill', label: 'carries (proposed)' },
    { id: 'rel-4', source: 'enablement', target: 'installation', label: 'controls (open question)' },
    { id: 'rel-5', source: 'package', target: 'client', label: 'uploaded to (proposed)' },
    { id: 'rel-6', source: 'selection', target: 'skill', label: 'chooses (proposed)' },
  ];
  return project;
}

function object(value: unknown, label: string): Record<string, unknown> {
  if (!value || typeof value !== 'object' || Array.isArray(value)) throw new Error(`${label} must be an object.`);
  return value as Record<string, unknown>;
}
function string(value: unknown, label: string, allowEmpty = false): string {
  if (typeof value !== 'string' || (!allowEmpty && !value.trim()) || value.length > 100_000) throw new Error(`${label} must be ${allowEmpty ? 'a' : 'a nonempty'} string (at most 100,000 characters).`);
  return value;
}
function position(value: unknown): [number, number, number] {
  if (!Array.isArray(value) || value.length !== 3 || value.some(n => typeof n !== 'number' || !Number.isFinite(n) || Math.abs(n) > 100_000)) throw new Error('Positions must contain three finite coordinates between -100000 and 100000.');
  return [...value] as [number, number, number];
}
function array(value: unknown, label: string): unknown[] {
  if (!Array.isArray(value) || value.length > 10_000) throw new Error(`${label} must be an array with at most 10,000 entries.`);
  return value;
}
/** Validate the persisted shape and copy only supported fields. Draft meanings may remain incomplete. */
export function assertProject(value: unknown): DomainProject {
  const raw = object(value, 'Project');
  if (raw.version !== 1) throw new Error('Unsupported project version; expected version 1.');
  const project: DomainProject = {
    version: 1, name: string(raw.name, 'Project name'), description: string(raw.description, 'Project description', true),
    contexts: array(raw.contexts, 'Contexts').map((entry) => {
      const c = object(entry, 'Context');
      const color = string(c.color, 'Context color');
      if (!/^#[\da-f]{6}$/i.test(color)) throw new Error('Context color must be a six-digit hex color.');
      return { id: string(c.id, 'Context ID'), name: string(c.name, 'Context name', true), description: string(c.description, 'Context description', true), color, position: position(c.position) };
    }),
    concepts: array(raw.concepts, 'Concepts').map(entry => {
      const c = object(entry, 'Concept');
      if (!kinds.includes(c.kind as ConceptKind)) throw new Error(`Unknown concept kind: ${String(c.kind)}.`);
      const identity = c.identity === undefined ? {} : { identity: string(c.identity, 'Concept identity', true) };
      const owner = c.ownerId === undefined ? {} : { ownerId: string(c.ownerId, 'Aggregate owner ID', true) };
      const aliases = c.aliases === undefined ? {} : { aliases: array(c.aliases, 'Aliases').map(v => string(v, 'Alias')) };
      if (aliases.aliases && new Set(aliases.aliases).size !== aliases.aliases.length) throw new Error('Aliases must be unique.');
      return { ...identity, ...owner, ...aliases, id: string(c.id, 'Concept ID'), name: string(c.name, 'Concept name', true), definition: string(c.definition, 'Concept definition', true), contextId: string(c.contextId, 'Concept context'), kind: c.kind as ConceptKind, position: position(c.position), invariants: array(c.invariants, 'Invariants').map(v => string(v, 'Invariant', true)) };
    }),
    relationships: array(raw.relationships, 'Relationships').map(entry => {
      const r = object(entry, 'Relationship');
      return { id: string(r.id, 'Relationship ID'), source: string(r.source, 'Relationship source'), target: string(r.target, 'Relationship target'), label: string(r.label, 'Relationship label', true) };
    }),
  };
  const ids = new Set<string>();
  for (const item of [...project.contexts, ...project.concepts, ...project.relationships]) {
    if (ids.has(item.id)) throw new Error(`Duplicate ID: ${item.id}.`);
    ids.add(item.id);
  }
  const contexts = new Set(project.contexts.map(c => c.id));
  for (const c of project.concepts) if (!contexts.has(c.contextId)) throw new Error(`Concept ${c.name} references a missing context.`);
  for (const c of project.concepts) if (c.ownerId) {
    const owner = project.concepts.find(candidate => candidate.id === c.ownerId);
    if (!owner || owner.kind !== 'aggregate' || owner.contextId !== c.contextId || owner.id === c.id) throw new Error(`Concept ${c.name} references a missing or incompatible aggregate owner.`);
  }
  const nodes = new Set([...project.contexts, ...project.concepts].map(c => c.id));
  for (const r of project.relationships) if (!nodes.has(r.source) || !nodes.has(r.target)) throw new Error(`Relationship ${r.label || r.id} references a missing endpoint.`);
  if (raw.sourceDocument !== undefined) {
    object(raw.sourceDocument, 'Source document');
    // JSON serialization also rejects cycles and isolates imported data from live state.
    project.sourceDocument = JSON.parse(JSON.stringify(raw.sourceDocument));
  }
  return project;
}

export function validateProject(project: DomainProject): Finding[] {
  const findings: Finding[] = [];
  const add = (subjectId: string, title: string, message: string, severity: Finding['severity'] = 'warning') => findings.push({ id: `${subjectId}-${title}`, subjectId, title, message, severity });
  const names = new Set<string>();
  for (const context of project.contexts) {
    if (!context.name.trim()) add(context.id, 'Context needs a name', 'Name this bounded context.', 'error');
    if (!context.description.trim()) add(context.id, 'Boundary is undefined', 'Describe what this context owns and where its model applies.');
    if (names.has(context.name.trim().toLowerCase())) add(context.id, 'Repeated context name', 'Use distinct context names.', 'error');
    names.add(context.name.trim().toLowerCase());
  }
  const conceptNames = new Set<string>();
  for (const concept of project.concepts) {
    if (!concept.name.trim()) add(concept.id, 'Concept needs a name', 'Give this concept a name from the domain.', 'error');
    if (!concept.definition.trim()) add(concept.id, 'Meaning is undefined', 'Describe what this concept means.');
    if (concept.kind === 'unclassified') add(concept.id, 'Classification is open', 'Confirm identity, value equality, and ownership before choosing a domain kind.');
    if (concept.kind === 'aggregate' && !concept.identity?.trim()) add(concept.id, 'Aggregate identity is open', 'Name the value object carrying this aggregate root’s identity before canonical YAML export.');
    if (['entity', 'repository', 'factory'].includes(concept.kind) && !concept.ownerId) add(concept.id, 'Aggregate owner is open', 'Choose the aggregate that owns this member before canonical YAML export.');
    if (concept.ownerId && !project.concepts.some(owner => owner.id === concept.ownerId && owner.kind === 'aggregate' && owner.contextId === concept.contextId)) add(concept.id, 'Aggregate owner is invalid', 'Choose an aggregate in this bounded context.', 'error');
    if (concept.kind === 'aggregate' && !concept.invariants.some(v => v.trim())) add(concept.id, 'Consistency boundary is open', 'Record what must remain consistent inside this aggregate.');
    const key = `${concept.contextId}:${concept.name.trim().toLowerCase()}`;
    if (conceptNames.has(key)) add(concept.id, 'Repeated term', 'A context should define one meaning for a term.', 'error');
    conceptNames.add(key);
    if (!project.contexts.some(c => c.id === concept.contextId)) add(concept.id, 'Missing context', 'Place this concept inside an existing context.', 'error');
  }
  for (const c of project.concepts) if (c.ownerId) {
    const owner = project.concepts.find(candidate => candidate.id === c.ownerId);
    if (!owner || owner.kind !== 'aggregate' || owner.contextId !== c.contextId || owner.id === c.id) throw new Error(`Concept ${c.name} references a missing or incompatible aggregate owner.`);
  }
  const nodes = new Set([...project.contexts, ...project.concepts].map(c => c.id));
  for (const relationship of project.relationships) {
    if (!relationship.label.trim()) add(relationship.id, 'Relationship is unnamed', 'Explain what this connection means.');
    if (!nodes.has(relationship.source) || !nodes.has(relationship.target)) add(relationship.id, 'Missing endpoint', 'Reconnect or remove this relationship.', 'error');
  }
  return findings;
}

export function diffProjects(baseline: DomainProject, current: DomainProject): Change[] {
  const changes: Change[] = [];
  for (const [collection, subject] of [['contexts', 'context'], ['concepts', 'concept'], ['relationships', 'relationship']] as const) {
    const before = new Map(baseline[collection].map(item => [item.id, item]));
    const after = new Map(current[collection].map(item => [item.id, item]));
    for (const [id, item] of after) {
      const previous = before.get(id);
      const name = 'name' in item ? item.name : item.label;
      if (!previous) changes.push({ id, subject, name, type: 'added', detail: 'Added since the baseline.' });
      else {
        const fields = Object.keys(item).filter(key => JSON.stringify((item as unknown as Record<string, unknown>)[key]) !== JSON.stringify((previous as unknown as Record<string, unknown>)[key]));
        if (fields.length) changes.push({ id, subject, name, type: 'changed', detail: fields.length === 1 && fields[0] === 'position' ? 'Position changed; domain meaning unchanged.' : `Changed ${fields.join(', ')}.` });
      }
    }
    for (const [id, item] of before) if (!after.has(id)) changes.push({ id, subject, name: 'name' in item ? item.name : item.label, type: 'removed', detail: 'Removed since the baseline.' });
  }
  return changes;
}
