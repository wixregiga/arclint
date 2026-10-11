import { defineRule, s } from "arclint";

// hexagonal/allowed-packages: a zone uses only the third-party packages
// its rule lists. A package is an import from outside the repository
// that the project declares (go.mod, package.json, pyproject.toml); it
// is matched exactly as the code spells it, a Go package path, a
// TypeScript specifier, or a Python dotted module, so listing effect
// allows "effect" and not "effect/http". The standard library and the
// repository's own code are not judged here; the zone's imports rule
// judges those.
export default defineRule({
  type: "hexagonal/allowed-packages",
  description: "a zone uses only the listed third-party packages, matched exactly as written",
  capability: "exact",
  params: s.object({
    packages: s.array(s.string()),
  }),
  check(ctx, params) {
    const packages = params.packages as string[];
    for (const file of ctx.files()) {
      for (const imp of ctx.imports(file.path)) {
        if (imp.class === "external" && !packages.includes(imp.path)) {
          ctx.report({
            path: file.path,
            line: imp.line,
            message: packages.length === 0
              ? `package ${imp.path} is not allowed; this zone uses no third-party package`
              : `package ${imp.path} is not one of the packages this zone may use: ${packages.join(", ")}`,
          });
        }
      }
    }
  },
});
