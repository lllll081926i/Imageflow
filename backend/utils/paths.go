package utils

import (
	"errors"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"runtime"
	"sort"
	"strings"

	"github.com/imageflow/backend/models"
)

var supportedExtensions = map[string]bool{
	".jpg":  true,
	".jpeg": true,
	".png":  true,
	".webp": true,
	".gif":  true,
	".bmp":  true,
	".tiff": true,
	".tif":  true,
	".heic": true,
	".heif": true,
	".svg":  true,
}

func ValidateUserSuppliedPath(path string, allowEmpty bool) error {
	trimmed := strings.TrimSpace(path)
	if trimmed == "" {
		if allowEmpty {
			return nil
		}
		return errors.New("路径不能为空")
	}
	if strings.ContainsRune(trimmed, '\x00') {
		return errors.New("路径包含非法空字符")
	}

	cleaned := filepath.Clean(trimmed)
	if hasLeadingParentTraversal(cleaned) {
		return errors.New("不允许使用父级目录跳转路径")
	}
	return nil
}

func NormalizeUserSuppliedPath(path string) (string, error) {
	return normalizeUserSuppliedPath(path, false)
}

func NormalizeOptionalUserSuppliedPath(path string) (string, error) {
	return normalizeUserSuppliedPath(path, true)
}

func normalizeUserSuppliedPath(path string, allowEmpty bool) (string, error) {
	if err := ValidateUserSuppliedPath(path, allowEmpty); err != nil {
		return "", err
	}
	trimmed := strings.TrimSpace(path)
	if trimmed == "" {
		return "", nil
	}

	cleaned := filepath.Clean(trimmed)
	abs, err := filepath.Abs(cleaned)
	if err != nil {
		return "", err
	}
	return abs, nil
}

func hasLeadingParentTraversal(path string) bool {
	if filepath.IsAbs(path) {
		return false
	}
	normalized := strings.ReplaceAll(path, "\\", "/")
	return normalized == ".." || strings.HasPrefix(normalized, "../")
}

func isImageFile(path string) bool {
	ext := strings.ToLower(filepath.Ext(path))
	return supportedExtensions[ext]
}

func ExpandInputPaths(paths []string) (models.ExpandDroppedPathsResult, error) {
	var result models.ExpandDroppedPathsResult
	var files []models.DroppedFile

	for _, p := range paths {
		if strings.TrimSpace(p) == "" {
			continue
		}

		info, err := os.Stat(p)
		if err != nil {
			return models.ExpandDroppedPathsResult{}, fmt.Errorf("stat failed: %w", err)
		}

		if !info.IsDir() {
			if isImageFile(p) {
				files = append(files, models.DroppedFile{
					InputPath:     p,
					SourceRoot:    filepath.Dir(p),
					RelativePath:  filepath.Base(p),
					IsFromDirDrop: false,
					Size:          info.Size(),
					ModTime:       info.ModTime().Unix(),
				})
			}
			continue
		}

		result.HasDirectory = true

		root := p
		err = filepath.WalkDir(root, func(path string, d fs.DirEntry, walkErr error) error {
			if walkErr != nil {
				return walkErr
			}
			if d.IsDir() {
				return nil
			}

			if !isImageFile(path) {
				return nil
			}

			info, err := d.Info()
			if err != nil {
				// If we can't get info, just skip or use defaults
				return nil
			}

			rel, err := filepath.Rel(root, path)
			if err != nil {
				return err
			}

			files = append(files, models.DroppedFile{
				InputPath:     path,
				SourceRoot:    root,
				RelativePath:  filepath.ToSlash(rel),
				IsFromDirDrop: true,
				Size:          info.Size(),
				ModTime:       info.ModTime().Unix(),
			})
			return nil
		})
		if err != nil {
			return models.ExpandDroppedPathsResult{}, fmt.Errorf("walk dir failed: %w", err)
		}
	}

	sort.Slice(files, func(i, j int) bool {
		return strings.ToLower(files[i].InputPath) < strings.ToLower(files[j].InputPath)
	})

	result.Files = files
	return result, nil
}

func ListSystemFonts() ([]string, error) {
	if runtime.GOOS != "windows" {
		return []string{}, nil
	}

	winDir := os.Getenv("WINDIR")
	if winDir == "" {
		winDir = `C:\Windows`
	}
	fontDir := filepath.Join(winDir, "Fonts")
	entries, err := os.ReadDir(fontDir)
	if err != nil {
		return []string{}, err
	}

	allowed := map[string]bool{
		".ttf": true,
		".otf": true,
		".ttc": true,
	}

	fonts := make([]string, 0, len(entries))
	for _, entry := range entries {
		if entry.IsDir() {
			continue
		}
		name := entry.Name()
		ext := strings.ToLower(filepath.Ext(name))
		if !allowed[ext] {
			continue
		}
		fonts = append(fonts, filepath.Join(fontDir, name))
	}

	sort.Slice(fonts, func(i, j int) bool {
		return strings.ToLower(fonts[i]) < strings.ToLower(fonts[j])
	})

	return fonts, nil
}
