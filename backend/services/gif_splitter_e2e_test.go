package services

import (
	"image"
	"image/color/palette"
	"image/draw"
	"image/gif"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/imageflow/backend/models"
	"github.com/imageflow/backend/utils"
)

func TestSplitGIF_EndToEnd_ExportCompressResize(t *testing.T) {
	logger := newTestLogger(t)
	defer logger.Close()

	scriptsDir, err := utils.ResolvePythonScriptsDir()
	if err != nil {
		t.Skipf("skip e2e gif test: resolve scripts dir failed: %v", err)
	}

	executor, err := utils.NewPythonExecutor(scriptsDir, logger)
	if err != nil {
		t.Skipf("skip e2e gif test: python unavailable: %v", err)
	}
	defer executor.StopWorker()

	service := NewGIFSplitterService(executor, logger)
	tempDir := t.TempDir()
	inputPath := filepath.Join(tempDir, "sample.gif")
	if err := writeSampleGIF(inputPath); err != nil {
		t.Fatalf("failed to write sample gif: %v", err)
	}

	t.Run("export_frames", func(t *testing.T) {
		outputDir := filepath.Join(tempDir, "frames")
		res, err := service.SplitGIF(models.GIFSplitRequest{
			Action:       "export_frames",
			InputPath:    inputPath,
			OutputDir:    outputDir,
			OutputFormat: "png",
		})
		if shouldSkipGifE2E(err, res) {
			t.Skipf("skip e2e gif test (export): %v / %s", err, res.Error)
		}
		if err != nil {
			t.Fatalf("export failed: %v", err)
		}
		if !res.Success || res.ExportCount <= 0 {
			t.Fatalf("unexpected export result: %+v", res)
		}
	})

	t.Run("compress", func(t *testing.T) {
		outputPath := filepath.Join(tempDir, "compressed.gif")
		res, err := service.SplitGIF(models.GIFSplitRequest{
			Action:     "compress",
			InputPath:  inputPath,
			OutputPath: outputPath,
			Quality:    80,
		})
		if shouldSkipGifE2E(err, res) {
			t.Skipf("skip e2e gif test (compress): %v / %s", err, res.Error)
		}
		if err != nil {
			t.Fatalf("compress failed: %v", err)
		}
		if !res.Success {
			t.Fatalf("unexpected compress result: %+v", res)
		}
		if _, statErr := os.Stat(outputPath); statErr != nil {
			t.Fatalf("compressed output missing: %v", statErr)
		}
	})

	t.Run("resize", func(t *testing.T) {
		outputPath := filepath.Join(tempDir, "resized.gif")
		res, err := service.SplitGIF(models.GIFSplitRequest{
			Action:     "resize",
			InputPath:  inputPath,
			OutputPath: outputPath,
			Width:      8,
			MaintainAR: true,
		})
		if shouldSkipGifE2E(err, res) {
			t.Skipf("skip e2e gif test (resize): %v / %s", err, res.Error)
		}
		if err != nil {
			t.Fatalf("resize failed: %v", err)
		}
		if !res.Success || res.Width != 8 {
			t.Fatalf("unexpected resize result: %+v", res)
		}
		if _, statErr := os.Stat(outputPath); statErr != nil {
			t.Fatalf("resized output missing: %v", statErr)
		}
	})

	t.Run("convert_animation", func(t *testing.T) {
		outputPath := filepath.Join(tempDir, "converted.apng")
		res, err := service.SplitGIF(models.GIFSplitRequest{
			Action:       "convert_animation",
			InputPath:    inputPath,
			OutputPath:   outputPath,
			OutputFormat: "apng",
		})
		if shouldSkipGifE2E(err, res) {
			t.Skipf("skip e2e gif test (convert): %v / %s", err, res.Error)
		}
		if err != nil {
			t.Fatalf("convert failed: %v", err)
		}
		if !res.Success {
			t.Fatalf("unexpected convert result: %+v", res)
		}
		if _, statErr := os.Stat(outputPath); statErr != nil {
			t.Fatalf("converted output missing: %v", statErr)
		}
	})
}

func writeSampleGIF(path string) error {
	const w, h = 12, 12
	frameA := image.NewPaletted(image.Rect(0, 0, w, h), palette.Plan9)
	frameB := image.NewPaletted(image.Rect(0, 0, w, h), palette.Plan9)
	draw.Draw(frameA, frameA.Rect, &image.Uniform{C: palette.Plan9[2]}, image.Point{}, draw.Src)
	draw.Draw(frameB, frameB.Rect, &image.Uniform{C: palette.Plan9[6]}, image.Point{}, draw.Src)

	out, err := os.Create(path)
	if err != nil {
		return err
	}
	defer out.Close()

	return gif.EncodeAll(out, &gif.GIF{
		Image:     []*image.Paletted{frameA, frameB},
		Delay:     []int{8, 8},
		LoopCount: 0,
	})
}

func shouldSkipGifE2E(execErr error, res models.GIFSplitResult) bool {
	var msgParts []string
	if execErr != nil {
		msgParts = append(msgParts, execErr.Error())
	}
	if strings.TrimSpace(res.Error) != "" {
		msgParts = append(msgParts, res.Error)
	}
	if strings.TrimSpace(res.ErrorDetail) != "" {
		msgParts = append(msgParts, res.ErrorDetail)
	}
	all := strings.ToLower(strings.Join(msgParts, " | "))
	return strings.Contains(all, "no module named 'pil'") ||
		strings.Contains(all, "failed to find python") ||
		strings.Contains(all, "python worker is not running")
}
