package engine

import (
	"image"

	"github.com/disintegration/imaging"
)

// resizeImage scales img to exactly w×h, or derives one side when the other is
// zero, keeping the aspect ratio.
func resizeImage(img image.Image, w, h int) image.Image {
	if w <= 0 && h <= 0 {
		return img
	}
	if w <= 0 {
		return imaging.Resize(img, 0, h, imaging.Lanczos)
	}
	if h <= 0 {
		return imaging.Resize(img, w, 0, imaging.Lanczos)
	}
	return imaging.Resize(img, w, h, imaging.Lanczos)
}
