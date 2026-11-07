import { useEffect, useRef, type KeyboardEvent } from "react";
import type { Peek } from "../model/use-source-peek";
import { PathText } from "./CodeEvidence";

export interface SourcePeekProps {
  peek: Peek;
  onClose(): void;
}

const STATUS_TEXT: Record<Peek["status"], string> = {
  loading: "Reading…",
  ready: "",
  stale: "A newer assessment replaced the evidence this location came from. Assess again to read it.",
  withheld: "Withheld: this file's name or content says it may hold a secret, so no line of it is shown.",
  gone: "This location is not part of the assessment shown; the file was not analyzed in it.",
  failed: "The excerpt could not be read.",
};

/**
 * A few lines around one evidence location, below the map, as the file is
 * on disk now: the assessment kept no copy, so the peek always says the
 * lines may differ from the assessed code, an equal size included, and
 * adds that the file changed when its size did. Read-only: the text is shown as text, never as
 * markup, and nothing can be edited.
 * It takes focus when opened, so Escape closes it from there.
 */
export function SourcePeek({ peek, onClose }: SourcePeekProps) {
  const region = useRef<HTMLElement>(null);
  const where = `${peek.path}:${peek.line}`;

  // Every open by the reader focuses the peek, the same location again
  // included; a reading against a newer snapshot keeps the request and
  // does not move focus.
  useEffect(() => {
    region.current?.focus({ preventScroll: true });
  }, [peek.request]);

  useEffect(() => {
    if (peek.status !== "ready") return;
    // Scroll the lines alone: scrollIntoView would also scroll the map
    // and the page around them.
    const code = region.current?.querySelector<HTMLElement>(".st-peek__code");
    const target = code?.querySelector<HTMLElement>(".st-peek__line.is-target");
    if (code && target) code.scrollTop = Math.max(0, target.offsetTop - (code.clientHeight - target.offsetHeight) / 2);
  }, [peek.status, peek.excerpt]);

  function onKeyDown(e: KeyboardEvent<HTMLElement>) {
    if (e.key === "Escape") {
      e.preventDefault();
      e.stopPropagation();
      onClose();
    }
  }

  const excerpt = peek.excerpt;
  return (
    <section ref={region} className="st-peek" aria-label={`Source peek: ${where}`} tabIndex={-1} onKeyDown={onKeyDown} aria-busy={peek.status === "loading"}>
      <header className="st-peek__head">
        <div className="st-peek__row">
          <h2 className="st-peek__path" title={where}>
            <PathText path={peek.path} line={peek.line} />
          </h2>
          <span className="st-evidence__readonly">read-only</span>
          <button type="button" className="st-button st-button--small" onClick={onClose} aria-label="Close the source peek">
            Close
          </button>
        </div>
        <div className="st-peek__row st-peek__meta">
          {excerpt ? (
            <span className="st-peek__range">
              lines {excerpt.lines[0]?.number ?? peek.line}–{excerpt.lines[excerpt.lines.length - 1]?.number ?? peek.line} of {excerpt.totalLines}
            </span>
          ) : null}
          {excerpt ? (
            <span className="st-peek__disk" role="note">
              Current file on disk; it may differ from the assessed code.
            </span>
          ) : null}
          {excerpt?.changedSinceAssessment ? (
            <span className="st-peek__disk is-changed" role="note">
              Its size changed since the assessment.
            </span>
          ) : null}
        </div>
      </header>
      {peek.status !== "ready" ? (
        <p className={peek.status === "loading" ? "st-note st-peek__status" : "st-peek__status st-evidence__stale"} role={peek.status === "loading" ? undefined : "status"}>
          {STATUS_TEXT[peek.status]}
          {peek.message && peek.status === "failed" ? ` ${peek.message}` : null}
        </p>
      ) : null}
      {excerpt ? (
        <div className="st-peek__code" role="group" aria-label={`Lines of ${peek.path}`}>
          {excerpt.lines.map((l) => (
            <div key={l.number} className={l.number === excerpt.line ? "st-peek__line is-target" : "st-peek__line"} aria-current={l.number === excerpt.line ? "true" : undefined}>
              <span className="st-peek__number" aria-hidden="true">
                {l.number}
              </span>
              <span className="st-peek__text">
                {l.text}
                {l.truncated ? <span className="st-peek__cut" title="The line continues; the excerpt shows its start"> …</span> : null}
              </span>
            </div>
          ))}
        </div>
      ) : null}
    </section>
  );
}
