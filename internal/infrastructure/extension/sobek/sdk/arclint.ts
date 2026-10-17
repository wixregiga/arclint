// The embedded arclint extension SDK. Extensions import it as "arclint";
// the host resolves that specifier to this source during bundling, so no
// npm, Node, or tsc ever runs on the user's machine.
//
// defineRule computes the params JSON Schema eagerly (registration phase);
// the HOST validates rules.arclint.yaml params against it before check() is ever
// invoked. esbuild strips types without checking them, so type safety is
// an author-time editor concern backed by the generated arclint.d.ts.

import type * as API from "./api_gen";
import type { Schema, RuleDef } from "./api_gen";
export type { Capability, Ctx, RuleDef, Schema, TermCase } from "./api_gen";
export type * from "./types_gen";

interface SchemaState {
  schema: Record<string, unknown>;
  optional: boolean;
  hasDefault: boolean;
}

interface RuntimeSchema extends Schema {
  readonly __schema: true;
  readonly __state: SchemaState;
}

// DefinitionError marks a malformed declaration: a defineRule(...) call or
// a params schema built from something that is not a schema. The host
// reports it as a configuration error, never as extension code that
// failed while running. Each one is handed to the host as it is created,
// so the host recognizes it by identity without running extension code.
class DefinitionError extends Error {
  constructor(message: string) {
    super(message);
    this.name = "ArclintDefinitionError";
    const report = (globalThis as { __arclintDefinitionError?: (e: Error, m: string) => void })
      .__arclintDefinitionError;
    if (typeof report === "function") {
      report(this, message);
    }
  }
}

// schemaOf accepts only a schema built with s, so a malformed params
// declaration fails as a definition error rather than a TypeError.
function schemaOf(value: unknown, where: string): RuntimeSchema {
  if (!value || (value as RuntimeSchema).__schema !== true) {
    throw new DefinitionError(`${where} must be a schema built with s`);
  }
  return value as RuntimeSchema;
}

function node(base: Record<string, unknown>): RuntimeSchema {
  const state: SchemaState = { schema: { ...base }, optional: false, hasDefault: false };
  const api: RuntimeSchema = {
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
export const s: typeof API.s = {
  string: () => node({ type: "string" }),
  number: () => node({ type: "number" }),
  integer: () => node({ type: "integer" }),
  boolean: () => node({ type: "boolean" }),
  enum: (...values: string[]) => {
    for (const value of values) {
      if (typeof value !== "string") {
        throw new DefinitionError("s.enum: every value must be a string");
      }
    }
    return node({ type: "string", enum: values });
  },
  array: (items: Schema) => node({ type: "array", items: schemaOf(items, "s.array: items").toJSON() }),
  object: (props: Record<string, Schema>) => {
    if (props === null || typeof props !== "object" || Array.isArray(props)) {
      throw new DefinitionError("s.object: properties must be an object of schemas built with s");
    }
    const properties: Record<string, unknown> = {};
    const required: string[] = [];
    for (const key of Object.keys(props)) {
      const child = schemaOf(props[key], `s.object: property "${key}"`);
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

export const defineRule: typeof API.defineRule = (def: RuleDef) => {
  if (!def || typeof def.type !== "string" || def.type.length === 0) {
    throw new DefinitionError("defineRule: type is required and must be a non-empty string");
  }
  if (typeof def.check !== "function") {
    throw new DefinitionError("defineRule: check must be a function");
  }
  if (def.description !== undefined && typeof def.description !== "string") {
    throw new DefinitionError("defineRule: description must be a string");
  }
  const capability = def.capability ?? "heuristic";
  if (typeof capability !== "string") {
    throw new DefinitionError("defineRule: capability must be a string");
  }
  if (
    capability !== "exact" &&
    capability !== "structural" &&
    capability !== "heuristic" &&
    capability !== "advisory"
  ) {
    throw new DefinitionError(`defineRule: invalid capability "${capability}"`);
  }
  const paramsSchema = def.params != null
    ? schemaOf(def.params, "defineRule: params").toJSON()
    : { type: "object", properties: {}, additionalProperties: false };
  return {
    __arclintRule: true,
    type: def.type,
    description: def.description ?? "",
    capability,
    paramsSchema,
    check: def.check,
  };
};
