import { useEffect, useRef, useState } from "react";

export interface CopyTicketButtonProps {
  /** Produces the ticket text at the moment of copying. */
  ticket(): string;
  disabled?: boolean;
}

type Outcome = { kind: "idle" } | { kind: "copied" } | { kind: "shown"; text: string; reason: string };

/**
 * Copies the ticket to the clipboard; when the browser refuses or has no
 * clipboard, the text is shown in a box the author can copy by hand.
 * Nothing is sent anywhere.
 */
export function CopyTicketButton({ ticket, disabled = false }: CopyTicketButtonProps) {
  const [outcome, setOutcome] = useState<Outcome>({ kind: "idle" });
  const boxRef = useRef<HTMLTextAreaElement>(null);

  useEffect(() => {
    if (outcome.kind !== "copied") return;
    const timer = setTimeout(() => setOutcome({ kind: "idle" }), 2500);
    return () => clearTimeout(timer);
  }, [outcome]);

  useEffect(() => {
    if (outcome.kind === "shown") boxRef.current?.select();
  }, [outcome]);

  async function copy() {
    const text = ticket();
    const clipboard = globalThis.navigator?.clipboard;
    if (!clipboard || typeof clipboard.writeText !== "function") {
      setOutcome({ kind: "shown", text, reason: "This browser lends no clipboard here. Copy the text below." });
      return;
    }
    try {
      await clipboard.writeText(text);
      setOutcome({ kind: "copied" });
    } catch (error) {
      const reason = error instanceof Error ? error.message : String(error);
      setOutcome({ kind: "shown", text, reason: `The clipboard refused (${reason}). Copy the text below.` });
    }
  }

  return (
    <div className="st-copy-ticket">
      <button type="button" className="st-button st-button--small" onClick={() => void copy()} disabled={disabled} title="Copy the title, prose and the language used as Markdown">
        {outcome.kind === "copied" ? "Copied" : "Copy as ticket"}
      </button>
      {outcome.kind === "copied" ? (
        <span className="st-copy-ticket__note" role="status">
          The ticket is on the clipboard.
        </span>
      ) : null}
      {outcome.kind === "shown" ? (
        <div className="st-copy-ticket__fallback" role="group" aria-label="Ticket text">
          <p className="st-note" role="status">
            {outcome.reason}
          </p>
          <textarea ref={boxRef} className="st-copy-ticket__text" readOnly aria-label="Ticket Markdown" value={outcome.text} rows={10} />
          <button type="button" className="st-button st-button--small st-button--quiet" onClick={() => setOutcome({ kind: "idle" })}>
            Close
          </button>
        </div>
      ) : null}
    </div>
  );
}
