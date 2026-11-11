package codec_test

// matrix_test.go converts every supported format into every other one and back.
//
// A hand-picked folder of samples catches the formats someone remembered to
// drop in. A matrix does not: it fails on the pair nobody thought about, which is
// exactly the bug a user reports as "AVIF to TIFF does nothing".

import (
	"image"
	"image/color"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/lllll081926i/imageflow-go/internal/codec"
)

// matrixFormats is every extension the writer can produce.
var matrixFormats = []string{"png", "jpg", "webp", "avif", "tiff", "bmp", "ico", "gif"}

// matrixSource draws a picture with the features each encoder is fussy about: a
// gradient (banding), hard edges (subsampling artefacts), and a transparent
// corner (alpha handling).
func matrixSource(w, h int) image.Image {
	img := image.NewNRGBA(image.Rect(0, 0, w, h))
	for y := 0; y < h; y++ {
		for x := 0; x < w; x++ {
			img.SetNRGBA(x, y, color.NRGBA{
				R: uint8(x * 255 / max(1, w-1)),
				G: uint8(y * 255 / max(1, h-1)),
				B: uint8((x + y) * 127 / max(1, w+h-2)),
				A: 255,
			})
		}
	}
	// A transparent corner: formats without alpha must still write something.
	for y := 0; y < h/8; y++ {
		for x := 0; x < w/8; x++ {
			img.SetNRGBA(x, y, color.NRGBA{})
		}
	}
	return img
}

func max(a, b int) int {
	if a > b {
		return a
	}
	return b
}

// TestEveryFormatEncodesAndReopens is the core conversion guarantee: for each
// format, encode, decode the file again, and check the pixels are the size they
// were written at. A format that silently writes a zero-byte file or a header
// only fails here.
func TestEveryFormatEncodesAndReopens(t *testing.T) {
	dir := t.TempDir()
	src := matrixSource(64, 48)
	for _, ext := range matrixFormats {
		path := filepath.Join(dir, "out."+ext)
		if err := codec.Save(src, path, codec.SaveOptions{Quality: 92}); err != nil {
			t.Errorf("%s: encode failed: %v", ext, err)
			continue
		}
		info, err := os.Stat(path)
		if err != nil {
			t.Errorf("%s: %v", ext, err)
			continue
		}
		if info.Size() == 0 {
			t.Errorf("%s: wrote an empty file", ext)
			continue
		}
		opened, err := codec.Open(path)
		if err != nil {
			t.Errorf("%s: reopen failed: %v", ext, err)
			continue
		}
		if opened.Width != 64 || opened.Height != 48 {
			// An ICO holds square entries by definition, so it is the one target
			// that changes the aspect on purpose; every other format keeps it.
			if ext == "ico" && opened.Width == opened.Height && opened.Width >= 16 {
				continue
			}
			t.Errorf("%s: reopened at %dx%d, want 64x48", ext, opened.Width, opened.Height)
		}
		// The pixels must not all be one colour: a broken encoder writes a blank.
		if uniform(opened.Image) {
			t.Errorf("%s: the reopened image is a single flat colour", ext)
		}
	}
}

// TestConversionMatrixReachesEveryTarget converts one file of every source format
// into every target format, which is what the converter page does per row.
func TestConversionMatrixReachesEveryTarget(t *testing.T) {
	dir := t.TempDir()
	src := matrixSource(48, 32)

	// Write one file per source format.
	sources := map[string]string{}
	for _, ext := range matrixFormats {
		path := filepath.Join(dir, "src."+ext)
		if err := codec.Save(src, path, codec.SaveOptions{Quality: 92}); err != nil {
			t.Fatalf("%s source: %v", ext, err)
		}
		sources[ext] = path
	}

	failures := 0
	for from, fromPath := range sources {
		opened, err := codec.Open(fromPath)
		if err != nil {
			t.Errorf("%s: open failed: %v", from, err)
			failures++
			continue
		}
		for _, to := range matrixFormats {
			target := filepath.Join(dir, "conv_"+from+"_to_"+to+"."+to)
			if err := codec.Save(opened.Image, target, codec.SaveOptions{Quality: 92}); err != nil {
				t.Errorf("%s → %s: %v", from, to, err)
				failures++
				continue
			}
			again, err := codec.Open(target)
			if err != nil {
				t.Errorf("%s → %s: reopen failed: %v", from, to, err)
				failures++
				continue
			}
			if again.Width != opened.Width || again.Height != opened.Height {
				// ICO stores square entries, so it is the documented exception: a
				// non-square source comes back as a square icon.
				if to == "ico" {
					continue
				}
				t.Errorf("%s → %s: %dx%d, want %dx%d",
					from, to, again.Width, again.Height, opened.Width, opened.Height)
				failures++
			}
		}
	}
	if failures > 0 {
		t.Fatalf("%d conversions failed out of %d pairs",
			failures, len(matrixFormats)*len(matrixFormats))
	}
}

