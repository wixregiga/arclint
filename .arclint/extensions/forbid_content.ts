// The content-rule capability, delivered as an SDK extension: report
// every line matching a configured pattern. This file doubles as the
// SDK's showcase test case — a complete, real extension in under
// thirty lines.
import { defineRule, s } from "arclint";

export default defineRule({
  type: "forbid-content",
  description: "report lines matching a configured pattern",
  capability: "exact",
  params: s.object({
    pattern: s.string().describe("RegExp source matched against each line"),
  }),
  check(ctx, params) {
    const re = new RegExp(String(params.pattern));
    for (const file of ctx.files()) {
      const lines = ctx.read(file.path).split("\n");
      lines.forEach((line, index) => {
        if (re.test(line)) {
          ctx.report({
            path: file.path,
            line: index + 1,
            message: `forbidden content matching /${params.pattern}/`,
            fixHint: "remove the content or relocate it outside this rule's scope",
          });
        }
      });
    }
  },
});
