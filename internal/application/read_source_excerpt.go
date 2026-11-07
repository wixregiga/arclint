package application

import (
	"bytes"
	"errors"
	"fmt"
	"path"
	"regexp"
	"strings"
	"unicode/utf8"
)

// SourceFiles reads files of the repository for an excerpt. The adapter
// confines every read to the repository: a path that leaves it, by name
// or through a link, is refused, and so is a file larger than limit
// bytes or one that is not a regular file.
type SourceFiles interface {
	ReadSource(path string, limit int64) ([]byte, error)
}

// Refusals of an excerpt. A delivery maps each to its own answer.
var (
	// ErrExcerptOutsideEvidence means the snapshot did not analyze the
	// file as source, so no excerpt of it is read.
	ErrExcerptOutsideEvidence = errors.New("the file is not source this assessment analyzed")
	// ErrExcerptRequestInvalid means the path or line cannot name a
	// line of a repository file.
	ErrExcerptRequestInvalid = errors.New("the excerpt request does not name a line of a repository file")
	// ErrExcerptSourceUnreadable means the file cannot be read as text
	// within the limits: gone, too large, not a regular file, binary,
	// or shorter than the line.
	ErrExcerptSourceUnreadable = errors.New("the file cannot be read as an excerpt")
	// ErrExcerptWithheld means the file's name or content says it may
	// hold a credential, so no line of it is shown.
	ErrExcerptWithheld = errors.New("withheld: the file may hold a secret")
)

// Limits of one excerpt.
const (
	// ExcerptFileLimit is the largest file an excerpt reads: 2 MiB.
	ExcerptFileLimit = 2 << 20
	// ExcerptRadius is how many lines an excerpt shows on each side of
	// the line it is about.
	ExcerptRadius = 8
	// ExcerptLineLimit is how many characters of one line an excerpt
	// shows; the rest is cut and the line marked truncated.
	ExcerptLineLimit = 320
)

// ExcerptLine is one line of an excerpt.
type ExcerptLine struct {
	Number    int
	Text      string
	Truncated bool
}

// SourceExcerpt is a few lines of one file around one line, read-only,
// as the file is on disk when the excerpt is read: the assessment kept
// no copy of the file, so the lines may differ from those it judged.
// ChangedSinceAssessment is true when the file's size differs from the
// size the assessment observed, which proves it changed; an equal size
// proves nothing.
type SourceExcerpt struct {
	Snapshot               string
	Path                   string
	Line                   int
	TotalLines             int
	Lines                  []ExcerptLine
	ChangedSinceAssessment bool
}

// ReadSourceExcerpt reads a few lines around a location the evidence
// of one snapshot names. It reads only source files that snapshot
// analyzed, or the domain recording; it withholds a file whose path or
// content says it may hold a secret; and it writes nothing.
type ReadSourceExcerpt struct {
	files SourceFiles
}

// NewReadSourceExcerpt requires the source file port.
func NewReadSourceExcerpt(files SourceFiles) (ReadSourceExcerpt, error) {
	if files == nil {
		return ReadSourceExcerpt{}, fmt.Errorf("read source excerpt: missing source files")
	}
	return ReadSourceExcerpt{files: files}, nil
}

// Execute reads the excerpt around line of file, which must be a
// repository-relative path of source the snapshot analyzed, or the
// domain recording.
func (uc ReadSourceExcerpt) Execute(evidence DomainEvidence, file string, line int) (SourceExcerpt, error) {
	if err := excerptPath(file); err != nil {
		return SourceExcerpt{}, err
	}
	if line < 1 {
		return SourceExcerpt{}, fmt.Errorf("%w: line %d", ErrExcerptRequestInvalid, line)
	}
	if !evidence.Readable(file) {
		return SourceExcerpt{}, fmt.Errorf("%w: %s", ErrExcerptOutsideEvidence, file)
	}
	if sensitivePath(file) {
		return SourceExcerpt{}, fmt.Errorf("%w: %s", ErrExcerptWithheld, file)
	}
	content, err := uc.files.ReadSource(file, ExcerptFileLimit)
	if err != nil {
		return SourceExcerpt{}, fmt.Errorf("%w: %s: %v", ErrExcerptSourceUnreadable, file, err)
	}
	if bytes.IndexByte(content, 0) >= 0 {
		return SourceExcerpt{}, fmt.Errorf("%w: %s is not text", ErrExcerptSourceUnreadable, file)
	}
	if secretInContent(content) {
		return SourceExcerpt{}, fmt.Errorf("%w: %s", ErrExcerptWithheld, file)
	}
	text := strings.ReplaceAll(string(content), "\r\n", "\n")
	text = strings.TrimSuffix(text, "\n")
	lines := strings.Split(text, "\n")
	if line > len(lines) {
		return SourceExcerpt{}, fmt.Errorf("%w: %s has %d lines, not %d", ErrExcerptSourceUnreadable, file, len(lines), line)
	}
	first := max(1, line-ExcerptRadius)
	last := min(len(lines), line+ExcerptRadius)
	out := SourceExcerpt{Snapshot: evidence.Snapshot, Path: file, Line: line, TotalLines: len(lines)}
	if size, ok := evidence.assessedSize(file); ok && size != int64(len(content)) {
		out.ChangedSinceAssessment = true
	}
	for n := first; n <= last; n++ {
		out.Lines = append(out.Lines, excerptLine(n, lines[n-1]))
	}
	return out, nil
}

