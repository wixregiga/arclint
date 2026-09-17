import { defineRule, type Ctx } from "arclint";

// Layer direction is native. These checks supply the file and slice
// relationships native structure/independent constraints cannot express.
// Every read stays inside the web_source Zone selected by the Rule.
const sourceRoot = "web/src/";
const layers = ["app", "pages", "widgets", "features", "entities", "shared"];
const slicedLayers = ["pages", "widgets", "features", "entities"];
const sliceSegments = ["ui", "api", "model", "lib", "config"];

function sourceParts(path: string): string[] {
  return path.startsWith(sourceRoot) ? path.slice(sourceRoot.length).split("/") : [];
}

function sliceOf(path: string): string {
  const parts = sourceParts(path);
  return slicedLayers.includes(parts[0]) && parts.length >= 3
    ? `${sourceRoot}${parts[0]}/${parts[1]}`
    : "";
}

// Shared UI and libraries expose individual modules, avoiding a barrel
// that pulls every component or library into one public interface.
function interfaceOf(path: string): { owner: string; entry: string } | undefined {
  const parts = sourceParts(path);
  const slice = sliceOf(path);
  if (slice) return { owner: slice, entry: `${slice}/index.ts` };
  if (parts[0] !== "shared" || parts.length < 3) return undefined;
  if (parts[1] === "ui" || parts[1] === "lib") {
    const owner = `${sourceRoot}${parts.slice(0, 3).join("/")}`;
    return { owner, entry: parts.length === 3 ? path : `${owner}/index.ts` };
  }
  const owner = `${sourceRoot}${parts.slice(0, 2).join("/")}`;
  return { owner, entry: `${owner}/index.ts` };
}

function importsOf(ctx: Ctx, visit: (file: string, target: string, line: number) => void) {
  for (const file of ctx.files()) {
    for (const imported of ctx.imports(file.path)) {
      // A resolved source filename is observation evidence. Do not guess
      // aliases or read unselected targets to try to resolve an import.
      if (imported.class === "internal" && imported.targetFile.startsWith(sourceRoot)) {
        visit(file.path, imported.targetFile, imported.line);
      }
    }
  }
}

export default [
  defineRule({
    type: "web-source-layout",
    description: "Web source belongs to an FSD layer and code lives in purposeful segments",
    capability: "structural",
    check(ctx) {
      for (const file of ctx.files()) {
        const parts = sourceParts(file.path);
        if (!layers.includes(parts[0])) {
          ctx.report({ path: file.path, message: "Web source must belong to app, pages, widgets, features, entities, or shared" });
          continue;
        }
        if (!/\.tsx?$/.test(file.path)) continue;
        if (slicedLayers.includes(parts[0])) {
          if (parts.length === 3 && parts[2] === "index.ts") continue;
          if (parts.length >= 4 && sliceSegments.includes(parts[2])) continue;
          ctx.report({ path: file.path, message: "A Web slice exposes index.ts and keeps implementation in ui, api, model, lib, or config" });
        } else if (parts.length < 3) {
          ctx.report({ path: file.path, message: "App and Shared code belongs to a purpose-named segment" });
        }
      }
    },
  }),
  defineRule({
    type: "web-slice-isolation",
    description: "Web slices on the same FSD layer cannot import each other",
    capability: "structural",
    check(ctx) {
      importsOf(ctx, (file, target, line) => {
        const from = sliceOf(file);
        const to = sliceOf(target);
        if (from && to && from !== to && sourceParts(file)[0] === sourceParts(target)[0]) {
          ctx.report({ path: file, line, message: `Web slice ${from} cannot import sibling slice ${to}` });
        }
      });
    },
  }),
  defineRule({
    type: "web-public-interfaces",
    description: "Web slices and Shared modules expose explicit public entry files",
    capability: "structural",
    check(ctx) {
      const files = ctx.files();
      const paths = new Set(files.map((file) => file.path));
      const checked = new Set<string>();
      for (const file of files) {
        if (!/\.tsx?$/.test(file.path)) continue;
        const boundary = interfaceOf(file.path);
        if (!boundary || checked.has(boundary.owner)) continue;
        checked.add(boundary.owner);
        if (!paths.has(boundary.entry)) {
          // Anchor at an existing selected file; the missing entry is not
          // a selected subject and must not be read or reported as one.
          ctx.report({ path: file.path, message: `Web module ${boundary.owner} must expose ${boundary.entry}` });
        }
      }
      importsOf(ctx, (file, target, line) => {
        const from = interfaceOf(file);
        const to = interfaceOf(target);
        if (!to) return;
        if (from?.owner === to.owner) {
          if (file !== to.entry && target === to.entry) {
            ctx.report({ path: file, line, message: `Web implementation cannot import its own public entry ${to.entry}` });
          }
        } else if (target !== to.entry) {
          ctx.report({ path: file, line, message: `Import Web module ${to.owner} through ${to.entry}` });
        }
      });
    },
  }),
];
