import {
  CONCEPT_MARK,
  CONCEPT_MEANING,
  CONCEPT_TITLE,
  MEMBER_CONCEPTS,
  aliasesOf,
  connectionsOf,
  listOf,
  membersOf,
  missingRequired,
  ofConcept,
  propertiesOf,
  readingOf,
  readingProperty,
  textOf,
  type Concept,
  type Difference,
  type Entry,
  type Standing,
} from "../../../entities/language/index";
import { EntryForm, RemoveEntryButton, type DraftValues } from "../../../features/edit-entry/index";
import type { RelationDocument } from "../../../shared/transport/index";
import { Badge } from "../../../shared/ui/Badge";
import { KindBadge } from "../../../shared/ui/KindBadge";

export interface EntryPanelProps {
  entry: Entry;
  entries: readonly Entry[];
  relations: readonly RelationDocument[];
  standing: Standing | null;
  branch: string | null;
  difference: Difference | null;
  draft: DraftValues | null;
  editing: boolean;
  busy: boolean;
  error: string | null;
  onSelect(entry: Entry): void;
  onEdit(): void;
  onResume(): void;
  onDiscard(): void;
  onChange(field: string, value: string): void;
  onSave(): void;
  onCancel(): void;
  onRemove(): void;
  onNew(concept: Concept, context: string, owner: string): void;
}

/**
 * One entry as recorded: its reading, its other properties, its standing
 * against the default branch, and the connections the recording states,
 * each a step to the entry it names. Edit opens its form in place.
 */
export function EntryPanel(props: EntryPanelProps) {
  const { entry, entries, relations, standing, branch, difference, draft, editing, busy, error, onSelect } = props;
  const connections = connectionsOf(entries, relations, entry);
  const missing = missingRequired(entry);
  const aggregates = ofConcept(entries, entry.context, "aggregate").map((a) => a.name);
  const canHold = MEMBER_CONCEPTS[entry.concept] ?? [];
  const members = membersOf(entries, entry);
  const owner = entry.owner === "" ? null : entry.owner;

  return (
    <article className="st-card" aria-label={`${CONCEPT_TITLE[entry.concept]} ${entry.name}`}>
      <header className="st-card__head">
        <div className="st-eyebrow">
          {entry.concept === "bounded_context" ? "Bounded context" : owner ? `${entry.context} › ${owner}` : entry.context}
        </div>
        <div className="st-card__title-row">
          <KindBadge mark={CONCEPT_MARK[entry.concept]} label={CONCEPT_TITLE[entry.concept]} tone={entry.concept} size="large" proposed={standing === "proposed"} />
          <h1 className="st-card__title">{entry.name}</h1>
          <span className="st-card__kind">{CONCEPT_TITLE[entry.concept]}</span>
          {standing ? <StandingBadge standing={standing} branch={branch} difference={difference} /> : null}
        </div>
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
      </header>

      {editing ? (
        <EntryForm concept={entry.concept} recorded={entry} values={draft ?? {}} aggregates={aggregates} error={error} busy={busy} onChange={props.onChange} onSave={props.onSave} onCancel={props.onCancel} />
      ) : (
        <>
          {draft ? (
            <div className="st-unsaved" role="status">
              <Badge tone="draft">unsaved changes</Badge>
              <span>You were editing this entry.</span>
              <button type="button" className="st-button st-button--small st-button--primary" onClick={props.onResume}>
                Continue editing
              </button>
              <button type="button" className="st-button st-button--small" onClick={props.onDiscard}>
                Discard
              </button>
            </div>
          ) : null}
          <Reading entry={entry} missing={missing.includes(readingProperty(entry.concept))} />
          <dl className="st-properties">
            {propertiesOf(entry.concept)
              .filter((p) => p.name !== readingProperty(entry.concept) && p.name !== "aliases")
              .filter((p) => p.required || (p.shape === "list" ? listOf(entry, p.name).length > 0 : textOf(entry, p.name) !== ""))
              .map((p) => {
                const value = p.shape === "list" ? listOf(entry, p.name) : [textOf(entry, p.name)].filter((v) => v !== "");
                const target = p.name === "raisedBy" || p.name === "identity" ? connections.find((c) => (p.name === "raisedBy" ? c.kind === "raised-by" : c.kind === "identity")) : undefined;
                return (
                  <div className="st-property" key={p.name}>
                    <dt>{p.label}</dt>
                    <dd>
                      {value.length === 0 ? (
                        <span className="st-missing">not recorded yet</span>
                      ) : target ? (
                        <button type="button" className="st-link-button" onClick={() => onSelect(target.entry)}>
                          {value[0]}
                        </button>
                      ) : (
                        value.map((v) => (
                          <span key={v} className={p.advanced ? "st-value st-value--code" : "st-value"}>
                            {v}
                          </span>
                        ))
                      )}
                    </dd>
                  </div>
                );
              })}
          </dl>

          {connections.length > 0 ? (
            <section className="st-section" aria-label="Connections">
              <h2 className="st-section__heading">Connections</h2>
              <ul className="st-connections">
                {connections.map((c, i) => (
                  <li key={`${c.kind}-${c.entry.key}-${i}`}>
                    <span className="st-connections__label">{c.label}</span>
                    <button type="button" className="st-connections__target" onClick={() => onSelect(c.entry)}>
                      <KindBadge mark={CONCEPT_MARK[c.entry.concept]} label={CONCEPT_TITLE[c.entry.concept]} tone={c.entry.concept} size="small" />
                      <span>{c.entry.name}</span>
                      {c.entry.concept === "invariant" || c.entry.concept === "assertion" ? <small>{readingOf(c.entry)}</small> : null}
                    </button>
                    {c.note ? <span className="st-connections__note">{c.note}</span> : null}
                  </li>
                ))}
              </ul>
            </section>
          ) : null}

          {entry.concept === "bounded_context" ? <ContextContents entry={entry} entries={entries} onSelect={onSelect} /> : null}

          <div className="st-card__actions">
            <button type="button" className="st-button st-button--primary" onClick={props.onEdit} disabled={busy}>
              Edit
            </button>
            {canHold.map((concept) => (
              <button key={concept} type="button" className="st-button" title={CONCEPT_MEANING[concept]} disabled={busy} onClick={() => props.onNew(concept, entry.context, entry.name)}>
                + {CONCEPT_TITLE[concept]}
              </button>
            ))}
            <span className="st-card__actions-grow" />
            <RemoveEntryButton
              key={entry.key}
              entry={entry}
              carries={entry.concept === "bounded_context" ? entries.filter((e) => e.context === entry.name && e.concept !== "bounded_context").map((e) => e.name) : members.map((m) => m.name)}
              busy={busy}
              error={error}
              onRemove={props.onRemove}
            />
          </div>
        </>
      )}
    </article>
  );
}

