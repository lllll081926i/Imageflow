package iconkit

// sharpen.go sharpens the smallest ladder entries.
//
// The artwork is a detailed illustration: a girl holding a photo frame, a cat-ear
// silhouette and a wrench speech bubble. At 32 px and below the two-stage downscale
// keeps the tile's corners round but flattens the illustration into a soft blue
// blob, which is exactly the size a taskbar and a tray use. A light unsharp mask on
// those sizes restores the separations, so the icon still reads as "a picture"
// instead of "a blue square".

import (
	"image"
	"image/color"
)

// sharpenBelow is the size at and below which the mask is applied. Above it the
// artwork's own detail survives the downscale and a mask would only add halos.
const sharpenBelow = 48

// sharpenFloor is the smallest size the mask still helps. The artwork's line work is
// dense enough that even 16 px keeps an edge if the mask is applied, and without it
// the eyes and the frame's outline merge into the tile.
const sharpenFloor = 16

// contrastBelow is the size at and below which the contrast lift is applied. Above
// it the artwork's own contrast is enough and a lift would only deepen the tile's
// blue.
const contrastBelow = 48

// contrastAmount is the lift's strength, as the fraction of the distance from mid
// grey that each channel gains. 0.45 was chosen by comparing 16, 24 and 32 px side
// by side on a light and a dark surface: the artwork is a pale illustration on a
// pale blue tile, so without the lift the face and the frame land within 15% of the
// tile and the icon reads as a single blue smudge; at 0.7 the tile's own gradient
// bands into visible steps.
const contrastAmount = 0.45

// sharpenAmount is the mask's strength. 0.4 kept the artwork's floodlit top-left
// corner, but the new artwork's highlights are already pale, so at 0.4 the frame's
// rim and the eyes blur into the tile at 16 px; 0.5 restores both edges and the tile
// stays clean. At 0.8 the blush and the eye highlights turn into speckles that read
// as noise.
const sharpenAmount = 0.5

// Sharpen applies an unsharp mask built from a 3×3 box blur.
//
// It works on the straight (non-premultiplied) colour channels and leaves alpha
// alone: sharpening alpha would punch a bright ring into the tile's soft edge.
func Sharpen(src *image.NRGBA, amount float64) *image.NRGBA {
	if src == nil || amount <= 0 {
		return src
	}
	b := src.Bounds()
	w, h := b.Dx(), b.Dy()
	if w < 3 || h < 3 {
		return src
	}
	blurred := box3(src)
	out := image.NewNRGBA(b)
	for y := 0; y < h; y++ {
		for x := 0; x < w; x++ {
			c := src.NRGBAAt(x, y)
			d := blurred.NRGBAAt(x, y)
			sharpen := func(v, b uint8) uint8 {
				return clamp8(float64(v) + (float64(v)-float64(b))*amount)
			}
			out.SetNRGBA(x, y, color.NRGBA{
				R: sharpen(c.R, d.R),
				G: sharpen(c.G, d.G),
				B: sharpen(c.B, d.B),
				A: c.A,
			})
		}
	}
	return out
}

// box3 is a 3×3 box blur with clamped edges.
func box3(src *image.NRGBA) *image.NRGBA {
	b := src.Bounds()
	w, h := b.Dx(), b.Dy()
	out := image.NewNRGBA(b)
	at := func(x, y int) color.NRGBA {
		if x < 0 {
			x = 0
		}
		if y < 0 {
			y = 0
		}
		if x >= w {
			x = w - 1
		}
		if y >= h {
			y = h - 1
		}
		return src.NRGBAAt(x, y)
	}
	for y := 0; y < h; y++ {
		for x := 0; x < w; x++ {
			var sr, sg, sb int
			for dy := -1; dy <= 1; dy++ {
				for dx := -1; dx <= 1; dx++ {
					c := at(x+dx, y+dy)
					sr += int(c.R)
					sg += int(c.G)
					sb += int(c.B)
				}
			}
			out.SetNRGBA(x, y, color.NRGBA{
				R: uint8(sr / 9), G: uint8(sg / 9), B: uint8(sb / 9), A: src.NRGBAAt(x, y).A,
			})
		}
	}
	return out
}

func clamp8(v float64) uint8 {
	if v <= 0 {
		return 0
	}
	if v >= 255 {
		return 255
	}
	return uint8(v + 0.5)
}

// Contrast stretches each channel away from mid grey, leaving alpha alone.
//
// It is a plain linear stretch around 128, not a curve: the purpose is to keep two
// neighbouring tones apart after a downscale, and a curve with a toe would crush the
// tile's soft edge into a hard band.
func Contrast(src *image.NRGBA, amount float64) *image.NRGBA {
	if src == nil || amount == 0 {
		return src
	}
	b := src.Bounds()
	out := image.NewNRGBA(b)
	stretch := func(v uint8) uint8 {
		return clamp8(128 + (float64(v)-128)*(1+amount))
	}
	for y := b.Min.Y; y < b.Max.Y; y++ {
		for x := b.Min.X; x < b.Max.X; x++ {
			c := src.NRGBAAt(x, y)
			out.SetNRGBA(x, y, color.NRGBA{
				R: stretch(c.R), G: stretch(c.G), B: stretch(c.B), A: c.A,
			})
		}
	}
	return out
}
