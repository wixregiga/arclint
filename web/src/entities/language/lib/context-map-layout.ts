/*
 * Positions of the bounded contexts and the relations of the context map,
 * in map units: contexts in rows, each relation a line from its upstream
 * context to its downstream one with its kind beside it.
 */
import type { RelationDocument } from "../../../shared/transport/index";

export interface ContextNode {
  name: string;
  x: number;
  y: number;
  width: number;
  height: number;
}

export interface RelationLine {
  from: string;
  to: string;
  kind: string;
  description: string;
  x1: number;
  y1: number;
  x2: number;
  y2: number;
  /** Where the kind is written: the midpoint of the line. */
  labelX: number;
  labelY: number;
}

export interface ContextMapLayout {
  width: number;
  height: number;
  nodes: ContextNode[];
  lines: RelationLine[];
}

const BOX = { width: 180, height: 54 };
const GAP = { x: 72, y: 76 };

export function layoutContextMap(contexts: readonly string[], relations: readonly RelationDocument[]): ContextMapLayout {
  const perRow = Math.max(1, Math.ceil(Math.sqrt(contexts.length)));
  const nodes: ContextNode[] = contexts.map((name, i) => ({
    name,
    x: 24 + (i % perRow) * (BOX.width + GAP.x),
    y: 24 + Math.floor(i / perRow) * (BOX.height + GAP.y),
    width: BOX.width,
    height: BOX.height,
  }));
  const at = new Map(nodes.map((n) => [n.name, n]));
  const lines: RelationLine[] = [];
  for (const r of relations) {
    const a = at.get(r.from);
    const b = at.get(r.to);
    if (!a || !b) continue;
    const [x1, y1] = edgePoint(a, b);
    const [x2, y2] = edgePoint(b, a);
    lines.push({ from: r.from, to: r.to, kind: r.kind, description: r.description ?? "", x1, y1, x2, y2, labelX: (x1 + x2) / 2, labelY: (y1 + y2) / 2 });
  }
  const rows = Math.ceil(contexts.length / perRow);
  return {
    width: 48 + Math.min(perRow, Math.max(1, contexts.length)) * BOX.width + (Math.min(perRow, contexts.length) - 1) * GAP.x,
    height: 48 + rows * BOX.height + Math.max(0, rows - 1) * GAP.y,
    nodes,
    lines,
  };
}

/** The point on a box's edge where a line to another box's centre leaves it. */
function edgePoint(from: ContextNode, to: ContextNode): [number, number] {
  const cx = from.x + from.width / 2;
  const cy = from.y + from.height / 2;
  const tx = to.x + to.width / 2;
  const ty = to.y + to.height / 2;
  const dx = tx - cx;
  const dy = ty - cy;
  if (dx === 0 && dy === 0) return [cx, cy];
  const sx = dx === 0 ? Number.POSITIVE_INFINITY : from.width / 2 / Math.abs(dx);
  const sy = dy === 0 ? Number.POSITIVE_INFINITY : from.height / 2 / Math.abs(dy);
  const s = Math.min(sx, sy);
  return [cx + dx * s, cy + dy * s];
}
