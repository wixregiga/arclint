package application

import (
	"fmt"

	"github.com/wixregiga/arclint/internal/domain/rule"
	"github.com/wixregiga/arclint/internal/domain/vocab"
)

// DomainRecordingSetting works out, in memory, rules.arclint.yaml edited
// to name the project's DomainRecording under domain.file, proven
// through the strict loader, and whether that differs from the ruleset
// as it is. Naming the zero DomainRecording names none: it removes the
// setting, and the project's DomainRecording is then the one discovery
// finds. Nothing is written; the returned change writes it.
type DomainRecordingSetting interface {
	PlanDomainRecordingName(vocab.DomainRecording) (FileChange, bool, error)
}

// ConvertDomainRecordingResult reports a project moving its recorded
// language into another DomainRecording.
type ConvertDomainRecordingResult struct {
	ExportDomainRecordingResult
	// Named is the DomainRecording rules.arclint.yaml names under
	// domain.file once the project has moved: Target, or the zero
	// DomainRecording when the ruleset names none because discovery
	// finds Target without it.
	Named vocab.DomainRecording
	// RulesetChanged reports that rules.arclint.yaml was edited to say
	// so: it now names Target under domain.file, or no longer names a
	// file.
	RulesetChanged bool
}

// ConvertDomainRecording moves the project's recorded Ubiquitous
// Language into another DomainRecording: it translates the language
// there and proves the translation, makes rules.arclint.yaml name the
// new DomainRecording unless discovery finds it without, and removes
// the file the language was recorded in before, so the project records
// its language in exactly one file. The move is all or nothing.
type ConvertDomainRecording struct {
	export     ExportDomainRecording
	recordings DomainRecordings
	setting    DomainRecordingSetting
}

// NewConvertDomainRecording requires the translation, the port that
// reaches the project's files, and the ruleset setting.
func NewConvertDomainRecording(export ExportDomainRecording, recordings DomainRecordings, setting DomainRecordingSetting) (ConvertDomainRecording, error) {
	if export.knowledge == nil {
		return ConvertDomainRecording{}, fmt.Errorf("convert domain recording: missing export")
	}
	if recordings == nil {
		return ConvertDomainRecording{}, fmt.Errorf("convert domain recording: missing domain recordings")
	}
	if setting == nil {
		return ConvertDomainRecording{}, fmt.Errorf("convert domain recording: missing ruleset setting")
	}
	return ConvertDomainRecording{export: export, recordings: recordings, setting: setting}, nil
}

// Execute moves the language into the file target names, relative to
// the directory that holds rules.arclint.yaml. Every file the move
// touches is worked out in memory first: the translation, read back and
// proven; the ruleset, edited and proven; and the removal of the
// previous file. Then they are written in that order, and when one write
// fails the ones already written are put back, so an error means
// nothing changed.
func (uc ConvertDomainRecording) Execute(target string) (ConvertDomainRecordingResult, error) {
	to, err := uc.export.target(target)
	if err != nil {
		return ConvertDomainRecordingResult{}, err
	}
	from := uc.export.knowledge.Recording()
	named, err := uc.settingAfter(from, to)
	if err != nil {
		return ConvertDomainRecordingResult{}, fmt.Errorf("decide what %s names once the language moves to %s; nothing was written: %w", rule.RulesetFileName, to.Path(), err)
	}
	naming, changed, err := uc.setting.PlanDomainRecordingName(named)
	if err != nil {
		return ConvertDomainRecordingResult{}, fmt.Errorf("%s cannot be edited to name the domain file, so nothing was written: %w", rule.RulesetFileName, err)
	}
	exported, translation, err := uc.export.plan(to)
	if err != nil {
		return ConvertDomainRecordingResult{}, err
	}
	removal, err := uc.recordings.PlanRemoval(from)
	if err != nil {
		return ConvertDomainRecordingResult{}, fmt.Errorf("%s cannot be removed, so nothing was written: %w", from.Path(), err)
	}
	if err := applyTogether(translation, naming, removal); err != nil {
		return ConvertDomainRecordingResult{}, fmt.Errorf("move the domain from %s to %s: %w", from.Path(), to.Path(), err)
	}
	return ConvertDomainRecordingResult{ExportDomainRecordingResult: exported, Named: named, RulesetChanged: changed}, nil
}

// settingAfter decides what rules.arclint.yaml must name once the
// project has moved from one DomainRecording to another, judged against
// the files that will exist then: to, no longer from, and every other
// file the project holds. When discovery with no named file chooses to,
// the ruleset names none, the zero DomainRecording; otherwise it names
// to, because discovery would choose another file.
func (uc ConvertDomainRecording) settingAfter(from, to vocab.DomainRecording) (vocab.DomainRecording, error) {
	existsAfter := func(candidate vocab.DomainRecording) (bool, error) {
		switch candidate {
		case from:
			return false, nil
		case to:
			return true, nil
		default:
			return uc.recordings.Exists(candidate)
		}
	}
	chosen, _, err := vocab.ResolveDomainRecording(vocab.DomainRecording{}, existsAfter)
	if err != nil {
		return vocab.DomainRecording{}, fmt.Errorf("%w", err)
	}
	if chosen == to {
		return vocab.DomainRecording{}, nil
	}
	return to, nil
}
