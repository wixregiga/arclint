/**
 * A fresh id for one annotation. Feature ids and revisions are the
 * server's; an annotation's id only tells one annotation of a body from
 * another, so the browser mints it where the annotation is made.
 */
export function newReferenceId(): string {
  const c = globalThis.crypto;
  if (c && typeof c.randomUUID === "function") return c.randomUUID();
  return `ref-${Date.now().toString(36)}-${Math.random().toString(36).slice(2, 10)}`;
}
