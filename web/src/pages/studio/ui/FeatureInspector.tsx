import { useEffect, useMemo, useState } from "react";
import {
  CONCEPT_MARK,
  CONCEPT_TITLE,
  aliasesOf,
  connectionsOf,
  describeReading,
  describeTerm,
  hasWordCharacter,
  readingOf,
  searchEntries,
  usesWithin,
  type Entry,
  type Use,
} from "../../../entities/language/index";
import { CopyTicketButton } from "../../../features/copy-ticket/index";
import type { Association, EditorSelection } from "../../../features/write-feature/index";
import type { FeatureReading, RelationDocument } from "../../../shared/transport/index";
import { Badge } from "../../../shared/ui/Badge";
import { KindBadge } from "../../../shared/ui/KindBadge";

export interface FeatureInspectorProps {
  entries: readonly Entry[];
  relations: readonly RelationDocument[];
  uses: readonly Use[];
  selection: EditorSelection | null;
  /** The use at the cursor, when the cursor stands in one. */
  focused: Use | null;
  storeNote: string | null;
  onOpenEntry(entry: Entry): void;
  onAssociate(from: number, to: number, association: Association | null): void;
  onForget(id: string): void;
  onGoTo(from: number, to: number): void;
  ticket(): string;
}

type ReadingChoice = Exclude<FeatureReading, "plain">;

function whereOf(entry: Entry): string {
  if (entry.concept === "bounded_context") return "bounded context";
  return entry.owner === "" ? `${CONCEPT_TITLE[entry.concept]} in ${entry.context}` : `${CONCEPT_TITLE[entry.concept]} of ${entry.owner} in ${entry.context}`;
}

function readingsFor(entry: Entry): ReadingChoice[] {
  if (entry.concept === "invariant" || entry.concept === "assertion") return ["rule", "names"];
  if (entry.concept === "value_object") return ["names", "value"];
  return ["names"];
}

function readingLabel(reading: ReadingChoice, entry: Entry): string {
  switch (reading) {
    case "names":
      return `names ${entry.name}`;
    case "value":
      return `a value of ${entry.name}`;
    case "rule":
      return `states the rule ${entry.name}`;
  }
}

/**
 * The language beside the prose. What the selection can be tied to and
 * how; what the use at the cursor means now, where it is recorded and
 * what it connects to; the annotations that no longer resolve; and the
 * ticket. Every definition is the recording's as last read.
 */
export function FeatureInspector(props: FeatureInspectorProps) {
  const { entries, relations, uses, selection, focused, storeNote, onOpenEntry, onAssociate, onForget, onGoTo } = props;
  const unresolved = uses.filter((u) => u.kind === "unresolved");
  const selected = selection !== null && selection.from < selection.to && hasWordCharacter(selection.text) && !/[\r\n]/.test(selection.text) ? selection : null;

  return (
    <div className="st-feature-inspector">
      {storeNote ? <p className="st-note">{storeNote}</p> : null}
      {selected ? <SelectionActions key={`${selected.from}:${selected.to}`} selection={selected} entries={entries} uses={uses} onAssociate={onAssociate} /> : null}
      {focused ? <FocusedUse use={focused} entries={entries} relations={relations} onOpenEntry={onOpenEntry} onAssociate={onAssociate} onForget={onForget} /> : null}
      {!selected && !focused ? (
        <section className="st-card st-card--inspector" aria-label="Language at hand">
          <h2 className="st-section__heading">Language at hand</h2>
          <p className="st-note">Put the cursor on a lit word to read its definition. Select a word or phrase to tie it to a recorded entry, or to mark it plain.</p>
          <p className="st-note">
            <kbd>Ctrl</kbd> <kbd>Space</kbd> inserts a recorded name; <kbd>Ctrl</kbd> <kbd>I</kbd> shows the card for the use at the cursor.
          </p>
        </section>
      ) : null}
      {unresolved.length > 0 ? (
        <section className="st-card st-card--inspector" aria-label="Unresolved annotations">
          <h2 className="st-section__heading">Unresolved annotations</h2>
          <ul className="st-unresolved">
            {unresolved.map((u) => (
              <li key={u.reference!.id}>
                <span className="st-unresolved__quote">“{u.reference!.quote}”</span>
                {u.reference!.term ? <small> tied to {describeTerm(u.reference!.term)}</small> : null}
                <small className="st-unresolved__reason">{u.reason}</small>
                <span className="st-unresolved__actions">
                  {u.placed ? (
                    <button type="button" className="st-link-button" onClick={() => onGoTo(u.from, u.to)}>
                      Go to it
                    </button>
                  ) : null}
                  <button type="button" className="st-link-button" onClick={() => onForget(u.reference!.id)}>
                    Forget it
                  </button>
                </span>
              </li>
            ))}
          </ul>
        </section>
      ) : null}
      <section className="st-card st-card--inspector" aria-label="Share">
        <h2 className="st-section__heading">Share</h2>
        <p className="st-note">A Markdown ticket: title, prose and the language used with current definitions. Tied uses are told from cues; nothing is sent anywhere.</p>
        <CopyTicketButton ticket={props.ticket} />
      </section>
    </div>
  );
}

