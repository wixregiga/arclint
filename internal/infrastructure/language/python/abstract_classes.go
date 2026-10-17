package python

import (
	"strings"

	"github.com/odvcencio/gotreesitter"
)

// abstractBases are the bases whose naming declares a class abstract:
// an abstract base class through abc.ABC, or a protocol, which PEP 544
// declares only by naming typing.Protocol among the class's own bases.
var abstractBases = map[string]bool{
	"abc.ABC":                    true,
	"typing.Protocol":            true,
	"typing_extensions.Protocol": true,
}

// abstractMetaclass is the metaclass whose naming declares a class an
// abstract base class.
const abstractMetaclass = "abc.ABCMeta"

// markAbstractClasses marks every class whose own declaration states
// that it is abstract: a base naming abc.ABC or a Protocol, or the
// metaclass abc.ABCMeta. Each name is looked up in the scope the class
// statement runs in, the way Python looks it up, so an import counts
// only when no nearer binding, rebinding, or star import competes with
// it. An import through a module of the repository is followed to what
// that module imports, so a re-export of abc.ABC is abc.ABC, while a
// module of the repository that only shares the library's name is not
// the library. A file that assigns any name on that way, abc.ABC =
// object or facade.ABC = object, has replaced it. A class that inherits abstractness from a
// repository base without stating it is not marked.
func markAbstractClasses(df *declarationFacts, from importOrigin, dir string) {
	if df.ParseError != "" || df.module == nil {
		// Facts that did not parse hold no classes to mark.
		return
	}
	for _, c := range df.module.declared {
		if !declaresAbstract(df, c, from, dir) {
			continue
		}
		line := int(c.node.StartPoint().Row) + 1
		for i := range df.Decls {
			if d := &df.Decls[i]; d.Kind == declClass && d.Name == c.name && d.StartLine == line {
				d.Abstract = true
			}
		}
	}
}

// importOrigin follows an imported name to where it comes from.
type importOrigin interface {
	// repository reports whether a module imported from a file of the
	// directory is a module of the repository.
	repository(dir, module string) bool
	// follow names what a dotted path reaches in a module of the
	// repository, its first name qualified by that module's own imports
	// and the attributes after it kept, with the module's directory.
	// bound is false when the module does not resolve to one file or does
	// not bind the first name; a bound name the module defines itself,
	// or replaces, reaches "".
	follow(dir, module, path string) (string, string, bool)
}

// declaresAbstract reports whether a class's base list names an
// abstract base or the abstract metaclass: ABC, (ABC), Protocol[T],
// abc.ABC, or metaclass=ABCMeta.
func declaresAbstract(df *declarationFacts, c *pyClass, from importOrigin, dir string) bool {
	w := df.walker
	bases := c.node.ChildByFieldName("superclasses", w.lang)
	if bases == nil {
		return false
	}
	names := func(n *gotreesitter.Node) string {
		n = w.unparenthesized(n)
		if n.Type(w.lang) == nodeSubscript {
			if value := n.ChildByFieldName("value", w.lang); value != nil {
				n = w.unparenthesized(value)
			}
		}
		switch n.Type(w.lang) {
		case nodeIdentifier, nodeAttribute:
		default:
			return ""
		}
		written := w.dottedName(n)
		chain := libraryChain(qualifiedTarget(c.header, written, w, 0), from, dir, explicitModules(w))
		if len(chain) == 0 {
			return ""
		}
		// The file's own writes replace the base only when they run
		// before the class reads it: an attribute path is read by the
		// class statement, a bare name where it was bound. Every later
		// name on the chain is another module's, which follow judged.
		read := c.node.StartByte()
		if !strings.Contains(written, ".") {
			read = captureOffset(c.header, written, w, read)
		}
		if mutatesBefore(df.module, chain[0], read) {
			return ""
		}
		return chain[len(chain)-1]
	}
	for i := 0; i < bases.NamedChildCount(); i++ {
		arg := bases.NamedChild(i)
		if arg.Type(w.lang) == "keyword_argument" {
			name := arg.ChildByFieldName("name", w.lang)
			value := arg.ChildByFieldName("value", w.lang)
			if name != nil && value != nil && name.Text(w.src) == "metaclass" && names(value) == abstractMetaclass {
				return true
			}
			continue
		}
		if abstractBases[names(arg)] {
			return true
		}
	}
	return false
}

