import { describe, expect, it } from "vitest";
import { isMissingFeature, isStaleRevision } from "./feature-store";
import { GatewayFailure } from "./gateway";
import { createHttpFeatureStore } from "./http-feature-store";

function answering(handler: (url: string, init?: RequestInit) => Response | Promise<Response>): typeof fetch {
  return ((input: RequestInfo | URL, init?: RequestInit) => Promise.resolve(handler(String(input), init))) as typeof fetch;
}

const json = (status: number, body: unknown) => new Response(JSON.stringify(body), { status, headers: { "Content-Type": "application/json; charset=utf-8" } });

const document = { id: "f-1", revision: "r1", title: "Refunds", body: "A Tenant refunds.", references: [] };

describe("createHttpFeatureStore", () => {
  it("lists, reads, saves and removes under a nested base with the contract's paths and headers", async () => {
    const seen: { url: string; init?: RequestInit }[] = [];
    const store = createHttpFeatureStore({
      base: "http://host/nested/studio",
      fetch: answering((url, init) => {
        seen.push({ url, init });
        if (url.endsWith("api/features")) return json(200, { features: [{ id: "f-1", title: "Refunds", revision: "r1" }] });
        if (init?.method === "DELETE") return json(200, { removed: "f-1" });
        return json(200, { feature: document });
      }),
    });
    expect(await store.list()).toEqual([{ id: "f-1", title: "Refunds", revision: "r1" }]);
    expect(await store.read("f 1/x")).toEqual(document);
    expect(await store.save({ title: "Refunds", body: "A Tenant refunds.", references: [] })).toEqual(document);
    await store.remove("f-1", "r1");

    expect(seen.map((s) => s.url)).toEqual([
      "http://host/nested/studio/api/features",
      "http://host/nested/studio/api/feature?id=f%201%2Fx",
      "http://host/nested/studio/api/feature",
      "http://host/nested/studio/api/feature?id=f-1",
    ]);
    expect(seen[2].init?.method).toBe("POST");
    expect(JSON.parse(String(seen[2].init?.body))).toEqual({ title: "Refunds", body: "A Tenant refunds.", references: [] });
    expect(seen[3].init?.method).toBe("DELETE");
    expect((seen[3].init?.headers as Record<string, string>)["If-Match"]).toBe("r1");
  });

  it("treats a missing features list as empty", async () => {
    const store = createHttpFeatureStore({ base: "http://host/", fetch: answering(() => json(200, {})) });
    expect(await store.list()).toEqual([]);
  });

  it("tells a stale save, a missing draft and a refused request apart by status", async () => {
    const stale = createHttpFeatureStore({ base: "http://host/", fetch: answering(() => json(409, { error: "the draft changed since it was read" })) });
    const staleFailure = await stale.save({ id: "f-1", revision: "old", title: "", body: "", references: [] }).catch((e: unknown) => e);
    expect(staleFailure).toBeInstanceOf(GatewayFailure);
    expect(isStaleRevision(staleFailure)).toBe(true);
    expect(isMissingFeature(staleFailure)).toBe(false);
    expect((staleFailure as GatewayFailure).message).toContain("changed since");

    const gone = createHttpFeatureStore({ base: "http://host/", fetch: answering(() => json(404, { error: "no feature f-9" })) });
    const goneFailure = await gone.read("f-9").catch((e: unknown) => e);
    expect(isMissingFeature(goneFailure)).toBe(true);
    expect(isStaleRevision(goneFailure)).toBe(false);

    const bad = createHttpFeatureStore({ base: "http://host/", fetch: answering(() => json(400, { error: "references[0].from is negative" })) });
    const badFailure = (await bad.save({ title: "", body: "", references: [] }).catch((e: unknown) => e)) as GatewayFailure;
    expect(badFailure.kind).toBe("refused");
    expect(badFailure.status).toBe(400);
  });

  it("reports a server without the feature routes as unexpected, not as a refusal", async () => {
    const old = createHttpFeatureStore({ base: "http://host/", fetch: answering(() => new Response("404 page not found\n", { status: 404 })) });
    const failure = (await old.list().catch((e: unknown) => e)) as GatewayFailure;
    expect(failure.kind).toBe("unexpected");
    expect(isMissingFeature(failure)).toBe(false);
  });
});
