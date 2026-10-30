import { aliasesOf, readingOf, textOf, type Entry } from "../model/entry";

export type MatchedBy = "name" | "alias" | "reading" | "property";

export interface SearchHit {
  entry: Entry;
  matchedBy: MatchedBy;
  /** The alias that matched, when one did. */
  alias: string | null;
  /** The reading around the match, when the reading matched. */
  snippet: string | null;
}

const LIMIT = 60;

function rank(hit: SearchHit, q: string): number {
  const name = hit.entry.name.toLowerCase();
  if (hit.matchedBy === "name") return name === q ? 0 : name.startsWith(q) ? 1 : 2;
  if (hit.matchedBy === "alias") return (hit.alias ?? "").toLowerCase().startsWith(q) ? 3 : 4;
  if (hit.matchedBy === "reading") return 5;
  return 6;
}

function snippetAround(text: string, q: string): string {
  const at = text.toLowerCase().indexOf(q);
  if (at < 0) return text.slice(0, 80);
  const start = Math.max(0, at - 36);
  const end = Math.min(text.length, at + q.length + 44);
  return (start > 0 ? "…" : "") + text.slice(start, end) + (end < text.length ? "…" : "");
}

/**
 * The entries a query finds, across every context: by name first, then by
 * alias, then by the words of their reading, then by any other recorded
 * property. An empty query finds nothing; the navigator lists everything.
 */
export function searchEntries(entries: readonly Entry[], query: string): SearchHit[] {
  const q = query.trim().toLowerCase();
  if (q === "") return [];
  const hits: SearchHit[] = [];
  for (const entry of entries) {
    if (entry.name.toLowerCase().includes(q)) {
      hits.push({ entry, matchedBy: "name", alias: null, snippet: null });
      continue;
    }
    const alias = aliasesOf(entry).find((a) => a.toLowerCase().includes(q));
    if (alias !== undefined) {
      hits.push({ entry, matchedBy: "alias", alias, snippet: null });
      continue;
    }
    const reading = readingOf(entry);
    if (reading.toLowerCase().includes(q)) {
      hits.push({ entry, matchedBy: "reading", alias: null, snippet: snippetAround(reading, q) });
      continue;
    }
    const other = [textOf(entry, "identity"), textOf(entry, "on"), textOf(entry, "raisedBy"), entry.owner, entry.context].join(" ").toLowerCase();
    if (other.includes(q)) hits.push({ entry, matchedBy: "property", alias: null, snippet: null });
  }
  return hits
    .map((hit, index) => ({ hit, index, r: rank(hit, q) }))
    .sort((a, b) => a.r - b.r || a.index - b.index)
    .slice(0, LIMIT)
    .map((x) => x.hit);
}
