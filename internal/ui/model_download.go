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
		sources := []string{"github", "mirror"}
		source := max(0, indexOf(sources, configuration.AIModelSource))
		segmented(c, &source, []string{"普通 GitHub", "国内加速直连"})
		configuration.AIModelSource = sources[source]
		label(c, "国内加速 HTTPS 前缀")
		textField(c, &configuration.AIModelMirror, upscale.DefaultModelMirror)
		label(c, "下载代理")
		modes := []string{"system", "custom", "direct"}
		mode := max(0, indexOf(modes, configuration.AIModelProxyMode))
		segmented(c, &mode, []string{"系统代理", "自定义代理", "不使用代理"})
		configuration.AIModelProxyMode = modes[mode]
		label(c, "代理地址（默认端口 7890）")
		textField(c, &configuration.AIModelProxyURL, upscale.DefaultModelProxy)
	})
}

func (t *UpscaleTool) downloadPanel(c *myui.Context, a *App, st style) {
	model, downloading, downloadErr := t.modelDownload.snapshot()
	if t.downloadVersion != t.downloadApplied && !downloading {
		t.downloadApplied = t.downloadVersion
		if downloadErr == nil && model.Name != "" {
			t.detected = false
			a.SetStatus("模型已下载并校验：" + model.Name)
		}
	}
	sectionCard(c, "AI 模型", func() {
		if t.downloadModel == "" {
			t.downloadModel = upscale.DownloadableModels()[0].Name
		}
		labels := make([]string, 0, 3)
		for _, available := range upscale.DownloadableModels() {
			labels = append(labels, available.Name)
		}
		for _, installed := range t.env.Models {
			if indexOf(labels, installed.Name) < 0 {
				labels = append(labels, installed.Name)
			}
		}
		if indexOf(labels, t.downloadModel) < 0 {
			labels = append(labels, t.downloadModel)
		}
		selectField(c, &t.downloadModel, labels)
		t.modelName = t.downloadModel
		state := "未下载"
		for index, status := range t.env.Models {
			if status.Name != t.modelName {
				continue
			}
			t.model = index
			if status.Installed && status.HashOK && t.env.Available {
				state = "已就绪"
			} else if status.Installed && !status.HashOK {
				warnAlert(c, "模型校验失败")
			} else if status.Installed && !t.busy {
				warnAlert(c, "引擎无法运行，请检查 Vulkan 驱动")
			}
		}
		if t.busy {
			state = "正在探测"
		}
		for _, available := range upscale.DownloadableModels() {
			if available.Name == t.downloadModel {
				myui.Text(c, "适合："+available.Purpose).FontSize(12).TextColor(st.TextSub)
			}
		}
		_, importing, _ := t.modelImport.snapshot()
		custom := true
		for _, available := range upscale.UpscaylModels() {
			if available.Name == t.modelName {
				custom = false
			}
		}
		disabled := downloading || importing || custom || a.IsRunning() || t.models == "" || !upscale.ComponentDownloadSupported()
		myui.Row(c).FillWidth().Gap(8).Children(func() {
			a.ghostButtonState(c, st, "一键下载", lucide.Download(), disabled, false, func() { t.startModelDownload(a, a.Settings.AIModelSource) })
			myui.Text(c, state).FontSize(11).TextColor(st.TextSub)
			if downloading {
				a.ghostButton(c, st, "取消下载", lucide.X(), func() { t.modelDownload.cancelRequest() })
			}
		})
		if !upscale.ComponentDownloadSupported() {
			warnAlert(c, "当前平台暂无可用的 Upscayl 组件")
		}
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
	if a.cacheCleaning.Load() {
		a.SetStatus("正在清理缓存，请稍后下载模型")
		return
	}
	configuration := a.Settings
	root, name := configuration.AIModelsDir, t.downloadModel
	t.downloadVersion++
	t.downloadProgress.received.Store(0)
	t.downloadProgress.total.Store(0)
	options := upscale.DownloadOptions{Source: source, Mirror: configuration.AIModelMirror,
		ProxyMode: configuration.AIModelProxyMode, ProxyURL: configuration.AIModelProxyURL}
	progress := &t.downloadProgress
	t.modelDownload.request(a, fmt.Sprint(t.downloadVersion), func(ctx context.Context) (upscale.Model, error) {
		return upscale.DownloadComponent(ctx, root, name, options, func(received, total int64) {
			progress.received.Store(received)
			progress.total.Store(total)
			a.Invalidate()
		})
	})
}
