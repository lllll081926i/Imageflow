package codec

import (
	"bytes"
	"image"
	"image/color"
	"os"
	"path/filepath"
	"testing"
)

func TestColorProfileRoundTrip(t *testing.T) {
	profile := make([]byte, 256)
	copy(profile[36:], "acsp")
	copy(profile[128:], "ImageFlow-color-test")
	img := image.NewNRGBA(image.Rect(0, 0, 16, 12))
	for y := 0; y < 12; y++ {
		for x := 0; x < 16; x++ {
			img.SetNRGBA(x, y, color.NRGBA{R: uint8(x * 10), G: 70, B: 120, A: 255})
		}
	}
	plain, err := EncodeBytes(img, "png", SaveOptions{})
	if err != nil {
		t.Fatal(err)
	}
	seed, err := pngMetadata(plain, imageMetadata{icc: profile})
	if err != nil {
		t.Fatal(err)
	}
	source := filepath.Join(t.TempDir(), "source.png")
	if err := os.WriteFile(source, seed, 0600); err != nil {
		t.Fatal(err)
	}
	for _, format := range []string{"png", "jpg", "webp", "avif", "tiff"} {
		t.Run(format, func(t *testing.T) {
			target := filepath.Join(t.TempDir(), "image."+format)
			if err := Save(img, target, SaveOptions{MetadataSource: source, Quality: 85}); err != nil {
				t.Fatal(err)
			}
			meta, err := readMetadata(target)
			if err != nil {
				t.Fatal(err)
			}
			if !bytes.Equal(meta.icc, profile) {
				t.Fatalf("%s ICC 未完整保留：%d 字节", format, len(meta.icc))
			}
			decoded, err := Open(target)
			if err != nil || decoded.Width != 16 || decoded.Height != 12 {
				t.Fatalf("编码结果不可解码：%v", err)
			}
			roundTrip := filepath.Join(t.TempDir(), "back.png")
			if err := Save(decoded.Image, roundTrip, SaveOptions{MetadataSource: target}); err != nil {
				t.Fatal(err)
			}
			back, err := readMetadata(roundTrip)
			if err != nil || !bytes.Equal(back.icc, profile) {
				t.Fatalf("回转后配置丢失：%v", err)
			}
		})
	}
	protected := filepath.Join(t.TempDir(), "old.bmp")
	os.WriteFile(protected, []byte("keep"), 0600)
	if err := Save(img, protected, SaveOptions{MetadataSource: source}); err == nil {
		t.Fatal("不支持色彩配置的格式应明确拒绝")
	}
	after, _ := os.ReadFile(protected)
	if string(after) != "keep" {
		t.Fatal("保存失败覆盖了已有文件")
	}
}
