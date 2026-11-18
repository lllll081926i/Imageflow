package brand

// brand_test.go pins the artwork and the ladder rendered from it.
//
// The mark is a raster now, so the assertions are about the pixels the binary
// carries and the sizes the generator writes, not about an SVG document's tags.

import (
	"bytes"
	"image"
	"image/color"
	"image/png"
	"os"
	"testing"
)

// TestLogoSourceDecodes pins that the embedded artwork is a real square PNG with
// a transparent surround: it is the picture the whole icon family derives from.
func TestLogoSourceDecodes(t *testing.T) {
	src := LogoSource()
	if len(src) == 0 {
		t.Fatal("the artwork is not embedded")
	}
	img, err := Logo()
	if err != nil {
		t.Fatalf("decode artwork: %v", err)
	}
	b := img.Bounds()
	if b.Dx() != b.Dy() || b.Dx() < 512 {
		t.Fatalf("artwork is %dx%d, want a square of at least 512", b.Dx(), b.Dy())
	}
	if _, _, _, a := img.At(0, 0).RGBA(); a != 0 {
		t.Error("the artwork's corner must be transparent: it is the tile's shadow's tail")
	}
}

// TestMarkBoundsSitOnTheTile pins the crop the ladder is rendered from: it must be
// a square, inside the artwork, a real trim, and centred on the tile rather than on
// the canvas.
//
// The tile's centre is measured from the artwork itself rather than copied from the
// constant: the soft shadow makes the canvas centre a poor proxy, and a test that
// hard-coded the offset would pass even after the crop moved.
func TestMarkBoundsSitOnTheTile(t *testing.T) {
	bounds := MarkBounds()
	if bounds.Empty() {
		t.Fatal("empty mark bounds")
	}
	if bounds.Dx() != bounds.Dy() {
		t.Fatalf("mark bounds are %dx%d, want a square: a non-square crop stretches the tile", bounds.Dx(), bounds.Dy())
	}
	canvas := LogoBounds()
	if !bounds.In(canvas) {
		t.Fatalf("mark bounds %v are outside the artwork %v", bounds, canvas)
	}
	if bounds == canvas {
		t.Fatal("the crop equals the artwork: the margin was never trimmed")
	}
	img, err := Logo()
	if err != nil {
		t.Fatal(err)
	}
	tile := opaqueBounds(img)
	cx := (bounds.Min.X + bounds.Max.X) / 2
	cy := (bounds.Min.Y + bounds.Max.Y) / 2
	tileCX := (tile.Min.X + tile.Max.X) / 2
	tileCY := (tile.Min.Y + tile.Max.Y) / 2
	if diff := cx - tileCX; diff > 6 || diff < -6 {
		t.Errorf("crop centre x = %d, tile centre = %d", cx, tileCX)
	}
	if diff := cy - tileCY; diff > 6 || diff < -6 {
		t.Errorf("crop centre y = %d, tile centre = %d", cy, tileCY)
	}
	// A crop far smaller than the tile would clip it; one much larger would put the
	// artwork's margin back.
	if diff := bounds.Dx() - tile.Dx(); diff > 40 || diff < 0 {
		t.Errorf("crop width = %d, tile width = %d", bounds.Dx(), tile.Dx())
	}
}

// opaqueBounds is the rectangle of fully opaque pixels: the tile without its soft
// edge.
func opaqueBounds(img image.Image) image.Rectangle {
	b := img.Bounds()
	minX, minY, maxX, maxY := b.Dx(), b.Dy(), 0, 0
	for y := b.Min.Y; y < b.Max.Y; y++ {
		for x := b.Min.X; x < b.Max.X; x++ {
			if _, _, _, a := img.At(x, y).RGBA(); a >= 0xFFFF {
				if x < minX {
					minX = x
				}
				if y < minY {
					minY = y
				}
				if x > maxX {
					maxX = x
				}
				if y > maxY {
					maxY = y
				}
			}
		}
	}
	if minX > maxX || minY > maxY {
		return b
	}
	return image.Rect(minX, minY, maxX+1, maxY+1)
}

// TestEveryLadderSizeRenders pins that each advertised size produces a square of
// exactly that size, with the tile filling it and transparent corners.
//
// The probes are taken a tenth of the way in from each edge: the crop keeps the
// tile's soft edge, so the outermost pixel is antialiased by design, and the corner
// band is cut by the tile's rounding.
func TestEveryLadderSizeRenders(t *testing.T) {
	for _, size := range IconLadder {
		img := MarkAt(size)
		if img == nil {
			t.Fatalf("%d: no image", size)
		}
		b := img.Bounds()
		if b.Dx() != size || b.Dy() != size {
			t.Fatalf("%d: rendered %dx%d", size, b.Dx(), b.Dy())
		}
		inset := size / 10
		if inset < 2 {
			inset = 2
		}
		for _, probe := range [][2]int{
			{size / 2, inset}, {size / 2, size - 1 - inset},
			{inset, size / 2}, {size - 1 - inset, size / 2},
		} {
			_, _, _, a := img.At(probe[0], probe[1]).RGBA()
			if a < 0x8000 {
				t.Errorf("%d: edge pixel %v alpha = %d, want the tile to reach it", size, probe, a)
			}
		}
		// The corners are cut by the tile's rounding, so they must not be opaque.
		if _, _, _, a := img.At(0, 0).RGBA(); a > 0x8000 {
			t.Errorf("%d: corner alpha = %d, want a rounded tile", size, a)
		}
	}
}

