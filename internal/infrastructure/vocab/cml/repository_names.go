package cmlvocab

import (
	"reflect"
	"strings"

	"github.com/wixregiga/arclint/internal/domain/vocab"
	"github.com/wixregiga/arclint/internal/infrastructure/vocab/cml/syntax"
)

// Repository names are model-wide identities in Context Mapper. Count the
// requested modeled names together with authored declarations that will survive
// the edit, rather than counting the old names of repositories being replaced.
func repositoryNameCounts(lang vocab.UbiquitousLanguage, existing *projection) map[string]int {
	counts := map[string]int{}
	for _, c := range lang.Contexts {
		for _, a := range c.Aggregates {
			counts[a.Repository]++
		}
	}
	if existing == nil {
		return counts
	}
	skip := map[any]bool{}
	for _, root := range existing.roots {
		if root.Repository != nil {
			skip[root.Repository] = true
		}
	}
	for name, bc := range existing.contexts {
		c, retained := lang.Context(name)
		if !retained {
			skip[bc] = true
			continue
		}
		for id, container := range existing.containers {
			context, name, _ := strings.Cut(id, "\x00")
			if context == c.Name {
				if _, retained := c.Aggregate(name); !retained {
					skip[container] = true
				}
			}
		}
	}
	visitRepositories(existing.file, skip, func(repo *syntax.Repository) { counts[repo.Name]++ })
	return counts
}

// Traverse only the public AST, including repositories on objects outside the
// adapter's projection and repositories nested in services or subdomains.
func visitRepositories(owner any, skip map[any]bool, visit func(*syntax.Repository)) {
	var walk func(reflect.Value)
	walk = func(v reflect.Value) {
		if !v.IsValid() {
			return
		}
		if v.Kind() == reflect.Interface {
			if !v.IsNil() {
				walk(v.Elem())
			}
			return
		}
		if v.Kind() == reflect.Pointer {
			if v.IsNil() || skip[v.Interface()] {
				return
			}
			if repo, ok := v.Interface().(*syntax.Repository); ok {
				visit(repo)
			}
			walk(v.Elem())
			return
		}
		switch v.Kind() {
		default:
			return
		case reflect.Struct:
			for i := 0; i < v.NumField(); i++ {
				f := v.Type().Field(i)
				if f.IsExported() && f.Name != nodeField {
					walk(v.Field(i))
				}
			}
		case reflect.Slice:
			for i := 0; i < v.Len(); i++ {
				walk(v.Index(i))
			}
		}
	}
	walk(reflect.ValueOf(owner))
}
