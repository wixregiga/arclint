package vocab_test

import (
	"errors"
	"strings"
	"testing"

	"github.com/wixregiga/arclint/internal/domain/vocab"
)

func TestNewDomainRecordingNamesTheFormatByTheExtension(t *testing.T) {
	for _, tc := range []struct {
		file   string
		path   string
		format vocab.DomainRecordingFormat
	}{
		{"domain.arclint.yaml", "domain.arclint.yaml", vocab.DomainRecordingYAML},
		{"model.yml", "model.yml", vocab.DomainRecordingYAML},
		{"docs/model.cml", "docs/model.cml", vocab.DomainRecordingCML},
		{"a/b/c/domain.model.cml", "a/b/c/domain.model.cml", vocab.DomainRecordingCML},
		{"x.yaml", "x.yaml", vocab.DomainRecordingYAML},
		{"my model (v2).yaml", "my model (v2).yaml", vocab.DomainRecordingYAML},
		{"..yaml", "..yaml", vocab.DomainRecordingYAML},
		{".hidden/model.yaml", ".hidden/model.yaml", vocab.DomainRecordingYAML},
	} {
		got, err := vocab.NewDomainRecording(tc.file)
		if err != nil {
			t.Errorf("NewDomainRecording(%q): %v", tc.file, err)
			continue
		}
		if got.Path() != tc.path || got.Format() != tc.format || got.IsZero() {
			t.Errorf("NewDomainRecording(%q) = %q (%s), want %q (%s)", tc.file, got.Path(), got.Format(), tc.path, tc.format)
		}
	}
}

func TestNewDomainRecordingRefusesAnExtensionThatNamesNoFormat(t *testing.T) {
	for _, file := range []string{
		"domain.txt",
		"domain.json",
		"domain",
		"domain.",
		"domain.YAML",
		"domain.Yml",
		"domain.CML",
		"domain.yaml.bak",
		"domain.yaml ",
		"domain.xml",
		".yaml",
		".cml",
		"docs/.yml",
		"docs/",
		"docs/model.yaml/",
		"docs/model.yaml/.",
		"docs/model.yaml/..",
		"docs/model.yaml/../.cml",
		".",
	} {
		_, err := vocab.NewDomainRecording(file)
		if err == nil {
			t.Errorf("NewDomainRecording(%q) accepted a name whose extension names no format", file)
			continue
		}
		if !strings.HasPrefix(err.Error(), "DomainRecording format-named-by-extension: ") {
			t.Errorf("NewDomainRecording(%q) error = %q, want the format-named-by-extension invariant named", file, err)
		}
	}
}

func TestNewDomainRecordingStaysInsideTheRepository(t *testing.T) {
	for _, tc := range []struct {
		name string
		file string
	}{
		{"empty", ""},
		{"absolute Unix", "/etc/domain.yaml"},
		{"absolute Unix at the root", "/domain.yaml"},
		{"network share", "//server/share/domain.yaml"},
		{"absolute Windows", `C:\models\domain.yaml`},
		{"absolute Windows with slashes", "C:/models/domain.yaml"},
		{"drive letter", "c:domain.yaml"},
		{"drive letter, upper case", "D:domain.yaml"},
		{"backslash separator", `docs\model.yaml`},
		{"leading backslash", `\domain.yaml`},
		{"backslash escape", `..\domain.yaml`},
		{"backslash after a slash", `docs/sub\model.cml`},
		{"parent", "../domain.yaml"},
		{"parent only", ".."},
		{"parent directory", "../"},
		{"parent of parent", "../../domain.yaml"},
		{"parent behind the current directory", "./../domain.yaml"},
		{"descends then climbs out", "a/../../b.yaml"},
		{"descends then climbs out twice", "a/b/../../../b.yaml"},
	} {
		_, err := vocab.NewDomainRecording(tc.file)
		if err == nil {
			t.Errorf("%s: NewDomainRecording(%q) accepted a path outside the repository", tc.name, tc.file)
			continue
		}
		if !strings.HasPrefix(err.Error(), "DomainRecording inside-the-repository: ") {
			t.Errorf("%s: NewDomainRecording(%q) error = %q, want the inside-the-repository invariant named", tc.name, tc.file, err)
		}
	}
}

func TestNewDomainRecordingCleansThePath(t *testing.T) {
	for file, want := range map[string]string{
		"./domain.arclint.yaml":  "domain.arclint.yaml",
		"docs//model.yaml":       "docs/model.yaml",
		"docs/./model.yaml":      "docs/model.yaml",
		"docs/../model.cml":      "model.cml",
		"a/b/../c/model.yml":     "a/c/model.yml",
		"./a/../a/model.yaml":    "a/model.yaml",
		"docs/sub/../../m.yaml":  "m.yaml",
		"colon/in:the/name.yaml": "colon/in:the/name.yaml",
		"ab:cd.yaml":             "ab:cd.yaml",
		"1:domain.yaml":          "1:domain.yaml",
	} {
		got, err := vocab.NewDomainRecording(file)
		if err != nil {
			t.Errorf("NewDomainRecording(%q): %v", file, err)
			continue
		}
		if got.Path() != want {
			t.Errorf("NewDomainRecording(%q).Path() = %q, want %q", file, got.Path(), want)
		}
	}
}

