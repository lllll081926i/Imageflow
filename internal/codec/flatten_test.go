package codec

import (
	"image"
	"image/color"
	"testing"
)

// TestFlattenAppliesAlphaOnce pins the white-background blend. RGBA() returns
// alpha-premultiplied channels; multiplying them by alpha a second time made
// translucent pixels roughly half as bright as the source when saving to a
// format without alpha.
func TestFlattenAppliesAlphaOnce(t *testing.T) {
	src := image.NewNRGBA(image.Rect(0, 0, 1, 1))
	src.SetNRGBA(0, 0, color.NRGBA{R: 200, G: 100, B: 50, A: 128})

	got := color.NRGBAModel.Convert(flatten(src).At(0, 0)).(color.NRGBA)

	// 200 * 128/255 + 255 * 127/255 = 227.39 -> 227.
	// The old double-premultiplied path produced about 100.
	const wantR uint8 = 227
	if got.R != wantR {
		t.Errorf("R = %d, want %d (alpha must be applied exactly once)", got.R, wantR)
	}
	if got.A != 255 {
		t.Errorf("A = %d, want an opaque result", got.A)
	}
}

func TestFlattenKeepsOpaqueImageUntouched(t *testing.T) {
	src := image.NewNRGBA(image.Rect(0, 0, 1, 1))
	src.SetNRGBA(0, 0, color.NRGBA{R: 12, G: 34, B: 56, A: 255})
	if flatten(src) != src {
		t.Error("an opaque image must be returned as-is")
	}
}
