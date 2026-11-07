package application

import (
	"crypto/rand"
	"encoding/hex"
	"fmt"
	"time"

	"github.com/wixregiga/arclint/internal/domain/conformance"
	"github.com/wixregiga/arclint/internal/domain/rule"
	"github.com/wixregiga/arclint/internal/domain/vocab"
)

// DomainEvidence is one assessment's evidence about the recorded
// language, as Studio reads it: every entry's EntryEvidence and what no
// entry can be given, under the snapshot that names this assessment.
// Excerpts are read against one snapshot, so a location from an older
// assessment is never read as if it belonged to a newer one.
type DomainEvidence struct {
	// Snapshot names this assessment; AssessedAt is when it began and
	// Took how long it ran.
	Snapshot   string
	AssessedAt time.Time
	Took       time.Duration
	// Recording is the domain recording's path; Found is false when the
	// project records no language yet.
	Recording   string
	Found       bool
	Attribution conformance.EvidenceAttribution
	languages   []rule.Language
	// readable is every source file the assessment analyzed in a
	// configured language, and the domain recording: the only files an
	// excerpt of this snapshot may read.
	readable map[string]int64
}

// Readable reports whether an excerpt of this snapshot may read the
// file: the assessment analyzed it as source, or it is the domain
// recording.
func (d DomainEvidence) Readable(path string) bool {
	_, ok := d.readable[path]
	return ok
}

// assessedSize is the size the assessment observed the file at; false
// when it was not observed, as the domain recording is not.
func (d DomainEvidence) assessedSize(path string) (int64, bool) {
	size, ok := d.readable[path]
	return size, ok && size >= 0
}

// ExpectedCarrier names the declaration the recording says carries the
// contract, spelled as the project's languages spell a method, for a
// contract whose carrier was not located.
func (d DomainEvidence) ExpectedCarrier(c conformance.ContractEvidence) string {
	if c.Kind == vocab.ConceptAssertion {
		return expectedAssertionCarrier(c.Key, c.Owner, d.languages)
	}
	return expectedInvariantCarrier(DomainInvariantRef{Key: c.Key, Owner: c.Owner, OwnerConcept: c.OwnerConcept}, d.languages)
}

// AssessDomainEvidence runs one complete Conformance Check, with the
// committed Baseline applied as `arclint check` applies it, and
// attributes its findings to the recorded entries. It reads the
// repository once: the declarations it locates entries with come from
// the same observations the check judged. It changes no source file and
// no domain recording; an Extension may refresh its transpile cache.
type AssessDomainEvidence struct {
	check FreshCheck
	now   func() time.Time
}

// FreshCheck composes the Conformance Check over the project as its
// files are now. A rule repository or an Extension host reads its files
// once, so a server that assesses again and again composes the check
// for every assessment, through the composition `arclint check` uses.
type FreshCheck func() (AssessConformance, error)

// NewAssessDomainEvidence takes the composition of the Conformance
// Check. now is the clock the snapshot is stamped with; nil uses the
// wall clock.
func NewAssessDomainEvidence(check FreshCheck, now func() time.Time) (AssessDomainEvidence, error) {
	if check == nil {
		return AssessDomainEvidence{}, fmt.Errorf("assess domain evidence: missing conformance check")
	}
	if now == nil {
		now = time.Now
	}
	return AssessDomainEvidence{check: check, now: now}, nil
}

// Execute assesses the repository as it is now and names the result
// with a fresh snapshot.
func (uc AssessDomainEvidence) Execute() (DomainEvidence, error) {
	started := uc.now()
	assess, err := uc.check()
	if err != nil {
		return DomainEvidence{}, fmt.Errorf("compose the conformance check: %w", err)
	}
	if assess.rules == nil {
		return DomainEvidence{}, fmt.Errorf("compose the conformance check: no rule repository")
	}
	run, err := assess.run(AssessConformanceRequest{}, []rule.Fact{rule.FactDeclarations})
	if err != nil {
		return DomainEvidence{}, err
	}
	facts, err := conformance.NewInspectionFacts(run.configured.Zones, run.observations, rule.FactDeclarations)
	if err != nil {
		return DomainEvidence{}, fmt.Errorf("prepare declarations: %w", err)
	}
	carriers, err := conformance.NewCarriers(facts, run.knowledge)
	if err != nil {
		return DomainEvidence{}, fmt.Errorf("locate the recorded language: %w", err)
	}
	readable := map[string]int64{}
	var failed []string
	for _, f := range run.observations.Files() {
		lf, ok := run.observations.FactsFor(f.Path)
		if !ok || lf.Language == "" {
			// Every scanned file is observed; only source a language
			// adapter analyzed is readable, so a manifest, a document
			// or a credentials file never is.
			continue
		}
		readable[f.Path] = f.Size
		if lf.ParseFailure != "" {
			failed = append(failed, f.Path)
		}
	}
	recording := run.knowledge.Recording.Path()
	if recording != "" {
		readable[recording] = -1
	}
	attribution, err := conformance.NewEvidenceAttribution(conformance.EvidenceAttributionSpec{
		Knowledge: run.knowledge, Assessment: run.assessment, Carriers: carriers, ParseFailures: failed,
	})
	if err != nil {
		return DomainEvidence{}, fmt.Errorf("attribute the assessment: %w", err)
	}
	snapshot, err := snapshotName()
	if err != nil {
		return DomainEvidence{}, err
	}
	return DomainEvidence{
		Snapshot:    snapshot,
		AssessedAt:  started,
		Took:        uc.now().Sub(started),
		Recording:   recording,
		Found:       run.found,
		Attribution: attribution,
		languages:   run.configured.Languages,
		readable:    readable,
	}, nil
}

// snapshotName is 128 random bits in hex: no two assessments of one
// server share it, and no client can guess another's.
func snapshotName() (string, error) {
	var b [16]byte
	if _, err := rand.Read(b[:]); err != nil {
		return "", fmt.Errorf("name the evidence snapshot: %w", err)
	}
	return hex.EncodeToString(b[:]), nil
}

// Composed reports whether the use case was built by its constructor.
func (uc AssessDomainEvidence) Composed() bool { return uc.check != nil && uc.now != nil }
