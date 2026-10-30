import { isConcept, type Concept } from "../../../entities/language/index";
import type { DraftValues } from "../../../features/edit-entry/index";
import type { FeatureDocument, FeatureReferenceDocument } from "../../../shared/transport/index";
import { featureDraftKey, sameText, restoreFeatureDrafts, restoreFeatureSelection, type FeatureDraft, type FeatureSelection } from "./feature-drafts";

/**
 * What the page holds that is not in the recording: the mode, what is
 * selected, whether its form is open, the entry drafts typed but not
 * saved, and the feature drafts. Drafts are keyed by what they edit, so
 * a draft survives moving to another entry, switching between Language,
 * Map and Features, or reloading the page, and is dropped only by
 * Cancel, Discard, or a save that went through.
 */
export type Mode = "language" | "map" | "features";

export type Selection =
  | { kind: "project" }
  | { kind: "entry"; key: string }
  | { kind: "new"; concept: Concept; context: string; owner: string };

export interface Workspace {
  mode: Mode;
  selection: Selection;
  editing: boolean;
  drafts: Record<string, DraftValues>;
  feature: FeatureSelection | null;
  featureDrafts: Record<string, FeatureDraft>;
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

export const initialWorkspace: Workspace = { mode: "language", selection: { kind: "project" }, editing: false, drafts: {}, feature: null, featureDrafts: {} };

export type WorkspaceAction =
  | { type: "select"; selection: Selection }
  | { type: "mode"; mode: Mode }
  | { type: "begin-edit"; key: string; values: DraftValues }
  | { type: "resume-edit" }
  | { type: "draft"; key: string; field: string; value: string }
  | { type: "cancel-edit"; key: string; then: Selection }
  | { type: "discard-draft"; key: string }
  | { type: "saved"; key: string; then: Selection }
  | { type: "forget-selection"; then: Selection }
  | { type: "open-feature"; feature: FeatureSelection | null }
  /** The author typed; the draft begins from `base` when there is none yet. */
  | { type: "feature-draft"; feature: FeatureSelection; base: FeatureDraft; title?: string; body?: string; references?: FeatureReferenceDocument[] }
  | { type: "feature-rebase"; feature: FeatureSelection; revision: string }
  | { type: "feature-discard"; feature: FeatureSelection }
  /** A save went through; edits typed after the submitted snapshot remain a draft. */
  | { type: "feature-saved"; feature: FeatureSelection; document: FeatureDocument }
  | { type: "feature-removed"; feature: FeatureSelection };

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
    case "open-feature":
      return { ...w, feature: action.feature };
    case "feature-draft": {
      const key = featureDraftKey(action.feature);
      const current = w.featureDrafts[key] ?? action.base;
      const next: FeatureDraft = {
        ...current,
        title: action.title ?? current.title,
        body: action.body ?? current.body,
        references: action.references ?? current.references,
      };
      return { ...w, featureDrafts: { ...w.featureDrafts, [key]: next } };
    }
    case "feature-rebase": {
      const key = featureDraftKey(action.feature);
      const current = w.featureDrafts[key];
      if (!current) return w;
      return { ...w, featureDrafts: { ...w.featureDrafts, [key]: { ...current, revision: action.revision } } };
    }
    case "feature-discard":
      return { ...w, featureDrafts: without(w.featureDrafts, featureDraftKey(action.feature)) };
    case "feature-saved": {
      const key = featureDraftKey(action.feature);
      const current = w.featureDrafts[key];
      const next: FeatureSelection = { kind: "saved", id: action.document.id };
      const featureDrafts = without(w.featureDrafts, key);
      if (current && !sameText(current, action.document)) {
        featureDrafts[featureDraftKey(next)] = { ...current, revision: action.document.revision };
      }
      const sameSelection = w.feature !== null && featureDraftKey(w.feature) === key;
      return { ...w, feature: sameSelection ? next : w.feature, featureDrafts };
    }
    case "feature-removed": {
      const same = w.feature !== null && featureDraftKey(w.feature) === featureDraftKey(action.feature);
      return { ...w, feature: same ? null : w.feature, featureDrafts: without(w.featureDrafts, featureDraftKey(action.feature)) };
    }
  }
}

function without<T>(drafts: Record<string, T>, key: string): Record<string, T> {
  if (!(key in drafts)) return drafts;
  const out = { ...drafts };
  delete out[key];
  return out;
}

const VERSION = 2;

export function serializeWorkspace(w: Workspace): string {
  return JSON.stringify({ version: VERSION, mode: w.mode, selection: w.selection, editing: w.editing, drafts: w.drafts, feature: w.feature, featureDrafts: w.featureDrafts });
}

/** The workspace a memory held, or the initial one when it held nothing readable. */
export function restoreWorkspace(text: string | null): Workspace {
  if (text === null) return initialWorkspace;
  try {
    const raw: unknown = JSON.parse(text);
    if (!raw || typeof raw !== "object" || ![1, VERSION].includes((raw as { version?: unknown }).version as number)) return initialWorkspace;
    const r = raw as { mode?: unknown; selection?: unknown; editing?: unknown; drafts?: unknown; feature?: unknown; featureDrafts?: unknown };
    const selection = restoreSelection(r.selection);
    return {
      mode: r.mode === "map" ? "map" : r.mode === "features" ? "features" : "language",
      selection,
      editing: selection.kind === "new" ? true : r.editing === true,
      drafts: restoreDrafts(r.drafts),
      feature: restoreFeatureSelection(r.feature),
      featureDrafts: restoreFeatureDrafts(r.featureDrafts),
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
