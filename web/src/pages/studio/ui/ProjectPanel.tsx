import { CONCEPT_MARK, CONCEPT_TITLE, type Difference, type Entry, type RelationDifference } from "../../../entities/language/index";
import { ProjectForm, type ProjectValues } from "../../../features/describe-project/index";
import type { AcceptedDocument, OverviewDocument } from "../../../shared/transport/index";
import { Badge } from "../../../shared/ui/Badge";
import { KindBadge } from "../../../shared/ui/KindBadge";

export interface ProjectPanelProps {
  overview: OverviewDocument;
  accepted: AcceptedDocument | null;
  entries: readonly Entry[];
  differences: readonly Difference[];
  relationDifferences: readonly RelationDifference[];
  values: ProjectValues | null;
  editing: boolean;
  busy: boolean;
  error: string | null;
  onEdit(): void;
  onResume(): void;
  onDiscard(): void;
  onChange(field: keyof ProjectValues, value: string): void;
  onSave(): void;
  onCancel(): void;
  onNewContext(): void;
  onSelect(entry: Entry): void;
}

/**
 * The project: its name and description, what its language holds, and
 * how the working tree's recording differs from the default branch.
 * Without a domain file, naming the project begins the recording.
 */
export function ProjectPanel(props: ProjectPanelProps) {
  const { overview, accepted, entries, differences, relationDifferences, values, editing, busy, error, onSelect } = props;
  const recorded: ProjectValues = { name: overview.project ?? "", description: overview.description ?? "" };
  const begins = !overview.found;
  const contexts = entries.filter((e) => e.concept === "bounded_context");
  const counts = overview.counts;

  return (
    <article className="st-card" aria-label="Project">
      <header className="st-card__head">
        <div className="st-eyebrow">Project · {overview.source}</div>
        <div className="st-card__title-row">
          <h1 className={recorded.name === "" ? "st-card__title is-empty" : "st-card__title"}>{recorded.name === "" ? "Unnamed project" : recorded.name}</h1>
        </div>
      </header>
      {editing ? (
        <ProjectForm recorded={recorded} values={values ?? recorded} begins={begins} error={error} busy={busy} onChange={props.onChange} onSave={props.onSave} onCancel={props.onCancel} />
      ) : (
        <>
          {values ? (
            <div className="st-unsaved" role="status">
              <Badge tone="draft">unsaved changes</Badge>
              <span>You were editing the project.</span>
              <button type="button" className="st-button st-button--small st-button--primary" onClick={props.onResume}>
                Continue editing
              </button>
              <button type="button" className="st-button st-button--small" onClick={props.onDiscard}>
                Discard
              </button>
            </div>
          ) : null}
          {begins ? (
            <p className="st-reading st-reading--missing">
              No domain file exists at {overview.source} yet. Name the project to begin its language.
            </p>
          ) : recorded.description === "" ? (
            <p className="st-reading st-reading--missing">No description is recorded yet.</p>
          ) : (
            <p className="st-reading">{recorded.description}</p>
          )}
          {!begins ? (
            <section className="st-section" aria-label="What the language holds">
              <h2 className="st-section__heading">What the language holds</h2>
              <p className="st-counts">{countsLine(counts)}</p>
              {contexts.length === 0 ? (
                <p className="st-note">No bounded context is recorded yet. The language begins with one: a part of the business with its own words.</p>
              ) : (
                <ul className="st-group__items">
                  {contexts.map((c) => (
                    <li key={c.key}>
                      <button type="button" className="st-link-button" onClick={() => onSelect(c)}>
                        <KindBadge mark={CONCEPT_MARK.bounded_context} label={CONCEPT_TITLE.bounded_context} tone="bounded_context" size="small" /> {c.name}
                      </button>
                      <small>{typeof c.properties.definition === "string" ? c.properties.definition : ""}</small>
                    </li>
                  ))}
                </ul>
              )}
            </section>
          ) : null}
          {accepted ? <Comparison accepted={accepted} entries={entries} differences={differences} relationDifferences={relationDifferences} onSelect={onSelect} /> : null}
          <div className="st-card__actions">
            <button type="button" className="st-button st-button--primary" onClick={props.onEdit} disabled={busy}>
              {begins ? "Name the project" : "Edit"}
            </button>
            {!begins ? (
              <button type="button" className="st-button" onClick={props.onNewContext} disabled={busy}>
                + Bounded context
              </button>
            ) : null}
          </div>
        </>
      )}
    </article>
  );
}

function Comparison({ accepted, entries, differences, relationDifferences, onSelect }: { accepted: AcceptedDocument; entries: readonly Entry[]; differences: readonly Difference[]; relationDifferences: readonly RelationDifference[]; onSelect(entry: Entry): void }) {
  if (!accepted.available) {
    return (
      <section className="st-section" aria-label="Default branch">
        <h2 className="st-section__heading">Against the default branch</h2>
        <p className="st-note">Not available: {accepted.reason}</p>
      </section>
    );
  }
  const total = differences.length + relationDifferences.length;
  return (
    <section className="st-section" aria-label="Default branch">
      <h2 className="st-section__heading">Against {accepted.branch}</h2>
      {total === 0 ? (
        <p className="st-note">
          <Badge tone="accepted">Accepted</Badge> The working tree records the language exactly as {accepted.branch} does.
        </p>
      ) : (
        <ul className="st-differences" aria-label={`${total} differences`}>
          {differences.map((d) => {
            const live = entries.find((e) => e.key === d.key);
            return (
              <li key={d.key} className={`st-difference st-difference--${d.kind}`}>
                <Badge tone={d.kind === "removed" ? "removed" : "proposed"}>{d.kind}</Badge>
                <KindBadge mark={CONCEPT_MARK[d.concept]} label={CONCEPT_TITLE[d.concept]} tone={d.concept} size="small" />
                {live ? (
                  <button type="button" className="st-link-button" onClick={() => onSelect(live)}>
                    {d.name}
                  </button>
                ) : (
                  <span>{d.name}</span>
                )}
                <small>
                  {d.context}
                  {d.owner ? ` › ${d.owner}` : ""}
                  {d.kind === "changed" ? ` · ${d.fields.map((f) => f.field).join(", ")}` : ""}
                </small>
              </li>
            );
          })}
          {relationDifferences.map((d) => (
            <li key={`${d.from}-${d.to}`} className={`st-difference st-difference--${d.kind}`}>
              <Badge tone={d.kind === "removed" ? "removed" : "proposed"}>{d.kind}</Badge>
              <span>
                relation {d.from} → {d.to}
              </span>
              <small>{d.relation.replace(/_/g, " ")}</small>
            </li>
          ))}
        </ul>
      )}
    </section>
  );
}

/** The counts line, in the overview's words. */
export function countsLine(counts: OverviewDocument["counts"]): string {
  const phrase = (n: number, one: string, many: string) => `${n} ${n === 1 ? one : many}`;
  const parts = [phrase(counts.contexts, "context", "contexts")];
  const more: [number, string, string][] = [
    [counts.aggregates, "aggregate", "aggregates"],
    [counts.entities, "entity", "entities"],
    [counts.valueObjects, "value object", "value objects"],
    [counts.invariants, "invariant", "invariants"],
    [counts.assertions, "assertion", "assertions"],
    [counts.specifications, "specification", "specifications"],
    [counts.events, "event", "events"],
    [counts.services, "service", "services"],
    [counts.questions, "question", "questions"],
    [counts.relations, "relation", "relations"],
  ];
  for (const [n, one, many] of more) if (n > 0) parts.push(phrase(n, one, many));
  return parts.join(" · ");
}
