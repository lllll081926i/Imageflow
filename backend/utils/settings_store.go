package utils

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/imageflow/backend/models"
)

const maxRecentPaths = 4

func clampInt(v, minV, maxV int) int {
	if v < minV {
		return minV
	}
	if v > maxV {
		return maxV
	}
	return v
}

func normalizeSavedPath(path string) string {
	trimmed := strings.TrimSpace(path)
	if trimmed == "" {
		return ""
	}
	if trimmed == "/" || trimmed == `\` {
		return trimmed
	}
	cleaned := strings.TrimRight(trimmed, "/\\")
	if len(cleaned) == 2 && cleaned[1] == ':' {
		return trimmed
	}
	if cleaned == "" {
		return trimmed
	}
	return cleaned
}

func normalizeRecentPaths(paths []string) []string {
	if len(paths) == 0 {
		return []string{}
	}

	normalized := make([]string, 0, maxRecentPaths)
	seen := make(map[string]struct{}, len(paths))
	for _, raw := range paths {
		path := normalizeSavedPath(raw)
		if path == "" {
			continue
		}
		key := strings.ToLower(path)
		if _, ok := seen[key]; ok {
			continue
		}
		seen[key] = struct{}{}
		normalized = append(normalized, path)
		if len(normalized) == maxRecentPaths {
			break
		}
	}
	return normalized
}

func normalizeSettings(s models.AppSettings) models.AppSettings {
	defaults := models.DefaultAppSettings()
	if s.MaxConcurrency == 0 {
		s.MaxConcurrency = defaults.MaxConcurrency
	}
	s.MaxConcurrency = clampInt(s.MaxConcurrency, 1, 32)
	if strings.TrimSpace(s.OutputPrefix) == "" {
		s.OutputPrefix = defaults.OutputPrefix
	}
	if strings.TrimSpace(s.OutputTemplate) == "" {
		s.OutputTemplate = defaults.OutputTemplate
	}
	if strings.TrimSpace(s.ConflictStrategy) == "" {
		s.ConflictStrategy = defaults.ConflictStrategy
	}
	if s.ConflictStrategy != "rename" {
		s.ConflictStrategy = defaults.ConflictStrategy
	}
	s.DefaultOutputDir = normalizeSavedPath(s.DefaultOutputDir)
	s.RecentInputDirs = normalizeRecentPaths(s.RecentInputDirs)
	s.RecentOutputDirs = normalizeRecentPaths(s.RecentOutputDirs)
	return s
}

func NormalizeSettings(s models.AppSettings) models.AppSettings {
	return normalizeSettings(s)
}

func settingsFilePath() (string, error) {
	dir, err := os.UserConfigDir()
	if err != nil {
		return "", err
	}
	appDir := filepath.Join(dir, "imageflow")
	return filepath.Join(appDir, "settings.json"), nil
}

func LoadSettings() (models.AppSettings, error) {
	fp, err := settingsFilePath()
	if err != nil {
		return models.DefaultAppSettings(), err
	}

	b, err := os.ReadFile(fp)
	if err != nil {
		if os.IsNotExist(err) {
			return models.DefaultAppSettings(), nil
		}
		return models.DefaultAppSettings(), err
	}

	var s models.AppSettings
	if err := json.Unmarshal(b, &s); err != nil {
		return models.DefaultAppSettings(), fmt.Errorf("failed to parse settings: %w", err)
	}
	return normalizeSettings(s), nil
}

func SaveSettings(s models.AppSettings) (models.AppSettings, error) {
	fp, err := settingsFilePath()
	if err != nil {
		return normalizeSettings(s), err
	}

	s = normalizeSettings(s)

	if err := os.MkdirAll(filepath.Dir(fp), 0o755); err != nil {
		return s, err
	}

	b, err := json.MarshalIndent(s, "", "  ")
	if err != nil {
		return s, err
	}
	if err := os.WriteFile(fp, b, 0o644); err != nil {
		return s, err
	}
	return s, nil
}
