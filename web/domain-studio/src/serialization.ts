import { parseDocument, stringify } from 'yaml';
import { assertProject } from './domain';
import type { Concept, ConceptKind, DomainProject, PatternSummary } from './contracts';

type RecordValue = Record<string, unknown>;
const contextKinds = ['partnership', 'shared_kernel', 'customer_supplier', 'conformist', 'anticorruption_layer', 'open_host_service', 'published_language', 'separate_ways'];
const groups: [string, ConceptKind][] = [['aggregates', 'aggregate'], ['value_objects', 'value_object'], ['events', 'domain_event'], ['services', 'domain_service'], ['specifications', 'specification']];
const colors = ['#66d8df', '#f6b85c', '#b1a1ee', '#ef9377', '#98c989'];
function record(value: unknown, label: string): RecordValue {
  if (!value || typeof value !== 'object' || Array.isArray(value)) throw new Error(`${label} must be a mapping.`);
  return value as RecordValue;
}
function onlyKeys(value: RecordValue, allowed: string[], label: string): void {
  for (const key of Object.keys(value)) if (!allowed.includes(key)) throw new Error(`${label} contains unsupported field ${key}. Import a Studio JSON workspace to retain noncanonical metadata.`);
}
function text(value: unknown, label: string): string {
  if (typeof value !== 'string' || !value.trim()) throw new Error(`${label} must be nonempty text.`);
  return value;
}
function parse(textInput: string): unknown {
  if (textInput.length > 5_000_000) throw new Error('Import is limited to 5 MB.');
  const document = parseDocument(textInput, { uniqueKeys: true });
  if (document.errors.length) throw new Error(document.errors[0].message);
  return document.toJS({ maxAliasCount: 50 });
}
const pathId = (path: string[]) => `yaml:${path.map(encodeURIComponent).join('/')}`;
function readPath(root: RecordValue, path: string[]): unknown {
  let result: unknown = root;
  for (const key of path) result = record(result, path.join('.'))[key];
  return result;
}
function checkedStringMap(value: unknown, label: string): Record<string, string> {
  const values = record(value, label);
  for (const [key, v] of Object.entries(values)) {
    if (!/^[a-z][a-z0-9]*(-[a-z0-9]+)*$/.test(key)) throw new Error(`${label} key ${key} must use lowercase kebab case.`);
    text(v, `${label}.${key}`);
  }
  return values as Record<string, string>;
}
function checkTerm(value: unknown, label: string, kind: ConceptKind): RecordValue {
  const term = record(value, label);
  const allowed: Partial<Record<ConceptKind, string[]>> = {
    aggregate: ['definition', 'identity', 'aliases', 'entities', 'invariants', 'assertions', 'repository', 'factory'],
    entity: ['definition', 'identity', 'aliases'],
    value_object: ['definition', 'aliases', 'invariants'],
    domain_event: ['definition', 'raised_by'],
    domain_service: ['definition'], specification: ['definition'],
  };
  onlyKeys(term, allowed[kind] ?? ['definition'], label);
  text(term.definition, `${label}.definition`);
  if (kind === 'aggregate') text(term.identity, `${label}.identity`);
  if (term.identity !== undefined) text(term.identity, `${label}.identity`);
  if (term.aliases !== undefined && (!Array.isArray(term.aliases) || term.aliases.some(a => typeof a !== 'string' || !a.trim()) || new Set(term.aliases).size !== term.aliases.length)) throw new Error(`${label}.aliases must contain unique nonempty names.`);
  if (term.invariants !== undefined) checkedStringMap(term.invariants, `${label}.invariants`);
  if (term.assertions !== undefined) {
    for (const [key, assertion] of Object.entries(record(term.assertions, `${label}.assertions`))) {
      if (!/^[a-z][a-z0-9]*(-[a-z0-9]+)*$/.test(key)) throw new Error(`Invalid assertion key: ${key}.`);
      const a = record(assertion, key);
      onlyKeys(a, ['on', 'statement'], key);
      text(a.on, `${key}.on`); text(a.statement, `${key}.statement`);
    }
  }
  for (const key of ['repository', 'factory', 'raised_by']) if (term[key] !== undefined) text(term[key], `${label}.${key}`);
  return term;
}

