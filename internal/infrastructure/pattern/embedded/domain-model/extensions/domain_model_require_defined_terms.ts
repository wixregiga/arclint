// The vocabulary capability: consume the project's recorded Ubiquitous
// Language through ctx.domain() and report every recorded term that
// carries no definition text, plus every assertion or invariant missing
// a statement or owner. Inert while the project records no vocabulary.
import { defineRule } from "arclint";

export default defineRule({
  type: "domain-model/require-defined-terms",
  description: "every recorded vocabulary term carries a definition",
  check(ctx) {
    const domain = ctx.domain();
    const VOCABULARY = domain.source;
    for (const bound of domain.contexts ?? []) {
      const groups: [
        string,
        { name: string; definition?: string; line: number }[],
      ][] = [
        ["entity", bound.entities ?? []],
        ["value object", bound.valueObjects ?? []],
        ["specification", bound.specifications ?? []],
        ["domain event", bound.events ?? []],
      ];
      for (const [kind, terms] of groups) {
        for (const term of terms) {
          if (!term.definition) {
            ctx.report({
              path: VOCABULARY,
              line: term.line,
              message: `${kind} "${term.name}" has no definition recorded in the project vocabulary`,
              fixHint: "arclint domain define <type> <name> --definition <text>",
            });
          }
        }
      }
      for (const a of bound.assertions ?? []) {
        if (!a.statement) {
          ctx.report({
            path: VOCABULARY,
            line: a.line,
            message: `assertion in context "${bound.name}" has no statement recorded in the project vocabulary`,
            fixHint: "record the assertion statement, owner, id, and on",
          });
        }
        if (!a.owner) {
          ctx.report({
            path: VOCABULARY,
            line: a.line,
            message: `assertion in context "${bound.name}" has no owner recorded in the project vocabulary`,
            fixHint: "record exactly one owner term for the assertion",
          });
        }
      }
      for (const inv of bound.invariants ?? []) {
        if (!inv.statement) {
          ctx.report({
            path: VOCABULARY,
            line: inv.line,
            message: `invariant in context "${bound.name}" has no statement recorded in the project vocabulary`,
            fixHint: "record the invariant statement and its owner",
          });
        }
        if (!inv.owner) {
          ctx.report({
            path: VOCABULARY,
            line: inv.line,
            message: `invariant in context "${bound.name}" has no owner recorded in the project vocabulary`,
            fixHint: "record exactly one owner term for the invariant",
          });
        }
      }
    }
  },
});
