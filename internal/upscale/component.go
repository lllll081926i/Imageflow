package upscale

import (
	"archive/zip"
	"bytes"
	"context"
	"fmt"
	"io"
	"os"
	"path/filepath"
)

func ComponentDownloadSupported() bool {
	_, err := upscaylEngineResources()
	return err == nil
}

func ComponentEnginePath(root string) string {
	if root == "" {
		return ""
	}
	return filepath.Join(root, "engine", "realesrgan-ncnn-vulkan.exe")
}

func installEngineArchive(ctx context.Context, archivePath, root string) error {
	if err := ctx.Err(); err != nil {
		return err
	}
	archive, err := zip.OpenReader(archivePath)
	if err != nil {
		return err
	}
	defer archive.Close()
	stage, err := os.MkdirTemp(root, ".imageflow-engine-*")
	if err != nil {
		return err
	}
	defer os.RemoveAll(stage)
	names := []string{"realesrgan-ncnn-vulkan.exe", "vcomp140.dll", "vcomp140d.dll"}
	for _, name := range names {
		var entry *zip.File
		for _, candidate := range archive.File {
			if candidate.Name == name {
				if entry != nil {
					return fmt.Errorf("组件文件重复：%s", name)
				}
				entry = candidate
			}
		}
		if entry == nil || entry.UncompressedSize64 == 0 || entry.UncompressedSize64 > uint64(maxModelBundleBytes) || !entry.Mode().IsRegular() {
			return fmt.Errorf("组件文件缺失或无效：%s", name)
		}
		input, err := entry.Open()
		if err != nil {
			return err
		}
		output, err := os.OpenFile(filepath.Join(stage, name), os.O_CREATE|os.O_EXCL|os.O_WRONLY, 0o700)
		if err != nil {
			input.Close()
			return err
		}
		count, copyErr := io.Copy(output, io.LimitReader(contextReader{ctx, input}, maxModelBundleBytes+1))
		input.Close()
		closeErr := output.Close()
		if copyErr != nil {
			return copyErr
		}
		if closeErr != nil {
			return closeErr
		}
		if count > maxModelBundleBytes {
			return fmt.Errorf("组件文件超限：%s", name)
		}
	}
	modelInstallMu.Lock()
	defer modelInstallMu.Unlock()
	if err := ctx.Err(); err != nil {
		return err
	}
	target := filepath.Dir(ComponentEnginePath(root))
	if info, err := os.Lstat(target); err == nil {
		if !info.IsDir() || info.Mode()&os.ModeSymlink != 0 {
			return fmt.Errorf("组件目录已被占用：%s", target)
		}
		for _, name := range names {
			expected, err := os.ReadFile(filepath.Join(stage, name))
			if err != nil {
				return err
			}
			info, err := os.Lstat(filepath.Join(target, name))
			if err != nil || !info.Mode().IsRegular() || info.Size() != int64(len(expected)) {
				return fmt.Errorf("已有组件内容不匹配，未覆盖：%s", name)
			}
			installed, err := os.ReadFile(filepath.Join(target, name))
			if err != nil || !bytes.Equal(expected, installed) {
				return fmt.Errorf("已有组件内容不匹配，未覆盖：%s", name)
			}
		}
		return nil
	} else if !os.IsNotExist(err) {
		return err
	}
	return os.Rename(stage, target)
}
