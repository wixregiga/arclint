package syntax

import (
	"compress/gzip"
	"encoding/json"
	"fmt"
	"os"
	"strings"
	"testing"
)

// The table holds real Context Mapper 6.12.0 syntax verdicts, including the first
// error line. Linking and validator errors are deliberately separate: Parse does
// not link references or validate the domain. The Java oracle is dev-time only.
func TestContextMapperDifferential(t *testing.T) {
	t.Parallel()
	file, err := os.Open("testdata/context_mapper_6_12_verdicts.json.gz")
	if err != nil {
		t.Fatal(err)
	}
	defer file.Close()
	reader, err := gzip.NewReader(file)
	if err != nil {
		t.Fatal(err)
	}
	defer reader.Close()
	var cases []struct {
		Name     string `json:"name"`
		Input    string `json:"input"`
		Accepted bool   `json:"accepted"`
		Line     int    `json:"line"`
	}
	if err = json.NewDecoder(reader).Decode(&cases); err != nil {
		t.Fatal(err)
	}
	if len(cases) != 45649 {
		t.Fatalf("oracle table drift: %d inputs", len(cases))
	}
	checked := 0
	for i, tt := range cases {
		if !sampled(i, 200) {
			continue
		}
		checked++
		name := fmt.Sprintf("%s/%d", tt.Name, i)
		_, err = ParseFile(name, []byte(tt.Input))
		if (err == nil) != tt.Accepted {
			t.Errorf("%s: accepted=%t, want %t; %v", name, err == nil, tt.Accepted, err)
			continue
		}
		if !tt.Accepted {
			diagnostic, ok := err.(*Error)
			if !ok {
				t.Errorf("%s: want positioned syntax error, got %v", name, err)
				continue
			}
			if diagnostic.Pos.Line != tt.Line {
				t.Errorf("%s: error line %d, want %d: %v", name, diagnostic.Pos.Line, tt.Line, err)
			}
		}
	}
	t.Logf("%d of %d Context Mapper syntax verdicts agree, including rejection lines", checked, len(cases))
}

func TestSelectedUnorderedMemberDocDiagnostic(t *testing.T) {
	for _, keyword := range []string{"Module", "Aggregate"} {
		source := "BoundedContext Sales {\n " + keyword + " Orders\n \"service definition\"\n Service Worker\n}"
		_, err := Parse([]byte(source))
		diagnostic, ok := err.(*Error)
		if !ok || diagnostic.Pos.Line != 4 || diagnostic.Pos.Column != 2 || !strings.Contains(diagnostic.Message, "expected "+strconvLiteral(keyword)) || !strings.Contains(diagnostic.Message, `found "Service"`) {
			t.Fatalf("%s continuation: %v", keyword, err)
		}
	}
}
