package plain

import (
	"io"

	"github.com/wixregiga/arclint/internal/application"
	"github.com/wixregiga/arclint/internal/delivery/cli"
	"github.com/wixregiga/arclint/internal/delivery/cli/adapters/report/internal/domaintext"
	"github.com/wixregiga/arclint/internal/delivery/cli/adapters/report/internal/out"
	"github.com/wixregiga/arclint/internal/domain/vocab"
)

// The domain reports share their wording with the Lipgloss renderer
// through domaintext; the plain renderer lends the Style that changes
// nothing.

func writeDomainInit(w io.Writer, r cli.DomainInitReport) error {
	p := &out.Printer{W: w}
	domaintext.Init(p, domaintext.Plain(), r.Result)
	return p.Err
}

func writeMissingDomainGuidance(w io.Writer) error {
	p := &out.Printer{W: w}
	domaintext.Missing(p, domaintext.Plain())
	return p.Err
}

func writeOverviewText(w io.Writer, result application.DomainOverview) error {
	p := &out.Printer{W: w}
	domaintext.Overview(p, domaintext.Plain(), result)
	return p.Err
}

func writeListText(w io.Writer, result application.DomainListing) error {
	p := &out.Printer{W: w}
	domaintext.List(p, domaintext.Plain(), result)
	return p.Err
}

func writeShowText(w io.Writer, view application.DomainEntryView) error {
	p := &out.Printer{W: w}
	domaintext.Show(p, domaintext.Plain(), view)
	return p.Err
}

func writeExplainText(w io.Writer, docs []vocab.ConceptDoc) error {
	p := &out.Printer{W: w}
	domaintext.Explain(p, domaintext.Plain(), docs)
	return p.Err
}

func writeDefineText(w io.Writer, r cli.DomainDefineReport) error {
	p := &out.Printer{W: w}
	domaintext.Define(p, domaintext.Plain(), r.Result, r.Change)
	return p.Err
}

func writeRemoveText(w io.Writer, result application.DomainRemoveResult) error {
	p := &out.Printer{W: w}
	domaintext.Remove(p, domaintext.Plain(), result)
	return p.Err
}
