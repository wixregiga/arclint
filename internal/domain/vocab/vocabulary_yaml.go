package vocab

import (
	"strconv"
	"strings"
)

// VocabularyYAML returns the domain-librarian VOCAB.yaml content
// byte-exact to the litmus vocabulary file. Hand-assembled so comments
// and flow-style lines match; yaml.Marshal is never used.
func VocabularyYAML() string {
	var b strings.Builder
	b.WriteString(VOCABHeaderComment)
	b.WriteString("\n")
	b.WriteString("version: ")
	b.WriteString(strconv.Itoa(UbiquitousLanguageVersion))
	b.WriteString("\n\n")

	b.WriteString("vocabulary:\n")
	for _, term := range VocabularyTerms() {
		b.WriteString("  ")
		b.WriteString(string(term.Term))
		b.WriteString(": ")
		if term.Term == TermContextRelation {
			b.WriteString(ContextRelationFlowYAML())
		} else {
			b.WriteString(term.Definition)
		}
		b.WriteString("\n")
	}
	b.WriteString("\n")

	b.WriteString("distillation_rules:\n")
	for _, r := range DistillationRules() {
		b.WriteString("  - {id: ")
		b.WriteString(r.ID)
		b.WriteString(", rule: ")
		b.WriteString(yamlDoubleQuoted(r.Rule))
		b.WriteString(", example: ")
		b.WriteString(yamlDoubleQuoted(r.Example))
		b.WriteString("}\n")
	}
	b.WriteString("\n")

	b.WriteString("clarification_questions:\n")
	b.WriteString("  insufficient_info:\n")
	for _, q := range InsufficientInfoQuestions() {
		b.WriteString("    - {q: ")
		b.WriteString(yamlDoubleQuoted(q.Question))
		b.WriteString(", decides: ")
		b.WriteString(q.Decides)
		b.WriteString("}\n")
	}
	b.WriteString("  conflict:\n")
	for _, q := range ConflictQuestions() {
		b.WriteString("    - {q: ")
		b.WriteString(yamlDoubleQuoted(q.Question))
		b.WriteString(", decides: ")
		b.WriteString(q.Decides)
		b.WriteString("}\n")
	}
	b.WriteString("  policy: ")
	b.WriteString(ClarificationPolicy)
	b.WriteString("\n\n")

	lib := LibraryFileSpec()
	b.WriteString("library_file:\n")
	b.WriteString("  purpose: ")
	b.WriteString(lib.Purpose)
	b.WriteString("\n")
	b.WriteString("  json_schema: ")
	b.WriteString(lib.JSONSchema)
	b.WriteString("  # ")
	b.WriteString(lib.JSONSchemaComment)
	b.WriteString("\n")
	b.WriteString("  header: ")
	b.WriteString(yamlDoubleQuoted(lib.Header))
	b.WriteString("  # ")
	b.WriteString(lib.HeaderComment)
	b.WriteString("\n")
	b.WriteString("  shape: |  # ")
	b.WriteString(lib.ShapeComment)
	b.WriteString("\n")
	shape := strings.TrimSuffix(lib.Shape, "\n")
	b.WriteString(shape)
	b.WriteString("\n")
	b.WriteString("  rules:\n")
	for _, rule := range lib.Rules {
		b.WriteString("    - ")
		b.WriteString(rule)
		b.WriteString("\n")
	}
	return b.String()
}

// yamlDoubleQuoted emits a double-quoted YAML scalar, escaping \ and ".
func yamlDoubleQuoted(s string) string {
	var b strings.Builder
	b.WriteByte('"')
	for _, r := range s {
		switch r {
		case '\\':
			b.WriteString(`\\`)
		case '"':
			b.WriteString(`\"`)
		default:
			b.WriteRune(r)
		}
	}
	b.WriteByte('"')
	return b.String()
}
