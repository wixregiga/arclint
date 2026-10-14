package typescript

import (
	"os"
	"path"
	"path/filepath"
	"sort"
	"strings"

	"github.com/odvcencio/gotreesitter"
)

// Skipped declaration files provide only uncertainty guards, never facts or
// declaration targets. Script declarations share a global scope in TypeScript;
// two files, or a language builtin, can therefore contribute to the same name.
func (r *resolutionIndex) recordDeclarationGuards(root string, ps *parsers) {
	globals := map[string]int{}
	for _, f := range r.files {
		r.recordAmbientTargets(f)
		if f.module && !f.globalAugmentation {
			continue
		}
		r.externalMembersQuestion = r.externalMembersQuestion || f.namespaceDeclaration
		for name, bindings := range f.scope.bindings {
			globals[name] += len(bindings)
		}
	}
	for name, count := range globals {
		if count > 1 || builtinGlobalNames[name] {
			r.globalQuestions[name] = "global declaration merging is not resolved"
		}
	}
	var skipped []string
	for file := range r.imports.files {
		if strings.HasSuffix(file, ".d.ts") {
			skipped = append(skipped, file)
		}
	}
	sort.Strings(skipped)
	for _, file := range skipped {
		f := r.readDeclarationGuard(root, file, ps)
		if f == nil {
			if r.declarationGuardFailure == "" {
				r.declarationGuardFailure = "declaration file " + file + " could not be parsed to rule out merging"
			}
			continue
		}
		r.recordAmbientTargets(f)
		if !f.module || f.globalAugmentation {
			for name := range f.scope.bindings {
				r.globalQuestions[name] = ".d.ts-only global declaration or merging is not resolved"
			}
			r.externalMembersQuestion = r.externalMembersQuestion || f.namespaceDeclaration
		}
		f.tree.Release()
	}
}

func (r *resolutionIndex) readDeclarationGuard(root, file string, ps *parsers) *resolutionFile {
	src, err := os.ReadFile(filepath.Join(root, filepath.FromSlash(file)))
	if err != nil {
		return nil
	}
	p, err := ps.forFile(file)
	if err != nil {
		return nil
	}
	tree, err := p.ParseStrict(src)
	if err != nil {
		return nil
	}
	f := &resolutionFile{path: file, src: src, lang: p.Language(), tree: tree, scope: newScope(nil), exports: map[string][]exportBinding{}, nodes: map[*gotreesitter.Node]*lexicalScope{}, fields: map[string]*gotreesitter.Node{}, types: map[string][]*gotreesitter.Node{}}
	f.scope.function = true
	f.index(tree.RootNode(), f.scope, "")
	return f
}

func (r *resolutionIndex) recordAmbientTargets(f *resolutionFile) {
	for _, spec := range f.ambientModules {
		r.ambientModules[spec] = true
		owner := r.imports.owners[f.path]
		if owner == nil {
			owner = r.guardProject(f.path)
		}
		_, _, file := r.imports.classify(owner, path.Dir(f.path), spec)
		if file != "" {
			r.ambientTargets[file] = true
		}
	}
}

func (r *resolutionIndex) guardProject(file string) *project {
	for dir := path.Dir(file); ; dir = path.Dir(dir) {
		if p := r.imports.configs.project(path.Join(dir, "tsconfig.json")); p != nil {
			return p
		}
		if dir == "." {
			return nil
		}
	}
}
