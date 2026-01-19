package utils

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"

	"github.com/imageflow/backend/models"
)

func clampInt(v, minV, maxV int) int {
	if v < minV {
		return minV
	}
	if v > maxV {
		return maxV
	}
	return v
}

func normalizeSettings(s models.AppSettings) models.AppSettings {
	if s.MaxConcurrency == 0 {
		s.MaxConcurrency = models.DefaultAppSettings().MaxConcurrency
	}
	s.MaxConcurrency = clampInt(s.MaxConcurrency, 1, 32)
	return s
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

