package services

import (
	"fmt"
	"github.com/imageflow/backend/models"
	"github.com/imageflow/backend/utils"
)

// GIFSplitterService handles GIF splitting into frames
type GIFSplitterService struct {
	executor *utils.PythonExecutor
	logger   *utils.Logger
}

// NewGIFSplitterService creates a new GIF splitter service
func NewGIFSplitterService(executor *utils.PythonExecutor, logger *utils.Logger) *GIFSplitterService {
	return &GIFSplitterService{
		executor: executor,
		logger:   logger,
	}
}

// SplitGIF splits a GIF into individual frames
func (s *GIFSplitterService) SplitGIF(req models.GIFSplitRequest) (models.GIFSplitResult, error) {
	s.logger.Info("Splitting GIF: %s -> %s", req.InputPath, req.OutputDir)

	var result models.GIFSplitResult
	err := s.executor.ExecuteAndParse("gif_splitter.py", req, &result)
	if err != nil {
		s.logger.Error("GIF splitting failed: %v", err)
		return models.GIFSplitResult{Success: false, Error: err.Error()}, err
	}

	if !result.Success {
		s.logger.Error("GIF splitting failed: %s", result.Error)
		return result, fmt.Errorf("GIF splitting failed: %s", result.Error)
	}

	s.logger.Info("GIF split successfully: %d frames", result.FrameCount)
	return result, nil
}
