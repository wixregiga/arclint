import { describe, expect, it } from "vitest";
import { entriesOf } from "../../../entities/language/index";
import { changeFrom, defineRequestFrom, draftOf, hasChanges, missingIn, NAME, removeRequestFrom } from "./entry-form";

const counts = { contexts: 1, aggregates: 1, entities: 0, valueObjects: 0, invariants: 0, assertions: 0, specifications: 0, events: 0, services: 0, questions: 0, relations: 0 };
const entries = entriesOf({
  source: "domain.arclint.yaml",
  found: true,
  counts,
  contexts: [{ name: "sales", definition: "Selling.", aggregates: [{ name: "Order", definition: "An order.", identity: "OrderID", aliases: ["PO", "Purchase Order"] }] }],
});
const order = entries[1];

describe("entry form", () => {
  it("holds the recorded values as text, a list one per line", () => {
    expect(draftOf("aggregate", order)).toEqual({ [NAME]: "Order", definition: "An order.", identity: "OrderID", aliases: "PO\nPurchase Order", repository: "", factory: "" });
    expect(draftOf("invariant", null)).toEqual({ [NAME]: "", statement: "", verifiedBy: "" });
  });

  it("changes only what differs, and never clears a property left alone", () => {
    const values = { ...draftOf("aggregate", order), definition: "A customer's order.  " };
    expect(changeFrom("aggregate", order, values)).toEqual({ definition: "A customer's order." });
    expect(hasChanges("aggregate", order, draftOf("aggregate", order))).toBe(false);
    const request = defineRequestFrom({ concept: "aggregate", context: "sales", owner: "" }, order, { ...values, aliases: "PO\n\n  Purchase Order \nBuy" });
    expect(request).toEqual({ concept: "aggregate", context: "sales", owner: "", name: "Order", change: { definition: "A customer's order.", aliases: ["PO", "Purchase Order", "Buy"] } });
  });

  it("records an emptied property deliberately", () => {
    expect(changeFrom("aggregate", order, { ...draftOf("aggregate", order), aliases: "" })).toEqual({ aliases: [] });
  });

  it("sends every filled property of a new entry and names what is still required", () => {
    const values = { [NAME]: " Money ", definition: "An amount.", aliases: "" };
    expect(defineRequestFrom({ concept: "value_object", context: "sales", owner: "" }, null, values)).toEqual({ concept: "value_object", context: "sales", owner: "", name: "Money", change: { definition: "An amount." } });
    expect(missingIn("aggregate", { [NAME]: "Cart", definition: "A cart." }).map((p) => p.name)).toEqual(["identity"]);
    expect(missingIn("assertion", { [NAME]: "x" }).map((p) => p.name)).toEqual(["on", "statement"]);
  });

  it("names a bounded context without a context and removes an entry as show names it", () => {
    expect(defineRequestFrom({ concept: "bounded_context", context: "billing", owner: "" }, null, { [NAME]: "billing", definition: "Billing." }).context).toBe("");
    expect(removeRequestFrom(order)).toEqual({ concept: "aggregate", context: "sales", owner: "", name: "Order" });
    expect(removeRequestFrom(entries[0])).toEqual({ concept: "bounded_context", context: "", owner: "", name: "sales" });
  });
});
