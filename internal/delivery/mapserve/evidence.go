package mapserve

import (
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"strconv"
	"sync"
	"time"

	"github.com/wixregiga/arclint/internal/application"
	"github.com/wixregiga/arclint/internal/domain/conformance"
)

// Routes of the code evidence. Both are GETs that change no source
// file and no domain recording; the assessment may refresh an
// Extension's transpile cache under .arclint/cache.
const (
	EvidencePath = "/api/evidence"
	SourcePath   = "/api/source"
)

// keptSnapshots is how many assessments the server keeps excerpts
// readable for: a page reading an excerpt of an assessment a newer one
// has replaced more than this many times is told its evidence is stale.
const keptSnapshots = 4

// pathLimit bounds the path an excerpt request names.
const pathLimit = 4096

// Evidence is the code evidence a server lends its page: the
// assessment that attributes findings to the recorded entries, and the
// excerpts read against one assessment's snapshot.
type Evidence struct {
	Assess   application.AssessDomainEvidence
	Excerpts application.ReadSourceExcerpt
}

// evidenceRoutes keeps the latest assessments, so an excerpt is read
// only from source files the assessment it belongs to analyzed.
type evidenceRoutes struct {
	evidence *Evidence
	// edits is the server's edit lock: an assessment reads the domain
	// recording while no edit is writing it.
	edits *sync.RWMutex
	// running lets one assessment run at a time; a second request waits.
	running sync.Mutex
	mu      sync.Mutex
	kept    []application.DomainEvidence
}

func (r *evidenceRoutes) keep(e application.DomainEvidence) {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.kept = append(r.kept, e)
	if len(r.kept) > keptSnapshots {
		r.kept = r.kept[len(r.kept)-keptSnapshots:]
	}
}

func (r *evidenceRoutes) snapshot(name string) (application.DomainEvidence, bool) {
	r.mu.Lock()
	defer r.mu.Unlock()
	for _, e := range r.kept {
		if e.Snapshot == name {
			return e, true
		}
	}
	return application.DomainEvidence{}, false
}

// fromThePageItself refuses a request a browser marks as sent by
// another site. A cross-site page can make a browser send a GET without
// an Origin, as an image or a no-cors fetch; it cannot read the answer,
// but it could make the server assess the repository again and again.
// Browsers send Sec-Fetch-Site on every request; a client that sends
// none, such as curl, is not a page on another site.
func fromThePageItself(next http.HandlerFunc) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		switch site := r.Header.Get("Sec-Fetch-Site"); site {
		case "", "same-origin", "none":
			next(w, r)
		default:
			writeError(w, http.StatusForbidden, fmt.Errorf("a request from a %s page is not from the map's own page", site))
		}
	}
}

// assess runs one assessment of the repository as it is now.
func (r *evidenceRoutes) assess(w http.ResponseWriter, _ *http.Request) {
	r.running.Lock()
	defer r.running.Unlock()
	r.edits.RLock()
	result, err := r.evidence.Assess.Execute()
	r.edits.RUnlock()
	if err != nil {
		writeError(w, http.StatusInternalServerError, fmt.Errorf("assess code evidence: %w", err))
		return
	}
	r.keep(result)
	body, err := json.Marshal(evidenceDocumentOf(result))
	if err != nil {
		writeError(w, http.StatusInternalServerError, fmt.Errorf("render code evidence: %w", err))
		return
	}
	writeJSON(w, http.StatusOK, append(body, '\n'))
}

// source reads an excerpt around one location of one snapshot's
// evidence. The request names the snapshot, a repository-relative path
// and a line; nothing the page sends is trusted further than that.
func (r *evidenceRoutes) source(w http.ResponseWriter, req *http.Request) {
	q := req.URL.Query()
	name, file, lineText := q.Get("snapshot"), q.Get("path"), q.Get("line")
	if name == "" || file == "" || len(file) > pathLimit {
		writeError(w, http.StatusBadRequest, errors.New("an excerpt names a snapshot, a path and a line"))
		return
	}
	line, err := strconv.Atoi(lineText)
	if err != nil {
		writeError(w, http.StatusBadRequest, fmt.Errorf("line %q is not a number", lineText))
		return
	}
	evidence, ok := r.snapshot(name)
	if !ok {
		writeError(w, http.StatusConflict, errors.New("this evidence is stale: a newer assessment replaced it; assess again"))
		return
	}
	excerpt, err := r.evidence.Excerpts.Execute(evidence, file, line)
	switch {
	case errors.Is(err, application.ErrExcerptRequestInvalid):
		writeError(w, http.StatusBadRequest, err)
		return
	case errors.Is(err, application.ErrExcerptWithheld):
		writeError(w, http.StatusForbidden, err)
		return
	case errors.Is(err, application.ErrExcerptOutsideEvidence):
		writeError(w, http.StatusNotFound, err)
		return
	case err != nil:
		writeError(w, http.StatusUnprocessableEntity, err)
		return
	}
	doc := excerptDocument{Snapshot: excerpt.Snapshot, Path: excerpt.Path, Line: excerpt.Line, TotalLines: excerpt.TotalLines, Lines: []excerptLineDocument{}}
	for _, l := range excerpt.Lines {
		doc.Lines = append(doc.Lines, excerptLineDocument{Number: l.Number, Text: l.Text, Truncated: l.Truncated})
	}
	body, err := json.Marshal(doc)
	if err != nil {
		writeError(w, http.StatusInternalServerError, fmt.Errorf("render excerpt: %w", err))
		return
	}
	writeJSON(w, http.StatusOK, append(body, '\n'))
}

