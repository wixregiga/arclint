package yamlrule

import (
	"fmt"
	"os"
	"strconv"
	"strings"

	"gopkg.in/yaml.v3"

	"github.com/wixregiga/arclint/internal/application"
	"github.com/wixregiga/arclint/internal/domain/vocab"
	"github.com/wixregiga/arclint/internal/infrastructure/filechange"
)

// PlanDomainRecordingName implements the application's
// DomainRecordingSetting port. Naming no DomainRecording, the zero
// value, is the absence of the domain section, so it removes the
// section with its own comments; any DomainRecording, even one
// discovery would find anyway, is written as domain.file, replacing the
// file the section names or adding the section before zones, else
// before rules, else at the end. Lines are spliced at the positions the
// YAML parser reports, so comments, blank lines, quoting and key order
// everywhere else survive. The edited document is read back through the
// strict loader, all in memory; the returned change writes it, and
// changed reports whether it differs from the ruleset as it is.
func (e Editor) PlanDomainRecordingName(recording vocab.DomainRecording) (application.FileChange, bool, error) {
	out, changed, err := e.namingDomainRecording(recording)
	if err != nil {
		return nil, false, err
	}
	change, err := filechange.Write(e.path, out)
	if err != nil {
		return nil, false, fmt.Errorf("name domain file: %w", err)
	}
	return change, changed, nil
}

// namingDomainRecording reads the ruleset and returns it edited to name
// recording, proven through the strict loader, and whether that differs
// from the ruleset as it is.
func (e Editor) namingDomainRecording(recording vocab.DomainRecording) ([]byte, bool, error) {
	data, err := os.ReadFile(e.path)
	if err != nil {
		return nil, false, fmt.Errorf("name domain file: %w", err)
	}
	parsed, err := parse(data, e.path)
	if err != nil {
		return nil, false, fmt.Errorf("name domain file: %w", err)
	}
	if parsed.pattern != nil {
		return nil, false, fmt.Errorf("name domain file: %s is a pattern distribution file, not a repository ruleset", e.path)
	}
	current, err := ParseRecording(data, e.path)
	if err != nil {
		return nil, false, fmt.Errorf("name domain file: %w", err)
	}
	if current == recording {
		return data, false, nil
	}
	doc, err := newEditableDocument(data)
	if err != nil {
		return nil, false, fmt.Errorf("name domain file: %s: %w", e.path, err)
	}
	if err := doc.nameDomainRecording(recording); err != nil {
		return nil, false, fmt.Errorf("name domain file: %s: %w", e.path, err)
	}
	out := doc.bytes()
	written, err := ParseRecording(out, e.path)
	if err != nil {
		return nil, false, fmt.Errorf("name domain file: %s: the edit produced an invalid ruleset, nothing was written: %w", e.path, err)
	}
	if written != recording {
		return nil, false, fmt.Errorf("name domain file: %s: the edit names %q instead of %q, nothing was written", e.path, written.Path(), recording.Path())
	}
	if _, err := parse(out, e.path); err != nil {
		return nil, false, fmt.Errorf("name domain file: %s: the edit produced an invalid ruleset, nothing was written: %w", e.path, err)
	}
	return out, true, nil
}

// nameDomainRecording collects the line edits that make the document
// name recording.
func (d *editableDocument) nameDomainRecording(recording vocab.DomainRecording) error {
	keyNode, value, idx := d.key(keyDomain)
	if recording.IsZero() {
		if keyNode == nil {
			return nil
		}
		start := d.ownedHeadStart(keyNode.Line - 1)
		end := d.blockEnd(d.nextTopKeyLine(idx), keyNode.Line)
		for end < len(d.lines) && strings.TrimSpace(d.lines[end]) == "" {
			end++
		}
		d.edits = append(d.edits, lineEdit{start: start, end: end})
		return nil
	}
	file := strconv.Quote(recording.Path())
	if keyNode == nil {
		d.insertDomainBlock(file)
		return nil
	}
	if value.Kind != yaml.MappingNode {
		return fmt.Errorf("line %d: %s is not a mapping", keyNode.Line, keyDomain)
	}
	for i := 0; i+1 < len(value.Content); i += 2 {
		if value.Content[i].Value == keyFile {
			return d.replaceScalarText(value.Content[i+1], file)
		}
	}
	return fmt.Errorf("line %d: %s names no %s", keyNode.Line, keyDomain, keyFile)
}

