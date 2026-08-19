package sobekextension_test

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/wixregiga/arclint/internal/domain/conformance"
	sobekextension "github.com/wixregiga/arclint/internal/infrastructure/extension/sobek"
)

// writeExtensions materializes files under <root>/.arclint/extensions.
func writeExtensions(t *testing.T, files map[string]string) string {
	t.Helper()
	root := t.TempDir()
	dir := filepath.Join(root, ".arclint", "extensions")
	if err := os.MkdirAll(dir, 0o755); err != nil {
		t.Fatal(err)
	}
	for name, content := range files {
		if err := os.WriteFile(filepath.Join(dir, name), []byte(content), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	return root
}

// fakeHost lends an in-memory tree; the Host struct is the real seam the
// engine injects, so this exercises the true extension-side contract.
func fakeHost(files map[string]string) sobekextension.Host {
	var infos []sobekextension.FileInfo
	var paths []string
	for p := range files {
		paths = append(paths, p)
	}
	// Deterministic order.
	for i := 0; i < len(paths); i++ {
		for j := i + 1; j < len(paths); j++ {
			if paths[j] < paths[i] {
				paths[i], paths[j] = paths[j], paths[i]
			}
		}
	}
	for _, p := range paths {
		base := p[strings.LastIndex(p, "/")+1:]
		e := ""
		if idx := strings.LastIndex(base, "."); idx >= 0 {
			e = base[idx:]
		}
		infos = append(infos, sobekextension.FileInfo{
			Path: p, Name: base, Stem: strings.TrimSuffix(base, e), Ext: e,
			Dir: strings.TrimSuffix(strings.TrimSuffix(p, base), "/"), Size: len(files[p]),
		})
	}
	return sobekextension.Host{
		Files: func(glob string) ([]sobekextension.FileInfo, error) { return infos, nil },
		Read: func(path string) (string, error) {
			content, ok := files[path]
			if !ok {
				return "", os.ErrNotExist
			}
			return content, nil
		},
		Imports: func(path string) []sobekextension.ImportInfo { return nil },
		Modules: func() map[string][]string { return map[string][]string{} },
	}
}

const namingRule = `
import { defineRule, s } from "arclint";

export default defineRule({
  type: "stem-suffix",
  params: s.object({ suffix: s.string().default("Handler") }),
  check(ctx, params) {
    for (const f of ctx.files()) {
      if (!f.stem.endsWith(params.suffix as string)) {
        ctx.report({ path: f.path, message: "bad name: " + f.name, fixHint: "rename it" });
      }
    }
  },
});
`

func TestLoadAndCheckWithDefaults(t *testing.T) {
	root := writeExtensions(t, map[string]string{"naming.ts": namingRule})
	reg, err := sobekextension.LoadDir(root, sobekextension.Options{})
	if err != nil {
		t.Fatal(err)
	}
	rt := reg.Get("stem-suffix")
	if rt == nil {
		t.Fatal("rule type not registered")
	}
	// Empty params: the schema default must be applied host-side.
	params, err := rt.ValidateParams(nil)
	if err != nil {
		t.Fatal(err)
	}
	if params["suffix"] != "Handler" {
		t.Errorf("default not applied: %v", params)
	}
	host := fakeHost(map[string]string{
		"a/UserHandler.go": "package a",
		"a/broken.go":      "package a",
	})
	got, err := rt.Check(host, params)
	if err != nil {
		t.Fatal(err)
	}
	if len(got) != 1 || got[0].Path != "a/broken.go" || got[0].FixHint == "" {
		t.Fatalf("violations: %+v", got)
	}
}

func TestParamsValidationRejectsBadTypes(t *testing.T) {
	root := writeExtensions(t, map[string]string{"naming.ts": namingRule})
	reg, err := sobekextension.LoadDir(root, sobekextension.Options{})
	if err != nil {
		t.Fatal(err)
	}
	rt := reg.Get("stem-suffix")
	if _, err := rt.ValidateParams(map[string]any{"suffix": 123}); err == nil {
		t.Error("numeric suffix accepted against string schema")
	}
	if _, err := rt.ValidateParams(map[string]any{"unknown": true}); err == nil {
		t.Error("unknown param accepted against additionalProperties:false")
	}
}

func TestBareImportRejected(t *testing.T) {
	// The binding must be USED, or TypeScript import elision drops the
	// import before resolution ever happens.
	root := writeExtensions(t, map[string]string{"bad.ts": `
import lodash from "lodash";
export default { helper: lodash };
`})
	_, err := sobekextension.LoadDir(root, sobekextension.Options{})
	if err == nil || !strings.Contains(err.Error(), "bare import") {
		t.Fatalf("expected bare-import rejection, got: %v", err)
	}
}

func TestRelativeImportsBundled(t *testing.T) {
	// Discovery is top-level only; shared helpers live in subdirectories
	// and are bundled via relative imports.
	root := writeExtensions(t, map[string]string{"rule.ts": `
import { defineRule } from "arclint";
import { LIMIT } from "./lib/helper";

export default defineRule({
  type: "uses-helper",
  check(ctx) {
    const files = ctx.files();
    if (files.length > LIMIT) {
      ctx.report({ path: files[0].path, message: "over limit " + LIMIT });
    }
  },
});
`})
	lib := filepath.Join(root, ".arclint", "extensions", "lib")
	if err := os.MkdirAll(lib, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(lib, "helper.ts"), []byte("export const LIMIT: number = 1;\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	reg, err := sobekextension.LoadDir(root, sobekextension.Options{})
	if err != nil {
		t.Fatal(err)
	}
	rt := reg.Get("uses-helper")
	if rt == nil {
		t.Fatal("rule not registered")
	}
	params, _ := rt.ValidateParams(nil)
	got, err := rt.Check(fakeHost(map[string]string{"a.go": "package a", "b.go": "package b"}), params)
	if err != nil {
		t.Fatal(err)
	}
	if len(got) != 1 || !strings.Contains(got[0].Message, "over limit 1") {
		t.Fatalf("violations: %+v", got)
	}
}

func TestTwoPhaseGuard(t *testing.T) {
	root := writeExtensions(t, map[string]string{"eager.ts": `
declare const __arclint: { report(v: unknown): void };
__arclint.report({ path: "x", message: "too early" });
export default {};
`})
	_, err := sobekextension.LoadDir(root, sobekextension.Options{})
	if err == nil || !strings.Contains(err.Error(), "registration phase") {
		t.Fatalf("expected registration-phase guard error, got: %v", err)
	}
}

func TestDeterministicClockAndRandom(t *testing.T) {
	src := `
import { defineRule } from "arclint";

export default defineRule({
  type: "det",
  check(ctx) {
    ctx.report({ path: "p", message: Date.now() + ":" + Math.random() + ":" + Math.random() });
  },
});
`
	messages := make([]string, 2)
	for i := 0; i < 2; i++ {
		root := writeExtensions(t, map[string]string{"det.ts": src})
		reg, err := sobekextension.LoadDir(root, sobekextension.Options{})
		if err != nil {
			t.Fatal(err)
		}
		rt := reg.Get("det")
		params, err := rt.ValidateParams(nil)
		if err != nil {
			t.Fatal(err)
		}
		got, err := rt.Check(fakeHost(nil), params)
		if err != nil {
			t.Fatal(err)
		}
		if len(got) != 1 {
			t.Fatalf("violations: %+v", got)
		}
		messages[i] = got[0].Message
	}
	if messages[0] != messages[1] {
		t.Errorf("Date.now/Math.random not deterministic across runs: %q vs %q", messages[0], messages[1])
	}
	if strings.Contains(messages[0], "undefined") {
		t.Errorf("override broke the API: %s", messages[0])
	}
}

func TestInterruptTimeout(t *testing.T) {
	root := writeExtensions(t, map[string]string{"spin.ts": `
import { defineRule } from "arclint";

export default defineRule({
  type: "spin",
  check() {
    for (;;) {}
  },
});
`})
	reg, err := sobekextension.LoadDir(root, sobekextension.Options{CheckTimeout: 200 * time.Millisecond})
	if err != nil {
		t.Fatal(err)
	}
	rt := reg.Get("spin")
	params, _ := rt.ValidateParams(nil)
	start := time.Now()
	_, err = rt.Check(fakeHost(nil), params)
	elapsed := time.Since(start)
	if err == nil || !strings.Contains(err.Error(), "timeout") {
		t.Fatalf("expected timeout error, got: %v", err)
	}
	if elapsed > 3*time.Second {
		t.Errorf("interrupt took %s; the engine hung past the timeout", elapsed)
	}
}

func TestDuplicateTypeRejected(t *testing.T) {
	rule := `
import { defineRule } from "arclint";
export default defineRule({ type: "dup", check() {} });
`
	root := writeExtensions(t, map[string]string{"a.ts": rule, "b.ts": rule})
	_, err := sobekextension.LoadDir(root, sobekextension.Options{})
	if err == nil || !strings.Contains(err.Error(), "already registered") {
		t.Fatalf("expected duplicate-type error, got: %v", err)
	}
}

func TestRealpathDedupAndDtsIgnored(t *testing.T) {
	root := writeExtensions(t, map[string]string{
		"a.ts":         namingRule,
		"arclint.d.ts": "declare module \"arclint\" {}",
	})
	dir := filepath.Join(root, ".arclint", "extensions")
	if err := os.Symlink(filepath.Join(dir, "a.ts"), filepath.Join(dir, "b.ts")); err != nil {
		t.Skipf("symlinks unavailable: %v", err)
	}
	// b.ts resolves to a.ts's real path: without dedup this would be a
	// duplicate-type registration error.
	reg, err := sobekextension.LoadDir(root, sobekextension.Options{})
	if err != nil {
		t.Fatal(err)
	}
	if reg.Get("stem-suffix") == nil || len(reg.Types()) != 1 {
		t.Fatalf("registered types: %d", len(reg.Types()))
	}
}

func TestMissingDirIsEmptyRegistry(t *testing.T) {
	reg, err := sobekextension.LoadDir(t.TempDir(), sobekextension.Options{})
	if err != nil || !reg.Empty() {
		t.Fatalf("empty registry expected, got %v / %v", reg, err)
	}
}

func TestSDKInit(t *testing.T) {
	root := t.TempDir()
	files, err := sobekextension.SDKInit(root)
	if err != nil {
		t.Fatal(err)
	}
	if len(files) != 2 {
		t.Fatalf("files: %v", files)
	}
	dts, err := os.ReadFile(filepath.Join(root, ".arclint", "extensions", "arclint.d.ts"))
	if err != nil {
		t.Fatal(err)
	}
	for _, marker := range []string{`declare module "arclint"`, "defineRule", "FileInfo", "ViolationInput", "export const s"} {
		if !strings.Contains(string(dts), marker) {
			t.Errorf("arclint.d.ts lacks %q", marker)
		}
	}
	if _, err := os.Stat(filepath.Join(root, ".arclint", "extensions", "tsconfig.json")); err != nil {
		t.Error(err)
	}
}

const forbidContentRule = `
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
            message: "forbidden content matching /" + params.pattern + "/",
            fixHint: "remove the content",
          });
        }
      });
    }
  },
});
`

// ctx.read must use Observations content, not the repository root the
// evaluator loads extensions from: Rule Tests supply fixture bytes that
// differ from (or are absent on) disk.
func TestEvaluatorReadUsesObservationContent(t *testing.T) {
	root := writeExtensions(t, map[string]string{"forbid.ts": forbidContentRule})
	// Production file is clean; a root-based read would find no match.
	if err := os.MkdirAll(filepath.Join(root, "m"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(root, "m", "a.go"), []byte("package m\n"), 0o644); err != nil {
		t.Fatal(err)
	}

	eval, err := sobekextension.NewEvaluator(root)
	if err != nil {
		t.Fatalf("NewEvaluator: %v", err)
	}
	obs, err := conformance.NewObservations([]conformance.ObservedFile{
		{Path: "m/a.go", Size: 1},
	}, nil)
	if err != nil {
		t.Fatalf("NewObservations: %v", err)
	}
	// Fixture-shaped content that the production file does not have.
	obs = obs.WithContent(conformance.MapContent(map[string]string{
		"m/a.go": "package m\nfunc f() { panic(\"x\") }\n",
	}))

	findings, err := eval.Evaluate("forbid-content", map[string]any{
		"pattern": `\bpanic\(`,
	}, []string{"m/a.go"}, nil, obs)
	if err != nil {
		t.Fatalf("Evaluate: %v", err)
	}
	if len(findings) != 1 {
		t.Fatalf("findings = %+v, want one from fixture content", findings)
	}
	if findings[0].Path != "m/a.go" || findings[0].Line != 2 {
		t.Errorf("finding = %+v, want m/a.go:2", findings[0])
	}

	// Missing production path still reads fixture content.
	missingRoot := writeExtensions(t, map[string]string{"forbid.ts": forbidContentRule})
	evalMissing, err := sobekextension.NewEvaluator(missingRoot)
	if err != nil {
		t.Fatalf("NewEvaluator missing: %v", err)
	}
	findings, err = evalMissing.Evaluate("forbid-content", map[string]any{
		"pattern": `\bpanic\(`,
	}, []string{"m/a.go"}, nil, obs)
	if err != nil {
		t.Fatalf("Evaluate missing production file: %v", err)
	}
	if len(findings) != 1 {
		t.Fatalf("missing production file: findings = %+v, want fixture-driven match", findings)
	}
}
