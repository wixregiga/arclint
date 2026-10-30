/*
 * A stand-in for the Go feature store following docs/studio-feature-writing.md:
 * server-minted ids, revisions computed from persisted content, 404 for an
 * unknown id, 409 and no write for a stale revision. It exists so the Web
 * side can be exercised before the Go routes are integrated; the parent's
 * persistence suite replaces it with the real server. Browser tests that
 * use it say so in their titles.
 */

export interface StoredFeature {
  id: string;
  revision: string;
  title: string;
  body: string;
  references: unknown[];
}

export interface StubAnswer {
  status: number;
  body: unknown;
}

function hash(text: string): string {
  let h = 2166136261;
  for (let i = 0; i < text.length; i++) {
    h ^= text.charCodeAt(i);
    h = Math.imul(h, 16777619) >>> 0;
  }
  return `rev-${h.toString(16)}`;
}

export class FeatureStoreStub {
  readonly files = new Map<string, StoredFeature>();
  private counter = 0;
  readonly log: string[] = [];

  list(): StubAnswer {
    this.log.push("GET api/features");
    const features = [...this.files.values()].sort((a, b) => a.id.localeCompare(b.id)).map(({ id, title, revision }) => ({ id, title, revision }));
    return { status: 200, body: { features } };
  }

  read(id: string | null): StubAnswer {
    this.log.push(`GET api/feature?id=${id}`);
    const file = id ? this.files.get(id) : undefined;
    if (!file) return { status: 404, body: { error: `no feature ${id ?? ""}` } };
    return { status: 200, body: { feature: file } };
  }

  save(raw: unknown): StubAnswer {
    this.log.push("POST api/feature");
    if (!raw || typeof raw !== "object") return { status: 400, body: { error: "a JSON object is required" } };
    const r = raw as { id?: unknown; revision?: unknown; title?: unknown; body?: unknown; references?: unknown };
    if (typeof r.title !== "string" || typeof r.body !== "string" || !Array.isArray(r.references)) return { status: 400, body: { error: "title, body and references are required" } };
    const content = { title: r.title, body: r.body, references: r.references };
    if (r.id === undefined) {
      const id = `feature-${++this.counter}`;
      const file: StoredFeature = { id, revision: hash(JSON.stringify(content)), ...content };
      this.files.set(id, file);
      return { status: 200, body: { feature: file } };
    }
    if (typeof r.id !== "string") return { status: 400, body: { error: "id is a string" } };
    const existing = this.files.get(r.id);
    if (!existing) return { status: 404, body: { error: `no feature ${r.id}` } };
    if (r.revision !== existing.revision) return { status: 409, body: { error: `feature ${r.id} changed since it was read` } };
    const file: StoredFeature = { id: r.id, revision: hash(JSON.stringify(content)), ...content };
    this.files.set(r.id, file);
    return { status: 200, body: { feature: file } };
  }

  remove(id: string | null, ifMatch: string | null): StubAnswer {
    this.log.push(`DELETE api/feature?id=${id}`);
    const file = id ? this.files.get(id) : undefined;
    if (!file) return { status: 404, body: { error: `no feature ${id ?? ""}` } };
    if (ifMatch !== file.revision) return { status: 409, body: { error: `feature ${id} changed since it was read` } };
    this.files.delete(id!);
    return { status: 200, body: { removed: id } };
  }

  /** Changes a stored feature behind the client's back, as another browser would. */
  edit(id: string, title: string): void {
    const file = this.files.get(id)!;
    const content = { title, body: file.body, references: file.references };
    this.files.set(id, { id, revision: hash(JSON.stringify(content)), ...content });
  }

  /** Answers one request by method, path and body, as the HTTP route would. */
  answer(method: string, url: URL, body: string | null, ifMatch: string | null = null): StubAnswer | null {
    const path = url.pathname.replace(/^.*\/api\//, "api/");
    if (path === "api/features" && method === "GET") return this.list();
    if (path !== "api/feature") return null;
    if (method === "GET") return this.read(url.searchParams.get("id"));
    if (method === "DELETE") return this.remove(url.searchParams.get("id"), ifMatch);
    if (method === "POST") {
      try {
        return this.save(body === null ? null : JSON.parse(body));
      } catch {
        return { status: 400, body: { error: "the request is not JSON" } };
      }
    }
    return null;
  }
}
