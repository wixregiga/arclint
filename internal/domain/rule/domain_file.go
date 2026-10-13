package rule

import (
	"fmt"
	"path"
	"strings"

	"github.com/wixregiga/arclint/internal/domain/vocab"
)

// DomainFile is the project's domain file as the ruleset knows it: the
// DomainRecording rules.arclint.yaml names under domain.file, that init
// writes there, or that a translation moves the language into. A path a
// person writes becomes the project's DomainRecording only through
// NewDomainFile, because only the ruleset says which file holds the
// project's language.
type DomainFile struct {
	recording vocab.DomainRecording
}

// NewDomainFile builds the DomainFile of file, the path as written. It
// applies never-the-ruleset to the path: rules.arclint.yaml, however
// the path spells it, is refused, in any letter case, because a
// case-insensitive file system reaches the ruleset through each of those
// names even before the ruleset exists. The DomainRecording then applies its
// own invariants. The filesystem completes never-the-ruleset through
// links, which a path alone cannot show.
func NewDomainFile(file string) (DomainFile, error) {
	if strings.EqualFold(path.Clean(file), RulesetFileName) {
		return DomainFile{}, fmt.Errorf("DomainFile never-the-ruleset: %q is %s itself; name the file the language is recorded in", file, RulesetFileName)
	}
	recording, err := vocab.NewDomainRecording(file)
	if err != nil {
		return DomainFile{}, fmt.Errorf("%w", err)
	}
	return DomainFile{recording: recording}, nil
}

// Recording returns the DomainRecording the domain file is.
func (f DomainFile) Recording() vocab.DomainRecording { return f.recording }
