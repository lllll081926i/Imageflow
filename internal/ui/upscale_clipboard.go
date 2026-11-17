package ui

import (
	"bytes"
	"context"
	"fmt"
	"image/png"
	"os"

	"github.com/egoist/mygo/transfer"
	myui "github.com/egoist/mygo/ui"
	"github.com/lllll081926i/imageflow-go/internal/output"
)

func (t *UpscaleTool) pasteClipboard(a *App, services myui.Services) {
	if a.IsRunning() || a.cacheCleaning.Load() {
		return
	}
	data := services.ReadClipboardData(transfer.PNG)
	if _, ok := data.Preferred(transfer.PNG); !ok {
		a.SetStatus("剪贴板中没有图片")
		return
	}
	raw, err := data.Read(transfer.PNG)
	if err != nil || len(raw) == 0 || len(raw) > 128<<20 {
		a.SetStatus("无法读取剪贴板图片，或图片超过 128 MiB")
		return
	}
	t.pasteVersion++
	root := a.Settings.AIModelsDir
	t.clipboardImport.request(a, fmt.Sprint(t.pasteVersion), func(ctx context.Context) (string, error) {
		return a.clipboardCache.Use(root, func(directory string) (string, error) {
			return saveClipboardInput(ctx, directory, raw)
		})
	})
}

func saveClipboardInput(ctx context.Context, root string, raw []byte) (string, error) {
	configuration, err := png.DecodeConfig(bytes.NewReader(raw))
	if err != nil {
		return "", err
	}
	if err := output.CheckSize(configuration.Width, configuration.Height); err != nil {
		return "", err
	}
	if _, err := png.Decode(bytes.NewReader(raw)); err != nil {
		return "", err
	}
	if err := ctx.Err(); err != nil {
		return "", err
	}
	if err := os.MkdirAll(root, 0o755); err != nil {
		return "", fmt.Errorf("剪贴板输入目录不可写：%w", err)
	}
	file, err := os.CreateTemp(root, "image-*.png")
	if err != nil {
		return "", err
	}
	path := file.Name()
	if err := file.Close(); err != nil {
		os.Remove(path)
		return "", err
	}
	if err := output.Atomic(path, func(file *os.File) error {
		if err := ctx.Err(); err != nil {
			return err
		}
		_, err := file.Write(raw)
		return err
	}); err != nil {
		os.Remove(path)
		return "", err
	}
	return path, nil
}

func (t *UpscaleTool) applyClipboard(a *App) {
	path, busy, err := t.clipboardImport.snapshot()
	if busy || t.pasteApplied == t.pasteVersion {
		return
	}
	t.pasteApplied = t.pasteVersion
	if err != nil {
		a.SetStatus("粘贴图片失败：" + err.Error())
	} else if path != "" {
		a.AddPaths([]string{path})
	}
}
