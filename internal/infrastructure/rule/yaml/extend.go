package yamlrule

import (
	"bytes"
	"errors"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strings"

	"gopkg.in/yaml.v3"

	"github.com/wixregiga/arclint/internal/application"
	"github.com/wixregiga/arclint/internal/domain/rule"
)

// Editor implements the application's RulesetEditor port over one
// ruleset file. It records an Installation by splicing lines at the
// positions the YAML parser reports, so comments, blank lines, quoting,
// and key order everywhere else in the document survive untouched.
type Editor struct {
	path string
}

// NewEditor binds the editor to a ruleset file path.
func NewEditor(path string) (Editor, error) {
	abs, err := filepath.Abs(path)
	if err != nil {
		return Editor{}, fmt.Errorf("ruleset path: %w", err)
	}
	return Editor{path: abs}, nil
}

// Path returns the absolute ruleset file path.
func (e Editor) Path() string { return e.path }

// Exists reports whether the ruleset file is present.
func (e Editor) Exists() (bool, error) {
	_, err := os.Stat(e.path)
	if errors.Is(err, fs.ErrNotExist) {
		return false, nil
	}
	if err != nil {
		return false, fmt.Errorf("ruleset %s: %w", e.path, err)
	}
	return true, nil
}

// Extend writes the Installation into the extends list. An entry that
// already extends the same Pattern (any version) is moved to the
// Installation's version and keeps its bindings. Otherwise a new entry
// is appended; a Module the ruleset declares itself under a Pattern
// Module's name is bound to its declared paths and the declaration is
// removed, because a Module cannot be both declared and bound.
func (e Editor) Extend(inst rule.Installation) (application.RulesetChange, error) {
	if inst.IsZero() {
		return application.RulesetChange{}, fmt.Errorf("extend ruleset: unconstructed installation")
	}
	data, err := os.ReadFile(e.path)
	if err != nil {
		return application.RulesetChange{}, fmt.Errorf("extend ruleset: %w", err)
	}
	parsed, err := parse(data, e.path)
	if err != nil {
		return application.RulesetChange{}, fmt.Errorf("extend ruleset: %w", err)
	}
	if parsed.pattern != nil {
		return application.RulesetChange{}, fmt.Errorf("extend ruleset: %s is a pattern distribution file, not a repository ruleset", e.path)
	}
	doc, err := newEditableDocument(data)
	if err != nil {
		return application.RulesetChange{}, fmt.Errorf("extend ruleset: %s: %w", e.path, err)
	}
	change, err := doc.extend(inst)
	if err != nil {
		return application.RulesetChange{}, fmt.Errorf("extend ruleset: %s: %w", e.path, err)
	}
	out := doc.bytes()
	if err := e.verify(out, inst.Reference()); err != nil {
		return application.RulesetChange{}, fmt.Errorf("extend ruleset: %s: %w", e.path, err)
	}
	if err := writeWhole(e.path, out); err != nil {
		return application.RulesetChange{}, fmt.Errorf("extend ruleset: %w", err)
	}
	change.Path = e.path
	return change, nil
}

// verify re-reads the edited document through the strict grammar and
// confirms the reference is extended exactly once.
func (e Editor) verify(data []byte, ref rule.PatternReference) error {
	parsed, err := parse(data, e.path)
	if err != nil {
		return fmt.Errorf("the edit produced an invalid ruleset, nothing was written: %w", err)
	}
	count := 0
	for _, ext := range parsed.extends {
		if ext.ref == ref {
			count++
		}
	}
	if count != 1 {
		return fmt.Errorf("the edit produced an invalid ruleset, nothing was written: %s is extended %d times", ref, count)
	}
	return nil
}

// editableDocument is the ruleset as lines plus its parsed node tree;
// edits are collected against original positions and applied bottom
// up so no position shifts under a later edit.
type editableDocument struct {
	lines    []string
	trailing bool
	top      *yaml.Node
	edits    []lineEdit
}

