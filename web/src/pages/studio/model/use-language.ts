import { useCallback, useEffect, useRef, useState } from "react";
import { isGatewayFailure, type AcceptedDocument, type OverviewDocument, type StudioGateway } from "../../../shared/transport/index";

/** The recording as last read: loading, ready, or failed with why. */
export interface LanguageState {
  status: "loading" | "ready" | "failed";
  overview: OverviewDocument | null;
  accepted: AcceptedDocument | null;
  error: string | null;
}

export function failureText(error: unknown): string {
  if (isGatewayFailure(error)) return error.message;
  return error instanceof Error ? error.message : String(error);
}

/**
 * Reads the overview and the accepted recording through the gateway, and
 * reads them again on request: after every write, and when the reader
 * asks. A failed read keeps the last recording shown and says why.
 */
export function useLanguage(gateway: StudioGateway): { language: LanguageState; reload(): Promise<boolean> } {
  const [language, setLanguage] = useState<LanguageState>({ status: "loading", overview: null, accepted: null, error: null });
  const generation = useRef(0);

  const reload = useCallback(async (): Promise<boolean> => {
    const mine = ++generation.current;
    setLanguage((s) => ({ ...s, status: s.overview ? s.status : "loading", error: null }));
    const [overview, accepted] = await Promise.allSettled([gateway.overview(), gateway.accepted()]);
    if (mine !== generation.current) return false;
    if (overview.status === "rejected") {
      setLanguage((s) => ({ ...s, status: "failed", error: failureText(overview.reason) }));
      return false;
    }
    setLanguage({
      status: "ready",
      overview: overview.value,
      accepted: accepted.status === "fulfilled" ? accepted.value : { available: false, reason: failureText(accepted.reason) },
      error: null,
    });
    return true;
  }, [gateway]);

  useEffect(() => {
    void reload();
    return () => {
      generation.current++;
    };
  }, [reload]);

  return { language, reload };
}
