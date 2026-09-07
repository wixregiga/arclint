package yamlvocab

import (
	"strings"
	"testing"

	"gopkg.in/yaml.v3"
)

func TestReflowFoldedFillsBlocksToTheLineWidth(t *testing.T) {
	long := strings.Repeat("word ", 30) + "end"
	cases := []struct {
		name string
		in   string
		want string
	}{
		{
			"plain lines are untouched",
			"key: " + long + "\nother: x\n",
			"key: " + long + "\nother: x\n",
		},
		{
			"a folded block is filled",
			"    definition: >-\n      " + long + "\n    identity: OrderID\n",
			"    definition: >-\n" +
				"      word word word word word word word word word word word word word word word\n" +
				"      word word word word word word word word word word word word word word word\n" +
				"      end\n" +
				"    identity: OrderID\n",
		},
		{
			"a header with a comment is a header",
			"definition: >- # why\n  " + long + "\n",
			"definition: >- # why\n" +
				"  word word word word word word word word word word word word word word word\n" +
				"  word word word word word word word word word word word word word word word end\n",
		},
		{
			"more-indented lines are literal and untouched",
			"definition: >-\n  " + long + "\n    " + long + "\n  tail\n",
			"definition: >-\n" +
				"  word word word word word word word word word word word word word word word\n" +
				"  word word word word word word word word word word word word word word word end\n" +
				"    " + long + "\n" +
				"  tail\n",
		},
		{
			"blank lines inside the block are kept",
			"definition: >-\n  short\n\n  " + long + "\nnext: 1\n",
			"definition: >-\n  short\n\n" +
				"  word word word word word word word word word word word word word word word\n" +
				"  word word word word word word word word word word word word word word word end\n" +
				"next: 1\n",
		},
		{
			"a comment line is not a header",
			"# definition: >-\n  " + long + "\n",
			"# definition: >-\n  " + long + "\n",
		},
		{
			"a word longer than the width stands alone",
			"definition: >-\n  a " + strings.Repeat("x", 90) + " b\n",
			"definition: >-\n  a\n  " + strings.Repeat("x", 90) + "\n  b\n",
		},
		{
			"double spaces are never broken",
			"definition: >-\n  " + strings.Repeat("ab  ", 25) + "cd\n",
			"definition: >-\n  " + strings.Repeat("ab  ", 25) + "cd\n",
		},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			got := string(reflowFolded([]byte(c.in)))
			if got != c.want {
				t.Fatalf("reflowFolded:\n%s\nwant:\n%s", got, c.want)
			}
			// Filling never changes what YAML reads.
			var before, after any
			if err := yaml.Unmarshal([]byte(c.in), &before); err != nil {
				t.Fatalf("input does not parse: %v", err)
			}
			if err := yaml.Unmarshal([]byte(got), &after); err != nil {
				t.Fatalf("output does not parse: %v", err)
			}
			b, _ := yaml.Marshal(before)
			a, _ := yaml.Marshal(after)
			if string(a) != string(b) {
				t.Fatalf("filling changed the document:\n%s\nwas:\n%s", a, b)
			}
		})
	}
}
