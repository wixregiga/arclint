// Package pattern owns the Pattern aggregate. It distributes the rule
// context's Rule, Module, Extension, Test, and Language values as-is.
package pattern

import (
	"crypto/sha256"
	"encoding/binary"
	"encoding/hex"
	"fmt"
	"sort"

	"github.com/wixregiga/arclint/internal/domain/rule"
)

const treeDigestDomain = "arclint-pattern-tree-v1\x00"

// Pattern is one immutable distributed version of a named architecture.
// Rule order is retained for faithful materialization and has no precedence.
type Pattern struct {
	reference  Reference
	modules    []rule.Module
	rules      []rule.Rule
	extensions []rule.Extension
	tests      []rule.Test
	coverage   []rule.Language
	files      []File
	digest     string
}

// Spec carries the complete validated values and exact full-tree bytes used
// to construct one Pattern version.
type Spec struct {
	Reference  Reference
	Modules    []rule.Module
	Rules      []rule.Rule
	Extensions []rule.Extension
	Tests      []rule.Test
	Coverage   []rule.Language
	Files      []File
}

// New constructs an immutable Pattern version. Tests and language coverage
// are optional. Every supplied Test must target one of the carried Rules.
func New(spec Spec) (Pattern, error) {
	if spec.Reference.IsZero() {
		return Pattern{}, fmt.Errorf("pattern: unconstructed reference")
	}
	fail := func(format string, args ...any) (Pattern, error) {
		return Pattern{}, fmt.Errorf("pattern %s: %s", spec.Reference, fmt.Sprintf(format, args...))
	}
	if len(spec.Rules) == 0 {
		return fail("no rules")
	}

	seenModules := map[rule.ModuleName]bool{}
	modules := append([]rule.Module(nil), spec.Modules...)
	for _, module := range modules {
		if module.IsZero() {
			return fail("unconstructed module")
		}
		if seenModules[module.Name()] {
			return fail("duplicate module %q", module.Name())
		}
		seenModules[module.Name()] = true
	}

	seenRules := map[string]bool{}
	rules := make([]rule.Rule, 0, len(spec.Rules))
	for _, carried := range spec.Rules {
		if err := carried.Validate(); err != nil {
			return fail("invalid rule: %v", err)
		}
		id := carried.ID().Qualified()
		if seenRules[id] {
			return fail("duplicate rule id %q", id)
		}
		seenRules[id] = true
		stamped, err := carried.WithProvenance(spec.Reference)
		if err != nil {
			return fail("rule %q: %v", id, err)
		}
		rules = append(rules, stamped)
	}

	seenExtensions := map[string]bool{}
	extensions := make([]rule.Extension, 0, len(spec.Extensions))
	for _, extension := range spec.Extensions {
		if extension.IsZero() {
			return fail("unconstructed extension")
		}
		if seenExtensions[extension.Name()] {
			return fail("duplicate extension %q", extension.Name())
		}
		seenExtensions[extension.Name()] = true
		extensions = append(extensions, extension)
	}

	seenTests := map[string]bool{}
	tests := append([]rule.Test(nil), spec.Tests...)
	for _, test := range tests {
		if test.IsZero() {
			return fail("unconstructed rule test")
		}
		if !seenRules[test.RuleID()] {
			return fail("rule test %q identifies unknown rule %q", test.Name(), test.RuleID())
		}
		key := test.RuleID() + "\x00" + test.Name()
		if seenTests[key] {
			return fail("duplicate rule test %q for %q", test.Name(), test.RuleID())
		}
		seenTests[key] = true
	}

	seenCoverage := map[rule.Language]bool{}
	coverage := append([]rule.Language(nil), spec.Coverage...)
	for _, language := range coverage {
		if !language.Valid() {
			return fail("coverage language %q invalid", language)
		}
		if seenCoverage[language] {
			return fail("duplicate coverage language %q", language)
		}
		seenCoverage[language] = true
	}

	files := cloneFiles(spec.Files)
	if len(files) == 0 {
		return fail("full distribution tree is empty")
	}
	seenFiles := map[string]bool{}
	hasManifest := false
	for _, file := range files {
		if file.IsZero() {
			return fail("unconstructed distribution file")
		}
		if seenFiles[file.Path()] {
			return fail("duplicate distribution file %q", file.Path())
		}
		seenFiles[file.Path()] = true
		hasManifest = hasManifest || file.Path() == "pattern.yaml"
	}
	if !hasManifest {
		return fail("full distribution tree does not contain pattern.yaml")
	}
	sort.Slice(files, func(i, j int) bool { return files[i].Path() < files[j].Path() })

	return Pattern{
		reference:  spec.Reference,
		modules:    modules,
		rules:      rules,
		extensions: extensions,
		tests:      tests,
		coverage:   coverage,
		files:      files,
		digest:     digestTree(files),
	}, nil
}

// Reference identifies this exact Pattern version.
func (p Pattern) Reference() Reference { return p.reference }

// Modules returns the logical groupings carried by the Pattern.
func (p Pattern) Modules() []rule.Module { return append([]rule.Module(nil), p.modules...) }

// Rules returns the carried Rules stamped with their Pattern origin.
func (p Pattern) Rules() []rule.Rule { return append([]rule.Rule(nil), p.rules...) }

// Extensions returns the optional executable enforcement carried by the version.
func (p Pattern) Extensions() []rule.Extension {
	return append([]rule.Extension(nil), p.extensions...)
}

// Tests returns the optional validated Rule Tests carried by the version.
func (p Pattern) Tests() []rule.Test { return append([]rule.Test(nil), p.tests...) }

// Coverage returns declared language coverage. An empty result is valid.
func (p Pattern) Coverage() []rule.Language {
	return append([]rule.Language(nil), p.coverage...)
}

// Files returns the complete distribution tree in canonical path order with
// defensive copies of every file's exact bytes.
func (p Pattern) Files() []File { return cloneFiles(p.files) }

// Digest is the lowercase SHA-256 digest of the complete tree.
func (p Pattern) Digest() string { return p.digest }

func cloneFiles(files []File) []File {
	out := make([]File, 0, len(files))
	for _, file := range files {
		out = append(out, File{path: file.path, bytes: append([]byte(nil), file.bytes...)})
	}
	return out
}

// digestTree is deliberately independent of filesystem walk order. It hashes
// a versioned domain prefix followed by each UTF-8 path and exact content in
// lexical path order. Unsigned 64-bit big-endian lengths frame both fields, so
// no two different path/content boundaries can produce the same byte stream.
func digestTree(files []File) string {
	h := sha256.New()
	_, _ = h.Write([]byte(treeDigestDomain))
	var size [8]byte
	for _, file := range files {
		binary.BigEndian.PutUint64(size[:], uint64(len(file.path)))
		_, _ = h.Write(size[:])
		_, _ = h.Write([]byte(file.path))
		binary.BigEndian.PutUint64(size[:], uint64(len(file.bytes)))
		_, _ = h.Write(size[:])
		_, _ = h.Write(file.bytes)
	}
	return hex.EncodeToString(h.Sum(nil))
}
