package utils

import (
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"strings"
)

// ExtractEmbeddedPythonScripts writes embedded python files into the user cache directory and returns its path.
func ExtractEmbeddedPythonScripts(embedded fs.FS, embeddedRoot string) (string, error) {
	destRoot, err := embeddedExtractCacheRoot("python")
	if err != nil {
		return "", err
	}
	if err := os.MkdirAll(destRoot, 0755); err != nil {
		return "", fmt.Errorf("create embedded python dir: %w", err)
	}

	err = fs.WalkDir(embedded, embeddedRoot, func(p string, d fs.DirEntry, walkErr error) error {
		if walkErr != nil {
			return walkErr
		}
		if d.IsDir() {
			return nil
		}
		if filepath.Base(p) == ".keep" {
			return nil
		}

		rel := strings.TrimPrefix(p, embeddedRoot)
		rel = strings.TrimPrefix(rel, "/")
		if rel == "" {
			return nil
		}
		destPath := filepath.Join(destRoot, filepath.FromSlash(rel))
		if err := os.MkdirAll(filepath.Dir(destPath), 0755); err != nil {
			return err
		}

		if st, err := d.Info(); err == nil {
			if existing, err := os.Stat(destPath); err == nil && existing.Size() == st.Size() {
				same, err := sameEmbeddedFileContent(embedded, p, destPath)
				if err == nil && same {
					return nil
				}
			}
		}

		data, err := fs.ReadFile(embedded, p)
		if err != nil {
			return err
		}
		return os.WriteFile(destPath, data, 0644)
	})
	if err != nil {
		return "", fmt.Errorf("extract embedded python scripts: %w", err)
	}

	if !isPythonScriptsDir(destRoot) {
		return "", fmt.Errorf("embedded python scripts incomplete at %s", destRoot)
	}
	return destRoot, nil
}
