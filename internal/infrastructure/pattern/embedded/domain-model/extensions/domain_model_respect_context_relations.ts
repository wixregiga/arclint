// The context-map capability: recorded relations between bounded
// contexts become import rules over the Modules named after those
// contexts. A relation's `from` names the upstream context and `to`
// the downstream one; the downstream may depend on the upstream, so
// one-way kinds forbid the upstream module importing the downstream
// module, and separate_ways forbids integration in both directions.
// Partnership and shared_kernel record mutual dependence and constrain
// nothing. A context with no Module of the same name is unobservable
// here and is skipped. Inert while the project records no vocabulary.
// Context names bind to module names through the host's TermCase
// capability (flatcase): the same casing rules.arclint.yaml placeholders
// resolve with, never a local reimplementation.
import { defineRule } from "arclint";

function dirOf(path: string): string {
  const cut = path.lastIndexOf("/");
  return cut < 0 ? "." : path.slice(0, cut);
}

// Relation kinds where dependence flows downstream -> upstream only.
const ONE_WAY = new Set([
  "customer_supplier",
  "conformist",
  "anticorruption_layer",
  "open_host_service",
  "published_language",
]);

export default defineRule({
  type: "domain-model/respect-context-relations",
  description: "module imports respect the recorded context-map relations",
  capability: "exact",
  check(ctx) {
    const modules = ctx.modules();
    const moduleBySlug: Record<string, string> = {};
    for (const name of Object.keys(modules)) {
      moduleBySlug[ctx.caseTerm(name, "flatcase")] = name;
    }
    const forbidImports = (
      fromModule: string,
      intoModule: string,
      relation: { from: string; to: string; kind: string },
    ) => {
      const targetFiles = new Set(modules[intoModule]);
      const targetDirs = new Set(modules[intoModule].map(dirOf));
      for (const file of modules[fromModule]) {
        for (const imp of ctx.imports(file)) {
          if (imp.class !== "internal") continue;
          const hit =
            (imp.targetFile !== "" && targetFiles.has(imp.targetFile)) ||
            (imp.targetDir !== "" && targetDirs.has(imp.targetDir));
          if (hit) {
            ctx.report({
              path: file,
              line: imp.line,
              message: `module "${fromModule}" must not import module "${intoModule}": the recorded ${relation.kind} relation ("${relation.from}" -> "${relation.to}") forbids it`,
              fixHint:
                relation.kind === "separate_ways"
                  ? "separate_ways records no integration: remove the dependency"
                  : "dependence flows downstream to upstream only: invert or remove the import",
            });
          }
        }
      }
    };
    for (const relation of ctx.domain().relations) {
      if (
        relation.kind === "partnership" ||
        relation.kind === "shared_kernel"
      ) {
        continue;
      }
      const upstream = moduleBySlug[ctx.caseTerm(relation.from, "flatcase")];
      const downstream = moduleBySlug[ctx.caseTerm(relation.to, "flatcase")];
      if (!upstream || !downstream || upstream === downstream) continue;
      if (ONE_WAY.has(relation.kind)) {
        forbidImports(upstream, downstream, relation);
      } else if (relation.kind === "separate_ways") {
        forbidImports(upstream, downstream, relation);
        forbidImports(downstream, upstream, relation);
      }
    }
  },
});