// lineEdit replaces lines [start, end) with replacement; start == end
// inserts before start.
type lineEdit struct {
	start, end  int
	replacement []string
}

func newEditableDocument(data []byte) (*editableDocument, error) {
	var root yaml.Node
	if err := yaml.NewDecoder(bytes.NewReader(data)).Decode(&root); err != nil {
		return nil, fmt.Errorf("parse ruleset: %w", err)
	}
	top := &root
	if top.Kind == yaml.DocumentNode && len(top.Content) > 0 {
		top = top.Content[0]
	}
	if top.Kind != yaml.MappingNode {
		return nil, fmt.Errorf("document: expected an object")
	}
	text := string(data)
	trailing := strings.HasSuffix(text, "\n")
	lines := strings.Split(text, "\n")
	if trailing {
		lines = lines[:len(lines)-1]
	}
	return &editableDocument{lines: lines, trailing: trailing, top: top}, nil
}

// key returns the top-level key and value nodes for name, and the
// index of the key in the mapping's content, or -1.
func (d *editableDocument) key(name string) (*yaml.Node, *yaml.Node, int) {
	for i := 0; i+1 < len(d.top.Content); i += 2 {
		if d.top.Content[i].Value == name {
			return d.top.Content[i], d.top.Content[i+1], i
		}
	}
	return nil, nil, -1
}

// nextTopKeyLine returns the 0-based line of the top-level key that
// follows content index i, or the line count when none follows.
func (d *editableDocument) nextTopKeyLine(i int) int {
	if i+2 < len(d.top.Content) {
		return d.top.Content[i+2].Line - 1
	}
	return len(d.lines)
}

func (d *editableDocument) extend(inst rule.Installation) (application.RulesetChange, error) {
	extendsKey, extendsVal, extendsIdx := d.key(keyExtends)
	if extendsVal != nil && extendsVal.Kind == yaml.SequenceNode {
		if existing := d.sameNamedEntry(extendsVal, inst.Reference()); existing != nil {
			return d.replaceEntry(existing, inst)
		}
	}
	// Fold declared Modules of the same names into the Bindings; the
	// Pattern now owns them and a declaration beside a Binding is an
	// error at load.
	adopted, err := d.foldDeclaredModules(&inst)
	if err != nil {
		return application.RulesetChange{}, err
	}
	change := application.RulesetChange{Installation: inst, Adopted: adopted}
	switch {
	case extendsVal == nil:
		d.insertExtendsBlock(inst)
	case extendsVal.Kind != yaml.SequenceNode:
		return application.RulesetChange{}, fmt.Errorf("extends: expected a list of {pattern, bind} entries")
	case extendsVal.Style&yaml.FlowStyle != 0:
		if len(extendsVal.Content) > 0 {
			return application.RulesetChange{}, fmt.Errorf("extends is written as a flow list; write it as a block list (one `- pattern:` per line) and retry")
		}
		if err := d.replaceEmptyFlowExtends(extendsKey, inst); err != nil {
			return application.RulesetChange{}, err
		}
	default:
		indent := strings.Repeat(" ", extendsVal.Column-1)
		at := d.blockEnd(d.nextTopKeyLine(extendsIdx), extendsVal.Line)
		d.edits = append(d.edits, lineEdit{start: at, end: at, replacement: entryLines(inst, indent)})
	}
	return change, nil
}

// sameNamedEntry finds the extends item naming the same namespace and
// name as ref.
func (d *editableDocument) sameNamedEntry(seq *yaml.Node, ref rule.PatternReference) *yaml.Node {
	for _, item := range seq.Content {
		if item.Kind != yaml.MappingNode {
			continue
		}
		pv := mappingValue(item, keyPattern)
		if pv == nil {
			continue
		}
		existing, err := rule.ParsePatternReference(pv.Value)
		if err != nil {
			continue
		}
		if existing.Namespace() == ref.Namespace() && existing.Name() == ref.Name() {
			return item
		}
	}
	return nil
}

