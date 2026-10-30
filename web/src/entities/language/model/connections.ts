import type { RelationDocument } from "../../../shared/transport/index";
import { CONCEPT_TITLE } from "./concept";
import { membersOf, ofConcept, ownerOf, textOf, type Entry } from "./entry";

/**
 * A connection the recording states between two entries: the owner a
 * nested entry is recorded under and what an owner holds, the aggregate
 * an event names as raising it, the value object an identity names, and
 * the relations of the context map between bounded contexts. Studio
 * derives nothing beyond what is recorded.
 */
export type ConnectionKind = "owner" | "member" | "raises" | "raised-by" | "identity" | "identity-of" | "relation";

export interface Connection {
  kind: ConnectionKind;
  /** What the connection says, read from the entry it is listed under. */
  label: string;
  entry: Entry;
  /** The recorded description of a relation, when it has one. */
  note?: string;
}

function identityTarget(entries: readonly Entry[], entry: Entry): Entry | null {
  const identity = textOf(entry, "identity").trim();
  if (identity === "") return null;
  return entries.find((e) => e.context === entry.context && e.concept === "value_object" && e.name === identity) ?? null;
}

export function connectionsOf(entries: readonly Entry[], relations: readonly RelationDocument[], entry: Entry): Connection[] {
  const out: Connection[] = [];
  if (entry.concept === "bounded_context") {
    for (const r of relations) {
      if (r.from === entry.name) {
        const other = entries.find((e) => e.concept === "bounded_context" && e.name === r.to);
        if (other) out.push({ kind: "relation", label: `${relationWord(r.kind)} upstream of`, entry: other, note: r.description });
      } else if (r.to === entry.name) {
        const other = entries.find((e) => e.concept === "bounded_context" && e.name === r.from);
        if (other) out.push({ kind: "relation", label: `${relationWord(r.kind)} downstream of`, entry: other, note: r.description });
      }
    }
    return out;
  }
  const owner = ownerOf(entries, entry);
  if (owner) out.push({ kind: "owner", label: ownerLabel(entry), entry: owner });
  for (const m of membersOf(entries, entry)) out.push({ kind: "member", label: memberLabel(m), entry: m });
  if (entry.concept === "domain_event") {
    const raisedBy = textOf(entry, "raisedBy");
    const raiser = raisedBy === "" ? null : (ofConcept(entries, entry.context, "aggregate").find((a) => a.name === raisedBy) ?? null);
    if (raiser) out.push({ kind: "raised-by", label: "Raised by", entry: raiser });
  }
  if (entry.concept === "aggregate") {
    for (const e of ofConcept(entries, entry.context, "domain_event")) {
      if (textOf(e, "raisedBy") === entry.name) out.push({ kind: "raises", label: "Raises", entry: e });
    }
  }
  const identity = identityTarget(entries, entry);
  if (identity) out.push({ kind: "identity", label: "Identified by", entry: identity });
  if (entry.concept === "value_object") {
    for (const e of entries) {
      if (e.context === entry.context && (e.concept === "aggregate" || e.concept === "entity") && textOf(e, "identity") === entry.name) {
        out.push({ kind: "identity-of", label: "Identity of", entry: e });
      }
    }
  }
  return out;
}

function ownerLabel(member: Entry): string {
  switch (member.concept) {
    case "entity":
      return "Belongs to";
    case "invariant":
      return "Always holds for";
    case "assertion":
      return "Guaranteed by";
    default:
      return "Recorded under";
  }
}

function memberLabel(member: Entry): string {
  switch (member.concept) {
    case "entity":
      return "Holds";
    case "invariant":
      return "Always";
    case "assertion":
      return "Guarantees";
    default:
      return CONCEPT_TITLE[member.concept];
  }
}

/** A relation kind as the recording spells it, in words. */
export function relationWord(kind: string): string {
  return kind.replace(/_/g, " ");
}
