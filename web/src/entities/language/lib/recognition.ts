/*
 * Finding the recorded language in prose. Adapted from BWrite
 * (src/contexts/authoring/domain/terms.ts, MPL-2.0, commit c75ead5):
 * whole-word, case-insensitive matching of names and aliases with
 * Unicode word boundaries, the longest match winning. Where BWrite left a
 * phrase out when it named terms in several contexts, Studio reports the
 * occurrence as ambiguous with every candidate, so the author chooses.
 */
import { aliasesOf, type Entry } from "../model/entry";
import type { Concept } from "../model/concept";

/** The concepts whose names and aliases prose may name; rules and questions are tied on purpose, never guessed. */
export const RECOGNIZED_CONCEPTS: readonly Concept[] = ["bounded_context", "aggregate", "entity", "value_object", "domain_event", "domain_service", "specification"];

/**
 * One place the text spells a recorded name or alias. One entry is a cue
 * for it; several entries spelled the same are an ambiguity the author
 * resolves by tying the occurrence on purpose.
 */
export interface Recognition {
  from: number;
  to: number;
  text: string;
  entries: Entry[];
  /** The alias that matched, when the spelling is an alias of every candidate. */
  alias: string | null;
}

/** Collapses whitespace and trims; the display form of a name. */
export function cleanName(name: string): string {
  return name.normalize("NFC").trim().replace(/\s+/gu, " ");
}

/** The comparison key of a spelling: cleaned and lowercased. */
export function normalizeName(name: string): string {
  return cleanName(name).toLowerCase();
}

const WORD_CHAR = "[\\p{L}\\p{M}\\p{N}_]";
const HAS_WORD_CHAR = new RegExp(WORD_CHAR, "u");

export function hasWordCharacter(text: string): boolean {
  return HAS_WORD_CHAR.test(text);
}

function escapeRegExp(text: string): string {
  return text.replace(/[.*+?^${}()|[\]\\]/g, "\\$&");
}

interface Phrase {
  key: string;
  pattern: RegExp;
  entries: Entry[];
  /** True when the spelling is an alias of every entry it names. */
  alias: boolean;
}

function compile(entries: readonly Entry[]): Phrase[] {
  const byKey = new Map<string, { spelling: string; entries: Entry[]; names: number }>();
  for (const entry of entries) {
    if (!RECOGNIZED_CONCEPTS.includes(entry.concept)) continue;
    const spellings: [string, boolean][] = [[entry.name, true], ...aliasesOf(entry).map((a): [string, boolean] => [a, false])];
    for (const [spelling, isName] of spellings) {
      const clean = cleanName(spelling);
      if (!hasWordCharacter(clean)) continue;
      const key = clean.toLowerCase();
      const phrase = byKey.get(key) ?? { spelling: clean, entries: [], names: 0 };
      if (!phrase.entries.some((e) => e.key === entry.key)) phrase.entries.push(entry);
      if (isName) phrase.names++;
      byKey.set(key, phrase);
    }
  }
  return [...byKey.entries()].map(([key, p]) => ({
    key,
    // Words may be separated by any run of horizontal whitespace, never a line break.
    pattern: new RegExp(`(?<!${WORD_CHAR})(?:${p.spelling.split(" ").map(escapeRegExp).join("[^\\S\\r\\n]+")})(?!${WORD_CHAR})`, "giu"),
    entries: p.entries,
    alias: p.names === 0,
  }));
}

const compiled = new WeakMap<readonly Entry[], Phrase[]>();

function phrasesOf(entries: readonly Entry[]): Phrase[] {
  let phrases = compiled.get(entries);
  if (!phrases) {
    phrases = compile(entries);
    compiled.set(entries, phrases);
  }
  return phrases;
}

/**
 * Every occurrence of a recorded name or alias in `text`, as UTF-16
 * offsets in ascending order. Overlapping candidates resolve to the
 * longest, then the earliest; no occurrence spans a line break.
 */
export function recognize(text: string, entries: readonly Entry[]): Recognition[] {
  const phrases = phrasesOf(entries);
  if (phrases.length === 0 || text === "") return [];
  const candidates: Recognition[] = [];
  for (const phrase of phrases) {
    phrase.pattern.lastIndex = 0;
    for (let m = phrase.pattern.exec(text); m; m = phrase.pattern.exec(text)) {
      candidates.push({ from: m.index, to: m.index + m[0].length, text: m[0], entries: phrase.entries, alias: phrase.alias ? m[0] : null });
      if (m[0].length === 0) phrase.pattern.lastIndex++;
    }
  }
  candidates.sort((a, b) => b.to - b.from - (a.to - a.from) || a.from - b.from);
  const chosen: Recognition[] = [];
  for (const c of candidates) {
    if (chosen.every((k) => c.to <= k.from || c.from >= k.to)) chosen.push(c);
  }
  return chosen.sort((a, b) => a.from - b.from);
}
