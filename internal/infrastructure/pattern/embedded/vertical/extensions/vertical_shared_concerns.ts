import { defineRule, s } from "arclint";

export default defineRule({
  type: "vertical/shared-concerns",
  description: "require shared files to live under an allowed concern folder",
  capability: "structural",
  params: s.object({
    concerns: s.array(s.string()),
  }),
  check(ctx, params) {
    const allowed = params.concerns as string[];
    const prefix = "internal/shared/";
    for (const file of ctx.files()) {
      const path = file.path.replace(/\\/g, "/");
      if (!path.startsWith(prefix)) {
        ctx.report({
          path: file.path,
          message: `shared files must live under ${prefix}<concern>/`,
        });
        continue;
      }
      const rest = path.slice(prefix.length);
      const slash = rest.indexOf("/");
      if (slash === -1) {
        ctx.report({
          path: file.path,
          message: `shared files must live under ${prefix}<concern>/`,
        });
        continue;
      }
      const concern = rest.slice(0, slash);
      let ok = false;
      for (const c of allowed) {
        if (c === concern) {
          ok = true;
          break;
        }
      }
      if (!ok) {
        ctx.report({
          path: file.path,
          message: `shared concern "${concern}" is not allowed`,
        });
      }
    }
  },
});
