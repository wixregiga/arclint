package application

import (
	"errors"
	"fmt"
	"path"
	"path/filepath"
	"strings"
)

// FeatureFileDirectory is where feature documents and their sketches live, relative to the project.
const FeatureFileDirectory = ".arclint/studio/features"

// EvidenceCallout is one numbered change: where it sits on the before and after
// screenshots, and which sketch element it implements.
type EvidenceCallout struct {
	N      int     `json:"n"`
	Part   string  `json:"part"`
	Before *Region `json:"before,omitempty"`
	After  *Region `json:"after,omitempty"`
	Now    string  `json:"now,omitempty"`
	Target string  `json:"target,omitempty"`
}

// EvidenceRequest names the sketch (a feature's, or a one-off .excalidraw file),
// the two screenshots, the callouts, and where the pack goes.
type EvidenceRequest struct {
	Feature string `json:"feature,omitempty"`
	Sketch  string `json:"sketch,omitempty"`
	// SketchImage is an optional PNG export of the whole drawing; when given,
	// the pack also numbers the parts on it. SketchPadding is the export's
	// padding in pixels (Excalidraw's default is 10).
	SketchImage   string            `json:"sketchImage,omitempty"`
	SketchPadding *int              `json:"sketchPadding,omitempty"`
	Before        string            `json:"before"`
	After         string            `json:"after"`
	Callouts      []EvidenceCallout `json:"callouts"`
	// Out is the project-relative directory the pack is written into.
	Out string `json:"-"`
	// LinkBase prefixes every project-relative link in the markdown, e.g. a
	// raw.githubusercontent.com URL at the pushed branch; empty makes links relative to evidence.md.
	LinkBase string `json:"-"`
}

// EvidencePack lists the files one run wrote.
type EvidencePack struct {
	Files    []string
	Markdown string
}

// Evidence pack file names.
const (
	EvidenceBefore   = "before.png"
	EvidenceAfter    = "after.png"
	EvidenceSketch   = "sketch.png"
	EvidenceMarkdown = "evidence.md"
)

// BuildFeatureEvidence draws numbered callouts on before, after and the feature's
// sketch, and writes markdown linking each number to its sketch element.
type BuildFeatureEvidence struct {
	Store     FeatureDraftStore
	Files     SketchFiles
	Annotator ScreenshotAnnotator
	Writer    ArtifactWriter
}

