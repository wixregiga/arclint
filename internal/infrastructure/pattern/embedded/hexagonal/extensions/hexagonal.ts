import { defineRule, s, type Ctx } from "arclint";

type Language = "go" | "typescript" | "python";

const separators: Record<Language, string> = { go: "/", typescript: "/", python: "." };

function languageOf(path: string): Language | undefined {
  if (path.endsWith(".go")) return "go";
  if (path.endsWith(".ts") || path.endsWith(".tsx")) return "typescript";
  if (path.endsWith(".py")) return "python";
  return undefined;
}

function stripPythonComment(line: string): string {
  let quote: string | null = null;
  for (let i = 0; i < line.length; i++) {
    const c = line[i];
    if (quote !== null) {
      if (c === "\\") {
        i++;
      } else if (c === quote) {
        quote = null;
      }
      continue;
    }
    if (c === "'" || c === '"') {
      quote = c;
    } else if (c === "#") {
      return line.slice(0, i);
    }
  }
  return line;
}

function pythonFromImportedNames(lines: string[], startLine: number, mod: string): string[] {
  let idx = startLine - 1;
  if (idx < 0 || idx >= lines.length) return [];

  let logical = stripPythonComment(lines[idx]);
  idx++;
  while (logical.trimEnd().endsWith("\\") && idx < lines.length) {
    logical = logical.trimEnd().slice(0, -1) + " " + stripPythonComment(lines[idx]).trim();
    idx++;
  }

  const escaped = mod.replace(/[.*+?^${}()|[\]\\]/g, "\\$&");
  const fromRe = new RegExp(`^\\s*from\\s+${escaped}\\s+import\\s+([\\s\\S]+)$`);
  const names: string[] = [];

  for (const stmt of logical.split(";")) {
    const m = fromRe.exec(stmt);
    if (m === null) continue;
    let spec = m[1].trim();
    if (spec.startsWith("(")) {
      while (!spec.includes(")") && idx < lines.length) {
        spec += " " + stripPythonComment(lines[idx]).trim();
        idx++;
      }
      const close = spec.indexOf(")");
      spec = close >= 0 ? spec.slice(1, close) : spec.slice(1);
    }
    for (const part of spec.split(",")) {
      const item = /^([A-Za-z_]\w*)(?:\s+as\s+[A-Za-z_]\w*)?$/.exec(part.trim());
      if (item !== null) {
        names.push(item[1]);
      }
    }
  }
  return names;
}

function matchModule(mod: string, list: string[], sep: string): string | undefined {
  return list.find((m) => mod === m || mod.startsWith(m + sep));
}

function forbidStdlib(
  ctx: Ctx,
  filePath: string,
  language: Language,
  forbidden: string[],
  message: (impPath: string, hit: string) => string,
  allowed: string[] = [],
): void {
  if (forbidden.length === 0) return;
  const sep = separators[language];
  let lines: string[] | undefined;

  for (const imp of ctx.imports(filePath)) {
    if (imp.class !== "stdlib") continue;
    const mod = language === "typescript" && imp.path.startsWith("node:")
      ? imp.path.slice("node:".length)
      : imp.path;
    if (matchModule(mod, allowed, sep) !== undefined) continue;
    const hit = matchModule(mod, forbidden, sep);
    if (hit !== undefined) {
      ctx.report({ path: filePath, line: imp.line, message: message(imp.path, hit) });
      continue;
    }
    if (language === "python" && forbidden.some((m) => m.startsWith(mod + "."))) {
      lines ??= ctx.read(filePath).split("\n");
      for (const name of pythonFromImportedNames(lines, imp.line, mod)) {
        const qualified = `${mod}.${name}`;
        if (matchModule(qualified, allowed, ".") !== undefined) continue;
        const subHit = matchModule(qualified, forbidden, ".");
        if (subHit !== undefined) {
          ctx.report({ path: filePath, line: imp.line, message: message(qualified, subHit) });
        }
      }
    }
  }
}