/** Projects keep the complete parsed source, including fields outside the spatial projection. */
function fromDomain(document: unknown): DomainProject {
  const source = record(document, 'Domain document');
  onlyKeys(source, ['version', 'project', 'description', 'contexts', 'relations'], 'Domain document');
  if (source.version !== 1) throw new Error('Unsupported domain version; expected version 1.');
  const project: DomainProject = { version: 1, name: text(source.project, 'project'), description: source.description === undefined ? '' : text(source.description, 'description'), contexts: [], concepts: [], relationships: [], sourceDocument: structuredClone(source) };
  const contexts = record(source.contexts, 'contexts');
  const contextEntries = Object.entries(contexts);
  for (const [index, [name, raw]] of contextEntries.entries()) {
    if (!/^[a-z][a-z0-9_-]*$/.test(name)) throw new Error(`Invalid context name: ${name}.`);
    const context = record(raw, `contexts.${name}`);
    onlyKeys(context, ['definition', ...groups.map(([group]) => group), 'questions'], name);
    const cx = ((index % 3) - 1) * 34;
    const cz = Math.floor(index / 3) * 32 - 8;
    const contextId = pathId(['contexts', name]);
    project.contexts.push({ id: contextId, name, description: text(context.definition, `${name}.definition`), color: colors[index % colors.length], position: [cx, 0, cz] });
    let count = 0;
    const add = (termName: string, kind: ConceptKind, definition: string, path: string[], invariants: string[] = []): Concept => {
      if (!termName.trim() || termName.trim() !== termName) throw new Error('Domain term names cannot be empty or padded with whitespace.');
      const angle = count * 2.39996;
      const radius = 4 + Math.sqrt(count) * 3;
      const concept: Concept = { id: pathId(path), name: termName, kind, definition, contextId, invariants, position: [cx + Math.cos(angle) * radius, 1.5 + (count % 3) * 0.8, cz + Math.sin(angle) * radius] };
      count++; project.concepts.push(concept); return concept;
    };
    for (const [group, kind] of groups) {
      if (context[group] === undefined) continue;
      for (const [termName, rawTerm] of Object.entries(record(context[group], `${name}.${group}`))) {
        const path = ['contexts', name, group, termName];
        const term = checkTerm(rawTerm, path.join('.'), kind);
        const parent = add(termName, kind, String(term.definition), path, Object.values((term.invariants ?? {}) as Record<string, string>));
        if (typeof term.identity === 'string') parent.identity = term.identity;
        if (term.assertions !== undefined) parent.assertions = Object.entries(record(term.assertions, 'assertions')).map(([key, value]) => ({ key, on: String(record(value, key).on), statement: String(record(value, key).statement) }));
        if (typeof term.raised_by === 'string') parent.raisedBy = term.raised_by;
        if (Array.isArray(term.aliases)) parent.aliases = [...term.aliases] as string[];
        if (kind === 'aggregate') {
          for (const [memberName, rawMember] of Object.entries(record(term.entities ?? {}, `${termName}.entities`))) {
            const member = checkTerm(rawMember, memberName, 'entity');
            const node = add(memberName, 'entity', String(member.definition), [...path, 'entities', memberName]);
            node.ownerId = parent.id;
            if (typeof member.identity === 'string') node.identity = member.identity;
            if (Array.isArray(member.aliases)) node.aliases = [...member.aliases] as string[];
            project.relationships.push({ id: `${node.id}:owner`, source: parent.id, target: node.id, label: 'owns member' });
          }
          for (const role of ['repository', 'factory'] as const) if (term[role]) {
            const node = add(String(term[role]), role, `${role === 'repository' ? 'Repository' : 'Factory'} recorded for ${termName}.`, [...path, role]);
            node.ownerId = parent.id;
            project.relationships.push({ id: `${node.id}:owner`, source: node.id, target: parent.id, label: role === 'repository' ? 'repository for' : 'creates' });
          }
        }
      }
    }
    if (context.questions !== undefined) for (const [key, question] of Object.entries(checkedStringMap(context.questions, `${name}.questions`))) add(key, 'unclassified', question, ['contexts', name, 'questions', key]);
  }
  if (source.relations !== undefined) {
    if (!Array.isArray(source.relations)) throw new Error('relations must be an array.');
    source.relations.forEach((raw, index) => {
      const relation = record(raw, 'relation');
      onlyKeys(relation, ['from', 'to', 'kind', 'description'], 'relation');
      const from = text(relation.from, 'relation.from'); const to = text(relation.to, 'relation.to');
      const kind = text(relation.kind, 'relation.kind');
      if (!contextKinds.includes(kind)) throw new Error(`Unsupported context relation kind: ${kind}.`);
      if (relation.description !== undefined) text(relation.description, 'relation.description');
      project.relationships.push({ id: pathId(['relations', String(index)]), source: pathId(['contexts', from]), target: pathId(['contexts', to]), label: kind });
    });
  }
  return assertProject(project);
}

