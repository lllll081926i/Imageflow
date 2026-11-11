package ui

import (
	"context"
	"fmt"
	"image"

	"github.com/lllll081926i/imageflow-go/internal/batch"
	"github.com/lllll081926i/imageflow-go/internal/codec"
	"github.com/lllll081926i/imageflow-go/internal/paths"
)

// CancelImport 取消导入，并让窗口队列中尚未提交的旧结果失效。
func (a *App) CancelImport() {
	a.importMu.Lock()
	if a.importCancel != nil {
		a.importCancel()
	}
	a.importContext = nil
	a.importCancel = nil
	a.importPending = 0
	a.importMu.Unlock()
}

func (a *App) IsImporting() bool {
	a.importMu.Lock()
	defer a.importMu.Unlock()
	return a.importPending > 0
}

// ImportPaths 在后台遍历和解码，通过窗口更新提交结果。
func (a *App) ImportPaths(raw []string) {
	win := a.Window
	if win == nil {
		return
	}
	a.importPaths(raw, win.Update)
}

func (a *App) importPaths(raw []string, publish func(func())) {
	a.importMu.Lock()
	if a.importContext == nil || a.importContext.Err() != nil {
		a.importContext, a.importCancel = context.WithCancel(context.Background())
	}
	ctx := a.importContext
	a.importPending++
	a.importMu.Unlock()
	inputs := append([]string(nil), raw...)
	a.SetStatus("正在导入图片…")
	go func() {
		failed := 0
		defer func() {
			publish(func() {
				a.importMu.Lock()
				if a.importContext == ctx {
					a.importPending--
				}
				pending := a.importPending
				a.importMu.Unlock()
				if ctx.Err() == nil && pending == 0 {
					a.SetStatus(fmt.Sprintf("导入完成：%d 张，跳过 %d 项", a.Session.Len(), failed))
				}
			})
		}()
		a.importWorkMu.Lock()
		defer a.importWorkMu.Unlock()
		if ctx.Err() != nil {
			return
		}
		if err := batch.Execute(func(_ *batch.Cancel) error {
			expanded := paths.ExpandContext(ctx, inputs)
			failed += len(expanded.Skipped)
			if ctx.Err() != nil {
				return nil
			}
			for _, dir := range expanded.InputDirs {
				if ctx.Err() == nil {
					a.rememberInputDir(dir)
				}
			}
			previewPrepared := a.Preview() != nil
			for _, file := range expanded.Files {
				if ctx.Err() != nil {
					break
				}
				width, height, format, err := codec.Probe(file.Path)
				if err != nil {
					failed++
					continue
				}
				thumb := ""
				var preview image.Image
				if info, err := a.Thumbs.Get(file.Path, file.ModTimeNS, file.Size); err == nil {
					thumb = string(info.Data)
				}
				if !previewPrepared {
					if opened, err := codec.OpenSRGB(file.Path); err == nil {
						preview = opened.Image
						previewPrepared = true
					}
				}
				if ctx.Err() != nil {
					break
				}
				publish(func() {
					if ctx.Err() != nil {
						return
					}
					a.Session.Add([]paths.InputFile{file})
					rows := a.Session.Snapshot()
					for i, row := range rows {
						if paths.SamePath(row.Path, file.Path) {
							a.Session.SetMeta(i, width, height, format, thumb)
							break
						}
					}
					if a.Preview() == nil && preview != nil {
						a.setInitialPreview(file.Path, preview)
					}
					a.SetStatus(fmt.Sprintf("正在导入：已添加 %d 张", a.Session.Len()))
				})
			}
			return nil
		}, nil); err != nil {
			failed++
		}
	}()
}
