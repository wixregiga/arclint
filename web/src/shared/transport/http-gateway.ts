import type {
  AcceptedDocument,
  DefineReport,
  DefineRequest,
  OverviewDocument,
  ProjectRequest,
  RemoveReport,
  RemoveRequest,
} from "./documents";
import { GatewayFailure, type StudioGateway } from "./gateway";

export interface HttpGatewayOptions {
  /** The absolute URL every API path is relative to; it ends with a slash. */
  base: string;
  fetch?: typeof fetch;
}

/** The API paths the map server answers, relative to the injected base. */
export const API_PATHS = {
  overview: "api/overview",
  accepted: "api/accepted",
  define: "api/define",
  remove: "api/remove",
  project: "api/project",
} as const;

/** `base` with its trailing slash, so `new URL("api/x", base)` stays under it. */
export function normalizedBase(base: string): string {
  return base.endsWith("/") ? base : `${base}/`;
}

export function apiUrl(base: string, path: string): string {
  return new URL(path, normalizedBase(base)).toString();
}

/** The gateway over HTTP, as the Go map server serves it. */
export function createHttpGateway(options: HttpGatewayOptions): StudioGateway {
  const base = normalizedBase(options.base);
  const doFetch = options.fetch ?? globalThis.fetch.bind(globalThis);

  async function call<T>(path: string, init?: RequestInit): Promise<T> {
    let response: Response;
    try {
      response = await doFetch(apiUrl(base, path), { ...init, cache: "no-store" });
    } catch (error) {
      const reason = error instanceof Error ? error.message : String(error);
      throw new GatewayFailure("unreachable", `The map server did not answer (${reason}).`);
    }
    const text = await response.text();
    let body: unknown = null;
    try {
      body = text === "" ? null : JSON.parse(text);
    } catch {
      body = null;
    }
    if (response.ok) {
      if (body === null || typeof body !== "object") {
        throw new GatewayFailure("unexpected", `The map server answered ${response.status} without a JSON document.`, response.status);
      }
      return body as T;
    }
    const message = errorMessage(body);
    if (message !== null) {
      throw new GatewayFailure("refused", message, response.status);
    }
    throw new GatewayFailure(
      "unexpected",
      `The map server answered ${response.status} ${response.statusText || ""}`.trim() + (text ? `: ${text.trim().slice(0, 200)}` : "."),
      response.status,
    );
  }

  function post<T>(path: string, request: unknown): Promise<T> {
    return call<T>(path, {
      method: "POST",
      headers: { "Content-Type": "application/json" },
      body: JSON.stringify(request),
    });
  }

  return {
    overview: () => call<OverviewDocument>(API_PATHS.overview),
    accepted: () => call<AcceptedDocument>(API_PATHS.accepted),
    define: (request: DefineRequest) => post<DefineReport>(API_PATHS.define, request),
    remove: (request: RemoveRequest) => post<RemoveReport>(API_PATHS.remove, request),
    describeProject: (request: ProjectRequest) => post<DefineReport>(API_PATHS.project, request),
  };
}

function errorMessage(body: unknown): string | null {
  if (body !== null && typeof body === "object" && "error" in body && typeof (body as { error: unknown }).error === "string") {
    return (body as { error: string }).error;
  }
  return null;
}