export function importProject(input: string): DomainProject {
  const value = record(parse(input), 'Import');
  return 'project' in value && !Array.isArray(value.contexts) ? fromDomain(value) : assertProject(value);
}
export function exportProject(project: DomainProject): string { return JSON.stringify(assertProject(project), null, 2); }

function invariantMap(values: string[], previous: unknown = {}): Record<string, string> {
  const old = record(previous, 'Invariants');
  const keys = Object.keys(old);
  const used = new Set<string>();
  // Match unchanged statements first so moving/deleting a line never renames
  // a surviving invariant. Edited lines reuse remaining keys; new ones get
  // keys that cannot overwrite any imported invariant.
  const assigned = values.map(value => {
    text(value, 'Invariant');
    const key = keys.find(key => !used.has(key) && old[key] === value);
    if (key) used.add(key);
    return key;
  });
  const result: Record<string, string> = Object.create(null);
  values.forEach((value, i) => {
    let key = assigned[i] ?? keys.find(key => !used.has(key));
    if (!key) {
      let suffix = 1;
      do { key = `recorded-rule-${suffix++}`; } while (used.has(key) || Object.hasOwn(old, key));
    }
    used.add(key);
    result[key] = value;
  });
  return result;
}
/** Rebuild ownership from current identities, carrying canonical metadata by stable source ID. */
function sourceExport(project: DomainProject): RecordValue {
  const source = project.sourceDocument;
  const original = source ? fromDomain(source) : undefined;
  const oldContext = (id: string) => original?.contexts.find(c => c.id === id);
  const oldTerm = (id: string): RecordValue => {
    if (!source || !id.startsWith('yaml:')) return {};
    const value = id.slice(5).split('/').map(decodeURIComponent).reduce<unknown>((value, key) => value && typeof value === 'object' ? (value as RecordValue)[key] : undefined, source);
    return value && typeof value === 'object' && !Array.isArray(value) ? structuredClone(value as RecordValue) : {};
  };
  const contexts: Record<string, RecordValue> = Object.create(null);
  for (const context of project.contexts) {
    if (!/^[a-z][a-z0-9_-]*$/.test(context.name)) throw new Error(`Canonical context names use lowercase letters, numbers, underscores or hyphens: ${context.name}.`);
    if (Object.hasOwn(contexts, context.name)) throw new Error(`Duplicate context name: ${context.name}.`);
    const target: RecordValue = { definition: text(context.description, `${context.name} definition`) };
    const previous = oldContext(context.id);
    // Retain authored empty collections as well as all actual entries.
    const raw = previous && source ? record(record(source.contexts, 'contexts')[previous.name], 'context') : {};
    for (const key of [...groups.map(([group]) => group), 'questions']) if (raw[key] && !Object.keys(record(raw[key], key)).length) target[key] = Object.create(null);
    contexts[context.name] = target;
  }
  const contextRecord = (id: string) => contexts[project.contexts.find(c => c.id === id)!.name];
  const question = (contextId: string, key: string, value: string) => {
    const target = contextRecord(contextId);
    const questions = (target.questions ??= Object.create(null)) as RecordValue;
    if (Object.hasOwn(questions, key)) throw new Error(`Repeated open question ${key}.`);
    questions[key] = value;
  };
  const keyFor = (id: string) => `studio-${Array.from(id).map(char => char.codePointAt(0)!.toString(16)).join('-')}`;
  const termRecord = (entry: Concept): RecordValue => {
    const raw = oldTerm(entry.id);
    // These fields are derived from current explicit membership below.
    delete raw.entities; delete raw.repository; delete raw.factory;
    raw.definition = text(entry.definition, `${entry.name} definition`);
    if (entry.kind === 'aggregate') raw.identity = text(entry.identity, `${entry.name} aggregate identity`);
    else if (entry.identity) raw.identity = entry.identity;
    else delete raw.identity;
    if (entry.aliases?.length) raw.aliases = [...entry.aliases]; else delete raw.aliases;
    if (entry.invariants.length || raw.invariants !== undefined) {
      if (!['aggregate', 'value_object'].includes(entry.kind)) throw new Error(`Canonical YAML cannot represent invariants on ${entry.kind}.`);
      raw.invariants = invariantMap(entry.invariants, raw.invariants ?? {});
    }
    if (entry.assertions !== undefined) raw.assertions = Object.fromEntries(entry.assertions.map(({key,on,statement}) => [key,{on,statement}]));
    if (entry.raisedBy !== undefined) raw.raised_by = entry.raisedBy;
    return raw;
  };
  const records = new Map<string, RecordValue>();
  for (const entry of project.concepts) {
    const raw = oldTerm(entry.id);
    const assertions = entry.assertions ?? Object.entries(record(raw.assertions ?? {}, 'Assertions'));
    if (assertions.length && entry.kind !== 'aggregate') throw new Error(`Cannot reclassify ${entry.name} while retaining aggregate operation assertions. Resolve those contracts explicitly first.`);
    if ((entry.raisedBy || raw.raised_by) && entry.kind !== 'domain_event') throw new Error(`Cannot reclassify ${entry.name} while retaining its event source. Resolve that contract explicitly first.`);
    if (entry.kind === 'unclassified') {
      const previous = original?.concepts.find(c => c.id === entry.id && c.kind === 'unclassified');
      if (previous && /^[a-z][a-z0-9]*(-[a-z0-9]+)*$/.test(entry.name)) question(entry.contextId, entry.name, text(entry.definition, 'Open question'));
      else question(entry.contextId, keyFor(entry.id), `What kind of domain concept is ${entry.name}? Proposed meaning: ${entry.definition || '(not defined)'}.${entry.invariants.length ? ` Proposed rules: ${entry.invariants.join('; ')}` : ''}${entry.identity ? ` Proposed identity: ${entry.identity}.` : ''}${entry.aliases?.length ? ` Proposed aliases: ${entry.aliases.join(', ')}.` : ''}`);
      continue;
    }
    if (['entity','repository','factory'].includes(entry.kind)) continue;
    if (entry.ownerId) throw new Error(`${entry.name} has aggregate ownership metadata its kind cannot represent in canonical YAML.`);
    const group = groups.find(([,kind]) => kind === entry.kind)![0];
    const context = contextRecord(entry.contextId), target = (context[group] ??= Object.create(null)) as RecordValue;
    if (Object.hasOwn(target, entry.name)) throw new Error(`Repeated term ${entry.name}.`);
    const term = termRecord(entry); target[text(entry.name, 'Domain name')] = term; records.set(entry.id,term);
  }
  for (const entry of project.concepts.filter(c => ['entity','repository','factory'].includes(c.kind))) {
    const owner = project.concepts.find(c => c.id === entry.ownerId), aggregate = owner && records.get(owner.id);
    if (!owner || owner.kind !== 'aggregate' || owner.contextId !== entry.contextId || !aggregate) throw new Error(`${entry.name} needs an aggregate owner in the same context before canonical YAML export.`);
    if (entry.kind === 'entity') {
      const members = (aggregate.entities ??= Object.create(null)) as RecordValue;
      if (Object.hasOwn(members,entry.name)) throw new Error(`Repeated member ${entry.name}.`);
      members[text(entry.name,'Entity name')] = termRecord(entry);
    } else {
      if (aggregate[entry.kind]) throw new Error(`${owner.name} has more than one ${entry.kind}; canonical YAML supports one.`);
      if (entry.invariants.length || entry.identity || entry.aliases?.length) throw new Error(`Canonical YAML records ${entry.kind} by name; extra invariants, identity, or aliases must stay in Studio JSON.`);
      aggregate[entry.kind] = text(entry.name,entry.kind);
      const generated = `${entry.kind === 'repository' ? 'Repository' : 'Factory'} recorded for ${owner.name}.`;
      const previous = original?.concepts.find(c => c.id === entry.id);
      const inheritedRoleDescription = previous?.kind === entry.kind && entry.definition === previous.definition;
      if (entry.definition.trim() && entry.definition !== generated && !inheritedRoleDescription) question(entry.contextId,keyFor(entry.id),`Proposed description of ${entry.name}, ${entry.kind} of ${owner.name}: ${entry.definition}. Does this describe its role correctly?`);
    }
  }
  const relations: RecordValue[] = [];
  for (const relation of project.relationships) {
    const member = project.concepts.find(c => relation.id === `${c.id}:owner`);
    if (member?.ownerId) continue; // Aggregate membership already records this exact relationship.
    const from = project.contexts.find(c => c.id === relation.source), to = project.contexts.find(c => c.id === relation.target);
    if (from && to && contextKinds.includes(relation.label)) {
      const old = oldTerm(relation.id);
      relations.push({...old,from:from.name,to:to.name,kind:relation.label});
    } else {
      const nodes = [...project.contexts,...project.concepts], from = nodes.find(c => c.id === relation.source)!, to = nodes.find(c => c.id === relation.target)!;
      question('contextId' in from ? from.contextId : from.id,keyFor(relation.id),`Proposed relationship: ${from.name} → ${to.name}: ${relation.label || '(unnamed)'}. How should this connection be recorded in the domain?`);
    }
  }
  const result = {version:1,project:project.name,...(project.description ? {description:project.description}:{}),contexts,...(relations.length || source?.relations ? {relations}: {})};
  // Validate the complete result so incompatible reclassifications cannot drop contracts.
  fromDomain(result);
  return result;
}

/** Canonical semantics are independent of layout and presentation. */
export function exportDomainYaml(input: DomainProject): string { return stringify(sourceExport(assertProject(input))); }

export function parsePattern(input: string): PatternSummary {
  const document = record(parse(input), 'Pattern document');
  const manifest = record(document.pattern, 'pattern');
  const namespace = text(manifest.namespace, 'pattern.namespace');
  const name = text(manifest.name, 'pattern.name');
  const version = text(manifest.version, 'pattern.version');
  const rules = record(document.rules, 'rules');
  return {
    name: `${namespace}/${name}@${version}`,
    description: typeof manifest.documentation === 'string' ? manifest.documentation : 'Imported Pattern definition. Inspecting these rules does not execute ArcLint conformance checks.',
    rules: Object.entries(rules).map(([id, raw]) => {
      const rule = record(raw, `rules.${id}`);
      return { id: `${namespace}/${name}:${id}`, description: [typeof rule.rationale === 'string' ? rule.rationale : '', stringify(rule).trim()].filter(Boolean).join('\n\n') };
    }),
  };
}
