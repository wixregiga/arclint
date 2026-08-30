package lipgloss

import (
	"github.com/charmbracelet/lipgloss"
	"github.com/wixregiga/arclint/internal/domain/rule"
)

// Theme holds semantic styles bound to one writer via lipgloss.Renderer.
// Roles: Error, Warning, Info, OK, Fail, Muted, Path.
type Theme struct {
	Error   lipgloss.Style
	Warning lipgloss.Style
	Info    lipgloss.Style
	OK      lipgloss.Style
	Fail    lipgloss.Style
	Muted   lipgloss.Style
	Path    lipgloss.Style
	// Bold styles summary counts and section headers.
	Bold lipgloss.Style
}

// NewTheme builds semantic styles on rdr. Callers that need deterministic
// ANSI (tests) must SetColorProfile on rdr before NewTheme.
func NewTheme(rdr *lipgloss.Renderer) Theme {
	return Theme{
		Error:   rdr.NewStyle().Foreground(lipgloss.Color("1")),
		Warning: rdr.NewStyle().Foreground(lipgloss.Color("3")),
		Info:    rdr.NewStyle().Foreground(lipgloss.Color("4")),
		OK:      rdr.NewStyle().Foreground(lipgloss.Color("2")),
		Fail:    rdr.NewStyle().Foreground(lipgloss.Color("1")).Bold(true),
		Muted:   rdr.NewStyle().Faint(true),
		Path:    rdr.NewStyle().Foreground(lipgloss.Color("6")).Faint(true),
		Bold:    rdr.NewStyle().Bold(true),
	}
}

func (th Theme) severity(s string) lipgloss.Style {
	switch rule.Severity(s) {
	case rule.SeverityError:
		return th.Error
	case rule.SeverityWarning:
		return th.Warning
	case rule.SeverityInfo:
		return th.Info
	default:
		return th.Muted
	}
}

// severityBracket colors "[sev]" without changing surrounding tokens.
func (th Theme) severityBracket(sev string) string {
	return th.severity(sev).Render("[" + sev + "]")
}

// pathAnchor colors path or path:line without inserting spaces.
func (th Theme) pathAnchor(path string, line int) string {
	if path == "" {
		return ""
	}
	if line > 0 {
		return th.Path.Render(path) + ":" + th.Path.Render(itoa(line))
	}
	return th.Path.Render(path)
}

// atPath mirrors plain atPath with path tokens colored.
func (th Theme) atPath(path string, line int, message string) string {
	if path == "" {
		return message
	}
	if line > 0 {
		return th.Path.Render(path) + ":" + th.Path.Render(itoa(line)) + ": " + message
	}
	return th.Path.Render(path) + ": " + message
}

func itoa(n int) string {
	if n == 0 {
		return "0"
	}
	var b [20]byte
	i := len(b)
	for n > 0 {
		i--
		b[i] = byte('0' + n%10)
		n /= 10
	}
	return string(b[i:])
}
