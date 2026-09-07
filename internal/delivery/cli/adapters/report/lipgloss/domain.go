package lipgloss

import (
	"github.com/charmbracelet/lipgloss"

	"github.com/wixregiga/arclint/internal/application"
	"github.com/wixregiga/arclint/internal/delivery/cli"
	"github.com/wixregiga/arclint/internal/delivery/cli/adapters/report/internal/domaintext"
	"github.com/wixregiga/arclint/internal/delivery/cli/adapters/report/internal/out"
	"github.com/wixregiga/arclint/internal/domain/vocab"
)

// domainStyle lends the theme to the domain wording shared with the
// plain renderer: headings bold, secondary lines muted, successes in
// the OK color, sources as paths, and a missing anchor at warning
// weight.
func (th Theme) domainStyle() domaintext.Style {
	return domaintext.Style{
		Bold:  render(th.Bold),
		Muted: render(th.Muted),
		OK:    render(th.OK),
		Path:  render(th.Path),
		Warn:  render(th.Warning),
	}
}

func render(style lipgloss.Style) func(string) string {
	return func(s string) string { return style.Render(s) }
}

func writeDomainInit(p *out.Printer, th Theme, result application.InitDomainResult) {
	domaintext.Init(p, th.domainStyle(), result)
}

func writeDomainMissing(p *out.Printer, th Theme) {
	domaintext.Missing(p, th.domainStyle())
}

func writeDomainOverview(p *out.Printer, th Theme, result application.DomainOverview) {
	domaintext.Overview(p, th.domainStyle(), result)
}

func writeDomainList(p *out.Printer, th Theme, result application.DomainListing) {
	domaintext.List(p, th.domainStyle(), result)
}

func writeDomainShow(p *out.Printer, th Theme, view application.DomainEntryView) {
	domaintext.Show(p, th.domainStyle(), view)
}

func writeDomainExplain(p *out.Printer, th Theme, docs []vocab.ConceptDoc) {
	domaintext.Explain(p, th.domainStyle(), docs)
}

func writeDomainDefine(p *out.Printer, th Theme, rep cli.DomainDefineReport) {
	domaintext.Define(p, th.domainStyle(), rep.Result, rep.Change)
}

func writeDomainRemove(p *out.Printer, th Theme, result application.DomainRemoveResult) {
	domaintext.Remove(p, th.domainStyle(), result)
}

func writeDomainKnowledge(p *out.Printer, th Theme, d *application.DomainKnowledge) {
	domaintext.Knowledge(p, th.domainStyle(), d)
}
