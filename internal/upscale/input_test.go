package upscale

import (
	"context"
	"errors"
	"image"
	"image/color"
	"os"
	"testing"

	"github.com/lllll081926i/imageflow-go/internal/codec"
)

func TestInferenceInputAndAlpha(t *testing.T) {
	source := image.NewNRGBA(image.Rect(3, 4, 6, 5))
	source.SetNRGBA(3, 4, color.NRGBA{R: 255})
	source.SetNRGBA(4, 4, color.NRGBA{R: 255, A: 128})
	source.SetNRGBA(5, 4, color.NRGBA{B: 255, A: 255})
	path, err := writeInferenceInput(context.Background(), source, t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	decoded, err := codec.Open(path)
	if err != nil {
		t.Fatal(err)
	}
	for column := 0; column < 3; column++ {
		if _, _, _, alpha := decoded.Image.At(column, 0).RGBA(); alpha != 65535 {
			t.Fatal("推理输入必须不透明")
		}
	}
	if pixel := color.NRGBAModel.Convert(decoded.Image.At(0, 0)).(color.NRGBA); pixel != (color.NRGBA{255, 255, 255, 255}) {
		t.Fatalf("透明输入必须复合白底：%v", pixel)
	}
	rendered := image.NewNRGBA(image.Rect(0, 0, 3, 1))
	result, restored, err := restoreInputAlpha(context.Background(), rendered, source)
	if err != nil || !restored {
		t.Fatalf("透明通道恢复失败：%t %v", restored, err)
	}
	for column, want := range []uint8{0, 128, 255} {
		if pixel := color.NRGBAModel.Convert(result.At(column, 0)).(color.NRGBA); pixel.A != want {
			t.Fatalf("位置 %d：Alpha=%d，期望 %d", column, pixel.A, want)
		}
	}
}

func TestInferenceInputCancellation(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	directory := t.TempDir()
	_, err := writeInferenceInput(ctx, image.NewNRGBA(image.Rect(0, 0, 2, 2)), directory)
	if !errors.Is(err, context.Canceled) {
		t.Fatalf("取消未传播：%v", err)
	}
	entries, err := os.ReadDir(directory)
	if err != nil || len(entries) != 0 {
		t.Fatalf("取消后遗留输入文件：%v %v", entries, err)
	}
}

func TestRejectUnsupportedTile(t *testing.T) {
	req := auditUpscaleRequest(t, "model-name")
	req.Tile = 16
	_, err := Run(context.Background(), req)
	var failure *Error
	if !errors.As(err, &failure) || failure.Code != CodeBadRequest {
		t.Fatalf("不支持的 Tile 应拒绝执行：%v", err)
	}
}
