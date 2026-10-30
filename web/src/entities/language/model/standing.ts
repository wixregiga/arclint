import type { RelationDocument } from "../../../shared/transport/index";
import { propertiesOf } from "./concept";
import type { Entry, EntryAddress, PropertyValue } from "./entry";

/**
 * Where an entry stands against the recording the default branch holds:
 * accepted when the branch records it the same, proposed when the
 * working tree records it differently or the branch does not record it.
 */
export type Standing = "accepted" | "proposed";

export type DifferenceKind = "added" | "changed" | "removed";

export interface FieldDifference {
  field: string;
  before: PropertyValue | null;
  after: PropertyValue | null;
}

export interface Difference extends EntryAddress {
  key: string;
  kind: DifferenceKind;
  fields: FieldDifference[];
}

function same(a: PropertyValue | undefined, b: PropertyValue | undefined): boolean {
  const as = Array.isArray(a) ? a.join("\n") : (a ?? "");
  const bs = Array.isArray(b) ? b.join("\n") : (b ?? "");
  return as === bs;
}

function empty(v: PropertyValue | undefined): boolean {
  return Array.isArray(v) ? v.length === 0 : (v ?? "") === "";
}

/**
 * The transformation from the accepted recording to the current one: each
 * entry added, removed or changed, with every property that differs.
 * Current entries first in their order, then the removed ones in the
 * accepted order.
 */
export function differences(accepted: readonly Entry[], current: readonly Entry[]): Difference[] {
  const before = new Map(accepted.map((e) => [e.key, e]));
  const after = new Map(current.map((e) => [e.key, e]));
  const out: Difference[] = [];
  for (const now of current) {
    const was = before.get(now.key);
    const address = { key: now.key, concept: now.concept, context: now.context, owner: now.owner, name: now.name };
    if (!was) {
      out.push({ ...address, kind: "added", fields: propertiesOf(now.concept).filter((p) => !empty(now.properties[p.name])).map((p) => ({ field: p.name, before: null, after: now.properties[p.name] ?? null })) });
      continue;
    }
    const fields = propertiesOf(now.concept)
      .filter((p) => !same(was.properties[p.name], now.properties[p.name]))
      .map((p) => ({ field: p.name, before: was.properties[p.name] ?? null, after: now.properties[p.name] ?? null }));
    if (fields.length > 0) out.push({ ...address, kind: "changed", fields });
  }
  for (const was of accepted) {
    if (after.has(was.key)) continue;
    out.push({ key: was.key, concept: was.concept, context: was.context, owner: was.owner, name: was.name, kind: "removed", fields: propertiesOf(was.concept).filter((p) => !empty(was.properties[p.name])).map((p) => ({ field: p.name, before: was.properties[p.name] ?? null, after: null })) });
  }
  return out;
}

export interface RelationDifference {
  kind: DifferenceKind;
  from: string;
  to: string;
  relation: string;
  before: RelationDocument | null;
  after: RelationDocument | null;
}

function pairKey(r: RelationDocument): string {
  return [r.from, r.to].sort().join("\u001f");
}

/** The relations of the context map that differ, one per pair of contexts. */
export function relationDifferences(accepted: readonly RelationDocument[], current: readonly RelationDocument[]): RelationDifference[] {
  const before = new Map(accepted.map((r) => [pairKey(r), r]));
  const after = new Map(current.map((r) => [pairKey(r), r]));
  const out: RelationDifference[] = [];
  for (const now of current) {
    const was = before.get(pairKey(now));
    if (!was) out.push({ kind: "added", from: now.from, to: now.to, relation: now.kind, before: null, after: now });
    else if (was.from !== now.from || was.to !== now.to || was.kind !== now.kind || (was.description ?? "") !== (now.description ?? "")) {
      out.push({ kind: "changed", from: now.from, to: now.to, relation: now.kind, before: was, after: now });
    }
  }
  for (const was of accepted) {
    if (!after.has(pairKey(was))) out.push({ kind: "removed", from: was.from, to: was.to, relation: was.kind, before: was, after: null });
  }
  return out;
}

/** The standing of every current entry; every key absent from the map is accepted. */
export function proposedKeys(diffs: readonly Difference[]): Set<string> {
  return new Set(diffs.filter((d) => d.kind !== "removed").map((d) => d.key));
}

export function standingOf(proposed: ReadonlySet<string>, key: string): Standing {
  return proposed.has(key) ? "proposed" : "accepted";
}
