package services

import (
	"fmt"
	"strings"

	"github.com/imageflow/backend/models"
	"github.com/imageflow/backend/utils"
)

// SubtitleStitchService handles movie dialogue long-image generation.
type SubtitleStitchService struct {
	executor utils.PythonRunner
	logger   *utils.Logger
}

// NewSubtitleStitchService creates a subtitle stitch service.
func NewSubtitleStitchService(executor utils.PythonRunner, logger *utils.Logger) *SubtitleStitchService {
	return &SubtitleStitchService{
		executor: executor,
		logger:   logger,
	}
}

// Generate performs "first full frame + subtitle strips" stitching.
func (s *SubtitleStitchService) Generate(req models.SubtitleStitchRequest) (models.SubtitleStitchResult, error) {
	payload := map[string]interface{}{
		"action":               "subtitle_stitch",
		"input_paths":          req.InputPaths,
		"output_path":          strings.TrimSpace(req.OutputPath),
		"subtitle_crop_ratio":  req.SubtitleCropRatio,
		"header_keep_full":     req.HeaderKeepFull,
		"dedup_enabled":        req.DedupEnabled,
		"dedup_threshold":      req.DedupThreshold,
		"minimum_strip_height": req.MinimumStripHeight,
	}

	s.logger.Info("Processing subtitle stitch: %d input(s)", len(req.InputPaths))

	var result models.SubtitleStitchResult
	err := s.executor.ExecuteAndParse("subtitle_stitcher.py", payload, &result)
	if err != nil {
		s.logger.Error("Subtitle stitch failed: %v", err)
		return models.SubtitleStitchResult{Success: false, Error: err.Error()}, err
	}

	if !result.Success {
		s.logger.Error("Subtitle stitch failed: %s", result.Error)
		return result, fmt.Errorf("subtitle stitch failed: %s", result.Error)
	}

	s.logger.Info("Subtitle stitch completed: %s", result.OutputPath)
	return result, nil
}
