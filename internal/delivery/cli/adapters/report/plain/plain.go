// Package plain renders sealed CLI reports as plain text, preserving the
// historical human/text byte contracts of the delivery/cli writers.
package plain

import (
	"fmt"
	"io"

	"github.com/wixregiga/arclint/internal/delivery/cli"
)

// Renderer is the plain-text sealed report adapter.
type Renderer struct{}

// fullWriter turns a writer contract violation into io.ErrShortWrite so
// helpers built on fmt cannot accidentally report a truncated report as clean.
type fullWriter struct {
	io.Writer
}

func (w fullWriter) Write(p []byte) (int, error) {
	n, err := w.Writer.Write(p)
	if err != nil {
		return n, fmt.Errorf("write report: %w", err)
	}
	if n < len(p) {
		return n, io.ErrShortWrite
	}
	return n, nil
}

// New returns the plain-text report Renderer.
func New() cli.Renderer {
	return Renderer{}
}

// Render writes r as plain text to w.
func (Renderer) Render(w io.Writer, r cli.Report) error {
	w = fullWriter{Writer: w}
	switch v := r.(type) {
	case cli.AssessmentReport:
		return writeCheck(w, v.Assessment)
	case cli.RuleListReport:
		return writeRuleRows(w, v.Rules)
	case cli.RuleDetailReport:
		return writeRuleDetail(w, v.Detail)
	case cli.RuleTestReport:
		return writeRuleTestResults(w, v.Results)
	case cli.InitReport:
		return writeInit(w, v)
	case cli.ContextReport:
		return writeContext(w, v.Context)
	case cli.DomainInitReport:
		return writeDomainInit(w, v)
	case cli.DomainMissingReport:
		return writeMissingDomainGuidance(w)
	case cli.DomainOverviewReport:
		if !v.Overview.Found {
			return writeMissingDomainGuidance(w)
		}
		return writeOverviewText(w, v.Overview)
	case cli.DomainListReport:
		if v.Listing.Language.Empty() {
			return writeMissingDomainGuidance(w)
		}
		return writeListText(w, v.Listing)
	case cli.DomainShowReport:
		return writeShowText(w, v.View)
	case cli.DomainExplainReport:
		return writeExplainText(w, v.Docs)
	case cli.DomainDefineReport:
		return writeDefineText(w, v)
	case cli.DomainRemoveReport:
		return writeRemoveText(w, v.Result)
	case cli.AgentsStatusReport:
		return writeAgentsStatus(w, v)
	case cli.BaselineCaptureReport:
		return writeBaselineCapture(w, v)
	case cli.BaselineRefreshReport:
		return writeBaselineRefresh(w, v)
	case cli.PatternsReport:
		return writePatterns(w, v)
	case cli.PatternVendorReport:
		return writePatternVendor(w, v)
	case cli.PatternInstallReport:
		return writePatternInstall(w, v)
	case cli.PatternExportReport:
		return writePatternExport(w, v)
	case cli.SDKInitReport:
		return writeSDKInit(w, v)
	default:
		return fmt.Errorf("report: unknown type %T", r)
	}
}
