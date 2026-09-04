package plain

import (
	"bytes"
	"testing"

	"github.com/wixregiga/arclint/internal/application"
	"github.com/wixregiga/arclint/internal/delivery/cli"
	"github.com/wixregiga/arclint/internal/domain/distribution"
)

func samplePatterns() []application.PatternSummary {
	return []application.PatternSummary{
		{
			Namespace: "arclint", Name: "domain-model", Version: "0.1.0", Source: distribution.SourceEmbedded,
			Digest: "sha256:3f2a9c1e5b7d0000", Rules: 3, Extensions: 3, Coverage: []string{"go", "ts"},
		},
		{
			Namespace: "acme", Name: "layers", Version: "1.2.0", Source: distribution.SourceLocal, Authored: true,
			Digest: "sha256:9b1c2d3e4f5a6666", Rules: 12, Extensions: 0, Coverage: []string{"go"},
		},
	}
}

func TestPlainPatternsBytes(t *testing.T) {
	var buf bytes.Buffer
	if err := New().Render(&buf, cli.PatternsReport{Patterns: samplePatterns()}); err != nil {
		t.Fatal(err)
	}
	want := "arclint/domain-model@0.1.0  embedded   3 rule(s)  3 extension(s)  coverage [go, ts]  3f2a9c1e5b7d\n" +
		"acme/layers@1.2.0           authored  12 rule(s)  0 extension(s)  coverage [go]  9b1c2d3e4f5a\n"
	if buf.String() != want {
		t.Fatalf("bytes = %q, want %q", buf.String(), want)
	}

	buf.Reset()
	if err := New().Render(&buf, cli.PatternsReport{Registry: "https://patterns.example.com"}); err != nil {
		t.Fatal(err)
	}
	if want := "registry https://patterns.example.com\nno patterns published\n"; buf.String() != want {
		t.Fatalf("bytes = %q, want %q", buf.String(), want)
	}
}

func TestPlainPatternInstallBytes(t *testing.T) {
	var buf bytes.Buffer
	err := New().Render(&buf, cli.PatternInstallReport{Result: application.InstallPatternResult{
		Reference: "acme/layers@1.2.0", Digest: "sha256:9b1c2d3e4f5a6666", Source: distribution.SourceRegistry,
		VendoredPath: ".arclint/patterns/acme/layers", VendorReplaced: "1.1.0",
		RulesetPath: "rules.arclint.yaml", RulesetReplaced: "1.1.0",
		Bound:   []application.BoundModule{{Module: "domain", Paths: []string{"internal/domain/**", "pkg/domain/**"}}},
		Unbound: []string{"app"},
		Adopted: []string{"domain"},
	}})
	if err != nil {
		t.Fatal(err)
	}
	want := "installed acme/layers@1.2.0 (registry, 9b1c2d3e4f5a)\n" +
		"vendored to .arclint/patterns/acme/layers, replacing 1.1.0\n" +
		"extended rules.arclint.yaml, moving the entry from 1.1.0\n" +
		"bound:\n" +
		"  domain: internal/domain/**, pkg/domain/**\n" +
		"adopted declared module(s): domain\n" +
		"unbound (bind each under extends[].bind before the ruleset loads):\n" +
		"  app\n" +
		"next: bind the unbound modules, then run `arclint check .`\n"
	if buf.String() != want {
		t.Fatalf("bytes = %q, want %q", buf.String(), want)
	}

	buf.Reset()
	err = New().Render(&buf, cli.PatternInstallReport{Result: application.InstallPatternResult{
		Reference: "arclint/vertical@0.1.0", Digest: "sha256:3f2a9c1e5b7d0000", Source: distribution.SourceEmbedded,
		RulesetPath: "rules.arclint.yaml", RulesetCreated: true,
		Bound: []application.BoundModule{{Module: "domain", Paths: []string{"internal/*/domain/**"}}},
	}})
	if err != nil {
		t.Fatal(err)
	}
	want = "installed arclint/vertical@0.1.0 (embedded, 3f2a9c1e5b7d)\n" +
		"wrote rules.arclint.yaml\n" +
		"bound:\n" +
		"  domain: internal/*/domain/**\n" +
		"next: run `arclint check .`\n"
	if buf.String() != want {
		t.Fatalf("bytes = %q, want %q", buf.String(), want)
	}
}

func TestPlainPatternVendorAndExportBytes(t *testing.T) {
	var buf bytes.Buffer
	err := New().Render(&buf, cli.PatternVendorReport{Result: application.VendorPatternResult{
		Reference: "acme/layers@1.2.0", Digest: "sha256:9b1c2d3e4f5a6666", Source: distribution.SourceRegistry,
		Path: ".arclint/patterns/acme/layers", Replaced: "1.1.0",
	}})
	if err != nil {
		t.Fatal(err)
	}
	want := "vendored acme/layers@1.2.0 (registry, 9b1c2d3e4f5a) to .arclint/patterns/acme/layers\n" +
		"replaced 1.1.0\n" +
		"next: commit the directory; every load verifies it against manifest.json\n"
	if buf.String() != want {
		t.Fatalf("bytes = %q, want %q", buf.String(), want)
	}

	buf.Reset()
	err = New().Render(&buf, cli.PatternVendorReport{Result: application.VendorPatternResult{
		Reference: "acme/layers@1.2.0", Digest: "sha256:9b1c2d3e4f5a6666", Source: distribution.SourceLocal, Unchanged: true,
	}})
	if err != nil {
		t.Fatal(err)
	}
	if want := "acme/layers@1.2.0 is already vendored under .arclint/patterns (9b1c2d3e4f5a); nothing written\n"; buf.String() != want {
		t.Fatalf("bytes = %q, want %q", buf.String(), want)
	}

	buf.Reset()
	err = New().Render(&buf, cli.PatternExportReport{Result: application.ExportPatternResult{
		Reference: "acme/layers@1.2.0", Digest: "sha256:9b1c2d3e4f5a6666", VersionDir: "registry/acme/layers/1.2.0", IndexPath: "registry/index.json", Replaced: true,
	}})
	if err != nil {
		t.Fatal(err)
	}
	want = "published acme/layers@1.2.0 (9b1c2d3e4f5a) to registry/acme/layers/1.2.0, replacing the listed version\n" +
		"updated registry/index.json\n"
	if buf.String() != want {
		t.Fatalf("bytes = %q, want %q", buf.String(), want)
	}
}
