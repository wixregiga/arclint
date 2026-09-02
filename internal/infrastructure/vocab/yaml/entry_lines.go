package yamlvocab

import "gopkg.in/yaml.v3"

// aliasHopLimit bounds alias resolution so a self-referencing chain
// cannot spin.
const aliasHopLimit = 64

// documentLines is where every recorded entry is written in the
// vocabulary file, indexed the way the decoded document is: contexts
// in file order, relations in file order.
type documentLines struct {
	contexts  []contextLines
	relations []int
}

// contextLines is where one bounded context is written, plus where the
// entries of its sections are written, in file order.
type contextLines struct {
	line           int
	entities       []int
	valueObjects   []int
	invariants     []int
	assertions     []int
	specifications []int
	events         []int
}

// context returns the lines of the i-th recorded context. An index the
// node tree does not reach resolves to no line rather than to a wrong
// one.
func (d documentLines) context(i int) contextLines {
	if i < 0 || i >= len(d.contexts) {
		return contextLines{}
	}
	return d.contexts[i]
}

// lineAt returns the i-th recorded line, 0 when the node tree records
// none.
func lineAt(lines []int, i int) int {
	if i < 0 || i >= len(lines) {
		return 0
	}
	return lines[i]
}

// readEntryLines walks the vocabulary content as a node tree and
// records where each context, term, invariant, and relation is
// written. Content that does not parse yields no lines: the strict
// decode in parse is what reports the failure.
func readEntryLines(data []byte) documentLines {
	var root yaml.Node
	if err := yaml.Unmarshal(data, &root); err != nil {
		return documentLines{}
	}
	body := documentBody(&root)
	if body == nil {
		return documentLines{}
	}
	var out documentLines
	_, contexts := mappingEntry(body, "contexts")
	for _, item := range sequenceItems(contexts) {
		out.contexts = append(out.contexts, readContextLines(item))
	}
	_, relations := mappingEntry(body, "relations")
	out.relations = entryLines(relations, "from")
	return out
}

// readContextLines records where one bounded context and each of its
// recorded entries are written.
func readContextLines(item *yaml.Node) contextLines {
	at := contextLines{line: entryLine(item, keyName)}
	body := resolveAlias(item)
	if body == nil || body.Kind != yaml.MappingNode {
		return at
	}
	for _, section := range []struct {
		key   string
		names string
		into  *[]int
	}{
		{"entities", keyName, &at.entities},
		{"value_objects", keyName, &at.valueObjects},
		{"invariants", keyStatement, &at.invariants},
		{"assertions", keyStatement, &at.assertions},
		{"specifications", keyName, &at.specifications},
		{"events", keyName, &at.events},
	} {
		_, seq := mappingEntry(body, section.key)
		*section.into = entryLines(seq, section.names)
	}
	return at
}

// entryLines records one line per item of a recorded section, anchored
// at the key that names the entry.
func entryLines(seq *yaml.Node, names string) []int {
	items := sequenceItems(seq)
	if len(items) == 0 {
		return nil
	}
	out := make([]int, len(items))
	for i, item := range items {
		out[i] = entryLine(item, names)
	}
	return out
}

// entryLine anchors one entry at the key that names it, falling back
// to where the entry itself starts. An entry written as an alias
// anchors where the alias is used, which is where the reader finds it
// in this list.
func entryLine(item *yaml.Node, names string) int {
	if item == nil {
		return 0
	}
	if item.Kind == yaml.MappingNode {
		if k, _ := mappingEntry(item, names); k != nil {
			return k.Line
		}
	}
	return item.Line
}

// documentBody returns the mapping the vocabulary document holds.
func documentBody(root *yaml.Node) *yaml.Node {
	n := root
	if n != nil && n.Kind == yaml.DocumentNode && len(n.Content) > 0 {
		n = n.Content[0]
	}
	n = resolveAlias(n)
	if n == nil || n.Kind != yaml.MappingNode {
		return nil
	}
	return n
}

// mappingEntry returns the key and value nodes recorded under key.
func mappingEntry(m *yaml.Node, key string) (k, v *yaml.Node) {
	m = resolveAlias(m)
	if m == nil || m.Kind != yaml.MappingNode {
		return nil, nil
	}
	for i := 0; i+1 < len(m.Content); i += 2 {
		if m.Content[i].Value == key {
			return m.Content[i], m.Content[i+1]
		}
	}
	return nil, nil
}

// sequenceItems returns the items of a recorded list, following an
// alias to the list it names.
func sequenceItems(n *yaml.Node) []*yaml.Node {
	n = resolveAlias(n)
	if n == nil || n.Kind != yaml.SequenceNode {
		return nil
	}
	return n.Content
}

// resolveAlias follows an alias to the node it names.
func resolveAlias(n *yaml.Node) *yaml.Node {
	for hops := 0; n != nil && n.Kind == yaml.AliasNode && hops < aliasHopLimit; hops++ {
		n = n.Alias
	}
	if n != nil && n.Kind == yaml.AliasNode {
		return nil
	}
	return n
}
