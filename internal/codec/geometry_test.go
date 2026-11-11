package codec

import (
	"context"
	"image"
	"image/color"
	"os"
	"path/filepath"
	"testing"
)

func TestHighDepthOrientation(t *testing.T) {
	src := image.NewNRGBA64(image.Rect(7, 9, 10, 11))
	for i := 0; i < 6; i++ {
		src.SetNRGBA64(7+i%3, 9+i/3, color.NRGBA64{uint16(12345 + i), 23456, 34567, uint16(i * 10000)})
	}
	for i, want := range [][]int{{0, 1, 2, 3, 4, 5}, {2, 1, 0, 5, 4, 3}, {5, 4, 3, 2, 1, 0}, {3, 4, 5, 0, 1, 2}, {0, 3, 1, 4, 2, 5}, {3, 0, 4, 1, 5, 2}, {5, 2, 4, 1, 3, 0}, {2, 5, 1, 4, 0, 3}} {
		out := orientHighDepth(src, i+1).(*image.NRGBA64)
		for p, index := range want {
			got := out.NRGBA64At(p%out.Bounds().Dx(), p/out.Bounds().Dx())
			if got != src.NRGBA64At(7+index%3, 9+index/3) {
				t.Fatalf("方向=%d 像素=%d：%v", i+1, p, got)
			}
		}
	}
	data, err := EncodeBytes(src, "png", SaveOptions{})
	if err != nil {
		t.Fatal(err)
	}
	exif := []byte{'I', 'I', 42, 0, 8, 0, 0, 0, 1, 0, 0x12, 1, 3, 0, 1, 0, 0, 0, 6, 0, 0, 0, 0, 0, 0, 0}
	data = append(append(append([]byte(nil), data[:33]...), metadataPNGChunk("eXIf", exif)...), data[33:]...)
	path := filepath.Join(t.TempDir(), "rotate.png")
	if err := os.WriteFile(path, data, 0600); err != nil {
		t.Fatal(err)
	}
	opened, err := Open(path)
	if err != nil {
		t.Fatal(err)
	}
	out, ok := opened.Image.(*image.NRGBA64)
	if !ok || out.NRGBA64At(0, 0) != src.NRGBA64At(7, 10) {
		t.Fatalf("PNG 方向处理损失位深或像素：%T", opened.Image)
	}
}

func TestHighDepthResize(t *testing.T) {
	src := image.NewNRGBA64(image.Rect(0, 0, 2, 2))
	want := color.NRGBA64{12345, 23456, 34567, 65535}
	for y := 0; y < 2; y++ {
		for x := 0; x < 2; x++ {
			src.SetNRGBA64(x, y, want)
		}
	}
	out, ok := ResizePixels(src, 5, 3).(*image.NRGBA64)
	if !ok {
		t.Fatal("缩放丢失 16 位像素")
	}
	for y := 0; y < 3; y++ {
		for x := 0; x < 5; x++ {
			if got := out.NRGBA64At(x, y); got != want {
				t.Fatalf("常量色缩放改变精度：%v", got)
			}
		}
	}
}

type cancelOnRead struct {
	image.Image
	cancel context.CancelFunc
}

func (s cancelOnRead) At(x, y int) color.Color { s.cancel(); return s.Image.At(x, y) }

func TestCancelledEncodeKeepsExistingOutput(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	img := cancelOnRead{image.NewNRGBA(image.Rect(0, 0, 4, 4)), cancel}
	path := filepath.Join(t.TempDir(), "out.png")
	original := []byte("原有文件")
	if err := os.WriteFile(path, original, 0600); err != nil {
		t.Fatal(err)
	}
	if err := SaveContext(ctx, img, path, SaveOptions{SRGB: true}); err != context.Canceled {
		t.Fatalf("取消结果=%v", err)
	}
	data, err := os.ReadFile(path)
	if err != nil || string(data) != string(original) {
		t.Fatal("取消后已有输出发生变化")
	}
}
