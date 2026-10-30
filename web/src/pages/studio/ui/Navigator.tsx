import { useEffect, useMemo, useState } from "react";
import {
  CONCEPT_MARK,
  CONCEPT_MEANING,
  CONCEPT_TITLE,
  CONTEXT_CONCEPTS,
  contextNames,
  membersOf,
  ofConcept,
  type Concept,
  type Entry,
  type Standing,
} from "../../../entities/language/index";
import { Badge } from "../../../shared/ui/Badge";
import { KindBadge } from "../../../shared/ui/KindBadge";
import type { Selection } from "../model/workspace";

export interface NavigatorProps {
  entries: readonly Entry[];
  selection: Selection;
  standingOf: ((key: string) => Standing) | null;
  draftKeys: ReadonlySet<string>;
  onSelect(entry: Entry): void;
  onNew(concept: Concept, context: string, owner: string): void;
}

/**
 * Every bounded context with what it records, in the recording's order,
 * each nested entry under the one it is recorded under. An owner's
 * members fold away until opened or selected.
 */
export function Navigator({ entries, selection, standingOf, draftKeys, onSelect, onNew }: NavigatorProps) {
  const contexts = contextNames(entries);
  const selectedKey = selection.kind === "entry" ? selection.key : null;
  const [open, setOpen] = useState<Set<string>>(() => new Set());
  const [adding, setAdding] = useState<string | null>(null);

  const selectedOwnerKey = useMemo(() => {
    if (!selectedKey) return null;
    const e = entries.find((x) => x.key === selectedKey);
    if (!e || e.owner === "") return null;
    const owner = entries.find((x) => x.context === e.context && x.name === e.owner && x.owner === "");
    return owner?.key ?? null;
  }, [entries, selectedKey]);
  useEffect(() => {
    if (selectedOwnerKey && !open.has(selectedOwnerKey)) setOpen((s) => new Set(s).add(selectedOwnerKey));
  }, [selectedOwnerKey, open]);

  function toggle(key: string) {
    setOpen((s) => {
      const next = new Set(s);
      if (next.has(key)) next.delete(key);
      else next.add(key);
      return next;
    });
  }

  function row(entry: Entry, depth: number) {
    const members = membersOf(entries, entry);
    const isOpen = open.has(entry.key);
    const current = entry.key === selectedKey;
    const standing = standingOf ? standingOf(entry.key) : null;
    return (
      <li key={entry.key} className="st-nav-item">
        <div className={current ? "st-nav-row is-current" : "st-nav-row"} style={{ paddingLeft: `${8 + depth * 14}px` }}>
          {members.length > 0 ? (
            <button type="button" className="st-nav-fold" aria-label={isOpen ? `Fold ${entry.name}` : `Unfold ${entry.name}`} aria-expanded={isOpen} onClick={() => toggle(entry.key)}>
              {isOpen ? "▾" : "▸"}
            </button>
          ) : (
            <span className="st-nav-fold st-nav-fold--none" aria-hidden="true" />
          )}
          <button type="button" className="st-nav-row__main" aria-current={current || undefined} onClick={() => onSelect(entry)} title={CONCEPT_TITLE[entry.concept]}>
            <KindBadge mark={CONCEPT_MARK[entry.concept]} label={CONCEPT_TITLE[entry.concept]} tone={entry.concept} size="small" proposed={standing === "proposed"} />
            <span className="st-nav-row__name">{entry.name}</span>
            {draftKeys.has(entry.key) ? <Badge tone="draft">unsaved</Badge> : null}
          </button>
        </div>
        {members.length > 0 && isOpen ? <ul className="st-nav-list">{members.map((m) => row(m, depth + 1))}</ul> : null}
      </li>
    );
  }

  return (
    <nav className="st-navigator" aria-label="The recorded language">
      <div className="st-navigator__head">
        <h2 className="st-navigator__heading">Language</h2>
        <button type="button" className="st-button st-button--small" onClick={() => onNew("bounded_context", "", "")} title={CONCEPT_MEANING.bounded_context}>
          + Context
        </button>
      </div>
      {contexts.length === 0 ? <p className="st-navigator__empty">No bounded context is recorded yet. The language begins with one.</p> : null}
      {contexts.map((context) => {
        const ctxEntry = entries.find((e) => e.concept === "bounded_context" && e.name === context)!;
        const current = ctxEntry.key === selectedKey;
        const standing = standingOf ? standingOf(ctxEntry.key) : null;
        const isAdding = adding === context;
        return (
          <section key={context} className="st-nav-context" aria-label={`Context ${context}`}>
            <div className={current ? "st-nav-row st-nav-row--context is-current" : "st-nav-row st-nav-row--context"}>
              <button type="button" className="st-nav-row__main" aria-current={current || undefined} onClick={() => onSelect(ctxEntry)}>
                <KindBadge mark={CONCEPT_MARK.bounded_context} label={CONCEPT_TITLE.bounded_context} tone="bounded_context" size="small" proposed={standing === "proposed"} />
                <span className="st-nav-row__name">{context}</span>
                {draftKeys.has(ctxEntry.key) ? <Badge tone="draft">unsaved</Badge> : null}
              </button>
              <button type="button" className="st-nav-add" aria-expanded={isAdding} aria-label={`Record in ${context}`} title={`Record something in ${context}`} onClick={() => setAdding(isAdding ? null : context)}>
                +
              </button>
            </div>
            {isAdding ? (
              <ul className="st-nav-menu" role="menu" aria-label={`What to record in ${context}`}>
                {CONTEXT_CONCEPTS.map((concept) => (
                  <li key={concept} role="none">
                    <button
                      type="button"
                      role="menuitem"
                      onClick={() => {
                        setAdding(null);
                        onNew(concept, context, "");
                      }}
                    >
                      <KindBadge mark={CONCEPT_MARK[concept]} label={CONCEPT_TITLE[concept]} tone={concept} size="small" />
                      <span>
                        <b>{CONCEPT_TITLE[concept]}</b>
                        <small>{CONCEPT_MEANING[concept]}</small>
                      </span>
                    </button>
                  </li>
                ))}
              </ul>
            ) : null}
            <ul className="st-nav-list">
              {CONTEXT_CONCEPTS.flatMap((concept) => ofConcept(entries, context, concept).map((e) => row(e, 1)))}
              {entries.filter((e) => e.context === context && e.concept !== "bounded_context").length === 0 ? (
                <li className="st-navigator__empty">Nothing recorded in {context} yet.</li>
              ) : null}
            </ul>
          </section>
        );
      })}
      {selection.kind === "new" && selection.concept === "bounded_context" ? (
        <p className="st-navigator__empty">
          <Badge tone="draft">new context</Badge> being recorded
        </p>
      ) : null}
    </nav>
  );
}
