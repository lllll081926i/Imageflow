package utils

import (
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"runtime"
	"strings"
)

// ExtractEmbeddedPythonRuntime writes embedded runtime files next to the executable and returns its path.
func ExtractEmbeddedPythonRuntime(embedded fs.FS, embeddedRoot string) (string, error) {
	destRoot, err := embeddedExtractRoot("python_runtime")
	if err != nil {
		return "", err
	}
	if err := os.MkdirAll(destRoot, 0755); err != nil {
		return "", fmt.Errorf("create embedded python runtime dir: %w", err)
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
				return nil
			}
		}

		data, err := fs.ReadFile(embedded, p)
		if err != nil {
			return err
		}
		return os.WriteFile(destPath, data, 0644)
	})
	if err != nil {
		return "", fmt.Errorf("extract embedded python runtime: %w", err)
	}

	if pythonExe := PythonExecutableFromRuntime(destRoot); pythonExe == "" {
		return "", fmt.Errorf("embedded python runtime incomplete at %s", destRoot)
	}

	return destRoot, nil
}

func PythonExecutableFromRuntime(runtimeDir string) string {
	if runtime.GOOS == "windows" {
		candidates := []string{
			filepath.Join(runtimeDir, "python.exe"),
			filepath.Join(runtimeDir, "pythonw.exe"),
			filepath.Join(runtimeDir, "python3.exe"),
		}
		for _, c := range candidates {
			if st, err := os.Stat(c); err == nil && !st.IsDir() {
				return c
			}
		}
		return ""
	}

	candidates := []string{
		filepath.Join(runtimeDir, "bin", "python3"),
		filepath.Join(runtimeDir, "bin", "python"),
		filepath.Join(runtimeDir, "python3"),
		filepath.Join(runtimeDir, "python"),
	}
	for _, c := range candidates {
		if st, err := os.Stat(c); err == nil && !st.IsDir() {
			return c
		}
	}
	return ""
}
