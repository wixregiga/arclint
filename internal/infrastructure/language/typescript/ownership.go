package typescript

import "path"

// Which configuration owns a file, and so whose paths and baseUrl its
// imports resolve through, follows the TypeScript language service
// (tsserver 5.9, editorServices.ts: forEachConfigFileLocation,
// tryFindDefaultConfiguredProjectForOpenScriptInfoOrClosedFileInfo,
// forEachAncestorProjectLoad, forEachResolvedProjectReferenceProjectLoad;
// TypeScript 5.7 release notes, "Searching Ancestor Configuration Files
// for Project Ownership"):
//
//  1. From the file's directory up, each directory offers tsconfig.json,
//     then jsconfig.json; the search goes no further than a
//     node_modules directory.
//  2. A configuration found that way owns the file when its program
//     holds it: its files, include and exclude select the file, or a
//     file they select imports it, directly or through other imports.
//     A file that one of its project references selects belongs to that
//     reference instead.
//  3. Otherwise its project references are searched: every direct
//     reference in order, then the references of each in turn. A
//     reference owns the file only when its own files, include and
//     exclude select it.
//  4. Otherwise the search climbs to the next configuration, unless
//     this one sets disableSolutionSearching.
//
// The search ends at the repository root, as tsserver's ends at the
// root of the workspace an editor opened, and reads only observed
// configuration files. A file no configuration owns gets no aliases.
// The import walk of step 2 follows the files this producer analyzes,
// .ts and .tsx, through relative and alias imports.

// configNames are the configuration files each directory offers, in
// the order tsserver tries them.
var configNames = []string{"tsconfig.json", "jsconfig.json"}

// own records the owning configuration of every analyzed file, given
// the specifiers each one imports: ownership needs every file's
// imports, because a configuration also owns the files its own files
// import.
func (r *resolver) own(specs map[string][]string) {
	r.specs = specs
	r.owners = make(map[string]*project, len(specs))
	r.selected = map[string]map[string]bool{}
	r.imported = map[string]map[string]bool{}
	r.reachable = map[string][]*project{}
	for file := range specs {
		r.owners[file] = r.ownerOf(file)
	}
}

// ownerOf searches for the configuration that owns file, nil for none.
func (r *resolver) ownerOf(file string) *project {
	seen := map[string]bool{}
	for cfg := r.nextConfig(path.Dir(file), ""); cfg != ""; cfg = r.nextConfig(path.Dir(cfg), path.Base(cfg)) {
		seen[cfg] = true
		p := r.configs.project(cfg)
		if p == nil {
			continue
		}
		if r.selection(p)[file] || r.importedBy(p)[file] {
			return p
		}
		if o := r.referenceOwning(p, file, seen); o != nil {
			return o
		}
		if p.disableSolutionSearching {
			return nil
		}
	}
	return nil
}

// selection returns the analyzed files p selects, computed once: the
// search asks the same configuration about many files.
func (r *resolver) selection(p *project) map[string]bool {
	if selected, ok := r.selected[p.file]; ok {
		return selected
	}
	selected := map[string]bool{}
	for file := range r.specs {
		if p.includes(file) {
			selected[file] = true
		}
	}
	r.selected[p.file] = selected
	return selected
}

// nextConfig returns the next configuration file the search reaches
// from dir, "" when none is left. after names the configuration just
// tried in dir ("" when none was), so tsconfig.json is followed by
// jsconfig.json in the same directory, and jsconfig.json by the parent
// directory.
func (r *resolver) nextConfig(dir, after string) string {
	names := configNames
	switch after {
	case configNames[0]:
		names = configNames[1:]
	case configNames[1]:
		names = nil
	}
	for {
		for _, name := range names {
			if f := path.Join(dir, name); r.files[f] {
				return f
			}
		}
		if dir == "." || path.Base(dir) == "node_modules" {
			return ""
		}
		dir = path.Dir(dir)
		names = configNames
	}
}

// referenceOwning searches p's project references for one that
// selects file: every direct reference first, then the references of
// each in turn, skipping configurations already searched.
func (r *resolver) referenceOwning(p *project, file string, seen map[string]bool) *project {
	var children []*project
	for _, ref := range p.references {
		if seen[ref] {
			continue
		}
		seen[ref] = true
		c := r.configs.project(ref)
		if c == nil {
			continue
		}
		if r.selection(c)[file] {
			return c
		}
		children = append(children, c)
	}
	for _, c := range children {
		if o := r.referenceOwning(c, file, seen); o != nil {
			return o
		}
	}
	return nil
}

// importedBy returns the analyzed files p's program holds only because
// a file p selects imports them, directly or through other imports,
// each resolved with p's options. A file one of p's references selects
// is redirected to that reference, so it is neither held nor followed.
func (r *resolver) importedBy(p *project) map[string]bool {
	if held, ok := r.imported[p.file]; ok {
		return held
	}
	refs := r.referencedBy(p)
	redirected := func(file string) bool {
		for _, ref := range refs {
			if r.selection(ref)[file] {
				return true
			}
		}
		return false
	}
	held := map[string]bool{}
	selected := r.selection(p)
	queue := make([]string, 0, len(selected))
	for file := range selected {
		queue = append(queue, file)
	}
	for len(queue) > 0 {
		file := queue[0]
		queue = queue[1:]
		dir := path.Dir(file)
		for _, spec := range r.specs[file] {
			target := r.targetFile(p, dir, spec)
			if _, analyzed := r.specs[target]; !analyzed || held[target] || selected[target] || redirected(target) {
				continue
			}
			held[target] = true
			queue = append(queue, target)
		}
	}
	r.imported[p.file] = held
	return held
}

// referencedBy returns every configuration p references, directly or
// through other references.
func (r *resolver) referencedBy(p *project) []*project {
	if refs, ok := r.reachable[p.file]; ok {
		return refs
	}
	seen := map[string]bool{p.file: true}
	var refs []*project
	var visit func(q *project)
	visit = func(q *project) {
		for _, ref := range q.references {
			if seen[ref] {
				continue
			}
			seen[ref] = true
			if c := r.configs.project(ref); c != nil {
				refs = append(refs, c)
				visit(c)
			}
		}
	}
	visit(p)
	r.reachable[p.file] = refs
	return refs
}
