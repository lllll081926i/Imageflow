package engine

import (
	"fmt"
	"image"
	"image/color"
	"image/draw"
	"strings"

	"golang.org/x/image/font"
	"golang.org/x/image/math/fixed"
)

// TextOptions carries the styling shared by the watermark and rich-text
// engines.
type TextOptions struct {
	Family    string
	Size      int
	Color     color.NRGBA
	Bold      bool
	Italic    bool
	Stroke    int
	StrokeCol color.NRGBA
	Shadow    bool
	ShadowCol color.NRGBA
	LineGap   float32
}

// DrawText draws one line at (x, baselineY) onto dst.
//
// golang.org/x/image's sfnt.Font caches glyph data without a lock, so every
// operation on a face is serialised through fontMu; the batch runs engines on
// several goroutines at once.
func DrawText(dst draw.Image, text string, x, baselineY float32, o TextOptions) (width float32, err error) {
	fontMu.Lock()
	defer fontMu.Unlock()
	face, err := FontFace(o.Family, o.Size, 72, o.Bold)
	if err != nil {
		return 0, err
	}
	if o.Shadow && o.ShadowCol.A > 0 {
		offset := float32(o.Size) / 24
		if offset < 1 {
			offset = 1
		}
		drawFace(dst, text, x+offset, baselineY+offset, face, o.ShadowCol, 0, color.NRGBA{})
	}
	if o.Stroke > 0 && o.StrokeCol.A > 0 {
		drawFace(dst, text, x, baselineY, face, o.StrokeCol, o.Stroke, o.StrokeCol)
	}
	drawFace(dst, text, x, baselineY, face, o.Color, 0, color.NRGBA{})
	return measureLocked(text, face), nil
}

// measureLocked measures without taking fontMu; the caller holds it.
func measureLocked(text string, face font.Face) float32 {
	d := font.Drawer{Face: face}
	adv, _ := d.BoundString(text)
	return float32((adv.Max.X - adv.Min.X).Round())
}

func drawFace(dst draw.Image, text string, x, y float32, face font.Face, c color.NRGBA, stroke int, strokeCol color.NRGBA) {
	if stroke <= 0 {
		d := font.Drawer{Dst: dst, Src: image.NewUniform(c), Face: face, Dot: fixed.Point26_6{X: fixed.I(int(x)), Y: fixed.I(int(y))}}
		d.DrawString(text)
		return
	}
	// Emulate a stroke by drawing the glyphs around the origin; an outline
	// library is not needed for the widths this tool offers.
	steps := stroke * 8
	if steps < 8 {
		steps = 8
	}
	for i := 0; i < steps; i++ {
		angle := float64(i) * 2 * 3.141592653589793 / float64(steps)
		dx := int(float64(stroke)*1.4*cos(angle) + 0.5)
		dy := int(float64(stroke)*1.4*sin(angle) + 0.5)
		d := font.Drawer{
			Dst: dst, Src: image.NewUniform(strokeCol), Face: face,
			Dot: fixed.Point26_6{X: fixed.I(int(x) + dx), Y: fixed.I(int(y) + dy)},
		}
		d.DrawString(text)
	}
}

// MeasureText reports the advance width of text in face.
func MeasureText(text string, face font.Face) float32 {
	fontMu.Lock()
	defer fontMu.Unlock()
	return measureLocked(text, face)
}

// MeasureHeight reports the line height of face.
func MeasureHeight(face font.Face) float32 {
	fontMu.Lock()
	defer fontMu.Unlock()
	m := face.Metrics()
	return float32((m.Ascent + m.Descent).Ceil())
}

// WrapText performs CJK-aware greedy wrapping: a rune that is not a space may
// start a new line anywhere, western words wrap at their spaces.
func WrapText(text string, face font.Face, maxWidth float32) []string {
	if maxWidth <= 0 {
		return []string{text}
	}
	fontMu.Lock()
	defer fontMu.Unlock()
	var out []string
	for _, raw := range strings.Split(strings.ReplaceAll(text, "\r\n", "\n"), "\n") {
		if raw == "" {
			out = append(out, "")
			continue
		}
		var line strings.Builder
		var lineWidth float32
		d := font.Drawer{Face: face}
		measure := func(s string) float32 {
			adv, _ := d.BoundString(s)
			return float32((adv.Max.X - adv.Min.X).Round())
		}
		for _, word := range splitRuns(raw) {
			w := measure(word)
			if lineWidth+w > maxWidth && line.Len() > 0 && strings.TrimSpace(word) != "" {
				out = append(out, line.String())
				line.Reset()
				lineWidth = 0
				word = strings.TrimLeft(word, " ")
				w = measure(word)
			}
			line.WriteString(word)
			lineWidth += w
		}
		out = append(out, line.String())
	}
	return out
}

// splitRuns splits on spaces but keeps CJK characters as individual tokens so a
// long sentence without spaces still wraps.
func splitRuns(s string) []string {
	var out []string
	var buf strings.Builder
	flush := func() {
		if buf.Len() > 0 {
			out = append(out, buf.String())
			buf.Reset()
		}
	}
	for _, r := range s {
		switch {
		case r == ' ':
			buf.WriteRune(r)
			flush()
		case r < 0x2E80:
			buf.WriteRune(r)
		default:
			flush()
			out = append(out, string(r))
		}
	}
	flush()
	return out
}

// hexToColor parses #RGB or #RRGGBB into an opaque colour.
func hexToColor(value string, alpha uint8) color.NRGBA {
	text := strings.TrimPrefix(strings.TrimSpace(value), "#")
	if len(text) == 3 {
		text = string([]byte{text[0], text[0], text[1], text[1], text[2], text[2]})
	}
	if len(text) != 6 {
		return color.NRGBA{R: 0, G: 0, B: 0, A: alpha}
	}
	var r, g, b uint8
	if _, err := fmt.Sscanf(text, "%02x%02x%02x", &r, &g, &b); err != nil {
		return color.NRGBA{A: alpha}
	}
	return color.NRGBA{R: r, G: g, B: b, A: alpha}
}

func cos(v float64) float64 { return cosTable(v) }
func sin(v float64) float64 { return sinTable(v) }
