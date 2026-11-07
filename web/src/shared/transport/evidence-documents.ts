/*
 * The code evidence the arclint map server answers with, spelled as
 * internal/delivery/mapserve/evidence.go spells it. Every value is
 * read-only: Studio shows the evidence and changes nothing with it.
 */

/** A file and line; `readable` says whether an excerpt of it may be read in this snapshot. */
export interface EvidenceLocationDocument {
  path: string;
  line: number;
  readable: boolean;
}

/** How an entry's evidence reads as a whole. */
export type EvidenceReading = "findings" | "partial" | "no_attributed_findings";

/** How the entry's declaration was found. Located is presence, never proof. */
export type DeclarationState = "located" | "ambiguous" | "missing" | "not_observed" | "not_applicable";

/** What ties a finding to the entry, read from the finding's structure. */
export type EvidenceBasis = "contract" | "recorded_line" | "bounded_context" | "declaring_file";

export type PartialKind = "assessment_incomplete" | "parse_failed" | "contract_undecided" | "check_undecided" | "not_observed";

export interface PartialDocument {
  kind: PartialKind;
  rule?: string;
  contract?: string;
  outcome?: string;
  paths?: string[];
}

export interface FindingDocument {
  rule: string;
  message: string;
  /** active, suppressed or baselined. */
  status: string;
  severity: string;
  outcome: string;
  assurance: string;
  subject: { kind: string; identity: string };
  location: EvidenceLocationDocument | null;
  bases: EvidenceBasis[];
  remediation?: string;
}

export interface ContractEvidenceDocument {
  kind: "invariant" | "assertion";
  owner: string;
  key: string;
  carrier: { state: "found" | "missing" | "not_observed"; location: EvidenceLocationDocument | null; expected: string };
  /** The coverage status: structurally_checked, incomplete, unchecked, or the outcome that decided it. */
  coverage: string;
  notChecked: string[];
}

export interface EntryEvidenceDocument {
  concept: string;
  context: string;
  owner: string;
  name: string;
  reading: EvidenceReading;
  recordedAt: EvidenceLocationDocument | null;
  declarations: { state: DeclarationState; locations: EvidenceLocationDocument[] };
  contracts: ContractEvidenceDocument[];
  findings: FindingDocument[];
  partial: PartialDocument[];
}

export interface DiagnosticDocument {
  kind: string;
  rule: string;
  severity: string;
  message: string;
  location: EvidenceLocationDocument | null;
}

/** One assessment of the repository, attributed to the recorded entries. */
export interface EvidenceDocument {
  snapshot: string;
  assessedAt: string;
  tookMs: number;
  recording: string;
  found: boolean;
  complete: boolean;
  notEvaluated: string[];
  parseFailures: string[];
  entries: EntryEvidenceDocument[];
  unattributed: { findings: FindingDocument[]; diagnostics: DiagnosticDocument[] };
}

export interface ExcerptRequest {
  snapshot: string;
  path: string;
  line: number;
}

/**
 * A few lines of a file as it is on disk when the excerpt is read. The
 * assessment kept no copy, so they may differ from the code it judged;
 * `changedSinceAssessment` is true when the file's size differs from the
 * size the assessment observed, and an equal size proves nothing.
 */
export interface ExcerptDocument {
  snapshot: string;
  path: string;
  line: number;
  totalLines: number;
  lines: { number: number; text: string; truncated?: boolean }[];
  changedSinceAssessment: boolean;
}
