package engine

import (
	"context"
	"image"
	"image/color"
	"math"
	"os"
	"path/filepath"
	"testing"
)

type boundsOnlyImage struct{ bounds image.Rectangle }

func (s boundsOnlyImage) Bounds() image.Rectangle { return s.bounds }
func (s boundsOnlyImage) ColorModel() color.Model { return color.NRGBA64Model }
func (s boundsOnlyImage) At(int, int) color.Color { panic("超限缩放不应读取源像素") }

func TestResizeRejectsBeforeReadingPixels(t *testing.T) {
	img := boundsOnlyImage{image.Rect(0, 0, 8000, 8000)}
	for _, spec := range []ResizeSpec{
		{Mode: "percent", ScalePercent: 500},
		{Mode: "percent", ScalePercent: math.MaxFloat64},
		{Mode: "percent", ScalePercent: math.NaN()},
		{Mode: "exact", Width: 16000, Height: 16000},
		{Mode: "width", Width: 16000},
		{Mode: "short_edge", LongEdge: 16000},
	} {
		if _, err := applyResize(img, spec); err == nil {
			t.Fatalf("未拒绝超限尺寸：%+v", spec)
		}
	}
	if _, err := applyResize(boundsOnlyImage{image.Rect(0, 0, 1, 16000)}, ResizeSpec{Mode: "width", Width: 1000}); err == nil {
		t.Fatal("未拒绝按比例推导的超长边")
	}
}

func TestWholeListCancelledBeforeOutput(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	target := filepath.Join(t.TempDir(), "out.png")
	for _, res := range []Result{
		StitchContext(ctx, StitchRequest{InputPaths: []string{"unused.png"}, OutputPath: target}),
		GeneratePDFContext(ctx, PDFRequest{ImagePaths: []string{"unused.png"}, OutputPath: target}),
	} {
		if res.ErrorCode != "CANCELLED" {
			t.Fatalf("取消仍进入处理：%+v", res)
		}
	}
	if _, err := os.Stat(target); !os.IsNotExist(err) {
		t.Fatal("取消任务写出了文件")
	}
}