function Reading({ entry, missing }: { entry: Entry; missing: boolean }) {
  const text = readingOf(entry);
  if (missing || text.trim() === "") {
    const what = readingProperty(entry.concept);
    return <p className="st-reading st-reading--missing">No {what} is recorded yet. The language refuses the entry until it has one.</p>;
  }
  return <p className="st-reading">{text}</p>;
}

function StandingBadge({ standing, branch, difference }: { standing: Standing; branch: string | null; difference: Difference | null }) {
  if (standing === "accepted") return <Badge tone="accepted" title={`${branch ?? "The default branch"} records it the same.`}>Accepted</Badge>;
  const what = difference?.kind === "added" ? `not on ${branch ?? "the default branch"} yet` : `differs from ${branch ?? "the default branch"}: ${(difference?.fields ?? []).map((f) => f.field).join(", ")}`;
  return (
    <Badge tone="proposed" title={what}>
      Proposed · {what}
    </Badge>
  );
}

function ContextContents({ entry, entries, onSelect }: { entry: Entry; entries: readonly Entry[]; onSelect(entry: Entry): void }) {
  const groups = (["aggregate", "value_object", "domain_event", "domain_service", "specification", "question"] as const)
    .map((concept) => ({ concept, items: ofConcept(entries, entry.name, concept) }))
    .filter((g) => g.items.length > 0);
  if (groups.length === 0) return <p className="st-note">Nothing is recorded in {entry.name} yet. Use + beside it in the navigator, or the buttons below.</p>;
  return (
    <section className="st-section" aria-label="Recorded here">
      <h2 className="st-section__heading">Recorded here</h2>
      {groups.map((g) => (
        <div className="st-group" key={g.concept}>
          <h3 className="st-group__heading">{CONCEPT_TITLE[g.concept]}s</h3>
          <ul className="st-group__items">
            {g.items.map((e) => (
              <li key={e.key}>
                <button type="button" className="st-link-button" onClick={() => onSelect(e)}>
                  {e.name}
                </button>
                <small>{readingOf(e)}</small>
              </li>
            ))}
          </ul>
        </div>
      ))}
    </section>
  );
}