// The evidence document. web/src/shared/transport/evidence-documents.ts
// mirrors it field for field.
type (
	evidenceDocument struct {
		Snapshot      string                  `json:"snapshot"`
		AssessedAt    string                  `json:"assessedAt"`
		TookMs        int64                   `json:"tookMs"`
		Recording     string                  `json:"recording"`
		Found         bool                    `json:"found"`
		Complete      bool                    `json:"complete"`
		NotEvaluated  []string                `json:"notEvaluated"`
		ParseFailures []string                `json:"parseFailures"`
		Entries       []entryEvidenceDocument `json:"entries"`
		Unattributed  unattributedDocument    `json:"unattributed"`
	}
	locationDocument struct {
		Path     string `json:"path"`
		Line     int    `json:"line"`
		Readable bool   `json:"readable"`
	}
	entryEvidenceDocument struct {
		Concept      string                     `json:"concept"`
		Context      string                     `json:"context"`
		Owner        string                     `json:"owner"`
		Name         string                     `json:"name"`
		Reading      string                     `json:"reading"`
		RecordedAt   *locationDocument          `json:"recordedAt"`
		Declarations declarationsDocument       `json:"declarations"`
		Contracts    []contractEvidenceDocument `json:"contracts"`
		Findings     []findingDocument          `json:"findings"`
		Partial      []partialDocument          `json:"partial"`
	}
	declarationsDocument struct {
		State     string             `json:"state"`
		Locations []locationDocument `json:"locations"`
	}
	contractEvidenceDocument struct {
		Kind       string          `json:"kind"`
		Owner      string          `json:"owner"`
		Key        string          `json:"key"`
		Carrier    carrierDocument `json:"carrier"`
		Coverage   string          `json:"coverage"`
		NotChecked []string        `json:"notChecked"`
	}
	carrierDocument struct {
		State    string            `json:"state"`
		Location *locationDocument `json:"location"`
		Expected string            `json:"expected"`
	}
	findingDocument struct {
		Rule        string            `json:"rule"`
		Message     string            `json:"message"`
		Status      string            `json:"status"`
		Severity    string            `json:"severity"`
		Outcome     string            `json:"outcome"`
		Assurance   string            `json:"assurance"`
		Subject     subjectDocument   `json:"subject"`
		Location    *locationDocument `json:"location"`
		Bases       []string          `json:"bases"`
		Remediation string            `json:"remediation,omitempty"`
	}
	subjectDocument struct {
		Kind     string `json:"kind"`
		Identity string `json:"identity"`
	}
	partialDocument struct {
		Kind     string   `json:"kind"`
		Rule     string   `json:"rule,omitempty"`
		Contract string   `json:"contract,omitempty"`
		Outcome  string   `json:"outcome,omitempty"`
		Paths    []string `json:"paths,omitempty"`
	}
	unattributedDocument struct {
		Findings    []findingDocument    `json:"findings"`
		Diagnostics []diagnosticDocument `json:"diagnostics"`
	}
	diagnosticDocument struct {
		Kind     string            `json:"kind"`
		Rule     string            `json:"rule"`
		Severity string            `json:"severity"`
		Message  string            `json:"message"`
		Location *locationDocument `json:"location"`
	}
	excerptDocument struct {
		Snapshot   string                `json:"snapshot"`
		Path       string                `json:"path"`
		Line       int                   `json:"line"`
		TotalLines int                   `json:"totalLines"`
		Lines      []excerptLineDocument `json:"lines"`
	}
	excerptLineDocument struct {
		Number    int    `json:"number"`
		Text      string `json:"text"`
		Truncated bool   `json:"truncated,omitempty"`
	}
)

