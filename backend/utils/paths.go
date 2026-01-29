package utils

import (
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
