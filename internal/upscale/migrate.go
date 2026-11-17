package upscale

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/lllll081926i/imageflow-go/internal/output"
)

func isLegacyModel(name string) bool {
	switch strings.ToLower(name) {
	case "realesrgan-x4plus", "realesrgan-x4plus-anime", "realesr-animevideov3":
		return true
	}
	return false
}

func MigrateLegacyModels(ctx context.Context, root string) error {
	if err := ctx.Err(); err != nil {
		return err
	}
	if strings.TrimSpace(root) == "" {
		return nil
	}
	root, err := filepath.Abs(root)
	if err != nil {
		return err
	}
	info, err := os.Lstat(root)
	if os.IsNotExist(err) {
		return nil
	}
	if err != nil {
		return err
	}
	if !info.IsDir() || info.Mode()&os.ModeSymlink != 0 {
		return fmt.Errorf("拒绝迁移非普通模型目录：%s", root)
	}
	modelInstallMu.Lock()
	defer modelInstallMu.Unlock()
	var archive string
	archivePath := func() (string, error) {
		if archive != "" {
			return archive, nil
		}
		trash := filepath.Join(root, ".trash")
		if info, err := os.Lstat(trash); err == nil {
			if !info.IsDir() || info.Mode()&os.ModeSymlink != 0 {
				return "", fmt.Errorf("旧模型回收目录不是普通目录：%s", trash)
			}
		} else if !os.IsNotExist(err) {
			return "", err
		}
		if err := os.MkdirAll(trash, 0o755); err != nil {
			return "", err
		}
		archive, err = os.MkdirTemp(trash, "legacy-"+time.Now().Format("20060102-150405")+"-*")
		return archive, err
	}
	groups := []struct {
		dir   string
		names []string
	}{
		{"realesrgan-x4plus", []string{"realesrgan-x4plus.param", "realesrgan-x4plus.bin"}},
		{"realesrgan-x4plus-anime", []string{"realesrgan-x4plus-anime.param", "realesrgan-x4plus-anime.bin"}},
		{"realesr-animevideov3", []string{"realesr-animevideov3-x2.param", "realesr-animevideov3-x2.bin"}},
		{"engine", []string{"realesrgan-ncnn-vulkan.exe", "vcomp140.dll", "vcomp140d.dll"}},
	}
	for _, group := range groups {
		directory := filepath.Join(root, group.dir)
		info, err := os.Lstat(directory)
		if os.IsNotExist(err) {
			continue
		}
		if err != nil {
			return err
		}
		if !info.IsDir() || info.Mode()&os.ModeSymlink != 0 {
			return fmt.Errorf("拒绝迁移非普通旧组件目录：%s", directory)
		}
		for _, name := range group.names {
			if err := ctx.Err(); err != nil {
				return err
			}
			source := filepath.Join(directory, name)
			info, err := os.Lstat(source)
			if os.IsNotExist(err) {
				continue
			}
			if err != nil {
				return err
			}
			if !info.Mode().IsRegular() {
				return fmt.Errorf("拒绝迁移非普通旧组件文件：%s", source)
			}
			targetRoot, err := archivePath()
			if err != nil {
				return err
			}
			target := filepath.Join(targetRoot, group.dir, name)
			if err := os.MkdirAll(filepath.Dir(target), 0o755); err != nil {
				return err
			}
			if err := os.Rename(source, target); err != nil {
				return fmt.Errorf("移走旧组件失败，保留原文件：%w", err)
			}
		}
	}
	manifest := filepath.Join(root, "imageflow-models.json")
	models, err := readManifest(manifest)
	if os.IsNotExist(err) {
		return nil
	}
	if err != nil {
		return fmt.Errorf("旧模型清单无效，未改写：%w", err)
	}
	kept := make([]Model, 0, len(models))
	for _, model := range models {
		if !isLegacyModel(model.Name) {
			kept = append(kept, model)
		}
	}
	if len(kept) == len(models) {
		return nil
	}
	if len(kept) == 0 {
		targetRoot, err := archivePath()
		if err != nil {
			return err
		}
		return os.Rename(manifest, filepath.Join(targetRoot, "imageflow-models.json"))
	}
	data, err := json.MarshalIndent(kept, "", "  ")
	if err != nil {
		return err
	}
	return output.Atomic(manifest, func(file *os.File) error {
		if err := ctx.Err(); err != nil {
			return err
		}
		_, err := file.Write(data)
		return err
	})
}
