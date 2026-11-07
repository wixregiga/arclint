package application_test

import (
	"errors"
	"strings"
	"testing"
	"time"

	"github.com/wixregiga/arclint/internal/application"
	"github.com/wixregiga/arclint/internal/domain/conformance"
	"github.com/wixregiga/arclint/internal/domain/rule"
	"github.com/wixregiga/arclint/internal/domain/vocab"
)

// fakeSourceFiles serves file contents and records the limit asked.
type fakeSourceFiles struct {
	files map[string]string
	limit int64
	reads []string
}

func (f *fakeSourceFiles) ReadSource(path string, limit int64) ([]byte, error) {
	f.limit = limit
	f.reads = append(f.reads, path)
	content, ok := f.files[path]
	if !ok {
		return nil, errors.New("no such file")
	}
	return []byte(content), nil
}

// excerptEvidence assesses a project whose analyzed Go files are the
// given paths, plus a manifest no language analyzes, and returns its
// evidence: the snapshot excerpts are read against.
func excerptEvidence(t *testing.T, analyzed ...string) application.DomainEvidence {
	t.Helper()
	files := []conformance.ObservedFile{{Path: "go.mod"}}
	facts := map[string]conformance.LanguageFacts{}
	for _, p := range analyzed {
		files = append(files, conformance.ObservedFile{Path: p})
		facts[p] = conformance.LanguageFacts{Language: rule.LanguageGo, Package: "catalog", DeclarationsAvailable: true}
	}
	obs, err := conformance.NewObservations(files, facts)
	if err != nil {
		t.Fatal(err)
	}
	lang, err := vocab.NewUbiquitousLanguage("shop", "", []vocab.BoundedContext{{Name: "catalog", Definition: "What is on sale."}}, nil)
	if err != nil {
		t.Fatal(err)
	}
	observations := &fakeObservations{obs: obs}
	assess, err := application.NewAssessConformance(fakeRepository{cfg: rule.Configured{Languages: []rule.Language{rule.LanguageGo}}}, observations, &fakeBaselineSource{}, nil, &fakeKnowledge{lang: lang, found: true})
	if err != nil {
		t.Fatal(err)
	}
	at := time.Date(2026, 11, 7, 12, 0, 0, 0, time.UTC)
	uc, err := application.NewAssessDomainEvidence(func() (application.AssessConformance, error) { return assess, nil }, func() time.Time { return at })
	if err != nil {
		t.Fatal(err)
	}
	evidence, err := uc.Execute()
	if err != nil {
		t.Fatal(err)
	}
	if observations.calls != 1 {
		t.Fatalf("the assessment observed the repository %d times, want once", observations.calls)
	}
	declarations := false
	for _, f := range observations.facts {
		declarations = declarations || f == rule.FactDeclarations
	}
	if !declarations {
		t.Fatalf("the assessment observed %v without declarations", observations.facts)
	}
	return evidence
}

func readExcerpt(t *testing.T, files map[string]string, path string, line int) (application.SourceExcerpt, *fakeSourceFiles, error) {
	t.Helper()
	paths := make([]string, 0, len(files))
	for p := range files {
		if p != "go.mod" {
			paths = append(paths, p)
		}
	}
	evidence := excerptEvidence(t, paths...)
	source := &fakeSourceFiles{files: files}
	uc, err := application.NewReadSourceExcerpt(source)
	if err != nil {
		t.Fatal(err)
	}
	got, err := uc.Execute(evidence, path, line)
	return got, source, err
}

func numbered(n int) string {
	lines := make([]string, n)
	for i := range lines {
		lines[i] = "line " + strings.Repeat("x", i%3)
	}
	return strings.Join(lines, "\n") + "\n"
}

func TestExcerptReadsAFewBoundedLinesAroundTheLocation(t *testing.T) {
	long := strings.Repeat("é", application.ExcerptLineLimit+50)
	content := numbered(30) + long + "\r\nbad \xff byte\r\n"
	got, source, err := readExcerpt(t, map[string]string{"catalog/event.go": content}, "catalog/event.go", 31)
	if err != nil {
		t.Fatal(err)
	}
	if source.limit != application.ExcerptFileLimit {
		t.Errorf("read limit %d, want %d", source.limit, application.ExcerptFileLimit)
	}
	if got.TotalLines != 32 || got.Line != 31 || got.Lines[0].Number != 31-application.ExcerptRadius || got.Lines[len(got.Lines)-1].Number != 32 {
		t.Fatalf("excerpt %d..%d of %d around %d", got.Lines[0].Number, got.Lines[len(got.Lines)-1].Number, got.TotalLines, got.Line)
	}
	target := got.Lines[len(got.Lines)-2]
	if !target.Truncated || len([]rune(target.Text)) != application.ExcerptLineLimit {
		t.Errorf("a long line is cut to %d characters and marked: truncated %v, %d characters", application.ExcerptLineLimit, target.Truncated, len([]rune(target.Text)))
	}
	if last := got.Lines[len(got.Lines)-1].Text; last != "bad � byte" {
		t.Errorf("invalid UTF-8 and CRLF: %q", last)
	}
}

