import { defineRule, s, type FileInfo, type ImportInfo } from "arclint";

// Open-set enforcement of the feature-slice dependency matrix, with NO
// enumerated feature or concept lists. Identity comes from shape:
//
//   internal/app, internal/shared, cmd     fixed places
//   internal/<d> with <d>/command.go       FEATURE
//   any other internal/<d>                 CONCEPT
//
// Enforced here (everything YAML cannot scope without named modules):
//   - feature -> feature imports forbidden
//   - concept -> feature and shared -> feature imports forbidden
//     (dependencies point inward, never toward features)
//   - features and concepts must not import third-party libraries
//   - concepts must not import the banned stdlib technologies
//   - every concept owns repo.go (its port)            [provides/provider]
//   - loose .go files directly under internal/, and stray .go trees
//     outside internal/ and cmd/, are drift            [invariant/provider]
//   - use-case files stay thin                         [invariant, warn]
//
// NOT enforced here, to avoid duplicate findings: imports of shared/ and
// app/ are policed open-set by the YAML `protected` rules.

export default defineRule({
  type: "feature-slice",
  description:
    "Open-set feature/concept dependency matrix: purity, ports, drift, thin use cases.",
  contract: "consumes",
  blame: "consumer",
  params: s.object({
    maxUseCaseLines: s
      .integer()
      .default(200)
      .describe("Line cap for a feature's use-case files (warn above it)."),
    bannedConceptImports: s
      .array(s.string())
      .default(["net/http", "database/sql", "net/rpc", "html/template"])
      .describe("Stdlib packages concepts may never import (framework, DB, HTTP)."),
  }),
  check(ctx, params) {
    const maxLines = params.maxUseCaseLines as number;
    const bannedStdlib = new Set(params.bannedConceptImports as string[]);
    const files = ctx.files();

    // --- classify internal/<dir> by shape ------------------------------
    const dirs = new Set<string>();
    const hasCommand = new Set<string>();
    const hasRepo = new Set<string>();
    for (const f of files) {
      const parts = f.path.split("/");
      if (parts[0] !== "internal" || parts.length < 3) continue;
      dirs.add(parts[1]);
      if (parts.length === 3 && parts[2] === "command.go") hasCommand.add(parts[1]);
      if (parts.length === 3 && parts[2] === "repo.go") hasRepo.add(parts[1]);
    }
    const kind = (d: string): "app" | "shared" | "feature" | "concept" =>
      d === "app" ? "app" : d === "shared" ? "shared" : hasCommand.has(d) ? "feature" : "concept";

    const targetTop = (imp: ImportInfo): string | null => {
      const t = imp.targetDir;
      if (!t.startsWith("internal/")) return null;
      return t.split("/")[1] ?? null;
    };

    // --- dependency matrix + import hygiene ----------------------------
    for (const f of files) {
      const parts = f.path.split("/");
      const inInternal = parts[0] === "internal" && parts.length >= 3;

      // Drift: loose .go directly under internal/, or stray .go outside
      // the declared places. These are shape violations of the tree
      // itself, not bad imports: invariant, blamed on the module.
      if (parts[0] === "internal" && parts.length === 2 && f.path.endsWith(".go")) {
        ctx.report({
          path: f.path,
          message:
            "loose Go file directly under internal/ fits no place (feature, concept, shared, app) — drift",
          fixHint: "move it into a feature or concept package",
          contract: "invariant",
          blame: "provider",
        });
        continue;
      }
      if (parts[0] !== "internal" && parts[0] !== "cmd" && f.path.endsWith(".go")) {
        ctx.report({
          path: f.path,
          message: `${parts.length > 1 ? parts[0] + "/" : "(repository root)"} holds Go code outside the declared places (cmd, internal) — drift`,
          fixHint: "relocate under internal/ as a feature or concept, or under cmd/",
          contract: "invariant",
          blame: "provider",
        });
        continue;
      }
      if (!inInternal || !f.path.endsWith(".go")) continue;

      const from = parts[1];
      const fromKind = kind(from);
      for (const imp of ctx.imports(f.path)) {
        // Third-party ban for the open sets (YAML cannot scope this
        // without named modules).
        if ((fromKind === "feature" || fromKind === "concept") && imp.class === "external") {
          ctx.report({
            path: f.path,
            line: imp.line,
            message: `${fromKind} "${from}" imports third-party "${imp.path}"; features and concepts stay dependency-free`,
            fixHint: "move the technology behind a shared adapter and a concept port",
          });
          continue;
        }
        if (fromKind === "concept" && imp.class === "stdlib" && bannedStdlib.has(imp.path)) {
          ctx.report({
            path: f.path,
            line: imp.line,
            message: `concept "${from}" imports "${imp.path}"; concepts are domain-pure: no framework, DB, or HTTP`,
            fixHint: "move the technology concern into shared/ behind the concept's port",
          });
          continue;
        }
        if (imp.class !== "internal") continue;
        const to = targetTop(imp);
        if (to === null || to === from) continue;
        const toKind = kind(to);
        // shared/app targets are policed by the YAML protected rules;
        // reporting them here would duplicate findings.
        if (toKind === "shared" || toKind === "app") continue;
        if (fromKind === "feature" && toKind === "feature") {
          ctx.report({
            path: f.path,
            line: imp.line,
            message: `feature "${from}" imports feature "${to}"; features must not import other features`,
            fixHint: "extract the shared rule into a concept package both features can use",
          });
        } else if ((fromKind === "concept" || fromKind === "shared") && toKind === "feature") {
          ctx.report({
            path: f.path,
            line: imp.line,
            message: `${fromKind} "${from}" imports feature "${to}"; dependencies point inward, never toward features`,
            fixHint: "invert the dependency through a concept port",
          });
        }
      }
    }

    // --- concepts own their ports: a promise the concept failed --------
    for (const d of Array.from(dirs).sort()) {
      if (kind(d) !== "concept" || hasRepo.has(d)) continue;
      const first = files.find((f: FileInfo) => f.path.startsWith(`internal/${d}/`));
      if (!first) continue;
      ctx.report({
        path: first.path,
        message: `internal/${d}/ has no command.go (so it is not a feature) and no repo.go port (so it is not a well-formed concept) — drift or a concept missing its port`,
        fixHint: `add internal/${d}/repo.go with the port interface, make it a feature with command.go, or remove it`,
        contract: "provides",
        blame: "provider",
      });
    }

    // --- thin use cases (warn) -----------------------------------------
    for (const f of files) {
      const parts = f.path.split("/");
      if (parts[0] !== "internal" || parts.length !== 3 || !f.path.endsWith(".go")) continue;
      if (kind(parts[1]) !== "feature") continue;
      const lines = ctx.read(f.path).split("\n").length;
      if (lines > maxLines) {
        ctx.report({
          path: f.path,
          message: `use-case file has ${lines} lines (cap ${maxLines}); use cases are thin orchestration over concepts and ports`,
          fixHint: "push rules into a concept package; keep the use case orchestration-only",
          severity: "warn",
          contract: "invariant",
          blame: "provider",
        });
      }
    }
  },
});
