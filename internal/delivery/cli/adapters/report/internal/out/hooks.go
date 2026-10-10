package out

import (
	"time"

	"github.com/wixregiga/arclint/internal/application"
)

// LastHookEvent states when a hook event last reached the project.
func LastHookEvent(at time.Time) string {
	if at.IsZero() {
		return "none has reached this project"
	}
	return at.Format("2006-01-02 15:04:05 -0700")
}

// HookScope names whose configuration a hook file is: the user's, for
// every project, on the distribution's side or the Windows side, or this
// project's.
func HookScope(host application.HookFileStatus) string {
	switch {
	case host.Scope != "user":
		return "project"
	case host.Windows:
		return "user (Windows)"
	}
	return "user"
}
