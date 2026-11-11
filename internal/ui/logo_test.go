package ui

// logo_test.go pins the application mark: it is the original project's logo.svg,
// it rasterises, and the trimmed variant used for small sizes is the same shape
// with the margin removed rather than a different picture.

import (
	"bytes"
	"image/png"
	"testing"

	"github.com/lllll081926i/imageflow-go/internal/brand"
)

// TestLogoIsTheOriginalMark pins that the compiled-in mark is the original
// project's logo.svg: it must rasterise, it must be square, and the centre dot
// must be the blue/indigo the original shipped rather than a flat colour.
func TestLogoIsTheOriginalMark(t *testing.T) {
	img := logoImage(64)
	if img == nil {
		t.Fatal("logo did not rasterise")
	}
	b := img.Bounds()
	if b.Dx() != 64 || b.Dy() != 64 {
		t.Fatalf("logo = %dx%d, want 64x64", b.Dx(), b.Dy())
	}
	// The full document keeps its 9% margin: the corner is transparent.
	if _, _, _, a := img.At(0, 0).RGBA(); a != 0 {
		t.Errorf("logo corner alpha = %d, want transparent", a)
	}
	r, g, bl, a := img.At(32, 32).RGBA()
	if a == 0 {
		t.Fatal("logo centre is transparent")
	}
	if bl>>8 < 100 || bl <= r || bl <= g {
		t.Errorf("logo centre = %d,%d,%d, want a blue/indigo dot", r>>8, g>>8, bl>>8)
	}
}

// TestIconIsTheSameShapeWithoutMargin pins the trimmed variant.
//
// The packaging step renders resources/icon.png from brand.IconSVG, and the app's
// title bar and tray use the same document. The trim is a viewBox change only, so
// the mark must reach the edges: at 16 px a 9% margin per side is two wasted
// pixels of a twelve-pixel picture, which is why the small icons read as too
// small.
func TestIconIsTheSameShapeWithoutMargin(t *testing.T) {
	img := iconImage(64)
	if img == nil {
		t.Fatal("icon did not rasterise")
	}
	if b := img.Bounds(); b.Dx() != 64 || b.Dy() != 64 {
		t.Fatalf("icon = %dx%d, want 64x64", b.Dx(), b.Dy())
	}
	// The rounded square's own edge now sits within a pixel of the canvas edge on
	// all four sides: sample the middle of each edge and require ink.
	edges := []struct {
		name string
		x, y int
	}{
		{"top", 32, 0},
		{"bottom", 32, 63},
		{"left", 0, 32},
		{"right", 63, 32},
	}
	for _, e := range edges {
		if _, _, _, a := img.At(e.x, e.y).RGBA(); a == 0 {
			t.Errorf("%s edge is transparent: the margin was not trimmed", e.name)
		}
	}
	// The corners stay clear: the square is rounded.
	if _, _, _, a := img.At(0, 0).RGBA(); a != 0 {
		t.Errorf("icon corner alpha = %d, want transparent", a)
	}
}

// TestIconDocumentIsTheLogoWithAViewBox pins that the two documents differ only in
// their viewBox: the shape must not drift between the window and the installer.
func TestIconDocumentIsTheLogoWithAViewBox(t *testing.T) {
	if brand.LogoSVG == brand.IconSVG {
		t.Fatal("the icon document is identical: the margin was never trimmed")
	}
	// Every drawing element of the logo must appear in the icon document.
	for _, tag := range []string{
		`rx="200"`,
		`stroke-width="32"`,
		`M360 512C360 423.634`,
		`M660 512C660 600.366`,
		`r="60"`,
		`#007AFF`, `#5856D6`, `#AF52DE`,
	} {
		if !bytes.Contains([]byte(brand.IconSVG), []byte(tag)) {
			t.Errorf("icon document lost %q", tag)
		}
	}
	if !bytes.Contains([]byte(brand.IconSVG), []byte(`viewBox="96 96 832 832"`)) {
		t.Error("icon document does not trim to the mark's bounds")
	}
}

// TestTrayIconUsesTheTrimmedMark pins that the tray hands the platform the same
// mark at the size a menu bar asks for: 32×32, transparent corners, and cached.
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
	if b.Dx() != 32 || b.Dy() != 32 {
		t.Fatalf("tray icon = %dx%d, want 32x32", b.Dx(), b.Dy())
	}
	if _, _, _, a := img.At(0, 0).RGBA(); a != 0 {
		t.Error("tray icon corner must be transparent")
	}
	// The mark reaches the edge of the trimmed document, so the middle of the top
	// edge carries ink. With the untrimmed document it was transparent.
	if _, _, _, a := img.At(16, 0).RGBA(); a == 0 {
		t.Error("tray icon still carries the logo's margin: the mark is a size too small")
	}
	if again := trayIconPNG(); &again[0] != &data[0] {
		t.Error("tray icon was re-rendered on the second call")
	}
}