// excerptPath refuses a path that is not a clean, relative, slash
// separated path inside the repository. The adapter confines the read
// again; this refuses the request before any file is touched.
func excerptPath(file string) error {
	switch {
	case file == "", strings.ContainsRune(file, 0), strings.Contains(file, `\`):
		return fmt.Errorf("%w: %q", ErrExcerptRequestInvalid, file)
	case path.IsAbs(file), path.Clean(file) != file, file == ".", file == "..", strings.HasPrefix(file, "../"):
		return fmt.Errorf("%w: %q is not a clean path inside the repository", ErrExcerptRequestInvalid, file)
	}
	return nil
}

// sensitivePath reports a path that conventionally holds credentials
// or keys. An analyzed source file can be named so too (secrets.ts,
// .env.ts), so the name alone withholds it: a directory of keys, an
// environment file, a key or certificate store, a credentials
// configuration file, or any name holding "secret". A source file named
// for a domain term such as credentials.go or password.go is not
// withheld by its name; its content is still read for secrets.
func sensitivePath(file string) bool {
	parts := strings.Split(strings.ToLower(file), "/")
	for _, dir := range parts[:len(parts)-1] {
		switch dir {
		case ".ssh", ".aws", ".gnupg", ".docker", ".kube", "secrets", ".secrets":
			return true
		}
	}
	base := parts[len(parts)-1]
	switch {
	case strings.Contains(base, "secret"),
		base == ".env", strings.HasPrefix(base, ".env."), strings.HasPrefix(base, "env."),
		strings.HasPrefix(base, "id_rsa"), strings.HasPrefix(base, "id_dsa"), strings.HasPrefix(base, "id_ecdsa"), strings.HasPrefix(base, "id_ed25519"),
		base == ".netrc", base == ".npmrc", base == ".pypirc", base == ".htpasswd",
		base == "credentials.json", base == "credentials.yaml", base == "credentials.yml", base == "credentials.toml", base == "credentials.ini", base == "credentials.xml",
		strings.HasPrefix(base, "service-account"):
		return true
	}
	for _, ext := range []string{".pem", ".key", ".p12", ".pfx", ".jks", ".keystore", ".crt", ".cer", ".der", ".kdbx", ".gpg", ".asc"} {
		if strings.HasSuffix(base, ext) {
			return true
		}
	}
	return false
}

// secretContent matches content that holds a credential: private key
// material, the token formats of the common hosts, and a string
// literal of twelve or more characters, spaces included, assigned to a
// name that says it is a secret, with or without a declared type between them (Go's
// var name string = "...", TypeScript's name: string = "...", Python's
// name: str = "..."). A file holding any of them is withheld whole, so no
// excerpt of it shows part of a credential.
var secretContent = []*regexp.Regexp{
	regexp.MustCompile(`-----BEGIN[ A-Z0-9]*PRIVATE KEY( BLOCK)?-----`),
	regexp.MustCompile(`\b(AKIA|ASIA|AGPA|AIDA|AROA)[0-9A-Z]{16}\b`),
	regexp.MustCompile(`\bgh[pousr]_[A-Za-z0-9]{36,}\b`),
	regexp.MustCompile(`\bgithub_pat_[A-Za-z0-9_]{50,}\b`),
	regexp.MustCompile(`\bglpat-[A-Za-z0-9_-]{20,}\b`),
	regexp.MustCompile(`\bxox[abposr]-[A-Za-z0-9-]{10,}\b`),
	regexp.MustCompile(`\bsk-(ant-|proj-|live-|test-)?[A-Za-z0-9_-]{20,}\b`),
	regexp.MustCompile(`\bAIza[0-9A-Za-z_-]{35}\b`),
	regexp.MustCompile(`\beyJ[A-Za-z0-9_-]{10,}\.eyJ[A-Za-z0-9_-]{10,}\.[A-Za-z0-9_-]{10,}`),
	regexp.MustCompile(`(?i)(password|passwd|secret|token|api[_-]?key|access[_-]?key|private[_-]?key|client[_-]?secret)[A-Za-z0-9_]*["']?(\s*:\s*[A-Za-z_][\w.\[\]<>|*, ]{0,48}?|\s+[A-Za-z_*\[][\w.\[\]*]{0,48})?\s*(:=|=|:)\s*["'\x60][^"'\x60\n]{12,}["'\x60]`),
}

func secretInContent(content []byte) bool {
	for _, re := range secretContent {
		if re.Match(content) {
			return true
		}
	}
	return false
}

// excerptLine keeps valid UTF-8 and at most ExcerptLineLimit
// characters of one line.
func excerptLine(n int, raw string) ExcerptLine {
	text := strings.ToValidUTF8(strings.TrimSuffix(raw, "\r"), "�")
	if utf8.RuneCountInString(text) <= ExcerptLineLimit {
		return ExcerptLine{Number: n, Text: text}
	}
	runes := []rune(text)
	return ExcerptLine{Number: n, Text: string(runes[:ExcerptLineLimit]), Truncated: true}
}

// Composed reports whether the use case was built by its constructor.
func (uc ReadSourceExcerpt) Composed() bool { return uc.files != nil }
