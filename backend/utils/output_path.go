package utils

import (
	"fmt"
	"os"
	"path/filepath"
)

func ResolveOutputPath(basePath string, reserved map[string]struct{}) (string, error) {
	if basePath == "" {
		return "", fmt.Errorf("base path is empty")
	}
	basePath = filepath.Clean(basePath)

	if !pathExists(basePath) && !isReserved(basePath, reserved) {
		return basePath, nil
	}

	dir := filepath.Dir(basePath)
	ext := filepath.Ext(basePath)
	base := filepath.Base(basePath[:len(basePath)-len(ext)])
	if base == "" {
		base = "output"
	}

	for i := 1; i < 10000; i++ {
		suffix := fmt.Sprintf("_%02d", i)
		candidate := filepath.Join(dir, fmt.Sprintf("%s%s%s", base, suffix, ext))
		if !pathExists(candidate) && !isReserved(candidate, reserved) {
			return candidate, nil
		}
	}

	return "", fmt.Errorf("failed to resolve unique output path")
}

func pathExists(path string) bool {
	_, err := os.Stat(path)
	return err == nil
}

func isReserved(path string, reserved map[string]struct{}) bool {
	if reserved == nil {
		return false
	}
	path = filepath.Clean(path)
	_, exists := reserved[path]
	return exists
}
