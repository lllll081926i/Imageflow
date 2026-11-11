package engine_test

import (
	"image"
	"image/color"
	"path/filepath"
	"testing"

	"github.com/lllll081926i/imageflow-go/internal/codec"
	"github.com/lllll081926i/imageflow-go/internal/engine"
)

// TestRichTextThemesChangeColours pins the four built-in canvases: each theme must
// paint its own background, and a dark theme must not stay white.
func TestRichTextThemesChangeColours(t *testing.T) {
	blocks := []engine.RichTextBlock{
		{Type: "heading", Level: 1, Text: "标题"},
		{Type: "paragraph", Text: "正文"},
	}
	cases := []struct {
		theme   string
		darkish bool
	}{
		{engine.RichTextThemeLight, false},
		{engine.RichTextThemeDark, true},
		{engine.RichTextThemeSepia, false},
		{engine.RichTextThemeOcean, false},
	}
	var seen []color.NRGBA
	for _, c := range cases {
		img, err := engine.RenderRichTextImage(engine.RichTextRequest{
			Blocks: blocks, Width: 400, Padding: 24, FontSize: 16,
			LineHeight: 1.6, Theme: c.theme,
		})
		if err != nil {
			t.Fatalf("%q: %v", c.theme, err)
		}
		// Sample a corner: it is always background.
		r, g, b, _ := img.At(2, 2).RGBA()
		corner := color.NRGBA{R: uint8(r >> 8), G: uint8(g >> 8), B: uint8(b >> 8), A: 255}
		lum := 0.299*float64(corner.R) + 0.587*float64(corner.G) + 0.114*float64(corner.B)
		if c.darkish && lum > 90 {
			t.Errorf("theme %q background is not dark: %+v", c.theme, corner)
		}
		if !c.darkish && lum < 200 {
			t.Errorf("theme %q background is not light: %+v", c.theme, corner)
		}
		seen = append(seen, corner)
	}
	for i := 1; i < len(seen); i++ {
		if seen[i] == seen[0] {
			t.Errorf("theme %d repeats theme 0's background", i)
		}
	}
}

// TestRichTextImageBlock draws a picture on the canvas, which the panel offers as
// an image block.
func TestRichTextImageBlock(t *testing.T) {
	dir := t.TempDir()
	pic := samplePNG(t, dir, 120, 60)
	out := filepath.Join(dir, "with-image.png")
	res := engine.RenderRichText(engine.RichTextRequest{
		Blocks: []engine.RichTextBlock{
			{Type: "heading", Level: 1, Text: "带图"},
			{Type: "image", Path: pic, WidthPercent: 50},
		},
		OutputPath: out,
		Width:      600,
		Padding:    24,
		FontSize:   16,
		LineHeight: 1.6,
	})
	if !res.Success {
		t.Fatalf("%s", res.Error)
	}
	if res.Height < 120 {
		t.Fatalf("canvas height %d does not include the image", res.Height)
	}
	opened, err := codec.Open(out)
	if err != nil {
		t.Fatal(err)
	}
	// A 120x60 source at 50% of a 600-wide canvas, less padding, is about
	// 276x138; the drawn picture must keep that aspect ratio and not spill.
	if opened.Height > 400 {
		t.Errorf("canvas grew to %d, the image block ignored its width fraction", opened.Height)
	}
	// The image block paints pixels the background does not: count non-background
	// pixels below the heading.
	nonBg := 0
	for y := opened.Height / 2; y < opened.Height; y += 2 {
		for x := 0; x < opened.Width; x += 2 {
			r, g, b, _ := opened.Image.At(x, y).RGBA()
			if r>>8 < 240 || g>>8 < 240 || b>>8 < 240 {
				nonBg++
			}
		}
	}
	if nonBg < 50 {
		t.Fatalf("image block painted only %d pixels", nonBg)
	}
	_ = image.Rect
}
