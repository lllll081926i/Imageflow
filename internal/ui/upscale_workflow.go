package ui

import (
	"context"
	"errors"

	myui "github.com/egoist/mygo/ui"
	"github.com/lllll081926i/imageflow-go/internal/upscale"
	"github.com/lobehub/lobe-mygo/pkg/lucide"
)

func (t *UpscaleTool) selectedModelStatus() upscale.ModelStatus {
	for _, status := range t.env.Models {
		if status.Name == t.downloadModel {
			return status
		}
	}
	return upscale.ModelStatus{}
}

func (t *UpscaleTool) canDownloadModel() bool {
	if t.models == "" || !upscale.ComponentDownloadSupported() {
		return false
	}
	for _, model := range upscale.UpscaylModels() {
		if model.Name == t.downloadModel {
			return true
		}
	}
	return false
}

func (t *UpscaleTool) pendingComponent(a *App) string {
	if _, downloading, err := t.modelDownload.snapshot(); downloading {
		if errors.Is(err, context.Canceled) {
			return "正在取消模型下载…"
		}
		return "正在下载模型…"
	}
	if _, importing, err := t.modelImport.snapshot(); importing {
		if errors.Is(err, context.Canceled) {
			return "正在取消模型导入…"
		}
		return "正在导入模型…"
	}
	if t.busy || !t.detected {
		return "正在检测 AI 组件…"
	}
	if a.cacheCleaning.Load() {
		return "正在清理缓存…"
	}
	return ""
}

func (t *UpscaleTool) upscaleBlock(a *App) string {
	if pending := t.pendingComponent(a); pending != "" {
		return pending
	}
	return t.componentBlock()
}

func (t *UpscaleTool) componentBlock() string {
	status := t.selectedModelStatus()
	switch {
	case !status.Installed:
		return "所选模型尚未安装"
	case !status.HashOK:
		return "所选模型校验失败"
	case !t.env.Available:
		return "本地推理引擎未就绪"
	default:
		return ""
	}
}

func (t *UpscaleTool) primaryAction(a *App) (string, *myui.SVG, bool, func()) {
	if pending := t.pendingComponent(a); pending != "" {
		return pending, lucide.Activity(), true, a.Start
	}
	if t.componentBlock() != "" {
		status := t.selectedModelStatus()
		if t.canDownloadModel() && (!status.Installed || (status.HashOK && !t.env.Available)) {
			return "下载 AI 组件", lucide.Download(), false, func() {
				t.startModelDownload(a, a.Settings.AIModelSource)
			}
		}
		return "重新检测 AI 组件", lucide.RotateCcw(), false, func() { t.detected = false }
	}
	if a.Session.Len() == 0 {
		return "选择要放大的图片", lucide.FolderOpen(), false, a.pickFiles
	}
	return "放大图片", lucide.Sparkles(), false, a.Start
}
