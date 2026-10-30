/*
 * Positions of one bounded context's entries on the map, in map units.
 * Adapted from ArcLint's static Studio (internal/delivery/mapserve/
 * static/studio.js, MPL-2.0, same repository), as pure functions over
 * Entry values so the layout is tested without a browser.
 *
 * One column per aggregate: the aggregate at the top, its entities in a
 * row below, its invariants and assertions down the left, the events it
 * raises down the right. Then a column of value objects with their
 * invariants, one of domain services with their assertions, and one of
 * what remains: events no aggregate raises, specifications and questions.
 * A column wraps into the next one when it reaches a wrap height; with a
 * viewport, the wrap height is the one whose map fits that viewport at
 * the largest scale. Lines connect each owner to its members, each
 * aggregate to the events it raises, and each identity to the value
 * object it names.
 */
import { membersOf, ofConcept, textOf, type Entry } from "../model/entry";

export interface MapNode {
  key: string;
  x: number;
  y: number;
  align: "centre" | "left" | "right";
  maxWidth: number;
  /** A member row: a dot beside its name rather than a dot above it. */
  member: boolean;
}

export type MapLineKind = "owner" | "raises" | "identity";

export interface MapLine {
  kind: MapLineKind;
  x1: number;
  y1: number;
  x2: number;
  y2: number;
}

export interface MapLayout {
  width: number;
  height: number;
  nodes: MapNode[];
  lines: MapLine[];
}

export interface Viewport {
  width: number;
  height: number;
}

const MARGIN = { width: 48, height: 96 };
const DOT = 12;
const ROW = 26;

export function fit(layout: MapLayout, viewport: Viewport): number {
  return Math.min(1, (viewport.width - MARGIN.width) / layout.width, (viewport.height - MARGIN.height) / layout.height);
}

export function layoutContext(entries: readonly Entry[], context: string, viewport?: Viewport): MapLayout {
  const tallest = layoutWrapped(entries, context, Number.POSITIVE_INFINITY);
  if (!viewport || !(viewport.width > MARGIN.width) || !(viewport.height > MARGIN.height)) return tallest;
  let best = tallest;
  let bestFit = fit(tallest, viewport);
  for (let wrap = 250; wrap < tallest.height; wrap += 40) {
    const candidate = layoutWrapped(entries, context, wrap);
    const f = fit(candidate, viewport);
    if (f > bestFit) {
      best = candidate;
      bestFit = f;
    }
  }
  return best;
}

function layoutWrapped(entries: readonly Entry[], context: string, columnHeight: number): MapLayout {
  const nodes = new Map<string, MapNode>();
  const lines: MapLine[] = [];
  let x = 0;
  let height = 250;
  const aggregates = ofConcept(entries, context, "aggregate");
  const columnWidth = aggregates.length > 1 ? 300 : 440;

  function place(entry: Entry, node: Omit<MapNode, "key">) {
    nodes.set(entry.key, { key: entry.key, ...node });
  }
  function branch(ownerX: number, ownerY: number, left: number, members: readonly Entry[]) {
    if (members.length === 0) return;
    const spine = left + 14;
    const last = nodes.get(members[members.length - 1].key)!;
    lines.push({ kind: "owner", x1: ownerX - DOT / 2, y1: ownerY + DOT / 2, x2: spine, y2: ownerY + DOT / 2 });
    lines.push({ kind: "owner", x1: spine, y1: ownerY + DOT / 2, x2: spine, y2: last.y + DOT / 2 });
    for (const m of members) {
      const at = nodes.get(m.key)!;
      lines.push({ kind: "owner", x1: spine, y1: at.y + DOT / 2, x2: left + 24, y2: at.y + DOT / 2 });
    }
  }

  const raisedByAggregate = (a: Entry) => ofConcept(entries, context, "domain_event").filter((e) => textOf(e, "raisedBy") === a.name);

  for (const a of aggregates) {
    const centre = x + columnWidth / 2;
    place(a, { x: centre, y: 46, align: "centre", maxWidth: columnWidth - 40, member: false });
    const ents = membersOf(entries, a).filter((m) => m.concept === "entity");
    ents.forEach((e, i) => {
      const ex = centre + (i - (ents.length - 1) / 2) * Math.min(96, (columnWidth - 40) / Math.max(1, ents.length));
      place(e, { x: ex, y: 162, align: "centre", maxWidth: 96, member: false });
      lines.push({ kind: "owner", x1: centre, y1: 70, x2: ex, y2: 170 });
    });
    const rules = membersOf(entries, a).filter((m) => m.concept !== "entity");
    rules.forEach((r, i) => place(r, { x: x + 24, y: 200 + i * ROW, align: "left", maxWidth: columnWidth / 2 - 30, member: true }));
    branch(centre, 46, x, rules);
    const events = raisedByAggregate(a);
    events.forEach((e, i) => {
      place(e, { x: x + columnWidth - 24, y: 200 + i * ROW, align: "right", maxWidth: columnWidth / 2 - 30, member: false });
      lines.push({ kind: "raises", x1: centre + DOT / 2, y1: 46 + DOT / 2, x2: x + columnWidth - 24 - DOT / 2, y2: 200 + i * ROW + DOT / 2 });
    });
    height = Math.max(height, 200 + Math.max(rules.length, events.length) * ROW + 30);
    x += columnWidth;
  }

  function column(items: readonly Entry[], members: (e: Entry) => Entry[], width: number) {
    if (items.length === 0) return;
    let y = 46;
    for (const item of items) {
      const own = members(item);
      const needed = 36 + own.length * ROW + 10;
      if (y > 46 && y + needed > columnHeight) {
        x += width;
        y = 46;
      }
      const ownerX = x + width / 2;
      const ownerY = y;
      place(item, { x: ownerX, y: ownerY, align: "centre", maxWidth: width - 40, member: false });
      y += 36;
      for (const m of own) {
        place(m, { x: x + 24, y, align: "left", maxWidth: width - 48, member: true });
        y += ROW;
      }
      branch(ownerX, ownerY, x, own);
      y += 10;
      height = Math.max(height, y + 20);
    }
    x += width;
  }
  column(ofConcept(entries, context, "value_object"), (v) => membersOf(entries, v), 220);
  column(ofConcept(entries, context, "domain_service"), (s) => membersOf(entries, s), 220);
  const raised = new Set(aggregates.flatMap((a) => raisedByAggregate(a).map((e) => e.key)));
  const loose = entries.filter(
    (e) => e.context === context && ((e.concept === "domain_event" && !raised.has(e.key)) || e.concept === "specification" || e.concept === "question"),
  );
  column(loose, () => [], 220);

  // An identity names a value object of the same context: a dashed line
  // from the aggregate or entity to it.
  for (const e of entries) {
    if (e.context !== context || (e.concept !== "aggregate" && e.concept !== "entity")) continue;
    const identity = textOf(e, "identity");
    if (identity === "") continue;
    const target = ofConcept(entries, context, "value_object").find((v) => v.name === identity);
    const from = nodes.get(e.key);
    const to = target ? nodes.get(target.key) : undefined;
    if (from && to) lines.push({ kind: "identity", x1: from.x, y1: from.y + DOT / 2, x2: to.x, y2: to.y + DOT / 2 });
  }

  return { width: Math.max(440, x), height, nodes: [...nodes.values()], lines };
}
