package services

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/imageflow/backend/models"
	"github.com/imageflow/backend/utils"
)

// ConverterService handles image format conversion
type ConverterService struct {
	executor utils.PythonRunner
	logger   *utils.Logger
}

// NewConverterService creates a new converter service
func NewConverterService(executor utils.PythonRunner, logger *utils.Logger) *ConverterService {
	return &ConverterService{
		executor: executor,
		logger:   logger,
	}
}

// Convert converts an image to a different format
func (s *ConverterService) Convert(req models.ConvertRequest) (models.ConvertResult, error) {
	req.InputPath = resolveInputPath(req.InputPath, req.OutputPath)
	if req.OutputPath != "" && !filepath.IsAbs(req.OutputPath) {
		if abs, err := filepath.Abs(req.OutputPath); err == nil {
			req.OutputPath = abs
		}
	}

	s.logger.Info("Converting image: %s -> %s (format: %s)", req.InputPath, req.OutputPath, req.Format)

	if strings.EqualFold(filepath.Ext(req.InputPath), ".svg") {
		tmp, cleanup, err := utils.RasterizeSVGToTempPNG(req)
		if err != nil {
			s.logger.Warn("SVG rasterization failed, falling back to Python: %v", err)
		} else {
			defer cleanup()

			req.InputPath = tmp
			req.ResizeMode = ""
			req.ScalePercent = 0
			req.LongEdge = 0
			req.Width = 0
			req.Height = 0
		}
	}

	var result models.ConvertResult
	err := s.executor.ExecuteAndParse("converter.py", req, &result)
	if err != nil {
		s.logger.Error("Conversion failed: %v", err)
		return models.ConvertResult{Success: false, Error: err.Error()}, err
	}

	if !result.Success {
		s.logger.Error("Conversion failed: %s", result.Error)
		return result, fmt.Errorf("conversion failed: %s", result.Error)
	}

	s.logger.Info("Conversion completed successfully")
	return result, nil
}

func resolveInputPath(inputPath, outputPath string) string {
	cleaned := strings.TrimSpace(inputPath)
	if cleaned == "" {
		return inputPath
	}
	if filepath.IsAbs(cleaned) {
		return cleaned
	}

	candidates := []string{}
	if outputPath != "" && filepath.IsAbs(outputPath) {
		candidates = append(candidates, filepath.Join(filepath.Dir(outputPath), cleaned))
	}
	if wd, err := os.Getwd(); err == nil && wd != "" {
		candidates = append(candidates, filepath.Join(wd, cleaned))
	}
	if exe, err := os.Executable(); err == nil {
		exeDir := filepath.Dir(exe)
		if exeDir != "" {
			candidates = append(candidates, filepath.Join(exeDir, cleaned))
		}
	}

	for _, candidate := range candidates {
		if _, err := os.Stat(candidate); err == nil {
			return candidate
		}
	}

	if abs, err := filepath.Abs(cleaned); err == nil {
		return abs
	}
	return cleaned
}

// ConvertBatch converts multiple images concurrently
func (s *ConverterService) ConvertBatch(requests []models.ConvertRequest) ([]models.ConvertResult, error) {
	s.logger.Info("Starting batch conversion of %d images", len(requests))

	results := make([]models.ConvertResult, 0, len(requests))
	var errs []error
	for _, req := range requests {
		res, err := s.Convert(req)
		results = append(results, res)
		if err != nil {
			errs = append(errs, err)
		}
	}

	s.logger.Info("Batch conversion completed")
	if len(errs) > 0 {
		return results, errors.Join(errs...)
	}
	return results, nil
}
