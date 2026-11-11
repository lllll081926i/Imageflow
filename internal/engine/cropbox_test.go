package engine_test

import (
	"path/filepath"
	"testing"

	"github.com/lllll081926i/imageflow-go/internal/codec"
	"github.com/lllll081926i/imageflow-go/internal/engine"
)

// TestParseCropRatio pins the spellings people type for a ratio.
func TestParseCropRatio(t *testing.T) {
	cases := map[string]engine.CropRatio{
		"16:9":    {W: 16, H: 9},
		"16x9":    {W: 16, H: 9},
		"16/9":    {W: 16, H: 9},
		"16×9":    {W: 16, H: 9},
		" 4 : 3 ": {W: 4, H: 3},
	}
	for text, want := range cases {
		got, ok := engine.ParseCropRatio(text)
		if !ok || got != want {
			t.Errorf("%q = %v, %v; want %v", text, got, ok, want)
		}
	}
	for _, bad := range []string{"", "16", "0:9", "a:b", "16:9:1"} {
		if _, ok := engine.ParseCropRatio(bad); ok {
			t.Errorf("%q must not parse", bad)
		}
	}
}

// TestCropBoxKeepsTheRatio pins the geometry: a 3:2 source cropped to 16:9 comes
// out 16:9, and the anchor decides which part of the picture survives.
func TestCropBoxKeepsTheRatio(t *testing.T) {
	dir := t.TempDir()
	src := samplePNG(t, dir, 300, 200)

	for _, anchor := range engine.CropAnchors {
		out := filepath.Join(dir, "crop_"+anchor+".png")
		res := engine.CropBox(engine.CropRequest{
			InputPath: src, OutputPath: out, RatioW: 16, RatioH: 9, Anchor: anchor,
		})
		if !res.Success {
			t.Fatalf("%s: %s", anchor, res.Error)
		}
		opened, err := codec.Open(out)
		if err != nil {
			t.Fatal(err)
		}
		// Largest 16:9 box inside 300x200 is 300x168 (rounded down by the ratio).
		if opened.Width != 300 || opened.Height != 168 {
			t.Errorf("%s = %dx%d, want 300x168", anchor, opened.Width, opened.Height)
		}
	}
}

// TestCropBoxScalesAndRounds pins the two post-steps: an exact output box, and
// corners that clear when asked.
func TestCropBoxScalesAndRounds(t *testing.T) {
	dir := t.TempDir()
	src := samplePNG(t, dir, 400, 400)

	out := filepath.Join(dir, "square.png")
	res := engine.CropBox(engine.CropRequest{
		InputPath: src, OutputPath: out, RatioW: 1, RatioH: 1,
		ScaledWidth: 128, ScaledHeight: 128,
	})
	if !res.Success {
		t.Fatalf("%s", res.Error)
	}
	opened, err := codec.Open(out)
	if err != nil {
		t.Fatal(err)
	}
	if opened.Width != 128 || opened.Height != 128 {
		t.Errorf("scaled crop = %dx%d, want 128x128", opened.Width, opened.Height)
	}

	rounded := filepath.Join(dir, "rounded.png")
	res = engine.CropBox(engine.CropRequest{
		InputPath: src, OutputPath: rounded, RatioW: 1, RatioH: 1,
		ScaledWidth: 64, ScaledHeight: 64, Radius: 50,
	})
	if !res.Success {
		t.Fatalf("rounded: %s", res.Error)
	}
	img, err := codec.Open(rounded)
	if err != nil {
		t.Fatal(err)
	}
	if _, _, _, a := img.Image.At(0, 0).RGBA(); a != 0 {
		t.Errorf("corner alpha = %d, want a clear corner", a)
	}
	if _, _, _, a := img.Image.At(32, 32).RGBA(); a == 0 {
		t.Error("centre is transparent; the mask ate the picture")
	}
}

// TestCropBoxRejectsBadRatio pins the guard.
func TestCropBoxRejectsBadRatio(t *testing.T) {
	dir := t.TempDir()
	src := samplePNG(t, dir, 40, 40)
	res := engine.CropBox(engine.CropRequest{
		InputPath: src, OutputPath: filepath.Join(dir, "x.png"), RatioW: 0, RatioH: 9,
	})
	if res.Success || res.ErrorCode != "CROP_BAD_RATIO" {
		t.Fatalf("got %+v", res)
	}
}
