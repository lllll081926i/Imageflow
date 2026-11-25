package ui

import (
	"context"
	"fmt"

	myui "github.com/egoist/mygo/ui"
	"github.com/lllll081926i/imageflow-go/internal/batch"
	"github.com/lllll081926i/imageflow-go/internal/engine"
	"github.com/lobehub/lobe-mygo/pkg/lucide"
)

// CompressTool 是静态图片压缩页。
type CompressTool struct {
	format   int
	quality  float64
	lossless bool
}

var compressionFormats = []string{"auto", "jpg", "png", "webp", "avif"}

func (t *CompressTool) ID() string      { return ToolCompress }
func (t *CompressTool) Name() string    { return "图片压缩" }
func (t *CompressTool) Icon() *myui.SVG { return lucide.Minimize2() }
func (t *CompressTool) ensure() {
	if t.quality == 0 {
		t.quality = 80
	}
	if t.format < 0 || t.format >= len(compressionFormats) {
		t.format = 0
	}
}

func (t *CompressTool) Panel(c *myui.Context, a *App) {
	t.ensure()
	sectionCard(c, "压缩设置", func() {
		segmented(c, &t.format, []string{"原格式", "JPG", "PNG", "WebP", "AVIF"})
		if t.format == 3 {
			revealBlock(c, "compress-webp-lossless", func() {
				switchField(c, &t.lossless, "WebP 无损压缩")
			})
		}
		if t.format != 2 && !(t.format == 3 && t.lossless) && (t.format != 0 || qualityRelevant(a, "")) {
			revealBlock(c, "compress-quality", func() {
				sliderRow(c, "质量", &t.quality, 1, 100, 1, "%.0f")
			})
		}
	})
}

func (t *CompressTool) PreviewCaption(a *App) string {
	if t.format == 2 || (t.format == 3 && t.lossless) || (t.format == 0 && !qualityRelevant(a, "")) {
		return "保持尺寸 · 无损压缩"
	}
	return fmt.Sprintf("保持尺寸 · 质量 %.0f", t.quality)
}

func (t *CompressTool) Run(a *App) {
	t.ensure()
	format, quality, lossless := compressionFormats[t.format], int(t.quality), t.format == 3 && t.lossless
	a.runOutputBatch("压缩", func(index int, item batch.Item) (string, engine.Result) {
		ext := engine.CompressionFormat(item.Path, format)
		if ext == "" {
			return "", engine.Fail("BAD_INPUT", "不支持的图片格式，请使用 JPG / PNG / WebP / AVIF")
		}
		out, skipped := a.outputPathFor(item, "compress", ext, index+1)
		if skipped {
			return "", engine.Skip("目标已存在")
		}
		return out, engine.OK()
	}, func(_ context.Context, index int, item batch.Item, out string) engine.Result {
		return engine.Compress(engine.CompressRequest{
			InputPath: item.Path, OutputPath: out, Format: format, Quality: quality, Lossless: lossless,
		})
	})
}
