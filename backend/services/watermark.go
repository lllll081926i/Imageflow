package services

import (
	"errors"
	"fmt"

	"github.com/imageflow/backend/models"
	"github.com/imageflow/backend/utils"
)

// WatermarkService handles watermark application
type WatermarkService struct {
	executor utils.PythonRunner
	logger   *utils.Logger
}

// NewWatermarkService creates a new watermark service
func NewWatermarkService(executor utils.PythonRunner, logger *utils.Logger) *WatermarkService {
	return &WatermarkService{
		executor: executor,
		logger:   logger,
	}
}

// AddWatermark adds a watermark to an image
func (s *WatermarkService) AddWatermark(req models.WatermarkRequest) (models.WatermarkResult, error) {
	s.logger.Info("Adding watermark to image: %s -> %s (type: %s)", req.InputPath, req.OutputPath, req.WatermarkType)

	position := req.Position
	if position == "tl" {
		position = "top-left"
	} else if position == "tc" {
		position = "top-center"
	} else if position == "tr" {
		position = "top-right"
	} else if position == "cl" {
		position = "center-left"
	} else if position == "c" {
		position = "center"
	} else if position == "cr" {
		position = "center-right"
	} else if position == "bl" {
		position = "bottom-left"
	} else if position == "bc" {
		position = "bottom-center"
	} else if position == "br" {
		position = "bottom-right"
	}

	payload := map[string]interface{}{
		"type":            req.WatermarkType,
		"input_path":      req.InputPath,
		"output_path":     req.OutputPath,
		"text":            req.Text,
		"watermark_path":  req.ImagePath,
		"position":        position,
		"opacity":         req.Opacity,
		"watermark_scale": req.Scale,
		"font_size":       req.FontSize,
		"font_color":      req.FontColor,
		"rotation":        req.Rotation,
	}

	var result models.WatermarkResult
	err := s.executor.ExecuteAndParse("watermark.py", payload, &result)
	if err != nil {
		s.logger.Error("Watermark application failed: %v", err)
		return models.WatermarkResult{Success: false, Error: err.Error()}, err
	}

	if !result.Success {
		s.logger.Error("Watermark application failed: %s", result.Error)
		return result, fmt.Errorf("watermark application failed: %s", result.Error)
	}

	s.logger.Info("Watermark applied successfully")
	return result, nil
}

// AddWatermarkBatch adds watermarks to multiple images concurrently
func (s *WatermarkService) AddWatermarkBatch(requests []models.WatermarkRequest) ([]models.WatermarkResult, error) {
	s.logger.Info("Starting batch watermark application for %d images", len(requests))

	results := make([]models.WatermarkResult, len(requests))
	var errs []error
	for i, req := range requests {
		res, err := s.AddWatermark(req)
		results[i] = res
		if err != nil {
			errs = append(errs, fmt.Errorf("watermark[%d]: %w", i, err))
		}
	}

	s.logger.Info("Batch watermark application completed")
	if len(errs) > 0 {
		return results, errors.Join(errs...)
	}
	return results, nil
}
