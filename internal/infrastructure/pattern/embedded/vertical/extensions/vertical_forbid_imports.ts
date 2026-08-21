import { defineRule, s } from "arclint";

export default defineRule({
  type: "vertical/forbid-imports",
  description: "forbid imports of named packages and their subpackages",
  capability: "exact",
  params: s.object({
    packages: s.array(s.string()),
  }),
  check(ctx, params) {
    const packages = params.packages as string[];
    for (const file of ctx.files()) {
      for (const imp of ctx.imports(file.path)) {
        for (const pkg of packages) {
          if (imp.path === pkg || imp.path.startsWith(pkg + "/")) {
            ctx.report({
              path: file.path,
              line: imp.line,
              message: `import of ${imp.path} is forbidden`,
            });
            break;
          }
        }
      }
    }
  },
});
