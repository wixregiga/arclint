package main

import (
	"fmt"
	"io/fs"
	"path"
	"regexp"
	"sort"
	"strings"
)

// exampleRoots are the worked examples whose sources carry ANCHOR
// markers, relative to the repository root.
var exampleRoots = []string{"docs/examples/go", "docs/examples/ts"}

// skippedDirs are directories under an example that hold installed or
// built output, never quoted source.
var skippedDirs = map[string]bool{"node_modules": true, "dist": true}

// marker matches one ANCHOR or ANCHOR_END line in a Go or TypeScript
// comment (//) or a YAML comment (#).
var marker = regexp.MustCompile(`^\s*(?://|#) (ANCHOR|ANCHOR_END): ([a-z0-9-]+)\s*$`)

// snippet is the text between one pair of markers, with the file it was
// quoted from.
type snippet struct {
	file string
	text string
}

// snippets holds every anchored region of the examples, keyed by the
// file's example root and the anchor name ("docs/examples/go#query"),
// and remembers which ones the page quoted.
type snippets struct {
	regions map[string]snippet
	used    map[string]bool
}

// readSnippets reads the anchored regions of every example. A marker
// opened twice in one example, closed without being opened, or left
// open refuses the read, and so does a region with no text.
func readSnippets(repo fs.FS) (*snippets, error) {
	s := &snippets{regions: map[string]snippet{}, used: map[string]bool{}}
	for _, root := range exampleRoots {
		err := fs.WalkDir(repo, root, func(p string, d fs.DirEntry, err error) error {
			if err != nil {
				return err
			}
			if d.IsDir() {
				if skippedDirs[d.Name()] {
					return fs.SkipDir
				}
				return nil
			}
			switch path.Ext(p) {
			case ".go", ".ts", ".yaml":
			default:
				return nil
			}
			data, err := fs.ReadFile(repo, p)
			if err != nil {
				return fmt.Errorf("read %s: %w", p, err)
			}
			return s.scan(root, p, string(data))
		})
		if err != nil {
			return nil, fmt.Errorf("read the anchors of %s: %w", root, err)
		}
	}
	return s, nil
}

func (s *snippets) scan(root, file, content string) error {
	open := map[string][]string{}
	var order []string
	for i, line := range strings.Split(content, "\n") {
		m := marker.FindStringSubmatch(line)
		if m == nil {
			for _, name := range order {
				open[name] = append(open[name], line)
			}
			continue
		}
		name, key := m[2], root+"#"+m[2]
		switch m[1] {
		case "ANCHOR":
			if _, dup := s.regions[key]; dup || open[name] != nil {
				return fmt.Errorf("%s:%d: anchor %q opened twice in %s", file, i+1, name, root)
			}
			open[name] = []string{}
			order = append(order, name)
		case "ANCHOR_END":
			lines, ok := open[name]
			if !ok {
				return fmt.Errorf("%s:%d: anchor %q closed without being opened", file, i+1, name)
			}
			text := dedent(lines)
			if text == "" {
				return fmt.Errorf("%s:%d: anchor %q holds no text", file, i+1, name)
			}
			s.regions[key] = snippet{file: strings.TrimPrefix(file, root+"/"), text: text}
			delete(open, name)
			order = remove(order, name)
		}
	}
	if len(order) > 0 {
		return fmt.Errorf("%s: anchor %q is never closed", file, order[0])
	}
	return nil
}

// get returns one anchored region of one example and marks it used.
func (s *snippets) get(root, name string) (snippet, error) {
	key := root + "#" + name
	sn, ok := s.regions[key]
	if !ok {
		return snippet{}, fmt.Errorf("%s: no anchor %q", root, name)
	}
	s.used[key] = true
	return sn, nil
}

// unused lists the anchored regions the page never quoted; a marker the
// page does not read is a marker that no longer says anything.
func (s *snippets) unused() []string {
	var out []string
	for key := range s.regions {
		if !s.used[key] {
			out = append(out, key)
		}
	}
	sort.Strings(out)
	return out
}

// dedent drops leading and trailing blank lines and the indentation
// every remaining non-blank line shares.
func dedent(lines []string) string {
	for len(lines) > 0 && strings.TrimSpace(lines[0]) == "" {
		lines = lines[1:]
	}
	for len(lines) > 0 && strings.TrimSpace(lines[len(lines)-1]) == "" {
		lines = lines[:len(lines)-1]
	}
	if len(lines) == 0 {
		return ""
	}
	prefix := lines[0][:len(lines[0])-len(strings.TrimLeft(lines[0], " \t"))]
	for _, line := range lines[1:] {
		if strings.TrimSpace(line) == "" {
			continue
		}
		for !strings.HasPrefix(line, prefix) {
			prefix = prefix[:len(prefix)-1]
		}
	}
	out := make([]string, len(lines))
	for i, line := range lines {
		out[i] = strings.TrimRight(strings.TrimPrefix(line, prefix), " \t")
	}
	return strings.Join(out, "\n")
}

func remove(names []string, name string) []string {
	out := names[:0]
	for _, n := range names {
		if n != name {
			out = append(out, n)
		}
	}
	return out
}
