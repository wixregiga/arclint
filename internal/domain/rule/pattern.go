package rule

import (
	"fmt"
	"regexp"
	"strings"
)

// patternVersion accepts the semantic-versioning core with optional
// pre-release or build metadata.
var patternVersion = regexp.MustCompile(`^\d+\.\d+\.\d+([\-+][0-9A-Za-z.\-+]+)?$`)

// PatternReference is an exact repository-owned reference to one
// distributed Pattern version. It resolves deterministically.
type PatternReference struct {
	namespace string
	name      string
	version   string
}

// NewPatternReference requires namespace, name, and an exact version.
func NewPatternReference(namespace, name, version string) (PatternReference, error) {
	if namespace == "" || name == "" {
		return PatternReference{}, fmt.Errorf("pattern reference: namespace and name required")
	}
	if !patternVersion.MatchString(version) {
		return PatternReference{}, fmt.Errorf("pattern reference %s/%s: version %q is not exact semver", namespace, name, version)
	}
	return PatternReference{namespace: namespace, name: name, version: version}, nil
}

// Namespace of the distributing Pattern.
func (r PatternReference) Namespace() string { return r.namespace }

// Name of the Pattern within its namespace.
func (r PatternReference) Name() string { return r.name }

// Version is the exact published version.
func (r PatternReference) Version() string { return r.version }

// IsZero reports an unconstructed reference.
func (r PatternReference) IsZero() bool { return r.name == "" }

func (r PatternReference) String() string {
	return r.namespace + "/" + r.name + "@" + r.version
}

// Pattern is a named, versioned, namespaced, tested collection of Rules
// dressed for distribution. A published version is immutable; every
// included Rule retains its own Rule ID; Pattern order creates no
// implicit Rule precedence.
type Pattern struct {
	ref        PatternReference
	rules      []Rule
	extensions []PatternExtension
	coverage   []Language
}

// NewPattern requires an exact identity and at least one valid Rule.
// Each carried Rule is stamped with this Pattern's provenance.
func NewPattern(namespace, name, version string, rules []Rule, extensions []PatternExtension, coverage []Language) (Pattern, error) {
	ref, err := NewPatternReference(namespace, name, version)
	if err != nil {
		return Pattern{}, err
	}
	if len(rules) == 0 {
		return Pattern{}, fmt.Errorf("pattern %s: no rules", ref)
	}
	seen := map[string]bool{}
	stamped := make([]Rule, 0, len(rules))
	for _, r := range rules {
		if r.id.IsZero() {
			return Pattern{}, fmt.Errorf("pattern %s: unconstructed rule", ref)
		}
		qualified := r.id.Qualified()
		if seen[qualified] {
			return Pattern{}, fmt.Errorf("pattern %s: duplicate rule id %q", ref, qualified)
		}
		seen[qualified] = true
		r.provenance = &ref
		stamped = append(stamped, r)
	}
	seenExt := map[string]bool{}
	copiedExt := make([]PatternExtension, 0, len(extensions))
	for _, e := range extensions {
		if e.fileName == "" {
			return Pattern{}, fmt.Errorf("pattern %s: unconstructed extension", ref)
		}
		if seenExt[e.fileName] {
			return Pattern{}, fmt.Errorf("pattern %s: duplicate extension file %q", ref, e.fileName)
		}
		seenExt[e.fileName] = true
		copiedExt = append(copiedExt, e)
	}
	for _, l := range coverage {
		if !l.Valid() {
			return Pattern{}, fmt.Errorf("pattern %s: coverage language %q invalid", ref, l)
		}
	}
	return Pattern{
		ref:        ref,
		rules:      stamped,
		extensions: copiedExt,
		coverage:   append([]Language(nil), coverage...),
	}, nil
}

// Reference identifies the exact Pattern.
func (p Pattern) Reference() PatternReference { return p.ref }

// Rules returns the Rules carried by the Pattern, each with Pattern
// provenance.
func (p Pattern) Rules() []Rule { return append([]Rule(nil), p.rules...) }

// Coverage returns the declared language coverage.
func (p Pattern) Coverage() []Language { return append([]Language(nil), p.coverage...) }

// Extensions returns the optional Extension sources carried by the Pattern.
func (p Pattern) Extensions() []PatternExtension {
	return append([]PatternExtension(nil), p.extensions...)
}

// PatternExtension is one installable Sobek entry file carried by a Pattern.
type PatternExtension struct {
	fileName string
	source   string
}

// NewPatternExtension requires an installable entry basename and non-blank source.
func NewPatternExtension(fileName, source string) (PatternExtension, error) {
	if err := validateExtensionFileName(fileName); err != nil {
		return PatternExtension{}, err
	}
	if strings.TrimSpace(source) == "" {
		return PatternExtension{}, fmt.Errorf("pattern extension %s: blank source", fileName)
	}
	return PatternExtension{fileName: fileName, source: source}, nil
}

// FileName is the installable entry basename.
func (e PatternExtension) FileName() string { return e.fileName }

// Source is the Extension file contents.
func (e PatternExtension) Source() string { return e.source }

// InstallableExtensionFileName reports whether fileName is an installable
// Sobek entry basename: a non-hidden .ts or .js file that is not a
// declaration file.
func InstallableExtensionFileName(fileName string) bool {
	return validateExtensionFileName(fileName) == nil
}

func validateExtensionFileName(fileName string) error {
	if fileName == "" {
		return fmt.Errorf("pattern extension: file name required")
	}
	if strings.ContainsAny(fileName, `/\`) || fileName == "." || fileName == ".." {
		return fmt.Errorf("pattern extension %q: file name must be a basename", fileName)
	}
	if strings.HasPrefix(fileName, ".") {
		return fmt.Errorf("pattern extension %q: hidden file names are not installable entries", fileName)
	}
	if strings.HasSuffix(fileName, ".d.ts") {
		return fmt.Errorf("pattern extension %q: declaration files are not installable entries", fileName)
	}
	if !strings.HasSuffix(fileName, ".ts") && !strings.HasSuffix(fileName, ".js") {
		return fmt.Errorf("pattern extension %q: must end in .ts or .js", fileName)
	}
	return nil
}
