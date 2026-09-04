package plain

import (
	"io"
	"strings"

	"github.com/wixregiga/arclint/internal/application"
	"github.com/wixregiga/arclint/internal/delivery/cli"
	"github.com/wixregiga/arclint/internal/delivery/cli/adapters/report/internal/out"
)

// writePatterns lists one Pattern per line: reference, where it
// resolves from, its size, coverage, and the short digest that names
// exactly the files a vendored copy is verified against.
func writePatterns(w io.Writer, r cli.PatternsReport) error {
	p := &out.Printer{W: w}
	if r.Registry != "" {
		p.Printf("registry %s\n", r.Registry)
	}
	if len(r.Patterns) == 0 {
		if r.Registry != "" {
			p.Println("no patterns published")
		} else {
			p.Println("no patterns available")
		}
		return p.Err
	}
	refs := make([]string, 0, len(r.Patterns))
	labels := make([]string, 0, len(r.Patterns))
	refWidth, labelWidth := 0, 0
	for _, row := range r.Patterns {
		ref := row.Namespace + "/" + row.Name + "@" + row.Version
		label := cli.PatternSourceLabel(row)
		refs = append(refs, ref)
		labels = append(labels, label)
		refWidth = max(refWidth, len(ref))
		labelWidth = max(labelWidth, len(label))
	}
	for i, row := range r.Patterns {
		coverage := ""
		if len(row.Coverage) > 0 {
			coverage = "  coverage [" + strings.Join(row.Coverage, ", ") + "]"
		}
		p.Printf("%-*s  %-*s  %2d rule(s)  %d extension(s)%s  %s\n",
			refWidth, refs[i], labelWidth, labels[i], row.Rules, row.Extensions, coverage, cli.ShortDigest(row.Digest))
	}
	return p.Err
}

func writePatternVendor(w io.Writer, r cli.PatternVendorReport) error {
	p := &out.Printer{W: w}
	res := r.Result
	if res.Unchanged {
		p.Printf("%s is already vendored under .arclint/patterns (%s); nothing written\n", res.Reference, cli.ShortDigest(res.Digest))
		return p.Err
	}
	p.Printf("vendored %s (%s, %s) to %s\n", res.Reference, res.Source, cli.ShortDigest(res.Digest), res.Path)
	if res.Replaced != "" {
		p.Printf("replaced %s\n", res.Replaced)
	}
	p.Println("next: commit the directory; every load verifies it against manifest.json")
	return p.Err
}

func writePatternInstall(w io.Writer, r cli.PatternInstallReport) error {
	p := &out.Printer{W: w}
	res := r.Result
	p.Printf("installed %s (%s, %s)\n", res.Reference, res.Source, cli.ShortDigest(res.Digest))
	if res.VendoredPath != "" {
		if res.VendorReplaced != "" {
			p.Printf("vendored to %s, replacing %s\n", res.VendoredPath, res.VendorReplaced)
		} else {
			p.Printf("vendored to %s\n", res.VendoredPath)
		}
	}
	switch {
	case res.RulesetCreated:
		p.Printf("wrote %s\n", res.RulesetPath)
	case res.RulesetReplaced != "":
		p.Printf("extended %s, moving the entry from %s\n", res.RulesetPath, res.RulesetReplaced)
	default:
		p.Printf("extended %s\n", res.RulesetPath)
	}
	writeBindings(p, res)
	return p.Err
}

// writeBindings spells what the extends entry binds and what the
// owner still has to bind before the ruleset loads.
func writeBindings(p *out.Printer, res application.InstallPatternResult) {
	if len(res.Bound) > 0 {
		p.Println("bound:")
		for _, b := range res.Bound {
			p.Printf("  %s: %s\n", b.Module, strings.Join(b.Paths, ", "))
		}
	}
	if len(res.Adopted) > 0 {
		p.Printf("adopted declared module(s): %s\n", strings.Join(res.Adopted, ", "))
	}
	if len(res.Unbound) > 0 {
		p.Println("unbound (bind each under extends[].bind before the ruleset loads):")
		for _, m := range res.Unbound {
			p.Printf("  %s\n", m)
		}
		p.Println("next: bind the unbound modules, then run `arclint check .`")
		return
	}
	p.Println("next: run `arclint check .`")
}

func writePatternExport(w io.Writer, r cli.PatternExportReport) error {
	p := &out.Printer{W: w}
	res := r.Result
	if res.Replaced {
		p.Printf("published %s (%s) to %s, replacing the listed version\n", res.Reference, cli.ShortDigest(res.Digest), res.VersionDir)
	} else {
		p.Printf("published %s (%s) to %s\n", res.Reference, cli.ShortDigest(res.Digest), res.VersionDir)
	}
	p.Printf("updated %s\n", res.IndexPath)
	return p.Err
}
