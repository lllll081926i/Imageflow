package codec

import (
	"bytes"
	"image"

	"github.com/gen2brain/avif"
	"github.com/gen2brain/webp"
	"github.com/lllll081926i/imageflow-go/internal/pngopt"
)

// CompressBytes 复用 jpegli / libwebp / libavif，并用 OxiPNG 优化 PNG。
func CompressBytes(img image.Image, format string, quality int, lossless bool) ([]byte, error) {
	var buf bytes.Buffer
	switch format {
	case "webp":
		err := webp.Encode(&buf, img, webp.Options{Quality: quality, Lossless: lossless, Method: 6})
		return buf.Bytes(), err
	case "avif":
		err := avif.Encode(&buf, img, avif.Options{Quality: quality, QualityAlpha: 100, Speed: 4})
		return buf.Bytes(), err
	}
	data, err := EncodeBytes(img, format, SaveOptions{Quality: quality, Lossless: lossless})
	if err != nil || format != "png" {
		return data, err
	}
	return pngopt.Optimize(data)
}
