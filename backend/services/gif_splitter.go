package services

import (
	"fmt"
	"strings"

	"github.com/imageflow/backend/models"
	"github.com/imageflow/backend/utils"
)

// GIFSplitterService handles GIF-related operations
type GIFSplitterService struct {
	executor utils.PythonRunner
	logger   *utils.Logger
}

// NewGIFSplitterService creates a new GIF splitter service
func NewGIFSplitterService(executor utils.PythonRunner, logger *utils.Logger) *GIFSplitterService {
	return &GIFSplitterService{
		executor: executor,
		logger:   logger,
	}
}

// SplitGIF processes GIF-related actions (export_frames, reverse, change_speed, build_gif, compress)
func (s *GIFSplitterService) SplitGIF(req models.GIFSplitRequest) (models.GIFSplitResult, error) {
	action := strings.ToLower(strings.TrimSpace(req.Action))
	if action == "" {
		action = "export_frames"
	}

	payload := map[string]interface{}{
		"action":      action,
		"input_path":  strings.TrimSpace(req.InputPath),
		"input_paths": req.InputPaths,
		"output_dir":  strings.TrimSpace(req.OutputDir),
		"output_path": strings.TrimSpace(req.OutputPath),
		"speed_factor": func() interface{} {
			if req.SpeedFactor == 0 {
				return nil
			}
			return req.SpeedFactor
		}(),
		"fps": func() interface{} {
			if req.FPS == 0 {
				return nil
			}
			return req.FPS
		}(),
		"quality": func() interface{} {
			if req.Quality == 0 {
				return nil
			}
			return req.Quality
		}(),
		"loop": req.Loop,
	}

	if action == "build_gif" && len(req.InputPaths) == 0 && req.InputPath != "" {
		payload["input_paths"] = []string{strings.TrimSpace(req.InputPath)}
	}

	outputFormat := strings.TrimSpace(req.OutputFormat)
	if outputFormat == "" {
		outputFormat = strings.TrimSpace(req.Format)
	}
	if outputFormat != "" {
		payload["output_format"] = outputFormat
	}

	frameRange := strings.TrimSpace(req.FrameRange)
	if frameRange == "" {
		if req.StartFrame != 0 || req.EndFrame != 0 {
			if req.EndFrame <= 0 {
				frameRange = fmt.Sprintf("%d", req.StartFrame)
			} else {
				frameRange = fmt.Sprintf("%d-%d", req.StartFrame, req.EndFrame)
			}
		}
	}
	if frameRange != "" {
		payload["frame_range"] = frameRange
	}

	s.logger.Info("Processing GIF action: %s", action)

	var result models.GIFSplitResult
	err := s.executor.ExecuteAndParse("gif_splitter.py", payload, &result)
	if err != nil {
		s.logger.Error("GIF processing failed: %v", err)
		return models.GIFSplitResult{Success: false, Error: err.Error()}, err
	}

	if !result.Success {
		s.logger.Error("GIF processing failed: %s", result.Error)
		return result, fmt.Errorf("GIF processing failed: %s", result.Error)
	}

	if action == "export_frames" {
		s.logger.Info("GIF exported successfully: %d frames", result.FrameCount)
	} else {
		s.logger.Info("GIF processing completed: %s", action)
	}
	return result, nil
}
