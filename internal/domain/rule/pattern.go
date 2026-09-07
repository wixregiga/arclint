package rule

import (
	"fmt"
	"regexp"
	"sort"
	"strings"
)

// patternVersion accepts the semantic-versioning core with optional
// pre-release or build metadata.
var patternVersion = regexp.MustCompile(`^\d+\.\d+\.\d+([\-+][0-9A-Za-z.\-+]+)?$`)

// PatternReference is an exact repository-owned reference to one
// distributed Pattern version, spelled namespace/name@version. It
// resolves deterministically.
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
	if err := validateQualifierParts(namespace, name); err != nil {
		return PatternReference{}, fmt.Errorf("pattern reference: %v", err)
	}
	if !patternVersion.MatchString(version) {
		return PatternReference{}, fmt.Errorf("pattern reference %s/%s: version %q is not exact semver", namespace, name, version)
	}
	return PatternReference{namespace: namespace, name: name, version: version}, nil
}

// ParsePatternReference reads the one published spelling of a
// reference, namespace/name@version. Every part is required: a
// repository always extends one exact version.
func ParsePatternReference(s string) (PatternReference, error) {
	s = strings.TrimSpace(s)
	at := strings.LastIndex(s, "@")
	if at < 0 {
		return PatternReference{}, fmt.Errorf("pattern reference %q: expected namespace/name@version", s)
	}
	path, version := s[:at], s[at+1:]
	slash := strings.Index(path, "/")
	if slash < 0 || strings.Count(path, "/") != 1 {
		return PatternReference{}, fmt.Errorf("pattern reference %q: expected namespace/name@version", s)
	}
	return NewPatternReference(path[:slash], path[slash+1:], version)
}

// Namespace of the distributing Pattern.
func (r PatternReference) Namespace() string { return r.namespace }

// Name of the Pattern within its namespace.
func (r PatternReference) Name() string { return r.name }

// Version is the exact published version.
func (r PatternReference) Version() string { return r.version }

// validateQualifierParts checks the namespace and name of a Pattern
// qualifier: each is an id part that also excludes "/", the separator
// between them.
func validateQualifierParts(namespace, name string) error {
	for _, part := range []struct{ label, value string }{{namespacePart, namespace}, {namePart, name}} {
		if strings.Contains(part.value, "/") {
			return fmt.Errorf("%s %q contains \"/\"", part.label, part.value)
		}
		if err := validateIDPart(part.value); err != nil {
			return fmt.Errorf("%s %q %v", part.label, part.value, err)
		}
	}
	return nil
}

// Qualifier is the namespace/name that qualifies every Rule ID this
// Pattern distributes; it never carries the version.
func (r PatternReference) Qualifier() string { return r.namespace + "/" + r.name }

// IsZero reports an unconstructed reference.
func (r PatternReference) IsZero() bool { return r.name == "" }

func (r PatternReference) String() string {
	return r.namespace + "/" + r.name + "@" + r.version
}

// PatternZone is one Zone a Pattern speaks about without owning
// its paths: a name, a description, and optionally the paths the
// Pattern suggests, which an adopting repository may accept as its
// Binding or replace. A Pattern Rule names a PatternZone; the paths
// arrive when the repository extends the Pattern.
type PatternZone struct {
	name           ZoneName
	description    string
	suggestedPaths []Glob
}

// NewPatternZone requires a valid name and a description; suggested
// paths are optional.
func NewPatternZone(name ZoneName, description string, suggestedPaths []Glob) (PatternZone, error) {
	if err := name.validate(); err != nil {
		return PatternZone{}, err
	}
	if strings.TrimSpace(description) == "" {
		return PatternZone{}, fmt.Errorf("pattern zone %s: description required", name)
	}
	for _, g := range suggestedPaths {
		if g.IsZero() {
			return PatternZone{}, fmt.Errorf("pattern zone %s: unconstructed suggested path", name)
		}
	}
	return PatternZone{
		name:           name,
		description:    strings.TrimSpace(description),
		suggestedPaths: append([]Glob(nil), suggestedPaths...),
	}, nil
}

