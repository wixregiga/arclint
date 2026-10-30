import { useEffect, useMemo, useRef, useState } from "react";
import {
  CONCEPT_MARK,
  CONCEPT_TITLE,
  contextNames,
  entryKey,
  fit,
  layoutContext,
  layoutContextMap,
  readingOf,
  relationInfluence,
  relationLabel,
  relationWord,
  type Entry,
  type Standing,
} from "../../../entities/language/index";
import type { RelationDocument } from "../../../shared/transport/index";

export interface LanguageMapProps {
  entries: readonly Entry[];
  relations: readonly RelationDocument[];
  context: string | null;
  selectedKey: string | null;
  standingOf: ((key: string) => Standing) | null;
  onSelect(entry: Entry): void;
}

const DOT = 12;
const CHAR = 6.4;
const READABLE = 0.72;

function trimmed(text: string, maxWidth: number): string {
  const chars = Math.max(4, Math.floor(maxWidth / CHAR));
  return text.length <= chars ? text : `${text.slice(0, chars - 1)}…`;
}

/**
 * The map: the context map above, the active context's entries below,
 * drawn from the recorded connections alone. Clicking a context opens it;
 * clicking an entry selects it for the inspector beside the map.
 */
export function LanguageMap({ entries, relations, context, selectedKey, standingOf, onSelect }: LanguageMapProps) {
  const contexts = contextNames(entries);
  const contextMap = useMemo(() => layoutContextMap(contexts, relations), [contexts, relations]);
  const stage = useRef<HTMLDivElement>(null);
  const [viewport, setViewport] = useState({ width: 0, height: 0 });
  const [zoom, setZoom] = useState(1);
  useEffect(() => {
    const el = stage.current;
    if (!el) return;
    const measure = () => setViewport({ width: el.clientWidth, height: el.clientHeight });
    measure();
    const observer = new ResizeObserver(measure);
    observer.observe(el);
    return () => observer.disconnect();
  }, []);
  const layout = useMemo(() => (context ? layoutContext(entries, context, viewport.width > 0 ? viewport : undefined) : null), [entries, context, viewport]);
  // Never shrink below a readable size: a large context scrolls instead.
  const scale = layout && viewport.width > 0 ? Math.max(READABLE, fit(layout, viewport)) * zoom : zoom;
  const byKey = useMemo(() => new Map(entries.map((e) => [e.key, e])), [entries]);

  return (
    <div className="st-map" aria-label="Map">
      <section className="st-map__contexts" aria-label="Context map">
        <h2 className="st-section__heading">Context map</h2>
        {contexts.length === 0 ? (
          <p className="st-note">No bounded context is recorded yet.</p>
        ) : (
          <svg className="st-context-map" viewBox={`0 0 ${contextMap.width} ${contextMap.height}`} width={contextMap.width} height={contextMap.height} role="group" aria-label="Bounded contexts and their relations">
            <defs>
              <marker id="st-arrow" viewBox="0 0 10 10" refX="9" refY="5" markerWidth="8" markerHeight="8" orient="auto-start-reverse">
                <path d="M 0 0 L 10 5 L 0 10 z" fill="currentColor" />
              </marker>
            </defs>
            {contextMap.lines.map((l) => (
              <g key={`${l.from}-${l.to}`} className="st-context-map__relation">
                <line
                  x1={l.x1} y1={l.y1} x2={l.x2} y2={l.y2}
                  markerStart={relationInfluence(l.kind) === "mutual" ? "url(#st-arrow)" : undefined}
                  markerEnd={relationInfluence(l.kind) !== "none" ? "url(#st-arrow)" : undefined}
                  strokeDasharray={l.kind === "separate_ways" ? "5 4" : undefined}
                >
                  <title>
                    {l.from}: {relationLabel(l.kind, "from")} {l.to}
                    {l.description ? `: ${l.description}` : ""}
                  </title>
                </line>
                <text x={l.labelX} y={l.labelY - 6} textAnchor="middle">
                  {relationWord(l.kind)}
                </text>
              </g>
            ))}
            {contextMap.nodes.map((n) => {
              const key = entryKey(n.name, "bounded_context", "", n.name);
              const entry = byKey.get(key);
              const proposed = standingOf?.(key) === "proposed";
              return (
                <g
                  key={n.name}
                  className={[
                    "st-context-map__context",
                    n.name === context ? "is-active" : "",
                    selectedKey === key ? "is-selected" : "",
                    proposed ? "is-proposed" : "",
                  ].join(" ")}
                  transform={`translate(${n.x} ${n.y})`}
                  role="button"
                  tabIndex={0}
                  aria-label={`Context ${n.name}`}
                  onClick={() => entry && onSelect(entry)}
                  onKeyDown={(e) => {
                    if ((e.key === "Enter" || e.key === " ") && entry) {
                      e.preventDefault();
                      onSelect(entry);
                    }
                  }}
                >
                  <rect width={n.width} height={n.height} rx={10} />
                  <text x={n.width / 2} y={n.height / 2 + 5} textAnchor="middle">
                    {trimmed(n.name, n.width - 24)}
                  </text>
                </g>
              );
            })}
          </svg>
        )}
      </section>
      <section className="st-map__context" aria-label={context ? `Map of ${context}` : "Map"}>
        <div className="st-map__bar">
          <h2 className="st-section__heading">{context ? `In ${context}` : "No context"}</h2>
          <span className="st-map__legend" aria-hidden="true">
            <span className="st-map__legend-line st-map__legend-line--owner" /> holds
            <span className="st-map__legend-line st-map__legend-line--raises" /> raises
            <span className="st-map__legend-line st-map__legend-line--identity" /> identified by
          </span>
          <span className="st-zoom" role="group" aria-label="Zoom">
            <button type="button" aria-label="Zoom out" onClick={() => setZoom((z) => Math.max(0.3, z / 1.2))}>
              −
            </button>
            <button type="button" aria-label="Reset zoom" title="Back to the fitted size" onClick={() => setZoom(1)}>
              {Math.round(scale * 100)}%
            </button>
            <button type="button" aria-label="Zoom in" onClick={() => setZoom((z) => Math.min(4, z * 1.2))}>
              +
            </button>
          </span>
        </div>
        <div className="st-map__stage" ref={stage}>
          {layout && layout.nodes.length > 0 ? (
            <svg className="st-concept-map" viewBox={`0 0 ${layout.width} ${layout.height}`} width={layout.width * scale} height={layout.height * scale} role="group" aria-label={`Entries of ${context}`}>
              {layout.lines.map((l, i) => (
                <line key={i} className={`st-concept-map__line st-concept-map__line--${l.kind}`} x1={l.x1} y1={l.y1} x2={l.x2} y2={l.y2} />
              ))}
              {layout.nodes.map((n) => {
                const entry = byKey.get(n.key);
                if (!entry) return null;
                const label = entry.concept === "invariant" || entry.concept === "assertion" || entry.concept === "question" ? readingOf(entry) || entry.name : entry.name;
                const proposed = standingOf?.(n.key) === "proposed";
                const classes = ["st-concept-map__node", `st-concept-map__node--${entry.concept}`, n.key === selectedKey ? "is-selected" : "", proposed ? "is-proposed" : ""].join(" ");
                const textX = n.member ? n.x + DOT + 4 : n.align === "right" ? n.x - DOT - 4 : n.x;
                const textY = n.member ? n.y + DOT / 2 + 4 : n.align === "centre" ? n.y + DOT + 14 : n.y + DOT / 2 + 4;
                const anchor = n.member ? "start" : n.align === "right" ? "end" : n.align === "left" ? "start" : "middle";
                const labelWidth = Math.min(n.maxWidth, label.length * CHAR + 8);
                const hitWidth = n.member ? DOT + 6 + labelWidth : n.align === "centre" ? Math.max(DOT + 8, labelWidth) : DOT + 6 + labelWidth;
                const hitX = n.member ? n.x - DOT / 2 - 2 : n.align === "right" ? n.x - hitWidth + DOT / 2 + 2 : n.align === "left" ? n.x - DOT / 2 - 2 : n.x - hitWidth / 2;
                return (
                  <g
                    key={n.key}
                    className={classes}
                    role="button"
                    tabIndex={0}
                    aria-label={`${CONCEPT_TITLE[entry.concept]} ${entry.name}`}
                    onClick={() => onSelect(entry)}
                    onKeyDown={(e) => {
                      if (e.key === "Enter" || e.key === " ") {
                        e.preventDefault();
                        onSelect(entry);
                      }
                    }}
                  >
                    <title>
                      {CONCEPT_TITLE[entry.concept]} {entry.name}
                      {readingOf(entry) ? `: ${readingOf(entry)}` : ""}
                    </title>
                    <rect className="st-concept-map__hit" x={hitX} y={n.y - 4} width={hitWidth} height={n.member ? DOT + 8 : DOT + 24} rx={4} />
                    <circle cx={n.x} cy={n.y + DOT / 2} r={DOT / 2} />
                    <text className="st-concept-map__mark" x={n.x} y={n.y + DOT / 2 + 2.5} textAnchor="middle">
                      {CONCEPT_MARK[entry.concept]}
                    </text>
                    <text className="st-concept-map__label" x={textX} y={textY} textAnchor={anchor}>
                      {trimmed(label, n.maxWidth)}
                    </text>
                  </g>
                );
              })}
            </svg>
          ) : (
            <p className="st-note st-map__empty">{context ? `Nothing is recorded in ${context} yet.` : "Choose a context to see its map."}</p>
          )}
        </div>
      </section>
    </div>
  );
}
