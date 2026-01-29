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

	"github.com/imageflow/backend/models"
	"github.com/imageflow/backend/services"
	"github.com/imageflow/backend/utils"
	"github.com/wailsapp/wails/v2/pkg/runtime"
)

// App struct
type App struct {
	ctx context.Context

	// Utilities
	logger     *utils.Logger
	executor   utils.PythonRunner
	scriptsDir string
	settings   models.AppSettings

	// Services
	converterService    *services.ConverterService
	compressorService   *services.CompressorService
	pdfGeneratorService *services.PDFGeneratorService
	gifSplitterService  *services.GIFSplitterService
	infoViewerService   *services.InfoViewerService
	metadataService     *services.MetadataService
	watermarkService    *services.WatermarkService
	adjusterService     *services.AdjusterService
	filterService       *services.FilterService
}

const (
	defaultPreviewMaxBytes = int64(4 * 1024 * 1024)
	previewMaxEdge         = 1280
	previewJPEGQuality     = 85
)

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
		if runtimeDir, err := utils.ExtractEmbeddedPythonRuntime(embeddedPythonFS, "embedded_python_runtime"); err == nil {
			if pythonExe := utils.PythonExecutableFromRuntime(runtimeDir); pythonExe != "" {
				_ = os.Setenv("IMAGEFLOW_PYTHON_EXE", pythonExe)
				_ = os.Setenv("PYTHONHOME", runtimeDir)
			}
		} else if exe, err := os.Executable(); err == nil {
			runtimeDir := filepath.Join(filepath.Dir(exe), "python_runtime")
			if pythonExe := utils.PythonExecutableFromRuntime(runtimeDir); pythonExe != "" {
				_ = os.Setenv("IMAGEFLOW_PYTHON_EXE", pythonExe)
				_ = os.Setenv("PYTHONHOME", runtimeDir)
			}
		}
	}

	scriptsDir, err := utils.ResolvePythonScriptsDir()
	if err != nil {
		embeddedDir, embedErr := utils.ExtractEmbeddedPythonScripts(embeddedPythonFS, "embedded_python")
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

	var runner utils.PythonRunner
	if settings.MaxConcurrency > 1 {
		pool, err := utils.NewPythonExecutorPool(scriptsDir, logger, settings.MaxConcurrency)
		if err != nil {
			a.logger.Error("Failed to initialize Python executor pool: %v", err)
			return
		}
		runner = pool
	} else {
		executor, err := utils.NewPythonExecutor(scriptsDir, logger)
		if err != nil {
			a.logger.Error("Failed to initialize Python executor: %v", err)
			return
		}
		runner = executor
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

	a.logger.Info("All services initialized successfully")
}

// shutdown is called when the app is closing
func (a *App) shutdown(ctx context.Context) {
	if a.executor != nil {
		a.executor.StopWorker()
	}

	if a.logger != nil {
		a.logger.Info("ImageFlow backend shutting down...")
		a.logger.Close()
	}
}

func (a *App) Ping() string {
	return "pong"
}

func (a *App) GetSettings() (models.AppSettings, error) {
	return a.settings, nil
}

func (a *App) SaveSettings(settings models.AppSettings) (models.AppSettings, error) {
	saved, err := utils.SaveSettings(settings)
	if err != nil {
		return saved, err
	}

	if saved.MaxConcurrency != a.settings.MaxConcurrency {
		var runner utils.PythonRunner
		if saved.MaxConcurrency > 1 {
			pool, err := utils.NewPythonExecutorPool(a.scriptsDir, a.logger, saved.MaxConcurrency)
			if err != nil {
				return a.settings, err
			}
			runner = pool
		} else {
			exec, err := utils.NewPythonExecutor(a.scriptsDir, a.logger)
			if err != nil {
				return a.settings, err
			}
			runner = exec
		}

		old := a.executor
		a.executor = runner
		a.converterService = services.NewConverterService(runner, a.logger)
		a.compressorService = services.NewCompressorService(runner, a.logger)
		a.pdfGeneratorService = services.NewPDFGeneratorService(runner, a.logger)
		a.gifSplitterService = services.NewGIFSplitterService(runner, a.logger)
		a.infoViewerService = services.NewInfoViewerService(runner, a.logger)
		a.metadataService = services.NewMetadataService(runner, a.logger)
		a.watermarkService = services.NewWatermarkService(runner, a.logger)
		a.adjusterService = services.NewAdjusterService(runner, a.logger)
		a.filterService = services.NewFilterService(runner, a.logger)

		go func(r utils.PythonRunner) {
			if r == nil {
				return
			}
			if err := r.StartWorker(); err != nil {
				a.logger.Warn("Python worker warmup failed: %v", err)
			}
		}(runner)

		if old != nil {
			old.StopWorker()
		}
	}

	a.settings = saved
	return saved, nil
}

func (a *App) SelectOutputDirectory() (string, error) {
	return runtime.OpenDirectoryDialog(a.ctx, runtime.OpenDialogOptions{
		Title: "选择输出文件夹",
	})
}

func (a *App) ExpandDroppedPaths(paths []string) (models.ExpandDroppedPathsResult, error) {
	return utils.ExpandInputPaths(paths)
}

// Convert converts an image to a different format
func (a *App) Convert(req models.ConvertRequest) (models.ConvertResult, error) {
	return a.converterService.Convert(req)
}

// ConvertBatch converts multiple images concurrently
func (a *App) ConvertBatch(requests []models.ConvertRequest) ([]models.ConvertResult, error) {
	n := len(requests)
	results := make([]models.ConvertResult, n)
	if n == 0 {
		return results, nil
	}
	var errsMu sync.Mutex
	var errs []error
	workers := a.settings.MaxConcurrency
	if workers < 1 {
		workers = 1
	}
	if workers > 32 {
		workers = 32
	}
	if workers > n {
		workers = n
	}

	jobs := make(chan int, workers)
	var wg sync.WaitGroup
	for w := 0; w < workers; w++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for idx := range jobs {
				res, err := a.converterService.Convert(requests[idx])
				results[idx] = res
				if err != nil {
					errsMu.Lock()
					errs = append(errs, fmt.Errorf("convert[%d]: %w", idx, err))
					errsMu.Unlock()
				}
			}
		}()
	}
	for i := 0; i < n; i++ {
		jobs <- i
	}
	close(jobs)
	wg.Wait()
	if len(errs) > 0 {
		return results, errors.Join(errs...)
	}
	return results, nil
}

