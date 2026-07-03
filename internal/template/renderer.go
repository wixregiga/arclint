package template

import (
	"bytes"
	"fmt"
	"strings"
)

// Renderer is the hand-rolled single-pass {{ }} interpolator
// (docs/design/templating.md §5). The grammar is: literal text, the escapes
// {{{{ -> {{ and }}}} -> }}, and tags of the form {{ var | filter | ... }}.
// Anything else is an error carrying a byte-exact position.
type Renderer struct {
	vars map[string]string
}

// NewRenderer builds a renderer over the fully-resolved variable map
// (manifest answers overlaid on built-ins, skipped variables present as "").
func NewRenderer(vars map[string]string) *Renderer {
	return &Renderer{vars: vars}
}

// Render interpolates src. origin names the source (a file path, a manifest
// field) and is used in error positions.
func (r *Renderer) Render(src []byte, origin string) ([]byte, error) {
	var out bytes.Buffer
	out.Grow(len(src))
	i := 0
	for i < len(src) {
		if bytes.HasPrefix(src[i:], []byte("{{{{")) {
			out.WriteString("{{")
			i += 4
			continue
		}
		if bytes.HasPrefix(src[i:], []byte("}}}}")) {
			out.WriteString("}}")
			i += 4
			continue
		}
		if bytes.HasPrefix(src[i:], []byte("{{")) {
			end := bytes.Index(src[i+2:], []byte("}}"))
			if end < 0 {
				return nil, posErrorf(origin, src, i, "unterminated {{ tag — close it with }} or escape a literal brace as {{{{")
			}
			body := string(src[i+2 : i+2+end])
			val, err := r.evalTag(body, origin, src, i)
			if err != nil {
				return nil, err
			}
			out.WriteString(val)
			i += 2 + end + 2
			continue
		}
		out.WriteByte(src[i])
		i++
	}
	return out.Bytes(), nil
}

// evalTag parses and evaluates one tag body: "var" or "var | filter | ...".
func (r *Renderer) evalTag(body, origin string, src []byte, off int) (string, error) {
	parts := strings.Split(body, "|")
	name := strings.TrimSpace(parts[0])
	if name == "" {
		return "", posErrorf(origin, src, off, "empty {{ }} tag — write {{ var }} or {{ var | filter }}")
	}
	if !identRE.MatchString(name) {
		return "", posErrorf(origin, src, off, "invalid variable name %q — names match %s", name, identRE)
	}
	val, ok := r.vars[name]
	if !ok {
		return "", posErrorf(origin, src, off, "unknown variable %q — declare it in template.yaml or use a built-in (repo_name, year, arclint_version)", name)
	}
	for _, raw := range parts[1:] {
		fname := strings.TrimSpace(raw)
		if fname == "" {
			return "", posErrorf(origin, src, off, "empty filter in tag %q — available filters: %s", body, strings.Join(filterNames(), ", "))
		}
		fn, ok := filters[fname]
		if !ok {
			return "", posErrorf(origin, src, off, "unknown filter %q — available filters: %s", fname, strings.Join(filterNames(), ", "))
		}
		val = fn(val)
	}
	return val, nil
}

// posErrorf formats an error as origin:line:col: message, computing the
// 1-based line and column from the byte offset.
func posErrorf(origin string, src []byte, off int, format string, args ...any) error {
	line := 1 + bytes.Count(src[:off], []byte("\n"))
	col := off - bytes.LastIndexByte(src[:off], '\n')
	return fmt.Errorf("%s:%d:%d: %s", origin, line, col, fmt.Sprintf(format, args...))
}
