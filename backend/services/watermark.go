package services

import (
	"fmt"
	"github.com/imageflow/backend/models"
	"github.com/imageflow/backend/utils"
)

// WatermarkService handles watermark application
type WatermarkService struct {
	executor *utils.PythonExecutor
	logger   *utils.Logger
}

// NewWatermarkService creates a new watermark service
func NewWatermarkService(executor *utils.PythonExecutor, logger *utils.Logger) *WatermarkService {
	return &WatermarkService{
		executor: executor,
		logger:   logger,
	}
}

// AddWatermark adds a watermark to an image
func (s *WatermarkService) AddWatermark(req models.WatermarkRequest) (models.WatermarkResult, error) {
	s.logger.Info("Adding watermark to image: %s -> %s (type: %s)", req.InputPath, req.OutputPath, req.WatermarkType)

	var result models.WatermarkResult
	err := s.executor.ExecuteAndParse("watermark.py", req, &result)
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
	resultChan := make(chan struct {
		index  int
		result models.WatermarkResult
	}, len(requests))

	// Process images concurrently
	for i, req := range requests {
		go func(idx int, r models.WatermarkRequest) {
			result, _ := s.AddWatermark(r)
			resultChan <- struct {
				index  int
				result models.WatermarkResult
			}{idx, result}
		}(i, req)
	}

	// Collect results
	for i := 0; i < len(requests); i++ {
		res := <-resultChan
		results[res.index] = res.result
	}

	s.logger.Info("Batch watermark application completed")
	return results, nil
}
