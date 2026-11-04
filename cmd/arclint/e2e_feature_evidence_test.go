package main

import (
	"bytes"
	"encoding/json"
	"image"
	"image/color"
	"image/png"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

const evidenceSketch = `{"type":"excalidraw","version":2,"source":"test","elements":[
 {"id":"frame","type":"rectangle","x":100,"y":50,"width":300,"height":200},
 {"id":"r8x","type":"rectangle","x":120,"y":70,"width":80,"height":30,"customData":{"arclintId":"save-button","links":["Draft"]}},
 {"id":"t1","type":"text","x":125,"y":75,"width":40,"height":20,"text":"Save","containerId":"r8x"},
 {"id":"gone","type":"rectangle","x":0,"y":0,"width":10,"height":10,"isDeleted":true,"customData":{"arclintId":"old"}}
],"appState":{},"files":{}}`

func writePNG(t *testing.T, path string, w, h int) {
	t.Helper()
	img := image.NewRGBA(image.Rect(0, 0, w, h))
	for i := range img.Pix {
		img.Pix[i] = 255
	}
	var b bytes.Buffer
	if err := png.Encode(&b, img); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, b.Bytes(), 0o600); err != nil {
		t.Fatal(err)
	}
}

func isRed(c color.Color) bool {
	r, g, b, _ := c.RGBA()
	return r>>8 > 180 && g>>8 < 80 && b>>8 < 80
}

func evidenceProject(t *testing.T) (string, string) {
	t.Helper()
	root := t.TempDir()
	rules, err := os.ReadFile(filepath.Join(repoCopy, "docs/examples/go", "rules.arclint.yaml"))
	if err != nil {
		t.Fatal(err)
	}
	write(t, root, "rules.arclint.yaml", string(rules))
	write(t, root, "save.excalidraw", evidenceSketch)
	writePNG(t, filepath.Join(root, "before-raw.png"), 400, 300)
	writePNG(t, filepath.Join(root, "after-raw.png"), 400, 300)
	out, stderr, code := runBin(t, root, os.Environ(), "feature", "new", "Save a draft")
	if code != 0 {
		t.Fatalf("feature new: %d %s", code, stderr)
	}
	return root, strings.TrimSpace(strings.TrimPrefix(out, "created feature "))
}

// A user attaches an Excalidraw sketch to a feature, then builds a pack whose
// numbers appear on both screenshots and link to the sketch's named part.
func TestFeatureEvidencePack(t *testing.T) {
	root, id := evidenceProject(t)
	out, stderr, code := runBin(t, root, os.Environ(), "feature", "sketch", "add", id, "save.excalidraw")
	if code != 0 || !strings.Contains(out, "save-button\tSave\tDraft") || strings.Contains(out, "old") {
		t.Fatalf("sketch add: %d %q %s", code, out, stderr)
	}
	stored := filepath.Join(root, ".arclint/studio/features", id+".sketch.excalidraw")
	if _, err := os.Stat(stored); err != nil {
		t.Fatalf("sketch not stored beside the feature: %v", err)
	}
	callouts, _ := json.Marshal(map[string]any{
		"feature": id, "before": "before-raw.png", "after": "after-raw.png",
		"callouts": []map[string]any{{"n": 1, "part": "save-button",
			"before": map[string]int{"x": 50, "y": 60, "w": 100, "h": 40},
			"after":  map[string]int{"x": 200, "y": 160, "w": 100, "h": 40}, "now": "grey link", "target": "primary button"}},
	})
	write(t, root, "callouts.json", string(callouts))
	_, stderr, code = runBin(t, root, os.Environ(), "feature", "evidence", "callouts.json", "--out", "pack", "--link-base", "https://raw.example/b/")
	if code != 0 {
		t.Fatalf("evidence: %d %s", code, stderr)
	}
	md, err := os.ReadFile(filepath.Join(root, "pack/evidence.md"))
	if err != nil {
		t.Fatal(err)
	}
	for _, want := range []string{
		"![before](https://raw.example/b/pack/before.png)",
		"| 1 | [`save-button`](https://raw.example/b/.arclint/studio/features/" + id + ".sketch.excalidraw#save-button) Save | grey link | primary button | Draft |",
	} {
		if !strings.Contains(string(md), want) {
			t.Fatalf("evidence.md lacks %q:\n%s", want, md)
		}
	}
	for name, at := range map[string]image.Point{"before.png": {50, 80}, "after.png": {200, 180}} {
		f, err := os.Open(filepath.Join(root, "pack", name))
		if err != nil {
			t.Fatal(err)
		}
		img, err := png.Decode(f)
		_ = f.Close()
		if err != nil {
			t.Fatal(err)
		}
		if !isRed(img.At(at.X, at.Y)) {
			t.Fatalf("%s: no callout box at %v", name, at)
		}
	}
}

// The pack refuses callouts a reviewer could not follow.
func TestFeatureEvidenceRefusesUnlinkedCallouts(t *testing.T) {
	root, _ := evidenceProject(t)
	box := map[string]int{"x": 1, "y": 1, "w": 5, "h": 5}
	for want, callout := range map[string]map[string]any{
		"no arclintId in the sketch": {"n": 1, "part": "missing", "after": box},
		"numbered 1, 2, 3":           {"n": 2, "part": "save-button", "after": box},
	} {
		b, _ := json.Marshal(map[string]any{"sketch": "save.excalidraw", "before": "before-raw.png", "after": "after-raw.png", "callouts": []any{callout}})
		write(t, root, "callouts.json", string(b))
		_, stderr, code := runBin(t, root, os.Environ(), "feature", "evidence", "callouts.json")
		if code != 2 || !strings.Contains(stderr, want) {
			t.Fatalf("want exit 2 with %q, got %d %s", want, code, stderr)
		}
	}
}

// A coding agent's harness finds the skill after one command.
func TestFeatureEvidenceSkillInstall(t *testing.T) {
	root, _ := evidenceProject(t)
	if _, stderr, code := runBin(t, root, os.Environ(), "feature", "skill"); code != 0 {
		t.Fatalf("feature skill: %d %s", code, stderr)
	}
	for _, dir := range []string{".agents", ".claude"} {
		b, err := os.ReadFile(filepath.Join(root, dir, "skills/feature-evidence/SKILL.md"))
		if err != nil || !strings.HasPrefix(string(b), "---\nname: feature-evidence\n") {
			t.Fatalf("%s skill: %v", dir, err)
		}
	}
}
