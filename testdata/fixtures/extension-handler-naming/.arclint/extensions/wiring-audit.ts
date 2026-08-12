import { defineRule } from "arclint";

// Exercises the per-finding contract/blame override: the rule TYPE defaults
// to provides/provider, but this finding is a consumer-side break and says
// so. The e2e test asserts the override reaches the JSON output.
export default defineRule({
  type: "wiring-audit",
  description: "Demo of per-finding contract and blame overrides.",
  contract: "provides",
  blame: "provider",
  check(ctx) {
    for (const f of ctx.files("internal/**/handlers/broken.go")) {
      ctx.report({
        path: f.path,
        message: "consumer-side break reported by a provides-typed rule",
        fixHint: "remove the offending dependency",
        contract: "consumes",
        blame: "consumer",
      });
    }
  },
});
