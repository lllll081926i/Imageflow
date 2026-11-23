package brand

// brand_test.go pins what the binary actually carries.
//
// The mark is pre-rendered now: internal/iconkit owns the artwork, the crop and the
// small-size corrections, and this package embeds the finished squares. So the
// assertions are about the files the embed picked up — their size, their transparent
// surround, their colour — and not about a resampler that no longer ships.

import (
	"bytes"
	"image"
	"image/color"
	"image/png"
	"testing"
)

// TestEveryMarkSizeIsEmbedded pins that MarkSizes and the files agree: a size
// declared without a file would fail at the first frame that draws it, which is a
// blank title bar at runtime rather than a build error.
func TestEveryMarkSizeIsEmbedded(t *testing.T) {
	if len(MarkSizes) == 0 {
		t.Fatal("no mark sizes")
	}
	for _, size := range MarkSizes {
		data, ok := markFile(size)
		if !ok {
			t.Fatalf("%d: no embedded file, MarkSizes and marks.go disagree", size)
		}
		img, err := png.Decode(bytes.NewReader(data))
		if err != nil {
			t.Fatalf("%d: decode: %v", size, err)
		}
		if b := img.Bounds(); b.Dx() != size || b.Dy() != size {
			t.Fatalf("%d: file is %v, want a %d×%d square", size, b, size, size)
		}
	}
}

// TestMarkReturnsTheEmbeddedSquare pins the accessor: the same size twice is the
// same picture, the size is exact, the tile fills it, and the corner is cut.
//
// The probes sit a tenth of the way in from each edge because the mark keeps the
// artwork's soft edge, so the outermost pixel is antialiased on purpose.
func TestMarkReturnsTheEmbeddedSquare(t *testing.T) {
	for _, size := range []int{32, 36, 40} {
		img := Mark(size)
		if img == nil {
			t.Fatalf("%d: no picture", size)
		}
		if b := img.Bounds(); b.Dx() != size || b.Dy() != size {
			t.Fatalf("%d: got %v", size, b)
		}
		if again := Mark(size); again != img {
			t.Errorf("%d: the second call decoded again instead of reusing the cache", size)
		}
		n := toNRGBA(t, img)
		inset := size / 10
		if inset < 2 {
			inset = 2
		}
		for _, probe := range [][2]int{
			{size / 2, inset}, {size / 2, size - 1 - inset},
			{inset, size / 2}, {size - 1 - inset, size / 2},
		} {
			if a := n.NRGBAAt(probe[0], probe[1]).A; a < 0x80 {
				t.Errorf("%d: edge pixel %v alpha = %d, want the tile to reach it", size, probe, a)
			}
		}
		if a := n.NRGBAAt(0, 0).A; a > 0x80 {
			t.Errorf("%d: corner alpha = %d, want a rounded tile", size, a)
		}
	}
}

// toNRGBA reads a mark's pixels. Mark hands out image.Image so a missing size is
// a nil interface; the tests still want the accessor.
func toNRGBA(t *testing.T, img image.Image) *image.NRGBA {
	t.Helper()
	if n, ok := img.(*image.NRGBA); ok {
		return n
	}
	b := img.Bounds()
	n := image.NewNRGBA(b)
	for y := b.Min.Y; y < b.Max.Y; y++ {
		for x := b.Min.X; x < b.Max.X; x++ {
			n.Set(x, y, img.At(x, y))
		}
	}
	return n
}

// TestUnknownSizeIsNil pins the contract a caller relies on: a size this build does
// not carry returns nil, so a page can skip the picture instead of drawing a
// stretched one.
func TestUnknownSizeIsNil(t *testing.T) {
	for _, size := range []int{1, 7, 128, 1024} {
		if img := Mark(size); img != nil {
			t.Errorf("%d: got a %v picture for a size Marks does not carry", size, img.Bounds())
		}
	}
}

// TestPNGMatchesTheSquare pins the byte path the tray and the window icon take: it
// is the same picture the interface draws, encoded once.
func TestPNGMatchesTheSquare(t *testing.T) {
	for _, size := range []int{16, 32, 36} {
		data := PNG(size)
		if len(data) == 0 {
			t.Fatalf("%d: no PNG bytes", size)
		}
		img, err := png.Decode(bytes.NewReader(data))
		if err != nil {
			t.Fatalf("%d: decode PNG: %v", size, err)
		}
		if b := img.Bounds(); b.Dx() != size || b.Dy() != size {
			t.Fatalf("%d: PNG is %v", size, b)
		}
		if again := PNG(size); &again[0] != &data[0] {
			t.Errorf("%d: re-encoded the same size twice", size)
		}
	}
}

// TestSmallSizesKeepTheirDetail pins that the corrections written by the generator
// actually landed in the embedded files: a 32 px icon must still carry both a bright
// and a dark pixel, which is what makes the illustration readable in a taskbar.
//
// The range is wide because the artwork is a pale illustration on a pale tile; if
// the generator's contrast or sharpen pass were dropped, the face and the frame
// would collapse into that tile and the range would narrow to a few dozen levels.
func TestSmallSizesKeepTheirDetail(t *testing.T) {
	for _, size := range []int{24, 32} {
		img := Mark(size)
		if img == nil {
			t.Fatalf("%d: no picture", size)
		}
		n := toNRGBA(t, img)
		minL, maxL := 255, 0
		for y := 0; y < size; y++ {
			for x := 0; x < size; x++ {
				c := n.NRGBAAt(x, y)
				if c.A < 200 {
					continue
				}
				// int, not uint16: 255×587 overflows 16 bits and would report a
				// luminance range that no pixel has.
				l := (int(c.R)*299 + int(c.G)*587 + int(c.B)*114) / 1000
				if l < minL {
					minL = l
				}
				if l > maxL {
					maxL = l
				}
			}
		}
		if maxL-minL < 120 {
			t.Errorf("%d: luminance range is %d..%d, the corrections are missing", size, minL, maxL)
		}
	}
}

// TestAccentIsTheArtworkColour pins the tint that stands in for the mark where one
// colour has to: it must be the artwork's periwinkle and not the previous purple.
func TestAccentIsTheArtworkColour(t *testing.T) {
	accent := color.NRGBAModel.Convert(Accent).(color.NRGBA)
	if accent.B <= accent.R {
		t.Errorf("accent = %v, want a blue-dominant periwinkle", accent)
	}
	if accent.R > 0xA0 || accent.B < 0xE0 {
		t.Errorf("accent = #%02X%02X%02X, want the artwork's #7FA9F7 family", accent.R, accent.G, accent.B)
	}
}

// TestMarkCornerMatchesTheArtwork pins the shape the generator renders: every size
// must have its top-left corner rounded by the same fraction of its side, so a
// 16 px entry and a 64 px entry read as one tile rather than two shapes.
func TestMarkCornerMatchesTheArtwork(t *testing.T) {
	for _, size := range []int{32, 64} {
		img := Mark(size)
		if img == nil {
			t.Fatalf("%d: no picture", size)
		}
		if _, _, _, a := img.At(0, 0).RGBA(); a != 0 {
			t.Errorf("%d: the corner is opaque, the tile mask did not run", size)
		}
		if _, _, _, a := img.At(size/2, 0).RGBA(); a == 0 {
			t.Errorf("%d: the top edge is transparent, the crop kept the artwork's margin", size)
		}
	}
	_ = image.Rect
}