// TestMarkAtIsCached pins that two calls for one size return the same bitmap: the
// rail, the About page and the tray ask for these sizes every frame.
func TestMarkAtIsCached(t *testing.T) {
	first := MarkAt(64)
	second := MarkAt(64)
	if first == nil || second == nil {
		t.Fatal("no image")
	}
	if first != second {
		t.Error("the second call re-rendered instead of reusing the cache")
	}
	// Mark is the uncorrected render and deliberately bypasses the cache, so the
	// corrections can be compared against it.
	if Mark(64) == first {
		t.Error("Mark handed out the corrected bitmap")
	}
}

// TestSmallSizesKeepTheirDetail pins that the smallest entries are not a flat
// blur: a 32 px icon must still contain both a bright and a dark pixel, which is
// what makes the illustration readable in a taskbar.
func TestSmallSizesKeepTheirDetail(t *testing.T) {
	for _, size := range []int{24, 32, 48} {
		img := MarkAt(size)
		if img == nil {
			t.Fatalf("%d: no image", size)
		}
		var minL, maxL uint8 = 255, 0
		for y := 0; y < size; y++ {
			for x := 0; x < size; x++ {
				c := color.NRGBAModel.Convert(img.At(x, y)).(color.NRGBA)
				if c.A < 200 {
					continue
				}
				l := uint8((uint16(c.R)*299 + uint16(c.G)*587 + uint16(c.B)*114) / 1000)
				if l < minL {
					minL = l
				}
				if l > maxL {
					maxL = l
				}
			}
		}
		if maxL-minL < 40 {
			t.Errorf("%d: luminance range is %d..%d, the illustration collapsed into a blur", size, minL, maxL)
		}
	}
}

// TestPNGEncodesEverySize pins the byte path the tray uses.
func TestPNGEncodesEverySize(t *testing.T) {
	for _, size := range []int{16, 32, 256} {
		data := PNG(size)
		if len(data) == 0 {
			t.Fatalf("%d: no PNG bytes", size)
		}
		img, err := png.Decode(bytes.NewReader(data))
		if err != nil {
			t.Fatalf("%d: decode PNG: %v", size, err)
		}
		if img.Bounds().Dx() != size || img.Bounds().Dy() != size {
			t.Fatalf("%d: PNG is %v", size, img.Bounds())
		}
	}
}

// TestRoundMaskCutsTheCorners pins the mask used for the smallest sizes.
func TestRoundMaskCutsTheCorners(t *testing.T) {
	img := image.NewNRGBA(image.Rect(0, 0, 32, 32))
	for y := 0; y < 32; y++ {
		for x := 0; x < 32; x++ {
			img.SetNRGBA(x, y, color.NRGBA{R: 10, G: 20, B: 200, A: 255})
		}
	}
	RoundMask(img, TileRadius)
	if _, _, _, a := img.At(0, 0).RGBA(); a != 0 {
		t.Error("corner survived the mask")
	}
	if _, _, _, a := img.At(16, 16).RGBA(); a != 0xFFFF {
		t.Error("the mask cut the centre")
	}
}

// TestResourcesMatchTheLadder pins that the checked-in files are the ones this
// build renders: a stale resources/icon.png is what the packager would embed.
func TestResourcesMatchTheLadder(t *testing.T) {
	path := "../../resources/icon.png"
	if _, err := os.Stat(path); err != nil {
		t.Skip("resources/icon.png is generated by cmd/imageflow-icon")
	}
	file, err := os.Open(path)
	if err != nil {
		t.Fatal(err)
	}
	defer file.Close()
	img, err := png.Decode(file)
	if err != nil {
		t.Fatal(err)
	}
	if img.Bounds().Dx() != 1024 || img.Bounds().Dy() != 1024 {
		t.Fatalf("packaged icon is %v, want 1024×1024", img.Bounds())
	}
	// The packager's icon is the tile at 1024, so a point a tenth of the way in from
	// each edge carries the tile and the corner is cut.
	if _, _, _, a := img.At(512, 102).RGBA(); a < 0x8000 {
		t.Error("packaged icon's top edge is transparent: it was rendered from the full canvas")
	}
	if _, _, _, a := img.At(0, 0).RGBA(); a > 0x8000 {
		t.Error("packaged icon's corner is opaque: it was not rounded")
	}
}