func TestExcerptRefusesWhatTheSnapshotDidNotAnalyze(t *testing.T) {
	files := map[string]string{"catalog/event.go": "package catalog\n", "go.mod": "module shop\n"}
	for path, want := range map[string]error{
		"go.mod":               application.ErrExcerptOutsideEvidence,
		"catalog/other.go":     application.ErrExcerptOutsideEvidence,
		"../catalog/event.go":  application.ErrExcerptRequestInvalid,
		"/etc/passwd":          application.ErrExcerptRequestInvalid,
		"catalog//event.go":    application.ErrExcerptRequestInvalid,
		`catalog\event.go`:     application.ErrExcerptRequestInvalid,
		"catalog/event.go\x00": application.ErrExcerptRequestInvalid,
	} {
		_, source, err := readExcerpt(t, files, path, 1)
		if !errors.Is(err, want) {
			t.Errorf("%q: %v, want %v", path, err, want)
		}
		if len(source.reads) != 0 {
			t.Errorf("%q: read %v before refusing", path, source.reads)
		}
	}
	if _, _, err := readExcerpt(t, files, "catalog/event.go", 0); !errors.Is(err, application.ErrExcerptRequestInvalid) {
		t.Errorf("line 0: %v", err)
	}
	if _, _, err := readExcerpt(t, files, "catalog/event.go", 2); !errors.Is(err, application.ErrExcerptSourceUnreadable) {
		t.Errorf("beyond the end: %v", err)
	}
	if _, _, err := readExcerpt(t, map[string]string{"catalog/blob.go": "pack\x00age"}, "catalog/blob.go", 1); !errors.Is(err, application.ErrExcerptSourceUnreadable) {
		t.Errorf("binary: %v", err)
	}
}

// An analyzed source file can still hold a secret: its name or its
// content withholds it whole, before any line is shown.
func TestExcerptWithholdsFilesThatMayHoldSecrets(t *testing.T) {
	for _, path := range []string{"catalog/secrets.ts", "catalog/client_secret.go", "config/.env.ts", "catalog/.env", "deploy/.ssh/keys.go", "secrets/db.go", "catalog/server.pem", "catalog/id_rsa.go", "deploy/credentials.json", "deploy/service-account.ts"} {
		_, source, err := readExcerpt(t, map[string]string{path: "package catalog\n"}, path, 1)
		if !errors.Is(err, application.ErrExcerptWithheld) {
			t.Errorf("%s: %v, want withheld", path, err)
		}
		if len(source.reads) != 0 {
			t.Errorf("%s: read before withholding by name", path)
		}
	}
	for name, content := range map[string]string{
		"private key":  "const k = `-----BEGIN OPENSSH PRIVATE KEY-----\nb3Blbn\n-----END OPENSSH PRIVATE KEY-----`\n",
		"aws key":      "var id = \"AKIAIOSFODNN7EXAMPLE\"\n",
		"github token": "token := \"ghp_" + strings.Repeat("a1", 18) + "\"\n",
		// Assembled here so no token literal sits in this file.
		"jwt":               "const t = \"" + strings.Join([]string{"eyJ" + strings.Repeat("hb", 8), "eyJ" + strings.Repeat("zd", 8), strings.Repeat("Qx", 10)}, ".") + "\"\n",
		"assignment":        "var dbPassword = \"correct-horse-battery\"\n",
		"yaml style":        "api_key: 'abcdefghijklmnop'\n",
		"go typed var":      "var dbPassword string = \"correct-horse-battery\"\n",
		"go typed const":    "const apiToken string = \"correct-horse-battery\"\n",
		"ts annotated":      "const clientSecret: string = 'correct-horse-battery';\n",
		"py annotated":      "api_key: str = \"correct-horse-battery\"\n",
		"go struct field":   "cfg := Config{Password: \"correct-horse-battery\"}\n",
		"spaced passphrase": "var dbPassword string = \"correct horse battery staple\"\n",
		"spaced ts":         "const secret = 'open sesame please';\n",
	} {
		_, _, err := readExcerpt(t, map[string]string{"catalog/config.go": content}, "catalog/config.go", 1)
		if !errors.Is(err, application.ErrExcerptWithheld) {
			t.Errorf("%s: %v, want withheld", name, err)
		}
	}
	// Domain words are not secrets: a file named for a password value
	// object, a short literal, and a reference to a credential by name.
	for path, content := range map[string]string{
		"catalog/password.go":    "package catalog\n\ntype Password struct{ hash string }\n",
		"catalog/credentials.go": "package catalog\n\nconst token = \"short\"\nvar secretName = os.Getenv(\"DB_PASSWORD\")\n",
	} {
		if _, _, err := readExcerpt(t, map[string]string{path: content}, path, 1); err != nil {
			t.Errorf("%s: %v, want an excerpt", path, err)
		}
	}
}
