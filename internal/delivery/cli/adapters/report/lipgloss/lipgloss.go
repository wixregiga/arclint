// Package lipgloss renders sealed CLI reports with charmbracelet/lipgloss
// styles bound per writer. Token grammar matches the plain adapter;
// color is applied without reordering or inserting spaces into
// compiler-shaped path:line findings.
package lipgloss

import (
	"fmt"
	"io"

	"github.com/charmbracelet/lipgloss"
	"github.com/muesli/termenv"
	"github.com/wixregiga/arclint/internal/delivery/cli"
	"github.com/wixregiga/arclint/internal/delivery/cli/adapters/report/internal/out"
)

// renderer is the sealed lipgloss report adapter.
type renderer struct {
	// setup runs on the per-writer lipgloss.Renderer before Theme
	// construction. Tests inject an explicit color profile here.
	setup func(*lipgloss.Renderer)
}

// New returns a cli.Renderer that colorizes plain-shaped report text.
func New() cli.Renderer {
	return renderer{}
}

// NewWithRendererSetup returns a Renderer that runs setup on each
// per-writer lipgloss.Renderer before building Theme. The setup seam
// is how tests force a deterministic color profile.
func NewWithRendererSetup(setup func(*lipgloss.Renderer)) cli.Renderer {
	return renderer{setup: setup}
}

// Render writes one sealed Report with styles derived from w.
func (r renderer) Render(w io.Writer, rep cli.Report) error {
	rdr := lipgloss.NewRenderer(w)
	// Composition selects this adapter only for color-enabled terminals.
	// An explicit ANSI profile keeps the adapter honest: selecting
	// RendererLipgloss always paints instead of silently becoming plain.
	rdr.SetColorProfile(termenv.ANSI)
	if r.setup != nil {
		r.setup(rdr)
	}
	th := NewTheme(rdr)
	p := &out.Printer{W: w}

	switch x := rep.(type) {
	case cli.AssessmentReport:
		writeCheck(p, th, x.Assessment)
	case cli.RuleListReport:
		writeRuleRows(p, th, x.Rules)
	case cli.RuleDetailReport:
		writeRuleDetail(p, th, x.Detail)
	case cli.RuleTestReport:
		writeRuleTestResults(p, th, x.Results)
	case cli.InitReport:
		writeInit(p, th, x.Path)
	case cli.ContextReport:
		writeContext(p, th, x.Context)
	case cli.DomainInitReport:
		writeDomainInit(p, th, x.Result)
	case cli.DomainMissingReport:
		writeDomainMissing(p, th)
	case cli.DomainOverviewReport:
		if !x.Overview.Found {
			writeDomainMissing(p, th)
		} else {
			writeDomainOverview(p, th, x.Overview)
		}
	case cli.DomainListReport:
		if x.Listing.Language.Empty() {
			writeDomainMissing(p, th)
		} else {
			writeDomainList(p, th, x.Listing)
		}
	case cli.DomainShowReport:
		writeDomainShow(p, th, x.View)
	case cli.DomainExplainReport:
		writeDomainExplain(p, th, x.Docs)
	case cli.DomainDefineReport:
		writeDomainDefine(p, th, x)
	case cli.DomainRemoveReport:
		writeDomainRemove(p, th, x.Result)
	case cli.ArtifactStatusReport:
		writeArtifactStatus(p, th, x.Writes)
	case cli.BaselineCaptureReport:
		writeBaselineCapture(p, th, x.Result)
	case cli.BaselineRefreshReport:
		writeBaselineRefresh(p, th, x.Result)
	case cli.PatternsReport:
		writePatterns(p, th, x)
	case cli.PatternVendorReport:
		writePatternVendor(p, th, x.Result)
	case cli.PatternInstallReport:
		writePatternInstall(p, th, x.Result)
	case cli.PatternExportReport:
		writePatternExport(p, th, x.Result)
	case cli.SDKInitReport:
		writeSDKInit(p, th, x.Paths)
	default:
		return fmt.Errorf("report: unknown type %T", rep)
	}
	return p.Err
}
