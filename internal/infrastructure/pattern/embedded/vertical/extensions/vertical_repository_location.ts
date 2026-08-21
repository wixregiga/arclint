import { defineRule, s } from "arclint";

export default defineRule({
  type: "vertical/repository-location",
  description: "require repository types to live in the application module",
  capability: "heuristic",
  params: s.object({
    module: s.string(),
  }),
  check(ctx, params) {
    const allowed = String(params.module);
    for (const file of ctx.files()) {
      const facts = ctx.facts(file.path);
      let earliest = 0;
      if (facts) {
        for (const d of facts.decls) {
          const typeHit =
            (d.kind === "interface" || d.kind === "struct" || d.kind === "type") &&
            d.name.endsWith("Repository");
          const methodHit = d.kind === "method" && d.owner.endsWith("Repository");
          if (typeHit || methodHit) {
            if (earliest === 0 || d.startLine < earliest) {
              earliest = d.startLine;
            }
          }
        }
      }
      const byName = file.stem.endsWith("_repository");
      if (!byName && earliest === 0) {
        continue;
      }
      const mods = ctx.moduleOf(file.path);
      let allowedHere = false;
      for (const m of mods) {
        if (m === allowed) {
          allowedHere = true;
          break;
        }
      }
      if (allowedHere) {
        continue;
      }
      ctx.report({
        path: file.path,
        line: earliest || 1,
        message: `repository types must live in module "${allowed}"`,
      });
    }
  },
});