// replaceEntry moves an existing entry to the Installation's version,
// keeping its bindings; the reported Installation carries them.
func (d *editableDocument) replaceEntry(item *yaml.Node, inst rule.Installation) (application.RulesetChange, error) {
	pv := mappingValue(item, keyPattern)
	existing, err := rule.ParsePatternReference(pv.Value)
	if err != nil {
		return application.RulesetChange{}, fmt.Errorf("extends entry at line %d: %w", item.Line, err)
	}
	if bv := mappingValue(item, keyBind); bv != nil {
		bindings, err := parseBindings(bv, "extends.bind")
		if err != nil {
			return application.RulesetChange{}, err
		}
		for _, b := range bindings {
			if inst, err = inst.Rebind(b.Module(), b.Paths()); err != nil {
				return application.RulesetChange{}, fmt.Errorf("the existing bind names a module %s does not list: %w", inst.Reference(), err)
			}
		}
	}
	change := application.RulesetChange{Installation: inst}
	if existing == inst.Reference() {
		return change, nil
	}
	change.Replaced = existing.Version()
	if err := d.replaceScalar(pv, inst.Reference().String()); err != nil {
		return application.RulesetChange{}, err
	}
	return change, nil
}

// replaceScalar rewrites one single-line scalar's text in place,
// keeping its quoting style.
func (d *editableDocument) replaceScalar(n *yaml.Node, value string) error {
	quote := ""
	switch n.Style {
	case yaml.DoubleQuotedStyle:
		quote = `"`
	case yaml.SingleQuotedStyle:
		quote = `'`
	case 0, yaml.TaggedStyle:
	default:
		return fmt.Errorf("line %d: the pattern reference is not a single-line scalar", n.Line)
	}
	line := n.Line - 1
	col := n.Column - 1
	raw := quote + n.Value + quote
	if line < 0 || line >= len(d.lines) || col+len(raw) > len(d.lines[line]) || d.lines[line][col:col+len(raw)] != raw {
		return fmt.Errorf("line %d: cannot locate the pattern reference %q", n.Line, n.Value)
	}
	text := d.lines[line]
	d.edits = append(d.edits, lineEdit{start: line, end: line + 1, replacement: []string{text[:col] + quote + value + quote + text[col+len(raw):]}})
	return nil
}

var emptyFlowExtends = regexp.MustCompile(`^(\s*)extends\s*:\s*\[\s*\]\s*(#.*)?$`)

// replaceEmptyFlowExtends turns `extends: []` into a block list.
func (d *editableDocument) replaceEmptyFlowExtends(keyNode *yaml.Node, inst rule.Installation) error {
	line := keyNode.Line - 1
	m := emptyFlowExtends.FindStringSubmatch(d.lines[line])
	if m == nil {
		return fmt.Errorf("line %d: cannot rewrite the empty extends list", keyNode.Line)
	}
	head := m[1] + "extends:"
	if m[2] != "" {
		head += " " + m[2]
	}
	d.edits = append(d.edits, lineEdit{start: line, end: line + 1, replacement: append([]string{head}, entryLines(inst, m[1]+"  ")...)})
	return nil
}

// insertExtendsBlock adds a whole extends section before modules, else
// before rules, else at the end of the document.
func (d *editableDocument) insertExtendsBlock(inst rule.Installation) {
	block := append([]string{"extends:"}, entryLines(inst, "  ")...)
	at := len(d.lines)
	for _, name := range []string{keyModules, keyRules} {
		if keyNode, _, _ := d.key(name); keyNode != nil {
			at = d.headStart(keyNode.Line-1, "")
			break
		}
	}
	if at > 0 && strings.TrimSpace(d.lines[at-1]) != "" {
		block = append([]string{""}, block...)
	}
	if at < len(d.lines) {
		block = append(block, "")
	}
	d.edits = append(d.edits, lineEdit{start: at, end: at, replacement: block})
}

