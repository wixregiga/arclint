import type { Concept, DomainProject, Relationship } from './contracts';

export const kindNames: Record<Concept['kind'], string> = {
  unclassified: 'Unclassified', aggregate: 'Aggregate', entity: 'Entity', value_object: 'Value object',
  domain_event: 'Domain event', domain_service: 'Domain service', specification: 'Specification', repository: 'Repository', factory: 'Factory',
};
export type RecordedContract = { key: string; statement: string };
export type OperationAssertion = RecordedContract & { operation: string };
export function record(value: unknown): Record<string, unknown> {
  return value && typeof value === 'object' && !Array.isArray(value) ? value as Record<string, unknown> : {};
}
export function sourceTerm(project: DomainProject, id: string): Record<string, unknown> {
  if (!id.startsWith('yaml:')) return {};
  try {
    let value: unknown = project.sourceDocument;
    for (const segment of id.slice(5).split('/').map(decodeURIComponent)) {
      if (!Object.hasOwn(record(value), segment)) return {};
      value = record(value)[segment];
    }
    return record(value);
  } catch { return {}; }
}
/** Display projections preserve the difference between always-held invariants and operation post-conditions. */
export function conceptContracts(project: DomainProject, concept: Concept): { invariants: RecordedContract[]; assertions: OperationAssertion[] } {
  const source = sourceTerm(project, concept.id);
  const previous = Object.entries(record(source.invariants));
  const used = new Set<string>();
  const invariants = concept.invariants.map((statement, index) => {
    const key = previous.find(([key, value]) => value === statement && !used.has(key))?.[0] ?? `draft-${index + 1}`;
    used.add(key); return { key, statement };
  });
  const assertions = Object.entries(record(source.assertions)).flatMap(([key, value]) => {
    const entry = record(value);
    return typeof entry.on === 'string' && typeof entry.statement === 'string' ? [{ key, operation: entry.on, statement: entry.statement }] : [];
  });
  return { invariants, assertions };
}
export function relationshipDescription(project: DomainProject, relationship: Relationship): { label: string; meaning: string; direction: 'forward' | 'both' | 'none' } {
  const name = (id: string) => [...project.contexts, ...project.concepts].find(item => item.id === id)?.name ?? id;
  const source = name(relationship.source), target = name(relationship.target);
  const recordedRelations = project.sourceDocument?.relations;
  const canonical = relationship.id.startsWith('yaml:relations/') && Array.isArray(recordedRelations) && record(recordedRelations[Number(relationship.id.split('/').at(-1))]).kind === relationship.label && project.contexts.some(c => c.id === relationship.source) && project.contexts.some(c => c.id === relationship.target);
  if (canonical && ['partnership', 'shared_kernel'].includes(relationship.label)) return { label: relationship.label.replaceAll('_', ' '), meaning: `${source} ↔ ${target}: influence runs both ways.`, direction: 'both' };
  if (canonical && relationship.label === 'separate_ways') return { label: 'separate ways', meaning: `${source} and ${target} have no recorded connection.`, direction: 'none' };
  if (canonical) return { label: relationship.label.replaceAll('_', ' '), meaning: `${source} → ${target}: upstream model influences downstream (${relationship.label.replaceAll('_', ' ')}). This arrow does not show imports.`, direction: 'forward' };
  return { label: relationship.label, meaning: `${source} → ${target}: ${relationship.label}.`, direction: 'forward' };
}

/** Switch representation without altering the saved coordinates or deciding a domain kind. */
export function planPosition(position: [number, number, number]): { x: number; y: number } {
  return { x: position[0] * 28, y: position[2] * 28 };
}
