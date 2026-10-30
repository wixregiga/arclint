/**
 * A concept's badge: a filled circle in the concept's color with its short
 * mark in white, after BWrite's kind badge. `role="img"` with the full
 * name as its label, so a screen reader says "Aggregate", not "A". The
 * color comes from the `--kind-<tone>` custom properties in the stylesheet.
 */
export interface KindBadgeProps {
  mark: string;
  label: string;
  tone: string;
  proposed?: boolean;
  size?: "small" | "regular" | "large";
}

export function KindBadge({ mark, label, tone, proposed = false, size = "regular" }: KindBadgeProps) {
  const classes = ["st-kind-badge", `st-kind-badge--${tone}`, `st-kind-badge--${size}`];
  if (proposed) classes.push("st-kind-badge--proposed");
  return (
    <span className={classes.join(" ")} role="img" aria-label={proposed ? `Proposed ${label}` : label} title={label}>
      {mark}
    </span>
  );
}