// libraryChain follows a qualified name through the repository's
// modules to the library name it reaches, and lists every name on the
// way, the library name last: facade.abstract.ABC, where facade imports
// abc as abstract, is facade.abstract.ABC, then abc.ABC. Where the name
// splits into a module and the names read from it is decided the way
// Python reads it: a module path the file imports explicitly, the
// longest first, then a package's own binding before a submodule of
// the same name. explicit holds the module paths the file imports. Nil
// when the name ends in the repository or cannot be followed.
func libraryChain(q string, from importOrigin, dir string, explicit map[string]bool) []string {
	var chain []string
	for hop := 0; q != "" && hop < maxAliasHops; hop++ {
		chain = append(chain, q)
		relative := strings.HasPrefix(q, ".")
		top, _, dotted := strings.Cut(strings.TrimLeft(q, "."), ".")
		if !dotted && !relative || !relative && !from.repository(dir, top) {
			return chain
		}
		next := ""
		lead := len(q) - len(strings.TrimLeft(q, "."))
		for _, at := range orderedSplits(q, lead, explicit) {
			module, rest := q[:at], q[at+1:]
			if at < lead {
				module, rest = q[:lead], q[lead:]
			}
			followed, moduleDir, bound := from.follow(dir, module, rest)
			if !bound {
				continue
			}
			// A module that binds the name decides it, even to something
			// that is not an import.
			next, dir = followed, moduleDir
			break
		}
		q = next
		explicit = nil
	}
	return nil
}

// orderedSplits orders the dots that may separate a module from the
// names read from it: the module paths imported explicitly, longest
// first, since the import makes each the attribute its package holds,
// then the others shortest first, since a package's own binding is the
// attribute until a submodule import replaces it.
func orderedSplits(q string, lead int, explicit map[string]bool) []int {
	all := splitPoints(q, lead)
	var imported, rest []int
	for _, at := range all {
		module := q[:at]
		if at < lead {
			module = q[:lead]
		}
		if explicit[module] {
			imported = append(imported, at)
		} else {
			rest = append(rest, at)
		}
	}
	for l, r := 0, len(rest)-1; l < r; l, r = l+1, r-1 {
		rest[l], rest[r] = rest[r], rest[l]
	}
	return append(imported, rest...)
}

// explicitModules lists the module paths a file imports by name, each
// with the packages above it: import a.b.c imports a, a.b, and a.b.c,
// and from a.b import c imports a and a.b.
func explicitModules(w *pyWalker) map[string]bool {
	out := map[string]bool{}
	for _, imp := range w.imports {
		module := imp.loaded
		if imp.member != "" || module == "" {
			module = imp.module
		}
		if strings.HasPrefix(module, ".") {
			out[module] = true
			continue
		}
		for at := 0; at >= 0; {
			next := strings.Index(module[at:], ".")
			if next < 0 {
				out[module] = true
				break
			}
			out[module[:at+next]] = true
			at += next + 1
		}
	}
	return out
}

// noLimit counts every write of a module, wherever it runs.
const noLimit = ^uint32(0)

// captureOffset is where a bare name took the value it holds: the
// import or assignment that bound it, through an alias of another bare
// name to where that one was bound; fallback when nothing binds it.
func captureOffset(s *pyScope, name string, w *pyWalker, fallback uint32) uint32 {
	for hop := 0; hop < maxAliasHops; hop++ {
		b := s.lookup(name)
		if b == nil || b.node == nil {
			return fallback
		}
		alias := ""
		if b.kind != bindingImport {
			alias = aliasValue(b, w, hop)
		}
		if alias == "" || strings.Contains(alias, ".") {
			return b.node.StartByte()
		}
		s, name = b.scope, alias
	}
	return fallback
}

// mutatesBefore reports whether a module replaces a qualified name or a
// prefix of it in a statement that starts before limit: by assigning
// it, (abc).ABC = object as much as abc.ABC = object, or through the
// builtin setattr. An annotation alone, abc.ABC: type, assigns nothing.
// A setattr whose attribute is not a string literal may replace any
// attribute of its object.
func mutatesBefore(m *pyModule, q string, limit uint32) bool {
	w := m.facts.walker
	covers := func(target string) bool {
		return target != "" && (q == target || strings.HasPrefix(q, target+"."))
	}
	runs := func(n *gotreesitter.Node, s *pyScope) bool { return runsBefore(m, n, s, limit) }
	for _, mutation := range m.mutations {
		if !runs(mutation.node, mutation.scope) || !assigns(mutation.node, w) {
			continue
		}
		if covers(qualifiedTarget(mutation.scope, w.dottedName(mutation.node), w, 0)) {
			return true
		}
	}
	for _, call := range m.calls {
		if !runs(call.node, call.scope) {
			continue
		}
		object, attribute, ok := setattrTarget(call, w)
		if !ok {
			continue
		}
		target := qualifiedTarget(call.scope, object, w, 0)
		if target == "" {
			continue
		}
		if attribute == "" {
			// Some attribute of the object is replaced, never the object.
			if strings.HasPrefix(q, target+".") {
				return true
			}
			continue
		}
		if covers(target + "." + attribute) {
			return true
		}
	}
	return false
}

