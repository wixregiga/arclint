import type { FeatureDocument, FeatureSummaryDocument, SaveFeatureRequest } from "./feature-documents";
import { isGatewayFailure } from "./gateway";

/**
 * Where saved feature drafts live, as Studio sees it. App composition
 * injects one separately from the gateway, so a host can lend the
 * language without lending a draft store. Nothing calls it until the
 * Features view opens.
 */
export interface FeatureStore {
  list(): Promise<FeatureSummaryDocument[]>;
  read(id: string): Promise<FeatureDocument>;
  save(request: SaveFeatureRequest): Promise<FeatureDocument>;
  /** Removes the draft as last read; a stale revision is refused and removes nothing. */
  remove(id: string, revision: string): Promise<void>;
}

/** The server refused because the draft changed since it was last read (409). */
export function isStaleRevision(error: unknown): boolean {
  return isGatewayFailure(error) && error.kind === "refused" && error.status === 409;
}

/** The server has no draft at that id any more (404). */
export function isMissingFeature(error: unknown): boolean {
  return isGatewayFailure(error) && error.kind === "refused" && error.status === 404;
}