function checkGoStructTags(ctx: Ctx, filePath: string, tagKey: RegExp): void {
  const facts = ctx.facts(filePath);
  if (facts === null || !facts.declarationsAvailable) return;
  const fields = facts.decls.filter((d) => d.kind === "field");
  if (fields.length === 0) return;

  const lines = ctx.read(filePath).split("\n");
  const seen = new Set<string>();
  const goToken = /\/\/[^\n]*|\/\*[\s\S]*?\*\/|'(?:\\.|[^'\\])*'|`([^`]*)`|"((?:\\.|[^"\\])*)"/g;

  for (const field of fields) {
    const spanKey = `${field.startLine}:${field.endLine}`;
    if (seen.has(spanKey)) continue;
    seen.add(spanKey);

    const span = lines.slice(field.startLine - 1, field.endLine).join("\n");
    goToken.lastIndex = 0;
    let tok: RegExpExecArray | null;
    while ((tok = goToken.exec(span)) !== null) {
      let raw: string | undefined;
      if (tok[1] !== undefined) {
        raw = tok[1];
      } else if (tok[2] !== undefined) {
        raw = tok[2].replace(/\\(.)/g, "$1");
      } else {
        continue;
      }
      const m = tagKey.exec(raw);
      if (m === null) continue;
      const line = field.startLine + span.slice(0, tok.index).split("\n").length - 1;
      ctx.report({
        path: filePath,
        line,
        message: `a ${m[1]} struct tag ties this field to a wire format; map the value to a payload or a row in an adapter`,
      });
    }
  }
}

export default [
  defineRule({
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
              message: `package ${imp.path} is not one of the packages this zone may use: ${packages.join(", ")}`,
            });
          }
        }
      }
    },
  }),
  defineRule({
    type: "hexagonal/forbidden-modules",
    description: "a zone imports none of the listed standard-library modules of its file's language or their submodules, except explicitly allowed submodules",
    capability: "exact",
    params: s.object({
      go: s.array(s.string()).describe("Go standard-library package paths, such as os or net/http."),
      typescript: s.array(s.string()).describe("Node built-in module names, written without the node: prefix, such as fs or child_process."),
      python: s.array(s.string()).describe("Python standard-library modules, dotted as imported, such as subprocess or http.client."),
      allow: s.object({
        go: s.array(s.string()).optional().describe("Allowed Go standard-library submodules under a forbidden prefix, such as net/url under net."),
        typescript: s.array(s.string()).optional().describe("Allowed Node built-in submodules under a forbidden prefix."),
        python: s.array(s.string()).optional().describe("Allowed Python submodules under a forbidden prefix."),
      }).optional().describe("Per-language submodules exempted from a broader forbidden module prefix."),
    }),
    check(ctx, params) {
      const lists: Record<Language, string[]> = {
        go: params.go as string[],
        typescript: params.typescript as string[],
        python: params.python as string[],
      };
      const allow = (params.allow ?? {}) as Partial<Record<Language, string[]>>;
      for (const file of ctx.files()) {
        const language = languageOf(file.path);
        if (language === undefined) continue;
        forbidStdlib(
          ctx,
          file.path,
          language,
          lists[language],
          (impPath, hit) => `${impPath} is a forbidden module (${hit}); declare what the code needs as a port and do the work in an outbound adapter`,
          allow[language] ?? [],
        );
      }
    },
  }),
  defineRule({
    type: "hexagonal/wire-format",
    description: "a zone imports none of the listed standard-library encoders, and no Go struct field tag names a listed wire format",
    capability: "heuristic",
    params: s.object({
      go: s.array(s.string()).describe("Go standard-library encoder packages, such as encoding/json."),
      python: s.array(s.string()).describe("Python standard-library encoder modules, dotted as imported, such as json or pickle."),
      tags: s.array(s.string()).describe("Go struct tag keys that name a wire format, such as json or db."),
    }),
    check(ctx, params) {
      const goEncoders = params.go as string[];
      const pyEncoders = params.python as string[];
      const keys = params.tags as string[];
      const escaped = keys.map((k) => k.replace(/[.*+?^${}()|[\]\\]/g, "\\$&"));
      const tagKey = keys.length === 0 ? undefined : new RegExp(`(?:^|\\s)(${escaped.join("|")}):"`);
      const wireMessage = (impPath: string) => `${impPath} encodes values for the wire; map the value to a payload or a row in an adapter`;

      for (const file of ctx.files()) {
        const language = languageOf(file.path);
        if (language === "go") {
          forbidStdlib(ctx, file.path, "go", goEncoders, wireMessage);
          if (tagKey !== undefined) {
            checkGoStructTags(ctx, file.path, tagKey);
          }
        } else if (language === "python") {
          forbidStdlib(ctx, file.path, "python", pyEncoders, wireMessage);
        }
      }
    },
  }),
];