// Compress compresses an image
func (a *App) Compress(req models.CompressRequest) (models.CompressResult, error) {
	return a.compressorService.Compress(req)
}

// CompressBatch compresses multiple images concurrently
func (a *App) CompressBatch(requests []models.CompressRequest) ([]models.CompressResult, error) {
	n := len(requests)
	results := make([]models.CompressResult, n)
	if n == 0 {
		return results, nil
	}
	var errsMu sync.Mutex
	var errs []error
	workers := a.settings.MaxConcurrency
	if workers < 1 {
		workers = 1
	}
	if workers > 32 {
		workers = 32
	}
	if workers > n {
		workers = n
	}

	jobs := make(chan int, workers)
	var wg sync.WaitGroup
	for w := 0; w < workers; w++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for idx := range jobs {
				res, err := a.compressorService.Compress(requests[idx])
				results[idx] = res
				if err != nil {
					errsMu.Lock()
					errs = append(errs, fmt.Errorf("compress[%d]: %w", idx, err))
					errsMu.Unlock()
				}
			}
		}()
	}
	for i := 0; i < n; i++ {
		jobs <- i
	}
	close(jobs)
	wg.Wait()
	if len(errs) > 0 {
		return results, errors.Join(errs...)
	}
	return results, nil
}

// GeneratePDF generates a PDF from multiple images
func (a *App) GeneratePDF(req models.PDFRequest) (models.PDFResult, error) {
	return a.pdfGeneratorService.GeneratePDF(req)
}

// SplitGIF handles GIF-related actions (export_frames, reverse, change_speed, build_gif)
func (a *App) SplitGIF(req models.GIFSplitRequest) (models.GIFSplitResult, error) {
	return a.gifSplitterService.SplitGIF(req)
}

// GetInfo retrieves image information
func (a *App) GetInfo(req models.InfoRequest) (models.InfoResult, error) {
	return a.infoViewerService.GetInfo(req)
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
	if strings.TrimSpace(req.InputPath) == "" {
		return models.PreviewResult{Success: false, Error: "输入路径为空"}, errors.New("input path is empty")
	}

	maxPreviewBytes := getPreviewMaxBytes()
	if info, err := os.Stat(req.InputPath); err == nil && info.Size() > maxPreviewBytes {
		preview, err := a.buildPreviewFromConverter(req.InputPath)
		if err == nil && preview.Success && preview.DataURL != "" {
			return preview, nil
		}
		return models.PreviewResult{Success: false, Error: "PREVIEW_SKIPPED"}, nil
	}

	data, err := os.ReadFile(req.InputPath)
	if err != nil {
		return models.PreviewResult{Success: false, Error: err.Error()}, err
	}

	mimeType := detectPreviewMimeType(data, req.InputPath)
	dataURL := buildDataURL(data, mimeType)
	return models.PreviewResult{Success: true, DataURL: dataURL}, nil
}

