package application

import (
	"fmt"

	"github.com/wixregiga/arclint/internal/domain/rule"
	"github.com/wixregiga/arclint/internal/domain/vocab"
)

// DomainRecordings reaches any DomainRecording of the project: it opens
// the vocabulary repository that reads one, chosen by the format the
// DomainRecording is written in, and reports whether a DomainRecording's
// file exists. It works out in memory, without writing, the file a
// DomainRecording would hold with a language written into it, and the
// removal of a DomainRecording's file. Composition implements it,
// because choosing an adapter by format is composition's decision.
type DomainRecordings interface {
	Open(vocab.DomainRecording) (vocab.Repository, error)
	Exists(vocab.DomainRecording) (bool, error)
	PlanRecord(vocab.DomainRecording, vocab.UbiquitousLanguage) (PlannedRecording, error)
	PlanRemoval(vocab.DomainRecording) (FileChange, error)
}

// PlannedRecording is the file a DomainRecording would hold once a
// language is written into it, worked out in memory: Change writes it,
// and Language is the language that file reads back as.
type PlannedRecording struct {
	Change   FileChange
	Language vocab.UbiquitousLanguage
}

// ExportDomainRecordingResult reports one translation of the project's
// recorded language into another DomainRecording.
type ExportDomainRecordingResult struct {
	// Source is the project's DomainRecording the language was read from.
	Source vocab.DomainRecording
	// Target is the DomainRecording the language was written into.
	Target vocab.DomainRecording
	// Updated reports a Target that already recorded a language, which
	// was edited in place so everything its format holds beyond the
	// recorded language survives.
	Updated bool
	// Counts is the translated language in numbers.
	Counts vocab.Counts
}

// ExportDomainRecording writes the project's recorded Ubiquitous
// Language into another DomainRecording, in the format that
// DomainRecording's extension names, and proves the translation by
// reading it back: the language must survive unchanged.
type ExportDomainRecording struct {
	knowledge  vocab.Repository
	recordings DomainRecordings
}

// NewExportDomainRecording requires the project's vocabulary repository
// and the port that reaches the target.
func NewExportDomainRecording(knowledge vocab.Repository, recordings DomainRecordings) (ExportDomainRecording, error) {
	if knowledge == nil {
		return ExportDomainRecording{}, fmt.Errorf("export domain recording: missing knowledge repository")
	}
	if recordings == nil {
		return ExportDomainRecording{}, fmt.Errorf("export domain recording: missing domain recordings")
	}
	return ExportDomainRecording{knowledge: knowledge, recordings: recordings}, nil
}

// Execute translates the recorded language into the file target names,
// relative to the directory that holds rules.arclint.yaml. A target
// that already records a language is edited in place; a target that
// cannot be read is never overwritten. The translation is worked out
// and read back in memory first: when the language read back differs
// from the recorded one, the error says where they part ways and
// nothing is written.
func (uc ExportDomainRecording) Execute(target string) (ExportDomainRecordingResult, error) {
	to, err := uc.target(target)
	if err != nil {
		return ExportDomainRecordingResult{}, err
	}
	result, change, err := uc.plan(to)
	if err != nil {
		return ExportDomainRecordingResult{}, err
	}
	if err := applyTogether(change); err != nil {
		return ExportDomainRecordingResult{}, err
	}
	return result, nil
}

// target builds the DomainFile a translation writes from the path as
// written. A path that cannot be the project's domain file, or that is
// already the project's domain file, is a usage error.
func (uc ExportDomainRecording) target(file string) (vocab.DomainRecording, error) {
	named, err := rule.NewDomainFile(file)
	if err != nil {
		return vocab.DomainRecording{}, fmt.Errorf("%w: %v", ErrDomainUsage, err)
	}
	to := named.Recording()
	if to.Path() == uc.knowledge.Recording().Path() {
		return vocab.DomainRecording{}, fmt.Errorf("%w: %s is already the project's domain file; name another file", ErrDomainUsage, to.Path())
	}
	return to, nil
}

// plan works out, in memory, the translation of the recorded language
// into to and proves it by reading the planned file back. Nothing is
// written; the returned change writes it.
func (uc ExportDomainRecording) plan(to vocab.DomainRecording) (ExportDomainRecordingResult, FileChange, error) {
	from := uc.knowledge.Recording()
	destination, err := uc.recordings.Open(to)
	if err != nil {
		return ExportDomainRecordingResult{}, nil, fmt.Errorf("open %s: %w", to.Path(), err)
	}
	lang, found, err := uc.knowledge.RecordedLanguage()
	if err != nil {
		return ExportDomainRecordingResult{}, nil, fmt.Errorf("load domain model: %w", err)
	}
	if !found {
		return ExportDomainRecordingResult{}, nil, fmt.Errorf("%w: no recorded Ubiquitous Language at %s to translate", ErrDomainUsage, from.Path())
	}
	_, existed, err := destination.RecordedLanguage()
	if err != nil {
		return ExportDomainRecordingResult{}, nil, fmt.Errorf("%s already exists and cannot be read, so it is left untouched: %w", to.Path(), err)
	}
	planned, err := uc.recordings.PlanRecord(to, lang)
	if err != nil {
		return ExportDomainRecordingResult{}, nil, fmt.Errorf("write %s: %w", to.Path(), err)
	}
	if difference := lang.Difference(planned.Language); difference != "" {
		return ExportDomainRecordingResult{}, nil, fmt.Errorf("%s would not record the same language as %s, so nothing was written: %s", to.Path(), from.Path(), difference)
	}
	return ExportDomainRecordingResult{Source: from, Target: to, Updated: existed, Counts: lang.Counts()}, planned.Change, nil
}
