// The vocabulary capability, delivered as an SDK extension: consume
// the project's recorded Ubiquitous Language through ctx.domain() and
// report every recorded term that carries no definition text. This
// file doubles as the ctx.domain() showcase test case.
import { defineRule } from "arclint";

export default defineRule({
  type: "require-defined-terms",
  description: "every recorded vocabulary term carries a definition",
  check(ctx) {
    const domain = ctx.domain();
    const groups: [string, { name: string; definition?: string }[]][] = [
      ["entity", domain.entities],
      ["value object", domain.valueObjects],
      ["business rule", domain.businessRules],
      ["domain event", domain.events],
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
  },
});
