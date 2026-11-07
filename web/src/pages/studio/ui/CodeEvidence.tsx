import { CONCEPT_MARK, CONCEPT_TITLE, type Concept, type Entry } from "../../../entities/language/index";
import type { ContractEvidenceDocument, EntryEvidenceDocument, EvidenceDocument, EvidenceLocationDocument, FindingDocument } from "../../../shared/transport/index";
import { Badge } from "../../../shared/ui/Badge";
import { KindBadge } from "../../../shared/ui/KindBadge";
import { activeCount, BASIS_WORDS, COVERAGE_WORDS, disclosureOf, evidenceOf, partialText, type EvidenceState } from "../model/code-evidence";

export interface CodeEvidenceProps {
  entry: Entry;
  state: EvidenceState;
  onAssess(): void;
  onPeek(location: EvidenceLocationDocument, invoker: HTMLElement): void;
}

/**
 * What the code shows about the selected entry, read-only and only when
 * the reader asks: where it is declared, the declarations carrying its
 * contracts, and the findings attributed to it, beside what the whole
 * assessment could not decide or give to any entry.
 */
export function CodeEvidence({ entry, state, onAssess, onPeek }: CodeEvidenceProps) {
  const { status, document } = state;
  const busy = status === "assessing";
  return (
    <section className="st-section st-evidence" aria-label="Code evidence" aria-busy={busy}>
      <div className="st-evidence__head">
        <h2 className="st-section__heading">Code evidence</h2>
        <span className="st-evidence__readonly">read-only</span>
      </div>
      {status === "unavailable" ? (
        <p className="st-note">{state.error ?? "This host serves no code evidence. The recorded language is unaffected."}</p>
      ) : document === null ? (
        <>
          <p className="st-note">
            Optional. Runs ArcLint's checks over the project's code once and shows where this entry is declared, what carries its contracts and the findings tied to it. Nothing in
            the project changes; an extension may refresh its cache under <code>.arclint/cache</code>.
          </p>
          {state.error ? (
            <p className="st-evidence__alert" role="alert">
              The assessment failed: {state.error}
            </p>
          ) : null}
          <div className="st-evidence__actions">
            <button type="button" className="st-button st-button--small" onClick={onAssess} disabled={busy}>
              {busy ? "Assessing the code…" : status === "failed" ? "Try again" : "Assess code"}
            </button>
          </div>
        </>
      ) : (
        <AssessedEvidence entry={entry} state={state} document={document} busy={busy} onAssess={onAssess} onPeek={onPeek} />
      )}
    </section>
  );
}

function AssessedEvidence({ entry, state, document, busy, onAssess, onPeek }: { entry: Entry; state: EvidenceState; document: EvidenceDocument; busy: boolean; onAssess(): void; onPeek: CodeEvidenceProps["onPeek"] }) {
  const evidence = entry.concept === "question" ? null : evidenceOf(document, entry);
  return (
    <>
      <AssessmentStrip state={state} document={document} busy={busy} onAssess={onAssess} />
      {entry.concept === "question" ? (
        <p className="st-note">A question records no code; there is nothing to assess.</p>
      ) : evidence === null ? (
        <p className="st-evidence__gone" role="status">
          {entry.name} is not in the assessment from {timeOf(document.assessedAt)}. It was recorded or renamed after it; assess again to include it.
        </p>
      ) : (
        <EntryEvidence entry={entry} evidence={evidence} stale={state.stale !== null} onPeek={onPeek} />
      )}
      <Disclosure document={document} onPeek={onPeek} />
    </>
  );
}

function timeOf(iso: string): string {
  const at = new Date(iso);
  return Number.isNaN(at.getTime()) ? iso : at.toLocaleTimeString();
}

function AssessmentStrip({ state, document, busy, onAssess }: { state: EvidenceState; document: EvidenceDocument; busy: boolean; onAssess(): void }) {
  return (
    <div className="st-evidence__strip">
      <span className="st-evidence__when">
        Assessed {timeOf(document.assessedAt)} · {(document.tookMs / 1000).toFixed(1)} s
      </span>
      {state.stale ? <Badge tone="partial">Stale</Badge> : null}
      <span className="st-evidence__grow" />
      <button type="button" className="st-button st-button--small" onClick={onAssess} disabled={busy}>
        {busy ? "Assessing…" : "Assess again"}
      </button>
      {state.stale ? (
        <p className="st-evidence__stale" role="status">
          {state.stale} Shown as of {timeOf(document.assessedAt)}; it may not match the code or the recording now.
        </p>
      ) : null}
      {state.error ? (
        <p className="st-evidence__alert" role="alert">
          The assessment failed: {state.error}
        </p>
      ) : null}
    </div>
  );
}

