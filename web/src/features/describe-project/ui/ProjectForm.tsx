import { useEffect, useId, useRef, type KeyboardEvent } from "react";
import type { ProjectValues } from "../lib/project-form";

export interface ProjectFormProps {
  recorded: ProjectValues;
  values: ProjectValues;
  /** True when no domain file exists yet: naming the project begins the recording. */
  begins: boolean;
  error: string | null;
  busy: boolean;
  onChange(field: keyof ProjectValues, value: string): void;
  onSave(): void;
  onCancel(): void;
}

/** The project's name and description, the heading of its language. */
export function ProjectForm({ recorded, values, begins, error, busy, onChange, onSave, onCancel }: ProjectFormProps) {
  const id = useId();
  const nameRef = useRef<HTMLInputElement>(null);
  useEffect(() => {
    nameRef.current?.focus();
  }, []);
  const unchanged = values.name.trim() === recorded.name && values.description.trim() === recorded.description;

  function onKeyDown(e: KeyboardEvent<HTMLFormElement>) {
    if ((e.ctrlKey || e.metaKey) && (e.key === "Enter" || e.key === "s")) {
      e.preventDefault();
      if (!busy) onSave();
    } else if (e.key === "Escape") {
      e.preventDefault();
      onCancel();
    }
  }

  return (
    <form
      className="st-entry-form"
      aria-label={begins ? "Name the project" : "Edit the project"}
      onSubmit={(e) => {
        e.preventDefault();
        if (!busy) onSave();
      }}
      onKeyDown={onKeyDown}
    >
      <div className="st-field">
        <label htmlFor={`${id}-name`}>
          Project name<span className="st-field__required"> needed</span>
        </label>
        <input id={`${id}-name`} ref={nameRef} name="projectName" type="text" autoComplete="off" spellCheck={false} value={values.name} disabled={busy} onChange={(e) => onChange("name", e.target.value)} aria-describedby={`${id}-name-hint`} />
        <p className="st-field__hint" id={`${id}-name-hint`}>
          {begins ? "Naming the project writes its domain file; the language begins there." : "The name the domain file records; no file moves."}
        </p>
      </div>
      <div className="st-field">
        <label htmlFor={`${id}-description`}>
          Description<span className="st-field__optional"> optional</span>
        </label>
        <textarea id={`${id}-description`} name="projectDescription" rows={4} value={values.description} disabled={busy} onChange={(e) => onChange("description", e.target.value)} aria-describedby={`${id}-description-hint`} />
        <p className="st-field__hint" id={`${id}-description-hint`}>
          What the project is for, in the team's words. At most 100 words.
        </p>
      </div>
      {error ? (
        <p className="st-form-error" role="alert">
          {error}
        </p>
      ) : null}
      <div className="st-form-actions">
        <button type="submit" className="st-button st-button--primary" disabled={busy || unchanged}>
          {busy ? "Saving…" : begins ? "Begin the language" : "Save"}
        </button>
        <button type="button" className="st-button" disabled={busy} onClick={onCancel}>
          Cancel
        </button>
      </div>
    </form>
  );
}
