import { defineRule } from "arclint";

export default defineRule({
  type: "vertical/repository-context",
  description: "require repository methods to take ctx context.Context first",
  capability: "structural",
  check(ctx) {
    for (const file of ctx.files()) {
      const facts = ctx.facts(file.path);
      if (!facts) {
        continue;
      }
      for (const d of facts.decls) {
        if (d.kind !== "method" || !d.owner.endsWith("Repository")) {
          continue;
        }
        const first = d.params && d.params.length > 0 ? d.params[0] : undefined;
        if (first && first.name === "ctx" && first.type === "context.Context") {
          continue;
        }
        ctx.report({
          path: file.path,
          line: d.startLine,
          message: `Repository method "${d.owner}.${d.name}" must take ctx context.Context as its first parameter`,
        });
      }
    }
  },
});
