import type { Entry } from "../../../entities/language/index";
import type { EntryEvidenceDocument, EvidenceDocument, FindingDocument } from "../../../shared/transport/index";

/**
 * What Studio knows of the code evidence. `unavailable`: the host lends
 * no evidence, or the server has none. `idle`: nothing assessed yet.
 * `assessing`: an assessment runs; the last document, when any, stays
 * shown. `ready`: the last assessment answered. `failed`: the last
 * assessment failed; an earlier document stays shown, stale.
 *
 * `stale` says why the shown document may no longer describe the code
 * or the recording; a stale document is never presented as current.
 */
export interface EvidenceState {
  status: "unavailable" | "idle" | "assessing" | "ready" | "failed";
  document: EvidenceDocument | null;
  stale: string | null;
  error: string | null;
}

export const NO_EVIDENCE: EvidenceState = { status: "unavailable", document: null, stale: null, error: null };
export const NOT_ASSESSED: EvidenceState = { status: "idle", document: null, stale: null, error: null };

/** The evidence of one recorded entry in the document, or null when the assessment holds no such entry. */
export function evidenceOf(document: EvidenceDocument, entry: Pick<Entry, "concept" | "context" | "owner" | "name">): EntryEvidenceDocument | null {
  return document.entries.find((e) => e.concept === entry.concept && e.context === entry.context && e.owner === entry.owner && e.name === entry.name) ?? null;
}

/** How many findings report and gate. */
export function activeCount(findings: readonly FindingDocument[]): number {
  return findings.filter((f) => f.status === "active").length;
}

/**
 * What the whole assessment could not decide or give to an entry. Shown
 * beside every entry's evidence, so an entry without attributed findings
 * never reads as if the code had none.
 */
export interface AssessmentDisclosure {
  complete: boolean;
  notEvaluated: string[];
  parseFailures: string[];
  operational: number;
  /** Coverage and not-evaluated notes: what the assessment did not decide or run. */
  coverage: number;
  unattributedActive: number;
  unattributedOther: number;
  diagnostics: number;
}

export function disclosureOf(document: EvidenceDocument): AssessmentDisclosure {
  const findings = document.unattributed.findings;
  const active = activeCount(findings);
  return {
    complete: document.complete,
    notEvaluated: document.notEvaluated,
    parseFailures: document.parseFailures,
    operational: document.unattributed.diagnostics.filter((d) => d.kind === "operational").length,
    coverage: document.unattributed.diagnostics.filter((d) => d.kind !== "operational").length,
    unattributedActive: active,
    unattributedOther: findings.length - active,
    diagnostics: document.unattributed.diagnostics.length,
  };
}

/** The coverage status of a contract, in words a reader asks with. */
export const COVERAGE_WORDS: Record<string, string> = {
  structurally_checked: "structurally checked",
  incomplete: "incomplete",
  unchecked: "not checked",
  violates: "finding",
  suspected_violation: "suspected finding",
  undetermined: "undetermined",
  unsupported: "not observable",
  failed: "check failed",
  not_applicable: "excluded from its rules",
};

/** The basis tying a finding to the entry, in words. */
export const BASIS_WORDS: Record<string, string> = {
  contract: "its contract",
  recorded_line: "its recorded line",
  bounded_context: "its bounded context",
  declaring_file: "a file declaring it",
};

/** A partiality, in one sentence. */
export function partialText(p: { kind: string; rule?: string; contract?: string; outcome?: string; paths?: string[] }, context: string): string {
  switch (p.kind) {
    case "assessment_incomplete":
      return `Rule ${p.rule ?? ""} was stopped by its extension; the assessment has no result for it.`;
    case "parse_failed":
      return `${p.paths?.length ?? 0} analyzed file(s) failed to parse and were skipped.`;
    case "contract_undecided":
      return `Contract ${p.contract ?? ""} was not decided: ${COVERAGE_WORDS[p.outcome ?? ""] ?? p.outcome ?? ""}.`;
    case "check_undecided":
      return `Rule ${p.rule ?? ""} did not decide for ${context} or a file declaring this entry (${COVERAGE_WORDS[p.outcome ?? ""] ?? p.outcome ?? ""}).`;
    case "not_observed":
      return `None of ${context}'s code is in a language ArcLint analyzes here.`;
    default:
      return p.kind;
  }
}
