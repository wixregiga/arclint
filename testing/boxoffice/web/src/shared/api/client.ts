// The one HTTP client the web side speaks through: JSON in, JSON
// out, errors surfaced as ApiError.
export class ApiError extends Error {
  constructor(
    readonly status: number,
    message: string,
  ) {
    super(message);
  }
}

let organizerToken = "";

// setOrganizerToken unlocks the organizer's side of the counter for
// later calls.
export function setOrganizerToken(token: string): void {
  organizerToken = token;
}

// hasOrganizerToken says whether this session has unlocked the
// organizer's side already.
export function hasOrganizerToken(): boolean {
  return organizerToken !== "";
}

async function request<T>(method: string, path: string, body?: unknown): Promise<T> {
  const headers: Record<string, string> = { "Content-Type": "application/json" };
  if (organizerToken) headers.Authorization = `Bearer ${organizerToken}`;
  const res = await fetch(path, {
    method,
    headers,
    body: body === undefined ? undefined : JSON.stringify(body),
  });
  const text = await res.text();
  const data: unknown = text ? JSON.parse(text) : null;
  if (!res.ok) {
    const message =
      data && typeof data === "object" && "error" in data && typeof data.error === "string"
        ? data.error
        : res.statusText;
    throw new ApiError(res.status, message);
  }
  return data as T;
}

export const api = {
  get: <T>(path: string) => request<T>("GET", path),
  post: <T>(path: string, body?: unknown) => request<T>("POST", path, body),
  put: <T>(path: string, body?: unknown) => request<T>("PUT", path, body),
};
