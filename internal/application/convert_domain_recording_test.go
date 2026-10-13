package application

import (
	"errors"
	"strings"
	"testing"

	"github.com/wixregiga/arclint/internal/domain/vocab"
)

// recordedFile is one DomainRecording held in memory. lossy, when set,
// changes the language on its way in, the way a translation that loses
// meaning would.
type recordedFile struct {
	recording vocab.DomainRecording
	lang      vocab.UbiquitousLanguage
	found     bool
	broken    error
	lossy     func(vocab.UbiquitousLanguage) vocab.UbiquitousLanguage
	records   int
}

func (f *recordedFile) Recording() vocab.DomainRecording { return f.recording }

func (f *recordedFile) RecordedLanguage() (vocab.UbiquitousLanguage, bool, error) {
	if f.broken != nil {
		return vocab.UbiquitousLanguage{}, true, f.broken
	}
	lang := f.lang
	lang.Recording = f.recording
	return lang, f.found, nil
}

func (f *recordedFile) Record(lang vocab.UbiquitousLanguage) error {
	if f.lossy != nil {
		lang = f.lossy(lang)
	}
	f.lang, f.found = lang, true
	f.records++
	return nil
}

// memoryChange is one planned change to the files held in memory.
// failApply makes Apply fail the way a refused write does, changing
// nothing.
type memoryChange struct {
	apply, revert func()
	failApply     error
}

func (c *memoryChange) Apply() error {
	if c.failApply != nil {
		return c.failApply
	}
	c.apply()
	return nil
}

func (c *memoryChange) Revert() error {
	c.revert()
	return nil
}

type memoryRecordings struct {
	files     map[string]*recordedFile
	removed   []string
	existsErr error
	// failWrite and failRemove make the planned write of a path, or the
	// planned removal, fail when applied.
	failWrite  map[string]error
	failRemove error
}

func (m *memoryRecordings) Open(r vocab.DomainRecording) (vocab.Repository, error) {
	if f, ok := m.files[r.Path()]; ok {
		return f, nil
	}
	return &recordedFile{recording: r}, nil
}

func (m *memoryRecordings) Exists(r vocab.DomainRecording) (bool, error) {
	if m.existsErr != nil {
		return false, m.existsErr
	}
	_, ok := m.files[r.Path()]
	return ok, nil
}

// PlanRecord reads back what the file would record: a target set up as
// lossy loses meaning on the way in, as a lossy translation would.
func (m *memoryRecordings) PlanRecord(r vocab.DomainRecording, lang vocab.UbiquitousLanguage) (PlannedRecording, error) {
	previous, existed := m.files[r.Path()]
	readBack := lang
	if existed && previous.lossy != nil {
		readBack = previous.lossy(lang)
	}
	change := &memoryChange{
		failApply: m.failWrite[r.Path()],
		apply: func() {
			m.files[r.Path()] = &recordedFile{recording: r, lang: readBack, found: true, records: 1}
		},
		revert: func() {
			if existed {
				m.files[r.Path()] = previous
			} else {
				delete(m.files, r.Path())
			}
		},
	}
	return PlannedRecording{Change: change, Language: readBack}, nil
}

func (m *memoryRecordings) PlanRemoval(r vocab.DomainRecording) (FileChange, error) {
	previous, existed := m.files[r.Path()]
	return &memoryChange{
		failApply: m.failRemove,
		apply: func() {
			m.removed = append(m.removed, r.Path())
			delete(m.files, r.Path())
		},
		revert: func() {
			m.removed = m.removed[:len(m.removed)-1]
			if existed {
				m.files[r.Path()] = previous
			}
		},
	}, nil
}

// memorySetting keeps what the ruleset names: the path of each
// DomainRecording, the empty string for the zero DomainRecording, which
// names none. checkErr refuses the edit while it is planned; err fails
// the write itself.
type memorySetting struct {
	named    []string
	checkErr error
	err      error
}

func (m *memorySetting) PlanDomainRecordingName(r vocab.DomainRecording) (FileChange, bool, error) {
	if m.checkErr != nil {
		return nil, false, m.checkErr
	}
	return &memoryChange{
		failApply: m.err,
		apply:     func() { m.named = append(m.named, r.Path()) },
		revert:    func() { m.named = m.named[:len(m.named)-1] },
	}, true, nil
}

