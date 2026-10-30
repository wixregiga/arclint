import { CONCEPT_TITLE, describeTerm, readingOf, type Entry, type Use } from "../../../entities/language/index";

export interface TicketInput {
  title: string;
  body: string;
  uses: readonly Use[];
  project: string;
}

interface Listed {
  entry: Entry;
  /** How the author read each tied occurrence, when not simply by name. */
  readings: Set<string>;
}

function where(entry: Entry): string {
  if (entry.concept === "bounded_context") return "bounded context";
  const concept = CONCEPT_TITLE[entry.concept].toLowerCase();
  return entry.owner === "" ? `${concept} in ${entry.context}` : `${concept} of ${entry.owner} in ${entry.context}`;
}

function line(listed: Listed): string {
  const reading = readingOf(listed.entry).trim();
  const readings = [...listed.readings].sort();
  const suffix = readings.length ? ` Read as ${readings.join(" and ")}.` : "";
  return `- **${listed.entry.name}** (${where(listed.entry)}): ${reading === "" ? "no definition recorded yet" : reading}${suffix}`;
}

function collect(uses: readonly Use[], kind: "tied" | "cue"): Listed[] {
  const byKey = new Map<string, Listed>();
  for (const use of uses) {
    if (use.kind !== kind || !use.entry) continue;
    const listed = byKey.get(use.entry.key) ?? { entry: use.entry, readings: new Set<string>() };
    if (use.reading && use.reading !== "names") listed.readings.add(use.reading === "value" ? `a value of ${use.entry.name}` : `a rule of ${use.entry.name}`);
    byKey.set(use.entry.key, listed);
  }
  return [...byKey.values()];
}

/**
 * The feature as a ticket: its title and prose, then the language it
 * uses with each entry's context and current definition. Uses the
 * author tied on purpose are told apart from words recognition merely
 * found; unresolved and ambiguous uses are named without asserting what
 * was meant. Plain words add nothing. No status is carried: a ticket
 * says what was written, not whether it is right.
 */
export function ticketMarkdown(input: TicketInput): string {
  const title = input.title.trim() === "" ? "Untitled feature" : input.title.trim();
  const out: string[] = [`# ${title}`, ""];
  const body = input.body.replace(/\s+$/u, "");
  out.push(body === "" ? "_No prose yet._" : body, "");

  const tied = collect(input.uses, "tied");
  const cues = collect(input.uses, "cue").filter((c) => !tied.some((t) => t.entry.key === c.entry.key));
  const ambiguous = input.uses.filter((u) => u.kind === "ambiguous");
  const unresolved = input.uses.filter((u) => u.kind === "unresolved");

  if (tied.length + cues.length + ambiguous.length + unresolved.length === 0) {
    out.push(`## Language used`, "", `_No recorded term of ${input.project === "" ? "the project" : input.project} is used._`, "");
    return out.join("\n");
  }

  out.push(`## Language used`, "");
  if (tied.length) {
    out.push("### Tied on purpose", "", ...tied.map(line), "");
  }
  if (cues.length) {
    out.push("### Recognised automatically", "", "_Spellings the recording also uses; the author did not confirm them._", "", ...cues.map(line), "");
  }
  if (ambiguous.length) {
    const seen = new Set<string>();
    out.push("### Ambiguous", "");
    for (const u of ambiguous) {
      const key = u.text.toLowerCase();
      if (seen.has(key)) continue;
      seen.add(key);
      out.push(`- “${u.text}” is recorded as ${u.candidates.map((c) => `${c.name} (${where(c)})`).join(" and ")}; which is meant is not chosen.`);
    }
    out.push("");
  }
  if (unresolved.length) {
    out.push("### Unresolved", "");
    for (const u of unresolved) {
      const target = u.reference?.term ? ` tied to ${describeTerm(u.reference.term)}` : "";
      out.push(`- “${u.reference?.quote ?? u.text}”${target}: ${u.reason ?? "does not resolve."}`);
    }
    out.push("");
  }
  return out.join("\n");
}
