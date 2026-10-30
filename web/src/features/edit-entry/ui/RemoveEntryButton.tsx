import { useState } from "react";
import { CONCEPT_TITLE, type Entry } from "../../../entities/language/index";

export interface RemoveEntryButtonProps {
  entry: Entry;
  /** What else goes with it, in words, when the entry holds other entries. */
  carries: readonly string[];
  busy: boolean;
  error: string | null;
  onRemove(): void;
}

/** Removal in two deliberate steps: ask, then confirm in the entry's own name. */
export function RemoveEntryButton({ entry, carries, busy, error, onRemove }: RemoveEntryButtonProps) {
  const [asked, setAsked] = useState(false);
  if (!asked) {
    return (
      <button type="button" className="st-button st-button--quiet" disabled={busy} onClick={() => setAsked(true)}>
        Remove…
      </button>
    );
  }
  return (
    <div className="st-confirm" role="group" aria-label={`Remove ${entry.name}`}>
      <p>
        Remove the {CONCEPT_TITLE[entry.concept].toLowerCase()} <b>{entry.name}</b> from the recording?
        {carries.length > 0 ? ` What it holds goes with it: ${carries.join(", ")}.` : ""}
      </p>
      {error ? (
        <p className="st-form-error" role="alert">
          {error}
        </p>
      ) : null}
      <div className="st-form-actions">
        <button type="button" className="st-button st-button--danger" disabled={busy} onClick={onRemove}>
          {busy ? "Removing…" : `Remove ${entry.name}`}
        </button>
        <button type="button" className="st-button" disabled={busy} onClick={() => setAsked(false)}>
          Keep
        </button>
      </div>
    </div>
  );
}
