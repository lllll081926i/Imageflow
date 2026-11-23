// Package brand holds the application's identity: the mark the window draws, its
// colour, and the project's public URLs.
//
// The mark is pre-rendered, not resampled at runtime. internal/iconkit owns the
// artwork, the crop, the corner mask and the small-size corrections; this package
// embeds the finished squares that code produced, so the running binary carries
// no 1.5 MB source picture and no resampler. Regenerating them after a change to
// the artwork is `go run ./cmd/imageflow-marks`.
//
// The sizes below are the ones the shell actually draws, plus the neighbours a
// new page might ask for. A size that is not here has no picture: the generator
// must run first.
package brand

import (
	"bytes"
	"image"
	"image/color"
	"image/png"
	"sync"
)

// MarkSizes is every square this package embeds, in ascending order. It is
// deliberately short: 32 is the title-bar badge and the tray glyph, 36 the
// landing page's header, 40 the About row's, and the rest give a new page a
// little headroom without asking the app to resample anything.
var MarkSizes = []int{16, 24, 32, 36, 40, 48, 64}

// Accent is the mark's dominant colour, used where one tint has to stand in for
// the artwork (a status accent, a plate behind the mark).
//
// Measured from the artwork: the tile's flat backing behind the illustration is
// its modal saturated family, and that family's mean is 0x7F,0xA9,0xF7.
var Accent = color.NRGBA{R: 0x7F, G: 0xA9, B: 0xF7, A: 255}

// RepoURL is the project's public GitHub repository. The About page opens it and
// the release feed publishes to it.
const RepoURL = "https://github.com/lllll081926i/Imageflow"

// RepoLabel is how that repository reads in the UI.
const RepoLabel = "lllll081926i/Imageflow"

var (
	markMu  sync.Mutex
	markMap map[int]*image.NRGBA
	pngMap  map[int][]byte
)

// Mark returns the embedded mark at one size, or nil for a size this build does
// not carry. The picture is decoded once and shared: the rail, the title badge,
// the About row and the tray ask for their sizes every frame.
//
// The return type is image.Image, not *image.NRGBA, on purpose: a nil
// *image.NRGBA stored in an interface is a non-nil interface, and every caller
// that checks "is there a picture?" with `== nil` before calling Bounds would
// then panic instead of skipping the mark.
func Mark(size int) image.Image {
	markMu.Lock()
	defer markMu.Unlock()
	if img, ok := markMap[size]; ok {
		return img
	}
	img, err := decodeMark(size)
	if err != nil {
		return nil
	}
	if markMap == nil {
		markMap = map[int]*image.NRGBA{}
	}
	markMap[size] = img
	return img
}

// PNG returns the embedded mark at one size as PNG bytes, for the tray and the
// platform, which take a picture rather than an image.Image.
func PNG(size int) []byte {
	markMu.Lock()
	if data, ok := pngMap[size]; ok {
		markMu.Unlock()
		return data
	}
	markMu.Unlock()

	img := Mark(size)
	if img == nil {
		return nil
	}
	var buf bytes.Buffer
	if err := png.Encode(&buf, img); err != nil {
		return nil
	}
	data := buf.Bytes()

	markMu.Lock()
	if pngMap == nil {
		pngMap = map[int][]byte{}
	}
	pngMap[size] = data
	markMu.Unlock()
	return data
}

// decodeMark reads the embedded square for one size.
func decodeMark(size int) (*image.NRGBA, error) {
	data, ok := markFile(size)
	if !ok {
		return nil, errNoMark(size)
	}
	img, _, err := image.Decode(bytes.NewReader(data))
	if err != nil {
		return nil, err
	}
	nrgba, ok := img.(*image.NRGBA)
	if !ok {
		// A PNG with an alpha channel decodes to NRGBA; the copy keeps the
		// signature honest if that ever changes.
		b := img.Bounds()
		nrgba = image.NewNRGBA(b)
		for y := b.Min.Y; y < b.Max.Y; y++ {
			for x := b.Min.X; x < b.Max.X; x++ {
				nrgba.Set(x, y, img.At(x, y))
			}
		}
	}
	return nrgba, nil
}
