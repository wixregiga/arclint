import { describe, expect, it } from "vitest";
import { featureDraftKey } from "./feature-drafts";
import { draftKeyOf, initialWorkspace, reduceWorkspace, restoreWorkspace, serializeWorkspace, type Workspace } from "./workspace";

describe("workspace", () => {
  it("keeps a draft through selection and mode changes until it is cancelled or saved", () => {
    let w: Workspace = initialWorkspace;
    w = reduceWorkspace(w, { type: "select", selection: { kind: "entry", key: "k1" } });
    w = reduceWorkspace(w, { type: "begin-edit", key: "k1", values: { definition: "Old." } });
    w = reduceWorkspace(w, { type: "draft", key: "k1", field: "definition", value: "New text" });
    w = reduceWorkspace(w, { type: "select", selection: { kind: "entry", key: "k2" } });
    w = reduceWorkspace(w, { type: "mode", mode: "map" });
    expect(w.editing).toBe(false);
    expect(w.drafts.k1).toEqual({ definition: "New text" });
    w = reduceWorkspace(w, { type: "select", selection: { kind: "entry", key: "k1" } });
    w = reduceWorkspace(w, { type: "begin-edit", key: "k1", values: { definition: "Old." } });
    expect(w.drafts.k1).toEqual({ definition: "New text" });
    w = reduceWorkspace(w, { type: "saved", key: "k1", then: { kind: "entry", key: "k1" } });
    expect(w.drafts.k1).toBeUndefined();
    expect(w.editing).toBe(false);
  });

  it("returns from a cancelled new entry to where it was begun", () => {
    let w = reduceWorkspace(initialWorkspace, { type: "select", selection: { kind: "new", concept: "value_object", context: "sales", owner: "" } });
    expect(w.editing).toBe(true);
    const key = draftKeyOf(w.selection);
    w = reduceWorkspace(w, { type: "draft", key, field: "$name", value: "Money" });
    w = reduceWorkspace(w, { type: "cancel-edit", key, then: { kind: "entry", key: "ctx" } });
    expect(w.selection).toEqual({ kind: "entry", key: "ctx" });
    expect(w.drafts[key]).toBeUndefined();
  });

  it("survives a round trip through memory and ignores what it cannot read", () => {
    let w = reduceWorkspace(initialWorkspace, { type: "select", selection: { kind: "new", concept: "aggregate", context: "sales", owner: "" } });
    w = reduceWorkspace(w, { type: "draft", key: draftKeyOf(w.selection), field: "definition", value: "An order." });
    w = reduceWorkspace(w, { type: "mode", mode: "map" });
    expect(restoreWorkspace(serializeWorkspace(w))).toEqual(w);
    expect(restoreWorkspace(null)).toEqual(initialWorkspace);
    expect(restoreWorkspace("{not json")).toEqual(initialWorkspace);
    expect(restoreWorkspace(JSON.stringify({ version: 2, selection: { kind: "new", concept: "nonsense" }, drafts: { a: { x: 1, y: "kept" } } }))).toEqual({
      ...initialWorkspace,
      drafts: { a: { y: "kept" } },
    });
    expect(restoreWorkspace(JSON.stringify({ version: 1, mode: "map", drafts: { a: { y: "kept" } } }))).toMatchObject({ mode: "map", drafts: { a: { y: "kept" } }, feature: null, featureDrafts: {} });
  });

  it("keeps a feature draft across modes and drops it only when saved, discarded or removed", () => {
    const feature = { kind: "new", key: "k" } as const;
    const base = { title: "", body: "", references: [], revision: null };
    let w = reduceWorkspace(initialWorkspace, { type: "mode", mode: "features" });
    w = reduceWorkspace(w, { type: "open-feature", feature });
    w = reduceWorkspace(w, { type: "feature-draft", feature, base, body: "An Order" });
    w = reduceWorkspace(w, { type: "feature-draft", feature, base, title: "Orders" });
    w = reduceWorkspace(w, { type: "mode", mode: "language" });
    w = reduceWorkspace(w, { type: "mode", mode: "features" });
    expect(w.feature).toEqual(feature);
    expect(w.featureDrafts[featureDraftKey(feature)]).toEqual({ title: "Orders", body: "An Order", references: [], revision: null });

    const round = restoreWorkspace(serializeWorkspace(w));
    expect(round).toEqual(w);

    const saved = reduceWorkspace(w, { type: "feature-saved", feature, document: { id: "f-1", revision: "r1", title: "Orders", body: "An Order", references: [] } });
    expect(saved.feature).toEqual({ kind: "saved", id: "f-1" });
    expect(saved.featureDrafts).toEqual({});

    const savedFeature = { kind: "saved", id: "f-1" } as const;
    const reference = { id: "r", from: 3, to: 8, quote: "Order", reading: "names" as const, term: { concept: "aggregate", context: "sales", owner: "", name: "Order" } };
    let s = reduceWorkspace(saved, { type: "feature-draft", feature: savedFeature, base: { ...base, revision: "r1" }, references: [reference] });
    expect(s.featureDrafts[featureDraftKey(savedFeature)].revision).toBe("r1");
    s = reduceWorkspace(s, { type: "feature-rebase", feature: savedFeature, revision: "r2" });
    expect(s.featureDrafts[featureDraftKey(savedFeature)].revision).toBe("r2");
    expect(restoreWorkspace(serializeWorkspace(s)).featureDrafts[featureDraftKey(savedFeature)].references).toEqual([reference]);
    expect(reduceWorkspace(s, { type: "feature-discard", feature: savedFeature }).featureDrafts).toEqual({});
    const removed = reduceWorkspace(s, { type: "feature-removed", feature: savedFeature });
    expect(removed.feature).toBeNull();
    expect(removed.featureDrafts).toEqual({});
  });

  it("drops feature drafts and references it cannot read", () => {
    const text = JSON.stringify({
      version: 2,
      feature: { kind: "saved", id: 7 },
      featureDrafts: {
        ok: { title: "t", body: "b", references: [{ id: "a", from: 0, to: 1, quote: "b", reading: "names", term: { concept: "aggregate", context: "c", owner: "", name: "N" } }, { id: "bad", from: "x" }, { id: "p", from: 0, to: 1, quote: "b", reading: "weird" }] },
        bad: { title: 1 },
      },
    });
    const w = restoreWorkspace(text);
    expect(w.feature).toBeNull();
    expect(Object.keys(w.featureDrafts)).toEqual(["ok"]);
    expect(w.featureDrafts.ok.references.map((r) => r.id)).toEqual(["a"]);
    expect(w.featureDrafts.ok.revision).toBeNull();
  });
});


it("retains text typed during a save and stays on the feature now open", () => {
  const feature = { kind: "new" as const, key: "pending" };
  const base = { title: "Sent title", body: "Sent body", references: [], revision: null };
  let w = reduceWorkspace(initialWorkspace, { type: "open-feature", feature });
  w = reduceWorkspace(w, { type: "feature-draft", feature, base, body: "Sent body and newer typing" });
  w = reduceWorkspace(w, { type: "open-feature", feature: { kind: "new", key: "another" } });
  const saved = reduceWorkspace(w, { type: "feature-saved", feature, document: { ...base, id: "saved-id", revision: "r1" } });
  expect(saved.feature).toEqual({ kind: "new", key: "another" });
  expect(saved.featureDrafts[featureDraftKey({ kind: "saved", id: "saved-id" })]).toMatchObject({ body: "Sent body and newer typing", revision: "r1" });
});