func TestDefaultDomainRecordingIsTheYAMLFileBesideTheRuleset(t *testing.T) {
	got := mustDefaultRecording(t)
	if got.Path() != vocab.DefaultDomainRecordingPath || got.Path() != "domain.arclint.yaml" || got.Format() != vocab.DomainRecordingYAML || got.IsZero() {
		t.Errorf("mustDefaultRecording(t) = %q (%s)", got.Path(), got.Format())
	}
	viaConstructor, err := vocab.NewDomainRecording(vocab.DefaultDomainRecordingPath)
	if err != nil || viaConstructor != got {
		t.Errorf("NewDomainRecording(default path) = %+v, %v; want the default DomainRecording", viaConstructor, err)
	}
}

func TestDefaultDomainRecordingsAreTheThreeDefaultNamesYAMLFirst(t *testing.T) {
	got := vocab.DefaultDomainRecordings()
	want := []string{"domain.arclint.yaml", "domain.arclint.yml", "domain.arclint.cml"}
	if len(got) != len(want) {
		t.Fatalf("DefaultDomainRecordings() = %v, want %v", paths(got), want)
	}
	for i, file := range want {
		viaConstructor, err := vocab.NewDomainRecording(file)
		if err != nil {
			t.Fatal(err)
		}
		if got[i] != viaConstructor {
			t.Errorf("DefaultDomainRecordings()[%d] = %+v, want what NewDomainRecording(%q) builds: %+v", i, got[i], file, viaConstructor)
		}
	}
	if got[0] != mustDefaultRecording(t) {
		t.Errorf("the first default name is %q, want the default DomainRecording", got[0].Path())
	}
	got[0] = vocab.DomainRecording{}
	if vocab.DefaultDomainRecordings()[0].IsZero() {
		t.Errorf("DefaultDomainRecordings() hands out its own storage")
	}
}

func paths(recordings []vocab.DomainRecording) []string {
	out := make([]string, 0, len(recordings))
	for _, r := range recordings {
		out = append(out, r.Path())
	}
	return out
}

func recordingOf(t *testing.T, file string) vocab.DomainRecording {
	t.Helper()
	if file == "" {
		return vocab.DomainRecording{}
	}
	r, err := vocab.NewDomainRecording(file)
	if err != nil {
		t.Fatal(err)
	}
	return r
}

// existingFiles is an existence check over the given files, recording
// every file it was asked about.
func existingFiles(t *testing.T, asked *[]string, files ...string) func(vocab.DomainRecording) (bool, error) {
	t.Helper()
	have := map[string]bool{}
	for _, f := range files {
		have[f] = true
	}
	return func(r vocab.DomainRecording) (bool, error) {
		*asked = append(*asked, r.Path())
		return have[r.Path()], nil
	}
}

func TestResolveDomainRecordingChoosesTheFirstDefaultNameThatExists(t *testing.T) {
	const yaml, yml, cml = "domain.arclint.yaml", "domain.arclint.yml", "domain.arclint.cml"
	for _, tc := range []struct {
		name    string
		exist   []string
		chosen  string
		ignored []string
	}{
		{"none exists", nil, yaml, nil},
		{"only .yaml", []string{yaml}, yaml, nil},
		{"only .yml", []string{yml}, yml, nil},
		{"only .cml", []string{cml}, cml, nil},
		{".yaml and .yml", []string{yaml, yml}, yaml, []string{yml}},
		{".yaml and .cml", []string{yaml, cml}, yaml, []string{cml}},
		{".yml and .cml", []string{yml, cml}, yml, []string{cml}},
		{"all three", []string{yaml, yml, cml}, yaml, []string{yml, cml}},
		{"all three, listed in another order", []string{cml, yml, yaml}, yaml, []string{yml, cml}},
		{"a file that is not a default name does not count", []string{"docs/model.cml", "domain.cml", "domain.arclint.json"}, yaml, nil},
	} {
		t.Run(tc.name, func(t *testing.T) {
			var asked []string
			chosen, ignored, err := vocab.ResolveDomainRecording(vocab.DomainRecording{}, existingFiles(t, &asked, tc.exist...))
			if err != nil {
				t.Fatal(err)
			}
			if chosen != recordingOf(t, tc.chosen) {
				t.Errorf("chosen = %q, want %q", chosen.Path(), tc.chosen)
			}
			if strings.Join(paths(ignored), " ") != strings.Join(tc.ignored, " ") {
				t.Errorf("ignored = %v, want %v", paths(ignored), tc.ignored)
			}
			if strings.Join(asked, " ") != strings.Join(paths(vocab.DefaultDomainRecordings()), " ") {
				t.Errorf("asked about %v, want exactly the default names in order", asked)
			}
		})
	}
}

