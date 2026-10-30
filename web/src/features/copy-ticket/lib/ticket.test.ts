import { describe, expect, it } from "vitest";
import { entriesOf, termOf, usesOf } from "../../../entities/language/index";
import type { FeatureReferenceDocument, OverviewDocument } from "../../../shared/transport/index";
import { ticketMarkdown } from "./ticket";

const counts = { contexts: 0, aggregates: 0, entities: 0, valueObjects: 0, invariants: 0, assertions: 0, specifications: 0, events: 0, services: 0, questions: 0, relations: 0 };
const recording: OverviewDocument = {
  source: "domain.arclint.yaml",
  found: true,
  project: "shop",
  counts,
  contexts: [
    {
      name: "sales",
      definition: "Selling.",
      aggregates: [{ name: "Order", definition: "A customer's request to purchase.", identity: "OrderID", invariants: [{ key: "lines-frozen", statement: "Lines freeze once placed." }] }],
      valueObjects: [
        { name: "Money", definition: "An amount in a currency." },
        { name: "Status", definition: "Where an order stands." },
        { name: "Blank", definition: "" },
      ],
    },
    { name: "fulfilment", definition: "Shipping.", valueObjects: [{ name: "Order", definition: "A picking instruction." }] },
  ],
};
const entries = entriesOf(recording);
const by = (context: string, name: string) => entries.find((e) => e.context === context && e.name === name)!;

function ref(body: string, quote: string, partial: Partial<FeatureReferenceDocument>, nth = 0): FeatureReferenceDocument {
  let from = -1;
  for (let i = 0; i <= nth; i++) from = body.indexOf(quote, from + 1);
  return { id: `r-${quote}-${nth}`, from, to: from + quote.length, quote, reading: "names", ...partial };
}

describe("ticketMarkdown", () => {
  it("writes the title, the prose and the language used, telling tied uses from cues and plain words from both", () => {
    const body = "When an Order is shipped, its Money is settled and the order is done.\nBlank stays.";
    const references = [
      ref(body, "Order", { term: termOf(by("sales", "Order")) }),
      ref(body, "shipped", { reading: "value", term: termOf(by("sales", "Status")) }),
      ref(body, "order", { reading: "plain" }, 0),
    ];
    const uses = usesOf(body, references, entries);
    const text = ticketMarkdown({ title: "  Settle on shipping ", body, uses, project: "shop" });
    expect(text).toBe(
      [
        "# Settle on shipping",
        "",
        body,
        "",
        "## Language used",
        "",
        "### Tied on purpose",
        "",
        "- **Order** (aggregate in sales): A customer's request to purchase.",
        "- **Status** (value object in sales): Where an order stands. Read as a value of Status.",
        "",
        "### Recognised automatically",
        "",
        "_Spellings the recording also uses; the author did not confirm them._",
        "",
        "- **Money** (value object in sales): An amount in a currency.",
        "- **Blank** (value object in sales): no definition recorded yet",
        "",
      ].join("\n"),
    );
    expect(text).not.toContain("plain");
  });

  it("names ambiguous and unresolved uses without choosing for the author", () => {
    const body = "An Order arrives with Mony.";
    const gone = { id: "g", from: body.indexOf("Mony"), to: body.indexOf("Mony") + 4, quote: "Money", reading: "names" as const, term: termOf(by("sales", "Money")) };
    const removed = { id: "x", from: 2, to: 2, quote: "Line", reading: "names" as const, term: { concept: "entity", context: "sales", owner: "Order", name: "Line" } };
    const text = ticketMarkdown({ title: "", body, uses: usesOf(body, [gone, removed], entries), project: "" });
    expect(text).toContain("# Untitled feature");
    expect(text).toContain("### Ambiguous\n\n- “Order” is recorded as Order (aggregate in sales) and Order (value object in fulfilment); which is meant is not chosen.");
    expect(text).toContain("### Unresolved\n\n- “Money” tied to Money (value object in sales): The text changed; it annotated “Money”.\n- “Line” tied to Line (entity of Order in sales): The text it annotated was removed.");
    expect(text).not.toContain("Tied on purpose");
  });

  it("says when nothing recorded is used", () => {
    const text = ticketMarkdown({ title: "Hello", body: "Nothing here.", uses: [], project: "shop" });
    expect(text).toBe("# Hello\n\nNothing here.\n\n## Language used\n\n_No recorded term of shop is used._\n");
  });
});
