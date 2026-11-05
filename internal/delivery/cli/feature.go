package cli

import (
	"encoding/json"
	"fmt"
	"io"
	"os"
	"strings"

	"github.com/wixregiga/arclint/internal/application"
)

// FeatureUseCases are the feature commands' collaborators.
type FeatureUseCases struct {
	List     application.ListFeatureDrafts
	Get      application.GetFeatureDraft
	Save     application.SaveFeatureDraft
	Sketch   application.AttachFeatureSketch
	Parts    application.ReadSketchParts
	Evidence application.BuildFeatureEvidence
	Skill    application.PublishFeatureEvidenceSkill
}

const featureLong = `A feature is requested behavior written in the project's language. It is
stored in one place, .arclint/studio/features/<id>.json, which Studio's
Features view edits. A feature can carry a sketch: an Excalidraw drawing of the target
with named parts, so before/after screenshots can point at the part of the
sketch each change implements.`

// NewFeatureCommand is the feature command group.
func NewFeatureCommand(u FeatureUseCases) Command {
	return Command{
		Name:  "feature",
		Short: "features, their sketches, and before/after evidence for pull requests",
		Long:  featureLong,
		Subcommands: []Command{
			{
				Name: "list", Short: "list saved features with their ids",
				Run: func(ctx Context) error {
					list, err := u.List.Execute()
					if err != nil {
						return ConfigError(err)
					}
					for _, f := range list {
						if _, err := fmt.Fprintf(ctx.Stdout, "%s  %s\n", f.ID, f.Title); err != nil {
							return fmt.Errorf("write output: %w", err)
						}
					}
					return nil
				},
			},
			{
				Name: "new", Short: "create a feature with a title and an optional body", MaxArgs: 1,
				Example: `  arclint feature new "Tenant switcher" --body "A user switches tenant from the header."`,
				Flags:   []Flag{{Name: "body", Doc: "the feature's text"}},
				Run: func(ctx Context) error {
					if len(ctx.Args) != 1 || strings.TrimSpace(ctx.Args[0]) == "" {
						return ConfigError(fmt.Errorf("feature new needs a title"))
					}
					d, err := u.Save.Execute(application.FeatureDraft{Title: ctx.Args[0], Body: ctx.String("body"), References: []application.FeatureReference{}})
					if err != nil {
						return ConfigError(err)
					}
					return line(ctx.Stdout, "created feature %s\n", d.ID)
				},
			},
			{
				Name: "show", Short: "print a feature as JSON, including its sketch elements", MaxArgs: 1,
				Run: func(ctx Context) error {
					if len(ctx.Args) != 1 {
						return ConfigError(fmt.Errorf("feature show needs a feature id"))
					}
					d, err := u.Get.Execute(ctx.Args[0])
					if err != nil {
						return ConfigError(err)
					}
					enc := json.NewEncoder(ctx.Stdout)
					enc.SetIndent("", "  ")
					if err := enc.Encode(d); err != nil {
						return fmt.Errorf("write output: %w", err)
					}
					return nil
				},
			},
			{
				Name: "sketch", Short: "attach an Excalidraw drawing of the target to a feature, and list its named parts",
				Long: `A sketch is a standard .excalidraw file. A named part is any element whose
customData holds {"arclintId": "<short-id>"}, optionally with "label" and
"links" (invariants or domain terms). Callouts link to arclintIds.`,
				Subcommands: []Command{
					{
						Name: "add", Short: "store an .excalidraw drawing with the feature", MaxArgs: 2,
						Example: `  arclint feature sketch add <feature-id> tenant-switcher.excalidraw`,
						Run: func(ctx Context) error {
							if len(ctx.Args) != 2 {
								return ConfigError(fmt.Errorf("feature sketch add needs a feature id and an .excalidraw file"))
							}
							d, parts, err := u.Sketch.Execute(ctx.Args[0], ctx.Args[1])
							if err != nil {
								return ConfigError(err)
							}
							if err := line(ctx.Stdout, "sketch %s attached to feature %s\n", d.Sketch.File, d.ID); err != nil {
								return err
							}
							return printParts(ctx.Stdout, parts)
						},
					},
					{
						Name: "parts", Short: "list the named parts (arclintIds) of a feature's sketch or of an .excalidraw file", MaxArgs: 1,
						Example: `  arclint feature sketch parts <feature-id>
  arclint feature sketch parts --file one-off.excalidraw`,
						Flags: []Flag{{Name: "file", Doc: "a one-off .excalidraw file instead of a feature's sketch"}},
						Run: func(ctx Context) error {
							id := ""
							if len(ctx.Args) == 1 {
								id = ctx.Args[0]
							}
							parts, err := u.Parts.Execute(id, ctx.String("file"))
							if err != nil {
								return ConfigError(err)
							}
							return printParts(ctx.Stdout, parts)
						},
					},
				},
			},
			{
				Name: "evidence", Short: "annotate before/after screenshots with numbered callouts linked to the sketch", MaxArgs: 1,
				Long: `Reads a callouts JSON file:
  {"feature": "<id>" or "sketch": "one-off.excalidraw",
   "before": "before.png", "after": "after.png",
   "sketchImage": "optional PNG export of the drawing",
   "callouts": [{"n": 1, "part": "<arclintId>", "before": {"x","y","w","h"},
                 "after": {...}, "now": "...", "target": "..."}]}
and writes before.png and after.png with numbered callouts (and sketch.png when
sketchImage is given), plus evidence.md: a pull request section whose table
links every number to its sketch part.`,
				Example: `  arclint feature evidence callouts.json --out docs/evidence/my-branch \
    --link-base https://raw.githubusercontent.com/me/repo/my-branch/`,
				Flags: []Flag{
					{Name: "out", Default: "evidence", Doc: "directory, relative to the project root, to write the pack into"},
					{Name: "link-base", Doc: "prefix for links in evidence.md, e.g. a raw URL at the pushed branch; default makes them relative to evidence.md"},
				},
				Run: func(ctx Context) error {
					if len(ctx.Args) != 1 {
						return ConfigError(fmt.Errorf("feature evidence needs a callouts JSON file"))
					}
					req, err := readEvidenceRequest(ctx.Args[0])
					if err != nil {
						return ConfigError(err)
					}
					req.Out, req.LinkBase = ctx.String("out"), ctx.String("link-base")
					pack, err := u.Evidence.Execute(req)
					if err != nil {
						return ConfigError(err)
					}
					for _, f := range pack.Files {
						if err := line(ctx.Stdout, "wrote %s\n", f); err != nil {
							return err
						}
					}
					return nil
				},
			},
			{
				Name: "skill", Short: "install the feature-evidence agent skill for coding agents",
				Flags: []Flag{{Name: flagDir, Repeat: true, Doc: "directory to write SKILL.md into; repeatable; default " + strings.Join(application.FeatureEvidenceSkillDirectories, " and ")}},
				Run: func(ctx Context) error {
					dirs := ctx.Strings(flagDir)
					if len(dirs) == 0 {
						dirs = application.FeatureEvidenceSkillDirectories
					}
					for _, dir := range dirs {
						changed, p, err := u.Skill.Execute(dir)
						if err != nil {
							return ConfigError(err)
						}
						verb := "unchanged"
						if changed {
							verb = "wrote"
						}
						if err := line(ctx.Stdout, "%s %s\n", verb, p); err != nil {
							return err
						}
					}
					return nil
				},
			},
		},
	}
}

func line(w io.Writer, format string, args ...any) error {
	if _, err := fmt.Fprintf(w, format, args...); err != nil {
		return fmt.Errorf("write output: %w", err)
	}
	return nil
}

func printParts(w io.Writer, parts []application.SketchPart) error {
	if len(parts) == 0 {
		return line(w, "no named parts: give an element customData {\"arclintId\": \"<short-id>\"}\n")
	}
	for _, p := range parts {
		if err := line(w, "%s\t%s\t%s\n", p.ID, p.Label, strings.Join(p.Links, ", ")); err != nil {
			return err
		}
	}
	return nil
}

func readEvidenceRequest(path string) (application.EvidenceRequest, error) {
	var req application.EvidenceRequest
	b, err := os.ReadFile(path)
	if err != nil {
		return req, fmt.Errorf("read callouts: %w", err)
	}
	if err := json.Unmarshal(b, &req); err != nil {
		return req, fmt.Errorf("callouts %s: %w", path, err)
	}
	return req, nil
}
