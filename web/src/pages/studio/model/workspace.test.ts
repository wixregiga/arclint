import { describe, expect, it } from "vitest";
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
    expect(restoreWorkspace(JSON.stringify({ version: 1, selection: { kind: "new", concept: "nonsense" }, drafts: { a: { x: 1, y: "kept" } } }))).toEqual({
      ...initialWorkspace,
      drafts: { a: { y: "kept" } },
    });
  });
});
