package main

import (
	"context"
	"fmt"

	"github.com/imageflow/backend/models"
	"github.com/imageflow/backend/services"
	"github.com/imageflow/backend/utils"
	"github.com/wailsapp/wails/v2/pkg/runtime"
)

// App struct
type App struct {
	ctx context.Context

	// Utilities
	logger   *utils.Logger
	executor *utils.PythonExecutor

	// Services
	converterService    *services.ConverterService
	compressorService   *services.CompressorService
	pdfGeneratorService *services.PDFGeneratorService
	gifSplitterService  *services.GIFSplitterService
	infoViewerService   *services.InfoViewerService
	watermarkService    *services.WatermarkService
	adjusterService     *services.AdjusterService
	filterService       *services.FilterService
}

// NewApp creates a new App application struct
func NewApp() *App {
	return &App{}
}

// startup is called when the app starts
func (a *App) startup(ctx context.Context) {
	a.ctx = ctx

	// Initialize logger
	logger, err := utils.NewLogger(utils.InfoLevel, true)
	if err != nil {
		fmt.Printf("Failed to initialize logger: %v\n", err)
		return
	}
	a.logger = logger
	a.logger.Info("ImageFlow backend starting...")

	scriptsDir, err := utils.ResolvePythonScriptsDir()
	if err != nil {
		a.logger.Error("Failed to resolve Python scripts directory: %v", err)
		return
	}
	a.logger.Info("Python scripts directory: %s", scriptsDir)

	// Initialize Python executor
	executor, err := utils.NewPythonExecutor(scriptsDir, logger)
	if err != nil {
		a.logger.Error("Failed to initialize Python executor: %v", err)
		return
	}
	a.executor = executor

	// Initialize all services
	a.converterService = services.NewConverterService(executor, logger)
	a.compressorService = services.NewCompressorService(executor, logger)
	a.pdfGeneratorService = services.NewPDFGeneratorService(executor, logger)
	a.gifSplitterService = services.NewGIFSplitterService(executor, logger)
	a.infoViewerService = services.NewInfoViewerService(executor, logger)
	a.watermarkService = services.NewWatermarkService(executor, logger)
	a.adjusterService = services.NewAdjusterService(executor, logger)
	a.filterService = services.NewFilterService(executor, logger)

	a.logger.Info("All services initialized successfully")
}

// shutdown is called when the app is closing
func (a *App) shutdown(ctx context.Context) {
	if a.logger != nil {
		a.logger.Info("ImageFlow backend shutting down...")
		a.logger.Close()
	}
}

func (a *App) Ping() string {
	return "pong"
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
	return a.converterService.ConvertBatch(requests)
}

// Compress compresses an image
func (a *App) Compress(req models.CompressRequest) (models.CompressResult, error) {
	return a.compressorService.Compress(req)
}

// CompressBatch compresses multiple images concurrently
func (a *App) CompressBatch(requests []models.CompressRequest) ([]models.CompressResult, error) {
	return a.compressorService.CompressBatch(requests)
}

// GeneratePDF generates a PDF from multiple images
func (a *App) GeneratePDF(req models.PDFRequest) (models.PDFResult, error) {
	return a.pdfGeneratorService.GeneratePDF(req)
}

// SplitGIF splits a GIF into individual frames
func (a *App) SplitGIF(req models.GIFSplitRequest) (models.GIFSplitResult, error) {
	return a.gifSplitterService.SplitGIF(req)
}

// GetInfo retrieves image information
func (a *App) GetInfo(req models.InfoRequest) (models.InfoResult, error) {
	return a.infoViewerService.GetInfo(req)
}

// AddWatermark adds a watermark to an image
func (a *App) AddWatermark(req models.WatermarkRequest) (models.WatermarkResult, error) {
	return a.watermarkService.AddWatermark(req)
}

// AddWatermarkBatch adds watermarks to multiple images concurrently
func (a *App) AddWatermarkBatch(requests []models.WatermarkRequest) ([]models.WatermarkResult, error) {
	return a.watermarkService.AddWatermarkBatch(requests)
}

// Adjust applies adjustments to an image
func (a *App) Adjust(req models.AdjustRequest) (models.AdjustResult, error) {
	return a.adjusterService.Adjust(req)
}

// AdjustBatch applies adjustments to multiple images concurrently
func (a *App) AdjustBatch(requests []models.AdjustRequest) ([]models.AdjustResult, error) {
	return a.adjusterService.AdjustBatch(requests)
}

// ApplyFilter applies a filter to an image
func (a *App) ApplyFilter(req models.FilterRequest) (models.FilterResult, error) {
	return a.filterService.ApplyFilter(req)
}

// ApplyFilterBatch applies filters to multiple images concurrently
func (a *App) ApplyFilterBatch(requests []models.FilterRequest) ([]models.FilterResult, error) {
	return a.filterService.ApplyFilterBatch(requests)
}
