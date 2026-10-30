import { history, redo, undo } from "@codemirror/commands";
import { EditorState, type Transaction, type TransactionSpec } from "@codemirror/state";
import { describe, expect, it } from "vitest";
import type { FeatureReferenceDocument, FeatureTermDocument } from "../../../shared/transport/index";
import { annotateTransaction, annotations, annotationsOf, forgetTransaction } from "./annotations";

const TEXT = "An Order holds Money.\nThe order ships.";
const ORDER: FeatureTermDocument = { concept: "aggregate", context: "sales", owner: "", name: "Order" };
const MONEY: FeatureTermDocument = { concept: "value_object", context: "sales", owner: "", name: "Money" };
const tied = (id: string, quote: string, term: FeatureTermDocument): FeatureReferenceDocument => {
  const from = TEXT.indexOf(quote);
  return { id, from, to: from + quote.length, quote, reading: "names", term };
};

function state(references: FeatureReferenceDocument[] = [], doc = TEXT): EditorState {
  return EditorState.create({ doc, extensions: [history(), annotations(references)] });
}

function apply(s: EditorState, spec: TransactionSpec): EditorState {
  return s.update(spec).state;
}

function command(s: EditorState, run: (target: { state: EditorState; dispatch(tr: Transaction): void }) => boolean): EditorState {
  let next = s;
  run({
    state: s,
    dispatch(tr) {
      next = tr.state;
    },
  });
  return next;
}

describe("annotations", () => {
  it("keeps externally displaced annotations while the author continues writing", () => {
    const displaced = { ...tied("outside", "Order", ORDER), from: 900, to: 905 };
    const before = state([displaced]);
    const after = apply(before, { changes: { from: 0, insert: "Today " } });
    expect(after.doc.toString()).toBe("Today " + TEXT);
    expect(annotationsOf(after)).toEqual([displaced]);
  });
  it("moves with the text and keeps text typed at either edge outside", () => {
    const s0 = state([tied("a", "Order", ORDER), tied("b", "Money", MONEY)]);
    const s1 = apply(s0, { changes: { from: 0, insert: "Today " } });
    expect(annotationsOf(s1).map((r) => [r.id, s1.sliceDoc(r.from, r.to)])).toEqual([
      ["a", "Order"],
      ["b", "Money"],
    ]);
    const orderEnd = annotationsOf(s1)[0].to;
    const s2 = apply(s1, { changes: { from: orderEnd, insert: "s" } });
    expect(s2.sliceDoc(annotationsOf(s2)[0].from, annotationsOf(s2)[0].to)).toBe("Order");
    expect(s2.sliceDoc(annotationsOf(s2)[0].from, annotationsOf(s2)[0].to + 1)).toBe("Orders");
  });

  it("keeps an annotation whose text is deleted, collapsed to a position, so it can be shown unresolved", () => {
    const s0 = state([tied("a", "Order", ORDER)]);
    const { from, to } = annotationsOf(s0)[0];
    const s1 = apply(s0, { changes: { from, to, insert: "" } });
    expect(annotationsOf(s1)).toHaveLength(1);
    expect(annotationsOf(s1)[0]).toMatchObject({ id: "a", from, to: from, quote: "Order", term: ORDER });
  });

  it("keeps an annotation whose text changed, with its original quote", () => {
    const s0 = state([tied("a", "Order", ORDER)]);
    const s1 = apply(s0, { changes: { from: 5, to: 6, insert: "" } });
    const [a] = annotationsOf(s1);
    expect(s1.sliceDoc(a.from, a.to)).toBe("Orer");
    expect(a.quote).toBe("Order");
  });

  it("annotates a range, replacing what overlaps it, and clears it again", () => {
    const s0 = state([tied("a", "Order", ORDER)]);
    const s1 = apply(s0, annotateTransaction(s0, 3, 14, { id: "c", reading: "plain" }));
    expect(annotationsOf(s1)).toEqual([{ id: "c", from: 3, to: 14, quote: "Order holds", reading: "plain" }]);
    expect(s1.selection.main.from).toBe(3);
    expect(s1.selection.main.to).toBe(14);
    const s2 = apply(s1, annotateTransaction(s1, 3, 8, null));
    expect(annotationsOf(s2)).toEqual([]);
  });

  it("replaces the wording when asked and quotes the new text", () => {
    const s0 = state([]);
    const s1 = apply(s0, annotateTransaction(s0, 3, 8, { id: "d", reading: "names", term: ORDER }, "Purchase Order"));
    expect(s1.doc.toString()).toBe("An Purchase Order holds Money.\nThe order ships.");
    expect(annotationsOf(s1)).toEqual([{ id: "d", from: 3, to: 17, quote: "Purchase Order", reading: "names", term: ORDER }]);
  });

  it("forgets one annotation by id and leaves the rest", () => {
    const s0 = state([tied("a", "Order", ORDER), tied("b", "Money", MONEY)]);
    const s1 = apply(s0, forgetTransaction(s0, "a"));
    expect(annotationsOf(s1).map((r) => r.id)).toEqual(["b"]);
  });

  it("undoes and redoes annotations together with the text they annotate", () => {
    const s0 = state([tied("a", "Order", ORDER)]);
    const s1 = apply(s0, annotateTransaction(s0, 15, 20, { id: "b", reading: "names", term: MONEY }));
    expect(annotationsOf(s1).map((r) => r.id)).toEqual(["a", "b"]);
    const s2 = command(s1, undo);
    expect(annotationsOf(s2).map((r) => r.id)).toEqual(["a"]);
    const s3 = command(s2, redo);
    expect(annotationsOf(s3).map((r) => r.id)).toEqual(["a", "b"]);

    const s4 = apply(s3, { changes: { from: 0, to: 21, insert: "" } });
    expect(annotationsOf(s4).every((r) => r.from === r.to)).toBe(true);
    const s5 = command(s4, undo);
    expect(s5.doc.toString()).toBe(TEXT);
    expect(annotationsOf(s5).map((r) => s5.sliceDoc(r.from, r.to))).toEqual(["Order", "Money"]);
  });
});
