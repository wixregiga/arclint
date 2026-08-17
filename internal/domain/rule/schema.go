package rule

import (
	"fmt"
	"strings"
)

// FieldSchema describes one accepted Rule field: its kind, whether it
// is required, its default and enum where finite, and whether a Rule
// Override may change it.
type FieldSchema struct {
	Name         string
	Kind         string // "string", "enum", "glob", "glob_list", "module_list", "allow_list", "policy", "case"
	Required     bool
	Default      string
	Enum         []string
	Configurable bool
	Doc          string
}

// TypeSchema is the machine-readable description used to configure,
// validate, inspect, and autocomplete a complete Rule of one Rule
// Type: the common Rule fields plus the Type-specific parameters.
type TypeSchema struct {
	Type   Type
	Common []FieldSchema
	Params []FieldSchema
}

// Schema returns the complete schema contribution for this Rule Type.
func (t Type) Schema() TypeSchema {
	common := []FieldSchema{
		{
			Name: "id", Kind: "string", Required: true,
			Doc: "explicit stable Rule ID, optionally namespace-qualified",
		},
		{
			Name: "type", Kind: "enum", Required: true, Enum: typeStrings(),
			Doc: "one published ArcLint Rule Type",
		},
		{
			Name: "claim", Kind: "string",
			Doc: "architectural proposition; derived canonically when absent",
		},
		{
			Name: "severity", Kind: "enum", Default: string(DefaultSeverity),
			Enum:         []string{string(SeverityError), string(SeverityWarning), string(SeverityInfo)},
			Configurable: true,
			Doc:          "gate importance, independent from Assurance and Evidence Method",
		},
	}
	var params []FieldSchema
	switch t {
	case TypeConsumes:
		params = []FieldSchema{
			{
				Name: "internal", Kind: "allow_list",
				Doc: "declared Modules this Module may import; absent = unrestricted, empty = none",
			},
			{
				Name: "external", Kind: "policy", Default: string(ImportAllow),
				Enum: []string{string(ImportAllow), string(ImportForbid)},
				Doc:  "third-party import policy",
			},
			{
				Name: "stdlib", Kind: "policy", Default: string(ImportAllow),
				Enum: []string{string(ImportAllow), string(ImportForbid)},
				Doc:  "standard-library import policy",
			},
		}
	case TypeStructure:
		params = []FieldSchema{
			{Name: "require", Kind: "glob_list", Doc: "each glob must match at least one member file"},
			{Name: "forbid", Kind: "glob_list", Doc: "no member file may match any glob"},
		}
	case TypeNaming:
		params = []FieldSchema{
			{Name: "files", Kind: "glob", Doc: "narrows the member files judged; absent = all"},
			{
				Name: "case", Kind: "case", Required: true,
				Doc: "kebab-case | snake_case | camelCase | PascalCase | regex:<pattern>, any-of with |",
			},
		}
	case TypeLayers:
		params = []FieldSchema{
			{
				Name: "layers", Kind: "module_list", Required: true,
				Doc: "Modules ordered highest first; imports may go same or lower only",
			},
		}
	case TypeProtected:
		params = []FieldSchema{
			{Name: "module", Kind: "string", Required: true, Doc: "the protected Module"},
			{Name: "allow", Kind: "module_list", Doc: "Modules permitted to import it"},
		}
	case TypeAcyclic:
		params = []FieldSchema{
			{Name: "modules", Kind: "module_list", Doc: "cycle scope; absent = every declared Module"},
		}
	case TypeExtension:
		params = []FieldSchema{
			{
				Name: "uses", Kind: "string", Required: true,
				Doc: "the extension rule name registered under .arclint/extensions",
			},
			{
				Name: "with", Kind: "object",
				Doc: "parameters validated host-side against the extension's published schema",
			},
		}
	}
	return TypeSchema{Type: t, Common: common, Params: params}
}

// Describe explains the accepted configuration of this Rule Type.
func (s TypeSchema) Describe() string {
	var b strings.Builder
	fmt.Fprintf(&b, "rule type %s\n", s.Type)
	for _, section := range []struct {
		title  string
		fields []FieldSchema
	}{{"common", s.Common}, {"params", s.Params}} {
		for _, f := range section.fields {
			req := ""
			if f.Required {
				req = " (required)"
			}
			cfg := ""
			if f.Configurable {
				cfg = " (configurable)"
			}
			fmt.Fprintf(&b, "  %s.%s: %s%s%s — %s\n", section.title, f.Name, f.Kind, req, cfg, f.Doc)
		}
	}
	return b.String()
}

func typeStrings() []string {
	out := make([]string, 0, len(Types()))
	for _, t := range Types() {
		out = append(out, string(t))
	}
	return out
}
