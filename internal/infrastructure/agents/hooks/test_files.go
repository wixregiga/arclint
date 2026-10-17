package hooks

import (
	"path"
	"strings"
)

// testDirectories are directory names whose files are tests or the data
// tests read, in the languages ArcLint observes and their usual tooling.
var testDirectories = map[string]bool{"test": true, "tests": true, "__tests__": true, "testdata": true, "__mocks__": true}

// scriptExtensions are the JavaScript and TypeScript file extensions.
var scriptExtensions = map[string]bool{".ts": true, ".tsx": true, ".mts": true, ".cts": true, ".js": true, ".jsx": true, ".mjs": true, ".cjs": true}

// TestConventions tells test files apart by the naming conventions of Go,
// TypeScript, JavaScript and Python and their test runners.
type TestConventions struct{}

// Test reports whether the slash-separated, project-relative path is a
// test or lies in a directory of tests or test data.
func (TestConventions) Test(name string) bool {
	directory, base := path.Split(name)
	for _, segment := range strings.Split(strings.Trim(directory, "/"), "/") {
		if testDirectories[segment] {
			return true
		}
	}
	extension := path.Ext(base)
	stem := strings.TrimSuffix(base, extension)
	switch {
	case scriptExtensions[extension]:
		// guide.test.ts, guide.spec.tsx
		return strings.HasSuffix(stem, ".test") || strings.HasSuffix(stem, ".spec")
	case strings.HasSuffix(base, "_test.go"):
		return true
	case extension == ".py":
		return strings.HasPrefix(stem, "test_") || strings.HasSuffix(stem, "_test") || stem == "conftest"
	}
	return false
}
