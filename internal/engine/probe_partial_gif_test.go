package engine

import (
	"bytes"
	"image"
	"image/color"
	"image/gif"
	"os"
	"path/filepath"
	"testing"
)

// partialGIFBytes builds a two-frame GIF whose second frame only carries the
// changed sub-rectangle, as every optimiser produces.
func partialGIFBytes(t *testing.T) []byte {
	t.Helper()
	pal := color.Palette{
		color.NRGBA{A: 0},
		color.NRGBA{R: 255, A: 255},
		color.NRGBA{B: 255, A: 255},
	}
	first := image.NewPaletted(image.Rect(0, 0, 40, 40), pal)
	for i := range first.Pix {
		first.Pix[i] = 1
	}
	second := image.NewPaletted(image.Rect(20, 20, 30, 30), pal)
	for i := range second.Pix {
		second.Pix[i] = 2
	}
	var buf bytes.Buffer
	err := gif.EncodeAll(&buf, &gif.GIF{
		Image:     []*image.Paletted{first, second},
		Delay:     []int{20, 20},
		LoopCount: 0,
		Config:    image.Config{ColorModel: pal, Width: 40, Height: 40},
	})
	if err != nil {
		t.Fatalf("encode: %v", err)
	}
	return buf.Bytes()
}

// TestPartialGIFFramesKeepPreviousContent pins the compositing rule for optimised
// GIFs: a frame that only carries a changed sub-rectangle must still show the
// pixels the previous frame left on the canvas.
func TestPartialGIFFramesKeepPreviousContent(t *testing.T) {
	dir := t.TempDir()
	_ = dir
	src := filepath.Join(t.TempDir(), "partial.gif")
	if err := os.WriteFile(src, partialGIFBytes(t), 0o644); err != nil {
		t.Fatal(err)
	}
	anim, err := LoadAnimation(src)
	if err != nil {
		t.Fatal(err)
	}
	if len(anim.Frames) != 2 {
		t.Fatalf("frames = %d", len(anim.Frames))
	}
	// Frame 2 only encodes the 10x10 blue box at (20,20); the red background of
	// frame 1 must still be there.
	r, g, b, a := anim.Frames[1].Image.At(2, 2).RGBA()
	if a < 0x8000 {
		t.Fatalf("frame 2 corner is transparent, previous content was dropped")
	}
	if r>>8 < 200 || g>>8 > 60 || b>>8 > 60 {
		t.Fatalf("frame 2 corner = %d,%d,%d, want the red background", r>>8, g>>8, b>>8)
	}
	r, g, b, _ = anim.Frames[1].Image.At(25, 25).RGBA()
	if b>>8 < 200 || r>>8 > 60 {
		t.Fatalf("frame 2 centre = %d,%d,%d, want blue", r>>8, g>>8, b>>8)
	}
	_ = color.NRGBA{}
}
