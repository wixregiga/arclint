package main

import (
	"bufio"
	"fmt"
	"io"
	"path/filepath"
	"slices"
	"sort"
	"strconv"
	"strings"
	"text/tabwriter"

	"github.com/spf13/cobra"

	"github.com/wixregiga/arclint/internal/config"
	"github.com/wixregiga/arclint/internal/ext"
	"github.com/wixregiga/arclint/internal/lang"
	"github.com/wixregiga/arclint/internal/patterns"
	"github.com/wixregiga/arclint/internal/tree"
)

// newPatternsCmd lists available architectural patterns.
func newPatternsCmd() *cobra.Command {
	var showExtensions bool
	cmd := &cobra.Command{
		Use:   "patterns",
		Short: "list architectural patterns (builtin and .arclint/patterns)",
		Args:  cobra.NoArgs,
		RunE: func(cmd *cobra.Command, args []string) error {
			all, err := patterns.All(".")
			if err != nil {
				return &exitError{2, err.Error()}
			}
			w := tabwriter.NewWriter(cmd.OutOrStdout(), 2, 4, 2, ' ', 0)
			fmt.Fprintln(w, "NAME\tSOURCE\tRUNTIMES\tEXTENSIONS\tDESCRIPTION")
			for _, p := range all {
				fmt.Fprintf(w, "%s\t%s\t%s\t%d\t%s\n",
					p.Name, p.Source, strings.Join(p.Runtimes, ","), len(p.Extensions), p.Description)
			}
			if err := w.Flush(); err != nil {
				return err
			}
			if !showExtensions {
				return nil
			}
			out := cmd.OutOrStdout()
			fmt.Fprintln(out, "\nextension files (installed into .arclint/extensions/ by `arclint init`):")
			for _, p := range all {
				names := make([]string, 0, len(p.Extensions))
				for name := range p.Extensions {
					names = append(names, name)
				}
				sort.Strings(names)
				for _, name := range names {
					fmt.Fprintf(out, "  %s\t%s\n", p.Name, filepath.ToSlash(filepath.Join(".arclint", "extensions", name)))
				}
			}
			return nil
		},
	}
	cmd.Flags().BoolVar(&showExtensions, "extensions", false, "also list each pattern's extension files")
	return cmd
}

// newInitCmd sets a repository up: pick runtimes and a pattern, write
// rules.yaml plus the pattern's extensions, generate editor typings, and
// validate the result.
func newInitCmd() *cobra.Command {
	var runtimesFlag, patternFlag string
	var force bool
	cmd := &cobra.Command{
		Use:   "init",
		Short: "set this repository up: pick runtimes and a pattern, write rules.yaml",
		Long: "Interactive by default: detects the languages present, asks which to\n" +
			"analyze and which architectural pattern to start from, then writes\n" +
			"rules.yaml (and the pattern's extensions) and validates the result.\n" +
			"Pass --runtimes and --pattern to skip every prompt.",
		Args: cobra.NoArgs,
		RunE: func(cmd *cobra.Command, args []string) error {
			out := cmd.OutOrStdout()
			in := bufio.NewReader(cmd.InOrStdin())

			runtimes, err := resolveRuntimes(out, in, runtimesFlag)
			if err != nil {
				return err
			}
			pattern, err := resolvePattern(out, in, patternFlag, runtimes)
			if err != nil {
				return err
			}

			written, err := pattern.Materialize(".", runtimes, force)
			if err != nil {
				return &exitError{2, err.Error()}
			}
			if len(pattern.Extensions) > 0 {
				sdkFiles, err := ext.SDKInit(".")
				if err != nil {
					return &exitError{2, err.Error()}
				}
				for _, f := range sdkFiles {
					rel, err := filepath.Rel(".", f)
					if err != nil {
						rel = f
					}
					written = append(written, filepath.ToSlash(rel))
				}
			}
			for _, f := range written {
				fmt.Fprintf(out, "wrote %s\n", f)
			}

			// Validate what was written exactly as `arclint load` would.
			rs, err := config.Load("rules.yaml")
			if err != nil {
				return &exitError{2, err.Error()}
			}
			if err := config.WriteCache(rs, version); err != nil {
				return &exitError{2, fmt.Sprintf("cannot write cache: %v", err)}
			}
			if _, err := loadExtensionRegistry(rs); err != nil {
				return err
			}
			fmt.Fprintf(out, "\npattern %q ready for %s.\nnext:\n", pattern.Name, strings.Join(runtimes, ", "))
			fmt.Fprintln(out, "  arclint check .      run the contracts")
			fmt.Fprintln(out, "  arclint module ls    see what each module matched (adjust globs in rules.yaml)")
			fmt.Fprintln(out, "  arclint explain      learn every rule kind")
			return nil
		},
	}
	cmd.Flags().StringVar(&runtimesFlag, "runtimes", "", "comma-separated targets: go,ts,py (skips the prompt)")
	cmd.Flags().StringVar(&patternFlag, "pattern", "", "pattern name from `arclint patterns` (skips the prompt)")
	cmd.Flags().BoolVar(&force, "force", false, "overwrite existing rules.yaml / extension files")
	return cmd
}

