package engine

import (
	"bytes"
	"context"
	"image"
	"image/color"
	"os"
	"path/filepath"
	"testing"

	"github.com/pdfcpu/pdfcpu/pkg/api"
	"github.com/pdfcpu/pdfcpu/pkg/pdfcpu/model"
	"github.com/pdfcpu/pdfcpu/pkg/pdfcpu/types"
)

func TestAnimationResizeBudgetBeforeAllocation(t *testing.T) {
	for _, request := range []GIFRequest{
		{Width: 40000, Height: 2},
		{Width: 1000, MaintainAR: true},
		{Width: 4000, Height: 4000},
	} {
		source := boundsOnlyImage{image.Rect(0, 0, 1, 100)}
		animation := &Animation{Width: 1, Height: 100, Frames: []Frame{{Image: source}, {Image: source}, {Image: source}}}
		if err := resizeFrames(animation, request); err == nil {
			t.Fatalf("未在分配前拒绝超限请求：%+v", request)
		}
		if animation.Width != 1 || animation.Height != 100 || animation.Frames[0].Image != source {
			t.Fatal("拒绝请求后修改了原始帧")
		}
	}
}

func TestAPNGRejectsUnrepresentableDelayAtomically(t *testing.T) {
	target := filepath.Join(t.TempDir(), "existing.apng")
	original := []byte("保留原文件")
	if err := os.WriteFile(target, original, 0600); err != nil {
		t.Fatal(err)
	}
	animation := &Animation{Width: 2, Height: 2, Frames: []Frame{{Image: image.NewNRGBA(image.Rect(0, 0, 2, 2)), Delay: 65537}}}
	if err := writeAPNG(animation, target); err == nil {
		t.Fatal("无法精确表示的延时未报错")
	}
	actual, err := os.ReadFile(target)
	if err != nil {
		t.Fatal(err)
	}
	if !bytes.Equal(actual, original) {
		t.Fatal("编码失败覆盖了已有文件")
	}
}

func TestPDFClipsEachCoverCell(t *testing.T) {
	dir := t.TempDir()
	first := solid(t, dir, "first.png", 100, 10, color.NRGBA{R: 255, A: 255})
	second := solid(t, dir, "second.png", 10, 100, color.NRGBA{B: 255, A: 255})
	target := filepath.Join(dir, "cover.pdf")
	result := GeneratePDF(PDFRequest{ImagePaths: []string{first, second}, OutputPath: target, Layout: "two", FitMode: "cover"})
	if !result.Success {
		t.Fatal(result.Error)
	}
	file, err := os.Open(target)
	if err != nil {
		t.Fatal(err)
	}
	defer file.Close()
	document, err := api.ReadAndValidate(context.Background(), file, model.NewDefaultConfiguration())
	if err != nil {
		t.Fatal(err)
	}
	clips := 0
	for _, entry := range document.Table {
		stream, ok := entry.Object.(types.StreamDict)
		if !ok || stream.NameEntry("Subtype") != nil || stream.NameEntry("Type") != nil {
			continue
		}
		if err := stream.Decode(); err != nil {
			continue
		}
		count := bytes.Count(stream.Content, []byte("h W n"))
		if count > 0 {
			if bytes.Count(stream.Content, []byte("q\n")) != bytes.Count(stream.Content, []byte("Q\n")) {
				t.Fatal("图形状态未配对")
			}
			clips += count
		}
	}
	if clips != 2 {
		t.Fatalf("单元格裁切数量错误：%d", clips)
	}
}

func TestRegressionAPNGPreservesLongDelay(t *testing.T) {
	path := filepath.Join(t.TempDir(), "long.apng")
	animation := &Animation{Frames: []Frame{{Image: image.NewNRGBA(image.Rect(0, 0, 2, 2)), Delay: 70000}, {Image: image.NewNRGBA(image.Rect(0, 0, 2, 2)), Delay: 100}}, Width: 2, Height: 2}
	if err := writeAPNG(animation, path); err != nil {
		t.Fatal(err)
	}
	loaded, err := LoadAnimation(path)
	if err != nil {
		t.Fatal(err)
	}
	if loaded.Frames[0].Delay != 70000 {
		t.Fatalf("70000ms 被改成 %dms", loaded.Frames[0].Delay)
	}
}

func TestRegressionStretchClipsToCell(t *testing.T) {
	placed := placePicture(image.NewNRGBA(image.Rect(0, 0, 8, 8)), 32, 32, CollageStretch, CollageCell{Scale: 2})
	if placed.Bounds().Dx() != 32 || placed.Bounds().Dy() != 32 {
		t.Fatalf("中间画布 %v 超出单格大小，圆角遮罩将作用于裁切前的画布", placed.Bounds())
	}
}
