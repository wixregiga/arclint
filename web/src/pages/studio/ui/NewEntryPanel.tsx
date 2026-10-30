import { CONCEPT_MARK, CONCEPT_MEANING, CONCEPT_TITLE, ofConcept, type Concept, type Entry } from "../../../entities/language/index";
import { EntryForm, type DraftValues } from "../../../features/edit-entry/index";
import { KindBadge } from "../../../shared/ui/KindBadge";

export interface NewEntryPanelProps {
  concept: Concept;
  context: string;
  owner: string;
  entries: readonly Entry[];
  values: DraftValues;
  busy: boolean;
  error: string | null;
  onChange(field: string, value: string): void;
  onSave(): void;
  onCancel(): void;
}

/** The form that records a new entry where it was begun. */
export function NewEntryPanel({ concept, context, owner, entries, values, busy, error, onChange, onSave, onCancel }: NewEntryPanelProps) {
  const where = concept === "bounded_context" ? "in this project" : owner ? `under ${owner} in ${context}` : `in ${context}`;
  return (
    <article className="st-card" aria-label={`New ${CONCEPT_TITLE[concept]}`}>
      <header className="st-card__head">
        <div className="st-eyebrow">New {CONCEPT_TITLE[concept].toLowerCase()} {where}</div>
        <div className="st-card__title-row">
          <KindBadge mark={CONCEPT_MARK[concept]} label={CONCEPT_TITLE[concept]} tone={concept} size="large" />
          <h1 className="st-card__title">{(values.$name ?? "").trim() === "" ? `New ${CONCEPT_TITLE[concept].toLowerCase()}` : values.$name}</h1>
        </div>
        <p className="st-note">{CONCEPT_MEANING[concept]}</p>
      </header>
      <EntryForm concept={concept} recorded={null} values={values} aggregates={ofConcept(entries, context, "aggregate").map((a) => a.name)} error={error} busy={busy} onChange={onChange} onSave={onSave} onCancel={onCancel} />
    </article>
  );
}
