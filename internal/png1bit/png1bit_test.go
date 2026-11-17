package png1bit

import (
	"bytes"
	"encoding/binary"
	"image"
	"image/color"
	"image/png"
	"testing"
)

// header reads the IHDR fields back out of an encoded file.
func header(t *testing.T, raw []byte) (width, height, depth, colorType int) {
	t.Helper()
	if len(raw) < 33 {
		t.Fatalf("file too short: %d", len(raw))
	}
	if !bytes.HasPrefix(raw, []byte("\x89PNG\r\n\x1a\n")) {
		t.Fatal("missing PNG signature")
	}
	if string(raw[12:16]) != "IHDR" {
		t.Fatalf("first chunk = %q", raw[12:16])
	}
	width = int(binary.BigEndian.Uint32(raw[16:20]))
	height = int(binary.BigEndian.Uint32(raw[20:24]))
	depth = int(raw[24])
	colorType = int(raw[25])
	return
}

func TestEncodedGrayFileIsOneBitDeep(t *testing.T) {
	m := NewMask(9, 4)
	m.Set(0, 0, true)
	m.Set(8, 3, true)
	raw, err := Encode(m, Options{ColorType: Gray})
	if err != nil {
		t.Fatal(err)
	}
	w, h, depth, ct := header(t, raw)
	if w != 9 || h != 4 {
		t.Fatalf("size = %dx%d", w, h)
	}
	if depth != 1 {
		t.Fatalf("bit depth = %d, want 1", depth)
	}
	if ct != 0 {
		t.Fatalf("color type = %d, want 0 (greyscale)", ct)
	}
}

func TestEncodedPaletteFileIsOneBitDeep(t *testing.T) {
	m := NewMask(8, 2)
	m.Set(3, 1, true)
	raw, err := Encode(m, Options{
		ColorType: Palette,
		Black:     color.NRGBA{A: 255},
		White:     color.NRGBA{R: 255, G: 255, B: 255, A: 255},
	})
	if err != nil {
		t.Fatal(err)
	}
	_, _, depth, ct := header(t, raw)
	if depth != 1 || ct != 3 {
		t.Fatalf("depth=%d colorType=%d, want 1/3", depth, ct)
	}
	if !bytes.Contains(raw, []byte("PLTE")) {
		t.Fatal("indexed output must carry a PLTE chunk")
	}
}

// TestDecodedPixelsAreOnlyTheTwoColours is the acceptance check the PRD calls
// for: the file reopens and every pixel is one of the two values, with no
// intermediate grey.
func TestDecodedPixelsAreOnlyTheTwoColours(t *testing.T) {
	m := NewMask(13, 7)
	for y := 0; y < 7; y++ {
		for x := 0; x < 13; x++ {
			m.Set(x, y, (x+y)%3 == 0)
		}
	}
	for _, ct := range []ColorType{Gray, Palette} {
		raw, err := Encode(m, Options{
			ColorType: ct,
			Black:     color.NRGBA{A: 255},
			White:     color.NRGBA{R: 255, G: 255, B: 255, A: 255},
		})
		if err != nil {
			t.Fatalf("type %d: %v", ct, err)
		}
		img, err := png.Decode(bytes.NewReader(raw))
		if err != nil {
			t.Fatalf("type %d decode: %v", ct, err)
		}
		if img.Bounds().Dx() != 13 || img.Bounds().Dy() != 7 {
			t.Fatalf("type %d bounds = %v", ct, img.Bounds())
		}
		for y := 0; y < 7; y++ {
			for x := 0; x < 13; x++ {
				r, g, b, a := img.At(x, y).RGBA()
				if a != 0xFFFF {
					t.Fatalf("type %d pixel %d,%d alpha = %d", ct, x, y, a)
				}
				if r != g || g != b || (r != 0 && r != 0xFFFF) {
					t.Fatalf("type %d pixel %d,%d = %d,%d,%d", ct, x, y, r, g, b)
				}
				wantWhite := m.At(x, y)
				if gotWhite := r == 0xFFFF; gotWhite != wantWhite {
					t.Fatalf("type %d pixel %d,%d white=%v want %v", ct, x, y, gotWhite, wantWhite)
				}
			}
		}
	}
}

func TestFromImageThresholdsAndInverts(t *testing.T) {
	img := image.NewNRGBA(image.Rect(0, 0, 4, 1))
	img.SetNRGBA(0, 0, color.NRGBA{A: 255})                         // black
	img.SetNRGBA(1, 0, color.NRGBA{R: 255, G: 255, B: 255, A: 255}) // white
	img.SetNRGBA(2, 0, color.NRGBA{R: 200, G: 200, B: 200, A: 255}) // light grey
	img.SetNRGBA(3, 0, color.NRGBA{R: 0, G: 0, B: 0, A: 0})         // transparent
	m := FromImage(img, 128, false, color.NRGBA{R: 255, G: 255, B: 255, A: 255})
	if m.At(0, 0) {
		t.Error("black pixel became white")
	}
	if !m.At(1, 0) || !m.At(2, 0) {
		t.Error("light pixels must exceed the threshold")
	}
	if !m.At(3, 0) {
		t.Error("a transparent pixel must composite onto the background, not turn black")
	}
	inv := FromImage(img, 128, true, color.NRGBA{A: 255})
	if inv.At(1, 0) {
		t.Error("invert did not flip the white pixel")
	}
}

func TestOpaquePaletteIsRequired(t *testing.T) {
	m := NewMask(2, 2)
	if _, err := Encode(m, Options{
		ColorType: Palette,
		Black:     color.NRGBA{A: 128},
		White:     color.NRGBA{R: 255, G: 255, B: 255, A: 255},
	}); err == nil {
		t.Fatal("a translucent palette entry must be rejected")
	}
}

func TestRawClearsPaddingBits(t *testing.T) {
	m := NewMask(3, 2)
	m.Pix[0] = 0xFF
	raw := m.Raw()
	if raw[0] != 0xE0 {
		t.Fatalf("raw[0] = %#x, want 0xE0", raw[0])
	}
}

func TestEmptyMaskIsRejected(t *testing.T) {
	if _, err := Encode(&Mask{}, Options{ColorType: Gray}); err == nil {
		t.Fatal("a zero-sized mask must be rejected")
	}
}
