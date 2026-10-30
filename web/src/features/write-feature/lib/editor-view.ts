/*
 * The CodeMirror view of one feature's prose: created once on a host
 * element and driven through a handle. Adapted from BWrite's
 * feature-editor-view (src/contexts/authoring/editor/feature-editor-view.ts,
 * MPL-2.0, commit c75ead5) without Gherkin, rule placement or a
 * context menu: the prose is free, and the selection's actions live in
 * the inspector beside the editor.
 */
import { defaultKeymap, history, historyKeymap } from "@codemirror/commands";
import { EditorState, Prec } from "@codemirror/state";
import { drawSelection, EditorView, highlightSpecialChars, keymap, placeholder } from "@codemirror/view";
import type { Entry, Use } from "../../../entities/language/index";
import type { FeatureReading, FeatureReferenceDocument, FeatureTermDocument } from "../../../shared/transport/index";
import { annotateTransaction, annotations, annotationsOf, forgetTransaction } from "./annotations";
import { insertion } from "./insertion";
import { languageMarks, setLanguage, usesIn } from "./language-marks";
import { newReferenceId } from "./reference-id";

export interface EditorSelection {
  from: number;
  to: number;
  text: string;
}

/** What the author ties an occurrence to: a recorded entry read a certain way, or nothing, as a plain word. */
export type Association = { reading: Exclude<FeatureReading, "plain">; term: FeatureTermDocument } | { reading: "plain" };

export interface FeatureEditorHandle {
  focus(): void;
  select(from: number, to: number): void;
  /** Opens the card for the use at the cursor; false when there is none. */
  inspectAtCursor(): boolean;
  /** Opens the insertion list at the cursor. */
  openInsertion(): void;
  /** Annotates `[from, to)`, or clears every annotation there when `association` is null. One undo step. */
  associate(from: number, to: number, association: Association | null): void;
  /** Forgets one annotation by id, wherever it lies now. */
  forget(id: string): void;
}

export interface FeatureEditorSettings {
  initialBody: string;
  initialReferences: readonly FeatureReferenceDocument[];
  entries: readonly Entry[];
  labelledBy: string;
  placeholder: string;
  /** One editor step: the body and the annotations as they now stand, reported together. */
  onEdit(body: string, references: FeatureReferenceDocument[]): void;
  onSelectionChange(selection: EditorSelection, uses: readonly Use[]): void;
}

export interface FeatureEditorView extends FeatureEditorHandle {
  setLanguage(entries: readonly Entry[]): void;
  destroy(): void;
}

/** Creates the editor on `parent`; `settings` is read on every use for the host's current values. */
export function createFeatureEditorView(parent: HTMLElement, settings: () => FeatureEditorSettings): FeatureEditorView {
  const initial = settings();
  const marks = languageMarks(initial.entries);
  const insert = insertion(() => settings().entries);

  const reportSelection = (view: EditorView) => {
    const { from, to } = view.state.selection.main;
    settings().onSelectionChange({ from, to, text: view.state.sliceDoc(from, to) }, usesIn(view.state));
  };

  const view = new EditorView({
    parent,
    state: EditorState.create({
      doc: initial.initialBody,
      extensions: [
        history(),
        drawSelection(),
        highlightSpecialChars(),
        EditorView.lineWrapping,
        placeholder(initial.placeholder),
        insert.extension,
        annotations(initial.initialReferences),
        marks.extension,
        Prec.high(
          keymap.of([
            {
              key: "Mod-i",
              preventDefault: true,
              run: (v) => {
                marks.inspectAtCursor(v);
                return true;
              },
            },
          ]),
        ),
        keymap.of([...defaultKeymap, ...historyKeymap]),
        EditorView.contentAttributes.of({ "aria-labelledby": initial.labelledBy, spellcheck: "true" }),
        EditorView.updateListener.of((update) => {
          const references = annotationsOf(update.state);
          const annotationsChanged = references !== annotationsOf(update.startState);
          if (update.docChanged || annotationsChanged) {
            settings().onEdit(update.state.doc.toString(), [...references]);
          }
          if (update.docChanged || update.selectionSet || annotationsChanged) reportSelection(update.view);
        }),
      ],
    }),
  });
  reportSelection(view);

  return {
    focus: () => view.focus(),
    select(from, to) {
      view.dispatch({ selection: { anchor: from, head: to }, scrollIntoView: true });
      view.focus();
    },
    inspectAtCursor() {
      view.focus();
      return marks.inspectAtCursor(view);
    },
    openInsertion() {
      view.focus();
      insert.open(view);
    },
    associate(from, to, association) {
      const next = association === null ? null : association.reading === "plain" ? { id: newReferenceId(), reading: "plain" as const } : { id: newReferenceId(), reading: association.reading, term: association.term };
      view.dispatch(annotateTransaction(view.state, from, to, next));
      view.focus();
    },
    forget(id) {
      view.dispatch(forgetTransaction(view.state, id));
    },
    setLanguage(entries) {
      view.dispatch({ effects: setLanguage.of(entries) });
      reportSelection(view);
    },
    destroy: () => view.destroy(),
  };
}
