import type { EvidenceDocument, ExcerptDocument, ExcerptRequest } from "./evidence-documents";

/**
 * Read-only code evidence, lent apart from the language gateway: a host
 * may serve the language without it, and Studio then says the evidence
 * is unavailable. `assess` runs one assessment of the repository when
 * the reader asks; `excerpt` reads a few lines around one location of
 * one assessment's snapshot.
 */
export interface CodeEvidenceSource {
  assess(): Promise<EvidenceDocument>;
  excerpt(request: ExcerptRequest): Promise<ExcerptDocument>;
}

/** The server answered that the snapshot an excerpt names was replaced by newer assessments. */
export const STALE_SNAPSHOT_STATUS = 409;
/** The server withheld the excerpt: the file's name or content says it may hold a secret. */
export const WITHHELD_STATUS = 403;
/** The snapshot did not analyze the file the excerpt names as source. */
export const NOT_IN_SNAPSHOT_STATUS = 404;
