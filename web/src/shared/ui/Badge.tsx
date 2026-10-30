import type { ReactNode } from "react";

export type BadgeTone = "accepted" | "proposed" | "draft" | "removed" | "muted";

/** A small pill naming a state in words, such as an entry's standing. */
export function Badge({ tone, children, title }: { tone: BadgeTone; children: ReactNode; title?: string }) {
  return (
    <span className={`st-badge st-badge--${tone}`} title={title}>
      {children}
    </span>
  );
}
