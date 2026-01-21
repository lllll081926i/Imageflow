package services

import (
	"errors"
	"fmt"

	"github.com/imageflow/backend/models"
	"github.com/imageflow/backend/utils"
)

// FilterService handles image filter application
type FilterService struct {
	executor utils.PythonRunner
	logger   *utils.Logger
}

// NewFilterService creates a new filter service
func NewFilterService(executor utils.PythonRunner, logger *utils.Logger) *FilterService {
	return &FilterService{
		executor: executor,
		logger:   logger,
	}
}

// ApplyFilter applies a filter to an image
func (s *FilterService) ApplyFilter(req models.FilterRequest) (models.FilterResult, error) {
	s.logger.Info("Applying filter to image: %s -> %s (filter: %s)", req.InputPath, req.OutputPath, req.FilterType)

	payload := map[string]interface{}{
		"input_path":  req.InputPath,
		"output_path": req.OutputPath,
		"filter":      req.FilterType,
		"intensity":   req.Intensity,
	}

	var result models.FilterResult
	err := s.executor.ExecuteAndParse("filter.py", payload, &result)
	if err != nil {
		s.logger.Error("Filter application failed: %v", err)
		return models.FilterResult{Success: false, Error: err.Error()}, err
	}

	if !result.Success {
		s.logger.Error("Filter application failed: %s", result.Error)
		return result, fmt.Errorf("filter application failed: %s", result.Error)
	}

	s.logger.Info("Filter applied successfully")
	return result, nil
}

// ApplyFilterBatch applies filters to multiple images concurrently
func (s *FilterService) ApplyFilterBatch(requests []models.FilterRequest) ([]models.FilterResult, error) {
	s.logger.Info("Starting batch filter application for %d images", len(requests))

	results := make([]models.FilterResult, len(requests))
	var errs []error
	for i, req := range requests {
		res, err := s.ApplyFilter(req)
		results[i] = res
		if err != nil {
			errs = append(errs, fmt.Errorf("filter[%d]: %w", i, err))
		}
	}

	s.logger.Info("Batch filter application completed")
	if len(errs) > 0 {
		return results, errors.Join(errs...)
	}
	return results, nil
}
