package services

import (
	"fmt"
	"github.com/imageflow/backend/models"
	"github.com/imageflow/backend/utils"
	"strings"
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

// GetInfo retrieves image information including metadata
func (s *InfoViewerService) GetInfo(req models.InfoRequest) (models.InfoResult, error) {
	var err error
	if req.InputPath, err = validateRequiredPath(req.InputPath, "输入文件"); err != nil {
		return models.InfoResult{Success: false, Error: err.Error()}, err
	}
	s.logger.Info("Getting info for image: %s", req.InputPath)

	var result models.InfoResult
	payload := map[string]interface{}{
		"action":     "get_info",
		"input_path": req.InputPath,
	}
	err = s.executor.ExecuteAndParse("info_viewer.py", payload, &result)
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

// EditMetadata updates EXIF metadata using piexif
func (s *InfoViewerService) EditMetadata(req models.MetadataEditRequest) (models.MetadataEditResult, error) {
	var err error
	if req.InputPath, err = validateRequiredPath(req.InputPath, "输入文件"); err != nil {
		return models.MetadataEditResult{Success: false, InputPath: req.InputPath, OutputPath: req.OutputPath, Error: err.Error()}, err
	}
	if req.Overwrite && strings.TrimSpace(req.OutputPath) == "" {
		req.OutputPath = req.InputPath
	} else if req.OutputPath, err = validateRequiredPath(req.OutputPath, "输出文件"); err != nil {
		return models.MetadataEditResult{Success: false, InputPath: req.InputPath, OutputPath: req.OutputPath, Error: err.Error()}, err
	}
	s.logger.Info("Editing metadata: %s -> %s (overwrite=%v)", req.InputPath, req.OutputPath, req.Overwrite)

	var result models.MetadataEditResult
	payload := map[string]interface{}{
		"action":      "edit_exif",
		"input_path":  req.InputPath,
		"output_path": req.OutputPath,
		"exif_data":   req.ExifData,
		"overwrite":   req.Overwrite,
	}

	err = s.executor.ExecuteAndParse("info_viewer.py", payload, &result)
	if err != nil {
		s.logger.Error("Metadata edit failed: %v", err)
		return models.MetadataEditResult{Success: false, Error: err.Error()}, err
	}

	if !result.Success {
		s.logger.Error("Metadata edit failed: %s", result.Error)
		return result, fmt.Errorf("metadata edit failed: %s", result.Error)
	}

	if result.InputPath == "" {
		result.InputPath = req.InputPath
	}
	if result.OutputPath == "" {
		result.OutputPath = req.OutputPath
	}

	return result, nil
}
