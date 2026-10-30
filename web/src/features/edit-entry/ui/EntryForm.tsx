import { useEffect, useId, useRef, useState, type KeyboardEvent } from "react";
import { CONCEPT_TITLE, nameHint, nameLabel, propertiesOf, type Concept, type Entry, type PropertySpec } from "../../../entities/language/index";
import { missingIn, NAME, type DraftValues } from "../lib/entry-form";

export interface EntryFormProps {
  concept: Concept;
  /** The recorded entry being edited, or null when the form records a new one. */
  recorded: Entry | null;
  values: DraftValues;
  /** The aggregates of the context, for an event's Raised by. */
  aggregates: readonly string[];
  /** The reason the last save was refused, in the server's words. */
  error: string | null;
  busy: boolean;
  onChange(field: string, value: string): void;
  onSave(): void;
  onCancel(): void;
}

/**
 * One entry's properties as fields, with Save and Cancel. The form never
 * judges the language: it sends the change and shows the refusal it gets
 * back, keeping every typed value. Engineering details (repository,
 * factory, verifying tests) stay folded unless recorded or asked for.
 */
export function EntryForm(props: EntryFormProps) {
  const { concept, recorded, values, aggregates, error, busy, onChange, onSave, onCancel } = props;
  const id = useId();
  const specs = propertiesOf(concept);
  const advanced = specs.filter((s) => s.advanced);
  const plain = specs.filter((s) => !s.advanced);
  const anyAdvancedRecorded = advanced.some((s) => (values[s.name] ?? "").trim() !== "");
  const [showAdvanced, setShowAdvanced] = useState(anyAdvancedRecorded);
  const first = useRef<HTMLInputElement | HTMLTextAreaElement | null>(null);
  useEffect(() => {
    first.current?.focus();
  }, [recorded?.key, concept]);

  const missing = recorded === null ? missingIn(concept, values) : [];
  const nameMissing = recorded === null && (values[NAME] ?? "").trim() === "";

  function onKeyDown(e: KeyboardEvent<HTMLFormElement>) {
    if ((e.ctrlKey || e.metaKey) && (e.key === "Enter" || e.key === "s")) {
      e.preventDefault();
      if (!busy) onSave();
    } else if (e.key === "Escape") {
      e.preventDefault();
      onCancel();
    }
  }

  function field(spec: PropertySpec, index: number) {
    const fieldId = `${id}-${spec.name}`;
    const value = values[spec.name] ?? "";
    const needed = missing.some((m) => m.name === spec.name);
    const ref = index === 0 && recorded !== null ? first : undefined;
    const common = { id: fieldId, name: spec.name, value, disabled: busy, "aria-describedby": `${fieldId}-hint`, "aria-invalid": needed || undefined };
    return (
      <div className="st-field" key={spec.name}>
        <label htmlFor={fieldId}>
          {spec.label}
          {spec.required ? <span className="st-field__required"> needed</span> : <span className="st-field__optional"> optional</span>}
        </label>
        {spec.shape === "prose" || spec.shape === "list" ? (
          <textarea {...common} ref={ref as React.Ref<HTMLTextAreaElement>} rows={spec.shape === "list" ? 3 : 4} onChange={(e) => onChange(spec.name, e.target.value)} />
        ) : spec.shape === "aggregate" ? (
          <select {...common} ref={ref as React.Ref<HTMLSelectElement>} onChange={(e) => onChange(spec.name, e.target.value)}>
            <option value="">No aggregate raises it</option>
            {aggregates.map((a) => (
              <option key={a} value={a}>
                {a}
              </option>
            ))}
            {value !== "" && !aggregates.includes(value) ? <option value={value}>{value} (not recorded in this context)</option> : null}
          </select>
        ) : (
          <input {...common} ref={ref as React.Ref<HTMLInputElement>} type="text" autoComplete="off" spellCheck={false} onChange={(e) => onChange(spec.name, e.target.value)} />
        )}
        <p className="st-field__hint" id={`${fieldId}-hint`}>
          {spec.hint}
        </p>
      </div>
    );
  }

  return (
    <form
      className="st-entry-form"
      aria-label={recorded ? `Edit ${CONCEPT_TITLE[concept]} ${recorded.name}` : `New ${CONCEPT_TITLE[concept]}`}
      onSubmit={(e) => {
        e.preventDefault();
        if (!busy) onSave();
      }}
      onKeyDown={onKeyDown}
    >
      {recorded === null ? (
        <div className="st-field">
          <label htmlFor={`${id}-name`}>
            {nameLabel(concept)}
            <span className="st-field__required"> needed</span>
          </label>
          <input
            id={`${id}-name`}
            name={NAME}
            ref={first as React.Ref<HTMLInputElement>}
            type="text"
            autoComplete="off"
            spellCheck={false}
            value={values[NAME] ?? ""}
            disabled={busy}
            aria-describedby={`${id}-name-hint`}
            onChange={(e) => onChange(NAME, e.target.value)}
          />
          <p className="st-field__hint" id={`${id}-name-hint`}>
            {nameHint(concept)}
          </p>
        </div>
      ) : null}
      {plain.map((spec, i) => field(spec, i))}
      {advanced.length > 0 ? (
        <div className="st-advanced">
          <button type="button" className="st-link-button" aria-expanded={showAdvanced} onClick={() => setShowAdvanced((v) => !v)}>
            {showAdvanced ? "Hide engineering details" : "Engineering details"}
          </button>
          {showAdvanced ? advanced.map((spec, i) => field(spec, plain.length + i)) : null}
        </div>
      ) : null}
      {error ? (
        <p className="st-form-error" role="alert">
          {error}
        </p>
      ) : null}
      {recorded === null && (nameMissing || missing.length > 0) ? (
        <p className="st-form-note">
          Still needed before it can be recorded: {[...(nameMissing ? [nameLabel(concept).toLowerCase()] : []), ...missing.map((m) => m.label.toLowerCase())].join(", ")}.
        </p>
      ) : null}
      <div className="st-form-actions">
        <button type="submit" className="st-button st-button--primary" disabled={busy}>
          {busy ? "Saving…" : recorded ? "Save" : "Record"}
        </button>
        <button type="button" className="st-button" disabled={busy} onClick={onCancel}>
          Cancel
        </button>
        <span className="st-form-keys">
          <kbd>Ctrl</kbd>+<kbd>Enter</kbd> saves · <kbd>Esc</kbd> cancels
        </span>
      </div>
    </form>
  );
}
