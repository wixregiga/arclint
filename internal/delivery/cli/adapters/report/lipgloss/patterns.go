package lipgloss

import (
	"strings"

	"github.com/wixregiga/arclint/internal/application"
	"github.com/wixregiga/arclint/internal/delivery/cli"
	"github.com/wixregiga/arclint/internal/delivery/cli/adapters/report/internal/out"
)

// writePatterns lists one Pattern per line: reference, where it
// resolves from, its size, coverage, and the short digest that names
// exactly the files a vendored copy is verified against. Columns are
// padded on the raw text so ANSI codes never shift them.
func writePatterns(p *out.Printer, th Theme, r cli.PatternsReport) {
	if r.Registry != "" {
		p.Printf("%s %s\n", th.Bold.Render("registry"), th.Path.Render(r.Registry))
	}
	if len(r.Patterns) == 0 {
		if r.Registry != "" {
			p.Println(th.Muted.Render("no patterns published"))
		} else {
			p.Println(th.Muted.Render("no patterns available"))
		}
		return
	}
	refWidth, labelWidth := 0, 0
	for _, row := range r.Patterns {
		refWidth = max(refWidth, len(row.Namespace+"/"+row.Name+"@"+row.Version))
		labelWidth = max(labelWidth, len(cli.PatternSourceLabel(row)))
	}
	for _, row := range r.Patterns {
		ref := row.Namespace + "/" + row.Name + "@" + row.Version
		label := cli.PatternSourceLabel(row)
		coverage := ""
		if len(row.Coverage) > 0 {
			coverage = th.Muted.Render("  coverage [" + strings.Join(row.Coverage, ", ") + "]")
		}
		p.Printf("%s%s  %s%s  %s rule(s)  %s extension(s)%s  %s\n",
			th.Bold.Render(ref), strings.Repeat(" ", refWidth-len(ref)),
			th.Info.Render(label), strings.Repeat(" ", labelWidth-len(label)),
			th.Bold.Render(pad2(row.Rules)),
			th.Bold.Render(itoa(row.Extensions)),
			coverage,
			th.Muted.Render(cli.ShortDigest(row.Digest)))
	}
}

// pad2 right-aligns a count in two columns so rule counts line up.
func pad2(n int) string {
	s := itoa(n)
	if len(s) < 2 {
		return " " + s
	}
	return s
}

func writePatternVendor(p *out.Printer, th Theme, res application.VendorPatternResult) {
	if res.Unchanged {
		p.Printf("%s is already vendored under %s (%s); nothing written\n",
			th.Bold.Render(res.Reference), th.Path.Render(".arclint/patterns"), th.Muted.Render(cli.ShortDigest(res.Digest)))
		return
	}
	p.Printf("%s %s (%s, %s) to %s\n",
		th.OK.Render("vendored"), th.Bold.Render(res.Reference),
		th.Info.Render(string(res.Source)), th.Muted.Render(cli.ShortDigest(res.Digest)),
		th.Path.Render(res.Path))
	if res.Replaced != "" {
		p.Printf("%s %s\n", th.Warning.Render("replaced"), res.Replaced)
	}
	p.Printf("%s commit the directory; every load verifies it against manifest.json\n", th.Muted.Render("next:"))
}

func writePatternInstall(p *out.Printer, th Theme, res application.InstallPatternResult) {
	p.Printf("%s %s (%s, %s)\n",
		th.OK.Render("installed"), th.Bold.Render(res.Reference),
		th.Info.Render(string(res.Source)), th.Muted.Render(cli.ShortDigest(res.Digest)))
	if res.VendoredPath != "" {
		if res.VendorReplaced != "" {
			p.Printf("%s %s, replacing %s\n", th.OK.Render("vendored to"), th.Path.Render(res.VendoredPath), res.VendorReplaced)
		} else {
			p.Printf("%s %s\n", th.OK.Render("vendored to"), th.Path.Render(res.VendoredPath))
		}
	}
	switch {
	case res.RulesetCreated:
		p.Printf("%s %s\n", th.OK.Render("wrote"), th.Path.Render(res.RulesetPath))
	case res.RulesetReplaced != "":
		p.Printf("%s %s, moving the entry from %s\n", th.OK.Render("extended"), th.Path.Render(res.RulesetPath), res.RulesetReplaced)
	default:
		p.Printf("%s %s\n", th.OK.Render("extended"), th.Path.Render(res.RulesetPath))
	}
	if len(res.Bound) > 0 {
		p.Println(th.Bold.Render("bound:"))
		for _, b := range res.Bound {
			p.Printf("  %s: %s\n", b.Module, th.Path.Render(strings.Join(b.Paths, ", ")))
		}
	}
	if len(res.Adopted) > 0 {
		p.Printf("%s %s\n", th.Muted.Render("adopted declared module(s):"), strings.Join(res.Adopted, ", "))
	}
	if len(res.Unbound) > 0 {
		p.Println(th.Warning.Render("unbound") + " (bind each under extends[].bind before the ruleset loads):")
		for _, m := range res.Unbound {
			p.Printf("  %s\n", m)
		}
		p.Printf("%s bind the unbound modules, then run `arclint check .`\n", th.Muted.Render("next:"))
		return
	}
	p.Printf("%s run `arclint check .`\n", th.Muted.Render("next:"))
}

func writePatternExport(p *out.Printer, th Theme, res application.ExportPatternResult) {
	if res.Replaced {
		p.Printf("%s %s (%s) to %s, replacing the listed version\n",
			th.OK.Render("published"), th.Bold.Render(res.Reference), th.Muted.Render(cli.ShortDigest(res.Digest)), th.Path.Render(res.VersionDir))
	} else {
		p.Printf("%s %s (%s) to %s\n",
			th.OK.Render("published"), th.Bold.Render(res.Reference), th.Muted.Render(cli.ShortDigest(res.Digest)), th.Path.Render(res.VersionDir))
	}
	p.Printf("%s %s\n", th.OK.Render("updated"), th.Path.Render(res.IndexPath))
}
