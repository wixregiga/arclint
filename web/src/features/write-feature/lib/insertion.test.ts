import { EditorState } from "@codemirror/state";
import { describe, expect, it } from "vitest";
import type { OverviewDocument } from "../../../shared/transport/index";
import { entriesOf } from "../../../entities/language/index";
import { annotations, annotationsOf } from "./annotations";
import { insertable, insertionMatch, insertTransaction } from "./insertion";

const counts = { contexts: 0, aggregates: 0, entities: 0, valueObjects: 0, invariants: 0, assertions: 0, specifications: 0, events: 0, services: 0, questions: 0, relations: 0 };
const recording: OverviewDocument = {
  source: "domain.arclint.yaml",
  found: true,
  counts,
  contexts: [
    {
      name: "sales",
      definition: "Selling.",
      aggregates: [{ name: "Order", definition: "A request.", identity: "OrderID", invariants: [{ key: "lines-frozen", statement: "Lines freeze." }] }],
      valueObjects: [{ name: "OrderID", definition: "An id." }, { name: "Money", definition: "An amount." }],
    },
    { name: "fulfilment", definition: "Shipping.", valueObjects: [{ name: "Order", definition: "A picking instruction." }] },
  ],
};
const entries = insertable(entriesOf(recording));

describe("insertionMatch", () => {
  it("offers the whole language after whitespace or at a line start", () => {
    expect(insertionMatch("", 0, entries).entries.map((e) => e.name)).toEqual(["fulfilment", "Money", "Order", "Order", "OrderID", "sales"]);
    const match = insertionMatch("Given ", 6, entries);
    expect(match).toMatchObject({ from: 6, to: 6, query: "" });
  });

  it("narrows to the names the word before the cursor begins, both Orders listed with their contexts", () => {
    const match = insertionMatch("Given an ord", 12, entries);
    expect(match.from).toBe(9);
    expect(match.query).toBe("ord");
    expect(match.entries.map((e) => `${e.context}/${e.name}`)).toEqual(["fulfilment/Order", "sales/Order", "sales/OrderID"]);
  });

  it("puts an exact spelling first and keeps a word nothing begins as the query", () => {
    expect(insertionMatch("the money", 9, entries).entries.map((e) => e.name)).toEqual(["Money"]);
    const none = insertionMatch("the zebra", 9, entries);
    expect(none).toMatchObject({ from: 4, to: 9, query: "zebra" });
    expect(none.entries).toEqual([]);
  });

  it("never lists rule keys", () => {
    expect(entries.some((e) => e.concept === "invariant")).toBe(false);
  });
});

describe("insertTransaction", () => {
  it("writes the name over the typed word, ties it to the chosen entry and lands the cursor after it", () => {
    const state = EditorState.create({ doc: "Given an ord", extensions: [annotations([])] });
    const match = insertionMatch("Given an ord", 12, entries);
    const chosen = match.entries.find((e) => e.context === "fulfilment")!;
    const next = state.update(insertTransaction(state, match, chosen)).state;
    expect(next.doc.toString()).toBe("Given an Order");
    expect(next.selection.main.head).toBe(14);
    const [reference] = annotationsOf(next);
    expect(reference).toMatchObject({ from: 9, to: 14, quote: "Order", reading: "names", term: { concept: "value_object", context: "fulfilment", owner: "", name: "Order" } });
    expect(reference.id).not.toBe("");
  });
});
