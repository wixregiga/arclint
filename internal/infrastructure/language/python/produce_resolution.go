package python

import (
	"os"
	"path"
	"path/filepath"
	"sync"

	"github.com/wixregiga/arclint/internal/domain/conformance"
)

func resolvedFacts(root string, files []conformance.ObservedFile, res *resolver, modules *sync.Map, wantDecls, wantCalls, wantResolved, wantFields, wantImplementations bool) map[string]conformance.LanguageFacts {
	idx := &resolutionIndex{resolver: res, modules: map[string]*pyModule{}, patched: map[string]bool{}}
	out := map[string]conformance.LanguageFacts{}
	ps := newParsers()
	for _, file := range files {
		if !analyzable(file.Path) {
			continue
		}
		f := analyzeFile(ps, res, modules, root, file.Path, false, false)
		out[file.Path] = f
		src, err := os.ReadFile(filepath.Join(root, filepath.FromSlash(file.Path)))
		if err != nil {
			continue
		}
		df := extractDeclarations(ps, src)
		if df.ParseError != "" {
			continue
		}
		markAbstractClasses(df, origins{res: res, root: root, ps: ps, modules: modules}, path.Dir(file.Path))
		idx.modules[file.Path] = newModule(file.Path, df)
	}
	// Record repository-visible monkey patches before resolving any call.
	for _, m := range idx.modules {
		for _, mutation := range m.mutations {
			w := m.facts.walker
			obj := mutation.node.ChildByFieldName("object", w.lang)
			attr := mutation.node.ChildByFieldName(nodeAttribute, w.lang)
			if obj == nil || attr == nil {
				continue
			}
			if b := mutation.scope.lookup(obj.Text(w.src)); b != nil && b.kind == bindingSelf {
				continue
			}
			v := idx.expression(mutation.scope, obj)
			if v.module != "" {
				idx.patched[v.module+"::"+attr.Text(w.src)] = true
			}
			if v.class != nil {
				idx.patched[v.ref.File+":"+v.class.name+":"+attr.Text(w.src)] = true
			}
		}
	}
	for _, m := range idx.modules {
		for _, c := range m.calls {
			idx.recordSetattr(c)
		}
	}
	var implementations map[*pyClass][]conformance.Implementation
	if wantImplementations {
		implementations = idx.implementationsOf()
	}
	for file, m := range idx.modules {
		f := out[file]
		f.DeclarationsAvailable = wantDecls
		f.CallsAvailable = wantCalls
		f.ResolvedCallsAvailable = wantResolved
		f.FieldTypesAvailable = wantFields
		f.ImplementationsAvailable = wantImplementations
		for _, c := range m.declared {
			line := int(c.node.StartPoint().Row) + 1
			for i := range m.facts.Decls {
				if d := &m.facts.Decls[i]; d.Kind == declClass && d.Name == c.name && d.StartLine == line {
					d.Implements = implementations[c]
				}
			}
		}
		if wantFields {
			for _, field := range m.fields {
				refs := idx.annotationRefs(field.scope, field.annotation, map[*pyBinding]bool{})
				if len(refs) == 0 {
					refs = []conformance.Reference{unknown(m.facts.Decls[field.declaration].Name + " has no declared type")}
				}
				m.facts.Decls[field.declaration].Holds = refs
				own := unknown(m.facts.Decls[field.declaration].Name + " has no declared type")
				if field.annotation != nil {
					own = idx.annotationReceiver(field.scope, field.annotation, map[*pyBinding]bool{}).ref
				}
				m.facts.Decls[field.declaration].Type = own
			}
			idx.resultTypes(m)
			// A field the scope walk could not link to its assignment
			// still carries an answer: undetermined, never missing.
			for i := range m.facts.Decls {
				d := &m.facts.Decls[i]
				if d.Kind == declField && d.Type.IsZero() {
					reason := d.Name + " could not be linked to the scope that assigns it"
					d.Type, d.Holds = unknown(reason), []conformance.Reference{unknown(reason)}
				}
			}
		}
		if wantDecls {
			f.Declarations = m.facts.Decls
		}
		if wantCalls {
			f.Calls = m.facts.Calls
			if wantResolved {
				// Both walks are in source order. Definition defaults are collected
				// separately; use source offsets to associate calls, including calls
				// sharing a line or nested in another call.
				scopes := map[uint32]*pyScope{}
				for _, c := range m.calls {
					scopes[c.node.StartByte()] = c.scope
				}
				for i, node := range m.facts.CallNodes {
					if scope := scopes[node.StartByte()]; scope != nil {
						f.Calls[i].Target, f.Calls[i].Receiver = idx.call(pyCall{node, scope})
					} else {
						f.Calls[i].Target = unknown("call scope could not be established")
					}
				}
			}
		}
		out[file] = f
	}
	return out
}

// resultTypes names, for every def, the declaration its return
// annotation is, evaluated in the scope around the def; a def with no
// annotation is undetermined.
func (idx *resolutionIndex) resultTypes(m *pyModule) {
	w := m.facts.walker
	for _, fn := range m.functions {
		name, line := w.name(fn.node), int(fn.node.StartPoint().Row)+1
		ref := unknown(name + " has no return annotation")
		if rt := fn.node.ChildByFieldName("return_type", w.lang); rt != nil {
			ref = idx.annotationReceiver(fn.scope, rt, map[*pyBinding]bool{}).ref
		}
		for i := range m.facts.Decls {
			d := &m.facts.Decls[i]
			if (d.Kind == declMethod || d.Kind == bindingFunction) && d.Name == name && d.StartLine == line {
				d.Type = ref
			}
		}
	}
	for i := range m.facts.Decls {
		d := &m.facts.Decls[i]
		if (d.Kind == declMethod || d.Kind == bindingFunction) && d.Type.IsZero() {
			d.Type = unknown(d.Name + " could not be linked to the scope that defines it")
		}
	}
}

func (idx *resolutionIndex) recordSetattr(c pyCall) {
	w := c.scope.module.facts.walker
	fn := c.node.ChildByFieldName("function", w.lang)
	if fn == nil {
		return
	}
	v := idx.expression(c.scope, fn)
	if v.ref.Resolution != conformance.ResolutionExternal || v.ref.Name != builtinSetattr || v.ref.Package != "" && v.ref.Package != builtinsModule {
		return
	}
	args := c.node.ChildByFieldName("arguments", w.lang)
	if args == nil || args.NamedChildCount() < 2 {
		return
	}
	obj := idx.expression(c.scope, args.NamedChild(0))
	name, ok := pythonString(args.NamedChild(1).Text(w.src))
	if !ok {
		name = "*"
	}
	if obj.class != nil {
		idx.patched[obj.ref.File+":"+obj.class.name+":"+name] = true
	}
	if obj.module != "" {
		idx.patched[obj.module+"::"+name] = true
	}
}
