// Package iconkit renders the application's icon family from one artwork.
//
// It exists for the build-time icon generator, not for the app: the window,
// the tray and the title bar draw three pre-rendered sizes that
// internal/brand embeds, so the running binary carries neither the 1.5 MB
// source artwork nor a resampler. Everything in this package runs at build
// time, in cmd/imageflow-icon.
//
// The artwork is downscaled with a two-stage filter (bilinear prefilter, then
// Catmull-Rom). A plain nearest-neighbour downscale of a 1130 px tile to 32 px
// drops three edges in four and turns the rounded corners into a staircase; the
// two-stage path keeps them round, which is the point of shipping one artwork
// instead of a set of hand-drawn sizes.
package iconkit

import (
	"bytes"
	"image"
	"image/color"
	stddraw "image/draw"
	"image/png"
	"math"

	xdraw "golang.org/x/image/draw"
)

// RenderSquare crops src to box and scales it to size×size.
//
// Cropping before scaling is what removes the artwork's margin; scaling the
// whole canvas and then cropping would resample a larger picture for a
// smaller result and soften the tile's crisp edge.
func RenderSquare(src image.Image, box image.Rectangle, size int) *image.NRGBA {
	if src == nil || size < 1 {
		return nil
	}
	bounds := src.Bounds()
	clipped := box.Intersect(bounds)
	if clipped.Empty() {
		clipped = bounds
	}
	// Work on an NRGBA copy of the crop: every later step speaks NRGBA, and the
	// artwork's own image type is not something a caller should have to know.
	cropped := image.NewNRGBA(image.Rect(0, 0, clipped.Dx(), clipped.Dy()))
	stddraw.Draw(cropped, cropped.Bounds(), src, clipped.Min, stddraw.Src)

	dst := image.NewNRGBA(image.Rect(0, 0, size, size))
	if clipped.Dx() == size && clipped.Dy() == size {
		stddraw.Draw(dst, dst.Bounds(), cropped, image.Point{}, stddraw.Src)
		return dst
	}
	prefiltered := image.NewNRGBA(cropped.Bounds())
	xdraw.BiLinear.Scale(prefiltered, prefiltered.Bounds(), cropped, cropped.Bounds(), stddraw.Src, nil)
	xdraw.CatmullRom.Scale(dst, dst.Bounds(), prefiltered, prefiltered.Bounds(), stddraw.Src, nil)
	return dst
}

// RoundMask clears the pixels outside a rounded square.
//
// radius is a fraction of the side: TileRadius for the artwork's own corners.
func RoundMask(img *image.NRGBA, radius float64) *image.NRGBA {
	if img == nil || radius <= 0 {
		return img
	}
	b := img.Bounds()
	w, h := b.Dx(), b.Dy()
	r := math.Min(float64(w), float64(h)) * clamp01(radius)
	if r < 0.5 {
		return img
	}
	for y := 0; y < h; y++ {
		for x := 0; x < w; x++ {
			if !insideRounded(x, y, w, h, r) {
				img.SetNRGBA(x, y, color.NRGBA{})
			}
		}
	}
	return img
}

// insideRounded reports whether (x,y) lies inside a w×h rectangle rounded by r.
func insideRounded(x, y, w, h int, r float64) bool {
	cx, cy := float64(x)+0.5, float64(y)+0.5
	lx, ly := float64(w)-r, float64(h)-r
	var ox, oy float64
	switch {
	case cx < r && cy < r:
		ox, oy = cx-r, cy-r
	case cx > lx && cy < r:
		ox, oy = cx-lx, cy-r
	case cx < r && cy > ly:
		ox, oy = cx-r, cy-ly
	case cx > lx && cy > ly:
		ox, oy = cx-lx, cy-ly
	default:
		return true
	}
	return ox*ox+oy*oy <= r*r
}

func clamp01(v float64) float64 {
	if v < 0 {
		return 0
	}
	if v > 1 {
		return 1
	}
	return v
}

// TileRadius is the artwork's own corner radius as a fraction of the crop's side.
//
// Measured on the source: a least-squares circle through the tile's top-left arc,
// taken at the alpha midpoint (the edge a viewer perceives), has a radius of 307 px
// over a 1130 px crop. The mask that matches it is a 305 px circle: the arc's own
// radius leaves a pixel of the antialias halo outside the mask, and anything rounder
// than 305 starts eating the tile at 48 and 64 px. Re-drawing the corner rather than
// letting the downscale soften it is what keeps a 16 px entry from looking like a
// blob with clipped corners.
const TileRadius = 305.0 / 1130.0

// EncodePNG encodes any image as PNG bytes, or returns nil.
func EncodePNG(img image.Image) []byte {
	if img == nil {
		return nil
	}
	var buf bytes.Buffer
	if err := png.Encode(&buf, img); err != nil {
		return nil
	}
	return buf.Bytes()
}