function SelectionActions({ selection, entries, uses, onAssociate }: { selection: EditorSelection; entries: readonly Entry[]; uses: readonly Use[]; onAssociate: FeatureInspectorProps["onAssociate"] }) {
  const [query, setQuery] = useState("");
  const [picked, setPicked] = useState<Entry | null>(null);
  const [reading, setReading] = useState<ReadingChoice>("names");
  const within = usesWithin(uses, selection.from, selection.to);
  const annotated = within.filter((u) => u.reference !== null);
  const suggested = useMemo(() => {
    const out: Entry[] = [];
    for (const u of within) {
      if (u.entry && !out.some((e) => e.key === u.entry!.key)) out.push(u.entry);
      for (const c of u.candidates) if (!out.some((e) => e.key === c.key)) out.push(c);
    }
    return out;
  }, [within]);
  const hits = useMemo(() => (query.trim() === "" ? [] : searchEntries(entries, query).map((h) => h.entry)), [entries, query]);
  const options = query.trim() === "" ? (suggested.length ? suggested : searchEntries(entries, selection.text).map((h) => h.entry)) : hits;

  useEffect(() => {
    if (picked) setReading(readingsFor(picked)[0]);
  }, [picked]);

  return (
    <section className="st-card st-card--inspector" aria-label="Selection">
      <h2 className="st-section__heading">Selection</h2>
      <p className="st-selection__text">“{selection.text}”</p>
      <div className="st-field">
        <label htmlFor="st-tie-query">Tie it to a recorded entry</label>
        <input id="st-tie-query" type="search" placeholder="Search names, aliases, definitions" value={query} autoComplete="off" onChange={(e) => { setQuery(e.target.value); setPicked(null); }} />
      </div>
      <ul className="st-tie-options" role="listbox" aria-label="Entries to tie to">
        {options.length === 0 ? <li className="st-navigator__empty">Nothing recorded matches.</li> : null}
        {options.slice(0, 12).map((e) => (
          <li
            key={e.key}
            role="option"
            tabIndex={picked?.key === e.key || (picked === null && options[0]?.key === e.key) ? 0 : -1}
            aria-selected={picked?.key === e.key}
            className={picked?.key === e.key ? "st-tie-option is-active" : "st-tie-option"}
            onClick={() => setPicked(e)}
            onKeyDown={(event) => {
              if (event.key === "Enter" || event.key === " ") {
                event.preventDefault();
                setPicked(e);
              } else if (["ArrowDown", "ArrowUp", "Home", "End"].includes(event.key)) {
                event.preventDefault();
                const choices = [...event.currentTarget.parentElement!.querySelectorAll<HTMLElement>('[role="option"]')];
                const index = choices.indexOf(event.currentTarget);
                const next = event.key === "Home" ? 0 : event.key === "End" ? choices.length - 1 : (index + (event.key === "ArrowDown" ? 1 : -1) + choices.length) % choices.length;
                choices[next]?.focus();
              }
            }}
          >
            <KindBadge mark={CONCEPT_MARK[e.concept]} label={CONCEPT_TITLE[e.concept]} tone={e.concept} size="small" />
            <span className="st-tie-option__name">{e.name}</span>
            <small>{whereOf(e)}</small>
          </li>
        ))}
      </ul>
      {picked ? (
        <div className="st-tie-reading" role="radiogroup" aria-label="How the text reads">
          {readingsFor(picked).map((r) => (
            <label key={r} className="st-tie-reading__choice">
              <input type="radio" name="st-tie-reading" value={r} checked={reading === r} onChange={() => setReading(r)} />
              <span>{readingLabel(r, picked)}</span>
            </label>
          ))}
        </div>
      ) : null}
      <div className="st-form-actions">
        <button type="button" className="st-button st-button--primary st-button--small" disabled={!picked} onClick={() => picked && onAssociate(selection.from, selection.to, { reading, term: { concept: picked.concept, context: picked.context, owner: picked.owner, name: picked.name } })}>
          Tie
        </button>
        <button type="button" className="st-button st-button--small" onClick={() => onAssociate(selection.from, selection.to, { reading: "plain" })} title="Not a use of the language: no cue here">
          Mark plain
        </button>
        {annotated.length > 0 ? (
          <button type="button" className="st-button st-button--small st-button--quiet" onClick={() => onAssociate(selection.from, selection.to, null)}>
            Clear annotation
          </button>
        ) : null}
      </div>
      <p className="st-note">Tying says what you meant here; it records nothing in the language.</p>
    </section>
  );
}

