// @vitest-environment jsdom
import { describe, expect, it } from "vitest";
import type { AcceptedDocument, OverviewDocument, StudioGateway } from "../../shared/transport/index";

const counts = { contexts: 1, aggregates: 0, entities: 0, valueObjects: 1, invariants: 0, assertions: 0, specifications: 0, events: 0, services: 0, questions: 0, relations: 0 };

function gatewayFor(project: string): StudioGateway {
  const overview: OverviewDocument = {
    source: "domain.arclint.yaml",
    found: true,
    project,
    description: `${project} described.`,
    counts,
    contexts: [{ name: `${project}-context`, definition: "A context.", valueObjects: [{ name: "Money", definition: "An amount." }] }],
  };
  const accepted: AcceptedDocument = { available: false, reason: "no remote default branch" };
  return {
    overview: () => Promise.resolve(overview),
    accepted: () => Promise.resolve(accepted),
    define: () => Promise.reject(new Error("not expected")),
    remove: () => Promise.reject(new Error("not expected")),
    describeProject: () => Promise.reject(new Error("not expected")),
  };
}

async function until(check: () => boolean, what: string): Promise<void> {
  for (let i = 0; i < 200; i++) {
    if (check()) return;
    await new Promise((r) => setTimeout(r, 10));
  }
  throw new Error(`timed out waiting for ${what}`);
}

describe("mountStudio", () => {
  it("changes nothing on import", async () => {
    const before = document.documentElement.outerHTML;
    const module = await import("./mount-web");
    expect(typeof module.mountStudio).toBe("function");
    expect(document.documentElement.outerHTML).toBe(before);
    expect(document.querySelectorAll("style").length).toBe(0);
    expect("serviceWorker" in navigator && (navigator as Navigator & { serviceWorker?: { controller: unknown } }).serviceWorker?.controller).toBeFalsy();
  });

  it("mounts into a host container inside a shadow root, disposes, and mounts again", async () => {
    const { mountStudio } = await import("./mount-web");
    const host = document.createElement("div");
    host.id = "host";
    document.body.append(host);
    const mount = mountStudio(host, { gateway: gatewayFor("alpha") });
    expect(host.shadowRoot).not.toBeNull();
    expect(host.shadowRoot!.querySelector("style")!.textContent).toContain(".st-root");
    expect(document.head.querySelector("style")).toBeNull();
    await until(() => (mount.root.textContent ?? "").includes("alpha described."), "the project to render");
    expect(host.children.length).toBe(0);
    expect(document.body.className).toBe("");

    mount.dispose();
    expect(host.shadowRoot!.childNodes.length).toBe(0);
    mount.dispose();

    const again = mountStudio(host, { gateway: gatewayFor("beta") });
    await until(() => (again.root.textContent ?? "").includes("beta described."), "the second project to render");
    expect(again.root.textContent).not.toContain("alpha");
    again.dispose();
    host.remove();
  });

  it("runs two independent instances with their own vocabularies and memories", async () => {
    const { mountStudio } = await import("./mount-web");
    const a = document.createElement("section");
    const b = document.createElement("section");
    document.body.append(a, b);
    const first = mountStudio(a, { gateway: gatewayFor("shop") });
    const second = mountStudio(b, { gateway: gatewayFor("clinic") });
    await until(() => (first.root.textContent ?? "").includes("shop-context") && (second.root.textContent ?? "").includes("clinic-context"), "both to render");
    expect(first.root.textContent).not.toContain("clinic");
    expect(second.root.textContent).not.toContain("shop");
    expect(window.sessionStorage.length).toBe(0);
    expect(window.localStorage.length).toBe(0);
    first.dispose();
    await until(() => (second.root.textContent ?? "").includes("clinic-context"), "the second to stay");
    second.dispose();
    a.remove();
    b.remove();
  });

  it("lends the language alone when a host gives a gateway and no store: no feature request, and Features says so", async () => {
    const { mountStudio } = await import("./mount-web");
    const seen: string[] = [];
    const fetchSpy = ((input: RequestInfo | URL) => {
      seen.push(String(input));
      return Promise.reject(new TypeError("no network in this host"));
    }) as typeof fetch;
    const host = document.createElement("div");
    document.body.append(host);
    const mount = mountStudio(host, { gateway: gatewayFor("alpha"), fetch: fetchSpy });
    await until(() => (mount.root.textContent ?? "").includes("alpha described."), "the project to render");
    const tab = Array.from(mount.root.querySelectorAll<HTMLButtonElement>('[role="tab"]')).find((b) => b.textContent === "Features")!;
    tab.click();
    await until(() => (mount.root.textContent ?? "").includes("lends no feature store"), "the Features view to say there is no store");
    expect(seen).toEqual([]);
    mount.dispose();
    host.remove();
  });

  it("reads no feature until Features opens, then lists them under the same base", async () => {
    const { mountStudio } = await import("./mount-web");
    const seen: string[] = [];
    const fetchSpy = ((input: RequestInfo | URL) => {
      const url = String(input);
      seen.push(new URL(url).pathname);
      if (url.endsWith("api/features")) {
        return Promise.resolve(new Response(JSON.stringify({ features: [{ id: "f-1", title: "Refund a ticket", revision: "r1" }] }), { status: 200, headers: { "Content-Type": "application/json" } }));
      }
      if (url.endsWith("api/overview")) {
        return Promise.resolve(new Response(JSON.stringify({ source: "x", found: true, project: "gamma", counts, contexts: [] }), { status: 200, headers: { "Content-Type": "application/json" } }));
      }
      return Promise.resolve(new Response(JSON.stringify({ available: false, reason: "none" }), { status: 200, headers: { "Content-Type": "application/json" } }));
    }) as typeof fetch;
    const host = document.createElement("div");
    document.body.append(host);
    const mount = mountStudio(host, { apiBase: "/nested/studio/", fetch: fetchSpy });
    await until(() => (mount.root.textContent ?? "").includes("gamma"), "the project to render");
    expect(seen.sort()).toEqual(["/nested/studio/api/accepted", "/nested/studio/api/overview"]);
    const tab = Array.from(mount.root.querySelectorAll<HTMLButtonElement>('[role="tab"]')).find((b) => b.textContent === "Features")!;
    tab.click();
    await until(() => (mount.root.textContent ?? "").includes("Refund a ticket"), "the saved feature to be listed");
    expect(seen).toContain("/nested/studio/api/features");
    mount.dispose();
    host.remove();
  });

  it("builds its gateway under the host's base when only a base is lent", async () => {
    const { mountStudio } = await import("./mount-web");
    const seen: string[] = [];
    const fetchSpy = ((input: RequestInfo | URL) => {
      seen.push(String(input));
      return Promise.resolve(new Response(JSON.stringify({ source: "x", found: false, counts }), { status: 200, headers: { "Content-Type": "application/json" } }));
    }) as typeof fetch;
    const host = document.createElement("div");
    document.body.append(host);
    const mount = mountStudio(host, { apiBase: "/nested/studio/", fetch: fetchSpy });
    await until(() => seen.length >= 2, "the two reads");
    expect(seen.map((u) => new URL(u).pathname).sort()).toEqual(["/nested/studio/api/accepted", "/nested/studio/api/overview"]);
    mount.dispose();
    host.remove();
  });
});
