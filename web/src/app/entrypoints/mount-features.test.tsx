// @vitest-environment jsdom
import { EditorView } from "@codemirror/view";
import { afterEach, beforeEach, describe, expect, it } from "vitest";
import { mountStudio, type StudioMount } from "./mount-web";
import { inMemoryStudioMemory } from "../../shared/lib/studio-memory/index";
import { GatewayFailure, type FeatureDocument, type FeatureStore, type OverviewDocument, type SaveFeatureRequest, type StudioGateway } from "../../shared/transport/index";
import { initialWorkspace, serializeWorkspace } from "../../pages/studio/index";

const counts = { contexts: 1, aggregates: 1, entities: 0, valueObjects: 2, invariants: 0, assertions: 0, specifications: 0, events: 0, services: 0, questions: 0, relations: 0 };

const overview: OverviewDocument = {
  source: "domain.arclint.yaml",
  found: true,
  project: "shop",
  counts,
  contexts: [
    {
      name: "sales",
      definition: "Selling.",
      aggregates: [{ name: "Order", definition: "A customer's request to purchase.", identity: "OrderID" }],
      valueObjects: [
        { name: "OrderID", definition: "What tells one order apart." },
        { name: "Money", definition: "An amount in a currency.", aliases: ["Amount"] },
      ],
    },
    { name: "fulfilment", definition: "Shipping.", valueObjects: [{ name: "Order", definition: "A picking instruction." }] },
  ],
};

const gateway: StudioGateway = {
  overview: () => Promise.resolve(overview),
  accepted: () => Promise.resolve({ available: false, reason: "no remote" }),
  define: () => Promise.reject(new Error("not expected")),
  remove: () => Promise.reject(new Error("not expected")),
  describeProject: () => Promise.reject(new Error("not expected")),
};

/** An in-memory store with the contract's refusals: 404 for an unknown id, 409 and no write for a stale revision. */
function memoryStore() {
  const files = new Map<string, FeatureDocument>();
  let counter = 0;
  let revision = 0;
  const calls: string[] = [];
  const store: FeatureStore = {
    list: () => {
      calls.push("list");
      return Promise.resolve([...files.values()].map(({ id, title, revision }) => ({ id, title, revision })));
    },
    read: (id) => {
      calls.push(`read ${id}`);
      const file = files.get(id);
      return file ? Promise.resolve(file) : Promise.reject(new GatewayFailure("refused", `no feature ${id}`, 404));
    },
    save: (request: SaveFeatureRequest) => {
      calls.push("save");
      if (request.id === undefined) {
        const document: FeatureDocument = { id: `f-${++counter}`, revision: `r${++revision}`, title: request.title, body: request.body, references: request.references };
        files.set(document.id, document);
        return Promise.resolve(document);
      }
      const existing = files.get(request.id);
      if (!existing) return Promise.reject(new GatewayFailure("refused", `no feature ${request.id}`, 404));
      if (existing.revision !== request.revision) return Promise.reject(new GatewayFailure("refused", "changed since it was read", 409));
      const document: FeatureDocument = { id: request.id, revision: `r${++revision}`, title: request.title, body: request.body, references: request.references };
      files.set(document.id, document);
      return Promise.resolve(document);
    },
    remove: (id, rev) => {
      calls.push(`remove ${id}`);
      const existing = files.get(id);
      if (!existing) return Promise.reject(new GatewayFailure("refused", `no feature ${id}`, 404));
      if (existing.revision !== rev) return Promise.reject(new GatewayFailure("refused", "changed since it was read", 409));
      files.delete(id);
      return Promise.resolve();
    },
  };
  return { store, files, calls, bump: (id: string, title: string) => files.set(id, { ...files.get(id)!, title, revision: `r${++revision}` }) };
}

async function until(check: () => boolean, what: string): Promise<void> {
  for (let i = 0; i < 300; i++) {
    if (check()) return;
    await new Promise((r) => setTimeout(r, 10));
  }
  throw new Error(`timed out waiting for ${what}`);
}

