package main

import (
	"context"
	"encoding/base64"
	"errors"
	"fmt"
	"net/http"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"sync"
	"sync/atomic"

	"github.com/imageflow/backend/models"
	"github.com/imageflow/backend/services"
	"github.com/imageflow/backend/utils"
	"github.com/wailsapp/wails/v2/pkg/runtime"
)

// App struct
type App struct {
	ctx          context.Context
	stateMu      sync.RWMutex
	processingMu sync.Mutex

	// Utilities
	logger     *utils.Logger
	executor   utils.PythonRunner
	scriptsDir string
	settings   models.AppSettings

	// Services
	converterService      *services.ConverterService
	compressorService     *services.CompressorService
	pdfGeneratorService   *services.PDFGeneratorService
	gifSplitterService    *services.GIFSplitterService
	infoViewerService     *services.InfoViewerService
	metadataService       *services.MetadataService
	watermarkService      *services.WatermarkService
	adjusterService       *services.AdjusterService
	filterService         *services.FilterService
	subtitleStitchService *services.SubtitleStitchService
	cancelRequested       uint32

	previewCacheMu         sync.Mutex
	previewCache           map[string]previewCacheEntry
	previewCacheOrder      []string
	previewCacheMaxEntries int
}

const (
	defaultPreviewMaxBytes = int64(4 * 1024 * 1024)
	previewMaxEdge         = 1280
	previewJPEGQuality     = 85
	defaultPreviewCacheCap = 128
	maxPreviewCacheCap     = 1024
	cancelledErrorMessage  = "[PY_CANCELLED] operation cancelled"
)

type previewCacheEntry struct {
	DataURL         string
	FileSize        int64
	ModTimeUnixNano int64
}

var (
	saveAppSettings          = utils.SaveSettings
	buildRunnerForSettingsFn = buildRunnerForSettings
)

func buildRunnerForSettings(scriptsDir string, logger *utils.Logger, settings models.AppSettings) (utils.PythonRunner, error) {
	if settings.MaxConcurrency > 1 {
		return utils.NewPythonExecutorPool(scriptsDir, logger, settings.MaxConcurrency)
	}
	return utils.NewPythonExecutor(scriptsDir, logger)
}

// NewApp creates a new App application struct
func NewApp() *App {
	return &App{}
}

// startup is called when the app starts
func (a *App) startup(ctx context.Context) {
	a.ctx = ctx

	// Initialize logger
	enableFile := os.Getenv("IMAGEFLOW_FILE_LOG") == "1"
	logger, err := utils.NewLogger(utils.InfoLevel, enableFile)
	if err != nil {
		return
	}
	a.logger = logger
	a.logger.Info("ImageFlow backend starting...")

	if os.Getenv("IMAGEFLOW_PYTHON_EXE") == "" {
		preferredRuntime := ""
		if exe, err := os.Executable(); err == nil {
			candidate := filepath.Join(filepath.Dir(exe), "runtime")
			if pythonExe := utils.PythonExecutableFromRuntime(candidate); pythonExe != "" {
				preferredRuntime = candidate
				_ = os.Setenv("IMAGEFLOW_PYTHON_EXE", pythonExe)
				_ = os.Setenv("PYTHONHOME", candidate)
			}
		}

		if preferredRuntime == "" && utils.HasEmbeddedPythonRuntime(embeddedPythonFS, "embedded_python_runtime") {
			if runtimeDir, err := utils.ExtractEmbeddedPythonRuntime(embeddedPythonFS, "embedded_python_runtime"); err == nil {
				if pythonExe := utils.PythonExecutableFromRuntime(runtimeDir); pythonExe != "" {
					_ = os.Setenv("IMAGEFLOW_PYTHON_EXE", pythonExe)
					_ = os.Setenv("PYTHONHOME", runtimeDir)
				}
			}
		}
	}

	scriptsDir, err := utils.ResolvePythonScriptsDir()
	if err != nil {
		embeddedDir, embedErr := utils.ExtractEmbeddedPythonScripts(embeddedPythonFS, "python")
		if embedErr != nil {
			a.logger.Error("Failed to resolve Python scripts directory: %v", embedErr)
			return
		}
		scriptsDir = embeddedDir
	}
	a.scriptsDir = scriptsDir
	a.logger.Info("Python scripts directory: %s", scriptsDir)

	settings, err := utils.LoadSettings()
	if err != nil {
		a.logger.Error("Failed to load settings: %v", err)
		settings = models.DefaultAppSettings()
	}
	a.settings = settings

	runner, err := buildRunnerForSettingsFn(scriptsDir, logger, settings)
	if err != nil {
		a.logger.Error("Failed to initialize Python executor: %v", err)
		return
	}
	a.executor = runner

	go func(r utils.PythonRunner) {
		if r == nil {
			return
		}
		if err := r.StartWorker(); err != nil {
			a.logger.Warn("Python worker warmup failed: %v", err)
		}
	}(runner)

	// Initialize all services
	a.converterService = services.NewConverterService(runner, logger)
	a.compressorService = services.NewCompressorService(runner, logger)
	a.pdfGeneratorService = services.NewPDFGeneratorService(runner, logger)
	a.gifSplitterService = services.NewGIFSplitterService(runner, logger)
	a.infoViewerService = services.NewInfoViewerService(runner, logger)
	a.metadataService = services.NewMetadataService(runner, logger)
	a.watermarkService = services.NewWatermarkService(runner, logger)
	a.adjusterService = services.NewAdjusterService(runner, logger)
	a.filterService = services.NewFilterService(runner, logger)
	a.subtitleStitchService = services.NewSubtitleStitchService(runner, logger)

	a.logger.Info("All services initialized successfully")
}

