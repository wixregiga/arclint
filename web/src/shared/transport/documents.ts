/*
 * The documents the arclint map server exchanges with Studio, spelled as
 * the JSON renderer spells them (internal/delivery/cli/adapters/report/
 * json/domain.go) and as mapserve/server.go reads an edit. Studio reads
 * and writes the project's recorded language only through these shapes.
 */

export interface CountsDocument {
  contexts: number;
  aggregates: number;
  entities: number;
  valueObjects: number;
  invariants: number;
  assertions: number;
  specifications: number;
  events: number;
  services: number;
  questions: number;
  relations: number;
}

export interface InvariantDocument {
  key: string;
  statement: string;
  verifiedBy?: string[];
}

export interface AssertionDocument {
  key: string;
  on: string;
  statement: string;
  verifiedBy?: string[];
}

export interface EntityDocument {
  name: string;
  definition: string;
  identity?: string;
  aliases?: string[];
}

export interface AggregateDocument {
  name: string;
  definition: string;
  identity: string;
  aliases?: string[];
  entities?: EntityDocument[];
  invariants?: InvariantDocument[];
  assertions?: AssertionDocument[];
  repository?: string;
  factory?: string;
}

export interface ValueObjectDocument {
  name: string;
  definition: string;
  aliases?: string[];
  invariants?: InvariantDocument[];
}

export interface EventDocument {
  name: string;
  definition: string;
  raisedBy?: string;
}

export interface ServiceDocument {
  name: string;
  definition: string;
  assertions?: AssertionDocument[];
}

export interface TermDocument {
  name: string;
  definition: string;
}

export interface QuestionDocument {
  key: string;
  text: string;
}

export interface ContextDocument {
  name: string;
  definition: string;
  aggregates?: AggregateDocument[];
  valueObjects?: ValueObjectDocument[];
  events?: EventDocument[];
  services?: ServiceDocument[];
  specifications?: TermDocument[];
  questions?: QuestionDocument[];
}

export interface RelationDocument {
  from: string;
  to: string;
  kind: string;
  description?: string;
}

/** GET api/overview: the recording as the domain file holds it now. */
export interface OverviewDocument {
  source: string;
  found: boolean;
  project?: string;
  description?: string;
  counts: CountsDocument;
  contexts?: ContextDocument[];
  relations?: RelationDocument[];
}

/** GET api/accepted: the recording the remote default branch holds, or why there is none. */
export type AcceptedDocument =
  | { available: true; branch: string; overview: OverviewDocument }
  | { available: false; branch?: string; reason: string };

/**
 * POST api/define: one edit. A property present in change is set, even
 * to an empty value; one absent stays as recorded.
 */
export interface ChangeRequest {
  definition?: string;
  statement?: string;
  on?: string;
  verifiedBy?: string[];
  text?: string;
  identity?: string;
  aliases?: string[];
  repository?: string;
  factory?: string;
  raisedBy?: string;
}

export interface DefineRequest {
  concept: string;
  context: string;
  owner: string;
  name: string;
  change: ChangeRequest;
}

/** POST api/remove: the entry as `arclint domain remove <concept> <name>` names it. */
export interface RemoveRequest {
  concept: string;
  context: string;
  owner: string;
  name: string;
}

/** POST api/project: a property absent stays as recorded. */
export interface ProjectRequest {
  name?: string;
  description?: string;
}

/** The report of a define or a project edit, as `domain define --format json` prints it. */
export interface DefineReport {
  result: string;
  type: string;
  name?: string;
  context?: string;
  owner?: string;
  changed?: string[];
  values?: Record<string, unknown>;
  check?: { home: string; check: string; operation: string };
}

/** The report of a remove, as `domain remove --format json` prints it. */
export interface RemoveReport {
  result: string;
  type: string;
  name: string;
  context?: string;
  owner?: string;
  also?: string[];
  sourceFilesChanged: boolean;
}