function FocusedUse({ use, entries, relations, onOpenEntry, onAssociate, onForget }: { use: Use; entries: readonly Entry[]; relations: readonly RelationDocument[]; onOpenEntry(entry: Entry): void; onAssociate: FeatureInspectorProps["onAssociate"]; onForget(id: string): void }) {
  const eyebrow: Record<Use["kind"], string> = { cue: "Recognised, not confirmed", tied: "Tied on purpose", ambiguous: "Ambiguous", plain: "Plain word", unresolved: "Unresolved" };
  const entry = use.entry;
  return (
    <section className="st-card st-card--inspector" aria-label="At the cursor">
      <div className="st-eyebrow">{eyebrow[use.kind]}</div>
      {entry ? (
        <>
          <div className="st-card__title-row">
            <KindBadge mark={CONCEPT_MARK[entry.concept]} label={CONCEPT_TITLE[entry.concept]} tone={entry.concept} size="regular" />
            <h2 className="st-card__title st-card__title--inspector">{entry.name}</h2>
          </div>
          <p className="st-note">{whereOf(entry)}</p>
          {aliasesOf(entry).length > 0 ? (
            <div className="st-aliases" aria-label="Aliases">
              <span className="st-aliases__label">also called</span>
              {aliasesOf(entry).map((a) => (
                <span key={a} className="st-chip">
                  {a}
                </span>
              ))}
            </div>
          ) : null}
          <p className="st-reading">{readingOf(entry).trim() === "" ? <span className="st-missing">No definition recorded yet.</span> : readingOf(entry)}</p>
          {use.reading && use.reading !== "names" ? <p className="st-note">{describeReading(use.reading, entry.name)}</p> : null}
          <Connections entry={entry} entries={entries} relations={relations} onOpenEntry={onOpenEntry} />
          <div className="st-card__actions">
            <button type="button" className="st-button st-button--small" onClick={() => onOpenEntry(entry)}>
              Open in Language
            </button>
            {use.kind === "cue" ? (
              <>
                <button type="button" className="st-button st-button--small" onClick={() => onAssociate(use.from, use.to, { reading: "names", term: { concept: entry.concept, context: entry.context, owner: entry.owner, name: entry.name } })}>
                  Tie it
                </button>
                <button type="button" className="st-button st-button--small st-button--quiet" onClick={() => onAssociate(use.from, use.to, { reading: "plain" })}>
                  Mark plain
                </button>
              </>
            ) : null}
            {use.reference ? (
              <button type="button" className="st-button st-button--small st-button--quiet" onClick={() => onAssociate(use.from, use.to, null)}>
                Untie
              </button>
            ) : null}
          </div>
        </>
      ) : null}
      {use.kind === "ambiguous" ? (
        <>
          <h2 className="st-card__title st-card__title--inspector">{use.text}</h2>
          <p className="st-note">Recorded in more than one place. Which one is meant here is not chosen until you tie it.</p>
          <ul className="st-candidates">
            {use.candidates.map((c) => (
              <li key={c.key}>
                <KindBadge mark={CONCEPT_MARK[c.concept]} label={CONCEPT_TITLE[c.concept]} tone={c.concept} size="small" />
                <span>
                  <b>{c.name}</b> <small>{whereOf(c)}</small>
                  <small className="st-candidates__reading">{readingOf(c)}</small>
                </span>
                <button type="button" className="st-button st-button--small" onClick={() => onAssociate(use.from, use.to, { reading: "names", term: { concept: c.concept, context: c.context, owner: c.owner, name: c.name } })}>
                  Tie to this
                </button>
              </li>
            ))}
          </ul>
          <div className="st-card__actions">
            <button type="button" className="st-button st-button--small st-button--quiet" onClick={() => onAssociate(use.from, use.to, { reading: "plain" })}>
              Mark plain
            </button>
          </div>
        </>
      ) : null}
      {use.kind === "plain" ? (
        <>
          <h2 className="st-card__title st-card__title--inspector">{use.text}</h2>
          <p className="st-note">Marked as not a use of the language; no cue is drawn here.</p>
          <div className="st-card__actions">
            <button type="button" className="st-button st-button--small" onClick={() => onAssociate(use.from, use.to, null)}>
              Unmark
            </button>
          </div>
        </>
      ) : null}
      {use.kind === "unresolved" ? (
        <>
          <h2 className="st-card__title st-card__title--inspector">{use.text}</h2>
          <p className="st-form-error">{use.reason}</p>
          {use.reference?.term ? <p className="st-note">It was tied to {describeTerm(use.reference.term)}.</p> : null}
          <div className="st-card__actions">
            <button type="button" className="st-button st-button--small" onClick={() => onForget(use.reference!.id)}>
              Forget it
            </button>
          </div>
        </>
      ) : null}
    </section>
  );
}

function Connections({ entry, entries, relations, onOpenEntry }: { entry: Entry; entries: readonly Entry[]; relations: readonly RelationDocument[]; onOpenEntry(entry: Entry): void }) {
  const connections = connectionsOf(entries, relations, entry);
  if (connections.length === 0) return null;
  return (
    <section className="st-section" aria-label="Connections">
      <h3 className="st-section__heading">Connections</h3>
      <ul className="st-connections">
        {connections.map((c, i) => (
          <li key={`${c.kind}-${c.entry.key}-${i}`}>
            <span className="st-connections__label">{c.label}</span>
            <button type="button" className="st-connections__target" onClick={() => onOpenEntry(c.entry)} title="Open in Language">
              <KindBadge mark={CONCEPT_MARK[c.entry.concept]} label={CONCEPT_TITLE[c.entry.concept]} tone={c.entry.concept} size="small" />
              <span>{c.entry.name}</span>
              {c.entry.concept === "invariant" || c.entry.concept === "assertion" ? <small>{readingOf(c.entry)}</small> : null}
            </button>
            {c.note ? <span className="st-connections__note">{c.note}</span> : null}
          </li>
        ))}
      </ul>
      <Badge tone="muted">as recorded now</Badge>
    </section>
  );
}
