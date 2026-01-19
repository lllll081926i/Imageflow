package services

import (
	"fmt"
	"github.com/imageflow/backend/models"
	"github.com/imageflow/backend/utils"
)

// AdjusterService handles image adjustments
type AdjusterService struct {
	executor utils.PythonRunner
	logger   *utils.Logger
}

// NewAdjusterService creates a new adjuster service
func NewAdjusterService(executor utils.PythonRunner, logger *utils.Logger) *AdjusterService {
	return &AdjusterService{
		executor: executor,
		logger:   logger,
	}
}

// Adjust applies adjustments to an image
func (s *AdjusterService) Adjust(req models.AdjustRequest) (models.AdjustResult, error) {
	s.logger.Info("Adjusting image: %s -> %s", req.InputPath, req.OutputPath)

	var result models.AdjustResult
	err := s.executor.ExecuteAndParse("adjuster.py", req, &result)
	if err != nil {
		s.logger.Error("Image adjustment failed: %v", err)
		return models.AdjustResult{Success: false, Error: err.Error()}, err
	}

	if !result.Success {
		s.logger.Error("Image adjustment failed: %s", result.Error)
		return result, fmt.Errorf("image adjustment failed: %s", result.Error)
	}

	s.logger.Info("Image adjusted successfully")
	return result, nil
}

// AdjustBatch applies adjustments to multiple images concurrently
func (s *AdjusterService) AdjustBatch(requests []models.AdjustRequest) ([]models.AdjustResult, error) {
	s.logger.Info("Starting batch adjustment for %d images", len(requests))

	results := make([]models.AdjustResult, 0, len(requests))
	for _, req := range requests {
		res, _ := s.Adjust(req)
		results = append(results, res)
	}

	s.logger.Info("Batch adjustment completed")
	return results, nil
}