// A file the ruleset names is the project's own choice: no discovery
// takes place, so no file is asked about and none is passed over, even
// when every default name exists beside it.
func TestResolveDomainRecordingChoosesTheNamedFileWithoutDiscovery(t *testing.T) {
	const yaml, yml, cml = "domain.arclint.yaml", "domain.arclint.yml", "domain.arclint.cml"
	subsets := [][]string{nil, {yaml}, {yml}, {cml}, {yaml, yml}, {yaml, cml}, {yml, cml}, {yaml, yml, cml}}
	for _, named := range []string{"docs/model.cml", "docs/model.yaml", "model.yml", yaml, yml, cml} {
		for _, exist := range subsets {
			var asked []string
			chosen, ignored, err := vocab.ResolveDomainRecording(recordingOf(t, named), existingFiles(t, &asked, exist...))
			if err != nil {
				t.Fatal(err)
			}
			if chosen != recordingOf(t, named) {
				t.Errorf("named %q with %v: chosen = %q", named, exist, chosen.Path())
			}
			if len(ignored) != 0 {
				t.Errorf("named %q with %v: ignored = %v, want none", named, exist, paths(ignored))
			}
			if len(asked) != 0 {
				t.Errorf("named %q: asked about %v, want no question", named, asked)
			}
		}
	}
}

func TestResolveDomainRecordingChoosesANamedFileThatDoesNotExist(t *testing.T) {
	var asked []string
	chosen, ignored, err := vocab.ResolveDomainRecording(recordingOf(t, "docs/model.cml"), existingFiles(t, &asked))
	if err != nil || chosen.Path() != "docs/model.cml" || len(ignored) != 0 {
		t.Errorf("ResolveDomainRecording = %q, %v, %v; want the named file, nothing ignored", chosen.Path(), paths(ignored), err)
	}
}

func TestResolveDomainRecordingReportsWhatItCouldNotCheck(t *testing.T) {
	boom := errors.New("permission denied")
	_, _, err := vocab.ResolveDomainRecording(vocab.DomainRecording{}, func(r vocab.DomainRecording) (bool, error) {
		if r.Path() == "domain.arclint.yml" {
			return false, boom
		}
		return false, nil
	})
	if !errors.Is(err, boom) || !strings.Contains(err.Error(), "domain.arclint.yml") {
		t.Errorf("err = %v, want the failure wrapped with the file it was about", err)
	}
	if _, _, err := vocab.ResolveDomainRecording(vocab.DomainRecording{}, nil); err == nil {
		t.Errorf("ResolveDomainRecording without an existence check succeeded")
	}
}

func TestTheZeroDomainRecordingNamesNoFile(t *testing.T) {
	var zero vocab.DomainRecording
	if !zero.IsZero() || zero.Path() != "" || zero.Format() != "" {
		t.Errorf("zero DomainRecording = %q (%s), want no file", zero.Path(), zero.Format())
	}
	if !(vocab.UbiquitousLanguage{Recording: mustDefaultRecording(t)}).Empty() {
		t.Errorf("a language that only carries a DomainRecording records something")
	}
	lang, err := vocab.NewUbiquitousLanguage("shop", "", nil, nil)
	if err != nil {
		t.Fatalf("NewUbiquitousLanguage: %v", err)
	}
	if !lang.Recording.IsZero() {
		t.Errorf("a constructed language carries %q; constructors leave the DomainRecording zero", lang.Recording.Path())
	}
}

func TestDomainRecordingExtensionsListTheFormatsExactly(t *testing.T) {
	got := strings.Join(vocab.DomainRecordingExtensions(), " ")
	if got != ".cml .yaml .yml" {
		t.Errorf("DomainRecordingExtensions() = %q, want \".cml .yaml .yml\"", got)
	}
}

func TestDefineAndRemoveKeepWhereTheLanguageIsRecorded(t *testing.T) {
	lang := mustTicketing(t)
	recording, err := vocab.NewDomainRecording("docs/model.cml")
	if err != nil {
		t.Fatalf("NewDomainRecording: %v", err)
	}
	lang.Recording = recording
	defined, _, err := lang.Define(vocab.ConceptDomainService, vocab.Locator{Context: "sales", Name: "Seating"}, def("Assigns seats."))
	if err != nil {
		t.Fatalf("Define: %v", err)
	}
	if defined.Recording != recording {
		t.Errorf("Define dropped the DomainRecording: %+v", defined.Recording)
	}
	removed, _, err := defined.Remove(vocab.ConceptDomainService, vocab.Locator{Context: "sales", Name: "Seating"})
	if err != nil {
		t.Fatalf("Remove: %v", err)
	}
	if removed.Recording != recording {
		t.Errorf("Remove dropped the DomainRecording: %+v", removed.Recording)
	}
}

func mustDefaultRecording(t *testing.T) vocab.DomainRecording {
	t.Helper()
	recording, err := vocab.DefaultDomainRecording()
	if err != nil {
		t.Fatal(err)
	}
	return recording
}