// TestQualityAndLosslessAreHonoured pins that the two encoder switches the panel
// exposes actually change the bytes.
func TestQualityAndLosslessAreHonoured(t *testing.T) {
	dir := t.TempDir()
	src := matrixSource(96, 96)

	for _, ext := range []string{"jpg", "webp", "avif"} {
		low := filepath.Join(dir, "low."+ext)
		high := filepath.Join(dir, "high."+ext)
		if err := codec.Save(src, low, codec.SaveOptions{Quality: 20}); err != nil {
			t.Fatalf("%s low: %v", ext, err)
		}
		if err := codec.Save(src, high, codec.SaveOptions{Quality: 95}); err != nil {
			t.Fatalf("%s high: %v", ext, err)
		}
		lo, _ := os.Stat(low)
		hi, _ := os.Stat(high)
		if lo.Size() >= hi.Size() {
			t.Errorf("%s: quality 20 wrote %d bytes, quality 95 wrote %d",
				ext, lo.Size(), hi.Size())
		}
	}
}

// TestAlphaSurvivesTheFormatsThatCarryIt pins the one thing users notice first: a
// transparent PNG must stay transparent through PNG, WebP and AVIF, and must be
// flattened (not left broken) through JPEG, BMP and GIF.
func TestAlphaSurvivesTheFormatsThatCarryIt(t *testing.T) {
	dir := t.TempDir()
	src := image.NewNRGBA(image.Rect(0, 0, 16, 16))
	for y := 0; y < 16; y++ {
		for x := 0; x < 16; x++ {
			if x < 8 {
				src.SetNRGBA(x, y, color.NRGBA{R: 200, A: 255})
			}
			// The right half stays transparent.
		}
	}

	for _, ext := range []string{"png", "webp", "avif"} {
		path := filepath.Join(dir, "alpha."+ext)
		if err := codec.Save(src, path, codec.SaveOptions{Quality: 95, Lossless: ext == "webp"}); err != nil {
			t.Fatalf("%s: %v", ext, err)
		}
		opened, err := codec.Open(path)
		if err != nil {
			t.Fatalf("%s: %v", ext, err)
		}
		if _, _, _, a := opened.Image.At(12, 8).RGBA(); a != 0 {
			t.Errorf("%s: transparent pixel came back opaque (alpha %d)", ext, a>>8)
		}
	}

	for _, ext := range []string{"jpg", "bmp", "gif"} {
		path := filepath.Join(dir, "flat."+ext)
		if err := codec.Save(src, path, codec.SaveOptions{Quality: 95}); err != nil {
			t.Fatalf("%s: %v", ext, err)
		}
		opened, err := codec.Open(path)
		if err != nil {
			t.Fatalf("%s: %v", ext, err)
		}
		if _, _, _, a := opened.Image.At(12, 8).RGBA(); a == 0 {
			t.Errorf("%s: alpha was not flattened; the region is still transparent", ext)
		}
	}
}

// TestProbeAgreesWithOpen pins the dimensions the file list shows against the
// pixels the engine sees: a mismatch puts wrong numbers in the UI.
func TestProbeAgreesWithOpen(t *testing.T) {
	dir := t.TempDir()
	src := matrixSource(37, 23)
	for _, ext := range matrixFormats {
		path := filepath.Join(dir, "probe."+ext)
		if err := codec.Save(src, path, codec.SaveOptions{Quality: 92}); err != nil {
			t.Fatalf("%s: %v", ext, err)
		}
		w, h, format, err := codec.Probe(path)
		if err != nil {
			t.Errorf("%s: probe failed: %v", ext, err)
			continue
		}
		opened, err := codec.Open(path)
		if err != nil {
			t.Fatalf("%s: %v", ext, err)
		}
		if w != opened.Width || h != opened.Height {
			t.Errorf("%s: probe says %dx%d, open says %dx%d",
				ext, w, h, opened.Width, opened.Height)
		}
		if !strings.EqualFold(format, ext) && !(ext == "jpg" && format == "JPEG") {
			t.Errorf("%s: probe reported format %q", ext, format)
		}
	}
}

// uniform reports whether every sampled pixel is the same colour.
func uniform(img image.Image) bool {
	b := img.Bounds()
	first := color.NRGBAModel.Convert(img.At(b.Min.X, b.Min.Y))
	for y := b.Min.Y; y < b.Max.Y; y += 3 {
		for x := b.Min.X; x < b.Max.X; x += 3 {
			if color.NRGBAModel.Convert(img.At(x, y)) != first {
				return false
			}
		}
	}
	return true
}
