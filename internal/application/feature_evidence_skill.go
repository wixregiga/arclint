package application

import "fmt"

// FeatureEvidenceSkillDirectories are the harness skill directories the
// feature-evidence skill is installed into by default.
var FeatureEvidenceSkillDirectories = []string{".agents/skills/feature-evidence", ".claude/skills/feature-evidence"}

// PublishFeatureEvidenceSkill writes the SKILL.md that teaches a coding agent
// to attach a before/after evidence pack to its request and its pull request.
type PublishFeatureEvidenceSkill struct{ Writer ArtifactWriter }

// Execute writes SKILL.md into dir.
func (u PublishFeatureEvidenceSkill) Execute(dir string) (bool, string, error) {
	changed, p, err := u.Writer.Write(dir, "SKILL.md", []byte(FeatureEvidenceSkill))
	if err != nil {
		return false, p, fmt.Errorf("publish feature-evidence skill: %w", err)
	}
	return changed, p, nil
}

// FeatureEvidenceSkill is the installed skill text.
const FeatureEvidenceSkill = `---
name: feature-evidence
description: Use when a change alters what a user sees (UI, a page, rendered output). Builds a before/after evidence pack whose numbered callouts link to named parts of the feature's Excalidraw sketch in ArcLint, then attaches it to the request and pastes it into the pull request.
---

# Feature evidence: before/after with numbered callouts

The sketch is the target. It is a standard .excalidraw file stored with the
feature's text: ` + "`.arclint/studio/features/<feature-id>.json`" + ` names it and
` + "`<feature-id>.sketch.excalidraw`" + ` sits beside it. A named part is any element
whose customData holds ` + "`{\"arclintId\": \"<short-id>\"}`" + ` (optionally ` + "`label`" + `
and ` + "`links`" + ` to invariants or domain terms). Callouts use the arclintId, never
the Excalidraw element id. A one-off sketch can stay outside any feature.

## When

Any pull request that changes something a user sees. For changes with no
visible effect, skip it and say so in the pull request.

## Steps

1. Find the feature: ` + "`arclint feature list`" + `. If the request came with a
   drawing and the feature has none: ` + "`arclint feature sketch add <feature-id> drawing.excalidraw`" + `.
2. Read the part ids: ` + "`arclint feature sketch parts <feature-id>`" + ` (or
   ` + "`--file drawing.excalidraw`" + ` for a one-off sketch).
3. Before you edit, capture the current state on the base branch. After you
   edit, capture the same screen at the same viewport and zoom.
4. Write callouts.json. Number the changes 1, 2, 3... in order, mark each
   one on both screenshots where it exists, and leave unchanged areas unmarked.

` + "```json" + `
{"feature": "<feature-id>", "before": "before-raw.png", "after": "after-raw.png",
 "sketchImage": "drawing.png",
 "callouts": [
  {"n": 1, "part": "save-button", "before": {"x": 10, "y": 20, "w": 90, "h": 30},
   "after": {"x": 10, "y": 20, "w": 120, "h": 30}, "now": "grey link", "target": "primary button"}
 ]}
` + "```" + `

   ` + "`sketchImage`" + ` is optional: a PNG export of the whole drawing (padding 10, scale 1).
5. Build the pack:
   ` + "`arclint feature evidence callouts.json --out docs/evidence/<branch> --link-base https://raw.githubusercontent.com/<owner>/<repo>/<branch>/`" + `
   It refuses numbers that are out of order and part ids the sketch lacks.
6. Attach before.png, after.png and sketch.png to your request or report.
   Commit the pack with the change, and put evidence.md in the pull request body
   (` + "`gh pr create --body-file docs/evidence/<branch>/evidence.md`" + `, or append it).
`
