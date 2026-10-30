import { useCallback, useEffect, useRef, useState } from "react";
import { isGatewayFailure, isMissingFeature, isStaleRevision, type FeatureDocument, type FeatureStore, type FeatureSummaryDocument, type SaveFeatureRequest } from "../../../shared/transport/index";
import { failureText } from "./use-language";

export interface FeatureListState {
  status: "idle" | "loading" | "ready" | "failed";
  features: FeatureSummaryDocument[];
  error: string | null;
}

export type LoadedFeature =
  | { status: "loading"; document: FeatureDocument | null }
  | { status: "ready"; document: FeatureDocument }
  | { status: "missing"; document: null }
  | { status: "failed"; document: FeatureDocument | null; error: string };

/** What a save came to: through, refused because the store holds a newer revision, or not at all. */
export type SaveOutcome =
  | { kind: "saved"; document: FeatureDocument }
  | { kind: "stale"; current: FeatureDocument | null }
  | { kind: "missing" }
  | { kind: "failed"; message: string; retryable: boolean };

export type RemoveOutcome = { kind: "removed" } | { kind: "stale"; current: FeatureDocument | null } | { kind: "missing" } | { kind: "failed"; message: string; retryable: boolean };

export interface FeatureStoreState {
  list: FeatureListState;
  loaded: Record<string, LoadedFeature>;
  refresh(): Promise<void>;
  load(id: string): Promise<void>;
  save(request: SaveFeatureRequest): Promise<SaveOutcome>;
  remove(id: string, revision: string): Promise<RemoveOutcome>;
}

const NO_STORE: FeatureListState = { status: "idle", features: [], error: null };

/**
 * The saved features as the store holds them. Nothing is read until
 * `active` is first true, so a mount that never opens Features makes no
 * request. Saved documents are authoritative; the page keeps only the
 * author's typing over them.
 */
export function useFeatureStore(store: FeatureStore | null, active: boolean): FeatureStoreState {
  const [list, setList] = useState<FeatureListState>(NO_STORE);
  const [loaded, setLoaded] = useState<Record<string, LoadedFeature>>({});
  const opened = useRef(false);

  const refresh = useCallback(async () => {
    if (!store) return;
    setList((s) => ({ ...s, status: s.status === "ready" ? "ready" : "loading", error: null }));
    try {
      const features = await store.list();
      setList({ status: "ready", features, error: null });
    } catch (error) {
      setList((s) => ({ ...s, status: s.status === "ready" ? "ready" : "failed", error: failureText(error) }));
    }
  }, [store]);

  const load = useCallback(
    async (id: string) => {
      if (!store) return;
      setLoaded((l) => ({ ...l, [id]: { status: "loading", document: l[id]?.document ?? null } }));
      try {
        const document = await store.read(id);
        setLoaded((l) => ({ ...l, [id]: { status: "ready", document } }));
      } catch (error) {
        if (isMissingFeature(error)) setLoaded((l) => ({ ...l, [id]: { status: "missing", document: null } }));
        else setLoaded((l) => ({ ...l, [id]: { status: "failed", document: l[id]?.document ?? null, error: failureText(error) } }));
      }
    },
    [store],
  );

  const readCurrent = useCallback(
    async (id: string): Promise<FeatureDocument | null> => {
      if (!store) return null;
      try {
        const document = await store.read(id);
        setLoaded((l) => ({ ...l, [id]: { status: "ready", document } }));
        return document;
      } catch (error) {
        if (isMissingFeature(error)) setLoaded((l) => ({ ...l, [id]: { status: "missing", document: null } }));
        return null;
      }
    },
    [store],
  );

  const save = useCallback(
    async (request: SaveFeatureRequest): Promise<SaveOutcome> => {
      if (!store) return { kind: "failed", message: "This host lends no feature store.", retryable: false };
      try {
        const document = await store.save(request);
        setLoaded((l) => ({ ...l, [document.id]: { status: "ready", document } }));
        void refresh();
        return { kind: "saved", document };
      } catch (error) {
        if (isStaleRevision(error) && request.id) return { kind: "stale", current: await readCurrent(request.id) };
        if (isMissingFeature(error)) {
          if (request.id) setLoaded((l) => ({ ...l, [request.id!]: { status: "missing", document: null } }));
          void refresh();
          return { kind: "missing" };
        }
        return { kind: "failed", message: failureText(error), retryable: isGatewayFailure(error) && error.kind !== "refused" };
      }
    },
    [store, refresh, readCurrent],
  );

  const remove = useCallback(
    async (id: string, revision: string): Promise<RemoveOutcome> => {
      if (!store) return { kind: "failed", message: "This host lends no feature store.", retryable: false };
      try {
        await store.remove(id, revision);
        setLoaded((l) => {
          const out = { ...l };
          delete out[id];
          return out;
        });
        void refresh();
        return { kind: "removed" };
      } catch (error) {
        if (isStaleRevision(error)) return { kind: "stale", current: await readCurrent(id) };
        if (isMissingFeature(error)) {
          void refresh();
          return { kind: "missing" };
        }
        return { kind: "failed", message: failureText(error), retryable: isGatewayFailure(error) && error.kind !== "refused" };
      }
    },
    [store, refresh, readCurrent],
  );

  useEffect(() => {
    if (!active || opened.current) return;
    opened.current = true;
    void refresh();
  }, [active, refresh]);

  return { list, loaded, refresh, load, save, remove };
}
