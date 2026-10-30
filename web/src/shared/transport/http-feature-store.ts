import type { FeatureDocument, FeatureSummaryDocument } from "./feature-documents";
import type { FeatureStore } from "./feature-store";
import { httpCaller } from "./http-call";

export interface HttpFeatureStoreOptions {
  /** The absolute URL every API path is relative to; it ends with a slash. */
  base: string;
  fetch?: typeof fetch;
}

/** The API paths the map server answers for feature drafts, relative to the injected base. */
export const FEATURE_API_PATHS = {
  features: "api/features",
  feature: "api/feature",
} as const;

function featurePath(id: string): string {
  return `${FEATURE_API_PATHS.feature}?id=${encodeURIComponent(id)}`;
}

/** The feature store over HTTP, as the Go map server serves it under the same base as the language. */
export function createHttpFeatureStore(options: HttpFeatureStoreOptions): FeatureStore {
  const { call, post } = httpCaller(options.base, options.fetch);
  return {
    list: async () => (await call<{ features: FeatureSummaryDocument[] }>(FEATURE_API_PATHS.features)).features ?? [],
    read: async (id) => (await call<{ feature: FeatureDocument }>(featurePath(id))).feature,
    save: async (request) => (await post<{ feature: FeatureDocument }>(FEATURE_API_PATHS.feature, request)).feature,
    remove: async (id, revision) => {
      await call<{ removed: string }>(featurePath(id), { method: "DELETE", headers: { "If-Match": revision } });
    },
  };
}
