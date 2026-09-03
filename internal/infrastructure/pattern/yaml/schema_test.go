package yaml_test

import (
	"bytes"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"

	sj "github.com/santhosh-tekuri/jsonschema/v6"
	yamlv3 "gopkg.in/yaml.v3"

	patternyaml "github.com/wixregiga/arclint/internal/infrastructure/pattern/yaml"
)

func repositoryRoot(t *testing.T) string {
	t.Helper()
	_, file, _, ok := runtime.Caller(0)
	if !ok {
		t.Fatal("runtime.Caller: no source location")
	}
	return filepath.Join(filepath.Dir(file), "..", "..", "..", "..")
}

func TestPublishedPatternSchemaMatchesLoader(t *testing.T) {
	want, err := patternyaml.Schema()
	if err != nil {
		t.Fatalf("Schema: %v", err)
	}
	published := filepath.Join(repositoryRoot(t), "docs", "pattern.schema.json")
	if os.Getenv("UPDATE_PATTERN_SCHEMA") != "" {
		if err := os.WriteFile(published, want, 0o644); err != nil {
			t.Fatalf("write published schema: %v", err)
		}
	}
	got, err := os.ReadFile(published)
	if err != nil {
		t.Fatalf("read published schema: %v", err)
	}
	if !bytes.Equal(want, got) {
		t.Fatalf("docs/pattern.schema.json drifted from patternyaml.Schema(); regenerate with UPDATE_PATTERN_SCHEMA=1")
	}
}

func compilePatternSchema(t *testing.T) *sj.Schema {
	t.Helper()
	data, err := patternyaml.Schema()
	if err != nil {
		t.Fatalf("Schema: %v", err)
	}
	doc, err := sj.UnmarshalJSON(bytes.NewReader(data))
	if err != nil {
		t.Fatalf("unmarshal schema: %v", err)
	}
	const url = "https://raw.githubusercontent.com/wixregiga/arclint/main/docs/pattern.schema.json"
	compiler := sj.NewCompiler()
	if err := compiler.AddResource(url, doc); err != nil {
		t.Fatalf("add schema resource: %v", err)
	}
	schema, err := compiler.Compile(url)
	if err != nil {
		t.Fatalf("compile schema: %v", err)
	}
	return schema
}

func validatePatternSchema(t *testing.T, schema *sj.Schema, source string) error {
	t.Helper()
	var value any
	if err := yamlv3.Unmarshal([]byte(source), &value); err != nil {
		return err
	}
	data, err := json.Marshal(jsonifyPatternYAML(value))
	if err != nil {
		t.Fatalf("marshal generic document: %v", err)
	}
	instance, err := sj.UnmarshalJSON(bytes.NewReader(data))
	if err != nil {
		t.Fatalf("unmarshal generic document: %v", err)
	}
	return schema.Validate(instance)
}

func jsonifyPatternYAML(value any) any {
	switch typed := value.(type) {
	case map[string]any:
		out := make(map[string]any, len(typed))
		for key, entry := range typed {
			out[key] = jsonifyPatternYAML(entry)
		}
		return out
	case map[any]any:
		out := make(map[string]any, len(typed))
		for key, entry := range typed {
			out[fmt.Sprintf("%v", key)] = jsonifyPatternYAML(entry)
		}
		return out
	case []any:
		out := make([]any, len(typed))
		for i, entry := range typed {
			out[i] = jsonifyPatternYAML(entry)
		}
		return out
	default:
		return value
	}
}

func TestPatternSchemaAgreesWithLoaderOnRepresentationShape(t *testing.T) {
	schema := compilePatternSchema(t)
	cases := []struct {
		name     string
		manifest string
		accepted bool
	}{
		{"complete Pattern", completeManifest, true},
		{
			"coverage and tests omitted",
			strings.Replace(strings.Replace(completeManifest,
				"coverage: [go, typescript, python]\n", "", 1), "tests:\n  root: tests\n", "", 1),
			true,
		},
		{"unknown top-level key", strings.Replace(completeManifest, "coverage:", "unknown: true\ncoverage:", 1), false},
		{"unknown Rule key", strings.Replace(completeManifest, "    allow: []\n", "    mystery: true\n    allow: []\n", 1), false},
		{"field from another kind", strings.Replace(completeManifest, "    case: snake_case\n", "    case: snake_case\n    allow: []\n", 1), false},
		{"unsupported coverage", strings.Replace(completeManifest, "coverage: [go, typescript, python]", "coverage: [rust]", 1), false},
		{"inexact version", strings.Replace(completeManifest, "version: 1.2.3", "version: latest", 1), false},
		{"identity whitespace", strings.Replace(completeManifest, "name: complete", "name: complete pattern", 1), false},
		{"unsafe extension entry", strings.Replace(completeManifest, "extensions/forbid_imports.ts", "../forbid_imports.ts", 1), false},
		{
			"consumes without restriction",
			strings.Replace(strings.Replace(completeManifest, "    allow: []\n", "", 1), "    forbid: [external]\n", "", 1),
			false,
		},
		{"missing modules", strings.Replace(completeManifest, "modules:\n", "missing_modules:\n", 1), false},
	}

	for _, tt := range cases {
		t.Run(tt.name, func(t *testing.T) {
			schemaErr := validatePatternSchema(t, schema, tt.manifest)
			_, loaderErr := patternyaml.Load(completeFS(tt.manifest), ".")
			if got := schemaErr == nil; got != tt.accepted {
				t.Errorf("schema accepted=%v, want %v: %v", got, tt.accepted, schemaErr)
			}
			if got := loaderErr == nil; got != tt.accepted {
				t.Errorf("loader accepted=%v, want %v: %v", got, tt.accepted, loaderErr)
			}
			if (schemaErr == nil) != (loaderErr == nil) {
				t.Errorf("schema/loader diverged: schema=%v loader=%v", schemaErr, loaderErr)
			}
		})
	}
}
