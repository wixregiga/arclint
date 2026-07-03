// arclint init — create the .arclint/ configuration layout in the current
// directory (docs/design/cli.md, init section). init never prompts and is
// the one command that works without an existing .arclint/.
package cli

import (
	"fmt"
	"io"
	"io/fs"
	"os"
	"path/filepath"
	"sort"
	"strings"

	"github.com/spf13/cobra"

	"github.com/jofyi/arclint/internal/assets"
)

func init() { Register(newInitCmd()) }

func newInitCmd() *cobra.Command {
	var force, bare bool
	cmd := &cobra.Command{
		Use:   "init",
		Short: "Create the .arclint/ configuration layout in the current directory",
		Long: "Create .arclint/ with a heavily commented default rules.yaml, an empty\n" +
			"answers/ directory, and the builtin example templates (repo, service,\n" +
			"component). Refuses to touch an existing .arclint/ unless --force is given.",
		Args: cobra.NoArgs,
		RunE: func(cmd *cobra.Command, _ []string) error {
			root := Globals().Config
			if root == "" {
				wd, err := os.Getwd()
				if err != nil {
					return UsageErrorf("cannot determine the working directory — %v", err)
				}
				root = wd
			}
			opts := initOptions{Force: force, Bare: bare, Quiet: Globals().Quiet}
			return runInit(root, opts, cmd.OutOrStdout())
		},
	}
	cmd.Flags().BoolVar(&force, "force", false, "overwrite an existing .arclint/ directory")
	cmd.Flags().BoolVar(&bare, "bare", false, "create rules.yaml and empty templates/ only; skip example templates")
	return cmd
}

// initOptions carries the init flags, decoupled from cobra so tests can
// drive runInit directly.
type initOptions struct {
	Force bool
	Bare  bool
	Quiet bool
}

// runInit creates root/.arclint per the cli.md init contract. All failure
// paths are usage/config errors (exit 2).
//
// The full tree is staged into a sibling temp directory (created via
// os.MkdirTemp(root, ...), so it lives on the same filesystem as arcDir and
// the eventual swap is a cheap rename, not a copy) before anything touches
// the existing .arclint/. Only once staging succeeds completely do we swap
// the staged tree into place — see swapInPlace. This means a --force run
// that fails partway through (disk full, permission error, etc.) leaves the
// original .arclint/ exactly as it was, instead of deleting it up front and
// then failing mid-write with nothing to show for it.
func runInit(root string, opts initOptions, out io.Writer) error {
	arcDir := filepath.Join(root, ".arclint")

	exists := false
	if _, err := os.Stat(arcDir); err == nil {
		exists = true
		if !opts.Force {
			return UsageErrorf(".arclint/ already exists — run with --force to overwrite, or edit .arclint/rules.yaml directly")
		}
	} else if !os.IsNotExist(err) {
		return UsageErrorf("cannot inspect %s — %v", arcDir, err)
	}

	stageDir, err := os.MkdirTemp(root, ".arclint.staging-*")
	if err != nil {
		return UsageErrorf("cannot create a staging directory next to %s — %v", arcDir, err)
	}
	// If we return before the swap below completes the rename of stageDir
	// into arcDir, this cleans up the leftover staging directory. Once the
	// swap succeeds, stageDir no longer exists under this name, so this is
	// a harmless no-op.
	defer os.RemoveAll(stageDir)

	// answers/ ships empty; .gitkeep makes git track the directory.
	if err := os.MkdirAll(filepath.Join(stageDir, "answers"), 0o755); err != nil {
		return UsageErrorf("cannot create %s — path not writable (%v)", arcDir, err)
	}
	if err := os.WriteFile(filepath.Join(stageDir, "answers", ".gitkeep"), nil, 0o644); err != nil {
		return UsageErrorf("cannot write %s — %v", filepath.Join(arcDir, "answers", ".gitkeep"), err)
	}
	if err := os.WriteFile(filepath.Join(stageDir, "rules.yaml"), assets.DefaultRules(), 0o644); err != nil {
		return UsageErrorf("cannot write %s — %v", filepath.Join(arcDir, "rules.yaml"), err)
	}
	if err := os.MkdirAll(filepath.Join(stageDir, "templates"), 0o755); err != nil {
		return UsageErrorf("cannot create %s — %v", filepath.Join(arcDir, "templates"), err)
	}

	var templates []string
	if !opts.Bare {
		var err error
		templates, err = writeTemplates(filepath.Join(stageDir, "templates"))
		if err != nil {
			return UsageErrorf("cannot write builtin templates — %v", err)
		}
	}

	if err := swapInPlace(arcDir, stageDir, exists); err != nil {
		return UsageErrorf("cannot install %s — %v", arcDir, err)
	}

	if opts.Quiet {
		return nil
	}
	fmt.Fprintln(out, "created .arclint/")
	fmt.Fprintln(out, "  rules.yaml")
	fmt.Fprintln(out, "  answers/")
	if len(templates) == 0 {
		fmt.Fprintln(out, "  templates/")
	}
	for _, t := range templates {
		fmt.Fprintf(out, "  templates/%s/\n", t)
	}
	fmt.Fprintln(out)
	fmt.Fprintln(out, "try: arclint new service my-service")
	fmt.Fprintln(out, "try: arclint check")
	return nil
}

