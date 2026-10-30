/*
 * The feature drafts the arclint map server keeps for Studio, as the
 * fixed contract in docs/studio-feature-writing.md spells them. A draft
 * is prose with annotations over the same body; the server owns ids and
 * revisions, and Studio never derives either.
 */

/**
 * How an annotated occurrence is read: as naming its term, as one value
 * of it, as stating one of its rules, or as a plain word that is not a
 * use of the language at all. A reading is the author's choice; it
 * records no contract in the domain recording.
 */
export type FeatureReading = "names" | "value" | "rule" | "plain";

export const FEATURE_READINGS: readonly FeatureReading[] = ["names", "value", "rule", "plain"];

export function isFeatureReading(value: unknown): value is FeatureReading {
  return typeof value === "string" && (FEATURE_READINGS as readonly string[]).includes(value);
}

/** The recorded entry an annotation ties to, as `arclint domain define` addresses it. */
export interface FeatureTermDocument {
  concept: string;
  context: string;
  owner: string;
  name: string;
}

/**
 * One annotation of the saved body. `from` and `to` are UTF-16 code unit
 * offsets; `quote` is the text that was selected when the annotation was
 * made. A non-plain reading carries the term it ties to; plain carries none.
 */
export interface FeatureReferenceDocument {
  id: string;
  from: number;
  to: number;
  quote: string;
  reading: FeatureReading;
  term?: FeatureTermDocument;
}

/** GET api/feature?id=ID: one saved draft. */
export interface FeatureDocument {
  id: string;
  revision: string;
  title: string;
  body: string;
  references: FeatureReferenceDocument[];
}

/** GET api/features: every saved draft, by id, title and revision, in the server's order. */
export interface FeatureSummaryDocument {
  id: string;
  title: string;
  revision: string;
}

/**
 * POST api/feature. No id creates a draft; an id with the revision last
 * read replaces it. A stale revision is refused with 409 and nothing is
 * written.
 */
export interface SaveFeatureRequest {
  id?: string;
  revision?: string;
  title: string;
  body: string;
  references: FeatureReferenceDocument[];
}
