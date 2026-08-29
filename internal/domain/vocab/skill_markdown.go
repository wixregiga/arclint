package vocab

import (
	"strconv"
	"strings"
)

// SkillMarkdown returns the domain-librarian SKILL.md content
// byte-exact to the litmus skill protocol file.
func SkillMarkdown() string {
	var b strings.Builder
	b.WriteString("---\n")
	b.WriteString("name: ")
	b.WriteString(SkillName)
	b.WriteString("\n")
	b.WriteString("description: ")
	b.WriteString(SkillDescription)
	b.WriteString("\n")
	b.WriteString("---\n\n")
	b.WriteString("# ")
	b.WriteString(SkillTitle)
	b.WriteString("\n\n")
	b.WriteString(SkillIntro)
	b.WriteString("\n\n")
	b.WriteString("## Reference\n\n")
	b.WriteString(SkillReference)
	b.WriteString("\n\n")
	b.WriteString("## Protocol\n\n")
	for i, rule := range SkillProtocolRules() {
		b.WriteString(strconv.Itoa(i + 1))
		b.WriteString(". ")
		b.WriteString(rule)
		b.WriteString("\n")
	}
	b.WriteString("\n")
	b.WriteString("## Economy\n\n")
	b.WriteString(SkillEconomy)
	b.WriteString("\n")
	return b.String()
}
