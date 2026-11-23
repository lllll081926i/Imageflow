// Package iconkit renders the application's icon family from one artwork.
//
// It exists for the build-time icon generator, not for the app: the window, the
// tray and the title bar draw pre-rendered sizes that internal/brand embeds, so the
// running binary carries neither the source artwork nor a resampler. Everything in
// this package runs at build time, in cmd/imageflow-icon.
//
// One renderer means one shape, and a test can compare two sizes of the same render
// path instead of two pictures that merely look alike.
package iconkit

import (
	"bytes"
	"image"
	_ "image/png" // the artwork is a PNG
	"sync"
)

// The artwork is a 1132×1132 canvas: a rounded square tile with a transparent
// surround. Measured on the source's fully opaque pixels, the tile's solid area
// runs from (6,7) to (1127,1123) — 1122×1117 — and its edge fades out over about
// three pixels past that. The surround carries scattered pixels of near-zero
// alpha, which is why an alpha-based bounding box of the whole file is the whole
// canvas.
//
// An operating system draws the mark between 16 and 1024 pixels, so the icon
// ladder is rendered from a square crop of the tile rather than from the whole
// canvas: the full canvas carries margin per side, which at 16 px costs two of the
// picture's twelve usable pixels and makes the mark read small.
//
// The crop is square and centred on the solid tile (the artwork is 1122 wide and
// 1117 tall), rather than on the canvas. It takes the larger span, so no edge of
// the tile is clipped, and it keeps the tile's soft edge, which is what stops the
// icon from looking cut out.
//
// It is computed rather than written as a literal so the square and the centre
// stay one decision: the side is the tile's span plus a small margin for the soft
// edge.
var markBounds = func() image.Rectangle {
	const tileX0, tileY0, tileX1, tileY1 = 6, 7, 1127, 1123
	const side = 1130
	cx := (tileX0 + tileX1) / 2
	cy := (tileY0 + tileY1) / 2
	x0 := cx - side/2
	y0 := cy - side/2
	// Keep the crop inside the artwork, whatever the numbers above say.
	canvas := image.Rect(0, 0, 1132, 1132)
	if x0 < canvas.Min.X {
		x0 = canvas.Min.X
	}
	if y0 < canvas.Min.Y {
		y0 = canvas.Min.Y
	}
	if x0+side > canvas.Max.X {
		x0 = canvas.Max.X - side
	}
	if y0+side > canvas.Max.Y {
		y0 = canvas.Max.Y - side
	}
	return image.Rect(x0, y0, x0+side, y0+side)
}()

var (
	decodedOnce sync.Once
	decoded     image.Image
	decodedErr  error
)

// Decode reads one artwork. The generator renders every size of a run from the
// same decoded picture, so the result is kept for the process.
func Decode(data []byte) (image.Image, error) {
	decodedOnce.Do(func() {
		img, _, err := image.Decode(bytes.NewReader(data))
		if err != nil {
			decodedErr = err
			return
		}
		decoded = img
	})
	return decoded, decodedErr
}

// MarkBounds is the square the tile occupies inside the artwork. Every icon is
// rendered from this rectangle.
func MarkBounds() image.Rectangle { return markBounds }

// Render draws the artwork's tile at one square size on a transparent canvas,
// uncorrected. It is the raw render, kept separate so the size-dependent
// corrections in RenderCorrected can be tested against it.
func Render(src image.Image, size int) *image.NRGBA {
	if size < 1 {
		size = 1
	}
	if src == nil {
		return nil
	}
	return RenderSquare(src, markBounds, size)
}

// RenderCorrected draws the mark a caller should publish. At the sizes an
// operating system uses for a taskbar or a tray the artwork's fine detail does not
// survive the downscale, so the small entries get two corrections and nothing
// else: a light contrast lift separates the face and the frame from the pale tile,
// an unsharp mask restores the line work, and a rounded mask restores the tile's
// own corner.
//
// Every correction is bounded by size, and each one is skipped above the size where
// it stops helping: a 256 px icon is the artwork itself and needs none of them.
func RenderCorrected(src image.Image, size int) *image.NRGBA {
	if size < 1 {
		size = 1
	}
	if src == nil {
		return nil
	}
	img := Render(src, size)
	if img == nil || size > 64 {
		return img
	}
	if size <= contrastBelow {
		img = Contrast(img, contrastAmount)
	}
	if size >= sharpenFloor && size <= sharpenBelow {
		img = Sharpen(img, sharpenAmount)
	}
	return RoundMask(img, TileRadius)
}

// IconLadder is every PNG size the icon bundle carries, in ascending order.
//
// It is the ladder a desktop and a web project actually consume: the small favicon
// sizes, the taskbar and dock sizes, and the 1024 tile the packager embeds.
var IconLadder = []int{16, 24, 32, 48, 64, 96, 128, 144, 152, 167, 180, 192, 256, 384, 512, 1024}

// MarkSizes is every square internal/brand embeds for the shell to draw: the
// title-bar badge and tray glyph (32), the landing page's header (36), the About
// row's (40), and a little headroom either side.
//
// It is the only list: cmd/imageflow-marks writes one file per size, and
// internal/brand's embed switch carries exactly these. A test holds the two
// together, so a size cannot exist as a declaration without a file.
var MarkSizes = []int{16, 24, 32, 36, 40, 48, 64}

// ICOIconSizes are the entries of the multi-resolution icon.ico; FaviconSizes
// those of favicon.ico. A Windows shortcut and a browser tab want different
// ladders, and an ICO entry cannot exceed 256.
var (
	ICOIconSizes = []int{16, 24, 32, 48, 64, 128, 256}
	FaviconSizes = []int{16, 32, 48}
)
