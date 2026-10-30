import { useCallback, useEffect, useMemo, useRef, useState, type KeyboardEvent } from "react";
import {
  CONCEPT_TITLE,
  differences,
  entriesOf,
  entryAt,
  entryKey,
  membersOf,
  proposedKeys,
  relationDifferences,
  standingOf as standingIn,
  type Concept,
  type Entry,
  type Standing,
} from "../../../entities/language/index";
import { projectRequestFrom, type ProjectValues } from "../../../features/describe-project/index";
import { defineRequestFrom, draftOf, hasChanges, NAME, removeRequestFrom } from "../../../features/edit-entry/index";
import type { StudioMemory } from "../../../shared/lib/studio-memory/index";
import { isGatewayFailure, type StudioGateway } from "../../../shared/transport/index";
import { afterRemoval, whereBegun } from "../lib/addresses";
import { failureText, useLanguage } from "../model/use-language";
import { useWorkspace } from "../model/use-workspace";
import { draftKeyOf, PROJECT_DRAFT, type Selection } from "../model/workspace";
import { EntryPanel } from "./EntryPanel";
import { LanguageMap } from "./LanguageMap";
import { Navigator } from "./Navigator";
import { NewEntryPanel } from "./NewEntryPanel";
import { countsLine, ProjectPanel } from "./ProjectPanel";
import { TopBar } from "./TopBar";

export interface StudioScreenProps {
  gateway: StudioGateway;
  memory: StudioMemory;
}

interface Failure {
  key: string;
  message: string;
  /** The save to try again when the server could not be reached. */
  retry: (() => void) | null;
}

/**
 * The Studio screen: the recorded language in a navigator, one entry or
 * the project in the card, the map beside an inspector in map mode. Every
 * write goes through the gateway; the recording is read again after each.
 */
