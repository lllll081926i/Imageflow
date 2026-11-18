package engine_test

import (
	"bytes"
	"context"
	"image"
	"image/color"
	"os"
	"path/filepath"
	"testing"

	"github.com/lllll081926i/imageflow-go/internal/codec"
	"github.com/lllll081926i/imageflow-go/internal/colorprofile"
	"github.com/lllll081926i/imageflow-go/internal/engine"
	"github.com/pdfcpu/pdfcpu/pkg/api"
	"github.com/pdfcpu/pdfcpu/pkg/pdfcpu/model"
	"github.com/pdfcpu/pdfcpu/pkg/pdfcpu/types"
)

func TestPDFImagesUseSRGBAndWhitePaper(t *testing.T) {
	dir := t.TempDir()
	source := filepath.Join(dir, "transparent.png")
	if err := codec.Save(image.NewNRGBA(image.Rect(0, 0, 8, 8)), source, codec.SaveOptions{}); err != nil {
		t.Fatal(err)
	}
	target := filepath.Join(dir, "image.pdf")
	res := engine.GeneratePDF(engine.PDFRequest{ImagePaths: []string{source}, OutputPath: target})
	if !res.Success {
		t.Fatal(res.Error)
	}
	f, err := os.Open(target)
	if err != nil {
		t.Fatal(err)
	}
	defer f.Close()
	ctx, err := api.ReadAndValidate(context.Background(), f, model.NewDefaultConfiguration())
	if err != nil {
		t.Fatal(err)
	}
	images := 0
	for _, entry := range ctx.Table {
		sd, ok := entry.Object.(types.StreamDict)
		if !ok {
			continue
		}
		if subtype := sd.NameEntry("Subtype"); subtype == nil || *subtype != "Image" {
			continue
		}
		space, ok := sd.Dict["ColorSpace"].(types.Array)
		if !ok || len(space) != 2 || space[0] != types.Name("ICCBased") {
			t.Fatalf("图像未使用 ICCBased：%v", sd.Dict)
		}
		profile, _, err := ctx.DereferenceStreamDict(space[1])
		if err != nil {
			t.Fatal(err)
		}
		if err := profile.Decode(); err != nil {
			t.Fatal(err)
		}
		if !bytes.Equal(profile.Content, colorprofile.SRGB()) {
			t.Fatal("PDF 未嵌入标准 sRGB")
		}
		img, _, err := image.Decode(bytes.NewReader(sd.Raw))
		if err != nil {
			t.Fatal(err)
		}
		pixel := color.NRGBAModel.Convert(img.At(4, 4)).(color.NRGBA)
		if pixel.R < 250 || pixel.G < 250 || pixel.B < 250 {
			t.Fatalf("透明区域未合成白色纸面：%+v", pixel)
		}
		images++
	}
	if images != 1 {
		t.Fatalf("图像数量错误：%d", images)
	}
}

func TestRichTextMissingImageKeepsExistingOutput(t *testing.T) {
	target := filepath.Join(t.TempDir(), "existing.png")
	if err := os.WriteFile(target, []byte("existing"), 0600); err != nil {
		t.Fatal(err)
	}
	res := engine.RenderRichText(engine.RichTextRequest{OutputPath: target, Blocks: []engine.RichTextBlock{{Type: "paragraph", Text: "保留文字"}, {Type: "image", Path: filepath.Join(t.TempDir(), "missing.png")}}})
	if res.Success {
		t.Fatal("缺失图片被静默跳过")
	}
	raw, err := os.ReadFile(target)
	if err != nil || string(raw) != "existing" {
		t.Fatal("失败后覆盖了已有输出")
	}
}

type cancelAtExportCheck struct {
	context.Context
	cancel context.CancelFunc
	checks int
	at     int
}

func (ctx *cancelAtExportCheck) Err() error {
	ctx.checks++
	if ctx.checks == ctx.at {
		ctx.cancel()
	}
	return ctx.Context.Err()
}

func TestCanvasExportsKeepExistingOutputOnCancel(t *testing.T) {
	for _, tool := range []struct {
		name   string
		export func(context.Context, string) engine.Result
	}{
		{"自由布局", func(ctx context.Context, path string) engine.Result {
			return engine.LayoutContext(ctx, engine.CanvasRequest{Width: 64, Height: 64, OutputPath: path})
		}},
		{"富文本制图", func(ctx context.Context, path string) engine.Result {
			return engine.RenderRichTextContext(ctx, engine.RichTextRequest{Width: 320, OutputPath: path,
				Blocks: []engine.RichTextBlock{{Type: "paragraph", Text: "取消测试"}}})
		}},
	} {
		t.Run(tool.name, func(t *testing.T) {
			for _, at := range []int{1, 2, 4} {
				path := filepath.Join(t.TempDir(), "existing.png")
				original := []byte("已有输出")
				if err := os.WriteFile(path, original, 0600); err != nil {
					t.Fatal(err)
				}
				base, cancel := context.WithCancel(context.Background())
				result := tool.export(&cancelAtExportCheck{Context: base, cancel: cancel, at: at}, path)
				cancel()
				if result.Success || result.ErrorCode != "CANCELLED" {
					t.Fatalf("在检查点 %d 取消仍未返回取消状态：%+v", at, result)
				}
				data, err := os.ReadFile(path)
				if err != nil || !bytes.Equal(data, original) {
					t.Fatalf("在检查点 %d 取消后改变了已有输出：%q，%v", at, data, err)
				}
			}
		})
	}
}
