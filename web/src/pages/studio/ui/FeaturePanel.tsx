import { useId, useState, type Ref } from "react";
import type { Entry, Use } from "../../../entities/language/index";
import { FeatureEditor, type EditorSelection, type FeatureEditorHandle } from "../../../features/write-feature/index";
import type { FeatureReferenceDocument } from "../../../shared/transport/index";
import { Badge } from "../../../shared/ui/Badge";
import type { FeatureSelection } from "../model/feature-drafts";
import type { FeatureSaveState } from "../model/save-state";

export interface FeaturePanelProps {
  selection: FeatureSelection;
  /** Remounts the editor when it changes: another feature, or a draft discarded for the stored text. */
  editorKey: string;
  title: string;
  body: string;
  references: readonly FeatureReferenceDocument[];
  entries: readonly Entry[];
  dirty: boolean;
  storeAvailable: boolean;
  saveState: FeatureSaveState;
  removeError: string | null;
  busy: boolean;
  editorRef: Ref<FeatureEditorHandle>;
  onTitle(title: string): void;
  onEdit(body: string, references: FeatureReferenceDocument[]): void;
  onSelectionChange(selection: EditorSelection, uses: readonly Use[]): void;
  onSave(): void;
  onDiscard(): void;
  onRemove(): void;
  /** Stale save: write the draft over the newer revision. */
  onOverwrite(): void;
  /** Stale save: drop the draft and show what the store holds. */
  onTakeStored(): void;
  /** Missing on save: save the draft as a new feature. */
  onSaveAsNew(): void;
}

/**
 * One feature: its title and its prose, written freely in an editor that
 * cues the recorded language, with the save bar beneath. A failed or
 * stale save keeps the text and offers the way out beside it.
 */
export function FeaturePanel(props: FeaturePanelProps) {
  const { selection, editorKey, title, body, references, entries, dirty, storeAvailable, saveState, removeError, busy, editorRef } = props;
  const id = useId();
  const [confirming, setConfirming] = useState(false);
  const saved = selection.kind === "saved";

  return (
    <article className="st-card st-card--feature" aria-label={title.trim() === "" ? "Untitled feature" : `Feature ${title}`}>
      <header className="st-card__head st-feature-head">
        <div className="st-eyebrow">
          {saved ? "Feature" : "New feature"}
          {dirty ? (
            <>
              {" "}
              <Badge tone="draft">unsaved</Badge>
            </>
          ) : null}
        </div>
        <input
          id={`${id}-title`}
          className="st-feature-title"
          type="text"
          aria-label="Title"
          placeholder="Name the feature"
          value={title}
          autoComplete="off"
          onChange={(e) => props.onTitle(e.target.value)}
        />
      </header>

      <FeatureEditor
        key={editorKey}
        ref={editorRef}
        initialBody={body}
        initialReferences={references}
        entries={entries}
        labelledBy={`${id}-title`}
        placeholder="Write the feature in your own words. Recorded names light up as cues; Ctrl-Space inserts one, Ctrl-I inspects the one at the cursor."
        onEdit={props.onEdit}
        onSelectionChange={props.onSelectionChange}
      />

      {saveState.kind === "failed" ? (
        <p className="st-form-error" role="alert">
          {saveState.message}{" "}
          {saveState.retry ? (
            <button type="button" className="st-link-button" onClick={saveState.retry}>
              Try again
            </button>
          ) : null}{" "}
          Your text is kept.
        </p>
      ) : null}
      {saveState.kind === "stale" ? (
        <div className="st-conflict" role="alert">
          <p className="st-form-error">
            This feature changed in the store since you opened it{saveState.current ? `, now titled “${saveState.current.title}”` : ""}. Nothing was written; your text is kept.
          </p>
          <div className="st-form-actions">
            <button type="button" className="st-button st-button--primary" onClick={props.onOverwrite} disabled={busy}>
              Save mine over it
            </button>
            <button type="button" className="st-button" onClick={props.onTakeStored} disabled={busy}>
              Take the stored text
            </button>
          </div>
        </div>
      ) : null}
      {saveState.kind === "missing" ? (
        <div className="st-conflict" role="alert">
          <p className="st-form-error">This feature is no longer in the store. Nothing was written; your text is kept.</p>
          <div className="st-form-actions">
            <button type="button" className="st-button st-button--primary" onClick={props.onSaveAsNew} disabled={busy}>
              Save as a new feature
            </button>
          </div>
        </div>
      ) : null}
      {removeError ? (
        <p className="st-form-error" role="alert">
          {removeError}
        </p>
      ) : null}

      <div className="st-card__actions">
        <button type="button" className="st-button st-button--primary" onClick={props.onSave} disabled={busy || !storeAvailable || (!dirty && saved)} title={storeAvailable ? "Save the feature in the project" : "This host lends no feature store"}>
          {saveState.kind === "saving" ? "Saving…" : "Save"}
        </button>
        {dirty ? (
          <button type="button" className="st-button" onClick={props.onDiscard} disabled={busy}>
            {saved ? "Discard changes" : "Discard"}
          </button>
        ) : null}
        <span className="st-card__actions-grow" />
        {saved ? (
          confirming ? (
            <span className="st-confirm" role="group" aria-label="Confirm deletion">
              <span>Delete this feature from the store?</span>
              <button
                type="button"
                className="st-button st-button--small st-button--danger"
                disabled={busy}
                onClick={() => {
                  setConfirming(false);
                  props.onRemove();
                }}
              >
                Delete
              </button>
              <button type="button" className="st-button st-button--small" onClick={() => setConfirming(false)}>
                Keep
              </button>
            </span>
          ) : (
            <button type="button" className="st-button st-button--quiet" onClick={() => setConfirming(true)} disabled={busy}>
              Delete…
            </button>
          )
        ) : null}
      </div>
    </article>
  );
}
