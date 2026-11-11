package engine

import (
	"image"

	"github.com/lllll081926i/imageflow-go/internal/codec"
)

// SaveImage writes an already encoded image to path, used by tests and the
// palette preview.
func SaveImage(img image.Image, path string) error {
	return codec.Save(img, path, codec.SaveOptions{Quality: 95})
}
