package services

import (
	"strings"
	"testing"

	"github.com/imageflow/backend/models"
)

func TestNonGIFServicesRejectGIFInputs(t *testing.T) {
	logger := newTestLogger(t)
	defer logger.Close()

	tests := []struct {
		name       string
		run        func(runner *mockPythonRunner) (string, error)
		wantErrMsg string
	}{
		{
			name: "converter",
			run: func(runner *mockPythonRunner) (string, error) {
				service := NewConverterService(runner, logger)
				result, err := service.Convert(models.ConvertRequest{InputPath: "animated.gif", OutputPath: "out.png", Format: "png"})
				return result.Error, err
			},
		},
		{
			name: "compressor",
			run: func(runner *mockPythonRunner) (string, error) {
				service := NewCompressorService(runner, logger)
				result, err := service.Compress(models.CompressRequest{InputPath: "animated.gif", OutputPath: "out.gif", Level: 3})
				return result.Error, err
			},
		},
		{
			name: "watermark",
			run: func(runner *mockPythonRunner) (string, error) {
				service := NewWatermarkService(runner, logger)
				result, err := service.AddWatermark(models.WatermarkRequest{InputPath: "animated.gif", OutputPath: "out.gif", WatermarkType: "text"})
				return result.Error, err
			},
		},
		{
			name: "adjust",
			run: func(runner *mockPythonRunner) (string, error) {
				service := NewAdjusterService(runner, logger)
				result, err := service.Adjust(models.AdjustRequest{InputPath: "animated.gif", OutputPath: "out.gif"})
				return result.Error, err
			},
		},
		{
			name: "filter",
			run: func(runner *mockPythonRunner) (string, error) {
				service := NewFilterService(runner, logger)
				result, err := service.ApplyFilter(models.FilterRequest{InputPath: "animated.gif", OutputPath: "out.gif", FilterType: "none"})
				return result.Error, err
			},
		},
		{
			name: "pdf",
			run: func(runner *mockPythonRunner) (string, error) {
				service := NewPDFGeneratorService(runner, logger)
				result, err := service.GeneratePDF(models.PDFRequest{ImagePaths: []string{"cover.png", "animated.gif"}, OutputPath: "out.pdf"})
				return result.Error, err
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			called := false
			runner := &mockPythonRunner{
				executeAndParseFn: func(scriptName string, input interface{}, result interface{}) error {
					called = true
					return nil
				},
			}

			errMsg, err := tt.run(runner)
			if err == nil {
				t.Fatal("expected error for gif input")
			}
			if called {
				t.Fatal("expected runner not to be called for gif input")
			}
			if !strings.Contains(err.Error(), "GIF") {
				t.Fatalf("expected GIF error, got %v", err)
			}
			if !strings.Contains(errMsg, "GIF") {
				t.Fatalf("expected result error to mention GIF, got %q", errMsg)
			}
		})
	}
}

func TestServicesRejectParentTraversalPaths(t *testing.T) {
	logger := newTestLogger(t)
	defer logger.Close()

	tests := []struct {
		name string
		run  func(runner *mockPythonRunner) error
	}{
		{
			name: "converter input",
			run: func(runner *mockPythonRunner) error {
				service := NewConverterService(runner, logger)
				_, err := service.Convert(models.ConvertRequest{
					InputPath:  "../secret.png",
					OutputPath: "out.png",
					Format:     "png",
				})
				return err
			},
		},
		{
			name: "watermark image",
			run: func(runner *mockPythonRunner) error {
				service := NewWatermarkService(runner, logger)
				_, err := service.AddWatermark(models.WatermarkRequest{
					InputPath:     "input.png",
					OutputPath:    "out.png",
					WatermarkType: "image",
					ImagePath:     "../wm.png",
				})
				return err
			},
		},
		{
			name: "pdf image list",
			run: func(runner *mockPythonRunner) error {
				service := NewPDFGeneratorService(runner, logger)
				_, err := service.GeneratePDF(models.PDFRequest{
					ImagePaths: []string{"cover.png", "../secret.png"},
					OutputPath: "out.pdf",
				})
				return err
			},
		},
		{
			name: "subtitle stitch inputs",
			run: func(runner *mockPythonRunner) error {
				service := NewSubtitleStitchService(runner, logger)
				_, err := service.Generate(models.SubtitleStitchRequest{
					InputPaths: []string{"frame1.png", "../frame2.png"},
					OutputPath: "out.png",
				})
				return err
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			called := false
			runner := &mockPythonRunner{
				executeAndParseFn: func(scriptName string, input interface{}, result interface{}) error {
					called = true
					return nil
				},
			}

			err := tt.run(runner)
			if err == nil {
				t.Fatal("expected parent traversal path to be rejected")
			}
			if called {
				t.Fatal("expected runner not to be called for invalid path")
			}
			if !strings.Contains(err.Error(), "父级目录") {
				t.Fatalf("expected traversal error, got %v", err)
			}
		})
	}
}
