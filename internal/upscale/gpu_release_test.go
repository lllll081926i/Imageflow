package upscale

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"image/color"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/disintegration/imaging"
	"golang.org/x/image/tiff"
)

func TestRealGPUReleaseAcceptance(t *testing.T) {
	if os.Getenv("IMAGEFLOW_GPU_RELEASE") != "1" {
		t.Skip("需要显式启用发布前真实 GPU 验收")
	}
	base := os.Getenv("IMAGEFLOW_GPU_DIRECTORY")
	directory := os.Getenv("IMAGEFLOW_GPU_RELEASE_DIRECTORY")
	if !filepath.IsAbs(base) || !filepath.IsAbs(directory) {
		t.Fatal("必须指定引擎目录和独立验收输出目录绝对路径")
	}
	if err := os.MkdirAll(directory, 0755); err != nil {
		t.Fatal(err)
	}
	engine := filepath.Join(base, "realesrgan-ncnn-vulkan.exe")
	root := filepath.Join(base, "trusted-models")
	fixtures := filepath.Join("..", "..", "docs", "Go_Image_Toolbox_Full_Delivery_v2.0", "assets", "fixtures")
	scenarios := []struct {
		name, input  string
		model, scale int
	}{
		{"screenshot-4x", "screenshot_text_1600x900.png", 0, 4},
		{"odd-anime-4x", "odd_1001x667_crop.png", 1, 4},
		{"alpha-video-2x", "alpha_512x512.png", 2, 2},
		{"icon-alpha-4x", "icon_alpha_256.png", 1, 4},
	}
	if photos := os.Getenv("IMAGEFLOW_GPU_PHOTO_FIXTURES"); photos != "" {
		photo, err := imaging.Open(filepath.Join(photos, "flowers.png"))
		if err != nil {
			t.Fatal(err)
		}
		photo = imaging.Fit(photo, 1024, 768, imaging.Lanczos)
		photoPath := filepath.Join(directory, "photo-input.tiff")
		file, err := os.Create(photoPath)
		if err != nil {
			t.Fatal(err)
		}
		encodeErr := tiff.Encode(file, photo, nil)
		closeErr := file.Close()
		if encodeErr != nil || closeErr != nil {
			t.Fatalf("生成 TIFF 输入失败：%v %v", encodeErr, closeErr)
		}
		scenarios = append(scenarios,
			struct {
				name, input  string
				model, scale int
			}{"photo-tiff-4x", photoPath, 0, 4},
			struct {
				name, input  string
				model, scale int
			}{"photo-exif-2x", filepath.Join(photos, "orientation_6.jpg"), 0, 2})
	}
	for _, scenario := range scenarios {
		t.Run(scenario.name, func(t *testing.T) {
			input := scenario.input
			if !filepath.IsAbs(input) {
				input = filepath.Join(fixtures, input)
			}
			input, err := filepath.Abs(input)
			if err != nil {
				t.Fatal(err)
			}
			output := filepath.Join(directory, scenario.name+".png")
			report, err := Run(context.Background(), Request{EnginePath: engine, ModelsRoot: root,
				Model: DownloadableModels()[scenario.model].Name, InputPath: input, OutputPath: output,
				Scale: scenario.scale, Tile: 128, GPU: 0, Format: "png", Timeout: 5 * time.Minute})
			if err != nil {
				t.Fatal(err)
			}
			if scenario.name == "alpha-video-2x" || scenario.name == "icon-alpha-4x" {
				original, err := imaging.Open(input)
				if err != nil {
					t.Fatal(err)
				}
				sourceMax := uint8(0)
				for row := 0; row < original.Bounds().Dy(); row++ {
					for column := 0; column < original.Bounds().Dx(); column++ {
						pixel := color.NRGBAModel.Convert(original.At(column, row)).(color.NRGBA)
						if pixel.A > sourceMax {
							sourceMax = pixel.A
						}
					}
				}
				decoded, err := imaging.Open(output)
				if err != nil {
					t.Fatal(err)
				}
				transparent, opaque, partial := false, false, false
				for row := 0; row < decoded.Bounds().Dy(); row++ {
					for column := 0; column < decoded.Bounds().Dx(); column++ {
						pixel := color.NRGBAModel.Convert(decoded.At(column, row)).(color.NRGBA)
						transparent = transparent || pixel.A == 0
						opaque = opaque || pixel.A == 255
						partial = partial || pixel.A > 0 && pixel.A < 255
					}
				}
				if !transparent || !partial || sourceMax == 255 && !opaque {
					t.Fatalf("透明通道信息不完整：透明=%t，不透明=%t，半透明=%t", transparent, opaque, partial)
				}
				t.Logf("源图最大 Alpha=%d，输出保留透明和半透明像素", sourceMax)
			}
			data, err := json.MarshalIndent(report, "", "  ")
			if err != nil {
				t.Fatal(err)
			}
			if err := os.WriteFile(filepath.Join(directory, scenario.name+".json"), data, 0600); err != nil {
				t.Fatal(err)
			}
			t.Logf("%dx%d，推理耗时 %s", report.Width, report.Height, report.Elapsed)
		})
	}
	t.Run("cancel-active-inference", func(t *testing.T) {
		input, err := filepath.Abs(filepath.Join(fixtures, "screenshot_text_1600x900.png"))
		if err != nil {
			t.Fatal(err)
		}
		target := filepath.Join(directory, "cancel-protected.png")
		before := []byte("取消验收的既有目标，不得覆盖")
		if err := os.WriteFile(target, before, 0600); err != nil {
			t.Fatal(err)
		}
		ctx, cancel := context.WithCancel(context.Background())
		defer cancel()
		finished := make(chan error, 1)
		go func() {
			_, err := Run(ctx, Request{EnginePath: engine, ModelsRoot: root,
				Model: DownloadableModels()[0].Name, InputPath: input, OutputPath: target,
				Scale: 4, Tile: 64, GPU: 0, TTA: true, Format: "png"})
			finished <- err
		}()
		time.Sleep(2 * time.Second)
		select {
		case err := <-finished:
			t.Fatalf("取消前已结束，无法证明活动推理取消：%v", err)
		default:
		}
		started := time.Now()
		cancel()
		select {
		case err := <-finished:
			var failure *Error
			if !errors.As(err, &failure) || failure.Code != CodeCancelled {
				t.Fatalf("取消错误码不正确：%v", err)
			}
		case <-time.After(10 * time.Second):
			t.Fatal("活动 GPU 推理未在 10 秒内取消")
		}
		after, err := os.ReadFile(target)
		if err != nil || !bytes.Equal(before, after) {
			t.Fatal("取消覆盖了既有目标")
		}
		leftovers, err := filepath.Glob(filepath.Join(directory, ".imageflow-upscale-*"))
		if err != nil || len(leftovers) != 0 {
			t.Fatalf("取消后遗留临时输出：%v %v", leftovers, err)
		}
		t.Logf("活动推理取消耗时 %s，旧目标保持不变", time.Since(started))
	})
}
