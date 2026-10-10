// Package hooks connects Agent Host hooks to workflow Guidance: it reads
// the hosts' native events, keeps each session's Progress in the project's
// ArcLint cache together with the activities ArcLint's own commands record,
// and writes the hook configuration Codex and Claude Code read.
package hooks

import (
	"fmt"
	"path/filepath"
	"regexp"
	"strings"
)

// windowsDrive matches a Windows drive path such as C:\Users or c:/tmp.
var windowsDrive = regexp.MustCompile(`^([A-Za-z]):[\\/]`)

// project maps the paths a host reports to clean, slash-separated paths
// relative to the project root.
type project struct {
	root   string
	distro string
}

func newProject(root, distro string) (project, error) {
	absolute, err := filepath.Abs(root)
	if err != nil {
		return project{}, fmt.Errorf("workflow project root: %w", err)
	}
	if resolved, err := filepath.EvalSymlinks(absolute); err == nil {
		absolute = resolved
	}
	return project{root: absolute, distro: distro}, nil
}

// local maps a Windows host's path to this WSL distribution's view: the
// share \\wsl.localhost\<distro>\... or \\wsl$\<distro>\... to the path
// inside the distribution, and a drive path C:\... to /mnt/c/.... Windows
// compares distribution names without case. Any other Windows path, such
// as another distribution's share or a network share, stays as it is and
// lies outside the project.
func (p project) local(name string) string {
	if p.distro == "" {
		return name
	}
	slashed := strings.ReplaceAll(name, `\`, "/")
	if drive := windowsDrive.FindStringSubmatch(name); drive != nil {
		return "/mnt/" + strings.ToLower(drive[1]) + "/" + slashed[len(drive[0]):]
	}
	for _, share := range []string{"//wsl.localhost/", "//wsl$/"} {
		if len(slashed) <= len(share) || !strings.EqualFold(slashed[:len(share)], share) {
			continue
		}
		distro, rest, _ := strings.Cut(slashed[len(share):], "/")
		if strings.EqualFold(distro, p.distro) {
			return "/" + rest
		}
	}
	return name
}

// relative returns name relative to the project root, resolving a relative
// name against dir. It reports false for a name outside the project.
func (p project) relative(dir, name string) (string, bool) {
	name = p.local(name)
	if p.distro != "" && (strings.HasPrefix(name, `\\`) || strings.HasPrefix(name, "//")) {
		return "", false
	}
	if !filepath.IsAbs(name) {
		name = filepath.Join(p.local(dir), name)
	}
	name = filepath.Clean(name)
	if inside, ok := within(p.root, name); ok {
		return inside, true
	}
	return within(p.root, resolveExisting(name))
}

func within(root, name string) (string, bool) {
	relative, err := filepath.Rel(root, name)
	if err != nil || (relative != "." && !filepath.IsLocal(relative)) {
		return "", false
	}
	return filepath.ToSlash(relative), true
}

// resolveExisting resolves symbolic links in the longest existing prefix of
// name, so a path reached through a linked directory compares with the root.
func resolveExisting(name string) string {
	missing := ""
	for current := name; ; current = filepath.Dir(current) {
		if resolved, err := filepath.EvalSymlinks(current); err == nil {
			return filepath.Join(resolved, missing)
		}
		parent := filepath.Dir(current)
		if parent == current {
			return name
		}
		missing = filepath.Join(filepath.Base(current), missing)
	}
}
