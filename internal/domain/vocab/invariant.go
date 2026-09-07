package vocab

import (
	"fmt"
	"regexp"
	"strings"
)

// Invariant is one consistency rule an aggregate or a value object
// owns: a key that names the rule and is stable across edits, and the
// rule stated in the ubiquitous language. For an aggregate's invariant
// the key is also the name of the root method that enforces it,
// spelled in the language's method case. The owner is the nesting the
// invariant is recorded under; an Invariant never names its owner.
type Invariant struct {
	Key       string
	Statement string
	Line      int
}

// OwnedInvariant is an Invariant with the owner it is recorded under,
// the form a check or a listing works with.
type OwnedInvariant struct {
	Context      string
	Owner        string
	OwnerConcept Concept
	Invariant    Invariant
}

// recordedKey is KeyPattern compiled: the spelling every key takes,
// lowercase kebab-case starting with a letter.
var recordedKey = regexp.MustCompile(KeyPattern)

// checkOnlyKeys are the keys invariant/key-names-the-rule rejects: each
// names the act of checking and says nothing about the rule.
var checkOnlyKeys = []string{"validate", "check", "verify", "is-valid", "check-rules", "ensure", "assert", "guard"}

// keySpelling says what is wrong with a key's spelling, or nothing.
func keySpelling(key string) string {
	switch {
	case strings.TrimSpace(key) == "":
		return "is empty"
	case !recordedKey.MatchString(key):
		return "is not lowercase kebab-case"
	default:
		return ""
	}
}

// requireKey applies invariant/keyed-within-owner's spelling to one
// invariant or assertion key.
func requireKey(line int, what, key string) error {
	if wrong := keySpelling(key); wrong != "" {
		return fmt.Errorf("invariant/keyed-within-owner: %s%s key %q %s", at(line), what, key, wrong)
	}
	return nil
}

// validateInvariants applies invariant/keyed-within-owner and
// invariant/key-names-the-rule to the invariants one owner records.
func validateInvariants(where string, invariants []Invariant) error {
	seen := map[string]bool{}
	for _, inv := range invariants {
		if err := requireKey(inv.Line, where+": invariant", inv.Key); err != nil {
			return err
		}
		if seen[inv.Key] {
			return fmt.Errorf("invariant/keyed-within-owner: %s%s: invariant %q is recorded twice", at(inv.Line), where, inv.Key)
		}
		seen[inv.Key] = true
		for _, forbidden := range checkOnlyKeys {
			if inv.Key == forbidden {
				return fmt.Errorf("invariant/key-names-the-rule: %s%s: invariant key %q names the act of checking, not the rule it protects",
					at(inv.Line), where, inv.Key)
			}
		}
		if strings.TrimSpace(inv.Statement) == "" {
			return fmt.Errorf("invariant/keyed-within-owner: %s%s: invariant %q has an empty statement", at(inv.Line), where, inv.Key)
		}
	}
	return nil
}