// shutdown is called when the app is closing
func (a *App) shutdown(ctx context.Context) {
	a.stateMu.RLock()
	executor := a.executor
	logger := a.logger
	a.stateMu.RUnlock()

	if executor != nil {
		executor.StopWorker()
	}

	if logger != nil {
		logger.Info("ImageFlow backend shutting down...")
		logger.Close()
	}
}

func (a *App) Ping() string {
	return "pong"
}

func mergeOperationError(resultError string, err error) string {
	trimmed := strings.TrimSpace(resultError)
	if trimmed != "" {
		return trimmed
	}
	if err != nil {
		return err.Error()
	}
	return "处理失败"
}

func serviceNotReadyMessage(serviceName string) string {
	return fmt.Sprintf("%s未就绪，请重启应用后重试", serviceName)
}

func (a *App) startProcessing() {
	a.processingMu.Lock()
	a.beginCancelableOperation()
}

func (a *App) finishProcessing() {
	a.processingMu.Unlock()
}

func (a *App) beginCancelableOperation() {
	atomic.StoreUint32(&a.cancelRequested, 0)
}

func (a *App) requestCancelOperation() {
	atomic.StoreUint32(&a.cancelRequested, 1)
	a.stateMu.RLock()
	executor := a.executor
	a.stateMu.RUnlock()
	if executor != nil {
		executor.CancelActiveTask()
	}
}

func (a *App) isCancelRequested() bool {
	return atomic.LoadUint32(&a.cancelRequested) == 1
}

func isCancelledExecutionError(err error) bool {
	if err == nil {
		return false
	}
	return strings.Contains(err.Error(), "[PY_CANCELLED]")
}

func (a *App) CancelProcessing() bool {
	a.requestCancelOperation()
	return true
}

func (a *App) GetSettings() (models.AppSettings, error) {
	a.stateMu.RLock()
	defer a.stateMu.RUnlock()
	return a.settings, nil
}

func (a *App) SaveSettings(settings models.AppSettings) (models.AppSettings, error) {
	a.processingMu.Lock()
	defer a.processingMu.Unlock()

	saved := utils.NormalizeSettings(settings)

	a.stateMu.RLock()
	currentSettings := a.settings
	scriptsDir := a.scriptsDir
	logger := a.logger
	old := a.executor
	a.stateMu.RUnlock()

	var (
		runner     utils.PythonRunner
		rebuildErr error
	)
	if saved.MaxConcurrency != currentSettings.MaxConcurrency {
		runner, rebuildErr = buildRunnerForSettingsFn(scriptsDir, logger, saved)
		if rebuildErr != nil {
			return currentSettings, rebuildErr
		}
	}

	persisted, err := saveAppSettings(saved)
	if err != nil {
		if runner != nil {
			runner.StopWorker()
		}
		return saved, err
	}
	saved = persisted

	if runner != nil {
		a.stateMu.Lock()
		a.executor = runner
		a.converterService = services.NewConverterService(runner, logger)
		a.compressorService = services.NewCompressorService(runner, logger)
		a.pdfGeneratorService = services.NewPDFGeneratorService(runner, logger)
		a.gifSplitterService = services.NewGIFSplitterService(runner, logger)
		a.infoViewerService = services.NewInfoViewerService(runner, logger)
		a.metadataService = services.NewMetadataService(runner, logger)
		a.watermarkService = services.NewWatermarkService(runner, logger)
		a.adjusterService = services.NewAdjusterService(runner, logger)
		a.filterService = services.NewFilterService(runner, logger)
		a.subtitleStitchService = services.NewSubtitleStitchService(runner, logger)
		a.stateMu.Unlock()

		go func(r utils.PythonRunner) {
			if r == nil {
				return
			}
			if err := r.StartWorker(); err != nil {
				logger.Warn("Python worker warmup failed: %v", err)
			}
		}(runner)

		if old != nil && old != runner {
			old.StopWorker()
		}
	}

	a.stateMu.Lock()
	a.settings = saved
	a.stateMu.Unlock()
	return saved, nil
}

