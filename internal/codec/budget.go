package codec

import (
	"bytes"
	"encoding/binary"
	"fmt"
	"image"
	"image/gif"
	"image/jpeg"
	"image/png"
	"io"
	"path/filepath"
	"strings"

	"github.com/gen2brain/avif"
	"github.com/gen2brain/heic"
	"github.com/gen2brain/webp"
	"github.com/sergeymakinen/go-ico"
	"golang.org/x/image/bmp"
	"golang.org/x/image/tiff"
)

const MaxSourceBytes = 256 << 20

// decodeConfig 按格式读取有界头部，禁止在探测失败后回退到像素解码。
func decodeConfig(r io.Reader, path string) (image.Config, error) {
	r = io.LimitReader(r, 8<<20)
	switch strings.ToLower(filepath.Ext(path)) {
	case ".jpg", ".jpeg":
		return jpeg.DecodeConfig(r)
	case ".png", ".apng":
		return png.DecodeConfig(r)
	case ".gif":
		return gif.DecodeConfig(r)
	case ".webp":
		return webp.DecodeConfig(r)
	case ".avif":
		return avif.DecodeConfig(r)
	case ".heic", ".heif":
		// HEIF 序列解码器会读取全部帧，静态入口先拒绝序列品牌。
		h := make([]byte, 8)
		if _, err := io.ReadFull(r, h); err != nil {
			return image.Config{}, err
		}
		n := int(binary.BigEndian.Uint32(h))
		if string(h[4:]) != "ftyp" || n < 16 || n > 4096 {
			return image.Config{}, fmt.Errorf("无效的 HEIF 类型块")
		}
		brands := make([]byte, n-8)
		if _, err := io.ReadFull(r, brands); err != nil {
			return image.Config{}, err
		}
		for i := 0; i+4 <= len(brands); i += 4 {
			if i == 4 {
				continue
			}
			switch string(brands[i : i+4]) {
			case "msf1", "hevc", "hevx", "avis":
				return image.Config{}, fmt.Errorf("静态图片入口不支持 HEIF 图像序列")
			}
		}
		return heic.DecodeConfig(io.MultiReader(bytes.NewReader(h), bytes.NewReader(brands), r))
	case ".bmp":
		return bmp.DecodeConfig(r)
	case ".tif", ".tiff":
		return tiff.DecodeConfig(r)
	case ".ico":
		return ico.DecodeConfig(r)
	}
	return image.Config{}, fmt.Errorf("%w: %s", ErrUnsupported, filepath.Ext(path))
}