// Name is the ZoneName Pattern Rules and Bindings use.
func (m PatternZone) Name() ZoneName { return m.name }

// Description is the Pattern's statement of what the Zone is for.
func (m PatternZone) Description() string { return m.description }

// SuggestedPaths are the paths the Pattern proposes for its Binding.
func (m PatternZone) SuggestedPaths() []Glob {
	return append([]Glob(nil), m.suggestedPaths...)
}

// Binding gives one PatternZone its paths in an adopting repository.
type Binding struct {
	zone  ZoneName
	paths []Glob
}

// NewBinding requires a valid Zone name and at least one path.
func NewBinding(zone ZoneName, paths []Glob) (Binding, error) {
	if err := zone.validate(); err != nil {
		return Binding{}, err
	}
	if len(paths) == 0 {
		return Binding{}, fmt.Errorf("binding %s: at least one path required", zone)
	}
	for _, g := range paths {
		if g.IsZero() {
			return Binding{}, fmt.Errorf("binding %s: unconstructed path", zone)
		}
	}
	return Binding{zone: zone, paths: append([]Glob(nil), paths...)}, nil
}

// Zone is the bound PatternZone's name.
func (b Binding) Zone() ZoneName { return b.zone }

// Paths are the globs the repository gives the Zone.
func (b Binding) Paths() []Glob { return append([]Glob(nil), b.paths...) }

// PatternSpec is the complete input for constructing a Pattern.
type PatternSpec struct {
	Namespace     string
	Name          string
	Version       string
	Documentation string
	Coverage      []Language
	Zones         []PatternZone
	Rules         []Rule
	Extensions    []PatternExtension
}

// Pattern is a named, versioned, namespaced, tested collection of Rules
// and the Zones they speak about, dressed for distribution. A
// published version is immutable; every included Rule retains its own
// Rule ID under the Pattern's namespace; Pattern order creates no
// implicit Rule precedence.
type Pattern struct {
	ref           PatternReference
	documentation string
	coverage      []Language
	zones         []PatternZone
	rules         []Rule
	extensions    []PatternExtension
}

// NewPattern requires an exact identity and at least one valid Rule.
// Each carried Rule is stamped with this Pattern's provenance, must
// carry the Pattern's namespace, and may name only Zones the Pattern
// lists.
func NewPattern(spec PatternSpec) (Pattern, error) {
	ref, err := NewPatternReference(spec.Namespace, spec.Name, spec.Version)
	if err != nil {
		return Pattern{}, err
	}
	if len(spec.Rules) == 0 {
		return Pattern{}, fmt.Errorf("pattern %s: no rules", ref)
	}
	declared := map[ZoneName]bool{}
	zones := make([]PatternZone, 0, len(spec.Zones))
	for _, m := range spec.Zones {
		if m.name == "" {
			return Pattern{}, fmt.Errorf("pattern %s: unconstructed zone", ref)
		}
		if declared[m.name] {
			return Pattern{}, fmt.Errorf("pattern %s: duplicate zone %q", ref, m.name)
		}
		declared[m.name] = true
		zones = append(zones, m)
	}
	seen := map[string]bool{}
	stamped := make([]Rule, 0, len(spec.Rules))
	for _, r := range spec.Rules {
		if r.id.IsZero() {
			return Pattern{}, fmt.Errorf("pattern %s: unconstructed rule", ref)
		}
		if r.id.Qualifier() != ref.Qualifier() {
			return Pattern{}, fmt.Errorf("pattern %s: rule %s must carry the pattern qualifier %q", ref, r.id, ref.Qualifier())
		}
		qualified := r.id.Qualified()
		if seen[qualified] {
			return Pattern{}, fmt.Errorf("pattern %s: duplicate rule id %q", ref, qualified)
		}
		seen[qualified] = true
		for _, m := range r.ReferencedZones() {
			if !declared[m] {
				return Pattern{}, fmt.Errorf("pattern %s: rule %s names zone %q the pattern does not list", ref, r.id, m)
			}
		}
		r.provenance = &ref
		stamped = append(stamped, r)
	}
	seenExt := map[string]bool{}
	copiedExt := make([]PatternExtension, 0, len(spec.Extensions))
	for _, e := range spec.Extensions {
		if e.fileName == "" {
			return Pattern{}, fmt.Errorf("pattern %s: unconstructed extension", ref)
		}
		if seenExt[e.fileName] {
			return Pattern{}, fmt.Errorf("pattern %s: duplicate extension file %q", ref, e.fileName)
		}
		seenExt[e.fileName] = true
		copiedExt = append(copiedExt, e)
	}
	for _, l := range spec.Coverage {
		if !l.Valid() {
			return Pattern{}, fmt.Errorf("pattern %s: coverage language %q invalid", ref, l)
		}
	}
	return Pattern{
		ref:           ref,
		documentation: strings.TrimSpace(spec.Documentation),
		coverage:      append([]Language(nil), spec.Coverage...),
		zones:         zones,
		rules:         stamped,
		extensions:    copiedExt,
	}, nil
}

