import { useCallback, useEffect, useRef, useState } from "react";
import { NOT_IN_SNAPSHOT_STATUS, STALE_SNAPSHOT_STATUS, WITHHELD_STATUS, isGatewayFailure, type CodeEvidenceSource, type ExcerptDocument } from "../../../shared/transport/index";
import { failureText } from "./use-language";

/** One location the reader opened, with what reading it answered. */
export interface Peek {
  path: string;
  line: number;
  snapshot: string;
  status: "loading" | "ready" | "stale" | "withheld" | "gone" | "failed";
  excerpt: ExcerptDocument | null;
  message: string | null;
  /**
   * Counts the reader's opens. The peek takes focus when it changes, so
   * opening the same location again focuses it again, while a reading
   * against a newer snapshot keeps the number and leaves focus alone.
   */
  request: number;
}

/**
 * The read-only source peek below the map: one location at a time, read
 * against the snapshot of the evidence shown. When a newer assessment
 * replaces the snapshot, the open location is read again against it; a
 * location the newer assessment no longer analyzes says so instead of
 * showing the old lines.
 */
export function useSourcePeek(source: CodeEvidenceSource | null, snapshot: string | null, onStale: (reason: string) => void) {
  const [peek, setPeek] = useState<Peek | null>(null);
  const generation = useRef(0);
  const requests = useRef(0);

  const read = useCallback(
    async (path: string, line: number, at: string, request: number) => {
      if (!source) return;
      const mine = ++generation.current;
      setPeek((p) => ({ path, line, snapshot: at, status: "loading", excerpt: p && p.path === path && p.line === line ? p.excerpt : null, message: null, request }));
      try {
        const excerpt = await source.excerpt({ snapshot: at, path, line });
        if (mine !== generation.current) return;
        setPeek({ path, line, snapshot: at, status: "ready", excerpt, message: null, request });
      } catch (error) {
        if (mine !== generation.current) return;
        const message = failureText(error);
        let status: Peek["status"] = "failed";
        if (isGatewayFailure(error) && error.status === STALE_SNAPSHOT_STATUS) {
          status = "stale";
          onStale("A newer assessment replaced this evidence on the server.");
        } else if (isGatewayFailure(error) && error.kind === "refused" && error.status === WITHHELD_STATUS) {
          status = "withheld";
        } else if (isGatewayFailure(error) && error.kind === "refused" && error.status === NOT_IN_SNAPSHOT_STATUS) {
          status = "gone";
        }
        setPeek({ path, line, snapshot: at, status, excerpt: null, message, request });
      }
    },
    [source, onStale],
  );

  const open = useCallback((path: string, line: number) => {
    if (snapshot) void read(path, line, snapshot, ++requests.current);
  }, [read, snapshot]);

  const close = useCallback(() => {
    generation.current++;
    setPeek(null);
  }, []);

  // A newer assessment: read the open location again against it.
  const shown = peek ? `${peek.path}\u001f${peek.line}\u001f${peek.snapshot}` : null;
  useEffect(() => {
    if (!peek || !snapshot || peek.snapshot === snapshot) return;
    void read(peek.path, peek.line, snapshot, peek.request);
    // Only a new snapshot reads again; the open location is in `shown`.
  }, [snapshot, shown]);

  return { peek, open, close };
}