const ORDER = { concept: "aggregate", context: "sales", owner: "", name: "Order" };
const BODY = "An Order holds Money and ships.";

function click(root: HTMLElement, name: string): void {
  const button = Array.from(root.querySelectorAll<HTMLButtonElement>("button")).find((b) => b.textContent?.trim() === name);
  if (!button) throw new Error(`no button “${name}” among ${Array.from(root.querySelectorAll("button")).map((b) => b.textContent?.trim()).join(", ")}`);
  button.click();
}

describe("Studio mounted in Features mode over an injected store", () => {
  let host: HTMLDivElement;
  let mount: StudioMount | null = null;

  beforeEach(() => {
    // jsdom reports Apple as its vendor, has no execCommand and measures no layout; CodeMirror's Safari
    // selection path and its text measurement need these to exist. They affect nothing asserted here.
    if (typeof document.execCommand !== "function") document.execCommand = () => false;
    const rect = { x: 0, y: 0, top: 0, left: 0, bottom: 0, right: 0, width: 0, height: 0, toJSON: () => ({}) } as DOMRect;
    const rects = () => ({ length: 0, item: () => null, [Symbol.iterator]: [][Symbol.iterator] }) as unknown as DOMRectList;
    if (typeof Range.prototype.getClientRects !== "function") {
      Range.prototype.getClientRects = rects;
      Range.prototype.getBoundingClientRect = () => rect;
    }
    host = document.createElement("div");
    document.body.append(host);
  });

  afterEach(() => {
    mount?.dispose();
    mount = null;
    host.remove();
  });

  function open(store: FeatureStore, feature: { kind: "saved"; id: string } | null) {
    const memory = inMemoryStudioMemory();
    memory.write(serializeWorkspace({ ...initialWorkspace, mode: "features", feature }));
    mount = mountStudio(host, { gateway, featureStore: store, memory });
    return { root: mount.root, memory };
  }

  it("opens a saved feature, draws its uses, shows the definition at the cursor and ties a cue through the store", async () => {
    const { store, files, calls } = memoryStore();
    files.set("f-1", { id: "f-1", revision: "r0", title: "Ship an order", body: BODY, references: [{ id: "a", from: 3, to: 8, quote: "Order", reading: "names", term: ORDER }] });
    const { root } = open(store, { kind: "saved", id: "f-1" });
    await until(() => root.querySelector(".cm-content") !== null, "the editor");
    await until(() => root.querySelectorAll(".st-use").length === 2, "two uses drawn");
    const uses = Array.from(root.querySelectorAll<HTMLElement>(".st-use")).map((u) => [u.dataset.use, u.textContent]);
    expect(uses).toEqual([
      ["tied", "Order"],
      ["cue", "Money"],
    ]);
    expect(root.querySelector(".st-use-mark .st-kind-badge")?.getAttribute("aria-label")).toBe("Aggregate Order");
    expect((root.querySelector('input[aria-label="Title"]') as HTMLInputElement).value).toBe("Ship an order");
    expect([...new Set(calls)]).toEqual(["list", "read f-1"]);

    // The inspector follows the cursor: put it inside "Money".
    const cm = EditorView.findFromDOM(root.querySelector(".cm-editor") as HTMLElement)!;
    cm.dispatch({ selection: { anchor: BODY.indexOf("Money") + 2 } });
    await until(() => (root.querySelector('[aria-label="At the cursor"]')?.textContent ?? "").includes("An amount in a currency."), "the definition of Money");
    expect(root.querySelector('[aria-label="At the cursor"]')!.textContent).toContain("Recognised, not confirmed");
    expect(root.querySelector('[aria-label="At the cursor"]')!.textContent).toContain("Amount");

    click(root.querySelector('[aria-label="At the cursor"]') as HTMLElement, "Tie it");
    await until(() => root.querySelectorAll(".st-use--tied").length === 2, "Money tied");
    expect(root.textContent).toContain("unsaved");
    click(root, "Save");
    await until(() => !(root.textContent ?? "").includes("unsaved"), "the save to go through");
    const saved = files.get("f-1")!;
    expect(saved.revision).not.toBe("r0");
    expect(saved.references.map((r) => [r.quote, r.from, r.to, r.reading, r.term?.name])).toEqual([
      ["Order", 3, 8, "names", "Order"],
      ["Money", 15, 20, "names", "Money"],
    ]);
  });

  it("keeps the typed text on a stale save and offers to write over or take the stored text", async () => {
    const { store, files, bump } = memoryStore();
    files.set("f-1", { id: "f-1", revision: "r0", title: "Ship an order", body: BODY, references: [] });
    const { root } = open(store, { kind: "saved", id: "f-1" });
    await until(() => root.querySelector(".cm-content") !== null, "the editor");
    const title = root.querySelector('input[aria-label="Title"]') as HTMLInputElement;
    const setter = Object.getOwnPropertyDescriptor(HTMLInputElement.prototype, "value")!.set!;
    setter.call(title, "Ship an order today");
    title.dispatchEvent(new Event("input", { bubbles: true }));
    await until(() => (root.textContent ?? "").includes("unsaved"), "the draft");

    bump("f-1", "Renamed elsewhere");
    click(root, "Save");
    await until(() => (root.textContent ?? "").includes("changed in the store since you opened it"), "the conflict");
    expect(root.textContent).toContain("now titled “Renamed elsewhere”");
    expect(title.value).toBe("Ship an order today");
    expect(files.get("f-1")!.title).toBe("Renamed elsewhere");

    click(root, "Save mine over it");
    await until(() => files.get("f-1")!.title === "Ship an order today", "the overwrite");
    await until(() => !(root.textContent ?? "").includes("unsaved"), "the draft to clear");
  });

  it("says when a feature vanished from the store and lets the text be saved as new", async () => {
    const { store, files } = memoryStore();
    files.set("f-1", { id: "f-1", revision: "r0", title: "Ship an order", body: BODY, references: [] });
    const { root } = open(store, { kind: "saved", id: "f-1" });
    await until(() => root.querySelector(".cm-content") !== null, "the editor");
    const title = root.querySelector('input[aria-label="Title"]') as HTMLInputElement;
    Object.getOwnPropertyDescriptor(HTMLInputElement.prototype, "value")!.set!.call(title, "Ship it");
    title.dispatchEvent(new Event("input", { bubbles: true }));
    await until(() => (root.textContent ?? "").includes("unsaved"), "the draft");
    files.delete("f-1");
    click(root, "Save");
    await until(() => (root.textContent ?? "").includes("no longer in the store"), "the missing notice");
    click(root, "Save as a new feature");
    await until(() => files.size === 1 && [...files.values()][0].title === "Ship it", "the new feature");
    expect([...files.values()][0].id).not.toBe(undefined);
    expect(root.textContent).not.toContain("no longer in the store");
  });

  it("reports a store that cannot be read and keeps a new feature's text when saving fails", async () => {
    const failing: FeatureStore = {
      list: () => Promise.reject(new GatewayFailure("unreachable", "The map server did not answer (down).")),
      read: () => Promise.reject(new GatewayFailure("unreachable", "down")),
      save: () => Promise.reject(new GatewayFailure("unreachable", "The map server did not answer (down).")),
      remove: () => Promise.reject(new GatewayFailure("unreachable", "down")),
    };
    const { root } = open(failing, null);
    await until(() => (root.textContent ?? "").includes("did not answer"), "the list failure");
    click(root, "+ Feature");
    await until(() => root.querySelector(".cm-content") !== null, "the editor");
    const title = root.querySelector('input[aria-label="Title"]') as HTMLInputElement;
    Object.getOwnPropertyDescriptor(HTMLInputElement.prototype, "value")!.set!.call(title, "Refunds");
    title.dispatchEvent(new Event("input", { bubbles: true }));
    click(root, "Save");
    await until(() => (root.querySelector(".st-card--feature [role=alert]")?.textContent ?? "").includes("Your text is kept"), "the failure beside the text");
    expect(title.value).toBe("Refunds");
    expect(Array.from(root.querySelectorAll("button")).some((b) => b.textContent === "Try again")).toBe(true);
  });
});
