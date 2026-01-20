package services

import (
	"errors"
	"fmt"
	"github.com/imageflow/backend/models"
	"github.com/imageflow/backend/utils"
	"path/filepath"
	"strings"
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
	s.logger.Info("Converting image: %s -> %s (format: %s)", req.InputPath, req.OutputPath, req.Format)

	if strings.EqualFold(filepath.Ext(req.InputPath), ".svg") {
		tmp, cleanup, err := utils.RasterizeSVGToTempPNG(req)
		if err != nil {
			s.logger.Error("SVG rasterization failed: %v", err)
			return models.ConvertResult{Success: false, Error: err.Error()}, err
		}
		defer cleanup()

		req.InputPath = tmp
		req.ResizeMode = ""
		req.ScalePercent = 0
		req.LongEdge = 0
		req.Width = 0
		req.Height = 0
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
