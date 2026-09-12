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
function sourceExport(project: DomainProject): RecordValue {
  const source = structuredClone(project.sourceDocument!);
  const original = fromDomain(source);
  const fail = (detail: string): never => { throw new Error(`Canonical YAML cannot preserve this edit: ${detail}. Export the Studio JSON to keep every change, or edit the canonical source with its full ownership and identity metadata.`); };
  for (const group of ['contexts', 'concepts', 'relationships'] as const) {
    const oldIds = original[group].map(item => item.id).sort();
    if (JSON.stringify(oldIds) !== JSON.stringify(project[group].map(item => item.id).sort())) fail(`added or removed ${group}`);
  }
  const contextMap = record(source.contexts, 'contexts');
  for (const context of project.contexts) {
    const previous = original.contexts.find(c => c.id === context.id)!;
    if (context.name !== previous.name) fail('renaming a context may affect preserved metadata references');
    record(contextMap[previous.name], 'context').definition = text(context.description, 'Context definition');
  }
  for (const concept of project.concepts) {
    const previous = original.concepts.find(c => c.id === concept.id)!;
    if (concept.name !== previous.name || concept.kind !== previous.kind || concept.contextId !== previous.contextId || concept.ownerId !== previous.ownerId) fail(`renaming, reclassifying, or moving ${previous.name}`);
    const path = concept.id.slice(5).split('/').map(decodeURIComponent);
    if (concept.identity !== previous.identity && !['aggregate', 'entity'].includes(concept.kind)) fail(`identity metadata on ${concept.kind}`);
    if (JSON.stringify(concept.aliases) !== JSON.stringify(previous.aliases) && !['aggregate', 'entity', 'value_object'].includes(concept.kind)) fail(`aliases on ${concept.kind}`);
    if (concept.kind === 'unclassified') {
      if (concept.invariants.length) fail(`adding invariants to unresolved ${concept.name}`);
      record(readPath(source, path.slice(0, -1)), 'questions')[path.at(-1)!] = text(concept.definition, 'Question');
    } else if (concept.kind === 'repository' || concept.kind === 'factory') {
      if (concept.definition !== previous.definition || concept.invariants.length) fail(`adding unsupported detail to ${concept.kind}`);
    } else {
      const term = record(readPath(source, path), concept.name);
      term.definition = text(concept.definition, 'Definition');
      if (concept.identity !== previous.identity) {
        if (concept.kind === 'aggregate') term.identity = text(concept.identity, `${concept.name} identity`);
        else if (concept.identity?.trim()) term.identity = concept.identity;
        else delete term.identity;
      }
      if (JSON.stringify(concept.aliases) !== JSON.stringify(previous.aliases)) {
        if (concept.aliases?.length) term.aliases = [...concept.aliases];
        else delete term.aliases;
      }
      if (JSON.stringify(previous.invariants) !== JSON.stringify(concept.invariants)) {
        if (concept.kind !== 'aggregate' && concept.kind !== 'value_object') fail(`invariants on ${concept.kind}`);
        term.invariants = invariantMap(concept.invariants, term.invariants ?? {});
      }
    }
  }
  for (const relation of project.relationships) {
    const previous = original.relationships.find(r => r.id === relation.id)!;
    if (JSON.stringify(previous) !== JSON.stringify(relation)) fail('changed relationship semantics');
  }
  source.project = text(project.name, 'Project name');
  if (project.description) source.description = project.description;
  else delete source.description;
  return source;
}

