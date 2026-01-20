package utils

import (
	"os"
	"path/filepath"
)

func embeddedExtractRoot(subdir string) (string, error) {
	if exe, err := os.Executable(); err == nil {
		exeDir := filepath.Dir(exe)
		if exeDir != "" {
			return filepath.Join(exeDir, subdir), nil
		}
	}

	cacheDir, err := os.UserCacheDir()
	if err != nil || cacheDir == "" {
		cacheDir = os.TempDir()
	}
	return filepath.Join(cacheDir, "ImageFlow", subdir), nil
}
