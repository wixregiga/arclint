/*
 * The author's annotations of the prose inside the editor, moving with
 * the text as it is edited. Adapted from BWrite's domain-references
 * (src/contexts/authoring/editor/domain-references.ts, MPL-2.0, commit
 * c75ead5) with one deliberate difference: BWrite dropped every
 * annotation that stopped fitting its text, Studio keeps it. The text
 * typed at either edge of an annotation stays outside it; an annotation
 * whose text is deleted collapses to a position and is later shown as
 * unresolved, never discarded by the editor.
 */
import { invertedEffects, isolateHistory } from "@codemirror/commands";
import { StateEffect, StateField, type ChangeDesc, type EditorState, type Extension, type TransactionSpec } from "@codemirror/state";
import type { FeatureReferenceDocument } from "../../../shared/transport/index";

function mapAnnotations(references: readonly FeatureReferenceDocument[], changes: ChangeDesc): readonly FeatureReferenceDocument[] {
  if (changes.empty) return references;
  let changed = false;
  const mapped = references.map((r) => {
    const from = changes.mapPos(r.from, 1);
    const to = Math.max(from, changes.mapPos(r.to, -1));
    if (from === r.from && to === r.to) return r;
    changed = true;
    return { ...r, from, to };
  });
  return changed ? mapped : references;
}

/** Replaces the annotations wholesale, in the coordinates of the document after the transaction. */
export const setAnnotations = StateEffect.define<readonly FeatureReferenceDocument[]>({
  map: (references, mapping) => mapAnnotations(references, mapping),
});

export const annotationsField = StateField.define<readonly FeatureReferenceDocument[]>({
  create: () => [],
  update(references, tr) {
    let next = mapAnnotations(references, tr.changes);
    for (const effect of tr.effects) if (effect.is(setAnnotations)) next = effect.value;
    return next;
  },
});

/**
 * Lets undo and redo restore annotations: a transaction that sets them
 * explicitly, or whose edit moved one, records the annotations it
 * started with.
 */
const annotationHistory = invertedEffects.of((tr) => {
  const before = tr.startState.field(annotationsField);
  const after = tr.state.field(annotationsField);
  return tr.effects.some((e) => e.is(setAnnotations)) || (tr.docChanged && before !== after) ? [setAnnotations.of(before)] : [];
});

export function annotations(initial: readonly FeatureReferenceDocument[]): Extension {
  return [annotationsField.init(() => initial), annotationHistory];
}

export function annotationsOf(state: EditorState): readonly FeatureReferenceDocument[] {
  return state.field(annotationsField);
}

/** The annotations other than those overlapping `[from, to)`, with `next` added, in order of position. */
export function withAnnotation(
  references: readonly FeatureReferenceDocument[],
  from: number,
  to: number,
  next: FeatureReferenceDocument | null,
): FeatureReferenceDocument[] {
  const kept = references.filter((r) => r.to <= from || r.from >= to || (r.from === r.to && (r.from < from || r.from > to)));
  if (next) kept.push(next);
  return kept.sort((a, b) => a.from - b.from || a.to - b.to);
}

/**
 * The transaction that annotates `[from, to)` with `next`, or removes
 * every annotation there when `next` is null, optionally replacing the
 * text with `wording` first. One undo step; the result stays selected.
 */
export function annotateTransaction(
  state: EditorState,
  from: number,
  to: number,
  next: Omit<FeatureReferenceDocument, "from" | "to" | "quote"> | null,
  wording: string | null = null,
): TransactionSpec {
  const replace = wording !== null && wording !== state.sliceDoc(from, to);
  const changes = state.changes(replace ? { from, to, insert: wording } : []);
  const end = replace ? from + wording.length : to;
  const kept = mapAnnotations(withAnnotation(state.field(annotationsField), from, to, null), changes);
  const quote = changes.apply(state.doc).sliceString(from, end);
  const reference = next === null || from >= end ? null : { ...next, from, to: end, quote };
  return {
    changes,
    effects: setAnnotations.of(withAnnotation(kept, from, end, reference)),
    selection: { anchor: from, head: end },
    scrollIntoView: true,
    userEvent: replace ? "input.replace.language" : "language.annotate",
    annotations: isolateHistory.of("full"),
  };
}

/** The transaction that forgets one annotation by id, wherever it now lies. */
export function forgetTransaction(state: EditorState, id: string): TransactionSpec {
  return {
    effects: setAnnotations.of(state.field(annotationsField).filter((r) => r.id !== id)),
    userEvent: "language.forget",
    annotations: isolateHistory.of("full"),
  };
}