/** Canonical YAML is the language record. Studio JSON is the complete visual workspace. */
export function exportDomainYaml(input: DomainProject): string {
  const project = assertProject(input);
  if (project.sourceDocument) return stringify(sourceExport(project));
  const contexts: Record<string, RecordValue> = Object.create(null);
  for (const context of project.contexts) {
    if (!/^[a-z][a-z0-9_-]*$/.test(context.name)) throw new Error(`Canonical context names must use lowercase letters, numbers, underscores or hyphens: ${context.name}. Studio JSON accepts display names unchanged.`);
    if (Object.hasOwn(contexts, context.name)) throw new Error(`Duplicate context name: ${context.name}.`);
    contexts[context.name] = { definition: text(context.description, `${context.name} definition`) };
  }
  const questions = (contextId: string): RecordValue => {
    const context = project.contexts.find(c => c.id === contextId)!;
    const recordContext = contexts[context.name];
    return (recordContext.questions ??= Object.create(null)) as RecordValue;
  };
  const key = (id: string) => `studio-${Array.from(id).map(char => char.codePointAt(0)!.toString(16)).join('-')}`;
  const records = new Map<string, RecordValue>();
  const termRecord = (concept: Concept): RecordValue => {
    if (concept.invariants.length && !['aggregate', 'value_object'].includes(concept.kind)) throw new Error(`Canonical YAML cannot represent invariants on ${concept.kind}; use Studio JSON.`);
    if (concept.identity && !['aggregate', 'entity'].includes(concept.kind)) throw new Error(`Canonical YAML cannot represent identity on ${concept.kind}; use Studio JSON.`);
    if (concept.aliases?.length && !['aggregate', 'entity', 'value_object'].includes(concept.kind)) throw new Error(`Canonical YAML cannot represent aliases on ${concept.kind}; use Studio JSON.`);
    return {
      definition: text(concept.definition, `${concept.name} definition`),
      ...(concept.kind === 'aggregate' ? { identity: text(concept.identity, `${concept.name} aggregate identity`) } : concept.identity?.trim() ? { identity: concept.identity } : {}),
      ...(concept.aliases?.length ? { aliases: [...concept.aliases] } : {}),
      ...(concept.invariants.length ? { invariants: invariantMap(concept.invariants) } : {}),
    };
  };
  for (const concept of project.concepts) {
    const context = contexts[project.contexts.find(c => c.id === concept.contextId)!.name];
    if (concept.kind === 'unclassified') {
      questions(concept.contextId)[key(concept.id)] = `What kind of domain concept is ${concept.name}? Proposed meaning: ${concept.definition || '(not defined)'}.${concept.invariants.length ? ` Proposed rules: ${concept.invariants.join('; ')}` : ''}${concept.identity ? ` Proposed identity: ${concept.identity}.` : ''}${concept.aliases?.length ? ` Proposed aliases: ${concept.aliases.join(', ')}.` : ''}`;
      continue;
    }
    if (['entity', 'repository', 'factory'].includes(concept.kind)) continue;
    if (concept.ownerId) throw new Error(`${concept.name} has aggregate ownership metadata its kind cannot represent in canonical YAML.`);
    const group = groups.find(([, kind]) => concept.kind === kind)![0];
    const target = (context[group] ??= Object.create(null)) as RecordValue;
    if (Object.hasOwn(target, concept.name)) throw new Error(`Repeated term ${concept.name}.`);
    const term = termRecord(concept);
    target[text(concept.name, 'Concept name')] = term;
    records.set(concept.id, term);
  }
  for (const concept of project.concepts.filter(c => ['entity', 'repository', 'factory'].includes(c.kind))) {
    const owner = project.concepts.find(candidate => candidate.id === concept.ownerId);
    const aggregate = owner && records.get(owner.id);
    if (!owner || owner.kind !== 'aggregate' || owner.contextId !== concept.contextId || !aggregate) throw new Error(`${concept.name} needs an aggregate owner in the same context before canonical YAML export.`);
    if (concept.kind === 'entity') {
      const entities = (aggregate.entities ??= Object.create(null)) as RecordValue;
      if (Object.hasOwn(entities, concept.name)) throw new Error(`Repeated member ${concept.name} in ${owner.name}.`);
      entities[text(concept.name, 'Entity name')] = termRecord(concept);
    } else {
      if (aggregate[concept.kind]) throw new Error(`${owner.name} has more than one ${concept.kind}; canonical YAML supports one.`);
      if (concept.invariants.length || concept.identity || concept.aliases?.length) throw new Error(`Canonical YAML records ${concept.kind} by name; extra invariants, identity, or aliases must stay in Studio JSON.`);
      aggregate[concept.kind] = text(concept.name, `${concept.kind} name`);
      const generatedDefinition = `${concept.kind === 'repository' ? 'Repository' : 'Factory'} recorded for ${owner.name}.`;
      if (concept.definition.trim() && concept.definition !== generatedDefinition) questions(concept.contextId)[key(concept.id)] = `Proposed description of ${concept.name}, ${concept.kind} of ${owner.name}: ${concept.definition}. Does this describe its role correctly?`;
    }
  }
  const relations: RecordValue[] = [];
  for (const relationship of project.relationships) {
    const from = project.contexts.find(c => c.id === relationship.source);
    const to = project.contexts.find(c => c.id === relationship.target);
    if (from && to && contextKinds.includes(relationship.label)) relations.push({ from: from.name, to: to.name, kind: relationship.label });
    else {
      const nodes = [...project.contexts, ...project.concepts];
      const source = nodes.find(n => n.id === relationship.source)!;
      const target = nodes.find(n => n.id === relationship.target)!;
      const owner = 'contextId' in source ? source.contextId : source.id;
      questions(owner)[key(relationship.id)] = `Proposed relationship: ${source.name} → ${target.name}: ${relationship.label || '(unnamed)'}. How should this connection be recorded in the domain?`;
    }
  }
  return '# Canonical language draft. Unclassified concepts and free-form connections are open questions.\n# Use Studio JSON to retain visual positions, stable IDs, and all editing details.\n' + stringify({ version: 1, project: project.name, ...(project.description ? { description: project.description } : {}), contexts, ...(relations.length ? { relations } : {}) });
}

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
