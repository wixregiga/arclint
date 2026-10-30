import { isConcept, type Concept } from "../../../entities/language/index";
import type { DraftValues } from "../../../features/edit-entry/index";

/**
 * What the page holds that is not in the recording: the mode, what is
 * selected, whether its form is open, and the drafts typed but not saved.
 * Drafts are keyed by what they edit, so a draft survives moving to another
 * entry, switching to the map, or reloading the page, and is dropped only
 * by Cancel, Discard, or a save that went through.
 */
export type Mode = "language" | "map";

export type Selection =
  | { kind: "project" }
  | { kind: "entry"; key: string }
  | { kind: "new"; concept: Concept; context: string; owner: string };

export interface Workspace {
  mode: Mode;
  selection: Selection;
  editing: boolean;
  drafts: Record<string, DraftValues>;
}

export const PROJECT_DRAFT = "project";

export function draftKeyOf(selection: Selection): string {
  switch (selection.kind) {
    case "project":
      return PROJECT_DRAFT;
    case "entry":
      return selection.key;
    case "new":
      return ["new", selection.context, selection.concept, selection.owner].join("\u001f");
  }
}

export const initialWorkspace: Workspace = { mode: "language", selection: { kind: "project" }, editing: false, drafts: {} };

export type WorkspaceAction =
  | { type: "select"; selection: Selection }
  | { type: "mode"; mode: Mode }
  | { type: "begin-edit"; key: string; values: DraftValues }
  | { type: "resume-edit" }
  | { type: "draft"; key: string; field: string; value: string }
  | { type: "cancel-edit"; key: string; then: Selection }
  | { type: "discard-draft"; key: string }
  | { type: "saved"; key: string; then: Selection }
  | { type: "forget-selection"; then: Selection };

export function reduceWorkspace(w: Workspace, action: WorkspaceAction): Workspace {
  switch (action.type) {
    case "select":
      return { ...w, selection: action.selection, editing: action.selection.kind === "new" };
    case "mode":
      return { ...w, mode: action.mode };
    case "begin-edit":
      return { ...w, editing: true, drafts: action.key in w.drafts ? w.drafts : { ...w.drafts, [action.key]: action.values } };
    case "resume-edit":
      return { ...w, editing: true };
    case "draft": {
      const current = w.drafts[action.key] ?? {};
      return { ...w, drafts: { ...w.drafts, [action.key]: { ...current, [action.field]: action.value } } };
    }
    case "cancel-edit":
      return { ...w, editing: false, selection: w.selection.kind === "new" ? action.then : w.selection, drafts: without(w.drafts, action.key) };
    case "discard-draft":
      return { ...w, editing: false, drafts: without(w.drafts, action.key) };
    case "saved":
      return { ...w, editing: false, selection: action.then, drafts: without(w.drafts, action.key) };
    case "forget-selection":
      return { ...w, editing: false, selection: action.then };
  }
}

function without(drafts: Record<string, DraftValues>, key: string): Record<string, DraftValues> {
  if (!(key in drafts)) return drafts;
  const out = { ...drafts };
  delete out[key];
  return out;
}

const VERSION = 1;

export function serializeWorkspace(w: Workspace): string {
  return JSON.stringify({ version: VERSION, mode: w.mode, selection: w.selection, editing: w.editing, drafts: w.drafts });
}

/** The workspace a memory held, or the initial one when it held nothing readable. */
export function restoreWorkspace(text: string | null): Workspace {
  if (text === null) return initialWorkspace;
  try {
    const raw: unknown = JSON.parse(text);
    if (!raw || typeof raw !== "object" || (raw as { version?: unknown }).version !== VERSION) return initialWorkspace;
    const r = raw as { mode?: unknown; selection?: unknown; editing?: unknown; drafts?: unknown };
    const selection = restoreSelection(r.selection);
    return {
      mode: r.mode === "map" ? "map" : "language",
      selection,
      editing: selection.kind === "new" ? true : r.editing === true,
      drafts: restoreDrafts(r.drafts),
    };
  } catch {
    return initialWorkspace;
  }
}

function restoreSelection(raw: unknown): Selection {
  if (!raw || typeof raw !== "object") return { kind: "project" };
  const s = raw as { kind?: unknown; key?: unknown; concept?: unknown; context?: unknown; owner?: unknown };
  if (s.kind === "entry" && typeof s.key === "string") return { kind: "entry", key: s.key };
  if (s.kind === "new" && typeof s.concept === "string" && isConcept(s.concept) && typeof s.context === "string" && typeof s.owner === "string") {
    return { kind: "new", concept: s.concept, context: s.context, owner: s.owner };
  }
  return { kind: "project" };
}

function restoreDrafts(raw: unknown): Record<string, DraftValues> {
  if (!raw || typeof raw !== "object") return {};
  const out: Record<string, DraftValues> = {};
  for (const [key, values] of Object.entries(raw as Record<string, unknown>)) {
    if (!values || typeof values !== "object") continue;
    const draft: DraftValues = {};
    for (const [field, value] of Object.entries(values as Record<string, unknown>)) if (typeof value === "string") draft[field] = value;
    out[key] = draft;
  }
  return out;
}
