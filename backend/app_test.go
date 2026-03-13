package main

import (
	"errors"
	"io/fs"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"sync"
	"sync/atomic"
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

func TestServiceNotReadyMessage_IsReadableChinese(t *testing.T) {
	got := serviceNotReadyMessage("格式转换服务")
	want := "格式转换服务未就绪，请重启应用后重试"
	if got != want {
		t.Fatalf("expected %q, got %q", want, got)
	}
}

func TestGetImagePreview_EmptyInputUsesReadableChinese(t *testing.T) {
	app := &App{}
	result, err := app.GetImagePreview(models.PreviewRequest{})
	if err == nil {
		t.Fatalf("expected error for empty input path")
	}
	if result.Error != "输入路径为空" {
		t.Fatalf("expected readable chinese error, got %q", result.Error)
	}
}

func TestListSystemFonts_DoesNotPanicWithoutLogger(t *testing.T) {
	app := &App{}
	defer func() {
		if recovered := recover(); recovered != nil {
			t.Fatalf("ListSystemFonts should not panic when logger is nil: %v", recovered)
		}
	}()
	_, _ = app.ListSystemFonts()
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

type fakeSerializedRunner struct {
	delay      time.Duration
	running    int32
	maxRunning int32
}

func newFakeSerializedRunner(delay time.Duration) *fakeSerializedRunner {
	return &fakeSerializedRunner{delay: delay}
}

func (r *fakeSerializedRunner) SetTimeout(timeout time.Duration) {}

func (r *fakeSerializedRunner) StartWorker() error { return nil }

func (r *fakeSerializedRunner) Execute(scriptName string, input interface{}) ([]byte, error) {
	return nil, errors.New("not implemented in fake runner")
}

func (r *fakeSerializedRunner) ExecuteAndParse(scriptName string, input interface{}, result interface{}) error {
	current := atomic.AddInt32(&r.running, 1)
	defer atomic.AddInt32(&r.running, -1)
	for {
		maxRunning := atomic.LoadInt32(&r.maxRunning)
		if current <= maxRunning {
			break
		}
		if atomic.CompareAndSwapInt32(&r.maxRunning, maxRunning, current) {
			break
		}
	}
	time.Sleep(r.delay)

	switch typed := result.(type) {
	case *models.ConvertResult:
		req, _ := input.(models.ConvertRequest)
		*typed = models.ConvertResult{
			Success:    true,
			InputPath:  req.InputPath,
			OutputPath: req.OutputPath,
		}
		return nil
	default:
		return errors.New("unexpected result type")
	}
}

func (r *fakeSerializedRunner) CancelActiveTask() {}

func (r *fakeSerializedRunner) StopWorker() {}

func TestConvert_SerializesTopLevelProcessing(t *testing.T) {
	logger, err := utils.NewLogger(utils.ErrorLevel, false)
	if err != nil {
		t.Fatalf("failed to create logger: %v", err)
	}
	defer logger.Close()

	runner := newFakeSerializedRunner(120 * time.Millisecond)
	app := &App{
		logger:   logger,
		executor: runner,
		settings: models.AppSettings{MaxConcurrency: 4},
	}
	app.converterService = services.NewConverterService(runner, logger)

	requests := []models.ConvertRequest{
		{InputPath: "a.jpg", OutputPath: "a.png", Format: "png"},
		{InputPath: "b.jpg", OutputPath: "b.png", Format: "png"},
	}

	var wg sync.WaitGroup
	errCh := make(chan error, len(requests))
	for _, req := range requests {
		wg.Add(1)
		go func(req models.ConvertRequest) {
			defer wg.Done()
			result, err := app.Convert(req)
			if err != nil {
				errCh <- err
				return
			}
			if !result.Success {
				errCh <- errors.New("expected successful conversion result")
			}
		}(req)
	}
	wg.Wait()
	close(errCh)
	for err := range errCh {
		if err != nil {
			t.Fatalf("unexpected convert error: %v", err)
		}
	}

	if got := atomic.LoadInt32(&runner.maxRunning); got != 1 {
		t.Fatalf("expected top-level processing to be serialized, max concurrent executions = %d", got)
	}
}

func TestConvert_RejectsInPlaceOverwriteWhenFormatChanges(t *testing.T) {
	logger, err := utils.NewLogger(utils.ErrorLevel, false)
	if err != nil {
		t.Fatalf("failed to create logger: %v", err)
	}
	defer logger.Close()

	runner := newFakeSerializedRunner(0)
	app := &App{
		logger:   logger,
		executor: runner,
		settings: models.AppSettings{MaxConcurrency: 1},
	}
	app.converterService = services.NewConverterService(runner, logger)

	result, err := app.Convert(models.ConvertRequest{
		InputPath:  "C:/tmp/source.png",
		OutputPath: "C:/tmp/source.png",
		Format:     "jpg",
	})
	if err != nil {
		t.Fatalf("expected app.Convert to normalize service error into result, got %v", err)
	}
	if result.Success {
		t.Fatalf("expected in-place format change to be rejected")
	}
	if !strings.Contains(result.Error, "不能直接覆盖源文件") {
		t.Fatalf("expected readable overwrite error, got %q", result.Error)
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

func TestSaveSettings_DoesNotPersistWhenRunnerRebuildFails(t *testing.T) {
	tmpConfig := t.TempDir()
	t.Setenv("APPDATA", tmpConfig)

	original := models.AppSettings{
		MaxConcurrency:          1,
		OutputPrefix:            "OLD",
		OutputTemplate:          "{basename}",
		PreserveFolderStructure: true,
		ConflictStrategy:        "rename",
	}
	if _, err := saveAppSettings(original); err != nil {
		t.Fatalf("failed to seed settings: %v", err)
	}

	logger, err := utils.NewLogger(utils.ErrorLevel, false)
	if err != nil {
		t.Fatalf("failed to create logger: %v", err)
	}
	defer logger.Close()

	app := &App{
		logger:     logger,
		scriptsDir: t.TempDir(),
		settings:   original,
	}

	previousBuilder := buildRunnerForSettingsFn
	buildRunnerForSettingsFn = func(_ string, _ *utils.Logger, _ models.AppSettings) (utils.PythonRunner, error) {
		return nil, errors.New("runner rebuild failed")
	}
	defer func() {
		buildRunnerForSettingsFn = previousBuilder
	}()

	next := original
	next.MaxConcurrency = 4
	next.OutputPrefix = "NEW"

	if _, err := app.SaveSettings(next); err == nil {
		t.Fatal("expected SaveSettings to fail")
	}

	reloaded, err := utils.LoadSettings()
	if err != nil {
		t.Fatalf("failed to reload settings: %v", err)
	}
	if reloaded.OutputPrefix != original.OutputPrefix {
		t.Fatalf("expected persisted prefix %q, got %q", original.OutputPrefix, reloaded.OutputPrefix)
	}
	if reloaded.MaxConcurrency != original.MaxConcurrency {
		t.Fatalf("expected persisted concurrency %d, got %d", original.MaxConcurrency, reloaded.MaxConcurrency)
	}
}

func TestUpdateRecentPaths_PersistsIncrementallyWithoutOverwritingOtherSettings(t *testing.T) {
	tmpConfig := t.TempDir()
	t.Setenv("APPDATA", tmpConfig)

	original := models.AppSettings{
		MaxConcurrency:          6,
		OutputPrefix:            "KEEP",
		OutputTemplate:          "{prefix}_{basename}",
		PreserveFolderStructure: false,
		ConflictStrategy:        "rename",
		DefaultOutputDir:        "D:/Exports",
		RecentInputDirs:         []string{"D:/OldInput"},
		RecentOutputDirs:        []string{"D:/OldOutput"},
	}
	if _, err := saveAppSettings(original); err != nil {
		t.Fatalf("failed to seed settings: %v", err)
	}

	app := &App{
		settings: original,
	}

	updated, err := app.UpdateRecentPaths(models.RecentPathsUpdateRequest{
		InputDir:  " D:/Shots/New/ ",
		OutputDir: " D:/Exports/New/ ",
	})
	if err != nil {
		t.Fatalf("expected nil error, got %v", err)
	}

	if updated.MaxConcurrency != original.MaxConcurrency {
		t.Fatalf("expected max concurrency %d, got %d", original.MaxConcurrency, updated.MaxConcurrency)
	}
	if updated.OutputPrefix != original.OutputPrefix {
		t.Fatalf("expected output prefix %q, got %q", original.OutputPrefix, updated.OutputPrefix)
	}
	if updated.DefaultOutputDir != original.DefaultOutputDir {
		t.Fatalf("expected default output dir %q, got %q", original.DefaultOutputDir, updated.DefaultOutputDir)
	}

	wantInputs := []string{"D:/Shots/New", "D:/OldInput"}
	if len(updated.RecentInputDirs) != len(wantInputs) {
		t.Fatalf("expected %d input dirs, got %d", len(wantInputs), len(updated.RecentInputDirs))
	}
	for i, want := range wantInputs {
		if updated.RecentInputDirs[i] != want {
			t.Fatalf("expected input dir %d to be %q, got %q", i, want, updated.RecentInputDirs[i])
		}
	}

	wantOutputs := []string{"D:/Exports/New", "D:/OldOutput"}
	if len(updated.RecentOutputDirs) != len(wantOutputs) {
		t.Fatalf("expected %d output dirs, got %d", len(wantOutputs), len(updated.RecentOutputDirs))
	}
	for i, want := range wantOutputs {
		if updated.RecentOutputDirs[i] != want {
			t.Fatalf("expected output dir %d to be %q, got %q", i, want, updated.RecentOutputDirs[i])
		}
	}

	reloaded, err := utils.LoadSettings()
	if err != nil {
		t.Fatalf("failed to reload settings: %v", err)
	}
	if reloaded.OutputTemplate != original.OutputTemplate {
		t.Fatalf("expected output template %q, got %q", original.OutputTemplate, reloaded.OutputTemplate)
	}
	if reloaded.DefaultOutputDir != original.DefaultOutputDir {
		t.Fatalf("expected persisted default output dir %q, got %q", original.DefaultOutputDir, reloaded.DefaultOutputDir)
	}
	if reloaded.RecentInputDirs[0] != "D:/Shots/New" {
		t.Fatalf("expected latest recent input dir to persist, got %q", reloaded.RecentInputDirs[0])
	}
}

func TestEmbeddedPythonRuntimeIsBundledOnWindows(t *testing.T) {
	if runtime.GOOS != "windows" {
		t.Skip("embedded runtime is only bundled on Windows builds")
	}
	if _, err := fs.Stat(embeddedPythonFS, "embedded_python_runtime/python.exe"); err != nil {
		t.Fatalf("expected embedded python runtime payload, got error: %v", err)
	}
}
