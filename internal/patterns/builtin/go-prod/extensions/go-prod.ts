import { defineRule, s } from "arclint";

// Rule 1 of the go-prod pack: interface size, built on ctx.facts. The Go
// facts tier emits one "interface" decl per declaration plus one "method"
// decl per direct named member (owner = the interface name), and emits
// nothing for embedded interfaces. The count below is therefore exact
// over direct named methods and blind to embedding, which is why the
// capability claim is structural rather than exact: an interface that
// embeds three fat interfaces passes. Widening that requires an
// embedding fact in internal/lang, not a workaround here.
const interfaceSize = defineRule({
  type: "go-interface-size",
  description:
    "Go interfaces carry at most maxMembers direct named methods. Embedded interfaces are not counted; _test.go files are exempt.",
  contract: "invariant",
  blame: "provider",
  capability: "structural",
  params: s.object({
    maxMembers: s
      .integer()
      .default(3)
      .describe("Largest allowed number of direct named methods."),
    files: s
      .string()
      .default("**/*.go")
      .describe("Doublestar glob selecting the files checked."),
  }),
  check(ctx, params) {
    const max = params.maxMembers as number;
    for (const f of ctx.files(params.files as string)) {
      if (f.name.endsWith("_test.go")) continue;
      const facts = ctx.facts(f.path);
      if (!facts || facts.parseError) continue;
      // Interface decls precede their method decls in file order, so one
      // pass suffices; a receiver type can never share an interface's
      // name inside one package, so owner collisions cannot occur.
      const line = new Map<string, number>();
      const count = new Map<string, number>();
      for (const d of facts.decls) {
        if (d.kind === "interface") {
          line.set(d.name, d.startLine);
          if (!count.has(d.name)) count.set(d.name, 0);
        } else if (d.kind === "method" && count.has(d.owner)) {
          count.set(d.owner, (count.get(d.owner) ?? 0) + 1);
        }
      }
      for (const [name, n] of count) {
        if (n <= max) continue;
        ctx.report({
          path: f.path,
          line: line.get(name) ?? 0,
          message: `interface ${name} has ${n} direct methods; the maximum is ${max}`,
          fixHint:
            "split the interface into consumer-specific roles and define each where it is consumed",
        });
      }
    }
  },
});

export default [interfaceSize];
