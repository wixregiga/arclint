package application

import (
	"encoding/json"
	"errors"
	"fmt"
	"math"
	"regexp"
	"strings"
)

// FeatureSketch names the Excalidraw drawing a feature carries. The drawing is a
// standard .excalidraw file stored beside the feature document; its named parts
// are the elements whose customData holds an arclintId.
type FeatureSketch struct {
	File string `json:"file"`
}

// SketchPart is one named part of a sketch. Callouts address it by arclintId,
// never by the Excalidraw element id, which an editor may regenerate.
type SketchPart struct {
	ID    string   `json:"arclintId"`
	Label string   `json:"label,omitempty"`
	Links []string `json:"links,omitempty"`
	// Region is the part's bounding box in scene coordinates relative to the
	// top-left of the whole drawing, which is where an Excalidraw PNG export starts.
	Region
}

// Region is a rectangle in image pixels; X,Y is the top-left corner.
type Region struct {
	X int `json:"x"`
	Y int `json:"y"`
	W int `json:"w"`
	H int `json:"h"`
}

// Mark is one numbered badge drawn on an image, with an optional box.
type Mark struct {
	N int
	Region
}

// ScreenshotAnnotator draws numbered marks on a PNG and returns the new PNG.
type ScreenshotAnnotator interface {
	Annotate(png []byte, marks []Mark) ([]byte, error)
}

// SketchFiles reads files named on the command line and stores a feature's sketch beside it.
type SketchFiles interface {
	Read(path string) ([]byte, error)
	PutSketch(featureID string, excalidraw []byte) (name string, err error)
	Sketch(featureID, name string) ([]byte, error)
}

// ErrSketch refuses a drawing that is not a usable .excalidraw scene.
var ErrSketch = errors.New("invalid sketch")

var partID = regexp.MustCompile(`^[a-z0-9][a-z0-9-]{0,63}$`)

type excalidrawElement struct {
	ID          string  `json:"id"`
	Type        string  `json:"type"`
	X           float64 `json:"x"`
	Y           float64 `json:"y"`
	Width       float64 `json:"width"`
	Height      float64 `json:"height"`
	IsDeleted   bool    `json:"isDeleted"`
	Text        string  `json:"text"`
	ContainerID string  `json:"containerId"`
	CustomData  struct {
		ArclintID string   `json:"arclintId"`
		Label     string   `json:"label"`
		Links     []string `json:"links"`
	} `json:"customData"`
}

// ParseSketch reads an .excalidraw scene and returns its named parts in drawing order.
func ParseSketch(data []byte) ([]SketchPart, error) {
	var scene struct {
		Type     string              `json:"type"`
		Elements []excalidrawElement `json:"elements"`
	}
	if err := json.Unmarshal(data, &scene); err != nil {
		return nil, fmt.Errorf("%w: %w", ErrSketch, err)
	}
	if scene.Type != "excalidraw" {
		return nil, fmt.Errorf("%w: not an .excalidraw file (type %q)", ErrSketch, scene.Type)
	}
	minX, minY := math.Inf(1), math.Inf(1)
	labels := map[string]string{}
	for _, e := range scene.Elements {
		if e.IsDeleted {
			continue
		}
		minX, minY = math.Min(minX, math.Min(e.X, e.X+e.Width)), math.Min(minY, math.Min(e.Y, e.Y+e.Height))
		if e.Type == "text" && e.ContainerID != "" {
			labels[e.ContainerID] = e.Text
		}
	}
	parts := []SketchPart{}
	seen := map[string]bool{}
	for _, e := range scene.Elements {
		id := e.CustomData.ArclintID
		if e.IsDeleted || id == "" {
			continue
		}
		if !partID.MatchString(id) {
			return nil, fmt.Errorf("%w: arclintId %q must be lowercase letters, digits and hyphens", ErrSketch, id)
		}
		if seen[id] {
			return nil, fmt.Errorf("%w: arclintId %q names two elements", ErrSketch, id)
		}
		seen[id] = true
		label := e.CustomData.Label
		if label == "" {
			label = labels[e.ID]
		}
		x, y := math.Min(e.X, e.X+e.Width), math.Min(e.Y, e.Y+e.Height)
		label = strings.Join(strings.Fields(label), " ")
		parts = append(parts, SketchPart{ID: id, Label: label, Links: e.CustomData.Links, Region: Region{
			X: int(math.Round(x - minX)), Y: int(math.Round(y - minY)),
			W: int(math.Round(math.Abs(e.Width))), H: int(math.Round(math.Abs(e.Height))),
		}})
	}
	return parts, nil
}

func checkSketch(s *FeatureSketch) error {
	if s != nil && s.File == "" {
		return ErrFeatureInput
	}
	return nil
}

// AttachFeatureSketch stores an .excalidraw drawing with a feature.
type AttachFeatureSketch struct {
	Store FeatureDraftStore
	Files SketchFiles
}

// Execute replaces the feature's sketch with the drawing at path and returns its named parts.
func (u AttachFeatureSketch) Execute(featureID, path string) (FeatureDraft, []SketchPart, error) {
	draft, err := u.Store.Get(featureID)
	if err != nil {
		return draft, nil, fmt.Errorf("attach sketch: %w", err)
	}
	data, err := u.Files.Read(path)
	if err != nil {
		return draft, nil, fmt.Errorf("attach sketch: %w", err)
	}
	parts, err := ParseSketch(data)
	if err != nil {
		return draft, nil, fmt.Errorf("attach sketch %s: %w", path, err)
	}
	name, err := u.Files.PutSketch(draft.ID, data)
	if err != nil {
		return draft, nil, fmt.Errorf("attach sketch: %w", err)
	}
	draft.Sketch = &FeatureSketch{File: name}
	saved, err := u.Store.Save(draft)
	if err != nil {
		return saved, nil, fmt.Errorf("attach sketch: %w", err)
	}
	return saved, parts, nil
}

// ReadSketchParts lists the named parts of a feature's sketch, or of a one-off .excalidraw file.
type ReadSketchParts struct {
	Store FeatureDraftStore
	Files SketchFiles
}

// Execute reads the sketch of feature featureID, or the file at path when featureID is empty.
func (u ReadSketchParts) Execute(featureID, path string) ([]SketchPart, error) {
	data, _, err := sketchSource(u.Store, u.Files, featureID, path)
	if err != nil {
		return nil, err
	}
	parts, err := ParseSketch(data)
	if err != nil {
		return nil, fmt.Errorf("sketch parts: %w", err)
	}
	return parts, nil
}

// sketchSource returns the drawing and its project-relative link target.
func sketchSource(store FeatureDraftStore, files SketchFiles, featureID, path string) ([]byte, string, error) {
	if featureID == "" {
		if path == "" {
			return nil, "", errors.New("name a feature or a sketch file")
		}
		data, err := files.Read(path)
		return data, path, err
	}
	draft, err := store.Get(featureID)
	if err != nil {
		return nil, "", fmt.Errorf("sketch: %w", err)
	}
	if draft.Sketch == nil {
		return nil, "", fmt.Errorf("feature %s has no sketch; attach one with `arclint feature sketch add`", draft.ID)
	}
	data, err := files.Sketch(draft.ID, draft.Sketch.File)
	return data, FeatureFileDirectory + "/" + draft.Sketch.File, err
}