func convertFixture(t *testing.T) (*recordedFile, *memoryRecordings) {
	t.Helper()
	source, err := vocab.NewDomainRecording("domain.arclint.yaml")
	if err != nil {
		t.Fatal(err)
	}
	lang, err := vocab.NewUbiquitousLanguage("boxoffice", "Shows on sale.", []vocab.BoundedContext{{
		Name: "ordering", Definition: "The deal as struck.",
		Aggregates: []vocab.Aggregate{{Name: "Order", Definition: "The deal.", Identity: "OrderID"}},
	}}, nil)
	if err != nil {
		t.Fatal(err)
	}
	project := &recordedFile{recording: source, lang: lang, found: true}
	return project, &memoryRecordings{files: map[string]*recordedFile{source.Path(): project}}
}

func TestExportWritesAndProvesTheTranslation(t *testing.T) {
	project, recordings := convertFixture(t)
	export, err := NewExportDomainRecording(project, recordings)
	if err != nil {
		t.Fatal(err)
	}
	result, err := export.Execute("domain.arclint.cml")
	if err != nil {
		t.Fatal(err)
	}
	if result.Source.Path() != "domain.arclint.yaml" || result.Target.Path() != "domain.arclint.cml" || result.Updated {
		t.Fatalf("result = %+v", result)
	}
	if result.Counts.Aggregates != 1 || result.Counts.Contexts != 1 {
		t.Fatalf("counts = %+v", result.Counts)
	}
	written := recordings.files["domain.arclint.cml"]
	if written == nil || written.lang.Difference(project.lang) != "" {
		t.Fatalf("the target does not record the project's language")
	}
	if len(recordings.removed) != 0 {
		t.Fatalf("export removed %v; only convert removes the previous file", recordings.removed)
	}
}

func TestExportRefusesALossyTranslation(t *testing.T) {
	project, recordings := convertFixture(t)
	target, err := vocab.NewDomainRecording("domain.arclint.cml")
	if err != nil {
		t.Fatal(err)
	}
	recordings.files[target.Path()] = &recordedFile{recording: target, lossy: func(l vocab.UbiquitousLanguage) vocab.UbiquitousLanguage {
		l.Description = ""
		return l
	}}
	export, err := NewExportDomainRecording(project, recordings)
	if err != nil {
		t.Fatal(err)
	}
	lossy := recordings.files[target.Path()]
	_, err = export.Execute(target.Path())
	if err == nil || !strings.Contains(err.Error(), "would not record the same language") || !strings.Contains(err.Error(), "description differs") || !strings.Contains(err.Error(), "nothing was written") {
		t.Fatalf("err = %v, want the difference named and nothing written", err)
	}
	if recordings.files[target.Path()] != lossy || lossy.records != 0 {
		t.Fatal("the target was written although the translation does not read back the same")
	}
}

func TestExportNeverOverwritesAnUnreadableTarget(t *testing.T) {
	project, recordings := convertFixture(t)
	target, err := vocab.NewDomainRecording("model.cml")
	if err != nil {
		t.Fatal(err)
	}
	broken := &recordedFile{recording: target, broken: errors.New("line 3: unexpected }")}
	recordings.files[target.Path()] = broken
	export, err := NewExportDomainRecording(project, recordings)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := export.Execute(target.Path()); err == nil || !strings.Contains(err.Error(), "left untouched") {
		t.Fatalf("err = %v, want the unreadable target left untouched", err)
	}
	if broken.records != 0 {
		t.Fatal("the unreadable target was written")
	}
}

func TestExportRefusesUsageMistakes(t *testing.T) {
	project, recordings := convertFixture(t)
	export, err := NewExportDomainRecording(project, recordings)
	if err != nil {
		t.Fatal(err)
	}
	for _, target := range []string{"domain.arclint.yaml", "model.json", "../outside.cml"} {
		if _, err := export.Execute(target); !errors.Is(err, ErrDomainUsage) {
			t.Fatalf("Execute(%q) err = %v, want a usage error", target, err)
		}
	}
	project.found = false
	if _, err := export.Execute("domain.arclint.cml"); !errors.Is(err, ErrDomainUsage) || !strings.Contains(err.Error(), "no recorded Ubiquitous Language") {
		t.Fatalf("err = %v, want nothing to translate", err)
	}
}

func TestConvertMovesTheProjectToTheNewRecording(t *testing.T) {
	project, recordings := convertFixture(t)
	export, err := NewExportDomainRecording(project, recordings)
	if err != nil {
		t.Fatal(err)
	}
	setting := &memorySetting{}
	convert, err := NewConvertDomainRecording(export, recordings, setting)
	if err != nil {
		t.Fatal(err)
	}
	result, err := convert.Execute("domain.arclint.cml")
	if err != nil {
		t.Fatal(err)
	}
	if !result.RulesetChanged || !result.Named.IsZero() {
		t.Fatalf("result = %+v", result)
	}
	if len(recordings.removed) != 1 || recordings.removed[0] != "domain.arclint.yaml" {
		t.Fatalf("removed %v, want the previous file", recordings.removed)
	}
}

