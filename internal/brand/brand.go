// Package brand holds the application's identity: the logo artwork, the icon
// ladder rendered from it, and the project's public URLs.
//
// It is a package of its own because more than the UI needs the mark: the icon
// generator writes resources/ from the same bytes the window draws, so the
// installer, the taskbar, the tray and the in-app logo can never drift into three
// different pictures.
package brand

import (
	"bytes"
	_ "embed"
	"image"
	"image/color"
	"sync"
)

// The artwork is embedded rather than read from disk: the binary has to carry its
// own icon, and a missing file at runtime must not leave the window's mark blank.
//
//go:embed assets/logo.png
var logoSrc []byte

// LogoSource is the original artwork, byte for byte. A generator or a test can
// decode exactly what the binary carries.
func LogoSource() []byte { return logoSrc }

// The artwork is a 1254×1254 canvas: a rounded square tile with a soft shadow
// around it. Measured on the source's fully opaque pixels, the tile's solid area
// runs from (91,117) to (1148,1163) — 1058×1047 — and its soft edge fades out a few
// pixels past that. The shadow's faint tail reaches the canvas edge, which is why
// an alpha-based bounding box of the whole file is the whole canvas.
//
// An operating system draws the mark between 16 and 1024 pixels, so the icon ladder
// is rendered from a square crop of the tile rather than from the whole canvas: the
// full canvas carries roughly 7% of margin per side, which at 16 px costs two of the
// picture's twelve usable pixels and makes the mark read small.
//
// The crop is square and centred on the solid tile (the artwork is 1058 wide and
// 1047 tall), rather than on the canvas: the shadow is heavier below the tile, so
// the canvas centre sits about 13 px above the tile's centre. It takes the
// larger span, so no edge of the tile is clipped, and it keeps the tile's soft
// edge, which is what stops the icon from looking cut out.
//
// It is computed rather than written as a literal so the square and the centre stay
// one decision: the side is the tile's span plus a small margin for the soft edge.
var markBounds = func() image.Rectangle {
	const tileX0, tileY0, tileX1, tileY1 = 87, 113, 1153, 1168
	const side = 1066
	cx := (tileX0 + tileX1) / 2
	cy := (tileY0 + tileY1) / 2
	x0 := cx - side/2
	y0 := cy - side/2
	// Keep the crop inside the artwork, whatever the numbers above say.
	canvas := image.Rect(0, 0, 1254, 1254)
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

// Logo decodes the embedded artwork once and returns it.
func Logo() (image.Image, error) {
	decodedOnce.Do(func() {
		img, _, err := image.Decode(bytes.NewReader(logoSrc))
		if err != nil {
			decodedErr = err
			return
		}
		decoded = img
	})
	return decoded, decodedErr
}

// LogoBounds is the artwork's full canvas.
func LogoBounds() image.Rectangle {
	img, err := Logo()
	if err != nil {
		return image.Rect(0, 0, 1024, 1024)
	}
	return img.Bounds()
}

// MarkBounds is the square the tile occupies inside the artwork. Every icon is
// rendered from this rectangle.
func MarkBounds() image.Rectangle { return markBounds }

// RepoURL is the project's public GitHub repository. The About page opens it and
// the release feed publishes to it.
const RepoURL = "https://github.com/lllll081926i/Imageflow"

// RepoLabel is how that repository reads in the UI.
const RepoLabel = "lllll081926i/Imageflow"

// Accent is the mark's dominant colour, used where one tint has to stand in for
// the artwork (a status accent, a plate behind the mark).
var Accent = color.NRGBA{R: 0x7A, G: 0x7A, B: 0xF2, A: 255}

// IconLadder is every PNG size the icon bundle carries, in ascending order.
//
// It is the ladder a desktop and a web project actually consume: the small
// favicon sizes, the taskbar and dock sizes, and the 1024 tile the packager
// embeds.
var IconLadder = []int{16, 24, 32, 48, 64, 96, 128, 144, 152, 167, 180, 192, 256, 384, 512, 1024}

// ICOIconSizes are the entries of the multi-resolution icon.ico; FaviconSizes
// those of favicon.ico. A Windows shortcut and a browser tab want different
// ladders, and an ICO entry cannot exceed 256.
var (
	ICOIconSizes = []int{16, 24, 32, 48, 64, 128, 256}
	FaviconSizes = []int{16, 32, 48}
)
