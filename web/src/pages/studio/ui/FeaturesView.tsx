import { useCallback, useEffect, useLayoutEffect, useMemo, useRef, useState } from "react";
import { useAt, usesOf, type Entry, type Use } from "../../../entities/language/index";
import { ticketMarkdown } from "../../../features/copy-ticket/index";
import type { Association, EditorSelection, FeatureEditorHandle } from "../../../features/write-feature/index";
import type { FeatureReferenceDocument, FeatureStore, RelationDocument } from "../../../shared/transport/index";
import { draftFromDocument, featureDraftKey, newFeatureKey, sameText, type FeatureDraft, type FeatureSelection } from "../model/feature-drafts";
import { IDLE_SAVE, type FeatureSaveState } from "../model/save-state";
import { useFeatureStore } from "../model/use-feature-store";
import type { Workspace, WorkspaceAction } from "../model/workspace";
import { FeatureInspector } from "./FeatureInspector";
import { FeaturePanel } from "./FeaturePanel";
import { FeaturesNavigator } from "./FeaturesNavigator";

export interface FeaturesViewProps {
  store: FeatureStore | null;
  active: boolean;
  workspace: Workspace;
  dispatch(action: WorkspaceAction): void;
  entries: readonly Entry[];
  relations: readonly RelationDocument[];
  project: string;
  onOpenEntry(entry: Entry): void;
  onNotice(text: string): void;
}

const EMPTY_DRAFT: FeatureDraft = { title: "", body: "", references: [], revision: null };

/**
 * Features mode: the saved features in the navigator, one feature's
 * title and prose in the middle, the language at hand beside it. The
 * store is read when this view first shows; a feature's saved text is
 * authoritative and the author's typing over it is the draft the
 * workspace keeps across Language, Map and reloads.
 */