// foldDeclaredModules rebinds every Pattern Module the ruleset declares
// under modules to the declared paths and removes the declarations.
func (d *editableDocument) foldDeclaredModules(inst *rule.Installation) ([]rule.ModuleName, error) {
	_, modulesVal, modulesIdx := d.key(keyModules)
	if modulesVal == nil || modulesVal.Kind != yaml.MappingNode {
		return nil, nil
	}
	if modulesVal.Style&yaml.FlowStyle != 0 {
		return nil, d.foldFlowModules(modulesVal, inst)
	}
	entries, err := parseModules(modulesVal, false)
	if err != nil {
		return nil, err
	}
	declared := map[rule.ModuleName][]rule.Glob{}
	for _, m := range entries {
		declared[m.name] = m.paths
	}
	var adopted []rule.ModuleName
	var removeKeys []int
	for _, m := range inst.Modules() {
		paths, ok := declared[m.Name()]
		if !ok {
			continue
		}
		rebound, err := inst.Rebind(m.Name(), paths)
		if err != nil {
			return nil, fmt.Errorf("adopt declared module %s: %w", m.Name(), err)
		}
		*inst = rebound
		adopted = append(adopted, m.Name())
		for i := 0; i+1 < len(modulesVal.Content); i += 2 {
			if modulesVal.Content[i].Value == m.Name().String() {
				removeKeys = append(removeKeys, i)
			}
		}
	}
	if len(removeKeys) == 0 {
		return nil, nil
	}
	if len(removeKeys)*2 == len(modulesVal.Content) {
		keyNode, _, _ := d.key(keyModules)
		start := d.headStart(keyNode.Line-1, "")
		end := d.blockEnd(d.nextTopKeyLine(modulesIdx), keyNode.Line)
		d.edits = append(d.edits, lineEdit{start: start, end: end})
		return adopted, nil
	}
	sectionEnd := d.nextTopKeyLine(modulesIdx)
	for _, i := range removeKeys {
		keyNode := modulesVal.Content[i]
		indent := strings.Repeat(" ", keyNode.Column-1)
		start := d.headStart(keyNode.Line-1, indent)
		end := sectionEnd
		if i+2 < len(modulesVal.Content) {
			end = modulesVal.Content[i+2].Line - 1
		}
		end = d.blockEnd(end, keyNode.Line)
		d.edits = append(d.edits, lineEdit{start: start, end: end})
	}
	return adopted, nil
}

// foldFlowModules refuses to edit a flow-style modules map, which has
// no line structure to splice; the owner rewrites it as a block map.
func (d *editableDocument) foldFlowModules(modulesVal *yaml.Node, inst *rule.Installation) error {
	for _, m := range inst.Modules() {
		if mappingValue(modulesVal, m.Name().String()) != nil {
			return fmt.Errorf("modules declares %q, which the pattern binds, as part of a flow map; write modules as a block map and retry", m.Name())
		}
	}
	return nil
}

// headStart walks back from line over the comment lines at exactly
// that indent, which belong to it, returning the first line of the
// block.
func (d *editableDocument) headStart(line int, indent string) int {
	for line > 0 {
		prev := d.lines[line-1]
		if !strings.HasPrefix(prev, indent+"#") {
			break
		}
		line--
	}
	return line
}

// blockEnd walks back from end over blank and comment lines, which
// belong to whatever follows, but never before floor (a 1-based line).
func (d *editableDocument) blockEnd(end, floor int) int {
	for end > floor && end-1 < len(d.lines) {
		t := strings.TrimSpace(d.lines[end-1])
		if t != "" && !strings.HasPrefix(t, "#") {
			break
		}
		end--
	}
	return end
}

