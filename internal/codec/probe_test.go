package codec_test

import (
	"image"
	"image/color"
	"path/filepath"
	"testing"

	"github.com/lllll081926i/imageflow-go/internal/codec"
)

// TestProbeLabelsStillPNGAsPNG pins the format badge the file rows show.
// github.com/kettek/apng registers its decoder for the PNG magic number and
// sorts before image/png, so image.DecodeConfig reports a still PNG as "apng"
// unless Probe corrects it from the extension.
func TestProbeLabelsStillPNGAsPNG(t *testing.T) {
	dir := t.TempDir()
	cases := []struct {
		name   string
		ext    string
		format string
	}{
		{"a.png", "png", "PNG"},
		{"b.jpg", "jpg", "JPEG"},
		{"c.bmp", "bmp", "BMP"},
		{"d.tiff", "tiff", "TIFF"},
		{"e.webp", "webp", "WEBP"},
	}
	for _, c := range cases {
		path := filepath.Join(dir, c.name)
		img := image.NewNRGBA(image.Rect(0, 0, 6, 4))
		for i := range img.Pix {
			img.Pix[i] = 200
		}
		img.SetNRGBA(0, 0, color.NRGBA{R: 255, A: 255})
		if err := codec.Save(img, path, codec.SaveOptions{Quality: 90}); err != nil {
			t.Fatalf("%s: %v", c.ext, err)
		}
		w, h, got, err := codec.Probe(path)
		if err != nil {
			t.Fatalf("%s: %v", c.ext, err)
		}
		if w != 6 || h != 4 {
			t.Errorf("%s: %dx%d", c.ext, w, h)
		}
		if got != c.format {
			t.Errorf("%s: format = %q, want %q", c.ext, got, c.format)
		}
	}
}
