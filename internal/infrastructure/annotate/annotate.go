// Package annotate draws numbered callout badges on PNG screenshots with the
// standard library alone: a red box around each region and a numbered badge
// on its top-left corner.
package annotate

import (
	"bytes"
	"fmt"
	"image"
	"image/color"
	"image/draw"
	"image/png"
	"strconv"

	"github.com/wixregiga/arclint/internal/application"
)

var (
	red   = color.RGBA{220, 38, 38, 255}
	white = color.RGBA{255, 255, 255, 255}
)

// digits is a 3x5 bitmap font for 0-9, one row per string.
var digits = [10][5]string{
	{"111", "101", "101", "101", "111"},
	{"010", "110", "010", "010", "111"},
	{"111", "001", "111", "100", "111"},
	{"111", "001", "111", "001", "111"},
	{"101", "101", "111", "001", "001"},
	{"111", "100", "111", "001", "111"},
	{"111", "100", "111", "101", "111"},
	{"111", "001", "010", "010", "010"},
	{"111", "101", "111", "101", "111"},
	{"111", "101", "111", "001", "111"},
}

// PNG implements application.ScreenshotAnnotator.
type PNG struct{}

// Annotate decodes src, draws every mark, and encodes the result as PNG.
func (PNG) Annotate(src []byte, marks []application.Mark) ([]byte, error) {
	img, err := png.Decode(bytes.NewReader(src))
	if err != nil {
		return nil, fmt.Errorf("decode png: %w", err)
	}
	b := img.Bounds()
	canvas := image.NewRGBA(b)
	draw.Draw(canvas, b, img, b.Min, draw.Src)
	short := min(b.Dx(), b.Dy())
	radius := max(12, short/40)
	stroke := max(2, radius/5)
	for _, m := range marks {
		x, y := b.Min.X+m.X, b.Min.Y+m.Y
		if m.W > 0 && m.H > 0 {
			box(canvas, image.Rect(x, y, x+m.W, y+m.H), stroke)
		}
		badge(canvas, x, y, radius, m.N)
	}
	var out bytes.Buffer
	if err := png.Encode(&out, canvas); err != nil {
		return nil, fmt.Errorf("encode png: %w", err)
	}
	return out.Bytes(), nil
}

func box(c *image.RGBA, r image.Rectangle, stroke int) {
	for _, edge := range []image.Rectangle{
		image.Rect(r.Min.X, r.Min.Y, r.Max.X, r.Min.Y+stroke),
		image.Rect(r.Min.X, r.Max.Y-stroke, r.Max.X, r.Max.Y),
		image.Rect(r.Min.X, r.Min.Y, r.Min.X+stroke, r.Max.Y),
		image.Rect(r.Max.X-stroke, r.Min.Y, r.Max.X, r.Max.Y),
	} {
		draw.Draw(c, edge.Intersect(c.Bounds()), image.NewUniform(red), image.Point{}, draw.Src)
	}
}

func badge(c *image.RGBA, cx, cy, r, n int) {
	b := c.Bounds()
	cx = min(max(cx, b.Min.X+r), b.Max.X-r-1)
	cy = min(max(cy, b.Min.Y+r), b.Max.Y-r-1)
	ring := max(2, r/6)
	for dy := -r; dy <= r; dy++ {
		for dx := -r; dx <= r; dx++ {
			d := dx*dx + dy*dy
			switch {
			case d <= (r-ring)*(r-ring):
				c.Set(cx+dx, cy+dy, red)
			case d <= r*r:
				c.Set(cx+dx, cy+dy, white)
			}
		}
	}
	text := strconv.Itoa(n)
	scale := max(1, (r*11/10)/5)
	if len(text) > 1 {
		scale = max(1, (r*14/10)/(4*len(text)))
	}
	width := len(text)*4*scale - scale
	x0, y0 := cx-width/2, cy-5*scale/2
	for i, ch := range text {
		glyph := digits[ch-'0']
		for row, bits := range glyph {
			for col, bit := range bits {
				if bit == '1' {
					px := x0 + (i*4+col)*scale
					py := y0 + row*scale
					draw.Draw(c, image.Rect(px, py, px+scale, py+scale), image.NewUniform(white), image.Point{}, draw.Src)
				}
			}
		}
	}
}
