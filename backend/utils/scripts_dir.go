package utils

import (
	"errors"
	"os"
	"path/filepath"
)

func ResolvePythonScriptsDir() (string, error) {
	if v := os.Getenv("IMAGEFLOW_SCRIPTS_DIR"); v != "" {
		if filepath.IsAbs(v) {
			return v, nil
		}
		abs, err := filepath.Abs(v)
		if err != nil {
			return "", err
		}
		return abs, nil
	}

	candidates := make([]string, 0, 4)
	if wd, err := os.Getwd(); err == nil {
		candidates = append(candidates, wd)
	}
	if exe, err := os.Executable(); err == nil {
		candidates = append(candidates, filepath.Dir(exe))
	}

	for _, base := range candidates {
		if dir := findPythonDirUpwards(base, 8); dir != "" {
			return dir, nil
		}
	}

	return "", errors.New("python scripts directory not found; set IMAGEFLOW_SCRIPTS_DIR or ensure python folder exists")
}

func findPythonDirUpwards(start string, maxLevels int) string {
	current := start
	for i := 0; i <= maxLevels; i++ {
		try1 := filepath.Join(current, "python")
		if isPythonScriptsDir(try1) {
			abs, err := filepath.Abs(try1)
			if err == nil {
				return abs
			}
			return try1
		}

		parent := filepath.Dir(current)
		if parent == current {
			break
		}
		current = parent
	}
	return ""
}

func isPythonScriptsDir(dir string) bool {
	if st, err := os.Stat(dir); err != nil || !st.IsDir() {
		return false
	}
	if _, err := os.Stat(filepath.Join(dir, "converter.py")); err != nil {
		return false
	}
	return true
}
