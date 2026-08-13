import { defineRule, s, type Ctx, type DeclInfo } from "arclint";

// The Go-declaration half of DDD Flat, built on ctx.facts (parser-grade
// declaration facts) and ctx.moduleOf (the rules.yaml module aliases,
// never duplicated here). The ShelfBook predecessor of this file carried
// ~110 lines of hand-rolled brace matching and regex function scanning;
// facts replace all of it.

function inModule(ctx: Ctx, path: string, name: string): boolean {
  return ctx.moduleOf(path).includes(name);
}

function goFiles(ctx: Ctx, module: string) {
  return ctx.files("**/*.go").filter((f) => inModule(ctx, f.path, module));
}

const aggregateLocation = defineRule({
  type: "ddd-aggregate-location",
  description: "Configured aggregate roots and their behavior live under domain.",
  contract: "provides",
  blame: "provider",
  capability: "heuristic",
  params: s.object({
    aggregates: s
      .array(s.string())
      .default([])
      .describe("Aggregate root type names; empty disables the rule."),
  }),
  check(ctx, params) {
    const aggregates = new Set(params.aggregates as string[]);
    if (aggregates.size === 0) return;
    for (const f of ctx.files("**/*.go")) {
      if (inModule(ctx, f.path, "domain")) continue;
      const facts = ctx.facts(f.path);
      if (!facts) continue;
      for (const d of facts.decls) {
        if (d.kind !== "struct" || !aggregates.has(d.name)) continue;
        ctx.report({
          path: f.path,
          line: d.startLine,
          message: `aggregate root ${d.name} is declared outside the domain layer`,
          fixHint: "move the aggregate and its behavior under domain/ or internal/domain/",
        });
      }
    }
  },
});

const repositoryPlacement = defineRule({
  type: "ddd-repository-placement",
  description: "Repository interfaces live in domain; implementations live in infrastructure.",
  contract: "provides",
  blame: "provider",
  capability: "heuristic",
  params: s.object({
    suffixes: s
      .array(s.string())
      .default(["Repository", "Repo"])
      .describe("Type-name suffixes that mark repository types."),
  }),
  check(ctx, params) {
    const suffixes = params.suffixes as string[];
    const marks = (name: string) => suffixes.some((x) => name.endsWith(x));
    for (const f of ctx.files("**/*.go")) {
      const facts = ctx.facts(f.path);
      if (!facts) continue;
      for (const d of facts.decls) {
        if (!marks(d.name)) continue;
        if (d.kind === "interface" && !inModule(ctx, f.path, "domain")) {
          ctx.report({
            path: f.path,
            line: d.startLine,
            message: `repository interface ${d.name} is outside the domain layer`,
            fixHint: "put the interface next to the aggregate that it manages",
          });
        }
        if (
          d.kind === "struct" &&
          !inModule(ctx, f.path, "infrastructure") &&
          !inModule(ctx, f.path, "adapters")
        ) {
          ctx.report({
            path: f.path,
            line: d.startLine,
            message: `repository implementation ${d.name} is outside infrastructure`,
            fixHint: "move the concrete repository into infrastructure/ or an adapter package",
          });
        }
      }
    }
  },
});

const handlerBoundary = defineRule({
  type: "ddd-handler-boundary",
  description: "Handlers map requests and call use cases; they hold no business logic.",
  contract: "invariant",
  blame: "provider",
  capability: "heuristic",
  params: s.object({
    maxBranches: s
      .integer()
      .default(3)
      .describe("If-statements tolerated per handler; any loop, switch, or state mutation flags immediately."),
    handlerNames: s
      .array(s.string())
      .default(["ServeHTTP", "Handle", "Consume", "Run", "Execute"])
      .describe("Function-name prefixes treated as inbound handlers."),
  }),
  check(ctx, params) {
    const maxBranches = params.maxBranches as number;
    const prefixes = params.handlerNames as string[];
    for (const module of ["delivery", "adapters"]) {
      for (const f of goFiles(ctx, module)) {
        const facts = ctx.facts(f.path);
        if (!facts) continue;
        const lines = ctx.read(f.path).split("\n");
        for (const d of facts.decls) {
          if (d.kind !== "func" && d.kind !== "method") continue;
          if (!prefixes.some((p) => d.name.startsWith(p))) continue;
          const body = lines.slice(d.startLine - 1, d.endLine).join("\n");
          const branches = body.match(/\bif\b/g)?.length ?? 0;
          const hasControlFlow = /\b(?:for|switch|select)\b/.test(body);
          const changesState = /\b[A-Za-z_]\w*\.[A-Za-z_]\w*\s*(?:=(?!=)|\+=|-=|\*=|\/=)|\+\+|--/.test(body);
          if (!hasControlFlow && !changesState && branches <= maxBranches) continue;
          ctx.report({
            path: f.path,
            line: d.startLine,
            message: `handler ${d.name} contains business-logic signals`,
            fixHint: "move loops, decisions, and state changes into a use case or an aggregate",
          });
        }
      }
    }
  },
});

