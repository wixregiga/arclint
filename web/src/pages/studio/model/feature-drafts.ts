import { isFeatureReading, type FeatureDocument, type FeatureReferenceDocument } from "../../../shared/transport/index";

/**
 * Which feature the Features view shows: a saved one by the server's id,
 * or one not yet saved, told apart by a key the page minted. A saved
 * feature's text is authoritative in the store; a draft is only what
 * the author typed since, and the page keeps it until a save goes
 * through or the author discards it.
 */
export type FeatureSelection = { kind: "saved"; id: string } | { kind: "new"; key: string };

export interface FeatureDraft {
  title: string;
  body: string;
  references: FeatureReferenceDocument[];
  /** The revision the draft was typed over; null for a feature not saved yet. */
  revision: string | null;
}

export function featureDraftKey(selection: FeatureSelection): string {
  return selection.kind === "saved" ? `saved\u001f${selection.id}` : `new\u001f${selection.key}`;
}

export function newFeatureKey(): string {
  return `${Date.now().toString(36)}-${Math.random().toString(36).slice(2, 8)}`;
}

export function draftFromDocument(document: FeatureDocument): FeatureDraft {
  return { title: document.title, body: document.body, references: document.references, revision: document.revision };
}

export function sameText(draft: FeatureDraft, document: FeatureDocument): boolean {
  return draft.title === document.title && draft.body === document.body &&
    draft.references.length === document.references.length && draft.references.every((reference, index) => {
      const saved = document.references[index];
      return reference.id === saved.id && reference.from === saved.from && reference.to === saved.to &&
        reference.quote === saved.quote && reference.reading === saved.reading &&
        reference.term?.concept === saved.term?.concept && reference.term?.context === saved.term?.context &&
        reference.term?.owner === saved.term?.owner && reference.term?.name === saved.term?.name;
    });
}

export function restoreFeatureSelection(raw: unknown): FeatureSelection | null {
  if (!raw || typeof raw !== "object") return null;
  const s = raw as { kind?: unknown; id?: unknown; key?: unknown };
  if (s.kind === "saved" && typeof s.id === "string") return { kind: "saved", id: s.id };
  if (s.kind === "new" && typeof s.key === "string") return { kind: "new", key: s.key };
  return null;
}

export function restoreReference(raw: unknown): FeatureReferenceDocument | null {
  if (!raw || typeof raw !== "object") return null;
  const r = raw as Record<string, unknown>;
  if (typeof r.id !== "string" || typeof r.from !== "number" || typeof r.to !== "number" || typeof r.quote !== "string" || !isFeatureReading(r.reading)) return null;
  const out: FeatureReferenceDocument = { id: r.id, from: r.from, to: r.to, quote: r.quote, reading: r.reading };
  if (r.term && typeof r.term === "object") {
    const t = r.term as Record<string, unknown>;
    if (typeof t.concept === "string" && typeof t.context === "string" && typeof t.owner === "string" && typeof t.name === "string") {
      out.term = { concept: t.concept, context: t.context, owner: t.owner, name: t.name };
    }
  }
  return out;
}

export function restoreFeatureDrafts(raw: unknown): Record<string, FeatureDraft> {
  if (!raw || typeof raw !== "object") return {};
  const out: Record<string, FeatureDraft> = {};
  for (const [key, value] of Object.entries(raw as Record<string, unknown>)) {
    if (!value || typeof value !== "object") continue;
    const d = value as Record<string, unknown>;
    if (typeof d.title !== "string" || typeof d.body !== "string") continue;
    const references = Array.isArray(d.references) ? d.references.map(restoreReference).filter((r): r is FeatureReferenceDocument => r !== null) : [];
    out[key] = { title: d.title, body: d.body, references, revision: typeof d.revision === "string" ? d.revision : null };
  }
  return out;
}
