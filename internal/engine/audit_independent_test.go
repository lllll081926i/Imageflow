package engine

import (
	"context"
	"errors"
	"image"
	"image/color"
	"math"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/lllll081926i/imageflow-go/internal/output"
)

func TestAuditStandardCubeIdentity(t *testing.T) {
	lut, err := ParseLUT(strings.NewReader("LUT_3D_SIZE 2\n0 0 0\n1 0 0\n0 1 0\n1 1 0\n0 0 1\n1 0 1\n0 1 1\n1 1 1\n"))
	if err != nil {
		t.Fatal(err)
	}
	red, green, blue := lut.Apply(255, 0, 0)
	if red != 255 || green != 0 || blue != 0 {
		t.Fatalf("标准恒等 cube 将红色变成 (%v,%v,%v)", red, green, blue)
	}
}

func TestAuditZeroOpacity(t *testing.T) {
	src := image.NewNRGBA(image.Rect(0, 0, 1, 1))
	src.SetNRGBA(0, 0, color.NRGBA{R: 255, A: 255})
	got, err := RenderEdit(src, EditRequest{Opacity: 0})
	if err != nil {
		t.Fatal(err)
	}
	_, _, _, alpha := got.At(0, 0).RGBA()
	if alpha != 0 {
		t.Fatalf("0%% 不透明度仍为 alpha=%d", alpha)
	}
}

func TestAuditScreenshotBudget(t *testing.T) {
	src := image.NewNRGBA(image.Rect(0, 0, output.MaxOutputSide, 1))
	got, err := RenderScreenshot(src, ScreenshotRequest{Padding: 1})
	if err == nil {
		t.Fatalf("未拒绝超限画布 %v", got.Bounds())
	}
}

func TestAuditFourColorLimit(t *testing.T) {
	src := image.NewNRGBA(image.Rect(0, 0, 64, 1))
	index := 0
	for red := 0; red < 4; red++ {
		for green := 0; green < 4; green++ {
			for blue := 0; blue < 4; blue++ {
				src.SetNRGBA(index, 0, color.NRGBA{R: uint8(red * 85), G: uint8(green * 85), B: uint8(blue * 85), A: 255})
				index++
			}
		}
	}
	_, used, _, err := RenderPixelArt(src, PixelRequest{Width: 64, Height: 1, FitMode: "stretch", Colors: 4, Threshold: 128})
	if err != nil {
		t.Fatal(err)
	}
	if used > 4 {
		t.Fatalf("选择 4 色，实际输出 %d 色", used)
	}
}

func TestAuditGrayUsesLuminance(t *testing.T) {
	src := image.NewNRGBA(image.Rect(0, 0, 1, 1))
	src.SetNRGBA(0, 0, color.NRGBA{G: 255, A: 255})
	got, _, _, err := RenderPixelArt(src, PixelRequest{Width: 1, Height: 1, Gray: true, Colors: 4, Threshold: 128})
	if err != nil {
		t.Fatal(err)
	}
	red, _, _, _ := got.At(0, 0).RGBA()
	if red == 0 {
		t.Fatal("纯绿的灰阶被输出为纯黑，而不是亮度灰阶")
	}
}

func TestAuditFitComparesOverlap(t *testing.T) {
	ref := image.NewNRGBA(image.Rect(0, 0, 4, 4))
	res := image.NewNRGBA(image.Rect(0, 0, 4, 2))
	for row := 0; row < 4; row++ {
		for col := 0; col < 4; col++ {
			ref.SetNRGBA(col, row, color.NRGBA{R: 255, G: 255, B: 255, A: 255})
		}
	}
	for row := 0; row < 2; row++ {
		for col := 0; col < 4; col++ {
			res.SetNRGBA(col, row, color.NRGBA{R: 255, G: 255, B: 255, A: 255})
		}
	}
	got := Compare(CompareRequest{Reference: ref, Result: res, Align: AlignFit})
	if !math.IsInf(got.PSNR, 1) {
		t.Fatalf("重叠区全白且完全一致，PSNR=%v；补边参与了比较", got.PSNR)
	}
}

func TestPixelColorLimitAcrossDitherModes(t *testing.T) {
	source := image.NewNRGBA(image.Rect(0, 0, 32, 32))
	for row := 0; row < 32; row++ {
		for column := 0; column < 32; column++ {
			source.SetNRGBA(column, row, color.NRGBA{R: uint8(column * 8), G: uint8(row * 8), B: uint8((column + row) * 4), A: 255})
		}
	}
	for _, mode := range DitherModes {
		for _, limit := range []int{2, 4, 16} {
			_, count, _, err := RenderPixelArt(source, PixelRequest{Width: 32, Height: 32, Colors: limit, Dither: mode, Strength: 100})
			if err != nil || count > limit {
				t.Fatalf("%s/%d 色超限：%d %v", mode, limit, count, err)
			}
		}
	}
}

func TestPixelZeroThresholdIsNotDefaulted(t *testing.T) {
	source := image.NewNRGBA(image.Rect(0, 0, 1, 1))
	source.SetNRGBA(0, 0, color.NRGBA{A: 255})
	result, _, _, err := RenderPixelArt(source, PixelRequest{Width: 1, Height: 1, StrictBlackWhite: true, Threshold: 0})
	if err != nil {
		t.Fatal(err)
	}
	red, _, _, _ := result.At(0, 0).RGBA()
	if red != 65535 {
		t.Fatal("零阈值被重置为默认阈值")
	}
}

func TestCancelledAnimationWritersPreserveTarget(t *testing.T) {
	anim := &Animation{Width: 2, Height: 2, Frames: []Frame{{Image: image.NewNRGBA(image.Rect(0, 0, 2, 2)), Delay: 100}}}
	for _, format := range []string{"gif", "apng", "webp"} {
		t.Run(format, func(t *testing.T) {
			path := filepath.Join(t.TempDir(), "out."+format)
			if err := os.WriteFile(path, []byte("保留旧文件"), 0600); err != nil {
				t.Fatal(err)
			}
			ctx, cancel := context.WithCancel(context.Background())
			cancel()
			var err error
			switch format {
			case "gif":
				err = writeGIFContext(ctx, anim, path, 90)
			case "apng":
				err = writeAPNGContext(ctx, anim, path)
			case "webp":
				err = writeAnimatedWebPContext(ctx, anim, path, 90)
			}
			if !errors.Is(err, context.Canceled) {
				t.Fatalf("取消未传入编码：%v", err)
			}
			data, err := os.ReadFile(path)
			if err != nil || string(data) != "保留旧文件" {
				t.Fatalf("取消修改了旧文件：%q %v", data, err)
			}
		})
	}
}
