import { useEffect, useId, useMemo, useState, type KeyboardEvent, type RefObject } from "react";
import { CONCEPT_MARK, CONCEPT_TITLE, searchEntries, type Entry, type SearchHit } from "../../../entities/language/index";
import { KindBadge } from "../../../shared/ui/KindBadge";

export interface LanguageSearchProps {
  entries: readonly Entry[];
  query: string;
  inputRef?: RefObject<HTMLInputElement | null>;
  onQueryChange(query: string): void;
  onPick(entry: Entry): void;
}

/**
 * Find an entry by name, alias or the words of its reading, across every
 * context, and go to it. Arrow keys move through the hits, Enter picks the
 * active one, Escape clears the search.
 */
export function LanguageSearch({ entries, query, inputRef, onQueryChange, onPick }: LanguageSearchProps) {
  const id = useId();
  const hits = useMemo(() => searchEntries(entries, query), [entries, query]);
  const [active, setActive] = useState(0);
  useEffect(() => setActive(0), [query]);
  const open = query.trim() !== "";

  function pick(hit: SearchHit) {
    onPick(hit.entry);
    onQueryChange("");
  }

  function onKeyDown(e: KeyboardEvent<HTMLInputElement>) {
    if (!open) return;
    if (e.key === "ArrowDown") {
      e.preventDefault();
      setActive((a) => Math.min(a + 1, hits.length - 1));
    } else if (e.key === "ArrowUp") {
      e.preventDefault();
      setActive((a) => Math.max(a - 1, 0));
    } else if (e.key === "Enter") {
      e.preventDefault();
      if (hits[active]) pick(hits[active]);
    } else if (e.key === "Escape") {
      e.preventDefault();
      onQueryChange("");
    }
  }

  return (
    <div className="st-search">
      <input
        ref={inputRef}
        id={`${id}-query`}
        type="search"
        role="combobox"
        aria-label="Search the language"
        aria-expanded={open}
        aria-controls={`${id}-hits`}
        aria-activedescendant={open && hits[active] ? `${id}-hit-${active}` : undefined}
        aria-autocomplete="list"
        placeholder="Search names, aliases, definitions"
        autoComplete="off"
        spellCheck={false}
        value={query}
        onChange={(e) => onQueryChange(e.target.value)}
        onKeyDown={onKeyDown}
      />
      {open ? (
        <ul className="st-search__hits" id={`${id}-hits`} role="listbox" aria-label="Entries found">
          {hits.length === 0 ? (
            <li className="st-search__empty" role="option" aria-selected={false}>
              Nothing recorded matches “{query.trim()}”.
            </li>
          ) : null}
          {hits.map((hit, i) => (
            <li
              key={hit.entry.key}
              id={`${id}-hit-${i}`}
              role="option"
              aria-selected={i === active}
              className={i === active ? "st-search__hit is-active" : "st-search__hit"}
              onMouseEnter={() => setActive(i)}
              onMouseDown={(e) => {
                e.preventDefault();
                pick(hit);
              }}
            >
              <KindBadge mark={CONCEPT_MARK[hit.entry.concept]} label={CONCEPT_TITLE[hit.entry.concept]} tone={hit.entry.concept} size="small" />
              <span className="st-search__hit-main">
                <span className="st-search__hit-name">
                  {hit.entry.name}
                  {hit.alias ? <span className="st-search__hit-alias"> also “{hit.alias}”</span> : null}
                </span>
                <span className="st-search__hit-where">
                  {hit.entry.concept === "bounded_context" ? "bounded context" : hit.entry.owner ? `${hit.entry.context} · ${hit.entry.owner}` : hit.entry.context}
                </span>
                {hit.snippet ? <span className="st-search__hit-snippet">{hit.snippet}</span> : null}
              </span>
            </li>
          ))}
        </ul>
      ) : null}
    </div>
  );
}
