/*
 * The uses of the language in a feature's prose: what the text names by
 * itself (cues, or ambiguities between entries spelled the same), what
 * the author tied on purpose, what the author marked plain, and what no
 * longer resolves. Adapted from BWrite's resolveUsages
 * (src/contexts/authoring/domain/references.ts, MPL-2.0, commit c75ead5),
 * without its rule that every annotation must fit the text: an annotation
 * that no longer fits is kept and shown unresolved with its reason.
 */
import type { FeatureReading, FeatureReferenceDocument, FeatureTermDocument } from "../../../shared/transport/index";
import { isConcept } from "../model/concept";
import type { Entry } from "../model/entry";
import { recognize } from "./recognition";

export type UseKind = "cue" | "ambiguous" | "tied" | "plain" | "unresolved";

/**
 * One use. A cue or an ambiguity is automatic and says nothing about the
 * author's intent; a tied, plain or unresolved use is an annotation the
 * author made. `entry` is the recorded entry a cue or a tie names;
 * `candidates` are the entries an ambiguity could mean.
 */
export interface Use {
  kind: UseKind;
  from: number;
  to: number;
  /** The text at the range now, or the annotation's quote when the range no longer holds it. */
  text: string;
  entry: Entry | null;
  candidates: Entry[];
  reading: FeatureReading | null;
  reference: FeatureReferenceDocument | null;
  /** Why an annotation does not resolve, in words. */
  reason: string | null;
  alias: string | null;
  /** False when the annotation's range no longer lies within the text, so it cannot be drawn. */
  placed: boolean;
}

export function termOf(entry: Entry): FeatureTermDocument {
  return { concept: entry.concept, context: entry.context, owner: entry.owner, name: entry.name };
}

export function entryOfTerm(entries: readonly Entry[], term: FeatureTermDocument): Entry | null {
  if (!isConcept(term.concept)) return null;
  return entries.find((e) => e.concept === term.concept && e.context === term.context && e.owner === term.owner && e.name === term.name) ?? null;
}

export function describeTerm(term: FeatureTermDocument): string {
  const concept = term.concept.replace(/_/g, " ");
  return term.owner === "" ? `${term.name} (${concept} in ${term.context})` : `${term.name} (${concept} of ${term.owner} in ${term.context})`;
}

function resolveReference(reference: FeatureReferenceDocument, body: string, entries: readonly Entry[]): Use {
  const base = { from: reference.from, to: reference.to, candidates: [], reading: reference.reading, reference, alias: null };
  const inRange = Number.isInteger(reference.from) && Number.isInteger(reference.to) && reference.from >= 0 && reference.to <= body.length && reference.from < reference.to;
  if (!inRange) {
    const reason = reference.from === reference.to && reference.from >= 0 && reference.from <= body.length ? "The text it annotated was removed." : "Its position lies outside the text.";
    return { ...base, kind: "unresolved", text: reference.quote, entry: null, reason, placed: false };
  }
  const now = body.slice(reference.from, reference.to);
  if (now !== reference.quote) {
    return { ...base, kind: "unresolved", text: now, entry: null, reason: `The text changed; it annotated “${reference.quote}”.`, placed: true };
  }
  if (reference.reading === "plain") {
    return { ...base, kind: "plain", text: now, entry: null, reason: null, placed: true };
  }
  if (!reference.term) {
    return { ...base, kind: "unresolved", text: now, entry: null, reason: "It names no recorded entry.", placed: true };
  }
  const entry = entryOfTerm(entries, reference.term);
  if (!entry) {
    return { ...base, kind: "unresolved", text: now, entry: null, reason: `${describeTerm(reference.term)} is not in the recording now.`, placed: true };
  }
  return { ...base, kind: "tied", text: now, entry, reason: null, placed: true };
}

/**
 * Every use in `body`, by position: the author's annotations first in
 * their own right, then the cues and ambiguities recognition finds where
 * no placed annotation covers the text. Annotations that cannot be
 * placed come last.
 */
export function usesOf(body: string, references: readonly FeatureReferenceDocument[], entries: readonly Entry[]): Use[] {
  const annotated = references.map((r) => resolveReference(r, body, entries));
  const placed = annotated.filter((u) => u.placed);
  const uses: Use[] = [...placed];
  for (const r of recognize(body, entries)) {
    if (placed.some((p) => p.from < r.to && p.to > r.from)) continue;
    const ambiguous = r.entries.length > 1;
    uses.push({
      kind: ambiguous ? "ambiguous" : "cue",
      from: r.from,
      to: r.to,
      text: r.text,
      entry: ambiguous ? null : r.entries[0],
      candidates: ambiguous ? r.entries : [],
      reading: null,
      reference: null,
      reason: null,
      alias: r.alias,
      placed: true,
    });
  }
  uses.sort((a, b) => a.from - b.from || a.to - b.to);
  return [...uses, ...annotated.filter((u) => !u.placed)];
}

/** The placed use covering `pos`, treating both ends as inside; the earliest when two touch. */
export function useAt(uses: readonly Use[], pos: number): Use | null {
  for (const u of uses) {
    if (!u.placed) continue;
    if (u.from > pos) break;
    if (pos <= u.to) return u;
  }
  return null;
}

/** The placed uses overlapping `[from, to)`. */
export function usesWithin(uses: readonly Use[], from: number, to: number): Use[] {
  return uses.filter((u) => u.placed && u.from < to && u.to > from);
}

/** The word shown beside a tied use so its reading is not told by colour alone; null when it simply names the entry. */
export function readingWord(reading: FeatureReading): string | null {
  switch (reading) {
    case "value":
      return "value";
    case "rule":
      return "rule";
    default:
      return null;
  }
}

export function describeReading(reading: FeatureReading, name: string): string {
  switch (reading) {
    case "names":
      return `Names ${name}.`;
    case "value":
      return `A value of ${name}.`;
    case "rule":
      return `States a rule of ${name}.`;
    case "plain":
      return "A plain word, not a use of the language.";
  }
}
