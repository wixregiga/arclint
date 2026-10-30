import { Badge } from "../../../shared/ui/Badge";
import { featureDraftKey, type FeatureDraft, type FeatureSelection } from "../model/feature-drafts";
import type { FeatureListState } from "../model/use-feature-store";

export interface FeaturesNavigatorProps {
  list: FeatureListState;
  /** True when the host lends a store; without one nothing can be saved or listed. */
  storeAvailable: boolean;
  selection: FeatureSelection | null;
  drafts: Record<string, FeatureDraft>;
  onOpen(selection: FeatureSelection): void;
  onNew(): void;
  onRefresh(): void;
}

/**
 * The saved features as the store lists them, and the new ones typed
 * but not saved yet. A saved feature with unsaved typing is marked.
 */
export function FeaturesNavigator({ list, storeAvailable, selection, drafts, onOpen, onNew, onRefresh }: FeaturesNavigatorProps) {
  const currentKey = selection ? featureDraftKey(selection) : null;
  const newDrafts = Object.entries(drafts)
    .filter(([key]) => key.startsWith("new\u001f"))
    .map(([key, draft]) => ({ key: key.slice("new\u001f".length), draft }));
  const listedIds = new Set(list.features.map((f) => f.id));
  const unlistedSaved = Object.entries(drafts)
    .filter(([key]) => key.startsWith("saved\u001f") && !listedIds.has(key.slice("saved\u001f".length)))
    .map(([key, draft]) => ({ id: key.slice("saved\u001f".length), draft }));

  return (
    <nav className="st-navigator" aria-label="Features">
      <div className="st-navigator__head">
        <h2 className="st-navigator__heading">Features</h2>
        <button type="button" className="st-button st-button--small" onClick={onNew} title="Begin a new feature">
          + Feature
        </button>
      </div>
      {!storeAvailable ? <p className="st-navigator__empty">This host lends no feature store: features can be written here but not saved.</p> : null}
      {storeAvailable && list.status === "loading" ? <p className="st-navigator__empty">Reading saved features…</p> : null}
      {storeAvailable && list.error ? (
        <p className="st-navigator__empty" role="alert">
          {list.error}{" "}
          <button type="button" className="st-link-button" onClick={onRefresh}>
            Try again
          </button>
        </p>
      ) : null}
      {storeAvailable && list.status === "ready" && list.features.length === 0 && newDrafts.length === 0 ? <p className="st-navigator__empty">No feature is saved yet. Begin one and write freely; the language cues itself as you type.</p> : null}
      <ul className="st-nav-list" aria-label="Saved features">
        {list.features.map((f) => {
          const key = featureDraftKey({ kind: "saved", id: f.id });
          return (
            <li key={f.id} className="st-nav-item">
              <div className={key === currentKey ? "st-nav-row is-current" : "st-nav-row"}>
                <button type="button" className="st-nav-row__main" aria-current={key === currentKey || undefined} onClick={() => onOpen({ kind: "saved", id: f.id })}>
                  <span className="st-nav-row__name">{f.title.trim() === "" ? <i>Untitled feature</i> : f.title}</span>
                  {key in drafts ? <Badge tone="draft">unsaved</Badge> : null}
                </button>
              </div>
            </li>
          );
        })}
        {unlistedSaved.map(({ id, draft }) => {
          const key = featureDraftKey({ kind: "saved", id });
          return (
            <li key={`gone-${id}`} className="st-nav-item">
              <div className={key === currentKey ? "st-nav-row is-current" : "st-nav-row"}>
                <button type="button" className="st-nav-row__main" aria-current={key === currentKey || undefined} onClick={() => onOpen({ kind: "saved", id })}>
                  <span className="st-nav-row__name">{draft.title.trim() === "" ? <i>Untitled feature</i> : draft.title}</span>
                  <Badge tone="removed">not in store</Badge>
                </button>
              </div>
            </li>
          );
        })}
      </ul>
      {newDrafts.length > 0 ? (
        <section className="st-nav-context" aria-label="Not saved yet">
          <h3 className="st-navigator__heading st-navigator__heading--sub">Not saved yet</h3>
          <ul className="st-nav-list">
            {newDrafts.map(({ key, draft }) => {
              const draftKey = featureDraftKey({ kind: "new", key });
              return (
                <li key={key} className="st-nav-item">
                  <div className={draftKey === currentKey ? "st-nav-row is-current" : "st-nav-row"}>
                    <button type="button" className="st-nav-row__main" aria-current={draftKey === currentKey || undefined} onClick={() => onOpen({ kind: "new", key })}>
                      <span className="st-nav-row__name">{draft.title.trim() === "" ? <i>Untitled feature</i> : draft.title}</span>
                      <Badge tone="draft">new</Badge>
                    </button>
                  </div>
                </li>
              );
            })}
          </ul>
        </section>
      ) : null}
    </nav>
  );
}
