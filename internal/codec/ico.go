package codec

// ico.go encodes a multi-resolution Windows icon from in-memory images.

import (
	"fmt"
	"image"
	"os"

	"github.com/sergeymakinen/go-ico"
)

// SaveICO writes images into target as one .ico. Each image is stored as a PNG
// entry, which is what every Windows version since Vista reads, and it keeps
// alpha instead of the old 1-bit mask.
func SaveICO(images []image.Image, target string) error {
	if len(images) == 0 {
		return fmt.Errorf("没有可写入的图标")
	}
	for _, img := range images {
		if img == nil {
			return fmt.Errorf("图标为空")
		}
		b := img.Bounds()
		if b.Dx() > 256 || b.Dy() > 256 {
			return fmt.Errorf("ICO 条目不能超过 256×256")
		}
	}
	return writeFile(target, func(f *os.File) error {
		return ico.EncodeAll(f, images)
	})
}
