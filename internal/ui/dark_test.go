package ui

import (
	"testing"

	myui "github.com/egoist/mygo/ui"
)

// TestDarkToolPageHasNoWhiteBlocks walks the real dark frame and fails if a
// large patch of pure white appears, which is what an unthemed surface looks
// like on the first frame of a colour transition.
func TestDarkToolPageHasNoWhiteBlocks(t *testing.T) {
	a := newTestApp(t)
	for i := range a.Tools {
		a.Active = i
		a.ShowSettings = false
		tt := myui.NewTester(a.View, 1440, 900)
		tt.SetDark(true)
		tt.Frame()
		img := tt.Image()
		b := img.Bounds()

		white := 0
		for y := b.Min.Y; y < b.Max.Y; y += 3 {
			for x := b.Min.X; x < b.Max.X; x += 3 {
				c := img.RGBAAt(x, y)
				if c.R > 250 && c.G > 250 && c.B > 250 {
					white++
				}
			}
		}
		total := (b.Dx() / 3) * (b.Dy() / 3)
		if white*1000/total > 3 {
			t.Fatalf("tool %d: %d of %d sampled pixels are pure white (%.1f%%), a surface is unthemed",
				i, white, total, float64(white)*100/float64(total))
		}
	}
}
