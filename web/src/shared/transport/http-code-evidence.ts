import type { CodeEvidenceSource } from "./code-evidence";
import type { EvidenceDocument, ExcerptDocument, ExcerptRequest } from "./evidence-documents";
import { httpCaller } from "./http-call";

export interface HttpCodeEvidenceOptions {
  /** The absolute URL every API path is relative to; it ends with a slash. */
  base: string;
  fetch?: typeof fetch;
}

/** The API paths of the code evidence, relative to the injected base. */
export const EVIDENCE_API_PATHS = {
  assess: "api/evidence",
  source: "api/source",
} as const;

/**
 * The header every evidence request carries. A page on another site
 * cannot send it without the server allowing it first, which it never
 * does, so only Studio's own page makes the server assess or read.
 */
export const STUDIO_REQUEST_HEADER = "X-ArcLint-Studio";

/** The code evidence over HTTP, as the Go map server serves it. */
export function createHttpCodeEvidence(options: HttpCodeEvidenceOptions): CodeEvidenceSource {
  const { call } = httpCaller(options.base, options.fetch);
  const init: RequestInit = { headers: { [STUDIO_REQUEST_HEADER]: "1" } };
  return {
    assess: () => call<EvidenceDocument>(EVIDENCE_API_PATHS.assess, init),
    excerpt: (request: ExcerptRequest) => {
      const query = new URLSearchParams({ snapshot: request.snapshot, path: request.path, line: String(request.line) });
      return call<ExcerptDocument>(`${EVIDENCE_API_PATHS.source}?${query.toString()}`, init);
    },
  };
}
