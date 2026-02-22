package utils

import (
	"bytes"
	"crypto/sha256"
	"io"
	"io/fs"
	"os"
	"path/filepath"
	"strings"
)

const embeddedCacheRootEnv = "IMAGEFLOW_EMBEDDED_CACHE_ROOT"

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

func embeddedExtractCacheRoot(subdir string) (string, error) {
	if override := strings.TrimSpace(os.Getenv(embeddedCacheRootEnv)); override != "" {
		return filepath.Join(override, subdir), nil
	}

	cacheDir, err := os.UserCacheDir()
	if err != nil || cacheDir == "" {
		cacheDir = os.TempDir()
	}
	return filepath.Join(cacheDir, "ImageFlow", subdir), nil
}

func sameEmbeddedFileContent(embedded fs.FS, embeddedPath, destPath string) (bool, error) {
	embeddedFile, err := embedded.Open(embeddedPath)
	if err != nil {
		return false, err
	}
	defer embeddedFile.Close()

	destFile, err := os.Open(destPath)
	if err != nil {
		return false, err
	}
	defer destFile.Close()

	embeddedHash := sha256.New()
	if _, err := io.Copy(embeddedHash, embeddedFile); err != nil {
		return false, err
	}

	destHash := sha256.New()
	if _, err := io.Copy(destHash, destFile); err != nil {
		return false, err
	}

	return bytes.Equal(embeddedHash.Sum(nil), destHash.Sum(nil)), nil
}