// detectRuntimes walks the current tree and counts files per language
// target.
func detectRuntimes() map[string]int {
	counts := map[string]int{}
	t, err := tree.Walk(".", tree.Options{})
	if err != nil {
		return counts
	}
	for _, f := range t.Files {
		if target := lang.TargetOf(f.Path); target != "" {
			counts[target]++
		}
	}
	return counts
}

func parseRuntimes(s string) ([]string, error) {
	var out []string
	for _, part := range strings.Split(s, ",") {
		r := strings.TrimSpace(part)
		if r == "" {
			continue
		}
		if r != "go" && r != "ts" && r != "py" {
			return nil, fmt.Errorf("unknown runtime %q (go, ts, py)", r)
		}
		if !slices.Contains(out, r) {
			out = append(out, r)
		}
	}
	if len(out) == 0 {
		return nil, fmt.Errorf("at least one runtime is required (go, ts, py)")
	}
	return out, nil
}

func resolveRuntimes(out io.Writer, in *bufio.Reader, flag string) ([]string, error) {
	if flag != "" {
		rts, err := parseRuntimes(flag)
		if err != nil {
			return nil, &exitError{2, err.Error()}
		}
		return rts, nil
	}
	counts := detectRuntimes()
	var detected []string
	for _, r := range []string{"go", "ts", "py"} {
		if counts[r] > 0 {
			detected = append(detected, r)
		}
	}
	def := detected
	if len(def) == 0 {
		def = []string{"go"}
	}
	if len(detected) > 0 {
		var parts []string
		for _, r := range detected {
			parts = append(parts, fmt.Sprintf("%s (%d files)", r, counts[r]))
		}
		fmt.Fprintf(out, "detected: %s\n", strings.Join(parts, ", "))
	}
	fmt.Fprintf(out, "runtimes to analyze [%s]: ", strings.Join(def, ","))
	line, err := in.ReadString('\n')
	if err != nil && line == "" {
		return nil, &exitError{2, "init needs answers on stdin, or pass --runtimes and --pattern"}
	}
	line = strings.TrimSpace(line)
	if line == "" {
		return def, nil
	}
	rts, err := parseRuntimes(line)
	if err != nil {
		return nil, &exitError{2, err.Error()}
	}
	return rts, nil
}

func resolvePattern(out io.Writer, in *bufio.Reader, flag string, runtimes []string) (*patterns.Pattern, error) {
	if flag != "" {
		p, err := patterns.Find(".", flag)
		if err != nil {
			return nil, &exitError{2, err.Error()}
		}
		if !p.Supports(runtimes) {
			return nil, &exitError{2, fmt.Sprintf("pattern %q supports runtimes %v, not %v", p.Name, p.Runtimes, runtimes)}
		}
		return p, nil
	}
	all, err := patterns.All(".")
	if err != nil {
		return nil, &exitError{2, err.Error()}
	}
	var compatible []*patterns.Pattern
	for _, p := range all {
		if p.Supports(runtimes) {
			compatible = append(compatible, p)
		}
	}
	if len(compatible) == 0 {
		return nil, &exitError{2, fmt.Sprintf("no pattern supports runtimes %v; run `arclint patterns`", runtimes)}
	}
	fmt.Fprintf(out, "patterns for %s:\n", strings.Join(runtimes, ","))
	for i, p := range compatible {
		fmt.Fprintf(out, "  %d. %-16s %s\n", i+1, p.Name, p.Description)
	}
	fmt.Fprintf(out, "pattern [1]: ")
	line, err := in.ReadString('\n')
	if err != nil && line == "" {
		return nil, &exitError{2, "init needs answers on stdin, or pass --runtimes and --pattern"}
	}
	line = strings.TrimSpace(line)
	if line == "" {
		return compatible[0], nil
	}
	if n, err := strconv.Atoi(line); err == nil {
		if n < 1 || n > len(compatible) {
			return nil, &exitError{2, fmt.Sprintf("pattern %d is not on the list", n)}
		}
		return compatible[n-1], nil
	}
	for _, p := range compatible {
		if p.Name == line {
			return p, nil
		}
	}
	return nil, &exitError{2, fmt.Sprintf("unknown pattern %q; run `arclint patterns`", line)}
}
