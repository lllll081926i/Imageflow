package appcache

import (
	"context"
	"errors"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"

	"github.com/lllll081926i/imageflow-go/internal/paths"
)

const sessionMarker = ".imageflow-session"

type Report struct {
	Thumbnails int64
	Clipboard  int64
	Tools      int64
	Staging    int64
	InUse      int64
	Files      int
	Removed    int64
}

func (report Report) Total() int64 {
	return report.Thumbnails + report.Clipboard + report.Tools + report.Staging
}

type location struct {
	root string
	kind string
}

func locations(modelsRoot, userCache string) []location {
	var result []location
	if modelsRoot != "" {
		result = append(result, location{filepath.Join(modelsRoot, ".clipboard"), "clipboard"}, location{modelsRoot, "staging"})
	}
	if userCache != "" {
		result = append(result, location{filepath.Join(userCache, "imageflow", "thumb_cache"), "thumbnails"},
			location{filepath.Join(userCache, "imageflow", "tools"), "tools"})
	}
	return result
}

func safeDirectory(directory string) error {
	absolute, err := filepath.Abs(directory)
	if err != nil {
		return err
	}
	for current := absolute; ; current = filepath.Dir(current) {
		info, err := os.Lstat(current)
		if err != nil && !os.IsNotExist(err) {
			return err
		}
		if err == nil && (!info.IsDir() || info.Mode()&os.ModeSymlink != 0) {
			return fmt.Errorf("缓存路径不是普通目录：%s", current)
		}
		if filepath.Dir(current) == current {
			return nil
		}
	}
}

func knownFile(kind, relative string) bool {
	name := filepath.Base(relative)
	switch kind {
	case "clipboard":
		parent := filepath.Dir(relative)
		return (parent == "." || strings.HasPrefix(parent, "session-") && filepath.Base(parent) == parent) &&
			clipboardName(name)
	case "thumbnails":
		if filepath.Dir(relative) != "." || len(name) != 68 || !strings.HasSuffix(name, ".png") {
			return false
		}
		for _, character := range name[:64] {
			if !strings.ContainsRune("0123456789abcdef", character) {
				return false
			}
		}
		return true
	case "tools":
		parent := filepath.Dir(relative)
		return name == "oxipng.exe" && filepath.Base(parent) == parent && strings.HasPrefix(parent, "oxipng-9.1.5-") && len(strings.TrimPrefix(parent, "oxipng-9.1.5-")) == 12
	case "staging":
		parts := strings.Split(filepath.ToSlash(relative), "/")
		if len(parts) == 1 {
			return strings.HasSuffix(name, ".zip") && temporaryName(strings.TrimSuffix(name, ".zip"), ".imageflow-model-bundle-")
		}
		if !stagingDirectory(parts[0]) {
			return false
		}
		if len(parts) == 2 || len(parts) == 3 && parts[1] == "model" {
			return strings.HasSuffix(name, ".param") || strings.HasSuffix(name, ".bin")
		}
		return len(parts) == 3 && parts[1] == "engine" && (name == "upscayl-bin.exe" || name == "upscayl-bin" || name == "vcomp140.dll" || name == "vcomp140d.dll")
	}
	return false
}

func clipboardName(name string) bool {
	if !strings.HasPrefix(name, "image-") || !strings.HasSuffix(name, ".png") {
		return false
	}
	return temporaryName(strings.TrimSuffix(name, ".png"), "image-")
}

func stagingDirectory(name string) bool {
	return temporaryName(name, ".imageflow-upscayl-") || temporaryName(name, ".imageflow-custom-")
}

func temporaryName(name, prefix string) bool {
	if !strings.HasPrefix(name, prefix) {
		return false
	}
	number := strings.TrimPrefix(name, prefix)
	if number == "" {
		return false
	}
	for _, character := range number {
		if character < '0' || character > '9' {
			return false
		}
	}
	return true
}

func Inspect(ctx context.Context, modelsRoot, userCache string, protected []string, clean bool) (Report, error) {
	return inspect(ctx, locations(modelsRoot, userCache), protected, clean, time.Time{})
}

func PruneExpired(ctx context.Context, modelsRoot string, now time.Time) error {
	_, err := inspect(ctx, locations(modelsRoot, ""), nil, true, now.Add(-24*time.Hour))
	return err
}

