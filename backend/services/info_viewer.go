package services

import (
	"fmt"
	"github.com/imageflow/backend/models"
	"github.com/imageflow/backend/utils"
)

// InfoViewerService handles image information retrieval
type InfoViewerService struct {
	executor utils.PythonRunner
	logger   *utils.Logger
}

// NewInfoViewerService creates a new info viewer service
func NewInfoViewerService(executor utils.PythonRunner, logger *utils.Logger) *InfoViewerService {
	return &InfoViewerService{
		executor: executor,
		logger:   logger,
	}
}

// GetInfo retrieves image information including EXIF and histogram
func (s *InfoViewerService) GetInfo(req models.InfoRequest) (models.InfoResult, error) {
	s.logger.Info("Getting info for image: %s", req.InputPath)

	var result models.InfoResult
	err := s.executor.ExecuteAndParse("info_viewer.py", req, &result)
	if err != nil {
		s.logger.Error("Info retrieval failed: %v", err)
		return models.InfoResult{Success: false, Error: err.Error()}, err
	}

	if !result.Success {
		s.logger.Error("Info retrieval failed: %s", result.Error)
		return result, fmt.Errorf("info retrieval failed: %s", result.Error)
	}

	s.logger.Info("Info retrieved successfully: %dx%d %s", result.Width, result.Height, result.Format)
	return result, nil
}
