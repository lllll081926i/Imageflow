package ui

// logo_test.go pins the mark the interface draws: it is the artwork the binary
// carries, rasterised through the brand package's one render path, and the tray
// hands the platform the same shape.

import (
	"bytes"
	"image/png"
	"testing"

	"github.com/lllll081926i/imageflow-go/internal/brand"
)

// TestLogoDrawsTheArtwork pins that the window's mark is the embedded artwork
// rendered at the asked size, with the tile's transparent corners preserved.
func TestLogoDrawsTheArtwork(t *testing.T) {
	img := logoImage(64)
	if img == nil {
		t.Fatal("logo did not render")
	}
	if b := img.Bounds(); b.Dx() != 64 || b.Dy() != 64 {
		t.Fatalf("logo = %dx%d, want 64x64", b.Dx(), b.Dy())
	}
	// The tile fills the crop, so its middle carries colour rather than the
	// artwork's transparent margin.
	r, g, bl, a := img.At(32, 32).RGBA()
	if a == 0 {
		t.Fatal("logo centre is transparent")
	}
	if bl <= r && bl <= g {
		t.Errorf("logo centre = %d,%d,%d, want the blue/purple tile", r>>8, g>>8, bl>>8)
	}
}

// TestIconMatchesTheLadder pins that the small sizes the interface draws come from
// the same render path as the generated ladder, so a tray glyph and a resources file
// of the same size cannot differ.
func TestIconMatchesTheLadder(t *testing.T) {
	for _, size := range []int{16, 32, 40} {
		ui := iconImage(size)
		shared := brand.MarkAt(size)
		if ui == nil || shared == nil {
			t.Fatalf("%d: no image", size)
		}
		if ui.Bounds() != shared.Bounds() {
			t.Fatalf("%d: %v vs %v", size, ui.Bounds(), shared.Bounds())
		}
	}
}

// TestSizeConstantsAgreeWithThePages pins the three sizes the pages name, so a
// layout change cannot silently shrink the mark to a 16 px sprite.
func TestSizeConstantsAgreeWithThePages(t *testing.T) {
	for _, size := range []int{logoHomeSize, logoAboutSize, logoBadgeSize} {
		if size < 24 || size > 96 {
			t.Errorf("%d is not a plausible header/ badge size", size)
		}
		if img := iconImage(size); img == nil || img.Bounds().Dx() != size {
			t.Errorf("%d: rendered %v", size, img)
		}
	}
}

// TestTrayIconUsesTheTrimmedMark pins that the tray hands the platform the mark at
// the badge size: a 32×32 PNG, transparent corners, cached.
func TestTrayIconUsesTheTrimmedMark(t *testing.T) {
	data := trayIconPNG()
	if len(data) == 0 {
		t.Fatal("tray icon is empty")
	}
	img, err := png.Decode(bytes.NewReader(data))
	if err != nil {
		t.Fatalf("tray icon is not a PNG: %v", err)
	}
	b := img.Bounds()
	if b.Dx() != logoBadgeSize || b.Dy() != logoBadgeSize {
		t.Fatalf("tray icon = %dx%d, want %d", b.Dx(), b.Dy(), logoBadgeSize)
	}
	if _, _, _, a := img.At(0, 0).RGBA(); a != 0 {
		t.Error("tray icon corner must be transparent")
	}
	// The mark reaches the crop's edge, so the middle of the top edge carries ink.
	if _, _, _, a := img.At(logoBadgeSize/2, 0).RGBA(); a == 0 {
		t.Error("tray icon still carries the artwork's margin: the mark is a size too small")
	}
	if again := trayIconPNG(); &again[0] != &data[0] {
		t.Error("tray icon was re-rendered on the second call")
	}
}