func normalizeRecentPathValue(path string) string {
	trimmed := strings.TrimSpace(path)
	if trimmed == "" {
		return ""
	}
	if trimmed == "/" || trimmed == `\` {
		return trimmed
	}
	cleaned := strings.TrimRight(trimmed, "/\\")
	if len(cleaned) == 2 && cleaned[1] == ':' {
		return trimmed
	}
	if cleaned == "" {
		return trimmed
	}
	return cleaned
}

func mergeRecentPaths(current []string, next string) []string {
	normalized := normalizeRecentPathValue(next)
	if normalized == "" {
		clone := append([]string(nil), current...)
		return clone
	}
	merged := make([]string, 0, len(current)+1)
	merged = append(merged, normalized)
	for _, item := range current {
		if strings.EqualFold(normalizeRecentPathValue(item), normalized) {
			continue
		}
		merged = append(merged, item)
	}
	return merged
}

func sameStringSlice(a, b []string) bool {
	if len(a) != len(b) {
		return false
	}
	for i := range a {
		if a[i] != b[i] {
			return false
		}
	}
	return true
}

func (a *App) UpdateRecentPaths(req models.RecentPathsUpdateRequest) (models.AppSettings, error) {
	a.processingMu.Lock()
	defer a.processingMu.Unlock()

	a.stateMu.RLock()
	currentSettings := a.settings
	a.stateMu.RUnlock()

	next := currentSettings
	next.RecentInputDirs = mergeRecentPaths(currentSettings.RecentInputDirs, req.InputDir)
	next.RecentOutputDirs = mergeRecentPaths(currentSettings.RecentOutputDirs, req.OutputDir)
	next = utils.NormalizeSettings(next)

	if sameStringSlice(next.RecentInputDirs, currentSettings.RecentInputDirs) &&
		sameStringSlice(next.RecentOutputDirs, currentSettings.RecentOutputDirs) {
		return currentSettings, nil
	}

	persisted, err := saveAppSettings(next)
	if err != nil {
		return currentSettings, err
	}

	a.stateMu.Lock()
	a.settings = persisted
	a.stateMu.Unlock()
	return persisted, nil
}

func (a *App) SelectOutputDirectory() (string, error) {
	return runtime.OpenDirectoryDialog(a.ctx, runtime.OpenDialogOptions{
		Title: "选择输出文件夹",
	})
}

func (a *App) SelectInputFiles() ([]string, error) {
	return runtime.OpenMultipleFilesDialog(a.ctx, runtime.OpenDialogOptions{
		Title: "选择文件",
		Filters: []runtime.FileFilter{
			{
				DisplayName: "Images",
				Pattern:     "*.jpg;*.jpeg;*.png;*.webp;*.gif;*.bmp;*.tiff;*.tif;*.heic;*.heif;*.svg",
			},
		},
	})
}

func (a *App) SelectInputDirectory() (string, error) {
	return runtime.OpenDirectoryDialog(a.ctx, runtime.OpenDialogOptions{
		Title: "选择文件夹",
	})
}

func (a *App) ExpandDroppedPaths(paths []string) (models.ExpandDroppedPathsResult, error) {
	return utils.ExpandInputPaths(paths)
}

// Convert converts an image to a different format
func (a *App) Convert(req models.ConvertRequest) (models.ConvertResult, error) {
	a.startProcessing()
	defer a.finishProcessing()
	if a.converterService == nil {
		return models.ConvertResult{
			Success:    false,
			InputPath:  req.InputPath,
			OutputPath: req.OutputPath,
			Error:      serviceNotReadyMessage("格式转换服务"),
		}, nil
	}
	result, err := a.converterService.Convert(req)
	if err != nil {
		result.Success = false
		if strings.TrimSpace(result.InputPath) == "" {
			result.InputPath = req.InputPath
		}
		if strings.TrimSpace(result.OutputPath) == "" {
			result.OutputPath = req.OutputPath
		}
		if isCancelledExecutionError(err) || a.isCancelRequested() {
			result.Error = cancelledErrorMessage
		} else {
			result.Error = mergeOperationError(result.Error, err)
		}
		return result, nil
	}
	return result, nil
}

func batchWorkerCount(total, configured int) int {
	workers := configured
	if workers < 1 {
		workers = 1
	}
	if workers > 32 {
		workers = 32
	}
	if workers > total {
		workers = total
	}
	return workers
}

func buildBatchResults[TReq any, TRes any](requests []TReq, builder func(TReq) TRes) []TRes {
	results := make([]TRes, len(requests))
	for i, req := range requests {
		results[i] = builder(req)
	}
	return results
}

func executeBatch[TReq any, TRes any](
	requests []TReq,
	maxConcurrency int,
	isCancelRequested func() bool,
	buildCancelled func(req TReq) TRes,
	execute func(req TReq) (TRes, error),
	normalizeError func(res *TRes, req TReq, err error),
) []TRes {
	n := len(requests)
	results := make([]TRes, n)
	if n == 0 {
		return results
	}

	workers := batchWorkerCount(n, maxConcurrency)
	jobs := make(chan int, workers)
	var wg sync.WaitGroup

	for w := 0; w < workers; w++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for idx := range jobs {
				req := requests[idx]
				if isCancelRequested() {
					results[idx] = buildCancelled(req)
					continue
				}
				res, err := execute(req)
				if err != nil {
					normalizeError(&res, req, err)
				}
				results[idx] = res
			}
		}()
	}

	for i := 0; i < n; i++ {
		if isCancelRequested() {
			for j := i; j < n; j++ {
				results[j] = buildCancelled(requests[j])
			}
			break
		}
		jobs <- i
	}
	close(jobs)
	wg.Wait()
	return results
}

// ConvertBatch converts multiple images concurrently
func (a *App) ConvertBatch(requests []models.ConvertRequest) ([]models.ConvertResult, error) {
	a.startProcessing()
	defer a.finishProcessing()
	if len(requests) == 0 {
		return []models.ConvertResult{}, nil
	}
	if a.converterService == nil {
		errMsg := serviceNotReadyMessage("格式转换服务")
		return buildBatchResults(requests, func(req models.ConvertRequest) models.ConvertResult {
			return models.ConvertResult{
				Success:    false,
				InputPath:  req.InputPath,
				OutputPath: req.OutputPath,
				Error:      errMsg,
			}
		}), nil
	}

	results := executeBatch(
		requests,
		a.settings.MaxConcurrency,
		a.isCancelRequested,
		func(req models.ConvertRequest) models.ConvertResult {
			return models.ConvertResult{
				Success:    false,
				InputPath:  req.InputPath,
				OutputPath: req.OutputPath,
				Error:      cancelledErrorMessage,
			}
		},
		func(req models.ConvertRequest) (models.ConvertResult, error) {
			return a.converterService.Convert(req)
		},
		func(res *models.ConvertResult, req models.ConvertRequest, err error) {
			res.Success = false
			if strings.TrimSpace(res.InputPath) == "" {
				res.InputPath = req.InputPath
			}
			if strings.TrimSpace(res.OutputPath) == "" {
				res.OutputPath = req.OutputPath
			}
			if isCancelledExecutionError(err) || a.isCancelRequested() {
				res.Error = cancelledErrorMessage
			} else {
				res.Error = mergeOperationError(res.Error, err)
			}
		},
	)
	return results, nil
}

// Compress compresses an image
func (a *App) Compress(req models.CompressRequest) (models.CompressResult, error) {
	a.startProcessing()
	defer a.finishProcessing()
	if a.compressorService == nil {
		return models.CompressResult{
			Success:    false,
			InputPath:  req.InputPath,
			OutputPath: req.OutputPath,
			Error:      serviceNotReadyMessage("图片压缩服务"),
		}, nil
	}
	result, err := a.compressorService.Compress(req)
	if err != nil {
		result.Success = false
		if strings.TrimSpace(result.InputPath) == "" {
			result.InputPath = req.InputPath
		}
		if strings.TrimSpace(result.OutputPath) == "" {
			result.OutputPath = req.OutputPath
		}
		if isCancelledExecutionError(err) || a.isCancelRequested() {
			result.Error = cancelledErrorMessage
		} else {
			result.Error = mergeOperationError(result.Error, err)
		}
		return result, nil
	}
	return result, nil
}

// CompressBatch compresses multiple images concurrently
func (a *App) CompressBatch(requests []models.CompressRequest) ([]models.CompressResult, error) {
	a.startProcessing()
	defer a.finishProcessing()
	if len(requests) == 0 {
		return []models.CompressResult{}, nil
	}
	if a.compressorService == nil {
		errMsg := serviceNotReadyMessage("图片压缩服务")
		return buildBatchResults(requests, func(req models.CompressRequest) models.CompressResult {
			return models.CompressResult{
				Success:    false,
				InputPath:  req.InputPath,
				OutputPath: req.OutputPath,
				Error:      errMsg,
			}
		}), nil
	}

	results := executeBatch(
		requests,
		a.settings.MaxConcurrency,
		a.isCancelRequested,
		func(req models.CompressRequest) models.CompressResult {
			return models.CompressResult{
				Success:    false,
				InputPath:  req.InputPath,
				OutputPath: req.OutputPath,
				Error:      cancelledErrorMessage,
			}
		},
		func(req models.CompressRequest) (models.CompressResult, error) {
			return a.compressorService.Compress(req)
		},
		func(res *models.CompressResult, req models.CompressRequest, err error) {
			res.Success = false
			if strings.TrimSpace(res.InputPath) == "" {
				res.InputPath = req.InputPath
			}
			if strings.TrimSpace(res.OutputPath) == "" {
				res.OutputPath = req.OutputPath
			}
			if isCancelledExecutionError(err) || a.isCancelRequested() {
				res.Error = cancelledErrorMessage
			} else {
				res.Error = mergeOperationError(res.Error, err)
			}
		},
	)
	return results, nil
}

// GeneratePDF generates a PDF from multiple images
func (a *App) GeneratePDF(req models.PDFRequest) (models.PDFResult, error) {
	a.startProcessing()
	defer a.finishProcessing()
	if a.pdfGeneratorService == nil {
		return models.PDFResult{
			Success:    false,
			OutputPath: req.OutputPath,
			Error:      serviceNotReadyMessage("PDF 服务"),
		}, nil
	}
	result, err := a.pdfGeneratorService.GeneratePDF(req)
	if err != nil {
		result.Success = false
		if strings.TrimSpace(result.OutputPath) == "" {
			result.OutputPath = req.OutputPath
		}
		if isCancelledExecutionError(err) || a.isCancelRequested() {
			result.Error = cancelledErrorMessage
		} else {
			result.Error = mergeOperationError(result.Error, err)
		}
		return result, nil
	}
	return result, nil
}

// SplitGIF handles animation actions (export_frames, reverse, change_speed, build_gif, compress, resize, convert_animation)
func (a *App) SplitGIF(req models.GIFSplitRequest) (models.GIFSplitResult, error) {
	a.startProcessing()
	defer a.finishProcessing()
	if a.gifSplitterService == nil {
		return models.GIFSplitResult{
			Success:    false,
			InputPath:  req.InputPath,
			InputPaths: req.InputPaths,
			OutputDir:  req.OutputDir,
			OutputPath: req.OutputPath,
			Error:      serviceNotReadyMessage("GIF 服务"),
		}, nil
	}
	result, err := a.gifSplitterService.SplitGIF(req)
	if err != nil {
		result.Success = false
		if strings.TrimSpace(result.InputPath) == "" {
			result.InputPath = req.InputPath
		}
		if len(result.InputPaths) == 0 && len(req.InputPaths) > 0 {
			result.InputPaths = req.InputPaths
		}
		if strings.TrimSpace(result.OutputDir) == "" {
			result.OutputDir = req.OutputDir
		}
		if strings.TrimSpace(result.OutputPath) == "" {
			result.OutputPath = req.OutputPath
		}
		if isCancelledExecutionError(err) || a.isCancelRequested() {
			result.Error = cancelledErrorMessage
		} else {
			result.Error = mergeOperationError(result.Error, err)
		}
		return result, nil
	}
	return result, nil
}

// GenerateSubtitleLongImage handles "first full frame + subtitle strips" image generation.
func (a *App) GenerateSubtitleLongImage(req models.SubtitleStitchRequest) (models.SubtitleStitchResult, error) {
	a.startProcessing()
	defer a.finishProcessing()
	if a.subtitleStitchService == nil {
		return models.SubtitleStitchResult{
			Success:    false,
			OutputPath: req.OutputPath,
			Error:      serviceNotReadyMessage("字幕拼接服务"),
		}, nil
	}
	result, err := a.subtitleStitchService.Generate(req)
	if err != nil {
		result.Success = false
		if strings.TrimSpace(result.OutputPath) == "" {
			result.OutputPath = req.OutputPath
		}
		if isCancelledExecutionError(err) || a.isCancelRequested() {
			result.Error = cancelledErrorMessage
		} else {
			result.Error = mergeOperationError(result.Error, err)
		}
		return result, nil
	}
	return result, nil
}

// GetInfo retrieves image information
func (a *App) GetInfo(req models.InfoRequest) (models.InfoResult, error) {
	a.startProcessing()
	defer a.finishProcessing()

	if a.infoViewerService == nil {
		return models.InfoResult{
			Success:   false,
			InputPath: req.InputPath,
			Error:     serviceNotReadyMessage("信息读取服务"),
		}, nil
	}
	result, err := a.infoViewerService.GetInfo(req)
	if err != nil {
		result.Success = false
		if strings.TrimSpace(result.InputPath) == "" {
			result.InputPath = req.InputPath
		}
		result.Error = mergeOperationError(result.Error, err)
		return result, nil
	}
	return result, nil
}

func getPreviewMaxBytes() int64 {
	value := strings.TrimSpace(os.Getenv("IMAGEFLOW_PREVIEW_MAX_BYTES"))
	if value == "" {
		return defaultPreviewMaxBytes
	}
	parsed, err := strconv.ParseInt(value, 10, 64)
	if err != nil || parsed <= 0 {
		return defaultPreviewMaxBytes
	}
	return parsed
}

func getPreviewCacheCap() int {
	value := strings.TrimSpace(os.Getenv("IMAGEFLOW_PREVIEW_CACHE_ENTRIES"))
	if value == "" {
		return defaultPreviewCacheCap
	}
	parsed, err := strconv.Atoi(value)
	if err != nil || parsed <= 0 {
		return defaultPreviewCacheCap
	}
	if parsed > maxPreviewCacheCap {
		return maxPreviewCacheCap
	}
	return parsed
}

func removePreviewCacheOrderKey(order []string, key string) []string {
	for idx, item := range order {
		if item != key {
			continue
		}
		return append(order[:idx], order[idx+1:]...)
	}
	return order
}

func (a *App) ensurePreviewCacheLocked() {
	if a.previewCache == nil {
		a.previewCache = make(map[string]previewCacheEntry)
	}
	if a.previewCacheMaxEntries <= 0 {
		a.previewCacheMaxEntries = getPreviewCacheCap()
	}
}

func (a *App) getPreviewCacheEntry(key string, info os.FileInfo) (string, bool) {
	if key == "" || info == nil {
		return "", false
	}
	a.previewCacheMu.Lock()
	defer a.previewCacheMu.Unlock()

	a.ensurePreviewCacheLocked()
	entry, ok := a.previewCache[key]
	if !ok {
		return "", false
	}
	if entry.FileSize != info.Size() || entry.ModTimeUnixNano != info.ModTime().UnixNano() {
		delete(a.previewCache, key)
		a.previewCacheOrder = removePreviewCacheOrderKey(a.previewCacheOrder, key)
		return "", false
	}
	a.previewCacheOrder = append(removePreviewCacheOrderKey(a.previewCacheOrder, key), key)
	return entry.DataURL, true
}

func (a *App) setPreviewCacheEntry(key string, info os.FileInfo, dataURL string) {
	if key == "" || info == nil || strings.TrimSpace(dataURL) == "" {
		return
	}
	a.previewCacheMu.Lock()
	defer a.previewCacheMu.Unlock()

	a.ensurePreviewCacheLocked()
	a.previewCache[key] = previewCacheEntry{
		DataURL:         dataURL,
		FileSize:        info.Size(),
		ModTimeUnixNano: info.ModTime().UnixNano(),
	}
	a.previewCacheOrder = append(removePreviewCacheOrderKey(a.previewCacheOrder, key), key)
	for len(a.previewCacheOrder) > a.previewCacheMaxEntries {
		oldest := a.previewCacheOrder[0]
		a.previewCacheOrder = a.previewCacheOrder[1:]
		delete(a.previewCache, oldest)
	}
}

func detectPreviewMimeType(data []byte, inputPath string) string {
	mimeType := http.DetectContentType(data)
	if strings.HasPrefix(mimeType, "application/octet-stream") || strings.HasPrefix(mimeType, "text/plain") {
		ext := strings.ToLower(filepath.Ext(inputPath))
		switch ext {
		case ".jpg", ".jpeg":
			mimeType = "image/jpeg"
		case ".png":
			mimeType = "image/png"
		case ".webp":
			mimeType = "image/webp"
		case ".gif":
			mimeType = "image/gif"
		case ".bmp":
			mimeType = "image/bmp"
		case ".tif", ".tiff":
			mimeType = "image/tiff"
		case ".svg":
			mimeType = "image/svg+xml"
		}
	}
	return mimeType
}

func buildDataURL(data []byte, mimeType string) string {
	encoded := base64.StdEncoding.EncodeToString(data)
	return fmt.Sprintf("data:%s;base64,%s", mimeType, encoded)
}

func (a *App) buildPreviewFromConverter(inputPath string) (models.PreviewResult, error) {
	if a.converterService == nil {
		return models.PreviewResult{Success: false, Error: "PREVIEW_SKIPPED"}, errors.New("converter service not ready")
	}

	tmp, err := os.CreateTemp("", "imageflow-preview-*.jpg")
	if err != nil {
		return models.PreviewResult{Success: false, Error: err.Error()}, err
	}
	tmpPath := tmp.Name()
	_ = tmp.Close()
	defer func() {
		_ = os.Remove(tmpPath)
	}()

	req := models.ConvertRequest{
		InputPath:  inputPath,
		OutputPath: tmpPath,
		Format:     "jpg",
		Quality:    previewJPEGQuality,
		MaintainAR: true,
		ResizeMode: "long_edge",
		LongEdge:   previewMaxEdge,
	}

	if _, err := a.converterService.Convert(req); err != nil {
		return models.PreviewResult{Success: false, Error: "PREVIEW_SKIPPED"}, err
	}

	data, err := os.ReadFile(tmpPath)
	if err != nil {
		return models.PreviewResult{Success: false, Error: err.Error()}, err
	}

	dataURL := buildDataURL(data, "image/jpeg")
	return models.PreviewResult{Success: true, DataURL: dataURL}, nil
}

// GetImagePreview builds a data URL for previewing images in the frontend.
func (a *App) GetImagePreview(req models.PreviewRequest) (models.PreviewResult, error) {
	a.startProcessing()
	defer a.finishProcessing()

	inputPath := strings.TrimSpace(req.InputPath)
	if inputPath == "" {
		return models.PreviewResult{Success: false, Error: "输入路径为空"}, errors.New("input path is empty")
	}

	cacheKey := filepath.Clean(inputPath)
	fileInfo, statErr := os.Stat(inputPath)
	if statErr == nil {
		if dataURL, ok := a.getPreviewCacheEntry(cacheKey, fileInfo); ok {
			return models.PreviewResult{Success: true, DataURL: dataURL}, nil
		}
	}

	maxPreviewBytes := getPreviewMaxBytes()
	if statErr == nil && fileInfo.Size() > maxPreviewBytes {
		preview, err := a.buildPreviewFromConverter(inputPath)
		if err == nil && preview.Success && preview.DataURL != "" {
			a.setPreviewCacheEntry(cacheKey, fileInfo, preview.DataURL)
			return preview, nil
		}
		return models.PreviewResult{Success: false, Error: "PREVIEW_SKIPPED"}, nil
	}

	data, err := os.ReadFile(inputPath)
	if err != nil {
		return models.PreviewResult{Success: false, Error: err.Error()}, err
	}

	mimeType := detectPreviewMimeType(data, inputPath)
	dataURL := buildDataURL(data, mimeType)
	if statErr == nil {
		a.setPreviewCacheEntry(cacheKey, fileInfo, dataURL)
	}
	return models.PreviewResult{Success: true, DataURL: dataURL}, nil
}

// EditMetadata edits image metadata (EXIF)
func (a *App) EditMetadata(req models.MetadataEditRequest) (models.MetadataEditResult, error) {
	a.startProcessing()
	defer a.finishProcessing()

	if a.infoViewerService == nil {
		return models.MetadataEditResult{
			Success:    false,
			InputPath:  req.InputPath,
			OutputPath: req.OutputPath,
			Error:      serviceNotReadyMessage("元数据服务"),
		}, nil
	}
	result, err := a.infoViewerService.EditMetadata(req)
	if err != nil {
		result.Success = false
		if strings.TrimSpace(result.InputPath) == "" {
			result.InputPath = req.InputPath
		}
		if strings.TrimSpace(result.OutputPath) == "" {
			result.OutputPath = req.OutputPath
		}
		result.Error = mergeOperationError(result.Error, err)
		return result, nil
	}
	return result, nil
}

func (a *App) StripMetadata(req models.MetadataStripRequest) (models.MetadataStripResult, error) {
	a.startProcessing()
	defer a.finishProcessing()

	if a.metadataService == nil {
		return models.MetadataStripResult{
			Success:    false,
			InputPath:  req.InputPath,
			OutputPath: req.OutputPath,
			Error:      serviceNotReadyMessage("隐私清理服务"),
		}, nil
	}
	result, err := a.metadataService.StripMetadata(req)
	if err != nil {
		result.Success = false
		if strings.TrimSpace(result.InputPath) == "" {
			result.InputPath = req.InputPath
		}
		if strings.TrimSpace(result.OutputPath) == "" {
			result.OutputPath = req.OutputPath
		}
		result.Error = mergeOperationError(result.Error, err)
		return result, nil
	}
	return result, nil
}

// ResolveOutputPath resolves an output path with conflict strategy (rename).
func (a *App) ResolveOutputPath(req models.ResolveOutputPathRequest) (models.ResolveOutputPathResult, error) {
	base := strings.TrimSpace(req.BasePath)
	if base == "" {
		return models.ResolveOutputPathResult{Success: false, Error: "输出路径为空"}, errors.New("base path is empty")
	}
	strategy := strings.ToLower(strings.TrimSpace(req.Strategy))
	if strategy == "" {
		strategy = "rename"
	}
	if strategy != "rename" {
		strategy = "rename"
	}

	reserved := make(map[string]struct{}, len(req.Reserved))
	for _, p := range req.Reserved {
		normalized := strings.TrimSpace(p)
		if normalized == "" {
			continue
		}
		reserved[filepath.Clean(normalized)] = struct{}{}
	}

	path, err := utils.ResolveOutputPath(filepath.Clean(base), reserved)
	if err != nil {
		return models.ResolveOutputPathResult{Success: false, Error: err.Error()}, err
	}
	return models.ResolveOutputPathResult{Success: true, OutputPath: path}, nil
}

// AddWatermark adds a watermark to an image
func (a *App) AddWatermark(req models.WatermarkRequest) (models.WatermarkResult, error) {
	a.startProcessing()
	defer a.finishProcessing()
	if a.watermarkService == nil {
		return models.WatermarkResult{
			Success:    false,
			InputPath:  req.InputPath,
			OutputPath: req.OutputPath,
			Error:      serviceNotReadyMessage("水印服务"),
		}, nil
	}
	result, err := a.watermarkService.AddWatermark(req)
	if err != nil {
		result.Success = false
		if strings.TrimSpace(result.InputPath) == "" {
			result.InputPath = req.InputPath
		}
		if strings.TrimSpace(result.OutputPath) == "" {
			result.OutputPath = req.OutputPath
		}
		if isCancelledExecutionError(err) || a.isCancelRequested() {
			result.Error = cancelledErrorMessage
		} else {
			result.Error = mergeOperationError(result.Error, err)
		}
		return result, nil
	}
	return result, nil
}

// AddWatermarkBatch adds watermarks to multiple images concurrently
func (a *App) AddWatermarkBatch(requests []models.WatermarkRequest) ([]models.WatermarkResult, error) {
	a.startProcessing()
	defer a.finishProcessing()
	if len(requests) == 0 {
		return []models.WatermarkResult{}, nil
	}
	if a.watermarkService == nil {
		errMsg := serviceNotReadyMessage("水印服务")
		return buildBatchResults(requests, func(req models.WatermarkRequest) models.WatermarkResult {
			return models.WatermarkResult{
				Success:    false,
				InputPath:  req.InputPath,
				OutputPath: req.OutputPath,
				Error:      errMsg,
			}
		}), nil
	}

	results := executeBatch(
		requests,
		a.settings.MaxConcurrency,
		a.isCancelRequested,
		func(req models.WatermarkRequest) models.WatermarkResult {
			return models.WatermarkResult{
				Success:    false,
				InputPath:  req.InputPath,
				OutputPath: req.OutputPath,
				Error:      cancelledErrorMessage,
			}
		},
		func(req models.WatermarkRequest) (models.WatermarkResult, error) {
			return a.watermarkService.AddWatermark(req)
		},
		func(res *models.WatermarkResult, req models.WatermarkRequest, err error) {
			res.Success = false
			if strings.TrimSpace(res.InputPath) == "" {
				res.InputPath = req.InputPath
			}
			if strings.TrimSpace(res.OutputPath) == "" {
				res.OutputPath = req.OutputPath
			}
			if isCancelledExecutionError(err) || a.isCancelRequested() {
				res.Error = cancelledErrorMessage
			} else {
				res.Error = mergeOperationError(res.Error, err)
			}
		},
	)
	return results, nil
}

// ListSystemFonts returns available system font files.
func (a *App) ListSystemFonts() ([]string, error) {
	fonts, err := utils.ListSystemFonts()
	if err != nil {
		if a.logger != nil {
			a.logger.Warn("ListSystemFonts failed: %v", err)
		}
		return []string{}, err
	}
	return fonts, nil
}

// Adjust applies adjustments to an image
func (a *App) Adjust(req models.AdjustRequest) (models.AdjustResult, error) {
	a.startProcessing()
	defer a.finishProcessing()
	if a.adjusterService == nil {
		return models.AdjustResult{
			Success:    false,
			InputPath:  req.InputPath,
			OutputPath: req.OutputPath,
			Error:      serviceNotReadyMessage("调整服务"),
		}, nil
	}
	result, err := a.adjusterService.Adjust(req)
	if err != nil {
		result.Success = false
		if strings.TrimSpace(result.InputPath) == "" {
			result.InputPath = req.InputPath
		}
		if strings.TrimSpace(result.OutputPath) == "" {
			result.OutputPath = req.OutputPath
		}
		if isCancelledExecutionError(err) || a.isCancelRequested() {
			result.Error = cancelledErrorMessage
		} else {
			result.Error = mergeOperationError(result.Error, err)
		}
		return result, nil
	}
	return result, nil
}

// AdjustBatch applies adjustments to multiple images concurrently
func (a *App) AdjustBatch(requests []models.AdjustRequest) ([]models.AdjustResult, error) {
	a.startProcessing()
	defer a.finishProcessing()
	if len(requests) == 0 {
		return []models.AdjustResult{}, nil
	}
	if a.adjusterService == nil {
		errMsg := serviceNotReadyMessage("调整服务")
		return buildBatchResults(requests, func(req models.AdjustRequest) models.AdjustResult {
			return models.AdjustResult{
				Success:    false,
				InputPath:  req.InputPath,
				OutputPath: req.OutputPath,
				Error:      errMsg,
			}
		}), nil
	}

	results := executeBatch(
		requests,
		a.settings.MaxConcurrency,
		a.isCancelRequested,
		func(req models.AdjustRequest) models.AdjustResult {
			return models.AdjustResult{
				Success:    false,
				InputPath:  req.InputPath,
				OutputPath: req.OutputPath,
				Error:      cancelledErrorMessage,
			}
		},
		func(req models.AdjustRequest) (models.AdjustResult, error) {
			return a.adjusterService.Adjust(req)
		},
		func(res *models.AdjustResult, req models.AdjustRequest, err error) {
			res.Success = false
			if strings.TrimSpace(res.InputPath) == "" {
				res.InputPath = req.InputPath
			}
			if strings.TrimSpace(res.OutputPath) == "" {
				res.OutputPath = req.OutputPath
			}
			if isCancelledExecutionError(err) || a.isCancelRequested() {
				res.Error = cancelledErrorMessage
			} else {
				res.Error = mergeOperationError(res.Error, err)
			}
		},
	)
	return results, nil
}

// ApplyFilter applies a filter to an image
func (a *App) ApplyFilter(req models.FilterRequest) (models.FilterResult, error) {
	a.startProcessing()
	defer a.finishProcessing()
	if a.filterService == nil {
		return models.FilterResult{
			Success:    false,
			InputPath:  req.InputPath,
			OutputPath: req.OutputPath,
			Error:      serviceNotReadyMessage("滤镜服务"),
		}, nil
	}
	result, err := a.filterService.ApplyFilter(req)
	if err != nil {
		result.Success = false
		if strings.TrimSpace(result.InputPath) == "" {
			result.InputPath = req.InputPath
		}
		if strings.TrimSpace(result.OutputPath) == "" {
			result.OutputPath = req.OutputPath
		}
		if isCancelledExecutionError(err) || a.isCancelRequested() {
			result.Error = cancelledErrorMessage
		} else {
			result.Error = mergeOperationError(result.Error, err)
		}
		return result, nil
	}
	return result, nil
}

// ApplyFilterBatch applies filters to multiple images concurrently
func (a *App) ApplyFilterBatch(requests []models.FilterRequest) ([]models.FilterResult, error) {
	a.startProcessing()
	defer a.finishProcessing()
	if len(requests) == 0 {
		return []models.FilterResult{}, nil
	}
	if a.filterService == nil {
		errMsg := serviceNotReadyMessage("滤镜服务")
		return buildBatchResults(requests, func(req models.FilterRequest) models.FilterResult {
			return models.FilterResult{
				Success:    false,
				InputPath:  req.InputPath,
				OutputPath: req.OutputPath,
				Error:      errMsg,
			}
		}), nil
	}

	results := executeBatch(
		requests,
		a.settings.MaxConcurrency,
		a.isCancelRequested,
		func(req models.FilterRequest) models.FilterResult {
			return models.FilterResult{
				Success:    false,
				InputPath:  req.InputPath,
				OutputPath: req.OutputPath,
				Error:      cancelledErrorMessage,
			}
		},
		func(req models.FilterRequest) (models.FilterResult, error) {
			return a.filterService.ApplyFilter(req)
		},
		func(res *models.FilterResult, req models.FilterRequest, err error) {
			res.Success = false
			if strings.TrimSpace(res.InputPath) == "" {
				res.InputPath = req.InputPath
			}
			if strings.TrimSpace(res.OutputPath) == "" {
				res.OutputPath = req.OutputPath
			}
			if isCancelledExecutionError(err) || a.isCancelRequested() {
				res.Error = cancelledErrorMessage
			} else {
				res.Error = mergeOperationError(res.Error, err)
			}
		},
	)
	return results, nil
}
