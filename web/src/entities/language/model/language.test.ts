import { describe, expect, it } from "vitest";
import type { OverviewDocument } from "../../../shared/transport/index";
import { connectionsOf } from "./connections";
import { entriesOf, entryKey, membersOf, missingRequired, ownerOf, readingOf, topLevelOf } from "./entry";
import { differences, proposedKeys, relationDifferences, standingOf } from "./standing";
import { searchEntries } from "../lib/search";
import { layoutContext } from "../lib/map-layout";
import { layoutContextMap } from "../lib/context-map-layout";

const counts = { contexts: 0, aggregates: 0, entities: 0, valueObjects: 0, invariants: 0, assertions: 0, specifications: 0, events: 0, services: 0, questions: 0, relations: 0 };

/** A shop, as `arclint domain overview --format json` would print it. */
const shop: OverviewDocument = {
  source: "domain.arclint.yaml",
  found: true,
  project: "shop",
  description: "A shop.",
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
          entities: [{ name: "Line", definition: "One product on an order.", identity: "LineID" }],
          invariants: [{ key: "lines-frozen", statement: "Lines freeze once placed.", verifiedBy: ["order_test.go:TestLinesFreeze"] }],
          assertions: [{ key: "total-sums", on: "Place", statement: "The total sums the lines." }],
          repository: "OrderRepository",
        },
      ],
      valueObjects: [
        { name: "OrderID", definition: "What tells one order apart." },
        { name: "Money", definition: "An amount in a currency.", aliases: ["Amount"], invariants: [{ key: "non-negative", statement: "Money is never negative." }] },
      ],
      events: [
        { name: "OrderPlaced", definition: "An order was placed.", raisedBy: "Order" },
        { name: "DayClosed", definition: "The day closed." },
      ],
      services: [{ name: "Pricing", definition: "Prices an order.", assertions: [{ key: "priced-once", on: "Price", statement: "An order is priced once." }] }],
      specifications: [{ name: "Overdue", definition: "An order past its due date." }],
      questions: [{ key: "refund-window", text: "How long can an order be refunded?" }],
    },
    { name: "shipping", definition: "Shipping." },
  ],
  relations: [{ from: "sales", to: "shipping", kind: "customer_supplier", description: "Shipping serves sales." }],
};

describe("entriesOf", () => {
  it("flattens every entry with its context and owner, in the recording's order", () => {
    const entries = entriesOf(shop);
    expect(entries.map((e) => `${e.concept}:${e.owner ? e.owner + "/" : ""}${e.name}`)).toEqual([
      "bounded_context:sales",
      "aggregate:Order",
      "entity:Order/Line",
      "invariant:Order/lines-frozen",
      "assertion:Order/total-sums",
      "value_object:OrderID",
      "value_object:Money",
      "invariant:Money/non-negative",
      "domain_event:OrderPlaced",
      "domain_event:DayClosed",
      "domain_service:Pricing",
      "assertion:Pricing/priced-once",
      "specification:Overdue",
      "question:refund-window",
      "bounded_context:shipping",
    ]);
    expect(entries[1].properties).toEqual({ definition: "A customer's request to purchase.", identity: "OrderID", aliases: ["Purchase Order"], repository: "OrderRepository", factory: "" });
    expect(readingOf(entries[13])).toBe("How long can an order be refunded?");
  });

  it("holds nothing for a project without a domain file", () => {
    expect(entriesOf({ source: "domain.arclint.yaml", found: false, counts })).toEqual([]);
    expect(entriesOf(null)).toEqual([]);
  });

  it("knows owners, members and the top level of a context", () => {
    const entries = entriesOf(shop);
    const order = entries.find((e) => e.key === entryKey("sales", "aggregate", "", "Order"))!;
    expect(membersOf(entries, order).map((m) => m.name)).toEqual(["Line", "lines-frozen", "total-sums"]);
    const line = entries.find((e) => e.name === "Line")!;
    expect(ownerOf(entries, line)?.name).toBe("Order");
    expect(topLevelOf(entries, "sales").map((e) => e.name)).toEqual(["Order", "OrderID", "Money", "OrderPlaced", "DayClosed", "Pricing", "Overdue", "refund-window"]);
    expect(topLevelOf(entries, "shipping")).toEqual([]);
  });

  it("names the required properties an entry still lacks", () => {
    const entries = entriesOf({ ...shop, contexts: [{ name: "c", definition: "", aggregates: [{ name: "A", definition: "", identity: "" }] }] });
    expect(missingRequired(entries[0])).toEqual(["definition"]);
    expect(missingRequired(entries[1])).toEqual(["definition", "identity"]);
  });
});