// Reference identifies the exact Pattern.
func (p Pattern) Reference() PatternReference { return p.ref }

// Documentation is the link the Pattern publishes for its readers,
// empty when it publishes none.
func (p Pattern) Documentation() string { return p.documentation }

// Zones returns the Zones the Pattern speaks about, in declared
// order.
func (p Pattern) Zones() []PatternZone {
	return append([]PatternZone(nil), p.zones...)
}

// Rules returns the Rules carried by the Pattern, each with Pattern
// provenance.
func (p Pattern) Rules() []Rule { return append([]Rule(nil), p.rules...) }

// Coverage returns the declared language coverage.
func (p Pattern) Coverage() []Language { return append([]Language(nil), p.coverage...) }

// Extensions returns the optional Extension sources carried by the Pattern.
func (p Pattern) Extensions() []PatternExtension {
	return append([]PatternExtension(nil), p.extensions...)
}

// Bind gives every Zone the Pattern lists its repository paths. Every
// listed Zone must be bound and no Binding may name a Zone the
// Pattern does not list; the result is the concrete Zones, in the
// Pattern's declared order, each carrying the Pattern's description.
func (p Pattern) Bind(bindings []Binding) ([]Zone, error) {
	byName := map[ZoneName]Binding{}
	for _, b := range bindings {
		if _, dup := byName[b.zone]; dup {
			return nil, fmt.Errorf("pattern %s: zone %q bound twice", p.ref, b.zone)
		}
		byName[b.zone] = b
	}
	declared := map[ZoneName]bool{}
	out := make([]Zone, 0, len(p.zones))
	var unbound []string
	for _, m := range p.zones {
		declared[m.name] = true
		b, ok := byName[m.name]
		if !ok {
			unbound = append(unbound, string(m.name))
			continue
		}
		mod, err := NewZone(m.name, m.description, b.paths)
		if err != nil {
			return nil, fmt.Errorf("pattern %s: %v", p.ref, err)
		}
		out = append(out, mod)
	}
	if len(unbound) > 0 {
		return nil, fmt.Errorf("pattern %s: unbound zones %s; bind each under extends[].bind", p.ref, strings.Join(unbound, ", "))
	}
	var unknown []string
	for name := range byName {
		if !declared[name] {
			unknown = append(unknown, string(name))
		}
	}
	if len(unknown) > 0 {
		sort.Strings(unknown)
		return nil, fmt.Errorf("pattern %s: bind names zones the pattern does not list: %s", p.ref, strings.Join(unknown, ", "))
	}
	return out, nil
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