export function FeaturesView(props: FeaturesViewProps) {
  const { store, active, workspace, dispatch, entries, relations, project, onOpenEntry, onNotice } = props;
  const features = useFeatureStore(store, active);
  const selection = workspace.feature;
  const key = selection ? featureDraftKey(selection) : null;
  const draft = key ? (workspace.featureDrafts[key] ?? null) : null;
  const loaded = selection?.kind === "saved" ? (features.loaded[selection.id] ?? null) : null;
  const document = loaded?.document ?? null;

  const [saveStates, setSaveStates] = useState<Record<string, FeatureSaveState>>({});
  const [removeErrors, setRemoveErrors] = useState<Record<string, string | null>>({});
  const saveState = key ? (saveStates[key] ?? IDLE_SAVE) : IDLE_SAVE;
  const removeError = key ? (removeErrors[key] ?? null) : null;
  function setSaveState(next: FeatureSaveState) {
    if (key) setSaveStates((states) => ({ ...states, [key]: next }));
  }
  function setRemoveError(next: string | null) {
    if (key) setRemoveErrors((errors) => ({ ...errors, [key]: next }));
  }
  const [busy, setBusy] = useState(false);
  const [reset, setReset] = useState(0);
  const [editorSelection, setEditorSelection] = useState<EditorSelection | null>(null);
  const editorRef = useRef<FeatureEditorHandle>(null);

  useEffect(() => {
    setEditorSelection(null);
  }, [key]);

  useEffect(() => {
    if (selection?.kind === "saved" && store && !features.loaded[selection.id]) void features.load(selection.id);
  }, [selection, store, features]);

  const base: FeatureDraft | null = selection === null ? null : selection.kind === "new" ? EMPTY_DRAFT : document ? draftFromDocument(document) : null;
  const shown: FeatureDraft | null = draft ?? base;
  const dirty = draft !== null && (selection?.kind === "new" || document === null || !sameText(draft, document));

  // Replace stale editor contents before paint, while keeping local drafts and
  // the selection and undo history of an acknowledged, unchanged snapshot.
  const displayed = useRef<{ key: string | null; draft: FeatureDraft | null }>({ key: null, draft: null });
  useLayoutEffect(() => {
    const previous = displayed.current;
    displayed.current = { key, draft: shown };
    if (previous.key === key && previous.draft && !draft && document && !sameText(previous.draft, document)) {
      setEditorSelection(null);
      setReset((n) => n + 1);
    }
  }, [key, shown, draft, document]);

  const uses = useMemo(() => (shown ? usesOf(shown.body, shown.references, entries) : []), [shown, entries]);
  // The use at the cursor; with a selection, the use that is exactly the selection, so a fresh tie shows what it became.
  const focused: Use | null = useMemo(() => {
    if (!editorSelection) return null;
    if (editorSelection.from === editorSelection.to) return useAt(uses, editorSelection.from);
    return uses.find((u) => u.placed && u.from === editorSelection.from && u.to === editorSelection.to) ?? null;
  }, [uses, editorSelection]);

  const edit = useCallback(
    (change: { title?: string; body?: string; references?: FeatureReferenceDocument[] }) => {
      if (!selection || !shown) return;
      dispatch({ type: "feature-draft", feature: selection, base: shown, ...change });
    },
    [dispatch, selection, shown],
  );

  function beginNew() {
    const next: FeatureSelection = { kind: "new", key: newFeatureKey() };
    dispatch({ type: "open-feature", feature: next });
  }

  async function persist(request: { id?: string; revision?: string }) {
    if (!selection || !shown) return;
    setBusy(true);
    setRemoveError(null);
    setSaveState({ kind: "saving" });
    const outcome = await features.save({ ...request, title: shown.title, body: shown.body, references: shown.references });
    setBusy(false);
    switch (outcome.kind) {
      case "saved":
        setSaveState(IDLE_SAVE);
        dispatch({ type: "feature-saved", feature: selection, document: outcome.document });
        onNotice(`Saved feature “${outcome.document.title.trim() === "" ? "Untitled feature" : outcome.document.title}”.`);
        break;
      case "stale":
        setSaveState({ kind: "stale", current: outcome.current });
        break;
      case "missing":
        setSaveState({ kind: "missing" });
        break;
      case "failed":
        setSaveState({ kind: "failed", message: outcome.message, retry: outcome.retryable ? () => void persist(request) : null });
        break;
    }
  }

  function save() {
    if (!selection || !shown) return;
    if (selection.kind === "new") void persist({});
    else void persist({ id: selection.id, revision: shown.revision ?? document?.revision ?? "" });
  }

  function overwrite() {
    if (!selection || selection.kind !== "saved" || saveState.kind !== "stale") return;
    const revision = saveState.current?.revision;
    if (!revision) {
      void features.load(selection.id);
      setSaveState({ kind: "failed", message: "The stored revision could not be read; try saving again.", retry: save });
      return;
    }
    dispatch({ type: "feature-rebase", feature: selection, revision });
    void persist({ id: selection.id, revision });
  }

  function takeStored() {
    if (!selection) return;
    dispatch({ type: "feature-discard", feature: selection });
    setSaveState(IDLE_SAVE);
    setRemoveError(null);
    if (selection.kind === "saved") void features.load(selection.id);
  }

  function discard() {
    if (!selection) return;
    if (selection.kind === "new") {
      dispatch({ type: "feature-removed", feature: selection });
      return;
    }
    dispatch({ type: "feature-discard", feature: selection });
    setSaveState(IDLE_SAVE);
    setRemoveError(null);
  }

  async function remove() {
    if (!selection || selection.kind !== "saved") return;
    const revision = shown?.revision ?? document?.revision;
    if (!revision) return;
    setBusy(true);
    setRemoveError(null);
    const outcome = await features.remove(selection.id, revision);
    setBusy(false);
    switch (outcome.kind) {
      case "removed":
      case "missing":
        dispatch({ type: "feature-removed", feature: selection });
        onNotice(outcome.kind === "removed" ? "Deleted the feature." : "The feature was already gone from the store.");
        break;
      case "stale":
        setRemoveError(`The feature changed in the store since you opened it${outcome.current ? `, now titled “${outcome.current.title}”` : ""}; nothing was deleted. Take the stored text or save yours first.`);
        setSaveState({ kind: "stale", current: outcome.current });
        break;
      case "failed":
        setRemoveError(outcome.message);
        break;
    }
  }

  const associate = (from: number, to: number, association: Association | null) => editorRef.current?.associate(from, to, association);

  const storeNote = store ? null : "This host lends no feature store, so nothing written here can be saved.";
  const editorKey = key ? `${key}|${reset}` : "";

  let main: React.ReactNode;
  if (!selection) {
    main = (
      <article className="st-card st-card--feature-empty" aria-label="No feature open">
        <h1 className="st-card__title">Write a feature</h1>
        <p className="st-reading">Describe what the system should do in your own words. The recorded language lights up as you write, so its definitions stay at hand; tie a word to the entry you mean, or mark it plain.</p>
        <div className="st-card__actions">
          <button type="button" className="st-button st-button--primary" onClick={beginNew}>
            + Feature
          </button>
        </div>
      </article>
    );
  } else if (selection.kind === "saved" && !shown) {
    main =
      loaded?.status === "missing" ? (
        <article className="st-card st-card--feature-empty" role="alert" aria-label="Feature missing">
          <h1 className="st-card__title">This feature is not in the store</h1>
          <p className="st-reading">It was deleted, or the store moved. Nothing of yours is lost: no draft was open for it.</p>
          <div className="st-card__actions">
            <button type="button" className="st-button" onClick={() => dispatch({ type: "feature-removed", feature: selection })}>
              Close
            </button>
          </div>
        </article>
      ) : loaded?.status === "failed" ? (
        <article className="st-card st-card--feature-empty" role="alert" aria-label="Feature unavailable">
          <h1 className="st-card__title">Studio cannot read this feature</h1>
          <p className="st-reading">{loaded.error}</p>
          <div className="st-card__actions">
            <button type="button" className="st-button st-button--primary" onClick={() => void features.load(selection.id)}>
              Try again
            </button>
          </div>
        </article>
      ) : (
        <p className="st-note" aria-busy="true">
          Reading the feature…
        </p>
      );
  } else {
    main = (
      <FeaturePanel
        key={editorKey}
        selection={selection}
        editorKey={editorKey}
        title={shown!.title}
        body={shown!.body}
        references={shown!.references}
        entries={entries}
        dirty={dirty}
        storeAvailable={store !== null}
        saveState={saveState}
        removeError={removeError}
        busy={busy}
        editorRef={editorRef}
        onTitle={(title) => edit({ title })}
        onEdit={(body, references) => edit({ body, references })}
        onSelectionChange={(sel) => setEditorSelection(sel)}
        onSave={save}
        onDiscard={discard}
        onRemove={() => void remove()}
        onOverwrite={overwrite}
        onTakeStored={takeStored}
        onSaveAsNew={() => void persist({})}
      />
    );
  }

  return (
    <>
      <FeaturesNavigator
        list={features.list}
        storeAvailable={store !== null}
        selection={selection}
        drafts={workspace.featureDrafts}
        onOpen={(next) => dispatch({ type: "open-feature", feature: next })}
        onNew={beginNew}
        onRefresh={() => void features.refresh()}
      />
      <main className="st-main st-main--feature">{main}</main>
      <aside className="st-inspector" aria-label="Language at hand">
        {shown ? (
          <FeatureInspector
            entries={entries}
            relations={relations}
            uses={uses}
            selection={editorSelection}
            focused={focused}
            storeNote={storeNote}
            onOpenEntry={onOpenEntry}
            onAssociate={associate}
            onForget={(id) => editorRef.current?.forget(id)}
            onGoTo={(from, to) => editorRef.current?.select(from, to)}
            ticket={() => ticketMarkdown({ title: shown.title, body: shown.body, uses, project })}
          />
        ) : (
          <p className="st-note">{storeNote ?? "Open or begin a feature to see the language beside it."}</p>
        )}
      </aside>
    </>
  );
}
