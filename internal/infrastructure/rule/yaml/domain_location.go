package yamlrule

import (
	"errors"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"

	"github.com/wixregiga/arclint/internal/domain/rule"
	"github.com/wixregiga/arclint/internal/domain/vocab"
)

// ErrDomainFileIsRuleset matches the error CheckDomainFileLocation
// returns when a domain file reaches the ruleset through a link.
var ErrDomainFileIsRuleset = errors.New("the domain file is the ruleset")

// CheckDomainFileLocation applies the DomainFile invariant
// never-the-ruleset through links, which a path alone cannot show.
// location is where the recording's file lies with every symbolic link
// followed; rulesPath is the ruleset, followed the same way. A hard link
// leaves two names for one file, so when both exist the file system is
// asked whether they are the same file. A ruleset that does not exist
// yet is no file to reach.
func CheckDomainFileLocation(rulesPath string, recording vocab.DomainRecording, location string) error {
	ruleset, err := filepath.EvalSymlinks(rulesPath)
	if errors.Is(err, fs.ErrNotExist) {
		return nil
	}
	if err != nil {
		return fmt.Errorf("resolve %s: %w", rule.RulesetFileName, err)
	}
	same := location == ruleset
	if !same {
		same, err = sameFile(ruleset, location)
		if err != nil {
			return err
		}
	}
	if same {
		return fmt.Errorf("DomainFile never-the-ruleset: %q reaches %s through a link: %w", recording.Path(), rule.RulesetFileName, ErrDomainFileIsRuleset)
	}
	return nil
}

// sameFile reports whether location is the ruleset under another name.
// A location with no file yet names no file at all.
func sameFile(ruleset, location string) (bool, error) {
	domainFile, err := os.Stat(location)
	if errors.Is(err, fs.ErrNotExist) {
		return false, nil
	}
	if err != nil {
		return false, fmt.Errorf("inspect the domain file %s: %w", location, err)
	}
	rules, err := os.Stat(ruleset)
	if err != nil {
		return false, fmt.Errorf("inspect %s: %w", rule.RulesetFileName, err)
	}
	return os.SameFile(rules, domainFile), nil
}
