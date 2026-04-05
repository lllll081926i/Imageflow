package main

import (
	"bytes"
	"context"
	"encoding/base64"
	"errors"
	"fmt"
	"io"
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
	operationsMu sync.Mutex

	// Utilities
	logger              *utils.Logger
	executor            utils.PythonRunner
	scriptsDir          string
	settings            models.AppSettings
	saveSettingsFn      func(models.AppSettings) (models.AppSettings, error)
	buildRunnerFn       func(string, *utils.Logger, models.AppSettings) (utils.PythonRunner, error)
	readPreviewHeaderFn func(string) (string, error)

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
	operationsCond        *sync.Cond
	activeOperations      map[uint64]*activeOperation
	nextOperationID       uint64

	previewCacheMu         sync.Mutex
	previewCache           map[string]previewCacheEntry
	previewCacheOrder      []string
	previewCacheMaxEntries int
	previewCacheTotalBytes int
	previewCacheMaxBytes   int
}

const (
	defaultPreviewMaxBytes = int64(4 * 1024 * 1024)
	previewMaxEdge         = 1280
	previewJPEGQuality     = 85
	defaultPreviewCacheCap = 128
	maxPreviewCacheCap     = 1024
	defaultPreviewCacheMax = 32 * 1024 * 1024
	maxPreviewCacheMax     = 256 * 1024 * 1024
	cancelledErrorMessage  = "[PY_CANCELLED] operation cancelled"
)

type previewCacheEntry struct {
	DataURL         string
	FileSize        int64
	ModTimeUnixNano int64
	CachedBytes     int
}

type activeOperation struct {
	runner    utils.PythonRunner
	cancelled uint32
}

type operationHandle struct {
	id    uint64
	state *activeOperation
}

type appStateSnapshot struct {
	logger                *utils.Logger
	executor              utils.PythonRunner
	settings              models.AppSettings
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
}

func buildRunnerForSettings(scriptsDir string, logger *utils.Logger, settings models.AppSettings) (utils.PythonRunner, error) {
	if settings.MaxConcurrency > 1 {
		return utils.NewPythonExecutorPool(scriptsDir, logger, settings.MaxConcurrency)
	}
	return utils.NewPythonExecutor(scriptsDir, logger)
}

// NewApp creates a new App application struct
func NewApp() *App {
	return &App{
		saveSettingsFn:      utils.SaveSettings,
		buildRunnerFn:       buildRunnerForSettings,
		readPreviewHeaderFn: readPreviewHeader,
	}
}

func (a *App) saveSettings(settings models.AppSettings) (models.AppSettings, error) {
	if a.saveSettingsFn != nil {
		return a.saveSettingsFn(settings)
	}
	return utils.SaveSettings(settings)
}

func (a *App) buildRunner(scriptsDir string, logger *utils.Logger, settings models.AppSettings) (utils.PythonRunner, error) {
	if a.buildRunnerFn != nil {
		return a.buildRunnerFn(scriptsDir, logger, settings)
	}
	return buildRunnerForSettings(scriptsDir, logger, settings)
}

