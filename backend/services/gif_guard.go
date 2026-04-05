package services

import (
	"fmt"
	"path/filepath"
	"strings"

	"github.com/imageflow/backend/utils"
)

const gifGuardMessage = "GIF 文件暂不支持在该工具中处理，请使用 GIF 工具"

func rejectGIFPath(path string) error {
	trimmed := strings.TrimSpace(path)
	if trimmed == "" {
		return nil
	}
	if strings.EqualFold(filepath.Ext(trimmed), ".gif") {
		return fmt.Errorf(gifGuardMessage)
	}
	return nil
}

func rejectGIFPaths(paths []string) error {
	for _, path := range paths {
		if err := rejectGIFPath(path); err != nil {
			return err
		}
	}
	return nil
}

func validateRequiredPath(path, label string) (string, error) {
	trimmed := strings.TrimSpace(path)
	if err := utils.ValidateUserSuppliedPath(trimmed, false); err != nil {
		return "", fmt.Errorf("%s路径无效: %w", label, err)
	}
	return trimmed, nil
}

func validateOptionalPath(path, label string) (string, error) {
	trimmed := strings.TrimSpace(path)
	if err := utils.ValidateUserSuppliedPath(trimmed, true); err != nil {
		return "", fmt.Errorf("%s路径无效: %w", label, err)
	}
	return trimmed, nil
}

func validateRequiredPaths(paths []string, label string) ([]string, error) {
	if len(paths) == 0 {
		return nil, fmt.Errorf("%s路径不能为空", label)
	}

	normalized := make([]string, 0, len(paths))
	for i, path := range paths {
		value, err := validateRequiredPath(path, fmt.Sprintf("%s第 %d 项", label, i+1))
		if err != nil {
			return nil, err
		}
		normalized = append(normalized, value)
	}
	return normalized, nil
}

func validateOptionalPaths(paths []string, label string) ([]string, error) {
	if len(paths) == 0 {
		return nil, nil
	}

	normalized := make([]string, 0, len(paths))
	for i, path := range paths {
		value, err := validateRequiredPath(path, fmt.Sprintf("%s第 %d 项", label, i+1))
		if err != nil {
			return nil, err
		}
		normalized = append(normalized, value)
	}
	return normalized, nil
}
