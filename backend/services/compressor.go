package services

import (
	"fmt"
	"github.com/imageflow/backend/models"
	"github.com/imageflow/backend/utils"
)

// CompressorService handles image compression
type CompressorService struct {
	executor *utils.PythonExecutor
	logger   *utils.Logger
}

// NewCompressorService creates a new compressor service
func NewCompressorService(executor *utils.PythonExecutor, logger *utils.Logger) *CompressorService {
	return &CompressorService{
		executor: executor,
		logger:   logger,
	}
}

// Compress compresses an image
func (s *CompressorService) Compress(req models.CompressRequest) (models.CompressResult, error) {
	s.logger.Info("Compressing image: %s -> %s (mode: %s)", req.InputPath, req.OutputPath, req.Mode)

	var result models.CompressResult
	err := s.executor.ExecuteAndParse("compressor.py", req, &result)
	if err != nil {
		s.logger.Error("Compression failed: %v", err)
		return models.CompressResult{Success: false, Error: err.Error()}, err
	}

	if !result.Success {
		s.logger.Error("Compression failed: %s", result.Error)
		return result, fmt.Errorf("compression failed: %s", result.Error)
	}

	s.logger.Info("Compression completed: %.2f%% reduction", result.CompressionRate)
	return result, nil
}

// CompressBatch compresses multiple images concurrently
func (s *CompressorService) CompressBatch(requests []models.CompressRequest) ([]models.CompressResult, error) {
	s.logger.Info("Starting batch compression of %d images", len(requests))

	results := make([]models.CompressResult, len(requests))
	resultChan := make(chan struct {
		index  int
		result models.CompressResult
	}, len(requests))

	// Process images concurrently
	for i, req := range requests {
		go func(idx int, r models.CompressRequest) {
			result, _ := s.Compress(r)
			resultChan <- struct {
				index  int
				result models.CompressResult
			}{idx, result}
		}(i, req)
	}

	// Collect results
	for i := 0; i < len(requests); i++ {
		res := <-resultChan
		results[res.index] = res.result
	}

	s.logger.Info("Batch compression completed")
	return results, nil
}
