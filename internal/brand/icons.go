package brand

// icons.go renders the icon ladder from the embedded artwork.
//
// Every icon size the app uses comes from here: the window's mark, the tray
// glyph, the About badge, the packager's resources/icon.png and the .ico ladder.
// One renderer means one shape, and a test can compare two sizes of the same
// render path instead of two pictures that merely look alike.
//
// The artwork is downscaled with a two-stage filter (bilinear prefilter, then
// Catmull-Rom). A plain nearest-neighbour downscale of a 1064 px tile to 32 px
// drops three edges in four and turns the rounded corners into a staircase; the
// two-stage path keeps them round, which is the point of shipping one artwork
// instead of a set of hand-drawn sizes.

import (
	"bytes"
	"image"
	"image/color"
	stddraw "image/draw"
	"image/png"
	"math"
	"sync"

	xdraw "golang.org/x/image/draw"
)

// iconCache keeps one rendered square per size: the rail, the title badge, the
// About page and the tray all ask for the same handful of sizes every frame.
var (
	iconMu    sync.Mutex
	iconCache = map[int]*image.NRGBA{}
)

// Mark renders the artwork's tile at the requested square size on a transparent
// canvas: the plain downscale, with no correction applied and no caching. It is the
// raw render, kept separate so the size-dependent corrections in MarkAt can be
// tested against it.
func Mark(size int) image.Image {
	if size < 1 {
		size = 1
	}
	src, err := Logo()
	if err != nil {
		return nil
	}
	return RenderSquare(src, MarkBounds(), size)
}

func cachedIcon(size int) *image.NRGBA {
	iconMu.Lock()
	defer iconMu.Unlock()
	return iconCache[size]
}

func storeIcon(size int, img *image.NRGBA) {
	iconMu.Lock()
	iconCache[size] = img
	iconMu.Unlock()
}

// MarkAt is the mark a caller should draw. At the sizes an operating system uses
// for a taskbar or a tray the artwork's fine detail does not survive the
// downscale, so the small entries get two corrections and nothing else: the tile's
// own corner rounding is re-applied, a light contrast lift separates the white
// basket from the blue tile, and a mask restores the illustration's edges.
//
// Every correction is bounded by size, and each one is skipped above the size where
// it stops helping: a 256 px icon is the artwork itself and needs none of them.
// The finished bitmap is cached, because the corrections are per-frame work on a
// picture that never changes.
func MarkAt(size int) image.Image {
	if size < 1 {
		size = 1
	}
	if img := cachedIcon(size); img != nil {
		return img
	}
	img := Mark(size)
	if img == nil {
		return nil
	}
	nrgba, ok := img.(*image.NRGBA)
	if !ok || size > 64 {
		return img
	}
	if size <= contrastBelow {
		nrgba = Contrast(nrgba, contrastAmount)
	}
	if size >= sharpenFloor && size <= sharpenBelow {
		nrgba = Sharpen(nrgba, sharpenAmount)
	}
	nrgba = RoundMask(nrgba, TileRadius)
	storeIcon(size, nrgba)
	return nrgba
}

// RenderSquare crops src to box and scales it to size×size.
//
// Cropping before scaling is what removes the artwork's margin; scaling the whole
// canvas and then cropping would resample a larger picture for a smaller result
// and soften the tile's crisp edge.
func RenderSquare(src image.Image, box image.Rectangle, size int) *image.NRGBA {
	if src == nil || size < 1 {
		return nil
	}
	bounds := src.Bounds()
	clipped := box.Intersect(bounds)
	if clipped.Empty() {
		clipped = bounds
	}
	// Work on an NRGBA copy of the crop: the rest of the app speaks NRGBA, and the
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
// Measured on the source: the tile's straight edge begins 244 px below its top at
// the left side, and the crop is 1066 px wide, so the corners are re-drawn at
// 244/1066 of the icon's side. Re-drawing rather than letting the downscale soften
// them is what keeps a 16 px entry from looking like a blob with clipped corners.
const TileRadius = 244.0 / 1066.0

// PNG encodes one rendered size as PNG bytes, for a caller that hands the mark to
// the platform rather than drawing it.
func PNG(size int) []byte { return EncodePNG(MarkAt(size)) }

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