func (a *App) readPreviewHeader(inputPath string) (string, error) {
	if a.readPreviewHeaderFn != nil {
		return a.readPreviewHeaderFn(inputPath)
	}
	return readPreviewHeader(inputPath)
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

	runner, err := a.buildRunner(scriptsDir, logger, settings)
	if err != nil {
		a.logger.Error("Failed to initialize Python executor: %v", err)
		return
	}
	a.stateMu.Lock()
	a.settings = settings
	a.setRunnerStateLocked(runner, logger)
	a.stateMu.Unlock()

	go func(r utils.PythonRunner) {
		if r == nil {
			return
		}
		if err := r.StartWorker(); err != nil {
			a.logger.Warn("Python worker warmup failed: %v", err)
		}
	}(runner)

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

func (a *App) ensureOperationsStateLocked() {
	if a.operationsCond == nil {
		a.operationsCond = sync.NewCond(&a.operationsMu)
	}
	if a.activeOperations == nil {
		a.activeOperations = make(map[uint64]*activeOperation)
	}
}

func (a *App) currentStateSnapshot() appStateSnapshot {
	a.stateMu.RLock()
	defer a.stateMu.RUnlock()

	return a.currentStateSnapshotLocked()
}

func (a *App) currentStateSnapshotLocked() appStateSnapshot {
	return appStateSnapshot{
		logger:                a.logger,
		executor:              a.executor,
		settings:              a.settings,
		converterService:      a.converterService,
		compressorService:     a.compressorService,
		pdfGeneratorService:   a.pdfGeneratorService,
		gifSplitterService:    a.gifSplitterService,
		infoViewerService:     a.infoViewerService,
		metadataService:       a.metadataService,
		watermarkService:      a.watermarkService,
		adjusterService:       a.adjusterService,
		filterService:         a.filterService,
		subtitleStitchService: a.subtitleStitchService,
	}
}

func (a *App) setRunnerStateLocked(runner utils.PythonRunner, logger *utils.Logger) {
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
}

func (a *App) beginOperation() (appStateSnapshot, *operationHandle) {
	a.stateMu.RLock()
	snapshot := a.currentStateSnapshotLocked()
	a.operationsMu.Lock()
	a.ensureOperationsStateLocked()
	a.nextOperationID++
	id := a.nextOperationID
	op := &activeOperation{runner: snapshot.executor}
	a.activeOperations[id] = op
	a.operationsMu.Unlock()
	a.stateMu.RUnlock()
	return snapshot, &operationHandle{id: id, state: op}
}

func (a *App) finishProcessing(handle *operationHandle) {
	if handle == nil {
		return
	}

	a.operationsMu.Lock()
	defer a.operationsMu.Unlock()

	a.ensureOperationsStateLocked()
	delete(a.activeOperations, handle.id)
	a.operationsCond.Broadcast()
}

func (a *App) isRunnerActiveLocked(runner utils.PythonRunner) bool {
	for _, op := range a.activeOperations {
		if op.runner == runner {
			return true
		}
	}
	return false
}

func (a *App) stopRunnerWhenIdle(runner utils.PythonRunner) {
	if runner == nil {
		return
	}

	go func() {
		a.operationsMu.Lock()
		a.ensureOperationsStateLocked()
		for a.isRunnerActiveLocked(runner) {
			a.operationsCond.Wait()
		}
		a.operationsMu.Unlock()
		runner.StopWorker()
	}()
}

func (a *App) requestCancelOperation() {
	runners := make(map[utils.PythonRunner]struct{})

	a.operationsMu.Lock()
	a.ensureOperationsStateLocked()
	for _, op := range a.activeOperations {
		atomic.StoreUint32(&op.cancelled, 1)
		if op.runner != nil {
			runners[op.runner] = struct{}{}
		}
	}
	a.operationsMu.Unlock()

	for runner := range runners {
		runner.CancelActiveTask()
	}
}

func (a *App) isCancelRequested(handle *operationHandle) bool {
	if handle == nil || handle.state == nil {
		return false
	}
	return atomic.LoadUint32(&handle.state.cancelled) == 1
}

func isCancelledExecutionError(err error) bool {
	return utils.IsPythonCancelled(err)
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

	snapshot := a.currentStateSnapshot()
	currentSettings := snapshot.settings
	scriptsDir := a.scriptsDir
	logger := snapshot.logger
	old := snapshot.executor

	var (
		runner     utils.PythonRunner
		rebuildErr error
	)
	if saved.MaxConcurrency != currentSettings.MaxConcurrency {
		runner, rebuildErr = a.buildRunner(scriptsDir, logger, saved)
		if rebuildErr != nil {
			return currentSettings, rebuildErr
		}
	}

	persisted, err := a.saveSettings(saved)
	if err != nil {
		if runner != nil {
			runner.StopWorker()
		}
		return saved, err
	}
	saved = persisted

	a.stateMu.Lock()
	if runner != nil {
		a.setRunnerStateLocked(runner, logger)
	}
	a.settings = saved
	a.stateMu.Unlock()

	if runner != nil {
		go func(r utils.PythonRunner) {
			if r == nil {
				return
			}
			if err := r.StartWorker(); err != nil {
				logger.Warn("Python worker warmup failed: %v", err)
			}
		}(runner)

		if old != nil && old != runner {
			a.stopRunnerWhenIdle(old)
		}
	}
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

	persisted, err := a.saveSettings(next)
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
	normalized := make([]string, 0, len(paths))
	for i, raw := range paths {
		if strings.TrimSpace(raw) == "" {
			continue
		}
		path, err := utils.NormalizeUserSuppliedPath(raw)
		if err != nil {
			return models.ExpandDroppedPathsResult{}, fmt.Errorf("第 %d 个拖拽路径无效: %w", i+1, err)
		}
		normalized = append(normalized, path)
	}
	return utils.ExpandInputPaths(normalized)
}

// Convert converts an image to a different format
func (a *App) Convert(req models.ConvertRequest) (models.ConvertResult, error) {
	state, op := a.beginOperation()
	defer a.finishProcessing(op)
	if state.converterService == nil {
		return models.ConvertResult{
			Success:    false,
			InputPath:  req.InputPath,
			OutputPath: req.OutputPath,
			Error:      serviceNotReadyMessage("格式转换服务"),
		}, nil
	}
	result, err := state.converterService.Convert(req)
	if err != nil {
		result.Success = false
		if strings.TrimSpace(result.InputPath) == "" {
			result.InputPath = req.InputPath
		}
		if strings.TrimSpace(result.OutputPath) == "" {
			result.OutputPath = req.OutputPath
		}
		if isCancelledExecutionError(err) || a.isCancelRequested(op) {
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
	state, op := a.beginOperation()
	defer a.finishProcessing(op)
	if len(requests) == 0 {
		return []models.ConvertResult{}, nil
	}
	if state.converterService == nil {
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
		state.settings.MaxConcurrency,
		func() bool { return a.isCancelRequested(op) },
		func(req models.ConvertRequest) models.ConvertResult {
			return models.ConvertResult{
				Success:    false,
				InputPath:  req.InputPath,
				OutputPath: req.OutputPath,
				Error:      cancelledErrorMessage,
			}
		},
		func(req models.ConvertRequest) (models.ConvertResult, error) {
			return state.converterService.Convert(req)
		},
		func(res *models.ConvertResult, req models.ConvertRequest, err error) {
			res.Success = false
			if strings.TrimSpace(res.InputPath) == "" {
				res.InputPath = req.InputPath
			}
			if strings.TrimSpace(res.OutputPath) == "" {
				res.OutputPath = req.OutputPath
			}
			if isCancelledExecutionError(err) || a.isCancelRequested(op) {
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
	state, op := a.beginOperation()
	defer a.finishProcessing(op)
	if state.compressorService == nil {
		return models.CompressResult{
			Success:    false,
			InputPath:  req.InputPath,
			OutputPath: req.OutputPath,
			Error:      serviceNotReadyMessage("图片压缩服务"),
		}, nil
	}
	result, err := state.compressorService.Compress(req)
	if err != nil {
		result.Success = false
		if strings.TrimSpace(result.InputPath) == "" {
			result.InputPath = req.InputPath
		}
		if strings.TrimSpace(result.OutputPath) == "" {
			result.OutputPath = req.OutputPath
		}
		if isCancelledExecutionError(err) || a.isCancelRequested(op) {
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
	state, op := a.beginOperation()
	defer a.finishProcessing(op)
	if len(requests) == 0 {
		return []models.CompressResult{}, nil
	}
	if state.compressorService == nil {
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
		state.settings.MaxConcurrency,
		func() bool { return a.isCancelRequested(op) },
		func(req models.CompressRequest) models.CompressResult {
			return models.CompressResult{
				Success:    false,
				InputPath:  req.InputPath,
				OutputPath: req.OutputPath,
				Error:      cancelledErrorMessage,
			}
		},
		func(req models.CompressRequest) (models.CompressResult, error) {
			return state.compressorService.Compress(req)
		},
		func(res *models.CompressResult, req models.CompressRequest, err error) {
			res.Success = false
			if strings.TrimSpace(res.InputPath) == "" {
				res.InputPath = req.InputPath
			}
			if strings.TrimSpace(res.OutputPath) == "" {
				res.OutputPath = req.OutputPath
			}
			if isCancelledExecutionError(err) || a.isCancelRequested(op) {
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
	state, op := a.beginOperation()
	defer a.finishProcessing(op)
	if state.pdfGeneratorService == nil {
		return models.PDFResult{
			Success:    false,
			OutputPath: req.OutputPath,
			Error:      serviceNotReadyMessage("PDF 服务"),
		}, nil
	}
	result, err := state.pdfGeneratorService.GeneratePDF(req)
	if err != nil {
		result.Success = false
		if strings.TrimSpace(result.OutputPath) == "" {
			result.OutputPath = req.OutputPath
		}
		if isCancelledExecutionError(err) || a.isCancelRequested(op) {
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
	state, op := a.beginOperation()
	defer a.finishProcessing(op)
	if state.gifSplitterService == nil {
		return models.GIFSplitResult{
			Success:    false,
			InputPath:  req.InputPath,
			InputPaths: req.InputPaths,
			OutputDir:  req.OutputDir,
			OutputPath: req.OutputPath,
			Error:      serviceNotReadyMessage("GIF 服务"),
		}, nil
	}
	result, err := state.gifSplitterService.SplitGIF(req)
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
		if isCancelledExecutionError(err) || a.isCancelRequested(op) {
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
	state, op := a.beginOperation()
	defer a.finishProcessing(op)
	if state.subtitleStitchService == nil {
		return models.SubtitleStitchResult{
			Success:    false,
			OutputPath: req.OutputPath,
			Error:      serviceNotReadyMessage("字幕拼接服务"),
		}, nil
	}
	result, err := state.subtitleStitchService.Generate(req)
	if err != nil {
		result.Success = false
		if strings.TrimSpace(result.OutputPath) == "" {
			result.OutputPath = req.OutputPath
		}
		if isCancelledExecutionError(err) || a.isCancelRequested(op) {
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
	state, op := a.beginOperation()
	defer a.finishProcessing(op)

	if state.infoViewerService == nil {
		return models.InfoResult{
			Success:   false,
			InputPath: req.InputPath,
			Error:     serviceNotReadyMessage("信息读取服务"),
		}, nil
	}
	result, err := state.infoViewerService.GetInfo(req)
	if err != nil {
		result.Success = false
		if strings.TrimSpace(result.InputPath) == "" {
			result.InputPath = req.InputPath
		}
		if isCancelledExecutionError(err) || a.isCancelRequested(op) {
			result.Error = cancelledErrorMessage
		} else {
			result.Error = mergeOperationError(result.Error, err)
		}
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

func getPreviewCacheMaxBytes() int {
	value := strings.TrimSpace(os.Getenv("IMAGEFLOW_PREVIEW_CACHE_MAX_BYTES"))
	if value == "" {
		return defaultPreviewCacheMax
	}
	parsed, err := strconv.Atoi(value)
	if err != nil || parsed <= 0 {
		return defaultPreviewCacheMax
	}
	if parsed > maxPreviewCacheMax {
		return maxPreviewCacheMax
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
	if a.previewCacheMaxBytes <= 0 {
		a.previewCacheMaxBytes = getPreviewCacheMaxBytes()
	}
}

func (a *App) removePreviewCacheEntryLocked(key string) {
	entry, ok := a.previewCache[key]
	if !ok {
		return
	}
	delete(a.previewCache, key)
	a.previewCacheOrder = removePreviewCacheOrderKey(a.previewCacheOrder, key)
	a.previewCacheTotalBytes -= entry.CachedBytes
	if a.previewCacheTotalBytes < 0 {
		a.previewCacheTotalBytes = 0
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
		a.removePreviewCacheEntryLocked(key)
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
	entryBytes := len(dataURL)
	if a.previewCacheMaxBytes > 0 && entryBytes > a.previewCacheMaxBytes {
		a.removePreviewCacheEntryLocked(key)
		return
	}
	if existing, ok := a.previewCache[key]; ok {
		a.previewCacheTotalBytes -= existing.CachedBytes
		if a.previewCacheTotalBytes < 0 {
			a.previewCacheTotalBytes = 0
		}
	}
	a.previewCache[key] = previewCacheEntry{
		DataURL:         dataURL,
		FileSize:        info.Size(),
		ModTimeUnixNano: info.ModTime().UnixNano(),
		CachedBytes:     entryBytes,
	}
	a.previewCacheTotalBytes += entryBytes
	a.previewCacheOrder = append(removePreviewCacheOrderKey(a.previewCacheOrder, key), key)
	for len(a.previewCacheOrder) > a.previewCacheMaxEntries || (a.previewCacheMaxBytes > 0 && a.previewCacheTotalBytes > a.previewCacheMaxBytes) {
		oldest := a.previewCacheOrder[0]
		a.removePreviewCacheEntryLocked(oldest)
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
		case ".heic":
			mimeType = "image/heic"
		case ".heif":
			mimeType = "image/heif"
		case ".svg":
			mimeType = "image/svg+xml"
		}
	}
	return mimeType
}

var allowedPreviewExtensions = map[string]struct{}{
	".png":  {},
	".jpg":  {},
	".jpeg": {},
	".gif":  {},
	".bmp":  {},
	".webp": {},
	".tif":  {},
	".tiff": {},
	".svg":  {},
	".heic": {},
	".heif": {},
}

var directPreviewExtensions = map[string]struct{}{
	".png":  {},
	".jpg":  {},
	".jpeg": {},
	".gif":  {},
	".bmp":  {},
	".webp": {},
	".svg":  {},
}

func isAllowedPreviewExtension(inputPath string) bool {
	_, ok := allowedPreviewExtensions[strings.ToLower(filepath.Ext(inputPath))]
	return ok
}

func usesDirectPreview(inputPath string) bool {
	_, ok := directPreviewExtensions[strings.ToLower(filepath.Ext(inputPath))]
	return ok
}

func detectPreviewMimeTypeFromHeader(header []byte, inputPath string) string {
	header = bytes.TrimSpace(header)
	if len(header) == 0 {
		return ""
	}

	mimeType := http.DetectContentType(header)
	if strings.HasPrefix(mimeType, "image/") {
		return mimeType
	}

	lowerHeader := bytes.ToLower(header)
	switch strings.ToLower(filepath.Ext(inputPath)) {
	case ".svg":
		if bytes.Contains(lowerHeader, []byte("<svg")) {
			return "image/svg+xml"
		}
	case ".heic":
		if len(header) >= 12 && string(header[4:8]) == "ftyp" {
			return "image/heic"
		}
	case ".heif":
		if len(header) >= 12 && string(header[4:8]) == "ftyp" {
			return "image/heif"
		}
	}

	return ""
}

func validatePreviewFileInfo(inputPath string, info os.FileInfo) error {
	if strings.TrimSpace(inputPath) == "" {
		return errors.New("输入路径为空")
	}
	if info == nil {
		return errors.New("预览文件不存在")
	}
	if !info.Mode().IsRegular() {
		return errors.New("不支持预览该文件类型")
	}
	if !isAllowedPreviewExtension(inputPath) {
		return errors.New("不支持预览该文件类型")
	}
	return nil
}

func readPreviewHeader(inputPath string) (string, error) {
	file, err := os.Open(inputPath)
	if err != nil {
		return "", err
	}
	defer file.Close()

	header := make([]byte, 512)
	readBytes, err := file.Read(header)
	if err != nil && !errors.Is(err, io.EOF) {
		return "", err
	}
	if readBytes == 0 {
		return "", errors.New("文件为空，无法生成预览")
	}
	mimeType := detectPreviewMimeTypeFromHeader(header[:readBytes], inputPath)
	if mimeType == "" {
		return "", errors.New("不支持预览该文件类型")
	}
	return mimeType, nil
}

func (a *App) validatePreviewInputFile(inputPath string, info os.FileInfo) (string, error) {
	if err := validatePreviewFileInfo(inputPath, info); err != nil {
		return "", err
	}
	return a.readPreviewHeader(inputPath)
}

func shouldUseConvertedPreview(inputPath string, fileSize int64) bool {
	if !usesDirectPreview(inputPath) {
		return true
	}
	return fileSize > getPreviewMaxBytes()
}

func buildDataURL(data []byte, mimeType string) string {
	encoded := base64.StdEncoding.EncodeToString(data)
	return fmt.Sprintf("data:%s;base64,%s", mimeType, encoded)
}

func (a *App) buildPreviewFromConverter(converterService *services.ConverterService, inputPath string) (models.PreviewResult, error) {
	if converterService == nil {
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

	if _, err := converterService.Convert(req); err != nil {
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
	state, op := a.beginOperation()
	defer a.finishProcessing(op)

	if strings.TrimSpace(req.InputPath) == "" {
		return models.PreviewResult{Success: false, Error: "输入路径为空"}, errors.New("input path is empty")
	}
	inputPath, err := utils.NormalizeUserSuppliedPath(req.InputPath)
	if err != nil {
		return models.PreviewResult{Success: false, Error: err.Error()}, err
	}

	cacheKey := filepath.Clean(inputPath)
	fileInfo, statErr := os.Stat(inputPath)
	if statErr != nil {
		return models.PreviewResult{Success: false, Error: statErr.Error()}, statErr
	}
	if err := validatePreviewFileInfo(inputPath, fileInfo); err != nil {
		return models.PreviewResult{Success: false, Error: err.Error()}, err
	}
	if dataURL, ok := a.getPreviewCacheEntry(cacheKey, fileInfo); ok {
		return models.PreviewResult{Success: true, DataURL: dataURL}, nil
	}

	if _, err := a.validatePreviewInputFile(inputPath, fileInfo); err != nil {
		return models.PreviewResult{Success: false, Error: err.Error()}, err
	}

	if shouldUseConvertedPreview(inputPath, fileInfo.Size()) {
		preview, err := a.buildPreviewFromConverter(state.converterService, inputPath)
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
	if !strings.HasPrefix(mimeType, "image/") {
		return models.PreviewResult{Success: false, Error: "不支持预览该文件类型"}, errors.New("preview file is not a supported image")
	}
	dataURL := buildDataURL(data, mimeType)
	if statErr == nil {
		a.setPreviewCacheEntry(cacheKey, fileInfo, dataURL)
	}
	return models.PreviewResult{Success: true, DataURL: dataURL}, nil
}

// EditMetadata edits image metadata (EXIF)
func (a *App) EditMetadata(req models.MetadataEditRequest) (models.MetadataEditResult, error) {
	state, op := a.beginOperation()
	defer a.finishProcessing(op)

	if state.infoViewerService == nil {
		return models.MetadataEditResult{
			Success:    false,
			InputPath:  req.InputPath,
			OutputPath: req.OutputPath,
			Error:      serviceNotReadyMessage("元数据服务"),
		}, nil
	}
	result, err := state.infoViewerService.EditMetadata(req)
	if err != nil {
		result.Success = false
		if strings.TrimSpace(result.InputPath) == "" {
			result.InputPath = req.InputPath
		}
		if strings.TrimSpace(result.OutputPath) == "" {
			result.OutputPath = req.OutputPath
		}
		if isCancelledExecutionError(err) || a.isCancelRequested(op) {
			result.Error = cancelledErrorMessage
		} else {
			result.Error = mergeOperationError(result.Error, err)
		}
		return result, nil
	}
	return result, nil
}

func (a *App) StripMetadata(req models.MetadataStripRequest) (models.MetadataStripResult, error) {
	state, op := a.beginOperation()
	defer a.finishProcessing(op)

	if state.metadataService == nil {
		return models.MetadataStripResult{
			Success:    false,
			InputPath:  req.InputPath,
			OutputPath: req.OutputPath,
			Error:      serviceNotReadyMessage("隐私清理服务"),
		}, nil
	}
	result, err := state.metadataService.StripMetadata(req)
	if err != nil {
		result.Success = false
		if strings.TrimSpace(result.InputPath) == "" {
			result.InputPath = req.InputPath
		}
		if strings.TrimSpace(result.OutputPath) == "" {
			result.OutputPath = req.OutputPath
		}
		if isCancelledExecutionError(err) || a.isCancelRequested(op) {
			result.Error = cancelledErrorMessage
		} else {
			result.Error = mergeOperationError(result.Error, err)
		}
		return result, nil
	}
	return result, nil
}

// ResolveOutputPath resolves an output path with conflict strategy (rename).
func (a *App) ResolveOutputPath(req models.ResolveOutputPathRequest) (models.ResolveOutputPathResult, error) {
	if strings.TrimSpace(req.BasePath) == "" {
		return models.ResolveOutputPathResult{Success: false, Error: "输出路径为空"}, errors.New("base path is empty")
	}
	base, err := utils.NormalizeUserSuppliedPath(req.BasePath)
	if err != nil {
		return models.ResolveOutputPathResult{Success: false, Error: err.Error()}, err
	}
	strategy := strings.ToLower(strings.TrimSpace(req.Strategy))
	if strategy == "" {
		strategy = "rename"
	}
	if strategy != "rename" {
		strategy = "rename"
	}

	reserved := make(map[string]struct{}, len(req.Reserved))
	for i, p := range req.Reserved {
		normalized, err := utils.NormalizeOptionalUserSuppliedPath(p)
		if err != nil {
			return models.ResolveOutputPathResult{Success: false, Error: fmt.Sprintf("第 %d 个保留路径无效: %v", i+1, err)}, err
		}
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
	state, op := a.beginOperation()
	defer a.finishProcessing(op)
	if state.watermarkService == nil {
		return models.WatermarkResult{
			Success:    false,
			InputPath:  req.InputPath,
			OutputPath: req.OutputPath,
			Error:      serviceNotReadyMessage("水印服务"),
		}, nil
	}
	result, err := state.watermarkService.AddWatermark(req)
	if err != nil {
		result.Success = false
		if strings.TrimSpace(result.InputPath) == "" {
			result.InputPath = req.InputPath
		}
		if strings.TrimSpace(result.OutputPath) == "" {
			result.OutputPath = req.OutputPath
		}
		if isCancelledExecutionError(err) || a.isCancelRequested(op) {
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
	state, op := a.beginOperation()
	defer a.finishProcessing(op)
	if len(requests) == 0 {
		return []models.WatermarkResult{}, nil
	}
	if state.watermarkService == nil {
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
		state.settings.MaxConcurrency,
		func() bool { return a.isCancelRequested(op) },
		func(req models.WatermarkRequest) models.WatermarkResult {
			return models.WatermarkResult{
				Success:    false,
				InputPath:  req.InputPath,
				OutputPath: req.OutputPath,
				Error:      cancelledErrorMessage,
			}
		},
		func(req models.WatermarkRequest) (models.WatermarkResult, error) {
			return state.watermarkService.AddWatermark(req)
		},
		func(res *models.WatermarkResult, req models.WatermarkRequest, err error) {
			res.Success = false
			if strings.TrimSpace(res.InputPath) == "" {
				res.InputPath = req.InputPath
			}
			if strings.TrimSpace(res.OutputPath) == "" {
				res.OutputPath = req.OutputPath
			}
			if isCancelledExecutionError(err) || a.isCancelRequested(op) {
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
	state, op := a.beginOperation()
	defer a.finishProcessing(op)
	if state.adjusterService == nil {
		return models.AdjustResult{
			Success:    false,
			InputPath:  req.InputPath,
			OutputPath: req.OutputPath,
			Error:      serviceNotReadyMessage("调整服务"),
		}, nil
	}
	result, err := state.adjusterService.Adjust(req)
	if err != nil {
		result.Success = false
		if strings.TrimSpace(result.InputPath) == "" {
			result.InputPath = req.InputPath
		}
		if strings.TrimSpace(result.OutputPath) == "" {
			result.OutputPath = req.OutputPath
		}
		if isCancelledExecutionError(err) || a.isCancelRequested(op) {
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
	state, op := a.beginOperation()
	defer a.finishProcessing(op)
	if len(requests) == 0 {
		return []models.AdjustResult{}, nil
	}
	if state.adjusterService == nil {
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
		state.settings.MaxConcurrency,
		func() bool { return a.isCancelRequested(op) },
		func(req models.AdjustRequest) models.AdjustResult {
			return models.AdjustResult{
				Success:    false,
				InputPath:  req.InputPath,
				OutputPath: req.OutputPath,
				Error:      cancelledErrorMessage,
			}
		},
		func(req models.AdjustRequest) (models.AdjustResult, error) {
			return state.adjusterService.Adjust(req)
		},
		func(res *models.AdjustResult, req models.AdjustRequest, err error) {
			res.Success = false
			if strings.TrimSpace(res.InputPath) == "" {
				res.InputPath = req.InputPath
			}
			if strings.TrimSpace(res.OutputPath) == "" {
				res.OutputPath = req.OutputPath
			}
			if isCancelledExecutionError(err) || a.isCancelRequested(op) {
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
	state, op := a.beginOperation()
	defer a.finishProcessing(op)
	if state.filterService == nil {
		return models.FilterResult{
			Success:    false,
			InputPath:  req.InputPath,
			OutputPath: req.OutputPath,
			Error:      serviceNotReadyMessage("滤镜服务"),
		}, nil
	}
	result, err := state.filterService.ApplyFilter(req)
	if err != nil {
		result.Success = false
		if strings.TrimSpace(result.InputPath) == "" {
			result.InputPath = req.InputPath
		}
		if strings.TrimSpace(result.OutputPath) == "" {
			result.OutputPath = req.OutputPath
		}
		if isCancelledExecutionError(err) || a.isCancelRequested(op) {
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
	state, op := a.beginOperation()
	defer a.finishProcessing(op)
	if len(requests) == 0 {
		return []models.FilterResult{}, nil
	}
	if state.filterService == nil {
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
		state.settings.MaxConcurrency,
		func() bool { return a.isCancelRequested(op) },
		func(req models.FilterRequest) models.FilterResult {
			return models.FilterResult{
				Success:    false,
				InputPath:  req.InputPath,
				OutputPath: req.OutputPath,
				Error:      cancelledErrorMessage,
			}
		},
		func(req models.FilterRequest) (models.FilterResult, error) {
			return state.filterService.ApplyFilter(req)
		},
		func(res *models.FilterResult, req models.FilterRequest, err error) {
			res.Success = false
			if strings.TrimSpace(res.InputPath) == "" {
				res.InputPath = req.InputPath
			}
			if strings.TrimSpace(res.OutputPath) == "" {
				res.OutputPath = req.OutputPath
			}
			if isCancelledExecutionError(err) || a.isCancelRequested(op) {
				res.Error = cancelledErrorMessage
			} else {
				res.Error = mergeOperationError(res.Error, err)
			}
		},
	)
	return results, nil
}
