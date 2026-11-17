package ui

import (
	"context"
	"errors"
	"fmt"
	"sync/atomic"

	myui "github.com/egoist/mygo/ui"
	"github.com/lllll081926i/imageflow-go/internal/settings"
	"github.com/lllll081926i/imageflow-go/internal/upscale"
	"github.com/lobehub/lobe-mygo/pkg/lucide"
)

type modelDownloadProgress struct {
	received atomic.Int64
	total    atomic.Int64
}

func (a *App) modelDownloadSettings(c *myui.Context, st style, configuration *settings.Settings) {
	settingsSection(c, st, lucide.Download(), "AI 模型下载", func() {
		label(c, "默认下载源")
		myui.Row(c).FillWidth().Gap(8).Children(func() {
			a.ghostButtonState(c, st, "普通 GitHub", nil, false, configuration.AIModelSource == "github", func() { configuration.AIModelSource = "github" })
			a.ghostButtonState(c, st, "国内加速直连", nil, false, configuration.AIModelSource == "mirror", func() { configuration.AIModelSource = "mirror" })
		})
		label(c, "国内加速 HTTPS 前缀")
		textField(c, &configuration.AIModelMirror, upscale.DefaultModelMirror)
		label(c, "下载代理")
		myui.Row(c).FillWidth().Gap(6).Children(func() {
			for _, choice := range []struct{ value, label string }{{"system", "系统代理"}, {"custom", "自定义代理"}, {"direct", "不使用代理"}} {
				value := choice.value
				a.ghostButtonState(c, st, choice.label, nil, false, configuration.AIModelProxyMode == value, func() { configuration.AIModelProxyMode = value })
			}
		})
		label(c, "代理地址（默认端口 7890）")
		textField(c, &configuration.AIModelProxyURL, upscale.DefaultModelProxy)
		label(c, "模型保存目录")
		textField(c, &configuration.AIModelsDir, "选择模型根目录")
		a.ghostButton(c, st, "选择模型目录", lucide.FolderOpen(), func() { a.pickDirectoryInto(&configuration.AIModelsDir, nil) })
		label(c, "本地推理引擎")
		textField(c, &configuration.AIEnginePath, "选择 realesrgan-ncnn-vulkan")
	})
}

func (t *UpscaleTool) downloadPanel(c *myui.Context, a *App, st style, importing bool) {
	model, downloading, downloadErr := t.modelDownload.snapshot()
	if t.downloadVersion != t.downloadApplied && !downloading {
		t.downloadApplied = t.downloadVersion
		if downloadErr == nil && model.Name != "" {
			t.detected = false
			t.modelName = model.Name
			a.SetStatus("模型已下载并校验：" + model.Name)
		}
	}
	sectionCard(c, "下载模型", func() {
		if t.downloadModel == "" {
			t.downloadModel = upscale.DownloadableModels()[0].Name
		}
		labels := make([]string, 0, 3)
		for _, available := range upscale.DownloadableModels() {
			labels = append(labels, available.Name)
		}
		selectField(c, &t.downloadModel, labels)
		disabled := downloading || importing || a.IsRunning() || t.models == ""
		myui.Row(c).FillWidth().Gap(8).Children(func() {
			a.ghostButtonState(c, st, "按默认源下载", lucide.Download(), disabled, false, func() { t.startModelDownload(a, a.Settings.AIModelSource) })
			if downloading {
				a.ghostButton(c, st, "取消下载", lucide.X(), func() { t.modelDownload.cancelRequest() })
			}
		})
		myui.Row(c).FillWidth().Gap(8).Children(func() {
			a.ghostButtonState(c, st, "国内加速下载", nil, disabled, false, func() { t.startModelDownload(a, "mirror") })
			a.ghostButtonState(c, st, "GitHub 下载", nil, disabled, false, func() { t.startModelDownload(a, "github") })
		})
		if downloading {
			received, total := t.downloadProgress.received.Load(), t.downloadProgress.total.Load()
			message := fmt.Sprintf("正在下载 / 校验：%.1f MiB", float64(received)/(1<<20))
			if total > 0 {
				message += fmt.Sprintf(" / %.1f MiB", float64(total)/(1<<20))
			}
			myui.Text(c, message).FontSize(11).TextColor(st.TextSub)
		} else if errors.Is(downloadErr, context.Canceled) {
			myui.Text(c, "下载已取消").FontSize(11).TextColor(st.TextDim)
		} else if downloadErr != nil {
			warnAlert(c, "模型下载失败："+downloadErr.Error())
		} else if model.Name != "" {
			myui.Text(c, "已下载并校验："+model.Name).FontSize(11).TextColor(st.Success)
		}
	})
}

func (t *UpscaleTool) startModelDownload(a *App, source string) {
	configuration := a.Settings
	root, name := configuration.AIModelsDir, t.downloadModel
	t.downloadVersion++
	t.downloadProgress.received.Store(0)
	t.downloadProgress.total.Store(0)
	options := upscale.DownloadOptions{Source: source, Mirror: configuration.AIModelMirror,
		ProxyMode: configuration.AIModelProxyMode, ProxyURL: configuration.AIModelProxyURL}
	progress := &t.downloadProgress
	t.modelDownload.request(a, fmt.Sprint(t.downloadVersion), func(ctx context.Context) (upscale.Model, error) {
		return upscale.DownloadModel(ctx, root, name, options, func(received, total int64) {
			progress.received.Store(received)
			progress.total.Store(total)
			a.Invalidate()
		})
	})
}
