import { describe, expect, it } from "vitest";
import type { FeatureReferenceDocument, OverviewDocument } from "../../../shared/transport/index";
import { entriesOf, entryKey } from "../model/entry";
import { recognize } from "./recognition";
import { describeReading, entryOfTerm, termOf, useAt, usesOf, usesWithin } from "./uses";

const counts = { contexts: 0, aggregates: 0, entities: 0, valueObjects: 0, invariants: 0, assertions: 0, specifications: 0, events: 0, services: 0, questions: 0, relations: 0 };

/** Two contexts that both record an Order, plus an alias and a Unicode name. */
const recording: OverviewDocument = {
  source: "domain.arclint.yaml",
  found: true,
  counts,
  contexts: [
    {
      name: "sales",
      definition: "Selling.",
      aggregates: [
        {
          name: "Order",
          definition: "A customer's request to purchase.",
          identity: "OrderID",
          aliases: ["Purchase Order"],
          entities: [{ name: "Line", definition: "One product on an order." }],
          invariants: [{ key: "lines-frozen", statement: "Lines freeze once placed." }],
        },
      ],
      valueObjects: [
        { name: "Money", definition: "An amount in a currency.", aliases: ["Amount"] },
        { name: "Café Crème", definition: "A drink.", aliases: ["crème"] },
        { name: "OrderID", definition: "What tells one order apart." },
      ],
      questions: [{ key: "refund-window", text: "How long?" }],
    },
    {
      name: "fulfilment",
      definition: "Shipping.",
      valueObjects: [{ name: "Order", definition: "A picking instruction." }],
    },
  ],
};

const entries = entriesOf(recording);
const salesOrder = entries.find((e) => e.key === entryKey("sales", "aggregate", "", "Order"))!;
const money = entries.find((e) => e.name === "Money")!;
const linesFrozen = entries.find((e) => e.name === "lines-frozen")!;

function ref(id: string, body: string, quote: string, partial: Partial<FeatureReferenceDocument> = {}): FeatureReferenceDocument {
  const from = body.indexOf(quote);
  return { id, from, to: from + quote.length, quote, reading: "names", term: termOf(salesOrder), ...partial };
}

describe("recognize", () => {
  it("finds names and aliases as whole words, case-insensitively, longest first", () => {
    const text = "The purchase  order carries money; reorder and Orders are not it, nor is amounts.";
    const found = recognize(text, entries).map((r) => [r.text, r.entries.map((e) => `${e.context}/${e.name}`), r.alias]);
    expect(found).toEqual([
      ["purchase  order", ["sales/Order"], "purchase  order"],
      ["money", ["sales/Money"], null],
    ]);
  });

  it("reports a spelling two contexts record as ambiguous instead of choosing", () => {
    const [order] = recognize("An Order arrives.", entries);
    expect(order.entries.map((e) => `${e.context}/${e.concept}`)).toEqual(["sales/aggregate", "fulfilment/value_object"]);
    expect(order.alias).toBeNull();
  });

  it("matches Unicode names and keeps offsets in UTF-16 units", () => {
    const text = "🙂 Café Crème, then CRÈME.";
    const found = recognize(text, entries);
    expect(found.map((r) => [r.text, r.from, r.to])).toEqual([
      ["Café Crème", 3, 13],
      ["CRÈME", 20, 25],
    ]);
    expect(text.slice(found[0].from, found[0].to)).toBe("Café Crème");
  });

  it("never spans a line break and never recognises rule keys or questions", () => {
    expect(recognize("purchase\norder", entries).map((r) => [r.text, r.from])).toEqual([["order", 9]]);
    expect(recognize("lines-frozen refund-window", entries)).toEqual([]);
    expect(recognize("", entries)).toEqual([]);
  });
});

