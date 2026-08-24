// The vocabulary capability, delivered as an SDK extension: consume
// the project's recorded Ubiquitous Language through ctx.domain() and
// report every recorded term that carries no definition text, plus
// every invariant missing a statement or owner. This file doubles as
// the ctx.domain() showcase test case.
import { defineRule } from "arclint";

export default defineRule({
  type: "require-defined-terms",
  description: "every recorded vocabulary term carries a definition",
  check(ctx) {
    const domain = ctx.domain();
    for (const bound of domain.contexts ?? []) {
      const groups: [string, { name: string; definition?: string }[]][] = [
        ["entity", bound.entities ?? []],
        ["value object", bound.valueObjects ?? []],
        ["domain event", bound.events ?? []],
      ];
      for (const [kind, terms] of groups) {
        for (const term of terms) {
          if (!term.definition) {
            ctx.report({
              path: "ubiquitous-language.yaml",
              line: 1,
              message: `${kind} "${term.name}" has no definition recorded in the project vocabulary`,
              fixHint: "arclint domain define <type> <name> --definition <text>",
            });
          }
        }
      }
      for (const inv of bound.invariants ?? []) {
        if (!inv.statement) {
          ctx.report({
            path: "ubiquitous-language.yaml",
            line: 1,
            message: `invariant in context "${bound.name}" has no statement recorded in the project vocabulary`,
            fixHint: "record the invariant statement and its owner",
          });
        }
        if (!inv.owner) {
          ctx.report({
            path: "ubiquitous-language.yaml",
            line: 1,
            message: `invariant in context "${bound.name}" has no owner recorded in the project vocabulary`,
            fixHint: "record exactly one owner term for the invariant",
          });
        }
      }
    }
  },
});
