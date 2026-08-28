// Selection state: how many seats of each tier someone is deciding
// on, before any hold exists.
export type Selection = Record<string, number>;

export function adjust(selection: Selection, tier: string, by: number): Selection {
  const next = { ...selection, [tier]: Math.max(0, (selection[tier] ?? 0) + by) };
  if (next[tier] === 0) delete next[tier];
  return next;
}

export function picked(selection: Selection): [string, number][] {
  return Object.entries(selection);
}
