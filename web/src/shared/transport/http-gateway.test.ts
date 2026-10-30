import { describe, expect, it } from "vitest";
import { GatewayFailure } from "./gateway";
import { apiUrl } from "./http-call";
import { createHttpGateway } from "./http-gateway";

function answering(handler: (url: string, init?: RequestInit) => Response | Promise<Response>): typeof fetch {
  return ((input: RequestInfo | URL, init?: RequestInit) => Promise.resolve(handler(String(input), init))) as typeof fetch;
}

const json = (status: number, body: unknown) =>
  new Response(JSON.stringify(body), { status, headers: { "Content-Type": "application/json; charset=utf-8" } });

describe("apiUrl", () => {
  it("keeps every path under the base, at the origin root and nested", () => {
    expect(apiUrl("http://localhost:7000/", "api/overview")).toBe("http://localhost:7000/api/overview");
    expect(apiUrl("http://localhost:7000/nested/studio/", "api/define")).toBe("http://localhost:7000/nested/studio/api/define");
    expect(apiUrl("http://localhost:7000/nested/studio", "api/accepted")).toBe("http://localhost:7000/nested/studio/api/accepted");
  });
});

describe("createHttpGateway", () => {
  it("reads the overview and the accepted recording", async () => {
    const seen: string[] = [];
    const gateway = createHttpGateway({
      base: "http://host/studio/",
      fetch: answering((url) => {
        seen.push(url);
        if (url.endsWith("api/overview")) return json(200, { source: "domain.arclint.yaml", found: true, counts: {} });
        return json(200, { available: false, reason: "no remote default branch" });
      }),
    });
    const overview = await gateway.overview();
    const accepted = await gateway.accepted();
    expect(overview.found).toBe(true);
    expect(accepted.available).toBe(false);
    expect(seen).toEqual(["http://host/studio/api/overview", "http://host/studio/api/accepted"]);
  });

  it("posts an edit as JSON and returns the define report", async () => {
    let received: { url: string; init?: RequestInit } | null = null;
    const gateway = createHttpGateway({
      base: "http://host/",
      fetch: answering((url, init) => {
        received = { url, init };
        return json(200, { result: "recorded", type: "value_object", name: "Money", context: "sales", changed: ["definition"] });
      }),
    });
    const report = await gateway.define({ concept: "value_object", context: "sales", owner: "", name: "Money", change: { definition: "An amount." } });
    expect(report.result).toBe("recorded");
    expect(received!.url).toBe("http://host/api/define");
    expect(received!.init?.method).toBe("POST");
    expect((received!.init?.headers as Record<string, string>)["Content-Type"]).toBe("application/json");
    expect(JSON.parse(String(received!.init?.body))).toEqual({ concept: "value_object", context: "sales", owner: "", name: "Money", change: { definition: "An amount." } });
  });

  it("posts a project edit with only the properties given", async () => {
    let body = "";
    const gateway = createHttpGateway({
      base: "http://host/",
      fetch: answering((_url, init) => {
        body = String(init?.body);
        return json(200, { result: "updated", type: "project", changed: ["description"] });
      }),
    });
    await gateway.describeProject({ description: "A shop." });
    expect(JSON.parse(body)).toEqual({ description: "A shop." });
  });

  it("surfaces a refusal with the server's own words and status", async () => {
    const gateway = createHttpGateway({
      base: "http://host/",
      fetch: answering(() => json(422, { error: "invariant/keyed-within-owner: key is not lowercase kebab-case" })),
    });
    const failure = await gateway.define({ concept: "invariant", context: "", owner: "Order", name: "Bad Key", change: { statement: "x" } }).catch((e: unknown) => e);
    expect(failure).toBeInstanceOf(GatewayFailure);
    expect((failure as GatewayFailure).kind).toBe("refused");
    expect((failure as GatewayFailure).status).toBe(422);
    expect((failure as GatewayFailure).message).toContain("kebab-case");
  });

  it("tells an unreachable server apart from a route the server lacks", async () => {
    const down = createHttpGateway({ base: "http://host/", fetch: (() => Promise.reject(new TypeError("Failed to fetch"))) as typeof fetch });
    const downFailure = (await down.overview().catch((e: unknown) => e)) as GatewayFailure;
    expect(downFailure.kind).toBe("unreachable");

    const old = createHttpGateway({ base: "http://host/", fetch: answering(() => new Response("404 page not found\n", { status: 404 })) });
    const oldFailure = (await old.describeProject({ name: "x" }).catch((e: unknown) => e)) as GatewayFailure;
    expect(oldFailure.kind).toBe("unexpected");
    expect(oldFailure.status).toBe(404);
    expect(oldFailure.message).toContain("404");
  });
});
