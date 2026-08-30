package lipgloss

import (
	"strings"

	"github.com/wixregiga/arclint/internal/application"
	"github.com/wixregiga/arclint/internal/delivery/cli"
	"github.com/wixregiga/arclint/internal/delivery/cli/adapters/report/internal/out"
)

func writeInit(p *out.Printer, th Theme, path string) {
	p.Printf("%s %s\n", th.OK.Render("wrote"), th.Path.Render(path))
	p.Printf("%s declare your modules, then run `arclint check .`\n", th.Muted.Render("next:"))
}

func writeBaselineCapture(p *out.Printer, th Theme, result application.CaptureBaselineResult) {
	p.Printf("%s %s finding(s) across %s applied rule(s)\n",
		th.OK.Render("baseline captured:"),
		th.Bold.Render(itoa(result.Findings)),
		th.Bold.Render(itoa(result.Rules)))
}

func writeBaselineRefresh(p *out.Printer, th Theme, result application.RefreshBaselineResult) {
	p.Printf("%s %s finding(s) across %s applied rule(s), %s stale entr(ies) dropped\n",
		th.OK.Render("baseline refreshed:"),
		th.Bold.Render(itoa(result.Findings)),
		th.Bold.Render(itoa(result.Rules)),
		th.Bold.Render(itoa(result.RemovedStale)))
}

func writePatterns(p *out.Printer, th Theme, rows []application.PatternSummary) {
	if len(rows) == 0 {
		p.Println(th.Muted.Render("no patterns available"))
		return
	}
	for _, row := range rows {
		coverage := ""
		if len(row.Coverage) > 0 {
			coverage = th.Muted.Render("  coverage [" + strings.Join(row.Coverage, ", ") + "]")
		}
		id := th.Muted.Render(row.Namespace + "/" + row.Name + "@" + row.Version)
		p.Printf("%s  %s rule(s)  %s extension(s)%s\n",
			id,
			th.Bold.Render(itoa(row.Rules)),
			th.Bold.Render(itoa(row.Extensions)),
			coverage)
	}
}

func writeAgentsStatus(p *out.Printer, th Theme, writes []cli.ArtifactWrite) {
	for _, w := range writes {
		if w.Changed {
			p.Printf("%s %s\n", th.OK.Render("wrote"), th.Path.Render(w.Path))
			continue
		}
		p.Printf("%s %s\n", th.Path.Render(w.Path), th.Muted.Render("already current"))
	}
}

func writeSDKInit(p *out.Printer, th Theme, paths []string) {
	for _, path := range paths {
		p.Printf("%s %s\n", th.OK.Render("wrote"), th.Path.Render(path))
	}
}