describe("usesOf", () => {
  it("places tied and plain annotations over the text and cues around them", () => {
    const body = "An Order holds Money; the order stands.";
    const tied = ref("a", body, "Order");
    const plain: FeatureReferenceDocument = { id: "b", from: body.lastIndexOf("order"), to: body.lastIndexOf("order") + 5, quote: "order", reading: "plain" };
    const uses = usesOf(body, [tied, plain], entries);
    expect(uses.map((u) => [u.kind, u.text, u.entry?.name ?? null])).toEqual([
      ["tied", "Order", "Order"],
      ["cue", "Money", "Money"],
      ["plain", "order", null],
    ]);
    expect(uses[0].entry).toBe(salesOrder);
    expect(useAt(uses, 5)!.kind).toBe("tied");
    expect(useAt(uses, 8)!.kind).toBe("tied");
    expect(useAt(uses, 9)).toBeNull();
    expect(usesWithin(uses, 0, 20).map((u) => u.kind)).toEqual(["tied", "cue"]);
  });

  it("leaves an ambiguous spelling unchosen until the author ties it", () => {
    const body = "An Order arrives.";
    const [ambiguous] = usesOf(body, [], entries);
    expect(ambiguous.kind).toBe("ambiguous");
    expect(ambiguous.entry).toBeNull();
    expect(ambiguous.candidates.map((c) => c.context)).toEqual(["sales", "fulfilment"]);
    const chosen = usesOf(body, [ref("a", body, "Order", { term: termOf(entries.find((e) => e.context === "fulfilment" && e.name === "Order")!) })], entries);
    expect(chosen[0].kind).toBe("tied");
    expect(chosen[0].entry!.context).toBe("fulfilment");
  });

  it("keeps an annotation whose target is gone as unresolved with the reason, never rebinding it", () => {
    const body = "An Order holds Money.";
    const withoutSales = entries.filter((e) => e.context !== "sales");
    const uses = usesOf(body, [ref("a", body, "Order")], withoutSales);
    expect(uses[0].kind).toBe("unresolved");
    expect(uses[0].reason).toBe("Order (aggregate in sales) is not in the recording now.");
    expect(uses[0].entry).toBeNull();
    expect(uses[0].placed).toBe(true);
    // The other Order is still recorded; the unresolved annotation does not become a cue for it.
    expect(uses.filter((u) => u.kind === "ambiguous" || u.kind === "cue")).toEqual([]);
  });

  it("marks an annotation whose text changed or was removed as unresolved, keeping its quote", () => {
    const body = "An Ordr holds Money.";
    const changed = { id: "a", from: 3, to: 8, quote: "Order", reading: "names" as const, term: termOf(salesOrder) };
    const removed = { id: "b", from: 20, to: 20, quote: "Line", reading: "names" as const, term: termOf(salesOrder) };
    const outside = { id: "c", from: 80, to: 85, quote: "Money", reading: "names" as const, term: termOf(money) };
    const uses = usesOf(body, [changed, removed, outside], entries);
    expect(uses.map((u) => [u.kind, u.text, u.placed, u.reason])).toEqual([
      ["unresolved", "Ordr ", true, "The text changed; it annotated “Order”."],
      ["cue", "Money", true, null],
      ["unresolved", "Line", false, "The text it annotated was removed."],
      ["unresolved", "Money", false, "Its position lies outside the text."],
    ]);
    expect(uses[0].reference!.term).toEqual(termOf(salesOrder));
    expect(useAt(uses, 20)).toBeNull();
  });

  it("ties a rule reading to an invariant and describes each reading in words", () => {
    const body = "Lines freeze once placed.";
    const uses = usesOf(body, [{ id: "r", from: 0, to: body.length - 1, quote: "Lines freeze once placed", reading: "rule", term: termOf(linesFrozen) }], entries);
    expect(uses[0].kind).toBe("tied");
    expect(uses[0].entry).toBe(linesFrozen);
    expect(describeReading("rule", "lines-frozen")).toBe("States a rule of lines-frozen.");
    expect(describeReading("value", "Money")).toBe("A value of Money.");
    expect(describeReading("plain", "x")).toContain("plain word");
  });

  it("resolves a term only at its exact address", () => {
    expect(entryOfTerm(entries, { concept: "aggregate", context: "sales", owner: "", name: "Order" })).toBe(salesOrder);
    expect(entryOfTerm(entries, { concept: "value_object", context: "sales", owner: "", name: "Order" })).toBeNull();
    expect(entryOfTerm(entries, { concept: "spaceship", context: "sales", owner: "", name: "Order" })).toBeNull();
  });
});