// Execute validates the callouts against the sketch, then writes the pack.
func (u BuildFeatureEvidence) Execute(req EvidenceRequest) (EvidencePack, error) {
	var pack EvidencePack
	fail := func(err error) (EvidencePack, error) { return pack, fmt.Errorf("feature evidence: %w", err) }
	data, sketchLink, err := sketchSource(u.Store, u.Files, req.Feature, req.Sketch)
	if err != nil {
		return fail(err)
	}
	parts, err := ParseSketch(data)
	if err != nil {
		return fail(err)
	}
	title := req.Sketch
	if req.Feature != "" {
		draft, err := u.Store.Get(req.Feature)
		if err != nil {
			return fail(err)
		}
		title = draft.Title
	}
	byID := map[string]SketchPart{}
	for _, p := range parts {
		byID[p.ID] = p
	}
	if len(req.Callouts) == 0 {
		return fail(errors.New("give at least one callout"))
	}
	padding := 10
	if req.SketchPadding != nil {
		padding = *req.SketchPadding
	}
	var before, after, sketch []Mark
	for i, c := range req.Callouts {
		if c.N != i+1 {
			return fail(fmt.Errorf("callouts must be numbered 1, 2, 3... in order; callout %d has number %d", i+1, c.N))
		}
		p, ok := byID[c.Part]
		if !ok {
			return fail(fmt.Errorf("callout %d links to %q, which is no arclintId in the sketch", c.N, c.Part))
		}
		if c.Before == nil && c.After == nil {
			return fail(fmt.Errorf("callout %d marks neither the before nor the after screenshot", c.N))
		}
		if c.Before != nil {
			before = append(before, Mark{N: c.N, Region: *c.Before})
		}
		if c.After != nil {
			after = append(after, Mark{N: c.N, Region: *c.After})
		}
		r := p.Region
		r.X, r.Y = r.X+padding, r.Y+padding
		sketch = append(sketch, Mark{N: c.N, Region: r})
	}
	inputs := []struct {
		name  string
		png   func() ([]byte, error)
		marks []Mark
	}{
		{EvidenceBefore, func() ([]byte, error) { return u.Files.Read(req.Before) }, before},
		{EvidenceAfter, func() ([]byte, error) { return u.Files.Read(req.After) }, after},
		{EvidenceSketch, func() ([]byte, error) { return u.Files.Read(req.SketchImage) }, sketch},
	}
	for _, in := range inputs {
		if in.name == EvidenceSketch && req.SketchImage == "" {
			continue
		}
		src, err := in.png()
		if err != nil {
			return fail(err)
		}
		out, err := u.Annotator.Annotate(src, in.marks)
		if err != nil {
			return fail(fmt.Errorf("%s: %w", in.name, err))
		}
		_, p, err := u.Writer.Write(req.Out, in.name, out)
		if err != nil {
			return fail(err)
		}
		pack.Files = append(pack.Files, p)
	}
	pack.Markdown = evidenceMarkdown(title, sketchLink, req, byID)
	_, p, err := u.Writer.Write(req.Out, EvidenceMarkdown, []byte(pack.Markdown))
	if err != nil {
		return fail(err)
	}
	pack.Files = append(pack.Files, p)
	return pack, nil
}

func evidenceMarkdown(title, sketchLink string, req EvidenceRequest, parts map[string]SketchPart) string {
	// With a link base, links are project paths under it. Without one, they are
	// relative to evidence.md, which lives in req.Out, so the committed file renders.
	link := func(p string) string {
		if req.LinkBase != "" {
			return req.LinkBase + path.Clean(p)
		}
		rel, err := filepath.Rel(filepath.FromSlash(req.Out), filepath.FromSlash(p))
		if err != nil {
			return path.Clean(p)
		}
		return filepath.ToSlash(rel)
	}
	out := func(name string) string { return link(path.Join(req.Out, name)) }
	var b strings.Builder
	fmt.Fprintf(&b, "## Before and after: %s\n\n", cell(title))
	fmt.Fprintf(&b, "Sketch: [`%s`](%s). Each number marks the same change on both screenshots and links to the named sketch part it implements.\n\n", sketchLink, link(sketchLink))
	b.WriteString("| Before (current) | After (target) |\n| --- | --- |\n")
	fmt.Fprintf(&b, "| ![before](%s) | ![after](%s) |\n\n", out(EvidenceBefore), out(EvidenceAfter))
	b.WriteString("| # | Sketch part | Before: now | After: target | Links |\n| --- | --- | --- | --- | --- |\n")
	for _, c := range req.Callouts {
		p := parts[c.Part]
		now, target := c.Now, c.Target
		if c.Before == nil && now == "" {
			now = "not present"
		}
		if c.After == nil && target == "" {
			target = "removed"
		}
		fmt.Fprintf(&b, "| %d | [`%s`](%s#%s) %s | %s | %s | %s |\n", c.N, c.Part, link(sketchLink), c.Part,
			cell(p.Label), cell(now), cell(target), cell(strings.Join(p.Links, ", ")))
	}
	if req.SketchImage != "" {
		fmt.Fprintf(&b, "\n<details><summary>Sketch with the same numbers</summary>\n\n![sketch](%s)\n\n</details>\n", out(EvidenceSketch))
	}
	return b.String()
}

func cell(s string) string {
	return strings.NewReplacer("|", `\|`, "\n", " ").Replace(s)
}
