import type { RefObject } from "react";
import type { Entry } from "../../../entities/language/index";
import { LanguageSearch } from "../../../features/search-language/index";
import type { AcceptedDocument } from "../../../shared/transport/index";
import type { Mode } from "../model/workspace";

export interface TopBarProps {
  project: string;
  source: string;
  accepted: AcceptedDocument | null;
  entries: readonly Entry[];
  mode: Mode;
  query: string;
  searchRef: RefObject<HTMLInputElement | null>;
  reloading: boolean;
  onQueryChange(query: string): void;
  onPick(entry: Entry): void;
  onProject(): void;
  onMode(mode: Mode): void;
  onReload(): void;
}

export function TopBar(props: TopBarProps) {
  const { project, source, accepted, entries, mode, query, searchRef, reloading, onQueryChange, onPick, onProject, onMode, onReload } = props;
  return (
    <header className="st-topbar">
      <div className="st-brand">
        <span className="st-brand__app">ArcLint Studio</span>
        <span className="st-brand__divider" aria-hidden="true">
          /
        </span>
        <button type="button" className={project === "" ? "st-brand__project is-empty" : "st-brand__project"} onClick={onProject} title="The project and its description">
          {project === "" ? "Unnamed project" : project}
        </button>
        <span className="st-brand__source" title="The domain file the map server reads and writes">
          {source}
        </span>
      </div>
      <LanguageSearch entries={entries} query={query} inputRef={searchRef} onQueryChange={onQueryChange} onPick={onPick} />
      <div className="st-topbar__modes" role="tablist" aria-label="View">
        <button type="button" role="tab" aria-selected={mode === "language"} className="st-mode" onClick={() => onMode("language")}>
          Language
        </button>
        <button type="button" role="tab" aria-selected={mode === "map"} className="st-mode" onClick={() => onMode("map")}>
          Map
        </button>
        <button type="button" role="tab" aria-selected={mode === "features"} className="st-mode" onClick={() => onMode("features")}>
          Features
        </button>
      </div>
      <AcceptedStatus accepted={accepted} />
      <button type="button" className="st-button st-button--small st-button--quiet" onClick={onReload} disabled={reloading} title="Read the domain file again">
        {reloading ? "Reading…" : "Reload"}
      </button>
    </header>
  );
}

function AcceptedStatus({ accepted }: { accepted: AcceptedDocument | null }) {
  if (accepted === null) return <span className="st-accepted st-accepted--unknown">Default branch: reading…</span>;
  if (accepted.available) {
    return (
      <span className="st-accepted" title={`Each entry stands Accepted when ${accepted.branch} records it the same, Proposed when it differs or is new.`}>
        Compared with <b>{accepted.branch}</b>
      </span>
    );
  }
  return (
    <span className="st-accepted st-accepted--unavailable" title={accepted.reason}>
      Default branch unavailable
    </span>
  );
}
