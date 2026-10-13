package syntax

import (
	"strings"
	"testing"
)

// The braces of an optional body belong to what the author wrote in them. A
// body that held assigned fields only prints while one of them has a value, so
// an edit that clears the last one leaves no empty braces behind. Braces the
// author wrote around nothing, or around optional syntax that no field
// represents, such as a keyword kept at its default, stay.
func TestBracesGoWithTheLastAssignedField(t *testing.T) {
	cases := []struct {
		name, source string
		clear        bool
		want         string
	}{
		{"hint only", `ValueObject V { hint = "x" }`, true, "ValueObject V\n"},
		{"hint and a default keyword", `ValueObject V { immutable hint = "x" }`, true, "ValueObject V {\n      immutable\n    }\n"},
		{"hint and a negation", `ValueObject V { !cache hint = "x" }`, true, "ValueObject V {\n      ! cache\n    }\n"},
		{"hint kept", `ValueObject V { hint = "x" }`, false, "ValueObject V {\n      hint = \"x\"\n    }\n"},
		{"empty braces the author wrote", `ValueObject V {}`, true, "ValueObject V {}\n"},
		{"empty braces and a hint added then cleared", `ValueObject V {}`, false, "ValueObject V {}\n"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			file, err := Parse([]byte("BoundedContext A {\n  Module M {\n    " + tc.source + "\n  }\n}\n"))
			if err != nil {
				t.Fatal(err)
			}
			value := file.BoundedContexts[0].Modules[0].DomainObjects[0].(*ValueObject)
			if tc.clear {
				value.Hint = ""
				delete(value.Present, "hint")
			}
			data, err := FormatChecked(file)
			if err != nil {
				t.Fatal(err)
			}
			text := string(data)
			start := strings.Index(text, "    ValueObject V")
			if start < 0 {
				t.Fatalf("no value object in:\n%s", text)
			}
			got := strings.TrimPrefix(text[start:], "    ")
			if !strings.HasPrefix(got, tc.want) {
				t.Fatalf("printed\n%s\nwant it to start with\n%s", got, tc.want)
			}
		})
	}
}
