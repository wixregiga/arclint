import { GatewayFailure } from "./gateway";

/** `base` with its trailing slash, so `new URL("api/x", base)` stays under it. */
export function normalizedBase(base: string): string {
  return base.endsWith("/") ? base : `${base}/`;
}

export function apiUrl(base: string, path: string): string {
  return new URL(path, normalizedBase(base)).toString();
}

export interface HttpCaller {
  call<T>(path: string, init?: RequestInit): Promise<T>;
  post<T>(path: string, request: unknown): Promise<T>;
}

/**
 * One way of calling the map server, shared by the gateway and the
 * feature store: a JSON document on success, a `refused` failure carrying
 * the server's own {error} reason and status, `unreachable` when no
 * answer came, `unexpected` for an answer Studio cannot read.
 */
export function httpCaller(base: string, fetchImpl?: typeof fetch): HttpCaller {
  const root = normalizedBase(base);
  const doFetch = fetchImpl ?? globalThis.fetch.bind(globalThis);

  async function call<T>(path: string, init?: RequestInit): Promise<T> {
    let response: Response;
    try {
      response = await doFetch(apiUrl(root, path), { ...init, cache: "no-store" });
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

  return { call, post };
}

function errorMessage(body: unknown): string | null {
  if (body !== null && typeof body === "object" && "error" in body && typeof (body as { error: unknown }).error === "string") {
    return (body as { error: string }).error;
  }
  return null;
}