const READING: Record<string, { tone: "finding" | "partial" | "muted"; label: (n: number) => string; title: string }> = {
  findings: { tone: "finding", label: (n) => `${n} active finding${n === 1 ? "" : "s"}`, title: "Findings this assessment ties to the entry." },
  partial: { tone: "partial", label: () => "Partial", title: "No active finding is tied to the entry, but the assessment did not decide everything it needed to." },
  no_attributed_findings: { tone: "muted", label: () => "No attributed findings", title: "No active finding is tied to this entry. Findings no entry can be given are listed under the assessment; the checks are structural, not proof." },
};

function EntryEvidence({ entry, evidence, stale, onPeek }: { entry: Entry; evidence: EntryEvidenceDocument; stale: boolean; onPeek: CodeEvidenceProps["onPeek"] }) {
  const reading = READING[evidence.reading] ?? READING.partial!;
  const active = activeCount(evidence.findings);
  const declarations = evidence.declarations;
  return (
    <div className={stale ? "st-evidence__entry is-stale" : "st-evidence__entry"}>
      <p className="st-evidence__reading">
        <Badge tone={reading.tone} title={reading.title}>
          {reading.label(active)}
        </Badge>
        <span className="st-evidence__reading-note">{reading.title}</span>
      </p>
      <dl className="st-evidence__facts">
        {evidence.recordedAt ? (
          <div>
            <dt>Recorded at</dt>
            <dd>
              <Location location={evidence.recordedAt} onPeek={onPeek} />
            </dd>
          </div>
        ) : null}
        {declarations.state === "not_applicable" ? null : (
          <div>
            <dt>Declared in code</dt>
            <dd>
              {declarations.state === "located" || declarations.state === "ambiguous" ? (
                <>
                  {declarations.locations.map((l) => (
                    <Location key={`${l.path}:${l.line}`} location={l} onPeek={onPeek} />
                  ))}
                  <span className="st-evidence__aside">
                    {declarations.state === "ambiguous" ? `${declarations.locations.length} declarations spell ${entry.name}; nothing tells which is the model's.` : "Found: presence, not proof."}
                  </span>
                </>
              ) : declarations.state === "missing" ? (
                <span className="st-missing">No declaration spells {entry.name} in {entry.context}'s code.</span>
              ) : (
                <span className="st-evidence__aside">Not observed: none of {entry.context}'s code is in a language ArcLint analyzes here.</span>
              )}
            </dd>
          </div>
        )}
      </dl>
      {evidence.contracts.length > 0 ? (
        <div className="st-evidence__group">
          <h3 className="st-group__heading">Contract carriers</h3>
          <ul className="st-evidence__list">
            {evidence.contracts.map((c) => (
              <Contract key={`${c.kind}:${c.owner}:${c.key}`} contract={c} onPeek={onPeek} />
            ))}
          </ul>
        </div>
      ) : null}
      <div className="st-evidence__group">
        <h3 className="st-group__heading">Findings tied to {entry.name}</h3>
        {evidence.findings.length === 0 ? (
          <p className="st-note">None tied to this entry by its contract, recorded line, bounded context or declaring file.</p>
        ) : (
          <ul className="st-evidence__list">
            {evidence.findings.map((f, i) => (
              <Finding key={`${f.rule}:${f.location?.path ?? ""}:${f.location?.line ?? 0}:${i}`} finding={f} onPeek={onPeek} />
            ))}
          </ul>
        )}
      </div>
      {evidence.partial.length > 0 ? (
        <div className="st-evidence__group">
          <h3 className="st-group__heading">Not decided</h3>
          <ul className="st-evidence__reasons">
            {evidence.partial.map((p, i) => (
              <li key={`${p.kind}:${p.rule ?? ""}:${p.contract ?? ""}:${i}`}>{partialText(p, entry.context)}</li>
            ))}
          </ul>
        </div>
      ) : null}
    </div>
  );
}

function Contract({ contract, onPeek }: { contract: ContractEvidenceDocument; onPeek: CodeEvidenceProps["onPeek"] }) {
  const concept = contract.kind as Concept;
  return (
    <li className="st-evidence__item">
      <span className="st-evidence__item-head">
        <KindBadge mark={CONCEPT_MARK[concept]} label={CONCEPT_TITLE[concept]} tone={concept} size="small" />
        <span className="st-evidence__key">{contract.key}</span>
        <span className={`st-evidence__coverage st-evidence__coverage--${contract.coverage}`}>{COVERAGE_WORDS[contract.coverage] ?? contract.coverage}</span>
      </span>
      <span className="st-evidence__item-body">
        {contract.carrier.state === "found" && contract.carrier.location ? (
          <>
            <span className="st-evidence__label">carried by</span> <Location location={contract.carrier.location} onPeek={onPeek} />
          </>
        ) : contract.carrier.state === "missing" ? (
          <span className="st-missing">Carrier missing: expected {contract.carrier.expected}.</span>
        ) : (
          <span className="st-evidence__aside">Carrier not observed: no check read this code.</span>
        )}
      </span>
      {contract.notChecked.length > 0 ? <span className="st-evidence__aside">Not checked: {contract.notChecked.join("; ")}</span> : null}
    </li>
  );
}

