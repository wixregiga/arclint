package syntax

import (
	"bytes"
	"reflect"
	"testing"
)

func FuzzParseFormat(f *testing.F) {
	for _, source := range []string{
		"", "// comment", `import "x.cml"`, `BoundedContext Alpha {Aggregate Group {Entity Obj {String field}}}`, `ContextMap {Alpha [U,PL] -> [D,ACL] Beta}`,
		`UserStory Story {As a "actor" I want to read "item" so that "benefit"}`,
	} {
		f.Add([]byte(source))
	}
	f.Fuzz(func(t *testing.T, source []byte) {
		file, err := Parse(source)
		if err != nil {
			return
		}
		output, err := FormatChecked(file)
		if err != nil {
			t.Fatalf("accepted input cannot be formatted: %s\n%q", err, source)
		}
		assertAuthoredTokens(t, source, output)
		again, err := Parse(output)
		if err != nil {
			t.Fatal(err)
		}
		if !reflect.DeepEqual(semantic(reflect.ValueOf(file)), semantic(reflect.ValueOf(again))) {
			t.Fatalf("semantic round trip changed: %q -> %q", source, output)
		}
		if !bytes.Equal(output, formatted(t, again)) {
			t.Fatalf("not a fixed point: %q", output)
		}
	})
}
