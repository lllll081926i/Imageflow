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

func TestDetectPreviewMimeType_HEICAndHEIFFallback(t *testing.T) {
	data := []byte{0x00, 0x01, 0x02}

	heic := detectPreviewMimeType(data, "sample.heic")
	if heic != "image/heic" {
		t.Fatalf("expected image/heic, got %s", heic)
	}

	heif := detectPreviewMimeType(data, "sample.heif")
	if heif != "image/heif" {
		t.Fatalf("expected image/heif, got %s", heif)
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

func TestPreviewCache_EvictsWhenTotalByteBudgetExceeded(t *testing.T) {
	t.Setenv("IMAGEFLOW_PREVIEW_CACHE_MAX_BYTES", "100")

	tmpDir := t.TempDir()
	path := filepath.Join(tmpDir, "sample.png")
	if err := os.WriteFile(path, []byte("png"), 0o644); err != nil {
		t.Fatalf("failed to write sample file: %v", err)
	}
	info, err := os.Stat(path)
	if err != nil {
		t.Fatalf("failed to stat sample file: %v", err)
	}

	app := &App{}
	app.setPreviewCacheEntry("a", info, strings.Repeat("a", 70))
	if len(app.previewCache) != 1 {
		t.Fatalf("expected first cache entry to be stored, got %d", len(app.previewCache))
	}

	app.setPreviewCacheEntry("b", info, strings.Repeat("b", 70))
	if len(app.previewCache) != 1 {
		t.Fatalf("expected cache to evict oldest entry when byte budget exceeded, got %d entries", len(app.previewCache))
	}
	if _, ok := app.previewCache["a"]; ok {
		t.Fatalf("expected oldest cache entry to be evicted")
	}
	if _, ok := app.previewCache["b"]; !ok {
		t.Fatalf("expected newest cache entry to be retained")
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

func TestGetImagePreview_RejectsNonImageFiles(t *testing.T) {
	tmpDir := t.TempDir()
	path := filepath.Join(tmpDir, "notes.txt")
	if err := os.WriteFile(path, []byte("plain text"), 0o644); err != nil {
		t.Fatalf("failed to write sample text file: %v", err)
	}

	app := &App{}
	result, err := app.GetImagePreview(models.PreviewRequest{InputPath: path})
	if err == nil {
		t.Fatalf("expected preview request to reject non-image file")
	}
	if result.Success {
		t.Fatalf("expected preview result to fail for non-image file")
	}
	if !strings.Contains(result.Error, "不支持预览") {
		t.Fatalf("expected readable unsupported preview error, got %q", result.Error)
	}
}

func TestGetImagePreview_RejectsSpoofedImageExtension(t *testing.T) {
	tmpDir := t.TempDir()
	path := filepath.Join(tmpDir, "fake.png")
	if err := os.WriteFile(path, []byte("this is not a real png"), 0o644); err != nil {
		t.Fatalf("failed to write spoofed image file: %v", err)
	}

	app := &App{}
	result, err := app.GetImagePreview(models.PreviewRequest{InputPath: path})
	if err == nil {
		t.Fatalf("expected preview request to reject spoofed image file")
	}
	if result.Success {
		t.Fatalf("expected preview result to fail for spoofed image file")
	}
	if !strings.Contains(result.Error, "不支持预览") {
		t.Fatalf("expected readable unsupported preview error, got %q", result.Error)
	}
}

func TestGetImagePreview_RejectsParentTraversalPath(t *testing.T) {
	app := &App{}
	result, err := app.GetImagePreview(models.PreviewRequest{InputPath: "../secret.png"})
	if err == nil {
		t.Fatal("expected parent traversal preview path to be rejected")
	}
	if result.Success {
		t.Fatalf("expected preview to fail for traversal path, got %+v", result)
	}
	if !strings.Contains(result.Error, "父级目录") {
		t.Fatalf("expected traversal error, got %q", result.Error)
	}
}

func TestReadPreviewHeader_EmptyFileReturnsReadableError(t *testing.T) {
	tmpDir := t.TempDir()
	path := filepath.Join(tmpDir, "empty.png")
	if err := os.WriteFile(path, nil, 0o644); err != nil {
		t.Fatalf("failed to create empty file: %v", err)
	}

	_, err := readPreviewHeader(path)
	if err == nil {
		t.Fatal("expected empty preview file to be rejected")
	}
	if !strings.Contains(err.Error(), "文件为空") {
		t.Fatalf("expected empty file error, got %q", err.Error())
	}
}

func TestResolveOutputPath_RejectsParentTraversalBasePath(t *testing.T) {
	app := &App{}
	result, err := app.ResolveOutputPath(models.ResolveOutputPathRequest{BasePath: "../out.png"})
	if err == nil {
		t.Fatal("expected traversal base path to be rejected")
	}
	if result.Success {
		t.Fatalf("expected resolve output path to fail, got %+v", result)
	}
	if !strings.Contains(result.Error, "父级目录") {
		t.Fatalf("expected traversal error, got %q", result.Error)
	}
}

type fakePreviewConversionRunner struct {
	convertCalls int32
}

func (r *fakePreviewConversionRunner) SetTimeout(timeout time.Duration) {}

func (r *fakePreviewConversionRunner) StartWorker() error { return nil }

func (r *fakePreviewConversionRunner) Execute(scriptName string, input interface{}) ([]byte, error) {
	return nil, errors.New("not implemented in fake runner")
}

func (r *fakePreviewConversionRunner) ExecuteAndParse(scriptName string, input interface{}, result interface{}) error {
	if scriptName != "converter.py" {
		return errors.New("unexpected script name")
	}

	req, ok := input.(models.ConvertRequest)
	if !ok {
		return errors.New("unexpected convert request payload")
	}
	if err := os.WriteFile(req.OutputPath, []byte{
		0xFF, 0xD8, 0xFF, 0xE0, 0x00, 0x10, 'J', 'F', 'I', 'F', 0x00,
	}, 0o644); err != nil {
		return err
	}
	atomic.AddInt32(&r.convertCalls, 1)

	out, ok := result.(*models.ConvertResult)
	if !ok {
		return errors.New("unexpected convert result type")
	}
	*out = models.ConvertResult{
		Success:    true,
		InputPath:  req.InputPath,
		OutputPath: req.OutputPath,
	}
	return nil
}

func (r *fakePreviewConversionRunner) CancelActiveTask() {}

func (r *fakePreviewConversionRunner) StopWorker() {}

func TestGetImagePreview_ConvertsSmallHEICToJPEGPreview(t *testing.T) {
	logger, err := utils.NewLogger(utils.ErrorLevel, false)
	if err != nil {
		t.Fatalf("failed to create logger: %v", err)
	}
	defer logger.Close()

	tmpDir := t.TempDir()
	path := filepath.Join(tmpDir, "sample.heic")
	if err := os.WriteFile(path, []byte{
		0x00, 0x00, 0x00, 0x18, 'f', 't', 'y', 'p', 'h', 'e', 'i', 'c', 0x00, 0x00, 0x00, 0x00,
	}, 0o644); err != nil {
		t.Fatalf("failed to write sample heic file: %v", err)
	}

	runner := &fakePreviewConversionRunner{}
	app := &App{
		logger:   logger,
		executor: runner,
		settings: models.AppSettings{MaxConcurrency: 1},
	}
	app.converterService = services.NewConverterService(runner, logger)

	result, err := app.GetImagePreview(models.PreviewRequest{InputPath: path})
	if err != nil {
		t.Fatalf("expected preview generation to succeed, got %v", err)
	}
	if !result.Success {
		t.Fatalf("expected preview generation to succeed, got %+v", result)
	}
	if !strings.HasPrefix(result.DataURL, "data:image/jpeg;base64,") {
		t.Fatalf("expected HEIC preview to be converted to JPEG data url, got %q", result.DataURL)
	}
	if atomic.LoadInt32(&runner.convertCalls) != 1 {
		t.Fatalf("expected converter to be used once, got %d", runner.convertCalls)
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

func TestGetImagePreview_CacheHitSkipsHeaderRead(t *testing.T) {
	tmpDir := t.TempDir()
	path := filepath.Join(tmpDir, "sample.png")
	if err := os.WriteFile(path, []byte{
		0x89, 'P', 'N', 'G', 0x0d, 0x0a, 0x1a, 0x0a, 0x01, 0x02, 0x03,
	}, 0o644); err != nil {
		t.Fatalf("failed to write sample png file: %v", err)
	}

	app := NewApp()
	originalReadPreviewHeaderFn := app.readPreviewHeaderFn
	var headerReads int32
	app.readPreviewHeaderFn = func(inputPath string) (string, error) {
		atomic.AddInt32(&headerReads, 1)
		return originalReadPreviewHeaderFn(inputPath)
	}
	first, err := app.GetImagePreview(models.PreviewRequest{InputPath: path})
	if err != nil {
		t.Fatalf("first preview failed: %v", err)
	}
	if !first.Success || first.DataURL == "" {
		t.Fatalf("expected first preview to succeed")
	}
	if got := atomic.LoadInt32(&headerReads); got != 1 {
		t.Fatalf("expected first preview to read header once, got %d", got)
	}

	second, err := app.GetImagePreview(models.PreviewRequest{InputPath: path})
	if err != nil {
		t.Fatalf("second preview failed: %v", err)
	}
	if !second.Success || second.DataURL == "" {
		t.Fatalf("expected second preview to succeed")
	}
	if got := atomic.LoadInt32(&headerReads); got != 1 {
		t.Fatalf("expected cached preview to skip header reread, got %d reads", got)
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

func TestConvert_AllowsConcurrentTopLevelProcessing(t *testing.T) {
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

	if got := atomic.LoadInt32(&runner.maxRunning); got < 2 {
		t.Fatalf("expected top-level processing to allow concurrency, max concurrent executions = %d", got)
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

type fakeIdleCancelRunner struct {
	mu          sync.Mutex
	delay       time.Duration
	active      bool
	cancelCh    chan struct{}
	staleCancel bool
	startedCh   chan struct{}
}

func newFakeIdleCancelRunner(delay time.Duration) *fakeIdleCancelRunner {
	return &fakeIdleCancelRunner{
		delay:     delay,
		startedCh: make(chan struct{}, 1),
	}
}

func (r *fakeIdleCancelRunner) SetTimeout(timeout time.Duration) {}

func (r *fakeIdleCancelRunner) StartWorker() error { return nil }

func (r *fakeIdleCancelRunner) Execute(scriptName string, input interface{}) ([]byte, error) {
	return nil, errors.New("not implemented in fake runner")
}

func (r *fakeIdleCancelRunner) ExecuteAndParse(scriptName string, input interface{}, result interface{}) error {
	r.mu.Lock()
	if r.staleCancel {
		r.staleCancel = false
		r.mu.Unlock()
		return errors.New(cancelledErrorMessage)
	}
	cancelCh := make(chan struct{})
	r.cancelCh = cancelCh
	r.active = true
	r.mu.Unlock()

	select {
	case r.startedCh <- struct{}{}:
	default:
	}

	select {
	case <-time.After(r.delay):
	case <-cancelCh:
		r.mu.Lock()
		r.active = false
		r.cancelCh = nil
		r.mu.Unlock()
		return errors.New(cancelledErrorMessage)
	}

	r.mu.Lock()
	r.active = false
	r.cancelCh = nil
	r.mu.Unlock()

	out, ok := result.(*models.ConvertResult)
	if !ok {
		return errors.New("unexpected convert result type")
	}
	req, ok := input.(models.ConvertRequest)
	if !ok {
		return errors.New("unexpected convert request payload")
	}
	*out = models.ConvertResult{
		Success:    true,
		InputPath:  req.InputPath,
		OutputPath: req.OutputPath,
	}
	return nil
}

func (r *fakeIdleCancelRunner) CancelActiveTask() {
	r.mu.Lock()
	defer r.mu.Unlock()
	if r.active && r.cancelCh != nil {
		close(r.cancelCh)
		return
	}
	r.staleCancel = true
}

func (r *fakeIdleCancelRunner) StopWorker() {}

type fakeSaveSettingsRunner struct {
	delay              time.Duration
	allowFinish        chan struct{}
	startedCh          chan struct{}
	mu                 sync.Mutex
	active             bool
	stopCount          int
	stoppedWhileActive bool
}

func newFakeSaveSettingsRunner(delay time.Duration) *fakeSaveSettingsRunner {
	return &fakeSaveSettingsRunner{
		delay:       delay,
		allowFinish: make(chan struct{}),
		startedCh:   make(chan struct{}, 1),
	}
}

func (r *fakeSaveSettingsRunner) SetTimeout(timeout time.Duration) {}

func (r *fakeSaveSettingsRunner) StartWorker() error { return nil }

func (r *fakeSaveSettingsRunner) Execute(scriptName string, input interface{}) ([]byte, error) {
	return nil, errors.New("not implemented in fake runner")
}

func (r *fakeSaveSettingsRunner) ExecuteAndParse(scriptName string, input interface{}, result interface{}) error {
	r.mu.Lock()
	r.active = true
	r.mu.Unlock()

	select {
	case r.startedCh <- struct{}{}:
	default:
	}

	select {
	case <-r.allowFinish:
	case <-time.After(r.delay):
	}

	r.mu.Lock()
	r.active = false
	r.mu.Unlock()

	out, ok := result.(*models.ConvertResult)
	if !ok {
		return errors.New("unexpected convert result type")
	}
	req, ok := input.(models.ConvertRequest)
	if !ok {
		return errors.New("unexpected convert request payload")
	}
	*out = models.ConvertResult{
		Success:    true,
		InputPath:  req.InputPath,
		OutputPath: req.OutputPath,
	}
	return nil
}

func (r *fakeSaveSettingsRunner) CancelActiveTask() {}

func (r *fakeSaveSettingsRunner) StopWorker() {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.stopCount++
	if r.active {
		r.stoppedWhileActive = true
	}
}

func (r *fakeSaveSettingsRunner) snapshot() (stopCount int, stoppedWhileActive bool) {
	r.mu.Lock()
	defer r.mu.Unlock()
	return r.stopCount, r.stoppedWhileActive
}

type fakeMetadataRunner struct {
	scriptName string
	payload    map[string]interface{}
	result     models.MetadataStripResult
	err        error
}

func (r *fakeMetadataRunner) SetTimeout(timeout time.Duration) {}

func (r *fakeMetadataRunner) StartWorker() error { return nil }

func (r *fakeMetadataRunner) Execute(scriptName string, input interface{}) ([]byte, error) {
	return nil, errors.New("not implemented in fake runner")
}

func (r *fakeMetadataRunner) ExecuteAndParse(scriptName string, input interface{}, result interface{}) error {
	r.scriptName = scriptName
	payload, ok := input.(map[string]interface{})
	if !ok {
		return errors.New("unexpected metadata input payload")
	}
	r.payload = payload
	if r.err != nil {
		return r.err
	}

	out, ok := result.(*models.MetadataStripResult)
	if !ok {
		return errors.New("unexpected metadata result type")
	}
	*out = r.result
	return nil
}

func (r *fakeMetadataRunner) CancelActiveTask() {}

func (r *fakeMetadataRunner) StopWorker() {}

type fakeInfoRunner struct {
	result models.InfoResult
	err    error
}

func (r *fakeInfoRunner) SetTimeout(timeout time.Duration) {}

func (r *fakeInfoRunner) StartWorker() error { return nil }

func (r *fakeInfoRunner) Execute(scriptName string, input interface{}) ([]byte, error) {
	return nil, errors.New("not implemented in fake runner")
}

func (r *fakeInfoRunner) ExecuteAndParse(scriptName string, input interface{}, result interface{}) error {
	if r.err != nil {
		return r.err
	}
	out, ok := result.(*models.InfoResult)
	if !ok {
		return errors.New("unexpected info result type")
	}
	*out = r.result
	return nil
}

func (r *fakeInfoRunner) CancelActiveTask() {}

func (r *fakeInfoRunner) StopWorker() {}

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

func TestCancelProcessing_DoesNotCancelFutureWorkOnIdleRunner(t *testing.T) {
	logger, err := utils.NewLogger(utils.ErrorLevel, false)
	if err != nil {
		t.Fatalf("failed to create logger: %v", err)
	}
	defer logger.Close()

	runner := newFakeIdleCancelRunner(30 * time.Millisecond)
	app := &App{
		logger:   logger,
		executor: runner,
		settings: models.AppSettings{MaxConcurrency: 1},
	}
	app.converterService = services.NewConverterService(runner, logger)

	if !app.CancelProcessing() {
		t.Fatalf("expected cancel request to return true")
	}

	result, err := app.Convert(models.ConvertRequest{
		InputPath:  "future.jpg",
		OutputPath: "future.png",
		Format:     "png",
	})
	if err != nil {
		t.Fatalf("expected future convert request to complete, got %v", err)
	}
	if !result.Success {
		t.Fatalf("expected future convert request to succeed, got %+v", result)
	}
	if result.Error != "" {
		t.Fatalf("expected no cancellation error for future request, got %q", result.Error)
	}
}

func TestSaveSettings_DefersStoppingOldRunnerUntilActiveWorkFinishes(t *testing.T) {
	tmpConfig := t.TempDir()
	t.Setenv("APPDATA", tmpConfig)

	logger, err := utils.NewLogger(utils.ErrorLevel, false)
	if err != nil {
		t.Fatalf("failed to create logger: %v", err)
	}
	defer logger.Close()

	oldRunner := newFakeSaveSettingsRunner(2 * time.Second)
	newRunner := newFakeSaveSettingsRunner(0)
	app := &App{
		logger:     logger,
		scriptsDir: t.TempDir(),
		executor:   oldRunner,
		settings: models.AppSettings{
			MaxConcurrency:   1,
			OutputPrefix:     "OLD",
			OutputTemplate:   "{basename}",
			ConflictStrategy: "rename",
		},
	}
	app.converterService = services.NewConverterService(oldRunner, logger)

	if _, err := utils.SaveSettings(app.settings); err != nil {
		t.Fatalf("failed to seed settings: %v", err)
	}

	previousBuilder := app.buildRunnerFn
	app.buildRunnerFn = func(_ string, _ *utils.Logger, settings models.AppSettings) (utils.PythonRunner, error) {
		if settings.MaxConcurrency != 2 {
			t.Fatalf("expected rebuild with max concurrency 2, got %d", settings.MaxConcurrency)
		}
		return newRunner, nil
	}
	defer func() {
		app.buildRunnerFn = previousBuilder
	}()

	convertDone := make(chan error, 1)
	go func() {
		_, convertErr := app.Convert(models.ConvertRequest{
			InputPath:  "busy.jpg",
			OutputPath: "busy.png",
			Format:     "png",
		})
		convertDone <- convertErr
	}()

	select {
	case <-oldRunner.startedCh:
	case <-time.After(2 * time.Second):
		t.Fatal("timed out waiting for convert to start on old runner")
	}

	next := app.settings
	next.MaxConcurrency = 2
	if _, err := app.SaveSettings(next); err != nil {
		t.Fatalf("expected SaveSettings to succeed, got %v", err)
	}

	if stopCount, stoppedWhileActive := oldRunner.snapshot(); stopCount != 0 || stoppedWhileActive {
		t.Fatalf("expected old runner to remain alive during active request, stopCount=%d stoppedWhileActive=%v", stopCount, stoppedWhileActive)
	}

	close(oldRunner.allowFinish)
	select {
	case err := <-convertDone:
		if err != nil {
			t.Fatalf("expected convert to finish cleanly, got %v", err)
		}
	case <-time.After(2 * time.Second):
		t.Fatal("timed out waiting for convert to finish")
	}

	deadline := time.Now().Add(2 * time.Second)
	for {
		if stopCount, stoppedWhileActive := oldRunner.snapshot(); stopCount == 1 && !stoppedWhileActive {
			break
		}
		if time.Now().After(deadline) {
			stopCount, stoppedWhileActive := oldRunner.snapshot()
			t.Fatalf("expected old runner to stop after active request completed, stopCount=%d stoppedWhileActive=%v", stopCount, stoppedWhileActive)
		}
		time.Sleep(10 * time.Millisecond)
	}
}

func TestStripMetadata_ForwardsRequestToMetadataTool(t *testing.T) {
	logger, err := utils.NewLogger(utils.ErrorLevel, false)
	if err != nil {
		t.Fatalf("failed to create logger: %v", err)
	}
	defer logger.Close()

	runner := &fakeMetadataRunner{
		result: models.MetadataStripResult{
			Success:    true,
			InputPath:  "C:/tmp/source.jpg",
			OutputPath: "C:/tmp/output.jpg",
		},
	}
	app := &App{
		logger:   logger,
		executor: runner,
	}
	app.metadataService = services.NewMetadataService(runner, logger)

	req := models.MetadataStripRequest{
		InputPath:  "C:/tmp/source.jpg",
		OutputPath: "C:/tmp/output.jpg",
		Overwrite:  false,
	}
	result, err := app.StripMetadata(req)
	if err != nil {
		t.Fatalf("expected nil error, got %v", err)
	}
	if !result.Success {
		t.Fatalf("expected successful strip result, got %+v", result)
	}
	if runner.scriptName != "metadata_tool.py" {
		t.Fatalf("expected metadata tool script, got %q", runner.scriptName)
	}
	if got := runner.payload["action"]; got != "strip_metadata" {
		t.Fatalf("expected action strip_metadata, got %#v", got)
	}
	if got := runner.payload["input_path"]; got != req.InputPath {
		t.Fatalf("expected input path %q, got %#v", req.InputPath, got)
	}
	if got := runner.payload["output_path"]; got != req.OutputPath {
		t.Fatalf("expected output path %q, got %#v", req.OutputPath, got)
	}
	if got := runner.payload["overwrite"]; got != req.Overwrite {
		t.Fatalf("expected overwrite %v, got %#v", req.Overwrite, got)
	}
}

func TestGetInfo_PreservesStructuredImageData(t *testing.T) {
	logger, err := utils.NewLogger(utils.ErrorLevel, false)
	if err != nil {
		t.Fatalf("failed to create logger: %v", err)
	}
	defer logger.Close()

	runner := &fakeInfoRunner{
		result: models.InfoResult{
			Success:   true,
			InputPath: "C:/tmp/sample.png",
			FileName:  "sample.png",
			Format:    "PNG",
			Mode:      "RGBA",
			Width:     120,
			Height:    80,
			BitDepth:  32,
			FileSize:  4096,
			Basic: &models.InfoBasic{
				Path:       "C:/tmp/sample.png",
				Format:     "PNG",
				Width:      120,
				Height:     80,
				HasAlpha:   true,
				IsAnimated: false,
			},
			FormatDetails: map[string]string{
				"png.color_type": "RGBA",
			},
			Fields: []models.InfoField{
				{Key: "basic.format", Label: "格式", Value: "PNG", Group: "basic", Source: "container", Editable: false},
				{Key: "png.text.Author", Label: "作者", Value: "UnitTest", Group: "png_text", Source: "png_text", Editable: false},
			},
			Warnings: []models.InfoWarning{
				{Code: "LIMITED_METADATA", Message: "未检测到 EXIF"},
			},
		},
	}

	app := &App{
		logger:   logger,
		executor: runner,
	}
	app.infoViewerService = services.NewInfoViewerService(runner, logger)

	result, err := app.GetInfo(models.InfoRequest{InputPath: "C:/tmp/sample.png"})
	if err != nil {
		t.Fatalf("expected nil error, got %v", err)
	}
	if !result.Success {
		t.Fatalf("expected info result to succeed, got %+v", result)
	}
	if result.Basic == nil || result.Basic.Format != "PNG" {
		t.Fatalf("expected structured basic info to be preserved, got %+v", result.Basic)
	}
	if result.FormatDetails["png.color_type"] != "RGBA" {
		t.Fatalf("expected format details to be preserved, got %+v", result.FormatDetails)
	}
	if len(result.Fields) != 2 || result.Fields[1].Source != "png_text" {
		t.Fatalf("expected structured field list to be preserved, got %+v", result.Fields)
	}
	if len(result.Warnings) != 1 || result.Warnings[0].Code != "LIMITED_METADATA" {
		t.Fatalf("expected warnings to be preserved, got %+v", result.Warnings)
	}
}

func TestStripMetadata_RejectsMissingOutputPathBeforeInvokingTool(t *testing.T) {
	logger, err := utils.NewLogger(utils.ErrorLevel, false)
	if err != nil {
		t.Fatalf("failed to create logger: %v", err)
	}
	defer logger.Close()

	runner := &fakeMetadataRunner{
		result: models.MetadataStripResult{
			Success: false,
			Error:   "[BAD_INPUT] missing output_path",
		},
	}
	app := &App{
		logger:   logger,
		executor: runner,
	}
	app.metadataService = services.NewMetadataService(runner, logger)

	req := models.MetadataStripRequest{
		InputPath:  "C:/tmp/source.jpg",
		OutputPath: "",
		Overwrite:  false,
	}
	result, err := app.StripMetadata(req)
	if err != nil {
		t.Fatalf("expected app.StripMetadata to normalize validation error into result, got %v", err)
	}
	if result.Success {
		t.Fatalf("expected strip metadata to fail")
	}
	if result.Error != "输出文件路径无效: 路径不能为空" {
		t.Fatalf("expected service validation error, got %q", result.Error)
	}
	if runner.scriptName != "" {
		t.Fatalf("expected metadata tool not to be invoked, got %q", runner.scriptName)
	}
}

func TestStripMetadata_NormalizesExecutorErrorAndPreservesRequestPaths(t *testing.T) {
	logger, err := utils.NewLogger(utils.ErrorLevel, false)
	if err != nil {
		t.Fatalf("failed to create logger: %v", err)
	}
	defer logger.Close()

	runner := &fakeMetadataRunner{
		err: errors.New("python worker crashed"),
	}
	app := &App{
		logger:   logger,
		executor: runner,
	}
	app.metadataService = services.NewMetadataService(runner, logger)

	req := models.MetadataStripRequest{
		InputPath:  "C:/tmp/source.jpg",
		OutputPath: "C:/tmp/output.jpg",
		Overwrite:  false,
	}
	result, err := app.StripMetadata(req)
	if err != nil {
		t.Fatalf("expected app.StripMetadata to normalize service error into result, got %v", err)
	}
	if result.Success {
		t.Fatalf("expected strip metadata to fail")
	}
	if result.InputPath != req.InputPath {
		t.Fatalf("expected input path %q, got %q", req.InputPath, result.InputPath)
	}
	if result.OutputPath != req.OutputPath {
		t.Fatalf("expected output path %q, got %q", req.OutputPath, result.OutputPath)
	}
	if result.Error != "python worker crashed" {
		t.Fatalf("expected executor error to be preserved, got %q", result.Error)
	}
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
	if _, err := utils.SaveSettings(original); err != nil {
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

	previousBuilder := app.buildRunnerFn
	app.buildRunnerFn = func(_ string, _ *utils.Logger, _ models.AppSettings) (utils.PythonRunner, error) {
		return nil, errors.New("runner rebuild failed")
	}
	defer func() {
		app.buildRunnerFn = previousBuilder
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
	if _, err := utils.SaveSettings(original); err != nil {
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