function Finding({ finding, onPeek }: { finding: FindingDocument; onPeek: CodeEvidenceProps["onPeek"] }) {
  return (
    <li className="st-evidence__item">
      <span className="st-evidence__item-head">
        <Badge tone={finding.status === "active" ? "finding" : "muted"} title={`${finding.outcome}, ${finding.assurance} assurance, ${finding.severity}`}>
          {finding.status}
        </Badge>
        <code className="st-evidence__rule">{finding.rule}</code>
      </span>
      <span className="st-evidence__message">{finding.message}</span>
      <span className="st-evidence__item-body">
        {finding.location ? <Location location={finding.location} onPeek={onPeek} /> : <span className="st-evidence__aside">no file location</span>}
        {finding.bases.length > 0 ? <span className="st-evidence__aside">tied through {finding.bases.map((b) => BASIS_WORDS[b] ?? b).join(" and ")}</span> : null}
      </span>
    </li>
  );
}

/**
 * The whole assessment's limits and what no entry can be given. Always
 * shown, so an entry without attributed findings is read beside them.
 */
function Disclosure({ document, onPeek }: { document: EvidenceDocument; onPeek: CodeEvidenceProps["onPeek"] }) {
  const d = disclosureOf(document);
  const unattributed = document.unattributed.findings.length + d.diagnostics;
  const lines: string[] = [];
  if (!d.complete && d.notEvaluated.length > 0) lines.push(`Incomplete: ${d.notEvaluated.length} rule(s) were stopped by their extension and have no result.`);
  if (d.parseFailures.length > 0) lines.push(`${d.parseFailures.length} analyzed file(s) failed to parse and were skipped.`);
  if (d.unattributedActive > 0) lines.push(`${d.unattributedActive} active finding(s) are not tied to any entry.`);
  if (d.unattributedOther > 0) lines.push(`${d.unattributedOther} suppressed or baselined finding(s) are not tied to any entry.`);
  if (d.operational > 0) lines.push(`${d.operational} operational diagnostic(s), such as unknown imports.`);
  if (d.coverage > 0) lines.push(`${d.coverage} coverage note(s): what rules did not decide, did not run, or were disabled.`);
  return (
    <div className="st-evidence__group st-evidence__disclosure" aria-label="The whole assessment">
      <h3 className="st-group__heading">The whole assessment</h3>
      {lines.length === 0 ? (
        <p className="st-note">Every rule ran and decided, every analyzed file parsed, and every finding is tied to an entry.</p>
      ) : (
        <ul className="st-evidence__reasons">
          {lines.map((l) => (
            <li key={l}>{l}</li>
          ))}
        </ul>
      )}
      {unattributed > 0 ? (
        <details className="st-evidence__details">
          <summary>
            Not tied to any entry ({document.unattributed.findings.length} finding{document.unattributed.findings.length === 1 ? "" : "s"}, {d.diagnostics} diagnostic{d.diagnostics === 1 ? "" : "s"})
          </summary>
          <ul className="st-evidence__list">
            {document.unattributed.findings.map((f, i) => (
              <Finding key={`u:${f.rule}:${f.location?.path ?? ""}:${f.location?.line ?? 0}:${i}`} finding={f} onPeek={onPeek} />
            ))}
            {document.unattributed.diagnostics.map((g, i) => (
              <li className="st-evidence__item" key={`d:${g.kind}:${g.rule}:${i}`}>
                <span className="st-evidence__item-head">
                  <Badge tone="muted">{g.kind.replace("_", " ")}</Badge>
                  {g.rule ? <code className="st-evidence__rule">{g.rule}</code> : null}
                </span>
                <span className="st-evidence__message">{g.message}</span>
                {g.location ? (
                  <span className="st-evidence__item-body">
                    <Location location={g.location} onPeek={onPeek} />
                  </span>
                ) : null}
              </li>
            ))}
          </ul>
        </details>
      ) : null}
      {d.parseFailures.length > 0 ? (
        <details className="st-evidence__details">
          <summary>Files that failed to parse ({d.parseFailures.length})</summary>
          <ul className="st-evidence__paths">
            {d.parseFailures.map((p) => (
              <li key={p}>
                <code>{p}</code>
              </li>
            ))}
          </ul>
        </details>
      ) : null}
    </div>
  );
}

/**
 * A location: a button opening the peek when this snapshot may read it
 * at a line, plain text otherwise. A location without a file is never
 * drawn as a link.
 */
function Location({ location, onPeek }: { location: EvidenceLocationDocument; onPeek: CodeEvidenceProps["onPeek"] }) {
  const text = location.line > 0 ? `${location.path}:${location.line}` : location.path;
  if (!location.readable || location.line <= 0) {
    return (
      <span className="st-evidence__location st-evidence__location--plain" title="No excerpt of this location can be read">
        {text}
      </span>
    );
  }
  return (
    <button type="button" className="st-evidence__location" aria-label={`Peek at ${text}`} onClick={(e) => onPeek(location, e.currentTarget)}>
      {text}
    </button>
  );
}
