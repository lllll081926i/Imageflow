package main

import (
	"errors"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/imageflow/backend/models"
	"github.com/imageflow/backend/services"
	"github.com/imageflow/backend/utils"
)

func TestGetPreviewMaxBytes_Default(t *testing.T) {
	t.Setenv("IMAGEFLOW_PREVIEW_MAX_BYTES", "")
	got := getPreviewMaxBytes()
	if got != defaultPreviewMaxBytes {
		t.Fatalf("expected %d, got %d", defaultPreviewMaxBytes, got)
	}
}

func TestGetPreviewMaxBytes_Custom(t *testing.T) {
	t.Setenv("IMAGEFLOW_PREVIEW_MAX_BYTES", "12345")
	got := getPreviewMaxBytes()
	if got != 12345 {
		t.Fatalf("expected 12345, got %d", got)
	}
}

func TestDetectPreviewMimeType_Fallback(t *testing.T) {
	data := []byte{0x00, 0x01, 0x02}
	got := detectPreviewMimeType(data, "sample.png")
	if got != "image/png" {
		t.Fatalf("expected image/png, got %s", got)
	}
}

func TestBuildDataURL(t *testing.T) {
	data := []byte("abc")
	got := buildDataURL(data, "image/png")
	if !strings.HasPrefix(got, "data:image/png;base64,") {
		t.Fatalf("unexpected data url prefix: %s", got)
	}
}

func TestGetPreviewCacheCap_Default(t *testing.T) {
	t.Setenv("IMAGEFLOW_PREVIEW_CACHE_ENTRIES", "")
	if got := getPreviewCacheCap(); got != defaultPreviewCacheCap {
		t.Fatalf("expected default cache cap %d, got %d", defaultPreviewCacheCap, got)
	}
}

func TestGetImagePreview_CacheHitAndInvalidation(t *testing.T) {
	tmpDir := t.TempDir()
	path := filepath.Join(tmpDir, "sample.png")

	initial := []byte{0x89, 'P', 'N', 'G', 0x0d, 0x0a, 0x1a, 0x0a, 0x01, 0x02, 0x03}
	if err := os.WriteFile(path, initial, 0o644); err != nil {
		t.Fatalf("failed to write sample file: %v", err)
	}

	app := &App{}
	first, err := app.GetImagePreview(models.PreviewRequest{InputPath: path})
	if err != nil {
		t.Fatalf("first preview failed: %v", err)
	}
	if !first.Success || first.DataURL == "" {
		t.Fatalf("first preview should succeed with data url")
	}
	if len(app.previewCache) != 1 {
		t.Fatalf("expected cache size 1 after first preview, got %d", len(app.previewCache))
	}

	second, err := app.GetImagePreview(models.PreviewRequest{InputPath: path})
	if err != nil {
		t.Fatalf("second preview failed: %v", err)
	}
	if !second.Success || second.DataURL == "" {
		t.Fatalf("second preview should succeed with data url")
	}
	if second.DataURL != first.DataURL {
		t.Fatalf("expected cache hit to keep same data url")
	}

	updated := []byte{0x89, 'P', 'N', 'G', 0x0d, 0x0a, 0x1a, 0x0a, 0x10, 0x20, 0x30, 0x40, 0x50}
	if err := os.WriteFile(path, updated, 0o644); err != nil {
		t.Fatalf("failed to rewrite sample file: %v", err)
	}

	third, err := app.GetImagePreview(models.PreviewRequest{InputPath: path})
	if err != nil {
		t.Fatalf("third preview failed: %v", err)
	}
	if !third.Success || third.DataURL == "" {
		t.Fatalf("third preview should succeed with data url")
	}
	if third.DataURL == second.DataURL {
		t.Fatalf("expected cache invalidation after file content change")
	}
	if len(app.previewCache) != 1 {
		t.Fatalf("expected cache size 1 after invalidation refresh, got %d", len(app.previewCache))
	}
}

type fakeCancelableRunner struct {
	delay    time.Duration
	cancelCh chan struct{}
	once     sync.Once
}

func newFakeCancelableRunner(delay time.Duration) *fakeCancelableRunner {
	return &fakeCancelableRunner{
		delay:    delay,
		cancelCh: make(chan struct{}),
	}
}

func (r *fakeCancelableRunner) SetTimeout(timeout time.Duration) {}

func (r *fakeCancelableRunner) StartWorker() error { return nil }

func (r *fakeCancelableRunner) Execute(scriptName string, input interface{}) ([]byte, error) {
	return nil, errors.New("not implemented in fake runner")
}

func (r *fakeCancelableRunner) ExecuteAndParse(scriptName string, input interface{}, result interface{}) error {
	select {
	case <-time.After(r.delay):
	case <-r.cancelCh:
		return errors.New(cancelledErrorMessage)
	}

	switch scriptName {
	case "watermark.py":
		out, ok := result.(*models.WatermarkResult)
		if !ok {
			return errors.New("unexpected watermark result type")
		}
		payload, ok := input.(map[string]interface{})
		if !ok {
			return errors.New("unexpected watermark input payload")
		}
		out.Success = true
		if v, ok := payload["input_path"].(string); ok {
			out.InputPath = v
		}
		if v, ok := payload["output_path"].(string); ok {
			out.OutputPath = v
		}
		return nil
	case "adjuster.py":
		out, ok := result.(*models.AdjustResult)
		if !ok {
			return errors.New("unexpected adjust result type")
		}
		req, ok := input.(models.AdjustRequest)
		if !ok {
			return errors.New("unexpected adjust input payload")
		}
		out.Success = true
		out.InputPath = req.InputPath
		out.OutputPath = req.OutputPath
		return nil
	case "filter.py":
		out, ok := result.(*models.FilterResult)
		if !ok {
			return errors.New("unexpected filter result type")
		}
		payload, ok := input.(map[string]interface{})
		if !ok {
			return errors.New("unexpected filter input payload")
		}
		out.Success = true
		if v, ok := payload["input_path"].(string); ok {
			out.InputPath = v
		}
		if v, ok := payload["output_path"].(string); ok {
			out.OutputPath = v
		}
		return nil
	default:
		return errors.New("unexpected script name")
	}
}