func evidenceDocumentOf(e application.DomainEvidence) evidenceDocument {
	a := e.Attribution
	doc := evidenceDocument{
		Snapshot:      e.Snapshot,
		AssessedAt:    e.AssessedAt.UTC().Format(time.RFC3339Nano),
		TookMs:        e.Took.Milliseconds(),
		Recording:     e.Recording,
		Found:         e.Found,
		Complete:      a.Complete(),
		NotEvaluated:  nonNil(a.NotEvaluatedRules()),
		ParseFailures: nonNil(a.ParseFailures()),
		Entries:       []entryEvidenceDocument{},
		Unattributed:  unattributedDocument{Findings: []findingDocument{}, Diagnostics: []diagnosticDocument{}},
	}
	for _, entry := range a.Entries() {
		doc.Entries = append(doc.Entries, entryDocumentOf(e, entry))
	}
	for _, v := range a.Unattributed() {
		doc.Unattributed.Findings = append(doc.Unattributed.Findings, findingDocumentOf(e, v, nil))
	}
	for _, d := range a.Diagnostics() {
		doc.Unattributed.Diagnostics = append(doc.Unattributed.Diagnostics, diagnosticDocument{
			Kind: string(d.Kind()), Rule: d.RuleID(), Severity: string(d.Severity()), Message: d.Message(),
			Location: locationOf(e, d.Path(), d.Line()),
		})
	}
	return doc
}

func entryDocumentOf(e application.DomainEvidence, entry conformance.EntryEvidence) entryEvidenceDocument {
	address := entry.Entry()
	doc := entryEvidenceDocument{
		Concept: string(address.Concept), Context: address.Context, Owner: address.Owner, Name: address.Name,
		Reading:      string(entry.Reading()),
		Declarations: declarationsDocument{State: string(entry.Declarations()), Locations: []locationDocument{}},
		Contracts:    []contractEvidenceDocument{},
		Findings:     []findingDocument{},
		Partial:      []partialDocument{},
	}
	if at, ok := entry.RecordedAt(); ok {
		doc.RecordedAt = locationOf(e, at.Path, at.Line)
	}
	for _, l := range entry.Locations() {
		doc.Declarations.Locations = append(doc.Declarations.Locations, *locationOf(e, l.Path, l.Line))
	}
	for _, c := range entry.Contracts() {
		carrier := carrierDocument{State: "missing"}
		switch {
		case c.Located:
			carrier = carrierDocument{State: "found", Location: locationOf(e, c.Carrier.Path, c.Carrier.Line)}
		case !c.CodeRead:
			// No file of the context's code was read, so nothing says
			// the carrier is missing.
			carrier.State = "not_observed"
		default:
			carrier.Expected = e.ExpectedCarrier(c)
		}
		notChecked := []string{}
		if c.Covered {
			notChecked = nonNil(c.Coverage.Unperformed())
		}
		doc.Contracts = append(doc.Contracts, contractEvidenceDocument{
			Kind: string(c.Kind), Owner: c.Owner, Key: c.Key, Carrier: carrier, Coverage: c.CoverageStatus(), NotChecked: notChecked,
		})
	}
	for _, f := range entry.Findings() {
		doc.Findings = append(doc.Findings, findingDocumentOf(e, f.Violation, f.Bases))
	}
	for _, p := range entry.Partial() {
		doc.Partial = append(doc.Partial, partialDocument{Kind: string(p.Kind), Rule: p.Rule, Contract: p.Contract, Outcome: p.Outcome, Paths: p.Paths})
	}
	return doc
}

func findingDocumentOf(e application.DomainEvidence, v conformance.Violation, bases []conformance.EvidenceBasis) findingDocument {
	doc := findingDocument{
		Rule: v.Rule().Qualified(), Message: v.Message(), Status: string(v.Status()), Severity: string(v.Severity()),
		Outcome: string(v.Outcome()), Assurance: string(v.Assurance()),
		Subject:     subjectDocument{Kind: string(v.Subject().Kind()), Identity: v.Subject().Identity()},
		Location:    locationOf(e, v.Path(), v.Line()),
		Bases:       []string{},
		Remediation: v.Remediation(),
	}
	for _, b := range bases {
		doc.Bases = append(doc.Bases, string(b))
	}
	return doc
}

// locationOf is a location the page shows: nil when there is no path,
// and readable only when this snapshot may read an excerpt of the file
// at a line. A location without a line is shown but never opened.
func locationOf(e application.DomainEvidence, path string, line int) *locationDocument {
	if path == "" {
		return nil
	}
	return &locationDocument{Path: path, Line: line, Readable: line > 0 && e.Readable(path)}
}

func nonNil(s []string) []string {
	if s == nil {
		return []string{}
	}
	return s
}
