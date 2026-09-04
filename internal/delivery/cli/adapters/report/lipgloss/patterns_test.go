package lipgloss

import (
	"bytes"
	"strings"
	"testing"

	"github.com/wixregiga/arclint/internal/application"
	"github.com/wixregiga/arclint/internal/delivery/cli"
	"github.com/wixregiga/arclint/internal/domain/distribution"
)

// TestLipglossPatternsAlignsColumnsUnderStyling pins that padding is
// computed on raw text: the stripped output lines up exactly like the
// plain renderer's.
func TestLipglossPatternsAlignsColumnsUnderStyling(t *testing.T) {
	var buf bytes.Buffer
	err := ansiRenderer().Render(&buf, cli.PatternsReport{Patterns: []application.PatternSummary{
		{
			Namespace: "arclint", Name: "domain-model", Version: "0.1.0", Source: distribution.SourceEmbedded,
			Digest: "sha256:3f2a9c1e5b7d0000", Rules: 3, Extensions: 3, Coverage: []string{"go", "ts"},
		},
		{
			Namespace: "acme", Name: "layers", Version: "1.2.0", Source: distribution.SourceLocal, Vendored: true,
			Digest: "sha256:9b1c2d3e4f5a6666", Rules: 12, Extensions: 0, Coverage: []string{"go"},
		},
	}})
	if err != nil {
		t.Fatal(err)
	}
	raw := buf.String()
	if !strings.Contains(raw, "\x1b[1marclint/domain-model@0.1.0\x1b[0m") {
		t.Fatalf("reference is not bold: %q", raw)
	}
	want := "arclint/domain-model@0.1.0  embedded   3 rule(s)  3 extension(s)  coverage [go, ts]  3f2a9c1e5b7d\n" +
		"acme/layers@1.2.0           vendored  12 rule(s)  0 extension(s)  coverage [go]  9b1c2d3e4f5a\n"
	if out := stripANSI(raw); out != want {
		t.Fatalf("stripped = %q, want %q", out, want)
	}
}

func TestLipglossPatternInstallPreservesGrammar(t *testing.T) {
	var buf bytes.Buffer
	err := ansiRenderer().Render(&buf, cli.PatternInstallReport{Result: application.InstallPatternResult{
		Reference: "acme/layers@1.2.0", Digest: "sha256:9b1c2d3e4f5a6666", Source: distribution.SourceRegistry,
		VendoredPath: ".arclint/patterns/acme/layers",
		RulesetPath:  "rules.arclint.yaml",
		Bound:        []application.BoundModule{{Module: "domain", Paths: []string{"internal/domain/**"}}},
		Unbound:      []string{"app"},
	}})
	if err != nil {
		t.Fatal(err)
	}
	want := "installed acme/layers@1.2.0 (registry, 9b1c2d3e4f5a)\n" +
		"vendored to .arclint/patterns/acme/layers\n" +
		"extended rules.arclint.yaml\n" +
		"bound:\n" +
		"  domain: internal/domain/**\n" +
		"unbound (bind each under extends[].bind before the ruleset loads):\n" +
		"  app\n" +
		"next: bind the unbound modules, then run `arclint check .`\n"
	if out := stripANSI(buf.String()); out != want {
		t.Fatalf("stripped = %q, want %q", out, want)
	}

	buf.Reset()
	err = ansiRenderer().Render(&buf, cli.PatternVendorReport{Result: application.VendorPatternResult{
		Reference: "acme/layers@1.2.0", Digest: "sha256:9b1c2d3e4f5a6666", Source: distribution.SourceEmbedded, Path: ".arclint/patterns/acme/layers",
	}})
	if err != nil {
		t.Fatal(err)
	}
	if out := stripANSI(buf.String()); !strings.HasPrefix(out, "vendored acme/layers@1.2.0 (embedded, 9b1c2d3e4f5a) to .arclint/patterns/acme/layers\n") {
		t.Fatalf("stripped = %q", out)
	}

	buf.Reset()
	err = ansiRenderer().Render(&buf, cli.PatternExportReport{Result: application.ExportPatternResult{
		Reference: "acme/layers@1.2.0", Digest: "sha256:9b1c2d3e4f5a6666", VersionDir: "registry/acme/layers/1.2.0", IndexPath: "registry/index.json",
	}})
	if err != nil {
		t.Fatal(err)
	}
	if out := stripANSI(buf.String()); out != "published acme/layers@1.2.0 (9b1c2d3e4f5a) to registry/acme/layers/1.2.0\nupdated registry/index.json\n" {
		t.Fatalf("stripped = %q", out)
	}
}