export function StudioScreen({ gateway, memory }: StudioScreenProps) {
  const { language, reload } = useLanguage(gateway);
  const [workspace, dispatch] = useWorkspace(memory);
  const [query, setQuery] = useState("");
  const [pending, setPending] = useState<string | null>(null);
  const [failure, setFailure] = useState<Failure | null>(null);
  const [notice, setNotice] = useState<string | null>(null);
  const [reloading, setReloading] = useState(false);
  const searchRef = useRef<HTMLInputElement>(null);

  const overview = language.overview;
  const entries = useMemo(() => entriesOf(overview), [overview]);
  const relations = useMemo(() => overview?.relations ?? [], [overview]);
  const accepted = language.accepted;
  const acceptedEntries = useMemo(() => (accepted?.available ? entriesOf(accepted.overview) : []), [accepted]);
  const diffs = useMemo(() => (accepted?.available ? differences(acceptedEntries, entries) : []), [accepted, acceptedEntries, entries]);
  const relationDiffs = useMemo(() => (accepted?.available ? relationDifferences(accepted.overview.relations ?? [], relations) : []), [accepted, relations]);
  const proposed = useMemo(() => proposedKeys(diffs), [diffs]);
  const standingOf = useMemo<((key: string) => Standing) | null>(() => (accepted?.available ? (key) => standingIn(proposed, key) : null), [accepted, proposed]);
  const branch = accepted?.available ? accepted.branch : null;

  const selection = workspace.selection;
  const selected = selection.kind === "entry" ? entryAt(entries, selection.key) : null;
  const draftKey = draftKeyOf(selection);
  const draft = workspace.drafts[draftKey] ?? null;
  const draftKeys = useMemo(() => new Set(Object.keys(workspace.drafts)), [workspace.drafts]);

  // A selection the recording no longer holds falls back to the project.
  useEffect(() => {
    if (language.status !== "ready") return;
    if (selection.kind === "entry" && !selected) dispatch({ type: "forget-selection", then: { kind: "project" } });
    if (selection.kind === "new" && selection.concept !== "bounded_context" && !entries.some((e) => e.concept === "bounded_context" && e.name === selection.context)) {
      dispatch({ type: "forget-selection", then: { kind: "project" } });
    }
  }, [language.status, selection, selected, entries, dispatch]);

  const activeContext = useMemo(() => {
    if (selection.kind === "entry" && selected) return selected.context;
    if (selection.kind === "new" && selection.context !== "") return selection.context;
    return entries.find((e) => e.concept === "bounded_context")?.name ?? null;
  }, [selection, selected, entries]);

  const select = useCallback(
    (entry: Entry) => {
      setFailure(null);
      dispatch({ type: "select", selection: { kind: "entry", key: entry.key } });
    },
    [dispatch],
  );

  function beginNew(concept: Concept, context: string, owner: string) {
    setFailure(null);
    dispatch({ type: "select", selection: { kind: "new", concept, context, owner } });
  }

  function beginEdit() {
    setFailure(null);
    if (selection.kind === "project" && overview) {
      dispatch({ type: "begin-edit", key: PROJECT_DRAFT, values: { name: overview.project ?? "", description: overview.description ?? "" } });
    } else if (selected) {
      dispatch({ type: "begin-edit", key: selected.key, values: draftOf(selected.concept, selected) });
    }
  }

  async function run(key: string, what: () => Promise<string>, then: Selection) {
    setPending(key);
    setFailure(null);
    try {
      const said = await what();
      await reload();
      dispatch({ type: "saved", key, then });
      setNotice(said);
    } catch (error) {
      const retryable = isGatewayFailure(error) && error.kind !== "refused";
      setFailure({ key, message: failureText(error), retry: retryable ? () => void run(key, what, then) : null });
    } finally {
      setPending(null);
    }
  }

  function saveEntry() {
    if (!selected || !draft) return;
    if (!hasChanges(selected.concept, selected, draft)) {
      dispatch({ type: "cancel-edit", key: draftKey, then: selection });
      return;
    }
    const request = defineRequestFrom(selected, selected, draft);
    void run(draftKey, async () => {
      const report = await gateway.define(request);
      return `${report.result} ${CONCEPT_TITLE[selected.concept].toLowerCase()} ${selected.name}` + (report.changed?.length ? ` (${report.changed.join(", ")})` : "");
    }, { kind: "entry", key: selected.key });
  }

  function saveNew() {
    if (selection.kind !== "new" || !draft) return;
    const { concept, context, owner } = selection;
    const request = defineRequestFrom({ concept, context, owner }, null, draft);
    const name = request.name;
    const key = entryKey(concept === "bounded_context" ? name : context, concept, owner, name);
    void run(draftKey, async () => {
      const report = await gateway.define(request);
      return `${report.result} ${CONCEPT_TITLE[concept].toLowerCase()} ${name}`;
    }, { kind: "entry", key });
  }

  function saveProject() {
    if (!overview || !draft) return;
    const recorded: ProjectValues = { name: overview.project ?? "", description: overview.description ?? "" };
    const request = projectRequestFrom(recorded, { name: draft.name ?? "", description: draft.description ?? "" });
    if (request === null) {
      dispatch({ type: "cancel-edit", key: PROJECT_DRAFT, then: { kind: "project" } });
      return;
    }
    void run(PROJECT_DRAFT, async () => {
      const report = await gateway.describeProject(request);
      return `${report.result} project` + (report.changed?.length ? ` (${report.changed.join(", ")})` : "");
    }, { kind: "project" });
  }

  function removeSelected() {
    if (!selected) return;
    const gone = selected;
    const members = membersOf(entries, gone).map((m) => m.key);
    void run(`remove\u001f${gone.key}`, async () => {
      const report = await gateway.remove(removeRequestFrom(gone));
      for (const k of [gone.key, ...members]) if (k in workspace.drafts) dispatch({ type: "discard-draft", key: k });
      return `removed ${CONCEPT_TITLE[gone.concept].toLowerCase()} ${gone.name}` + (report.also?.length ? `; ${report.also.join(" ")}` : "");
    }, afterRemoval(entries, gone));
  }

  function cancel() {
    setFailure(null);
    if (selection.kind === "new") dispatch({ type: "cancel-edit", key: draftKey, then: whereBegun(entries, selection.context, selection.owner) });
    else dispatch({ type: "cancel-edit", key: draftKey, then: selection });
  }

  async function reloadNow() {
    setReloading(true);
    const ok = await reload();
    setReloading(false);
    if (ok) setNotice("Read the domain file again.");
  }

  function onKeyDown(e: KeyboardEvent<HTMLDivElement>) {
    if ((e.ctrlKey || e.metaKey) && e.key === "k") {
      e.preventDefault();
      searchRef.current?.focus();
      searchRef.current?.select();
    }
  }

  const busy = pending !== null;
  const errorFor = (key: string) => (failure && failure.key === key ? failure.message : null);

  let panel: React.ReactNode = null;
  if (overview) {
    if (selection.kind === "new") {
      panel = (
        <NewEntryPanel
          concept={selection.concept}
          context={selection.context}
          owner={selection.owner}
          entries={entries}
          values={draft ?? { [NAME]: "" }}
          busy={busy}
          error={errorFor(draftKey)}
          onChange={(field, value) => dispatch({ type: "draft", key: draftKey, field, value })}
          onSave={saveNew}
          onCancel={cancel}
        />
      );
    } else if (selection.kind === "entry" && selected) {
      panel = (
        <EntryPanel
          entry={selected}
          entries={entries}
          relations={relations}
          standing={standingOf ? standingOf(selected.key) : null}
          branch={branch}
          difference={diffs.find((d) => d.key === selected.key) ?? null}
          draft={draft}
          editing={workspace.editing}
          busy={busy}
          error={errorFor(draftKey) ?? errorFor(`remove\u001f${selected.key}`)}
          onSelect={select}
          onEdit={beginEdit}
          onResume={() => dispatch({ type: "resume-edit" })}
          onDiscard={() => dispatch({ type: "discard-draft", key: draftKey })}
          onChange={(field, value) => dispatch({ type: "draft", key: draftKey, field, value })}
          onSave={saveEntry}
          onCancel={cancel}
          onRemove={removeSelected}
          onNew={beginNew}
        />
      );
    } else {
      panel = (
        <ProjectPanel
          overview={overview}
          accepted={accepted}
          entries={entries}
          differences={diffs}
          relationDifferences={relationDiffs}
          values={draft ? { name: draft.name ?? "", description: draft.description ?? "" } : null}
          editing={workspace.editing}
          busy={busy}
          error={errorFor(PROJECT_DRAFT)}
          onEdit={beginEdit}
          onResume={() => dispatch({ type: "resume-edit" })}
          onDiscard={() => dispatch({ type: "discard-draft", key: PROJECT_DRAFT })}
          onChange={(field, value) => dispatch({ type: "draft", key: PROJECT_DRAFT, field, value })}
          onSave={saveProject}
          onCancel={cancel}
          onNewContext={() => beginNew("bounded_context", "", "")}
          onSelect={select}
        />
      );
    }
  }

  return (
    <div className="st-app" onKeyDown={onKeyDown}>
      <TopBar
        project={overview?.project ?? ""}
        source={overview?.source ?? ""}
        accepted={accepted}
        entries={entries}
        mode={workspace.mode}
        query={query}
        searchRef={searchRef}
        reloading={reloading}
        onQueryChange={setQuery}
        onPick={select}
        onProject={() => {
          setFailure(null);
          dispatch({ type: "select", selection: { kind: "project" } });
        }}
        onMode={(mode) => dispatch({ type: "mode", mode })}
        onReload={() => void reloadNow()}
      />
      {language.status === "failed" && !overview ? (
        <main className="st-body st-body--failed">
          <div className="st-card st-card--failure" role="alert">
            <h1 className="st-card__title">Studio cannot read the recording</h1>
            <p className="st-reading">{language.error}</p>
            <p className="st-note">Is `arclint map serve` still running? Your unsaved edits are kept.</p>
            <div className="st-card__actions">
              <button type="button" className="st-button st-button--primary" onClick={() => void reloadNow()} disabled={reloading}>
                Try again
              </button>
            </div>
          </div>
        </main>
      ) : language.status === "loading" ? (
        <main className="st-body st-body--loading" aria-busy="true">
          <p className="st-note">Reading the recording…</p>
        </main>
      ) : (
        <div className={workspace.mode === "map" ? "st-body st-body--map" : "st-body"}>
          <Navigator entries={entries} selection={selection} standingOf={standingOf} draftKeys={draftKeys} onSelect={select} onNew={beginNew} />
          {workspace.mode === "map" ? (
            <>
              <main className="st-main st-main--map">
                <LanguageMap entries={entries} relations={relations} context={activeContext} selectedKey={selection.kind === "entry" ? selection.key : null} standingOf={standingOf} onSelect={select} />
              </main>
              <aside className="st-inspector" aria-label="Inspector">
                {panel}
              </aside>
            </>
          ) : (
            <main className="st-main">{panel}</main>
          )}
        </div>
      )}
      <footer className="st-status">
        {language.error && overview ? (
          <span className="st-status__error" role="alert">
            {language.error}{" "}
            <button type="button" className="st-link-button" onClick={() => void reloadNow()}>
              Try again
            </button>
          </span>
        ) : failure && failure.retry ? (
          <span className="st-status__error" role="alert">
            {failure.message}{" "}
            <button type="button" className="st-link-button" onClick={failure.retry}>
              Try again
            </button>
          </span>
        ) : notice ? (
          <span className="st-status__notice" role="status">
            {notice}
          </span>
        ) : (
          <span />
        )}
        <span className="st-status__grow" />
        {overview ? <span className="st-status__counts">{countsLine(overview.counts)}</span> : null}
      </footer>
    </div>
  );
}
