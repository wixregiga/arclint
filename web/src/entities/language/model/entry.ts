import type { AssertionDocument, ContextDocument, InvariantDocument, OverviewDocument } from "../../../shared/transport/index";
import { MEMBER_CONCEPTS, propertiesOf, readingProperty, type Concept, type PropertyName } from "./concept";

export type PropertyValue = string | string[];
export type EntryProperties = Partial<Record<PropertyName, PropertyValue>>;

/**
 * One recorded entry, flat: where it sits (its context, and the owner it is
 * recorded under when nested) and the properties its concept records. A
 * bounded context is an entry too, its context its own name.
 */
export interface Entry {
  key: string;
  concept: Concept;
  context: string;
  owner: string;
  name: string;
  properties: EntryProperties;
}

/** A separator no recorded name holds, so a key never collides. */
const SEP = "\u001f";

export function entryKey(context: string, concept: Concept, owner: string, name: string): string {
  return [context, concept, owner, name].join(SEP);
}

export interface EntryAddress {
  concept: Concept;
  context: string;
  owner: string;
  name: string;
}

export function addressOf(entry: Entry): EntryAddress {
  return { concept: entry.concept, context: entry.context, owner: entry.owner, name: entry.name };
}

function entry(context: string, concept: Concept, owner: string, name: string, properties: EntryProperties): Entry {
  return { key: entryKey(context, concept, owner, name), concept, context, owner, name, properties };
}

function invariantEntry(context: string, owner: string, i: InvariantDocument): Entry {
  return entry(context, "invariant", owner, i.key, { statement: i.statement ?? "", verifiedBy: i.verifiedBy ?? [] });
}

function assertionEntry(context: string, owner: string, a: AssertionDocument): Entry {
  return entry(context, "assertion", owner, a.key, { on: a.on ?? "", statement: a.statement ?? "", verifiedBy: a.verifiedBy ?? [] });
}

/** Every entry one bounded context records, after the context itself, in the recording's order. */
export function entriesOfContext(ctx: ContextDocument): Entry[] {
  const c = ctx.name;
  const out: Entry[] = [entry(c, "bounded_context", "", c, { definition: ctx.definition ?? "" })];
  for (const a of ctx.aggregates ?? []) {
    out.push(entry(c, "aggregate", "", a.name, {
      definition: a.definition ?? "",
      identity: a.identity ?? "",
      aliases: a.aliases ?? [],
      repository: a.repository ?? "",
      factory: a.factory ?? "",
    }));
    for (const e of a.entities ?? []) {
      out.push(entry(c, "entity", a.name, e.name, { definition: e.definition ?? "", identity: e.identity ?? "", aliases: e.aliases ?? [] }));
    }
    for (const i of a.invariants ?? []) out.push(invariantEntry(c, a.name, i));
    for (const s of a.assertions ?? []) out.push(assertionEntry(c, a.name, s));
  }
  for (const v of ctx.valueObjects ?? []) {
    out.push(entry(c, "value_object", "", v.name, { definition: v.definition ?? "", aliases: v.aliases ?? [] }));
    for (const i of v.invariants ?? []) out.push(invariantEntry(c, v.name, i));
  }
  for (const e of ctx.events ?? []) {
    out.push(entry(c, "domain_event", "", e.name, { definition: e.definition ?? "", raisedBy: e.raisedBy ?? "" }));
  }
  for (const s of ctx.services ?? []) {
    out.push(entry(c, "domain_service", "", s.name, { definition: s.definition ?? "" }));
    for (const a of s.assertions ?? []) out.push(assertionEntry(c, s.name, a));
  }
  for (const s of ctx.specifications ?? []) out.push(entry(c, "specification", "", s.name, { definition: s.definition ?? "" }));
  for (const q of ctx.questions ?? []) out.push(entry(c, "question", "", q.key, { text: q.text ?? "" }));
  return out;
}

/** Every entry a recording holds, context by context. An overview with no file holds none. */
export function entriesOf(overview: OverviewDocument | null): Entry[] {
  if (!overview || !overview.found) return [];
  return (overview.contexts ?? []).flatMap(entriesOfContext);
}

export function entryAt(entries: readonly Entry[], key: string): Entry | null {
  return entries.find((e) => e.key === key) ?? null;
}

export function contextNames(entries: readonly Entry[]): string[] {
  return entries.filter((e) => e.concept === "bounded_context").map((e) => e.name);
}

/** The entries one context records directly: not the context, not what an owner holds. */
export function topLevelOf(entries: readonly Entry[], context: string): Entry[] {
  return entries.filter((e) => e.context === context && e.concept !== "bounded_context" && e.owner === "");
}

export function ofConcept(entries: readonly Entry[], context: string, concept: Concept): Entry[] {
  return entries.filter((e) => e.context === context && e.concept === concept);
}

/** What an owner records under itself, in the recording's order. */
export function membersOf(entries: readonly Entry[], owner: Entry): Entry[] {
  const concepts = MEMBER_CONCEPTS[owner.concept] ?? [];
  return entries.filter((e) => e.context === owner.context && e.owner === owner.name && concepts.includes(e.concept));
}

/** The entry a nested one is recorded under, or null when it is not nested. */
export function ownerOf(entries: readonly Entry[], member: Entry): Entry | null {
  if (member.owner === "") return null;
  return entries.find((e) => e.context === member.context && e.name === member.owner && (MEMBER_CONCEPTS[e.concept] ?? []).includes(member.concept)) ?? null;
}

/** The text of an entry's reading: its definition, statement or question. */
export function readingOf(entry: Entry): string {
  const value = entry.properties[readingProperty(entry.concept)];
  return typeof value === "string" ? value : "";
}

export function aliasesOf(entry: Entry): string[] {
  const value = entry.properties.aliases;
  return Array.isArray(value) ? value : [];
}

export function textOf(entry: Entry, name: PropertyName): string {
  const value = entry.properties[name];
  return typeof value === "string" ? value : "";
}

export function listOf(entry: Entry, name: PropertyName): string[] {
  const value = entry.properties[name];
  return Array.isArray(value) ? value : [];
}

/** The required properties an entry records nothing under; the language refuses it without them. */
export function missingRequired(entry: Entry): PropertyName[] {
  return propertiesOf(entry.concept)
    .filter((p) => p.required)
    .filter((p) => (p.shape === "list" ? listOf(entry, p.name).length === 0 : textOf(entry, p.name).trim() === ""))
    .map((p) => p.name);
}
