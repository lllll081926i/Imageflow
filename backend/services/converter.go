package services

import (
	"fmt"
	"github.com/imageflow/backend/models"
	"github.com/imageflow/backend/utils"
)

// ConverterService handles image format conversion
type ConverterService struct {
	executor *utils.PythonExecutor
	logger   *utils.Logger
}

// NewConverterService creates a new converter service
func NewConverterService(executor *utils.PythonExecutor, logger *utils.Logger) *ConverterService {
	return &ConverterService{
		executor: executor,
		logger:   logger,
	}
}

// Convert converts an image to a different format
func (s *ConverterService) Convert(req models.ConvertRequest) (models.ConvertResult, error) {
	s.logger.Info("Converting image: %s -> %s (format: %s)", req.InputPath, req.OutputPath, req.Format)

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

	results := make([]models.ConvertResult, len(requests))
	resultChan := make(chan struct {
		index  int
		result models.ConvertResult
	}, len(requests))

	// Process images concurrently
	for i, req := range requests {
		go func(idx int, r models.ConvertRequest) {
			result, _ := s.Convert(r)
			resultChan <- struct {
				index  int
				result models.ConvertResult
			}{idx, result}
		}(i, req)
	}

	// Collect results
	for i := 0; i < len(requests); i++ {
		res := <-resultChan
		results[res.index] = res.result
	}

	s.logger.Info("Batch conversion completed")
	return results, nil
}
