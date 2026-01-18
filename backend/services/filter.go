package services

import (
	"fmt"
	"github.com/imageflow/backend/models"
	"github.com/imageflow/backend/utils"
)

// FilterService handles image filter application
type FilterService struct {
	executor *utils.PythonExecutor
	logger   *utils.Logger
}

// NewFilterService creates a new filter service
func NewFilterService(executor *utils.PythonExecutor, logger *utils.Logger) *FilterService {
	return &FilterService{
		executor: executor,
		logger:   logger,
	}
}

// ApplyFilter applies a filter to an image
func (s *FilterService) ApplyFilter(req models.FilterRequest) (models.FilterResult, error) {
	s.logger.Info("Applying filter to image: %s -> %s (filter: %s)", req.InputPath, req.OutputPath, req.FilterType)

	var result models.FilterResult
	err := s.executor.ExecuteAndParse("filter.py", req, &result)
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
	resultChan := make(chan struct {
		index  int
		result models.FilterResult
	}, len(requests))

	// Process images concurrently
	for i, req := range requests {
		go func(idx int, r models.FilterRequest) {
			result, _ := s.ApplyFilter(r)
			resultChan <- struct {
				index  int
				result models.FilterResult
			}{idx, result}
		}(i, req)
	}

	// Collect results
	for i := 0; i < len(requests); i++ {
		res := <-resultChan
		results[res.index] = res.result
	}

	s.logger.Info("Batch filter application completed")
	return results, nil
}
