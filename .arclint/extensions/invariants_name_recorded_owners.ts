// The invariant-ownership capability: every recorded invariant's owner
// must itself be a term recorded in the same bounded context — an
// entity (enforced in behavior) or a value object (enforced at
// construction). An invariant with no owner at all is the
// require-defined-terms rule's finding, not this one's. Owners must
// use the canonical term, never an alias. Inert while the project
// records no vocabulary.
import { defineRule } from "arclint";

const VOCABULARY = "ubiquitous-language.yaml";

export default defineRule({
  type: "invariants-name-recorded-owners",
  description:
    "every recorded invariant is owned by a term recorded in its context",
  capability: "structural",
  check(ctx) {
    for (const bound of ctx.domain().contexts) {
      const canonical = new Set<string>();
      const aliased: Record<string, string> = {};
      for (const term of [...bound.entities, ...bound.valueObjects]) {
        canonical.add(term.name);
        for (const alias of term.aliases ?? []) {
          aliased[alias] = term.name;
        }
      }
      for (const invariant of bound.invariants) {
        if (!invariant.owner || canonical.has(invariant.owner)) continue;
        const resolved = aliased[invariant.owner];
        ctx.report({
          path: VOCABULARY,
          line: invariant.line,
          message: `invariant owner "${invariant.owner}" in context "${bound.name}" is not a recorded entity or value object of that context`,
          fixHint: resolved
            ? `"${invariant.owner}" is an alias: name the canonical term "${resolved}"`
            : "record the owner as an entity or value object, or name a recorded term",
        });
      }
    }
  },
});
