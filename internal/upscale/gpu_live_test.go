package upscale

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"image"
	"image/png"
	"os"
	"os/exec"
	"path/filepath"
	"testing"
	"time"

	"github.com/disintegration/imaging"
)

func TestRealGPUInference(t *testing.T) {
	if os.Getenv("IMAGEFLOW_GPU_LIVE") != "1" {
		t.Skip("需要显式启用真实 GPU 推理验证")
	}
	directory := os.Getenv("IMAGEFLOW_GPU_DIRECTORY")
	archivePath := os.Getenv("IMAGEFLOW_GPU_ARCHIVE")
	if !filepath.IsAbs(directory) || !filepath.IsAbs(archivePath) {
		t.Fatal("必须指定独立验收目录和已校验的官方归档绝对路径")
	}
	archive, err := os.ReadFile(archivePath)
	if err != nil {
		t.Fatal(err)
	}
	digest := sha256.Sum256(archive)
	if hex.EncodeToString(digest[:]) != ModelBundleSHA256 {
		t.Fatal("官方归档哈希不匹配")
	}
	modelsRoot := filepath.Join(directory, "trusted-models")
	for _, model := range DownloadableModels() {
		if err := installModelArchive(context.Background(), archivePath, modelsRoot, model); err != nil {
			t.Fatal(err)
		}
	}
	engine := filepath.Join(directory, "realesrgan-ncnn-vulkan.exe")
	environment := Detect(engine, modelsRoot)
	if !environment.Available {
		t.Fatal(environment.Reason)
	}
	source, err := imaging.Open(filepath.Join(directory, "input.jpg"))
	if err != nil {
		t.Fatal(err)
	}
	source = imaging.Fit(source, 128, 128, imaging.Lanczos)
	inputPath := filepath.Join(directory, "photo-input.png")
	if err := imaging.Save(source, inputPath); err != nil {
		t.Fatal(err)
	}
	for _, gpu := range []int{0, 1} {
		t.Run(fmt.Sprintf("engine-gpu-%d", gpu), func(t *testing.T) {
			ctx, cancel := context.WithTimeout(context.Background(), 2*time.Minute)
			defer cancel()
			cmd := exec.CommandContext(ctx, engine, "-i", inputPath,
				"-o", filepath.Join(directory, fmt.Sprintf("engine-gpu-%d.png", gpu)),
				"-m", DownloadableModels()[2].Dir(modelsRoot), "-n", "realesr-animevideov3",
				"-s", "2", "-t", "64", "-g", fmt.Sprint(gpu), "-f", "png", "-v")
			hideWindow(cmd)
			started := time.Now()
			log, runErr := cmd.CombinedOutput()
			if err := os.WriteFile(filepath.Join(directory, fmt.Sprintf("engine-gpu-%d.log", gpu)), log, 0600); err != nil {
				t.Fatal(err)
			}
			if runErr != nil {
				t.Fatalf("真实引擎失败：%v\n%s", runErr, log)
			}
			t.Logf("GPU %d 真实引擎耗时 %s\n%s", gpu, time.Since(started), log)
		})
	}
	for _, scenario := range []struct {
		name   string
		model  int
		scale  int
		format string
		gpu    int
		tta    bool
	}{
		{"photo-native", 0, 4, "png", 0, false},
		{"anime-native", 1, 4, "png", 0, false},
		{"video-native", 2, 2, "png", 0, false},
		{"photo-downscale-jpg", 0, 2, "jpg", 0, false},
		{"photo-downscale-webp", 0, 3, "webp", 0, false},
		{"video-tta", 2, 2, "png", 0, true},
		{"video-amd", 2, 2, "png", 1, false},
	} {
		t.Run(scenario.name, func(t *testing.T) {
			outputPath := filepath.Join(directory, scenario.name+"."+scenario.format)
			report, err := Run(context.Background(), Request{EnginePath: engine, ModelsRoot: modelsRoot,
				Model: DownloadableModels()[scenario.model].Name, InputPath: inputPath, OutputPath: outputPath,
				Scale: scenario.scale, Tile: 64, GPU: scenario.gpu, TTA: scenario.tta, Format: scenario.format,
				Timeout: 2 * time.Minute})
			if err != nil {
				t.Fatal(err)
			}
			decoded, err := imaging.Open(outputPath)
			if err != nil {
				t.Fatal(err)
			}
			expected := image.Rect(0, 0, source.Bounds().Dx()*scenario.scale, source.Bounds().Dy()*scenario.scale)
			if decoded.Bounds() != expected || report.Width != expected.Dx() || report.Height != expected.Dy() {
				t.Fatalf("输出尺寸错误：%v，报告：%+v", decoded.Bounds(), report)
			}
			data, err := json.MarshalIndent(report, "", "  ")
			if err != nil {
				t.Fatal(err)
			}
			if err := os.WriteFile(filepath.Join(directory, scenario.name+".json"), data, 0600); err != nil {
				t.Fatal(err)
			}
			t.Logf("%s：%dx%d，推理耗时 %s", report.Model, report.Width, report.Height, report.Elapsed)
		})
	}
	preview := imaging.New(source.Bounds().Dx()+source.Bounds().Dx()*4, source.Bounds().Dy()*4, image.Black)
	preview = imaging.Paste(preview, source, image.Pt(0, 0))
	result, err := imaging.Open(filepath.Join(directory, "photo-native.png"))
	if err != nil {
		t.Fatal(err)
	}
	preview = imaging.Paste(preview, result, image.Pt(source.Bounds().Dx(), 0))
	file, err := os.Create(filepath.Join(directory, "comparison.png"))
	if err != nil {
		t.Fatal(err)
	}
	encodeErr := png.Encode(file, preview)
	closeErr := file.Close()
	if encodeErr != nil || closeErr != nil {
		t.Fatalf("对比图写入失败：%v %v", encodeErr, closeErr)
	}
}
