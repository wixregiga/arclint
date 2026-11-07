import { useCallback, useRef, useState } from "react";
import { isGatewayFailure, type CodeEvidenceSource } from "../../../shared/transport/index";
import { NO_EVIDENCE, NOT_ASSESSED, type EvidenceState } from "./code-evidence";
import { failureText } from "./use-language";

/**
 * The code evidence, assessed only when the reader asks. A newer request
 * wins over a late older answer. A failed assessment keeps the last
 * document, marked stale with the reason; so does a change Studio made
 * or a reload, since the recording, and perhaps the code, changed after
 * the assessment.
 */
export function useCodeEvidence(source: CodeEvidenceSource | null): {
  evidence: EvidenceState;
  assess(): Promise<void>;
  markStale(reason: string): void;
} {
  const [evidence, setEvidence] = useState<EvidenceState>(source ? NOT_ASSESSED : NO_EVIDENCE);
  const generation = useRef(0);

  const assess = useCallback(async () => {
    if (!source) return;
    const mine = ++generation.current;
    setEvidence((s) => ({ ...s, status: "assessing", error: null }));
    try {
      const document = await source.assess();
      if (mine !== generation.current) return;
      setEvidence({ status: "ready", document, stale: null, error: null });
    } catch (error) {
      if (mine !== generation.current) return;
      if (isGatewayFailure(error) && error.kind === "unexpected" && error.status === 404) {
        setEvidence({ status: "unavailable", document: null, stale: null, error: "This map server serves no code evidence." });
        return;
      }
      const message = failureText(error);
      setEvidence((s) => ({
        status: "failed",
        document: s.document,
        stale: s.document ? "The last assessment failed, so this evidence was not refreshed." : null,
        error: message,
      }));
    }
  }, [source]);

  const markStale = useCallback((reason: string) => {
    setEvidence((s) => (s.document && !s.stale ? { ...s, stale: reason } : s));
  }, []);

  return { evidence, assess, markStale };
}