// replaceScalarText rewrites one single-line scalar, quotes included,
// with replacement written exactly as given.
func (d *editableDocument) replaceScalarText(n *yaml.Node, replacement string) error {
	quote := ""
	switch n.Style {
	case yaml.DoubleQuotedStyle:
		quote = `"`
	case yaml.SingleQuotedStyle:
		quote = `'`
	case 0, yaml.TaggedStyle:
	default:
		return fmt.Errorf("line %d: %s.%s is not a single-line scalar", n.Line, keyDomain, keyFile)
	}
	line, col := n.Line-1, n.Column-1
	if line < 0 || line >= len(d.lines) || col >= len(d.lines[line]) {
		return fmt.Errorf("line %d: cannot locate %s.%s", n.Line, keyDomain, keyFile)
	}
	text := d.lines[line]
	end, ok := scalarEnd(text, col, quote)
	if !ok {
		return fmt.Errorf("line %d: cannot locate %s.%s", n.Line, keyDomain, keyFile)
	}
	d.edits = append(d.edits, lineEdit{start: line, end: line + 1, replacement: []string{text[:col] + replacement + text[end:]}})
	return nil
}

// scalarEnd finds where a single-line scalar that starts at col ends, by
// its source spelling rather than its decoded value: a quoted scalar
// ends at its closing quote, past any escape (a doubled quote in single quotes, a
// backslash escape in double quotes); a plain scalar ends before a
// comment, a "#" after a space or a tab, or at the end of the line,
// trailing blanks excluded.
func scalarEnd(text string, col int, quote string) (int, bool) {
	if quote == "" {
		end := len(text)
		for i := col + 1; i < len(text); i++ {
			if text[i] == '#' && (text[i-1] == ' ' || text[i-1] == '\t') {
				end = i
				break
			}
		}
		return col + len(strings.TrimRight(text[col:end], " \t")), true
	}
	if !strings.HasPrefix(text[col:], quote) {
		return 0, false
	}
	for i := col + 1; i < len(text); i++ {
		switch {
		case quote == `"` && text[i] == '\\':
			i++
		case text[i] == quote[0] && quote == "'" && i+1 < len(text) && text[i+1] == '\'':
			i++
		case text[i] == quote[0]:
			return i + 1, true
		}
	}
	return 0, false
}

// ownedHeadStart is the first line of the comments a top-level key owns:
// the comment lines directly above it, unless they run up to the first
// line of the document, where they are the document's header (the schema
// modeline and the file's introduction) and belong to no key.
func (d *editableDocument) ownedHeadStart(line int) int {
	start := d.headStart(line, "")
	if start == 0 {
		return line
	}
	return start
}

// insertDomainBlock adds the domain section before zones, else before
// rules, else at the end of the document, followed by one blank line
// when something follows it. Removing the section takes that blank line
// with it, so naming the default again restores the document exactly.
func (d *editableDocument) insertDomainBlock(file string) {
	block := []string{keyDomain + ":", "  " + keyFile + ": " + file}
	at := len(d.lines)
	for _, name := range []string{keyZones, keyRules} {
		if keyNode, _, _ := d.key(name); keyNode != nil {
			at = d.ownedHeadStart(keyNode.Line - 1)
			break
		}
	}
	if at < len(d.lines) {
		block = append(block, "")
	}
	d.edits = append(d.edits, lineEdit{start: at, end: at, replacement: block})
}
