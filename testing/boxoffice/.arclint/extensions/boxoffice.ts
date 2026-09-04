// boxoffice's extension rules, one file exporting them all. Content
// fences (no panic, no transport in the domain) are the built-in
// content assertion in rules.arclint.yaml, not an extension: the proving
// ground recorded the re-authoring gap and arclint closed it.
//
// fsd-slice-isolation: slices on the same FSD layer never import
// each other; anything they share lives on a lower layer. The
// structural half of what Steiger's forbidden-imports rule checks,
// covering both runtimes.
import { defineRule, s } from "arclint";

// sliceOfFile names the slice a file belongs to: the first path
// segment under the layer root, only when the file sits inside one.
function sliceOfFile(path: string, root: string): string {
  const prefix = root.endsWith("/") ? root : root + "/";
  if (!path.startsWith(prefix)) return "";
  const rest = path.slice(prefix.length);
  const slash = rest.indexOf("/");
  return slash === -1 ? "" : rest.slice(0, slash);
}

// sliceOfDir names the slice a package directory belongs to; the
// directory itself may be the slice folder.
function sliceOfDir(dir: string, root: string): string {
  const prefix = root.endsWith("/") ? root : root + "/";
  if (!dir.startsWith(prefix)) return "";
  const rest = dir.slice(prefix.length);
  const slash = rest.indexOf("/");
  return slash === -1 ? rest : rest.slice(0, slash);
}

function dirOf(path: string): string {
  const slash = path.lastIndexOf("/");
  return slash === -1 ? "" : path.slice(0, slash);
}

const fsdSliceIsolation = defineRule({
  type: "fsd-slice-isolation",
  description: "Same-layer FSD slices must not import each other.",
  capability: "exact",
  params: s.object({
    layers: s
      .array(s.string())
      .describe("Layer roots holding slices, like internal/features or web/src/pages."),
  }),
  check(ctx, params) {
    const layers = params.layers as string[];
    for (const root of layers) {
      for (const f of ctx.files(root + "/**")) {
        const from = sliceOfFile(f.path, root);
        if (!from) continue;
        for (const imp of ctx.imports(f.path)) {
          if (imp.class !== "internal") continue;
          const targetDir = imp.targetFile ? dirOf(imp.targetFile) : imp.targetDir;
          if (!targetDir) continue;
          const to = sliceOfDir(targetDir, root);
          if (to && to !== from) {
            ctx.report({
              path: f.path,
              line: imp.line,
              message:
                'FSD slice isolation: ' + root + ' slice "' + from +
                '" imports sibling slice "' + to + '"',
              fixHint: "share it from a lower layer instead of a sibling slice",
            });
          }
        }
      }
    }
  },
});

const sliceFiles = defineRule({
  type: "slice-files",
  description: "Every slice under a layer root carries its required files.",
  capability: "structural",
  params: s.object({
    root: s.string().describe("Layer root holding slices, like internal/features."),
    require: s
      .array(s.string())
      .describe("Files each slice must contain; {slice} names the slice folder."),
  }),
  check(ctx, params) {
    const root = String(params.root);
    const prefix = root.endsWith("/") ? root : root + "/";
    const required = params.require as string[];
    const bySlice = new Map<string, string[]>();
    for (const f of ctx.files(prefix + "**")) {
      const rest = f.path.slice(prefix.length);
      const slash = rest.indexOf("/");
      if (slash <= 0) continue;
      const slice = rest.slice(0, slash);
      const files = bySlice.get(slice) ?? [];
      files.push(f.path);
      bySlice.set(slice, files);
    }
    for (const slice of [...bySlice.keys()].sort()) {
      // Findings must anchor at a file that exists inside the rule's
      // scope, so a missing file is reported at the slice's first
      // present file (proving-ground note: native structure rules
      // may anchor at the missing path itself; extensions may not).
      const anchor = (bySlice.get(slice) ?? []).sort()[0];
      for (const pattern of required) {
        const want = prefix + slice + "/" + pattern.replace(/\{slice\}/g, slice);
        if (ctx.files(want).length === 0) {
          ctx.report({
            path: anchor,
            message: `slice "${slice}" under ${root} is missing a required file matching "${want}"`,
            fixHint: "every slice ships complete: add the file or fold the slice away",
          });
        }
      }
    }
  },
});

const aggregateEncapsulation = defineRule({
  type: "aggregate-encapsulation",
  description: "A recorded aggregate's struct exposes no exported fields.",
  capability: "structural",
  params: s.object({
    root: s.string().describe("Layer root holding aggregate slices, like internal/entities."),
  }),
  check(ctx, params) {
    const root = String(params.root);
    const prefix = root.endsWith("/") ? root : root + "/";
    const domain = ctx.domain();
    for (const context of domain.contexts) {
      for (const entity of context.entities) {
        if (!entity.aggregate) continue;
        const slice = ctx.caseTerm(entity.name, "flatcase");
        for (const f of ctx.files(prefix + slice + "/*.go")) {
          if (f.name.endsWith("_test.go")) continue;
          const facts = ctx.facts(f.path);
          if (!facts) continue;
          for (const decl of facts.decls) {
            if (decl.kind === "field" && decl.owner === entity.name && decl.exported) {
              ctx.report({
                path: f.path,
                line: decl.startLine,
                message: `aggregate ${entity.name} exposes exported field ${decl.name}; invariants live behind methods`,
                fixHint: "unexport the field and route changes through the aggregate's methods",
              });
            }
          }
        }
      }
    }
  },
});

export default [fsdSliceIsolation, sliceFiles, aggregateEncapsulation];
