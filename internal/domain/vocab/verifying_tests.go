package vocab

import (
	"fmt"
	"strings"
)

// VerifyingTest is one automated test an invariant or an assertion
// names under verified_by: the path of the test's file from the
// repository root and the test's name, spelled path:name. Recording a
// test says the test exists to verify the statement; it never says the
// test proves it.
type VerifyingTest struct {
	Path string
	Name string
}

// String spells the test as it is recorded: the path, a colon, and the
// name.
func (t VerifyingTest) String() string { return t.Path + ":" + t.Name }

// ParseVerifyingTest reads one verified_by entry. The path ends at the
// first colon, so a name may hold colons and a path never does.
func ParseVerifyingTest(entry string) (VerifyingTest, error) {
	if strings.ContainsAny(entry, "\n\r") {
		return VerifyingTest{}, fmt.Errorf("%q spans more than one line", entry)
	}
	p, name, found := strings.Cut(entry, ":")
	if !found {
		return VerifyingTest{}, fmt.Errorf("%q names no test: spell it as the test file's path from the repository root, a colon, and the test's name", entry)
	}
	if wrong := testPathSpelling(p); wrong != "" {
		return VerifyingTest{}, fmt.Errorf("%q: the path %s", entry, wrong)
	}
	switch {
	case strings.TrimSpace(name) == "":
		return VerifyingTest{}, fmt.Errorf("%q names a file but no test after the colon", entry)
	case strings.TrimSpace(name) != name:
		return VerifyingTest{}, fmt.Errorf("%q: the test's name begins or ends with a space", entry)
	}
	return VerifyingTest{Path: p, Name: name}, nil
}

// testPathSpelling says what keeps a path from naming a file inside the
// repository, or nothing.
func testPathSpelling(p string) string {
	switch {
	case p == "":
		return "is empty"
	case strings.TrimSpace(p) != p:
		return "begins or ends with a space"
	case strings.HasPrefix(p, "/"):
		return "is absolute; write it from the repository root"
	case strings.Contains(p, `\`):
		return "uses backslashes; separate its parts with /"
	}
	for _, seg := range strings.Split(p, "/") {
		switch seg {
		case "":
			return "has an empty part"
		case ".", "..":
			return "has a . or .. part; write it from the repository root"
		}
	}
	return ""
}

// requireVerifyingTests applies a block's verified-by-names-tests
// invariant, named by id, to the entries one invariant or assertion
// records under verified_by.
func requireVerifyingTests(id string, line int, what string, entries []string) error {
	seen := map[string]bool{}
	for _, entry := range entries {
		if _, err := ParseVerifyingTest(entry); err != nil {
			return fmt.Errorf("%s: %s%s: verified_by %w", id, at(line), what, err)
		}
		if seen[entry] {
			return fmt.Errorf("%s: %s%s: verified_by names %q twice", id, at(line), what, entry)
		}
		seen[entry] = true
	}
	return nil
}