func (r *fakeCancelableRunner) CancelActiveTask() {
	r.once.Do(func() {
		close(r.cancelCh)
	})
}

func (r *fakeCancelableRunner) StopWorker() {}

func setupCancelableApp(t *testing.T) *App {
	t.Helper()
	logger, err := utils.NewLogger(utils.ErrorLevel, false)
	if err != nil {
		t.Fatalf("failed to create logger: %v", err)
	}
	t.Cleanup(func() {
		logger.Close()
	})

	runner := newFakeCancelableRunner(80 * time.Millisecond)
	app := &App{
		logger:   logger,
		executor: runner,
		settings: models.AppSettings{MaxConcurrency: 1},
	}
	app.watermarkService = services.NewWatermarkService(runner, logger)
	app.adjusterService = services.NewAdjusterService(runner, logger)
	app.filterService = services.NewFilterService(runner, logger)
	return app
}

func assertContainsCancelledResult[T interface {
	models.WatermarkResult | models.AdjustResult | models.FilterResult
}](t *testing.T, results []T) {
	t.Helper()
	cancelled := 0
	success := 0
	for _, item := range results {
		switch v := any(item).(type) {
		case models.WatermarkResult:
			if v.Error == cancelledErrorMessage {
				cancelled++
			}
			if v.Success {
				success++
			}
		case models.AdjustResult:
			if v.Error == cancelledErrorMessage {
				cancelled++
			}
			if v.Success {
				success++
			}
		case models.FilterResult:
			if v.Error == cancelledErrorMessage {
				cancelled++
			}
			if v.Success {
				success++
			}
		}
	}
	if cancelled == 0 {
		t.Fatalf("expected at least one cancelled result")
	}
	if success == 0 {
		t.Fatalf("expected at least one successful result before cancellation")
	}
}

func TestAddWatermarkBatch_RespectsCancellation(t *testing.T) {
	app := setupCancelableApp(t)
	requests := []models.WatermarkRequest{
		{InputPath: "a.jpg", OutputPath: "a_out.jpg", WatermarkType: "text"},
		{InputPath: "b.jpg", OutputPath: "b_out.jpg", WatermarkType: "text"},
		{InputPath: "c.jpg", OutputPath: "c_out.jpg", WatermarkType: "text"},
		{InputPath: "d.jpg", OutputPath: "d_out.jpg", WatermarkType: "text"},
	}

	go func() {
		time.Sleep(120 * time.Millisecond)
		app.CancelProcessing()
	}()

	results, err := app.AddWatermarkBatch(requests)
	if err != nil {
		t.Fatalf("expected nil error, got %v", err)
	}
	if len(results) != len(requests) {
		t.Fatalf("expected %d results, got %d", len(requests), len(results))
	}
	assertContainsCancelledResult(t, results)
}

func TestAdjustBatch_RespectsCancellation(t *testing.T) {
	app := setupCancelableApp(t)
	requests := []models.AdjustRequest{
		{InputPath: "a.jpg", OutputPath: "a_out.jpg"},
		{InputPath: "b.jpg", OutputPath: "b_out.jpg"},
		{InputPath: "c.jpg", OutputPath: "c_out.jpg"},
		{InputPath: "d.jpg", OutputPath: "d_out.jpg"},
	}

	go func() {
		time.Sleep(120 * time.Millisecond)
		app.CancelProcessing()
	}()

	results, err := app.AdjustBatch(requests)
	if err != nil {
		t.Fatalf("expected nil error, got %v", err)
	}
	if len(results) != len(requests) {
		t.Fatalf("expected %d results, got %d", len(requests), len(results))
	}
	assertContainsCancelledResult(t, results)
}

func TestApplyFilterBatch_RespectsCancellation(t *testing.T) {
	app := setupCancelableApp(t)
	requests := []models.FilterRequest{
		{InputPath: "a.jpg", OutputPath: "a_out.jpg", FilterType: "none"},
		{InputPath: "b.jpg", OutputPath: "b_out.jpg", FilterType: "none"},
		{InputPath: "c.jpg", OutputPath: "c_out.jpg", FilterType: "none"},
		{InputPath: "d.jpg", OutputPath: "d_out.jpg", FilterType: "none"},
	}

	go func() {
		time.Sleep(120 * time.Millisecond)
		app.CancelProcessing()
	}()

	results, err := app.ApplyFilterBatch(requests)
	if err != nil {
		t.Fatalf("expected nil error, got %v", err)
	}
	if len(results) != len(requests) {
		t.Fatalf("expected %d results, got %d", len(requests), len(results))
	}
	assertContainsCancelledResult(t, results)
}
