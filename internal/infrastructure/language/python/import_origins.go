package python

import (
	"fmt"
	"os"
	"path"
	"path/filepath"
	"strings"
	"sync"

	"github.com/wixregiga/arclint/internal/domain/conformance"
)

// origins follows imports into the repository's own modules, reading
// each module once for every worker: the parsed scopes are kept in
// modules by repository-relative file.
type origins struct {
	res     *resolver
	root    string
	ps      *parsers
	modules *sync.Map
}

func (o origins) repository(dir, module string) bool {
	if class, _, _ := o.res.classify(dir, module); class == conformance.ImportInternal {
		return true
	}
	// A module of the repository that shares a standard library name may
	// be the one an import reaches, depending on how the program runs; it
	// is never taken for the library.
	for _, root := range o.res.sourceRoots {
		if _, _, ok := o.res.resolveUnder(root, module); ok {
			return true
		}
	}
	return false
}

func (o origins) follow(dir, module, dotted string) (string, string) {
	file := o.moduleFile(dir, module)
	if file == "" {
		return "", ""
	}
	m := o.module(file)
	if m == nil {
		return "", ""
	}
	name, attributes, _ := strings.Cut(dotted, ".")
	q := qualifiedTarget(m.scope, name, m.facts.walker, 0)
	if q == "" {
		return "", ""
	}
	if attributes != "" {
		q += "." + attributes
	}
	if mutates(m, q) {
		// A module that replaces what it re-exports, or an attribute of
		// it, exports the replacement.
		return "", ""
	}
	return q, path.Dir(file)
}

// moduleFile is the repository file a module imported from the directory
// is: module.py, or a package's __init__.py.
func (o origins) moduleFile(dir, module string) string {
	_, targetDir, targetFile := o.res.classify(dir, module)
	if targetFile == "" && targetDir == "" {
		for _, root := range o.res.sourceRoots {
			if tf, td, ok := o.res.resolveUnder(root, module); ok {
				targetFile, targetDir = tf, td
				break
			}
		}
	}
	if targetFile != "" {
		return targetFile
	}
	if init := path.Join(targetDir, "__init__.py"); targetDir != "" && o.res.files[init] {
		return init
	}
	return ""
}

// module reads one observed repository file into its lexical scopes,
// once, never outside the repository; nil when it is not observed,
// cannot be read, or does not parse.
func (o origins) module(file string) *pyModule {
	if m, ok := o.modules.Load(file); ok {
		return m.(*pyModule)
	}
	var m *pyModule
	if src, err := o.read(file); err == nil {
		if df := extractDeclarations(o.ps, src); df.ParseError == "" {
			m = df.module
		}
	}
	actual, _ := o.modules.LoadOrStore(file, m)
	return actual.(*pyModule)
}

// read reads an observed file of the repository through a root that
// refuses any path leaving it.
func (o origins) read(file string) ([]byte, error) {
	if !o.res.files[file] {
		return nil, fmt.Errorf("%s is not an observed file", file)
	}
	root, err := os.OpenRoot(o.root)
	if err != nil {
		return nil, fmt.Errorf("open repository root: %w", err)
	}
	src, err := root.ReadFile(filepath.FromSlash(file))
	if closeErr := root.Close(); err == nil && closeErr != nil {
		err = closeErr
	}
	if err != nil {
		return nil, fmt.Errorf("read %s: %w", file, err)
	}
	return src, nil
}
