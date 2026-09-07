package yamlvocab

import (
	"regexp"
	"strings"
	"unicode/utf8"
)

// lineWidth is the column folded prose is filled to. yaml.v3 emits
// every folded scalar as one line however long, so the emitted document
// is refilled before it is written; hand wrapping is normalized to this
// fill rather than kept. Refilling only turns single spaces inside a
// folded block into line breaks, which YAML folds back into the same
// spaces, so the recorded text is unchanged.
const lineWidth = 80

// foldedHeader matches the header line yaml.v3 emits for a folded block
// scalar: an indented key or sequence item, ">" with its optional
// chomping and indentation indicators, and at most a comment. A plain
// scalar can never start with ">", so the match identifies a block.
var foldedHeader = regexp.MustCompile(`^( *)(?:(?:- )?[^#]*?: |- )>[+-]?[1-9]?(?: +#.*)?$`)

// reflowFolded fills the content lines of every folded block scalar in
// an emitted YAML document to lineWidth. Lines indented deeper than the
// block are literal in YAML and are left alone.
func reflowFolded(doc []byte) []byte {
	lines := strings.Split(string(doc), "\n")
	out := make([]string, 0, len(lines))
	for i := 0; i < len(lines); i++ {
		out = append(out, lines[i])
		if !foldedHeader.MatchString(lines[i]) {
			continue
		}
		headerIndent := indentation(lines[i])
		blockIndent := -1
		for i+1 < len(lines) {
			next := lines[i+1]
			if strings.TrimSpace(next) != "" {
				indent := indentation(next)
				if indent <= headerIndent {
					break
				}
				if blockIndent < 0 {
					blockIndent = indent
				}
				if indent == blockIndent {
					out = append(out, fill(next[:indent], next[indent:])...)
					i++
					continue
				}
			}
			out = append(out, next)
			i++
		}
	}
	return []byte(strings.Join(out, "\n"))
}

// indentation counts the leading spaces of a line.
func indentation(line string) int {
	return len(line) - len(strings.TrimLeft(line, " "))
}

// fill breaks text at single spaces so each line, with its indent, fits
// lineWidth. A word longer than the width stands on its own line.
func fill(indent, text string) []string {
	avail := lineWidth - len(indent)
	var lines []string
	for utf8.RuneCountInString(text) > avail {
		cut := breakPoint(text, avail)
		if cut < 0 {
			break
		}
		lines = append(lines, indent+text[:cut])
		text = text[cut+1:]
	}
	return append(lines, indent+text)
}

// breakPoint is the byte index of the space to break at: the last
// single space whose prefix fits avail runes, else the first one after
// it, else -1. A single space is one bounded by non-blank characters on
// both sides, so neither line gains leading or trailing blanks.
func breakPoint(text string, avail int) int {
	last, first := -1, -1
	columns := 0
	for i, r := range text {
		if r == ' ' && i > 0 && !blank(text[i-1]) && i+1 < len(text) && !blank(text[i+1]) {
			if columns <= avail {
				last = i
			} else if first < 0 {
				first = i
			}
		}
		columns++
	}
	if last >= 0 {
		return last
	}
	return first
}

func blank(b byte) bool { return b == ' ' || b == '\t' }
