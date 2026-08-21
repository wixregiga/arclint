import { defineRule } from "arclint";

function snakeCase(name: string): string {
  return name
    .replace(/([A-Z]+)([A-Z][a-z])/g, "$1_$2")
    .replace(/([a-z0-9])([A-Z])/g, "$1_$2")
    .toLowerCase();
}
function isUseCaseCandidate(name: string, stem: string, params: { type?: string }[] | undefined): boolean {
  if (params) {
    for (const p of params) {
      if ((p.type || "").replace(/^\*/, "").endsWith("Command")) {
        return true;
      }
    }
  }
  return stem.includes("_") && stem === snakeCase(name);
}

function validSignature(name: string, params: { name?: string; type?: string; optional?: boolean; variadic?: boolean }[] | undefined, results: string[] | undefined): boolean {
  if (!params || params.length !== 2) {
    return false;
  }
  if (params[0].optional || params[0].variadic || params[1].optional || params[1].variadic) {
    return false;
  }
  if (params[0].name !== "ctx" || params[0].type !== "context.Context") {
    return false;
  }
  if (params[1].name !== "cmd" || params[1].type !== name + "Command") {
    return false;
  }
  return !!results && results.length === 1 && results[0] === "error";
}

export default defineRule({
  type: "vertical/usecase",
  description: "require use-case functions to take a command and return error",
  capability: "heuristic",
  check(ctx) {
    for (const file of ctx.files()) {
      const facts = ctx.facts(file.path);
      if (!facts) {
        continue;
      }
      for (const d of facts.decls) {
        if (d.kind !== "func" || !d.exported || d.name.startsWith("New")) {
          continue;
        }
        if (!isUseCaseCandidate(d.name, file.stem, d.params)) {
          continue;
        }
        if (!validSignature(d.name, d.params, d.results)) {
          ctx.report({
            path: file.path,
            line: d.startLine,
            message: `Use case "${d.name}" must have signature ${d.name}(ctx context.Context, cmd ${d.name}Command) error`,
          });
        }
        const wantFile = snakeCase(d.name) + ".go";
        if (file.name !== wantFile) {
          ctx.report({
            path: file.path,
            line: d.startLine,
            message: `Use case "${d.name}" must be declared in "${wantFile}"`,
          });
        }
      }
    }
  },
});