// assigns reports whether the statement writing an attribute target
// gives it a value: an annotation alone does not.
func assigns(target *gotreesitter.Node, w *pyWalker) bool {
	parent := target.Parent()
	return parent == nil || parent.Type(w.lang) != nodeAssignment || parent.ChildByFieldName("right", w.lang) != nil
}

// setattrTarget reads a call of the builtin setattr: the dotted object
// it sets an attribute on, and the attribute when a string literal
// names it, "" otherwise. ok is false for any other call, and for a
// setattr whose name the module binds to something else.
func setattrTarget(call pyCall, w *pyWalker) (string, string, bool) {
	fn := call.node.ChildByFieldName("function", w.lang)
	if fn == nil || fn.Type(w.lang) != nodeIdentifier || fn.Text(w.src) != builtinSetattr || call.scope.lookup(builtinSetattr) != nil {
		return "", "", false
	}
	args := call.node.ChildByFieldName("arguments", w.lang)
	if args == nil || args.NamedChildCount() < 2 {
		return "", "", false
	}
	object := w.dottedName(args.NamedChild(0))
	if object == "" {
		return "", "", false
	}
	attribute, literal := stringLiteral(args.NamedChild(1), w)
	if !literal {
		attribute = ""
	}
	return object, attribute, true
}

// stringLiteral decodes a string literal, adjacent literals joined as
// Python joins them: "A" "BC" is ABC. ok is false for anything else.
func stringLiteral(n *gotreesitter.Node, w *pyWalker) (string, bool) {
	n = w.unparenthesized(n)
	switch n.Type(w.lang) {
	case nodeString:
		return pythonString(n.Text(w.src))
	case "concatenated_string":
		var joined strings.Builder
		for i := 0; i < n.NamedChildCount(); i++ {
			part, ok := stringLiteral(n.NamedChild(i), w)
			if !ok {
				return "", false
			}
			joined.WriteString(part)
		}
		return joined.String(), true
	}
	return "", false
}

// runsBefore reports whether a statement of a module runs on import
// before limit: at module level, or in a class body there, before limit;
// or inside a function that a call running on import before limit
// calls by its name. A function nobody calls on import writes nothing
// the class can see.
func runsBefore(m *pyModule, n *gotreesitter.Node, s *pyScope, limit uint32) bool {
	if runsOnImport(m, s) {
		return n.StartByte() < limit
	}
	def := enclosingDef(n, m.facts.walker)
	if def == nil {
		return false
	}
	w := m.facts.walker
	for _, call := range m.calls {
		if call.node.StartByte() >= limit || !runsOnImport(m, call.scope) {
			continue
		}
		fn := call.node.ChildByFieldName("function", w.lang)
		if fn == nil || fn.Type(w.lang) != nodeIdentifier {
			continue
		}
		if b := call.scope.lookup(fn.Text(w.src)); b != nil && b.kind == bindingFunction && b.node == def {
			return true
		}
	}
	return false
}

// runsOnImport reports whether code in a scope runs when its module is
// imported: the module's own scope, or a class body within it.
func runsOnImport(m *pyModule, s *pyScope) bool {
	for ; s != nil; s = s.parent {
		if s == m.scope {
			return true
		}
		if !s.classBody {
			return false
		}
	}
	return false
}

// enclosingDef is the function definition a node sits in, nil at module
// level.
func enclosingDef(n *gotreesitter.Node, w *pyWalker) *gotreesitter.Node {
	for p := n.Parent(); p != nil; p = p.Parent() {
		if p.Type(w.lang) == nodeFunction {
			return p
		}
	}
	return nil
}

// splitPoints lists, right to left, the dots that may separate a module
// from the names in it, and the end of the leading dots of a relative
// name, where the package itself is the module.
func splitPoints(q string, lead int) []int {
	var out []int
	for at := strings.LastIndex(q, "."); at >= lead; at = strings.LastIndex(q[:at], ".") {
		out = append(out, at)
	}
	if lead > 0 {
		out = append(out, lead-1)
	}
	return out
}
