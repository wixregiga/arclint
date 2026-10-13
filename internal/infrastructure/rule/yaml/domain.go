package yamlrule

import (
	"bytes"
	"errors"
	"fmt"
	"io"
	"io/fs"
	"os"

	"gopkg.in/yaml.v3"

	"github.com/wixregiga/arclint/internal/domain/rule"
	"github.com/wixregiga/arclint/internal/domain/vocab"
)

// keyFile is the one key of the top-level domain mapping: the project's
// DomainRecording.
const keyFile = "file"

// topLevel decodes one ruleset document into its top-level mapping.
// empty reports a document with no content at all; callers decide
// whether that is an error. source names the document in messages.
func topLevel(data []byte, source string) (top mapping, empty bool, err error) {
	var root yaml.Node
	if err := yaml.NewDecoder(bytes.NewReader(data)).Decode(&root); err != nil {
		if errors.Is(err, io.EOF) {
			return mapping{}, true, nil
		}
		return mapping{}, false, fmt.Errorf("%s: %v", source, err)
	}
	body := &root
	if body.Kind == yaml.DocumentNode {
		if len(body.Content) == 0 {
			return mapping{}, true, nil
		}
		body = body.Content[0]
	}
	top, err = asMapping(body, "document")
	if err != nil {
		return mapping{}, false, fmt.Errorf("%s: %v", source, err)
	}
	return top, false, nil
}

// parseDomain reads the optional top-level domain mapping: the
// DomainRecording the project writes its Ubiquitous Language in. An
// absent mapping names none, the zero DomainRecording; which file the
// project records in then is for vocab.ResolveDomainRecording to
// discover. Every other shape is an error naming its line: a mapping
// with a key other than file, a file that is not a string or cannot
// become a DomainRecording, and a Pattern distribution file, which
// never chooses a repository's domain file. Both the full loader and
// ReadRecording read the key through here.
func parseDomain(top mapping) (vocab.DomainRecording, error) {
	n := top.get(keyDomain)
	if n == nil {
		return vocab.DomainRecording{}, nil
	}
	line := top.lines[keyDomain]
	if top.get(keyPattern) != nil {
		return vocab.DomainRecording{}, fmt.Errorf("%s: line %d: a pattern distribution file does not accept %s; a Pattern never chooses a repository's domain file", keyDomain, line, keyDomain)
	}
	if n.Kind != yaml.MappingNode {
		return vocab.DomainRecording{}, fmt.Errorf("%s: line %d: expected an object with the key %s", keyDomain, line, keyFile)
	}
	var file *yaml.Node
	for i := 0; i+1 < len(n.Content); i += 2 {
		key, value := n.Content[i], n.Content[i+1]
		if key.Kind != yaml.ScalarNode {
			return vocab.DomainRecording{}, fmt.Errorf("%s: line %d: keys must be plain strings", keyDomain, key.Line)
		}
		if key.Value != keyFile {
			return vocab.DomainRecording{}, fmt.Errorf("%s: line %d: unknown key %q (accepted: %s)", keyDomain, key.Line, key.Value, keyFile)
		}
		if file != nil {
			return vocab.DomainRecording{}, fmt.Errorf("%s: line %d: key %q appears twice", keyDomain, key.Line, keyFile)
		}
		file = value
	}
	where := keyDomain + "." + keyFile
	if file == nil {
		return vocab.DomainRecording{}, fmt.Errorf("%s: line %d: %s is required (the file the project writes its Ubiquitous Language in)", keyDomain, line, keyFile)
	}
	if file.Kind != yaml.ScalarNode || file.Tag != "!!str" {
		return vocab.DomainRecording{}, fmt.Errorf("%s: line %d: expected a string", where, file.Line)
	}
	named, err := rule.NewDomainFile(file.Value)
	if err != nil {
		return vocab.DomainRecording{}, fmt.Errorf("%s: line %d: %w", where, file.Line, err)
	}
	return named.Recording(), nil
}

// ParseRecording reads only the domain.file key of one ruleset
// document. A document that is empty or names no domain file names none:
// the zero DomainRecording. source names the document in messages. The
// rest of the document is not read here; the full load judges it.
func ParseRecording(data []byte, source string) (vocab.DomainRecording, error) {
	top, empty, err := topLevel(data, source)
	if err != nil {
		return vocab.DomainRecording{}, err
	}
	if empty {
		return vocab.DomainRecording{}, nil
	}
	recording, err := parseDomain(top)
	if err != nil {
		return vocab.DomainRecording{}, fmt.Errorf("%s: %w", source, err)
	}
	return recording, nil
}

// ReadRecording reads only the domain.file key of the ruleset at
// rulesPath, so composition can choose the vocabulary adapter before
// the full ruleset loads (the full load needs the vocabulary). A
// missing ruleset file, or one that names no domain file, names none:
// the zero DomainRecording.
func ReadRecording(rulesPath string) (vocab.DomainRecording, error) {
	data, err := os.ReadFile(rulesPath)
	if err != nil {
		if errors.Is(err, fs.ErrNotExist) {
			return vocab.DomainRecording{}, nil
		}
		return vocab.DomainRecording{}, fmt.Errorf("read ruleset: %w", err)
	}
	return ParseRecording(data, rulesPath)
}

// RecordingParser adapts ParseRecording to the application's
// RulesetRecordingSource port, for the ruleset a Rule Test fixture
// carries.
type RecordingParser struct{}

// ParseRulesetRecording implements application.RulesetRecordingSource.
func (RecordingParser) ParseRulesetRecording(content []byte) (vocab.DomainRecording, error) {
	return ParseRecording(content, rule.RulesetFileName)
}