// projectRecordedIn is a project whose language is recorded in previous,
// with a stale file beside it for each name in beside.
func projectRecordedIn(t *testing.T, previous string, beside ...string) (*recordedFile, *memoryRecordings) {
	t.Helper()
	_, template := convertFixture(t)
	lang := template.files["domain.arclint.yaml"].lang
	recording, err := vocab.NewDomainRecording(previous)
	if err != nil {
		t.Fatal(err)
	}
	project := &recordedFile{recording: recording, lang: lang, found: true}
	recordings := &memoryRecordings{files: map[string]*recordedFile{previous: project}}
	for _, name := range beside {
		stale, err := vocab.NewDomainRecording(name)
		if err != nil {
			t.Fatal(err)
		}
		recordings.files[name] = &recordedFile{recording: stale, found: true}
	}
	return project, recordings
}

// What the ruleset is asked to name after a conversion depends on the
// files that will exist once the previous file is gone: nothing when
// discovery finds the target by itself, the target otherwise.
func TestConvertNamesTheTargetOnlyWhenDiscoveryWouldNotFindIt(t *testing.T) {
	for _, tc := range []struct {
		name     string
		previous string
		beside   []string
		target   string
		named    string
	}{
		{"YAML to CML, nothing else", "domain.arclint.yaml", nil, "domain.arclint.cml", ""},
		{"YAML to .yml, nothing else", "domain.arclint.yaml", nil, "domain.arclint.yml", ""},
		{"CML back to YAML", "domain.arclint.cml", nil, "domain.arclint.yaml", ""},
		{"a name outside the default names", "domain.arclint.yaml", nil, "docs/model.cml", "docs/model.cml"},
		{"a stale .yml that discovery would pick over the target", "model/domain.yaml", []string{"domain.arclint.yml"}, "domain.arclint.cml", "domain.arclint.cml"},
		{"a stale .yaml that discovery would pick over the target", "docs/model.cml", []string{"domain.arclint.yaml"}, "domain.arclint.yml", "domain.arclint.yml"},
		{"a stale .yml that remains after the previous .yaml is removed", "domain.arclint.yaml", []string{"domain.arclint.yml"}, "domain.arclint.cml", "domain.arclint.cml"},
		{"a stale .cml that the target outranks", "model/domain.yaml", []string{"domain.arclint.cml"}, "domain.arclint.yml", ""},
		{"a stale .yml that the target outranks", "model/domain.yaml", []string{"domain.arclint.yml"}, "domain.arclint.yaml", ""},
	} {
		t.Run(tc.name, func(t *testing.T) {
			project, recordings := projectRecordedIn(t, tc.previous, tc.beside...)
			export, err := NewExportDomainRecording(project, recordings)
			if err != nil {
				t.Fatal(err)
			}
			setting := &memorySetting{}
			convert, err := NewConvertDomainRecording(export, recordings, setting)
			if err != nil {
				t.Fatal(err)
			}
			result, err := convert.Execute(tc.target)
			if err != nil {
				t.Fatal(err)
			}
			if len(setting.named) != 1 || setting.named[0] != tc.named || result.Named.Path() != tc.named {
				t.Fatalf("the ruleset was asked to name %q, the result says %q; want %q", setting.named, result.Named.Path(), tc.named)
			}
			if len(recordings.removed) != 1 || recordings.removed[0] != tc.previous {
				t.Fatalf("removed %v, want only %s", recordings.removed, tc.previous)
			}
		})
	}
}

// A previous file that is itself a default name is gone by then, so it
// cannot make discovery choose it over the target.
func TestConvertDoesNotLetThePreviousFileOutrankTheTarget(t *testing.T) {
	project, recordings := convertFixture(t)
	export, err := NewExportDomainRecording(project, recordings)
	if err != nil {
		t.Fatal(err)
	}
	setting := &memorySetting{}
	convert, err := NewConvertDomainRecording(export, recordings, setting)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := convert.Execute("domain.arclint.cml"); err != nil {
		t.Fatal(err)
	}
	if len(setting.named) != 1 || setting.named[0] != "" {
		t.Fatalf("the ruleset was asked to name %q, want none: domain.arclint.yaml is removed, so discovery finds the target", setting.named)
	}
}

