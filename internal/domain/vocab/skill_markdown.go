package vocab

import "strings"

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
	b.WriteString(SkillProtocol1)
	b.WriteString("\n")
	b.WriteString(SkillProtocol2)
	b.WriteString("\n")
	b.WriteString(SkillProtocol3)
	b.WriteString("\n")
	b.WriteString(SkillProtocol4)
	b.WriteString("\n")
	b.WriteString(SkillProtocol5)
	b.WriteString("\n")
	b.WriteString(SkillProtocol6)
	b.WriteString("\n")
	b.WriteString(SkillProtocol7)
	b.WriteString("\n")
	b.WriteString(SkillProtocol8)
	b.WriteString("\n\n")
	b.WriteString("## Economy\n\n")
	b.WriteString(SkillEconomy)
	b.WriteString("\n")
	return b.String()
}
