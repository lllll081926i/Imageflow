package services

import (
	"fmt"
	"github.com/imageflow/backend/models"
	"github.com/imageflow/backend/utils"
)

// CompressorService handles image compression
type CompressorService struct {
	executor utils.PythonRunner
	logger   *utils.Logger
}

// NewCompressorService creates a new compressor service
func NewCompressorService(executor utils.PythonRunner, logger *utils.Logger) *CompressorService {
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

	results := make([]models.CompressResult, 0, len(requests))
	for _, req := range requests {
		res, _ := s.Compress(req)
		results = append(results, res)
	}

	s.logger.Info("Batch compression completed")
	return results, nil
}