const applicationThinness = defineRule({
  type: "ddd-application-thinness",
  description: "Use-case functions stay thin orchestration.",
  contract: "invariant",
  blame: "provider",
  capability: "structural",
  params: s.object({
    maxFunctionLines: s
      .integer()
      .default(40)
      .describe("Line span tolerated per application function."),
  }),
  check(ctx, params) {
    const maximum = params.maxFunctionLines as number;
    for (const f of goFiles(ctx, "application")) {
      const facts = ctx.facts(f.path);
      if (!facts) continue;
      for (const d of facts.decls) {
        if (d.kind !== "func" && d.kind !== "method") continue;
        const span = d.endLine - d.startLine + 1;
        if (span > maximum) {
          ctx.report({
            path: f.path,
            line: d.startLine,
            message: `application function ${d.name} spans ${span} lines; the soft limit is ${maximum}`,
            fixHint: "move core rules into an aggregate or a domain service",
          });
        }
      }
    }
  },
});

const packageNaming = defineRule({
  type: "ddd-package-naming",
  description: "Go code lives inside a declared DDD Flat layer.",
  contract: "invariant",
  blame: "provider",
  capability: "structural",
  check(ctx) {
    const reported = new Set<string>();
    for (const f of ctx.files("**/*.go")) {
      if (f.path.endsWith("_test.go")) continue;
      if (ctx.moduleOf(f.path).length > 0) continue;
      const top = f.dir === "" ? "(repository root)" : f.dir.split("/").slice(0, 2).join("/");
      if (reported.has(top)) continue;
      reported.add(top);
      ctx.report({
        path: f.path,
        message: `${top} holds Go code outside every DDD Flat layer`,
        fixHint: "move it under domain, application, infrastructure, delivery, or an allowed alias",
      });
    }
  },
});

const valueObjectEncapsulation = defineRule({
  type: "ddd-value-object-encapsulation",
  description: "Value objects hide mutable fields and use constructors.",
  contract: "invariant",
  blame: "provider",
  capability: "heuristic",
  params: s.object({
    names: s.array(s.string()).default([]).describe("Known value object type names."),
    suffixes: s
      .array(s.string())
      .default(["ID", "Value", "Money", "Date", "Status"])
      .describe("Type-name suffixes that mark value objects."),
    enforceConstructor: s.boolean().default(true),
  }),
  check(ctx, params) {
    const names = new Set(params.names as string[]);
    const suffixes = params.suffixes as string[];
    const requireConstructor = params.enforceConstructor as boolean;
    const isValueObject = (name: string) =>
      names.has(name) || suffixes.some((x) => name.endsWith(x));

    // Constructors may live in any domain file of the same package.
    const constructors = new Set<string>();
    const domainFacts: { path: string; decls: DeclInfo[] }[] = [];
    for (const f of goFiles(ctx, "domain")) {
      const facts = ctx.facts(f.path);
      if (!facts) continue;
      domainFacts.push({ path: f.path, decls: facts.decls });
      for (const d of facts.decls) {
        if (d.kind === "func" && d.name.startsWith("New")) constructors.add(d.name);
      }
    }
    for (const file of domainFacts) {
      for (const d of file.decls) {
        if (d.kind !== "struct" || !isValueObject(d.name)) continue;
        for (const field of file.decls) {
          if (field.kind === "field" && field.owner === d.name && field.exported) {
            ctx.report({
              path: file.path,
              line: field.startLine,
              message: `value object ${d.name} exposes mutable field ${field.name}`,
              fixHint: "make the field private and expose behavior that returns a new value",
            });
          }
        }
        if (requireConstructor && !constructors.has(`New${d.name}`)) {
          ctx.report({
            path: file.path,
            line: d.startLine,
            message: `value object ${d.name} has no New${d.name} constructor`,
            fixHint: "add a constructor that validates the value before creation",
          });
        }
      }
    }
  },
});

const technologyIsolation = defineRule({
  type: "ddd-technology-isolation",
  description: "Domain and application code never import technology packages.",
  contract: "consumes",
  blame: "consumer",
  capability: "exact",
  params: s.object({
    bannedStdlib: s
      .array(s.string())
      .default(["database/sql", "net/http", "net/rpc", "html/template", "os/exec", "plugin", "syscall"])
      .describe("Stdlib packages that are technology, not domain language."),
  }),
  check(ctx, params) {
    const banned = new Set(params.bannedStdlib as string[]);
    for (const module of ["domain", "application"]) {
      for (const f of goFiles(ctx, module)) {
        for (const imp of ctx.imports(f.path)) {
          if (imp.class !== "stdlib" || !banned.has(imp.path)) continue;
          ctx.report({
            path: f.path,
            line: imp.line,
            message: `${module} imports technology package ${imp.path}`,
            fixHint: "put the technology behind an interface and move its adapter to infrastructure or delivery",
          });
        }
      }
    }
  },
});

const testLocation = defineRule({
  type: "ddd-test-location",
  description: "Integration and e2e tests live under test/, tests/, or e2e/.",
  contract: "invariant",
  blame: "provider",
  capability: "structural",
  check(ctx) {
    for (const f of ctx.files("**/*_test.go")) {
      const integration = f.stem.endsWith("_integration_test") || f.stem.endsWith("_e2e_test");
      if (!integration) continue;
      const top = f.path.split("/")[0];
      if (top === "test" || top === "tests" || top === "e2e") continue;
      ctx.report({
        path: f.path,
        message: "integration test is outside a top-level test, tests, or e2e directory",
        fixHint: "move the test to test/, tests/, or e2e/",
        severity: "info",
      });
    }
  },
});

export default [
  aggregateLocation,
  repositoryPlacement,
  handlerBoundary,
  applicationThinness,
  packageNaming,
  valueObjectEncapsulation,
  technologyIsolation,
  testLocation,
];
