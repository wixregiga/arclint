package vocab

import (
	"strconv"
	"strings"
)

// VocabularyYAML returns the domain-librarian VOCAB.yaml content for a
// project, byte-exact to the litmus vocabulary file. The vocabulary is the
// meta-model's building blocks, one term per block in meta-model order
// with a comment naming each phase, followed by the context relation
// kinds, the distillation rules, the source tiers, and the rules the
// meta-model declines to enforce with their reasons and tiered basis. Hand-assembled so comments and flow-style lines match;
// yaml.Marshal is never used.
func VocabularyYAML() string {
	var b strings.Builder
	b.WriteString(VOCABHeaderComment)
	b.WriteString("\n")
	b.WriteString("version: ")
	b.WriteString(strconv.Itoa(UbiquitousLanguageVersion))
	b.WriteString("\n\n")

	model := DDD()
	b.WriteString("vocabulary:\n")
	phase := ""
	for _, block := range model.Blocks {
		if block.Phase != phase {
			phase = block.Phase
			if p, ok := model.Phase(phase); ok {
				b.WriteString("  # ")
				b.WriteString(p.Title)
				b.WriteString("\n")
			}
		}
		b.WriteString("  ")
		b.WriteString(block.Term)
		b.WriteString(": ")
		b.WriteString(yamlDoubleQuoted(block.Definition.Text))
		b.WriteString("\n")
	}
	b.WriteString("\n")

	b.WriteString("context_relation_kinds:\n")
	for _, d := range RelationKindDocs() {
		b.WriteString("  ")
		b.WriteString(string(d.Kind))
		b.WriteString(": ")
		b.WriteString(yamlDoubleQuoted(d.Meaning))
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

	b.WriteString("source_tiers:\n")
	for _, t := range Tiers() {
		b.WriteString("  ")
		b.WriteString(strconv.Itoa(int(t)))
		b.WriteString(": ")
		b.WriteString(yamlDoubleQuoted(t.Meaning()))
		b.WriteString("\n")
	}
	b.WriteString("\n")

	b.WriteString("declined_rules:\n")
	for _, d := range model.Declined {
		b.WriteString("  - {id: ")
		b.WriteString(d.ID)
		b.WriteString(", statement: ")
		b.WriteString(yamlDoubleQuoted(d.Statement))
		b.WriteString(", reason: ")
		b.WriteString(yamlDoubleQuoted(d.Reason))
		b.WriteString(", basis: ")
		b.WriteString(yamlDoubleQuoted(declinedBasis(model, d)))
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

// declinedBasis spells a declined rule's basis for a reader who has
// only VOCAB.yaml: every cited work in full with its page, labeled by
// its tier, or the owner decision labeled as one with its issue link.
func declinedBasis(m MetaModel, d DeclinedRule) string {
	if d.Decision != "" {
		return DecisionLabel + ": " + d.Decision
	}
	refs := m.References(d.Sources)
	spelled := make([]string, len(refs))
	for i, r := range refs {
		spelled[i] = r.Work.Tier.Label() + ": " + r.String()
	}
	return strings.Join(spelled, "; ")
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