// bytes applies the collected edits bottom up, so no edit shifts the
// original position of another, then re-joins the lines. Where an
// edit left two blank lines together one is dropped.
func (d *editableDocument) bytes() []byte {
	edits := append([]lineEdit(nil), d.edits...)
	sort.SliceStable(edits, func(i, j int) bool {
		if edits[i].start != edits[j].start {
			return edits[i].start > edits[j].start
		}
		return edits[i].end > edits[j].end
	})
	lines := d.lines
	seams := make([]int, 0, 2*len(edits))
	for _, e := range edits {
		delta := len(e.replacement) - (e.end - e.start)
		for i := range seams {
			seams[i] += delta
		}
		next := make([]string, 0, len(lines)+delta)
		next = append(next, lines[:e.start]...)
		next = append(next, e.replacement...)
		next = append(next, lines[e.end:]...)
		lines = next
		seams = append(seams, e.start, e.start+len(e.replacement))
	}
	sort.Sort(sort.Reverse(sort.IntSlice(seams)))
	for _, at := range seams {
		if at > 0 && at < len(lines) && strings.TrimSpace(lines[at]) == "" && strings.TrimSpace(lines[at-1]) == "" {
			lines = append(lines[:at], lines[at+1:]...)
		}
	}
	text := strings.Join(lines, "\n")
	if d.trailing || len(lines) > 0 {
		text += "\n"
	}
	return []byte(text)
}

// entryLines renders one extends item at the given dash indent: the
// reference, then a bind entry per Pattern Module, commented out when
// the Installation leaves the Module unbound. When nothing is bound the
// whole bind block is commented, so the document stays valid and the
// loader names the unbound Modules.
func entryLines(inst rule.Installation, indent string) []string {
	lines := []string{indent + "- pattern: " + inst.Reference().String()}
	off := ""
	if len(inst.Bindings()) == 0 {
		off = "# "
	}
	lines = append(lines, indent+"  "+off+"bind:")
	for _, m := range inst.Modules() {
		b, ok := inst.Binding(m.Name())
		if !ok {
			lines = append(lines, indent+"    # "+m.Description())
			lines = append(lines, indent+"    # "+m.Name().String()+": <glob>")
			continue
		}
		lines = append(lines, indent+"    "+m.Name().String()+": "+bindPaths(b.Paths()))
	}
	return lines
}

// bindPaths spells a Binding's paths: one quoted glob, or a flow list.
func bindPaths(paths []rule.Glob) string {
	if len(paths) == 1 {
		return quoteGlob(paths[0].String())
	}
	quoted := make([]string, 0, len(paths))
	for _, g := range paths {
		quoted = append(quoted, quoteGlob(g.String()))
	}
	return "[" + strings.Join(quoted, ", ") + "]"
}

// quoteGlob spells a glob as a double-quoted YAML scalar so `*` and
// `{` never read as YAML syntax.
func quoteGlob(s string) string {
	return `"` + strings.NewReplacer(`\`, `\\`, `"`, `\"`).Replace(s) + `"`
}

// mappingValue returns the value node under key in a mapping node.
func mappingValue(m *yaml.Node, key string) *yaml.Node {
	for i := 0; i+1 < len(m.Content); i += 2 {
		if m.Content[i].Value == key {
			return m.Content[i+1]
		}
	}
	return nil
}

// writeWhole replaces path through a temporary sibling, keeping the
// file's mode and leaving no temporary file behind on any failure.
func writeWhole(path string, data []byte) error {
	mode := fs.FileMode(0o600)
	if info, err := os.Stat(path); err == nil {
		mode = info.Mode().Perm()
	}
	tmp, err := os.CreateTemp(filepath.Dir(path), "."+filepath.Base(path)+".tmp-")
	if err != nil {
		return fmt.Errorf("create temp for %s: %w", path, err)
	}
	tmpName := tmp.Name()
	cleanup := true
	defer func() {
		if cleanup {
			_ = os.Remove(tmpName)
		}
	}()
	if _, err := tmp.Write(data); err != nil {
		_ = tmp.Close()
		return fmt.Errorf("write temp for %s: %w", path, err)
	}
	if err := tmp.Chmod(mode); err != nil {
		_ = tmp.Close()
		return fmt.Errorf("chmod temp for %s: %w", path, err)
	}
	if err := tmp.Close(); err != nil {
		return fmt.Errorf("close temp for %s: %w", path, err)
	}
	if err := os.Rename(tmpName, path); err != nil {
		return fmt.Errorf("rename temp onto %s: %w", path, err)
	}
	cleanup = false
	return nil
}