func inspect(ctx context.Context, roots []location, protected []string, clean bool, olderThan time.Time) (Report, error) {
	var report Report
	var failures []error
	for _, entry := range roots {
		if err := safeDirectory(entry.root); err != nil {
			failures = append(failures, err)
			continue
		}
		var visited []string
		err := filepath.WalkDir(entry.root, func(path string, item fs.DirEntry, walkErr error) error {
			if ctx.Err() != nil {
				return ctx.Err()
			}
			if os.IsNotExist(walkErr) {
				return nil
			}
			if walkErr != nil {
				return walkErr
			}
			if item.IsDir() {
				if path == entry.root {
					return nil
				}
				relative, err := filepath.Rel(entry.root, path)
				if err == nil && entry.kind == "staging" {
					parts := strings.Split(filepath.ToSlash(relative), "/")
					if stagingDirectory(parts[0]) && (len(parts) == 1 || len(parts) == 2 && (parts[1] == "engine" || parts[1] == "model")) {
						visited = append(visited, path)
						return nil
					}
					return fs.SkipDir
				}
				if err != nil || filepath.Dir(relative) != "." {
					return fs.SkipDir
				}
				if entry.kind == "clipboard" && strings.HasPrefix(relative, "session-") {
					marker, err := os.Lstat(filepath.Join(path, sessionMarker))
					if err == nil && marker.Mode().IsRegular() {
						visited = append(visited, path)
						return nil
					}
				}
				if entry.kind == "tools" && strings.HasPrefix(relative, "oxipng-9.1.5-") {
					visited = append(visited, path)
					return nil
				}
				return fs.SkipDir
			}
			relative, err := filepath.Rel(entry.root, path)
			if err != nil || !knownFile(entry.kind, relative) || item.Type()&os.ModeSymlink != 0 {
				return nil
			}
			info, err := item.Info()
			if err != nil {
				return err
			}
			if !info.Mode().IsRegular() {
				return nil
			}
			inUse := false
			for _, directory := range protected {
				if paths.CheckContained(directory, path) == nil {
					inUse = true
					break
				}
			}
			if clean && !inUse && (olderThan.IsZero() || info.ModTime().Before(olderThan)) {
				if err := os.Remove(path); err == nil {
					report.Removed += info.Size()
					return nil
				} else {
					failures = append(failures, err)
				}
			}
			report.Files++
			if inUse {
				report.InUse += info.Size()
			}
			switch entry.kind {
			case "clipboard":
				report.Clipboard += info.Size()
			case "thumbnails":
				report.Thumbnails += info.Size()
			case "tools":
				report.Tools += info.Size()
			case "staging":
				report.Staging += info.Size()
			}
			return nil
		})
		if err != nil {
			failures = append(failures, err)
		}
		if clean && err == nil {
			for index := len(visited) - 1; index >= 0; index-- {
				directory := visited[index]
				inUse := false
				for _, current := range protected {
					inUse = inUse || paths.CheckContained(current, directory) == nil
				}
				if inUse {
					continue
				}
				children, err := os.ReadDir(directory)
				if err != nil {
					continue
				}
				if entry.kind == "clipboard" && len(children) == 1 && children[0].Name() == sessionMarker {
					info, err := children[0].Info()
					if err == nil && info.Mode().IsRegular() && (olderThan.IsZero() || info.ModTime().Before(olderThan)) {
						os.Remove(filepath.Join(directory, sessionMarker))
					}
				}
				os.Remove(directory)
			}
		}
	}
	return report, errors.Join(failures...)
}

type Session struct {
	mu     sync.Mutex
	dirs   map[string]string
	closed bool
}

func (session *Session) Use(modelsRoot string, save func(string) (string, error)) (string, error) {
	session.mu.Lock()
	defer session.mu.Unlock()
	if session.closed {
		return "", context.Canceled
	}
	if modelsRoot == "" {
		return "", fmt.Errorf("未配置临时图片目录")
	}
	root := filepath.Join(modelsRoot, ".clipboard")
	if err := safeDirectory(root); err != nil {
		return "", err
	}
	if session.dirs == nil {
		session.dirs = map[string]string{}
	}
	directory := session.dirs[root]
	if directory == "" {
		if err := os.MkdirAll(root, 0o700); err != nil {
			return "", err
		}
		var err error
		directory, err = os.MkdirTemp(root, "session-*")
		if err != nil {
			return "", err
		}
		if err := os.WriteFile(filepath.Join(directory, sessionMarker), []byte("ImageFlow 临时图片\n"), 0o600); err != nil {
			return "", err
		}
		session.dirs[root] = directory
	}
	return save(directory)
}

func (session *Session) Protected() []string {
	session.mu.Lock()
	defer session.mu.Unlock()
	var result []string
	for _, directory := range session.dirs {
		result = append(result, directory)
	}
	return result
}

func (session *Session) Close() error {
	session.mu.Lock()
	defer session.mu.Unlock()
	session.closed = true
	var failures []error
	for _, directory := range session.dirs {
		_, err := inspect(context.Background(), []location{{directory, "clipboard"}}, nil, true, time.Time{})
		if err != nil {
			failures = append(failures, err)
			continue
		}
		os.Remove(filepath.Join(directory, sessionMarker))
		os.Remove(directory)
	}
	return errors.Join(failures...)
}

func IsClipboardInput(modelsRoot, input string) bool {
	if modelsRoot == "" {
		return false
	}
	root := filepath.Join(modelsRoot, ".clipboard")
	relative, err := filepath.Rel(root, input)
	return err == nil && paths.CheckContained(root, input) == nil && knownFile("clipboard", relative)
}
