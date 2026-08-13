// The embedded arclint extension SDK. Extensions import it as "arclint";
// the host resolves that specifier to this source during bundling, so no
// npm, Node, or tsc ever runs on the user's machine.
//
// defineRule computes the params JSON Schema eagerly (registration phase);
// the HOST validates rules.yaml params against it before check() is ever
// invoked. esbuild strips types without checking them, so type safety is
// an author-time editor concern backed by the generated arclint.d.ts.

export type Severity = "error" | "warn" | "info";
export type Contract = "consumes" | "provides" | "invariant";
export type Blame = "consumer" | "provider";

export interface FileInfo {
  path: string;
  name: string;
  stem: string;
  ext: string;
  dir: string;
  size: number;
}

export interface ImportInfo {
  path: string;
  line: number;
  /** stdlib | internal | external | unknown | cgo */
  class: string;
  /** Repo-relative package directory for resolved internal imports, else "". */
  targetDir: string;
  /** Repo-relative file for file-granular languages (JS/TS, Python), else "". */
  targetFile: string;
}

export interface ViolationInput {
  path: string;
  message: string;
  line?: number;
  fixHint?: string;
  severity?: Severity;
  /** Per-finding contract override; defaults to the rule type's contract. */
  contract?: Contract;
  /** Per-finding blame override; defaults to the rule type's blame. */
  blame?: Blame;
}

export interface DeclInfo {
  /** struct | interface | type | class | enum | func | method | field | const | var */
  kind: string;
  name: string;
  /** Enclosing declaration for members (receiver, interface, class), else "". */
  owner: string;
  exported: boolean;
  startLine: number;
  endLine: number;
}

export interface FactsInfo {
  path: string;
  /** Go package clause; "" for other languages. */
  package: string;
  decls: DeclInfo[];
  parseError?: string;
}

export interface Ctx {
  /** Repository files, optionally filtered by a doublestar glob. */
  files(glob?: string): FileInfo[];
  /** Read one file's content. Throws on unreadable paths. */
  read(path: string): string;
  /** Classified imports of one file, for every active language target. */
  imports(path: string): ImportInfo[];
  /** Declared module names to their member file paths. */
  modules(): Record<string, string[]>;
  /** Declaration facts for one file (lazy, cached); null when no active
   * target owns the file. Go facts are parser-exact; TS/Python come from
   * pinned tree-sitter grammars. */
  facts(path: string): FactsInfo | null;
  /** The sorted module names a file belongs to. */
  moduleOf(path: string): string[];
  /** Report one violation. */
  report(v: ViolationInput): void;
}

interface SchemaState {
  schema: Record<string, unknown>;
  optional: boolean;
  hasDefault: boolean;
}

export interface Schema {
  readonly __schema: true;
  readonly __state: SchemaState;
  optional(): Schema;
  default(v: unknown): Schema;
  describe(d: string): Schema;
  toJSON(): Record<string, unknown>;
}

function node(base: Record<string, unknown>): Schema {
  const state: SchemaState = { schema: { ...base }, optional: false, hasDefault: false };
  const api: Schema = {
    __schema: true,
    __state: state,
    optional() {
      state.optional = true;
      return api;
    },
    default(v: unknown) {
      state.schema.default = v;
      state.hasDefault = true;
      return api;
    },
    describe(d: string) {
      state.schema.description = d;
      return api;
    },
    toJSON() {
      return { ...state.schema };
    },
  };
  return api;
}

/** Minimal zod-style schema builder producing JSON Schema. */
export const s = {
  string: () => node({ type: "string" }),
  number: () => node({ type: "number" }),
  integer: () => node({ type: "integer" }),
  boolean: () => node({ type: "boolean" }),
  enum: (...values: string[]) => node({ type: "string", enum: values }),
  array: (items: Schema) => node({ type: "array", items: items.toJSON() }),
  object: (props: Record<string, Schema>) => {
    const properties: Record<string, unknown> = {};
    const required: string[] = [];
    for (const key of Object.keys(props)) {
      const child = props[key];
      properties[key] = child.toJSON();
      if (!child.__state.optional && !child.__state.hasDefault) {
        required.push(key);
      }
    }
    const schema: Record<string, unknown> = {
      type: "object",
      properties,
      additionalProperties: false,
    };
    if (required.length > 0) {
      schema.required = required;
    }
    return node(schema);
  },
};

export type Capability = "exact" | "structural" | "heuristic" | "advisory";

export interface RuleDef {
  /** Unique rule type name, referenced by rules.yaml entries. */
  type: string;
  /** One-line summary shown by arclint explain / rules ls. */
  description?: string;
  /** Contract clause this rule reports under. Default: invariant. */
  contract?: Contract;
  /** Blame side for violations. Default: provider. */
  blame?: Blame;
  /** How this rule enforces its claim: exact (imports/syntax facts),
   * structural (paths/declarations), heuristic (names/regex/complexity),
   * advisory (guidance only). Default: heuristic, the conservative claim. */
  capability?: Capability;
  /** Params schema; YAML params are host-validated against it. */
  params?: Schema;
  check(ctx: Ctx, params: Record<string, unknown>): void;
}

export function defineRule(def: RuleDef) {
  if (!def || typeof def.type !== "string" || def.type.length === 0) {
    throw new Error("defineRule: type is required and must be a non-empty string");
  }
  if (typeof def.check !== "function") {
    throw new Error("defineRule: check must be a function");
  }
  if (def.description !== undefined && typeof def.description !== "string") {
    throw new Error("defineRule: description must be a string");
  }
  const contract = def.contract ?? "invariant";
  if (contract !== "consumes" && contract !== "provides" && contract !== "invariant") {
    throw new Error(`defineRule: invalid contract "${contract}"`);
  }
  const blame = def.blame ?? "provider";
  if (blame !== "consumer" && blame !== "provider") {
    throw new Error(`defineRule: invalid blame "${blame}"`);
  }
  const capability = def.capability ?? "heuristic";
  if (
    capability !== "exact" &&
    capability !== "structural" &&
    capability !== "heuristic" &&
    capability !== "advisory"
  ) {
    throw new Error(`defineRule: invalid capability "${capability}"`);
  }
  const paramsSchema = def.params
    ? def.params.toJSON()
    : { type: "object", properties: {}, additionalProperties: false };
  return {
    __arclintRule: true,
    type: def.type,
    description: def.description ?? "",
    contract,
    blame,
    capability,
    paramsSchema,
    check: def.check,
  };
}