func TestConvertKeepsEverythingWhenItCannotTellWhichFilesExist(t *testing.T) {
	project, recordings := convertFixture(t)
	export, err := NewExportDomainRecording(project, recordings)
	if err != nil {
		t.Fatal(err)
	}
	setting := &memorySetting{}
	convert, err := NewConvertDomainRecording(export, recordings, setting)
	if err != nil {
		t.Fatal(err)
	}
	recordings.existsErr = errors.New("permission denied")
	_, err = convert.Execute("domain.arclint.cml")
	if err == nil || !strings.Contains(err.Error(), "permission denied") || !strings.Contains(err.Error(), "nothing was written") {
		t.Fatalf("err = %v", err)
	}
	if recordings.files["domain.arclint.cml"] != nil || len(setting.named) != 0 || len(recordings.removed) != 0 {
		t.Fatalf("wrote %v, named %v and removed %v although the files could not be checked", recordings.files["domain.arclint.cml"], setting.named, recordings.removed)
	}
}

// A ruleset that cannot be edited to name the new file is found out
// before the language is written anywhere, so no stray file is left.
func TestConvertWritesNothingWhenTheRulesetCannotBeEdited(t *testing.T) {
	project, recordings := convertFixture(t)
	export, err := NewExportDomainRecording(project, recordings)
	if err != nil {
		t.Fatal(err)
	}
	setting := &memorySetting{checkErr: errors.New(`unknown key "bogus_key"`)}
	convert, err := NewConvertDomainRecording(export, recordings, setting)
	if err != nil {
		t.Fatal(err)
	}
	_, err = convert.Execute("docs/model.cml")
	if err == nil || !strings.Contains(err.Error(), "bogus_key") || !strings.Contains(err.Error(), "nothing was written") {
		t.Fatalf("err = %v", err)
	}
	if recordings.files["docs/model.cml"] != nil || len(setting.named) != 0 || len(recordings.removed) != 0 {
		t.Fatalf("wrote %v, named %v and removed %v although the ruleset cannot be edited", recordings.files["docs/model.cml"], setting.named, recordings.removed)
	}
}

// convertFailingAt converts the fixture project to domain.arclint.cml
// with one write of the move failing, and requires the error to say so
// and every file to be as it was before.
func convertFailingAt(t *testing.T, fail func(*memoryRecordings, *memorySetting)) {
	t.Helper()
	project, recordings := convertFixture(t)
	export, err := NewExportDomainRecording(project, recordings)
	if err != nil {
		t.Fatal(err)
	}
	setting := &memorySetting{}
	fail(recordings, setting)
	convert, err := NewConvertDomainRecording(export, recordings, setting)
	if err != nil {
		t.Fatal(err)
	}
	_, err = convert.Execute("domain.arclint.cml")
	if err == nil || !strings.Contains(err.Error(), "refused") || !strings.Contains(err.Error(), "nothing was changed") {
		t.Fatalf("err = %v, want the failure and that nothing was changed", err)
	}
	if recordings.files["domain.arclint.cml"] != nil {
		t.Fatal("the target was left written")
	}
	if recordings.files["domain.arclint.yaml"] != project || len(recordings.removed) != 0 {
		t.Fatalf("the previous file was left removed: %v", recordings.removed)
	}
	if len(setting.named) != 0 {
		t.Fatalf("the ruleset was left naming %v", setting.named)
	}
}

// A move is all or nothing: whichever write fails, the ones before it
// are put back.
func TestConvertChangesNothingWhenAWriteFails(t *testing.T) {
	refused := errors.New("refused")
	t.Run("the target", func(t *testing.T) {
		convertFailingAt(t, func(r *memoryRecordings, _ *memorySetting) {
			r.failWrite = map[string]error{"domain.arclint.cml": refused}
		})
	})
	t.Run("the ruleset", func(t *testing.T) {
		convertFailingAt(t, func(_ *memoryRecordings, s *memorySetting) { s.err = refused })
	})
	t.Run("removing the previous file", func(t *testing.T) {
		convertFailingAt(t, func(r *memoryRecordings, _ *memorySetting) { r.failRemove = refused })
	})
}

// failingRevert is a change whose undo fails too.
type failingRevert struct{ memoryChange }

func (failingRevert) Revert() error { return errors.New("cannot put it back") }

// When putting a written file back fails as well, the error says the
// files are left part way and names both failures.
func TestApplyTogetherReportsAFailedUndo(t *testing.T) {
	written := &failingRevert{memoryChange{apply: func() {}}}
	refused := &memoryChange{failApply: errors.New("refused")}
	err := applyTogether(written, refused)
	if err == nil || !strings.Contains(err.Error(), "refused") || !strings.Contains(err.Error(), "cannot put it back") || !strings.Contains(err.Error(), "left part way") {
		t.Fatalf("err = %v", err)
	}
}
