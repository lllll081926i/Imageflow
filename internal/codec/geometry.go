package codec

import (
	"image"

	"github.com/disintegration/imaging"
	"golang.org/x/image/draw"
)

func highDepth(img image.Image) bool {
	switch img.(type) {
	case *image.NRGBA64, *image.RGBA64, *image.Gray16:
		return true
	}
	return false
}

// ResizePixels 保留高位深像素；调用者必须先核验目标尺寸。
func ResizePixels(img image.Image, w, h int) image.Image {
	if !highDepth(img) {
		return imaging.Resize(img, w, h, imaging.Lanczos)
	}
	b := img.Bounds()
	if w <= 0 {
		w = max(1, int(float64(b.Dx())*float64(h)/float64(b.Dy())+.5))
	}
	if h <= 0 {
		h = max(1, int(float64(b.Dy())*float64(w)/float64(b.Dx())+.5))
	}
	dst := image.NewNRGBA64(image.Rect(0, 0, w, h))
	draw.CatmullRom.Scale(dst, dst.Bounds(), img, b, draw.Src, nil)
	return dst
}

func orientHighDepth(img image.Image, orientation int) image.Image {
	b := img.Bounds()
	w, h := b.Dx(), b.Dy()
	if orientation >= 5 {
		w, h = h, w
	}
	dst := image.NewNRGBA64(image.Rect(0, 0, w, h))
	for y := 0; y < h; y++ {
		for x := 0; x < w; x++ {
			sx, sy := x, y
			switch orientation {
			case 2:
				sx = b.Dx() - 1 - x
			case 3:
				sx, sy = b.Dx()-1-x, b.Dy()-1-y
			case 4:
				sy = b.Dy() - 1 - y
			case 5:
				sx, sy = y, x
			case 6:
				sx, sy = y, b.Dy()-1-x
			case 7:
				sx, sy = b.Dx()-1-y, b.Dy()-1-x
			case 8:
				sx, sy = b.Dx()-1-y, x
			}
			dst.Set(x, y, img.At(b.Min.X+sx, b.Min.Y+sy))
		}
	}
	return dst
}