describe("connectionsOf", () => {
  it("lists what the recording states: owner, members, raised-by, identity and relations", () => {
    const entries = entriesOf(shop);
    const by = (name: string) => entries.find((e) => e.name === name)!;
    expect(connectionsOf(entries, shop.relations!, by("Order")).map((c) => `${c.label} ${c.entry.name}`)).toEqual([
      "Holds Line",
      "Always lines-frozen",
      "Guarantees total-sums",
      "Raises OrderPlaced",
      "Identified by OrderID",
    ]);
    expect(connectionsOf(entries, shop.relations!, by("OrderPlaced")).map((c) => `${c.label} ${c.entry.name}`)).toEqual(["Raised by Order"]);
    expect(connectionsOf(entries, shop.relations!, by("OrderID")).map((c) => `${c.label} ${c.entry.name}`)).toEqual(["Identity of Order"]);
    expect(connectionsOf(entries, shop.relations!, by("lines-frozen")).map((c) => `${c.label} ${c.entry.name}`)).toEqual(["Always holds for Order"]);
    expect(connectionsOf(entries, shop.relations!, by("sales")).map((c) => `${c.label} ${c.entry.name} (${c.note})`)).toEqual(["customer supplier upstream of shipping (Shipping serves sales.)"]);
    expect(connectionsOf(entries, shop.relations!, by("DayClosed"))).toEqual([]);
  });
});

describe("differences and standing", () => {
  it("marks added, changed and removed entries against the accepted recording", () => {
    const accepted = entriesOf(shop);
    const edited: OverviewDocument = JSON.parse(JSON.stringify(shop));
    edited.contexts![0].valueObjects![1].definition = "An amount of money in one currency.";
    edited.contexts![0].valueObjects!.push({ name: "Weight", definition: "How heavy." });
    edited.contexts![0].questions = [];
    const current = entriesOf(edited);
    const diffs = differences(accepted, current);
    expect(diffs.map((d) => `${d.kind} ${d.concept} ${d.name}`)).toEqual(["changed value_object Money", "added value_object Weight", "removed question refund-window"]);
    expect(diffs[0].fields).toEqual([{ field: "definition", before: "An amount in a currency.", after: "An amount of money in one currency." }]);
    const proposed = proposedKeys(diffs);
    expect(standingOf(proposed, entryKey("sales", "value_object", "", "Money"))).toBe("proposed");
    expect(standingOf(proposed, entryKey("sales", "value_object", "", "Weight"))).toBe("proposed");
    expect(standingOf(proposed, entryKey("sales", "aggregate", "", "Order"))).toBe("accepted");
  });

  it("compares relations by their pair of contexts", () => {
    const before = shop.relations!;
    expect(relationDifferences(before, [{ from: "shipping", to: "sales", kind: "conformist" }]).map((d) => d.kind)).toEqual(["changed"]);
    expect(relationDifferences(before, []).map((d) => d.kind)).toEqual(["removed"]);
    expect(relationDifferences([], before).map((d) => d.kind)).toEqual(["added"]);
  });
});

describe("searchEntries", () => {
  it("finds by name before alias before reading, across contexts", () => {
    const entries = entriesOf(shop);
    const order = searchEntries(entries, "order").map((h) => `${h.matchedBy}:${h.entry.name}`);
    expect(order.slice(0, 3)).toEqual(["name:Order", "name:OrderID", "name:OrderPlaced"]);
    expect(order).toContain("reading:Line");
    expect(order.indexOf("reading:Line")).toBeGreaterThan(order.indexOf("name:OrderPlaced"));
    const amount = searchEntries(entries, "amount");
    expect(amount[0].entry.name).toBe("Money");
    expect(amount[0].matchedBy).toBe("alias");
    expect(amount[0].alias).toBe("Amount");
    expect(searchEntries(entries, "shipping").map((h) => h.entry.name)).toContain("shipping");
    expect(searchEntries(entries, "")).toEqual([]);
  });
});

describe("layouts", () => {
  it("places every entry of the context once and draws its recorded lines", () => {
    const entries = entriesOf(shop);
    const layout = layoutContext(entries, "sales", { width: 1200, height: 700 });
    const placed = new Set(layout.nodes.map((n) => n.key));
    for (const e of entries.filter((e) => e.context === "sales" && e.concept !== "bounded_context")) expect(placed.has(e.key)).toBe(true);
    expect(layout.nodes.length).toBe(13);
    expect(layout.lines.filter((l) => l.kind === "raises").length).toBe(1);
    expect(layout.lines.filter((l) => l.kind === "identity").length).toBe(1);
    expect(layout.lines.filter((l) => l.kind === "owner").length).toBeGreaterThan(0);
    expect(layoutContext(entries, "shipping").nodes).toEqual([]);
  });

  it("lays the context map out with one line per relation", () => {
    const layout = layoutContextMap(["sales", "shipping"], shop.relations!);
    expect(layout.nodes.map((n) => n.name)).toEqual(["sales", "shipping"]);
    expect(layout.lines.length).toBe(1);
    expect(layout.lines[0].kind).toBe("customer_supplier");
    expect(layoutContextMap([], []).nodes).toEqual([]);
  });
});
