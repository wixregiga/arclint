import type { ConceptKind, DomainProject } from './contracts';

export const DOMAIN_KINDS: Record<ConceptKind, string> = {
  unclassified: 'Open question', aggregate: 'Aggregate', entity: 'Entity', value_object: 'Value object',
  domain_event: 'Domain event', domain_service: 'Domain service', specification: 'Specification', repository: 'Repository', factory: 'Factory',
};
export const escapeHtml = (value: unknown) => String(value ?? '').replace(/[&<>"']/g, c => ({'&':'&amp;','<':'&lt;','>':'&gt;','"':'&quot;',"'":'&#39;'})[c]!);

/** An interface avatar, never a record or simulated inhabitant of the domain. */
export function onyxAvatar() {
  return `<svg viewBox="0 0 100 100" aria-hidden="true" class="onyx-avatar">
    <ellipse cx="50" cy="91" rx="29" ry="4" fill="#233e4b" opacity=".13"/>
    <path d="M31 54 Q21 70 25 88 L75 88 Q78 68 66 53Z" fill="#233330"/>
    <path d="M29 30 Q39 20 52 23 Q66 20 74 33 L75 56 Q67 77 49 76 Q30 72 26 52Z" fill="#283d39"/>
    <path d="M30 28 Q8 30 16 58 Q21 67 30 55Z" fill="#172824"/>
    <path d="M70 29 Q91 32 83 60 Q78 67 71 56Z" fill="#172824"/>
    <path d="M35 55 Q49 43 66 55 L62 69 Q50 79 38 68Z" fill="#72847a"/>
    <ellipse cx="38" cy="43" rx="3.2" ry="4" fill="#eee9d5"/><ellipse cx="64" cy="43" rx="3.2" ry="4" fill="#eee9d5"/>
    <circle cx="39" cy="44" r="1.8" fill="#142921"/><circle cx="63" cy="44" r="1.8" fill="#142921"/>
    <path d="M44 55 Q51 51 58 55 L52 61 Q49 61 44 55" fill="#14251f"/>
    <path d="M48 68 Q52 73 55 67" fill="none" stroke="#1c3229" stroke-width="2" stroke-linecap="round"/>
    <path d="M31 76 Q48 85 69 77" fill="none" stroke="#b68032" stroke-width="5"/>
    <circle cx="51" cy="83" r="4" fill="#d8b06c"/>
  </svg>`;
}

export function domainTable(project: DomainProject, contextId: string | null, selectedId: string | null, query = '', page = 0) {
  const all = contextId ? project.concepts.filter(c => c.contextId === contextId) : project.contexts;
  const words = query.toLocaleLowerCase().trim();
  const matching = all.filter(item => `${item.name} ${'definition' in item ? item.definition : item.description}`.toLocaleLowerCase().includes(words));
  const pages = Math.max(1, Math.ceil(matching.length / 40));
  const current = Math.max(0, Math.min(page, pages - 1));
  const rows = matching.slice(current * 40, (current + 1) * 40);
  const esc = escapeHtml;
  return {
    total: matching.length, pages, page: current,
    html: `<div class="domain-table-scroll"><table><thead><tr><th>${contextId ? 'Domain' : 'Bounded context'}</th><th>${contextId ? 'Kind' : 'Recorded contents'}</th><th>Meaning</th><th><span class="sr-only">Actions</span></th></tr></thead><tbody>${rows.map(item => {
      const term = 'kind' in item ? item : null;
      const count = term ? null : project.concepts.filter(c => c.contextId === item.id).length;
      return `<tr class="${selectedId === item.id ? 'is-selected' : ''}"><td><button data-table-select="${esc(item.id)}">${esc(item.name || 'Unnamed')}</button>${term?.ownerId ? `<small>Within ${esc(project.concepts.find(c => c.id === term.ownerId)?.name)}</small>` : ''}</td><td>${term ? esc(DOMAIN_KINDS[term.kind]) : `${count} entries`}</td><td>${esc(term ? term.definition : 'description' in item ? item.description : '')}</td><td><button data-table-edit="${esc(item.id)}" aria-label="Edit ${esc(item.name)}">Edit ↗</button></td></tr>`;
    }).join('') || `<tr><td colspan="4">${query ? 'No matching domain entries.' : 'Start with a definition or a question below.'}</td></tr>`}</tbody></table></div>`,
  };
}
