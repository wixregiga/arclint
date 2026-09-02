package cli

import (
	"io"

	"github.com/wixregiga/arclint/internal/application"
	"github.com/wixregiga/arclint/internal/domain/conformance"
	"github.com/wixregiga/arclint/internal/domain/vocab"
)

// RendererName is the ArcLint-owned identity a sealed report Renderer is
// selected by at the composition root.
type RendererName string

// Sealed report renderer identities.
const (
	RendererPlain    RendererName = "plain"
	RendererJSON     RendererName = "json"
	RendererLipgloss RendererName = "lipgloss"
)

// Report is the sealed outbound product of one semantic CLI command.
// Concrete report types live in this package and implement report();
// adapters type-switch the closed set. Raw schema bytes and agents
// markdown bypass Renderer entirely.
//
//nolint:iface // Sealed interface for type-switching; report() is an intentional marker method.
type Report interface {
	report()
}

// Renderer writes one sealed Report to an output stream. Renderers never
// emit command errors: findings stay on stdout; ExitError diagnostics
// stay on stderr at the adapter boundary.
type Renderer interface {
	Render(w io.Writer, r Report) error
}

// AssessmentReport is the semantic product of `arclint check`.
type AssessmentReport struct {
	Assessment conformance.Assessment
}

func (AssessmentReport) report() {}

// RuleListReport is the one-line listing form of configured Rules.
type RuleListReport struct {
	Rules []application.RuleSummary
}

func (RuleListReport) report() {}

// RuleDetailReport is the complete view of one Rule.
type RuleDetailReport struct {
	Detail application.RuleDetail
}

func (RuleDetailReport) report() {}

// RuleTestReport is the result of running authored Rule Tests.
type RuleTestReport struct {
	Results []application.RuleTestResult
}

func (RuleTestReport) report() {}

// InitReport is the semantic product of `arclint init`.
type InitReport struct {
	Path string
}

func (InitReport) report() {}

// ContextReport is the semantic product of `arclint context`.
type ContextReport struct {
	Context application.ArchitecturalContext
}

func (ContextReport) report() {}

// DomainInitReport is the product of `arclint domain init`.
type DomainInitReport struct {
	Result application.InitDomainResult
}

func (DomainInitReport) report() {}

// DomainMissingReport guides the user when no Ubiquitous Language is recorded.
type DomainMissingReport struct{}

func (DomainMissingReport) report() {}

// DomainOverviewReport is the product of `arclint domain` / `domain overview`.
type DomainOverviewReport struct {
	Overview application.DomainOverview
}

func (DomainOverviewReport) report() {}

// DomainListReport is the product of `arclint domain list`.
type DomainListReport struct {
	Listing application.DomainListing
}

func (DomainListReport) report() {}

// DomainShowReport is the product of `arclint domain show`.
type DomainShowReport struct {
	View application.DomainDefinitionView
}

func (DomainShowReport) report() {}

// DomainExplainReport is the product of `arclint domain explain`.
// Single is true when the caller asked for one concept (JSON emits an
// object rather than an array).
type DomainExplainReport struct {
	Docs   []vocab.ConceptDoc
	Single bool
}

func (DomainExplainReport) report() {}

// DomainDefineReport is the product of `arclint domain define`.
// Request fields needed by plain/JSON docs are copied so adapters do
// not depend on the live request value. Definition is the pointer from
// the request (nil when unset); empty string means cleared.
type DomainDefineReport struct {
	Result       application.DomainDefineResult
	Definition   *string
	ClearAliases bool
	Owner        string
}

func (DomainDefineReport) report() {}

// DomainRemoveReport is the product of `arclint domain remove`.
type DomainRemoveReport struct {
	Result application.DomainRemoveResult
}

func (DomainRemoveReport) report() {}

// ArtifactWrite is one wrote/already-current status line.
type ArtifactWrite struct {
	Changed bool
	Path    string
}

// AgentsStatusReport is the product of agents write-status lines
// (install/skill). Raw AGENTS.md markdown stays a raw byte product.
type AgentsStatusReport struct {
	Writes []ArtifactWrite
}

func (AgentsStatusReport) report() {}

// BaselineCaptureReport is the product of `arclint baseline capture`.
type BaselineCaptureReport struct {
	Result application.CaptureBaselineResult
}

func (BaselineCaptureReport) report() {}

// BaselineRefreshReport is the product of `arclint baseline refresh`.
type BaselineRefreshReport struct {
	Result application.RefreshBaselineResult
}

func (BaselineRefreshReport) report() {}

// PatternsReport is the product of `arclint patterns`.
type PatternsReport struct {
	Patterns []application.PatternSummary
}

func (PatternsReport) report() {}

// SDKInitReport is the product of `arclint sdk init`.
type SDKInitReport struct {
	Paths []string
}

func (SDKInitReport) report() {}

// NewDomainDefineReport copies the define request fields adapters need.
func NewDomainDefineReport(result application.DomainDefineResult, req application.DefineDomainRequest) DomainDefineReport {
	return DomainDefineReport{
		Result:       result,
		ClearAliases: req.ClearAliases,
		Owner:        req.Owner,
		Definition:   req.Definition,
	}
}
