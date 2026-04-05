package services

import (
	"fmt"
	"github.com/imageflow/backend/models"
	"github.com/imageflow/backend/utils"
	"strings"
)

type MetadataService struct {
	executor utils.PythonRunner
	logger   *utils.Logger
}

func NewMetadataService(executor utils.PythonRunner, logger *utils.Logger) *MetadataService {
	return &MetadataService{
		executor: executor,
		logger:   logger,
	}
}

func (s *MetadataService) StripMetadata(req models.MetadataStripRequest) (models.MetadataStripResult, error) {
	var err error
	if req.InputPath, err = validateRequiredPath(req.InputPath, "输入文件"); err != nil {
		return models.MetadataStripResult{Success: false, InputPath: req.InputPath, OutputPath: req.OutputPath, Error: err.Error()}, err
	}
	if req.Overwrite && strings.TrimSpace(req.OutputPath) == "" {
		req.OutputPath = req.InputPath
	} else if req.OutputPath, err = validateRequiredPath(req.OutputPath, "输出文件"); err != nil {
		return models.MetadataStripResult{Success: false, InputPath: req.InputPath, OutputPath: req.OutputPath, Error: err.Error()}, err
	}
	s.logger.Info("Stripping metadata: %s -> %s (overwrite=%v)", req.InputPath, req.OutputPath, req.Overwrite)

	payload := map[string]interface{}{
		"action":      "strip_metadata",
		"input_path":  req.InputPath,
		"output_path": req.OutputPath,
		"overwrite":   req.Overwrite,
	}

	var result models.MetadataStripResult
	err = s.executor.ExecuteAndParse("metadata_tool.py", payload, &result)
	if err != nil {
		s.logger.Error("Metadata strip failed: %v", err)
		return models.MetadataStripResult{Success: false, InputPath: req.InputPath, OutputPath: req.OutputPath, Error: err.Error()}, err
	}

	if !result.Success {
		s.logger.Error("Metadata strip failed: %s", result.Error)
		return result, fmt.Errorf("metadata strip failed: %s", result.Error)
	}

	s.logger.Info("Metadata stripped successfully")
	return result, nil
}