// EditMetadata edits image metadata (EXIF)
func (a *App) EditMetadata(req models.MetadataEditRequest) (models.MetadataEditResult, error) {
	return a.infoViewerService.EditMetadata(req)
}

func (a *App) StripMetadata(req models.MetadataStripRequest) (models.MetadataStripResult, error) {
	return a.metadataService.StripMetadata(req)
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
	return a.watermarkService.AddWatermark(req)
}

// AddWatermarkBatch adds watermarks to multiple images concurrently
func (a *App) AddWatermarkBatch(requests []models.WatermarkRequest) ([]models.WatermarkResult, error) {
	n := len(requests)
	results := make([]models.WatermarkResult, n)
	if n == 0 {
		return results, nil
	}
	var errsMu sync.Mutex
	var errs []error
	workers := a.settings.MaxConcurrency
	if workers < 1 {
		workers = 1
	}
	if workers > 32 {
		workers = 32
	}
	if workers > n {
		workers = n
	}

	jobs := make(chan int, workers)
	var wg sync.WaitGroup
	for w := 0; w < workers; w++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for idx := range jobs {
				res, err := a.watermarkService.AddWatermark(requests[idx])
				results[idx] = res
				if err != nil {
					errsMu.Lock()
					errs = append(errs, fmt.Errorf("watermark[%d]: %w", idx, err))
					errsMu.Unlock()
				}
			}
		}()
	}
	for i := 0; i < n; i++ {
		jobs <- i
	}
	close(jobs)
	wg.Wait()
	if len(errs) > 0 {
		return results, errors.Join(errs...)
	}
	return results, nil
}

// Adjust applies adjustments to an image
func (a *App) Adjust(req models.AdjustRequest) (models.AdjustResult, error) {
	return a.adjusterService.Adjust(req)
}

// AdjustBatch applies adjustments to multiple images concurrently
func (a *App) AdjustBatch(requests []models.AdjustRequest) ([]models.AdjustResult, error) {
	n := len(requests)
	results := make([]models.AdjustResult, n)
	if n == 0 {
		return results, nil
	}
	var errsMu sync.Mutex
	var errs []error
	workers := a.settings.MaxConcurrency
	if workers < 1 {
		workers = 1
	}
	if workers > 32 {
		workers = 32
	}
	if workers > n {
		workers = n
	}

	jobs := make(chan int, workers)
	var wg sync.WaitGroup
	for w := 0; w < workers; w++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for idx := range jobs {
				res, err := a.adjusterService.Adjust(requests[idx])
				results[idx] = res
				if err != nil {
					errsMu.Lock()
					errs = append(errs, fmt.Errorf("adjust[%d]: %w", idx, err))
					errsMu.Unlock()
				}
			}
		}()
	}
	for i := 0; i < n; i++ {
		jobs <- i
	}
	close(jobs)
	wg.Wait()
	if len(errs) > 0 {
		return results, errors.Join(errs...)
	}
	return results, nil
}

// ApplyFilter applies a filter to an image
func (a *App) ApplyFilter(req models.FilterRequest) (models.FilterResult, error) {
	return a.filterService.ApplyFilter(req)
}

// ApplyFilterBatch applies filters to multiple images concurrently
func (a *App) ApplyFilterBatch(requests []models.FilterRequest) ([]models.FilterResult, error) {
	n := len(requests)
	results := make([]models.FilterResult, n)
	if n == 0 {
		return results, nil
	}
	var errsMu sync.Mutex
	var errs []error
	workers := a.settings.MaxConcurrency
	if workers < 1 {
		workers = 1
	}
	if workers > 32 {
		workers = 32
	}
	if workers > n {
		workers = n
	}

	jobs := make(chan int, workers)
	var wg sync.WaitGroup
	for w := 0; w < workers; w++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for idx := range jobs {
				res, err := a.filterService.ApplyFilter(requests[idx])
				results[idx] = res
				if err != nil {
					errsMu.Lock()
					errs = append(errs, fmt.Errorf("filter[%d]: %w", idx, err))
					errsMu.Unlock()
				}
			}
		}()
	}
	for i := 0; i < n; i++ {
		jobs <- i
	}
	close(jobs)
	wg.Wait()
	if len(errs) > 0 {
		return results, errors.Join(errs...)
	}
	return results, nil
}