// swapInPlace moves the fully-staged stageDir into arcDir's place.
//
// If arcDir does not yet exist, this is a plain rename — nothing to
// preserve. If arcDir exists (the --force path), the old directory is
// first renamed to a ".old" sibling, then stageDir is renamed to arcDir,
// then the ".old" backup is removed. Both renames are same-filesystem
// (stageDir and the ".old" path are both created under filepath.Dir(arcDir)
// via os.MkdirTemp/os.Rename), so they are effectively atomic on POSIX and
// on NTFS. If the rename of stageDir into arcDir fails after the old
// directory has already been moved aside, we attempt to rename the backup
// back into place so the user is never left without a .arclint/ at all.
func swapInPlace(arcDir, stageDir string, exists bool) error {
	if !exists {
		return os.Rename(stageDir, arcDir)
	}

	oldDir := arcDir + ".old"
	_ = os.RemoveAll(oldDir) // clear any stale leftover from a prior failed swap
	if err := os.Rename(arcDir, oldDir); err != nil {
		return fmt.Errorf("cannot move existing .arclint/ aside for the swap — %w", err)
	}
	if err := os.Rename(stageDir, arcDir); err != nil {
		// Best-effort restore: put the original back so the failure is
		// non-destructive even though it did not complete.
		if restoreErr := os.Rename(oldDir, arcDir); restoreErr != nil {
			return fmt.Errorf("swap failed (%v) and restore also failed (%v) — original config is at %s", err, restoreErr, oldDir)
		}
		return fmt.Errorf("cannot move staged .arclint/ into place (original restored) — %w", err)
	}
	return os.RemoveAll(oldDir)
}

// writeTemplates copies the embedded builtin templates into dst, decoding
// each embed-safe path (assets.DecodePath) to its on-disk name. It returns
// the sorted template names it wrote.
func writeTemplates(dst string) ([]string, error) {
	tfs := assets.Templates()
	names := map[string]bool{}

	err := fs.WalkDir(tfs, ".", func(p string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if p == "." {
			return nil
		}
		if top, _, _ := strings.Cut(p, "/"); top != "" {
			names[top] = true
		}
		target := filepath.Join(dst, filepath.FromSlash(assets.DecodePath(p)))
		if d.IsDir() {
			return os.MkdirAll(target, 0o755)
		}
		data, err := fs.ReadFile(tfs, p)
		if err != nil {
			return err
		}
		if err := os.MkdirAll(filepath.Dir(target), 0o755); err != nil {
			return err
		}
		return os.WriteFile(target, data, 0o644)
	})
	if err != nil {
		return nil, err
	}

	sorted := make([]string, 0, len(names))
	for n := range names {
		sorted = append(sorted, n)
	}
	sort.Strings(sorted)
	return sorted, nil
}
