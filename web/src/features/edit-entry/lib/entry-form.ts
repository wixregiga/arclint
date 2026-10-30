import { propertiesOf, type Concept, type Entry, type EntryAddress, type PropertySpec } from "../../../entities/language/index";
import type { ChangeRequest, DefineRequest, RemoveRequest } from "../../../shared/transport/index";

/**
 * What a form holds: every property as text, a list one value per line,
 * and the name of a new entry under NAME. Held as text so an unsaved form
 * survives a view switch or a refresh exactly as typed.
 */
export type DraftValues = Record<string, string>;

export const NAME = "$name";

export function draftOf(concept: Concept, recorded: Entry | null): DraftValues {
  const values: DraftValues = { [NAME]: recorded?.name ?? "" };
  for (const p of propertiesOf(concept)) {
    const v = recorded?.properties[p.name];
    values[p.name] = Array.isArray(v) ? v.join("\n") : (v ?? "");
  }
  return values;
}

function parsed(spec: PropertySpec, text: string | undefined): string | string[] {
  const t = text ?? "";
  if (spec.shape === "list") {
    return t
      .split("\n")
      .map((s) => s.trim())
      .filter((s) => s !== "");
  }
  return t.trim();
}

function sameValue(a: string | string[] | undefined, b: string | string[]): boolean {
  const as = Array.isArray(a) ? a.join("\n") : (a ?? "");
  const bs = Array.isArray(b) ? b.join("\n") : b;
  return as === bs;
}

/**
 * The change a filled form makes: for a recorded entry, every property
 * whose value differs from what is recorded, parsed back; for a new
 * entry, every property that is filled. A property left as recorded is
 * absent from the change, so the server leaves it as it is.
 */
export function changeFrom(concept: Concept, recorded: Entry | null, values: DraftValues): ChangeRequest {
  const change: ChangeRequest = {};
  for (const spec of propertiesOf(concept)) {
    const value = parsed(spec, values[spec.name]);
    if (recorded === null) {
      if (Array.isArray(value) ? value.length > 0 : value !== "") (change as Record<string, unknown>)[spec.name] = value;
      continue;
    }
    if (!sameValue(recorded.properties[spec.name], value)) (change as Record<string, unknown>)[spec.name] = value;
  }
  return change;
}

export function hasChanges(concept: Concept, recorded: Entry | null, values: DraftValues): boolean {
  if (recorded === null && (values[NAME] ?? "").trim() !== "") return true;
  return Object.keys(changeFrom(concept, recorded, values)).length > 0;
}

/** The required properties a new entry still lacks; the language would refuse it. */
export function missingIn(concept: Concept, values: DraftValues): PropertySpec[] {
  return propertiesOf(concept).filter((spec) => {
    const value = parsed(spec, values[spec.name]);
    return spec.required && (Array.isArray(value) ? value.length === 0 : value === "");
  });
}

export function defineRequestFrom(address: Omit<EntryAddress, "name">, recorded: Entry | null, values: DraftValues): DefineRequest {
  return {
    concept: address.concept,
    context: address.concept === "bounded_context" ? "" : address.context,
    owner: address.owner,
    name: recorded ? recorded.name : (values[NAME] ?? "").trim(),
    change: changeFrom(address.concept, recorded, values),
  };
}

export function removeRequestFrom(entry: Entry): RemoveRequest {
  return { concept: entry.concept, context: entry.concept === "bounded_context" ? "" : entry.context, owner: entry.owner, name: entry.name };
}
