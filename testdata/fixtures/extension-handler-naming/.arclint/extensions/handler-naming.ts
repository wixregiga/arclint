import { defineRule, s } from "arclint";

// The design proposal's driving rule 4: a TypeScript rule against the SDK,
// executed without TypeScript tooling on the user's machine.
export default defineRule({
  type: "handler-naming",
  params: s.object({ suffix: s.string().default("Handler") }),
  check(ctx, params) {
    const suffix = params.suffix as string;
    for (const f of ctx.files("internal/**/handlers/*.go")) {
      if (!f.stem.endsWith(suffix)) {
        ctx.report({
          path: f.path,
          message: `handler files must end in ${suffix}`,
          fixHint: `rename ${f.name} to <name>${suffix}${f.ext}`,
        });
      }
    }
  },
});
