package upscale

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestUpscaylTargetSize(t *testing.T) {
	for _, scenario := range []struct {
		width, height, scale, custom, wantWidth, wantHeight int
	}{
		{100, 75, 1, 0, 100, 75},
		{100, 75, 16, 0, 1600, 1200},
		{100, 75, 4, 333, 333, 250},
		{75, 100, 4, 150, 150, 200},
	} {
		width, height, err := targetSize(scenario.width, scenario.height, scenario.scale, scenario.custom)
		if err != nil || width != scenario.wantWidth || height != scenario.wantHeight {
			t.Fatalf("目标尺寸错误：%+v => %dx%d %v", scenario, width, height, err)
		}
	}
	for _, custom := range []int{-1, 16001, 1000000} {
		if _, _, err := targetSize(100, 75, 4, custom); err == nil {
			t.Fatalf("未拒绝超限宽度：%d", custom)
		}
	}
}

func TestUpscaylResourceSourcesAndCatalogue(t *testing.T) {
	for _, model := range UpscaylModels() {
		if isLegacyModel(model.Name) || model.Backend != "upscayl" || len(model.SHA256) != 64 {
			t.Fatalf("新目录模型不正确：%+v", model)
		}
	}
	for _, source := range []string{"github", "mirror"} {
		address, err := resourceURL("models/upscayl-lite-4x.bin", DownloadOptions{Source: source})
		if err != nil || !strings.Contains(address, UpscaylCommit) || !strings.HasPrefix(address, "https://") {
			t.Fatalf("下载源不正确：%s %v", address, err)
		}
		if source == "mirror" && !strings.HasPrefix(address, DefaultModelMirror) {
			t.Fatal("加速源未采用设置的前缀")
		}
	}
}

func TestUpscaylLiveMatrix(t *testing.T) {
	base := os.Getenv("IMAGEFLOW_UPSCAYL_AUDIT")
	if base == "" {
		t.Skip("需要显式指定已下载的 Upscayl 固定资源目录")
	}
	root := filepath.Join(base, "acceptance-models")
	engine := prepareUpscaylLive(t, root)
	models := UpscaylModels()
	input := os.Getenv("IMAGEFLOW_COMPONENT_LIVE_INPUT")
	if input == "" {
		t.Fatal("必须指定真实推理输入")
	}
	for _, model := range models {
		t.Run(model.Name, func(t *testing.T) {
			report, err := Run(context.Background(), Request{EnginePath: engine, ModelsRoot: root, Model: model.Name,
				InputPath: input, OutputPath: filepath.Join(base, model.Name+"-result.png"), Scale: 2, Tile: 64, GPU: 0, Format: "png"})
			if err != nil || report.Width != 256 || report.Height != 256 {
				t.Fatalf("七模型 GPU 验收失败：%+v %v", report, err)
			}
			t.Logf("真实 GPU：%dx%d，%s", report.Width, report.Height, report.Elapsed)
		})
	}
	for _, scenario := range []struct {
		name, format               string
		double                     bool
		custom, compression, width int
	}{
		{"double", "png", true, 0, 0, 512},
		{"width-jpg", "jpg", false, 333, 30, 333},
		{"width-webp", "webp", false, 123, 20, 123},
		{"double-width", "png", true, 257, 0, 257},
	} {
		t.Run(scenario.name, func(t *testing.T) {
			report, err := Run(context.Background(), Request{EnginePath: engine, ModelsRoot: root, Model: models[1].Name,
				InputPath: input, OutputPath: filepath.Join(base, scenario.name+"-result."+scenario.format), Scale: 2,
				Tile: 64, GPU: 0, Format: scenario.format, Double: scenario.double, CustomWidth: scenario.custom, Compression: scenario.compression})
			if err != nil || report.Width != scenario.width || report.Height != scenario.width {
				t.Fatalf("高级参数 GPU 验收失败：%+v %v", report, err)
			}
			t.Logf("真实 GPU：%dx%d，%s", report.Width, report.Height, report.Elapsed)
		})
	}
	for _, devices := range []string{"auto", "1", "0,1"} {
		t.Run("gpu-"+devices, func(t *testing.T) {
			report, err := Run(context.Background(), Request{EnginePath: engine, ModelsRoot: root, Model: models[1].Name,
				InputPath: input, OutputPath: filepath.Join(base, "gpu-"+devices+"-result.png"), Scale: 2,
				Tile: 64, GPUs: devices, Format: "png"})
			if err != nil || report.Width != 256 || report.Height != 256 {
				t.Fatalf("GPU 设备选择验收失败：%+v %v", report, err)
			}
			t.Logf("GPU %s：%dx%d，%s", devices, report.Width, report.Height, report.Elapsed)
		})
	}
}
