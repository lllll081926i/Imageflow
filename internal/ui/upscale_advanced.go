package ui

import (
	"context"
	"fmt"
	"time"

	"github.com/egoist/mygo"
	myui "github.com/egoist/mygo/ui"
	"github.com/lllll081926i/imageflow-go/internal/settings"
	"github.com/lllll081926i/imageflow-go/internal/upscale"
	"github.com/lobehub/lobe-mygo/pkg/lucide"
)

func (t *UpscaleTool) advancedPanel(c *myui.Context, a *App, st style) {
	imported, importing, importErr := t.modelImport.snapshot()
	if t.importVersion != t.importApplied && !importing {
		t.importApplied = t.importVersion
		if importErr == nil && imported.Name != "" {
			t.downloadModel, t.modelName, t.detected = imported.Name, imported.Name, false
			a.SetStatus("已导入本地模型：" + imported.Name)
		}
	}
	a.ghostButton(c, st, "高级选项", lucide.ChevronDown(), func() { t.advanced = !t.advanced })
	if !t.advanced {
		return
	}
	sectionCard(c, "推理设置", func() {
		sliderRow(c, "Tile（0 = 自动）", &t.tile, 0, 512, 32, "%.0f")
		switchField(c, &t.tta, "TTA（更慢，画质略好）")
		label(c, "GPU ID")
		textField(c, &t.gpuIDs, "自动，或 0,1")
	})
	sectionCard(c, "自定义模型", func() {
		index := t.importScale - 2
		if index < 0 || index > 2 {
			index = 2
		}
		label(c, "模型原生倍率")
		segmented(c, &index, []string{"2×", "3×", "4×"})
		t.importScale = index + 2
		_, downloading, _ := t.modelDownload.snapshot()
		a.ghostButtonState(c, st, "导入自定义模型", lucide.FolderOpen(), importing || downloading || a.IsRunning(), false, func() { t.chooseCustomModel(a) })
		if importing {
			a.ghostButton(c, st, "取消导入", lucide.X(), t.modelImport.cancelRequest)
		}
		if importErr != nil {
			warnAlert(c, importErr.Error())
		}
	})
	sectionCard(c, "偏好", func() {
		switchField(c, &t.skipPreview, "跳过结果预览")
		switchField(c, &t.notifyComplete, "完成通知")
		a.ghostButtonState(c, st, "重置 AI 参数", lucide.RotateCcw(), a.IsRunning(), false, func() {
			t.applyPreferences(settings.DefaultUpscale())
			t.persistPreferences(a)
		})
	})
	stats := a.Session.SettingsSnapshot().UpscaleStats
	_, _, lines := t.runState.snapshot()
	if len(lines) > 0 {
		sectionCard(c, "推理日志", func() {
			for _, line := range lines {
				myui.Text(c, line).FontSize(10).TextColor(st.TextDim).SingleLine().Tooltip(line)
			}
		})
	}
	if stats.Images > 0 {
		sectionCard(c, "本地统计", func() {
			metricRow(c, st, "已处理", fmt.Sprintf("%d 张", stats.Images))
			metricRow(c, st, "批量 / 双重", fmt.Sprintf("%d / %d 张", stats.BatchImages, stats.DoubleImages))
			metricRow(c, st, "平均耗时", (time.Duration(stats.TotalMillis/max(1, stats.Images)) * time.Millisecond).Round(time.Millisecond).String())
			metricRow(c, st, "上次耗时", (time.Duration(stats.LastMillis) * time.Millisecond).Round(time.Millisecond).String())
			metricRow(c, st, "上次使用", stats.LastUsed)
		})
	}
}

func (t *UpscaleTool) chooseCustomModel(a *App) {
	window, scale, root := a.Window, t.importScale, t.models
	if window == nil {
		return
	}
	go func() {
		paths, err := mygo.Dialog.Open(mygo.OpenDialogOptions{Parent: window,
			Title:   "仅导入可信来源的 NCNN 模型（同目录须有配对 .bin）",
			Filters: []mygo.FileFilter{{Name: "NCNN 参数", Extensions: []string{"param"}}}})
		if err != nil {
			a.SetStatus("选择模型失败：" + err.Error())
			return
		}
		if len(paths) == 0 {
			return
		}
		parameter := paths[0]
		a.publishStudio(func() {
			if a.cacheCleaning.Load() {
				a.SetStatus("正在清理缓存，请稍后导入模型")
				return
			}
			if a.IsRunning() {
				a.SetStatus("正在处理图片，请结束后重新导入模型")
				return
			}
			if _, downloading, _ := t.modelDownload.snapshot(); downloading {
				a.SetStatus("正在下载模型，请结束后重新导入模型")
				return
			}
			t.importVersion++
			t.modelImport.request(a, fmt.Sprint(t.importVersion), func(ctx context.Context) (upscale.Model, error) {
				return upscale.ImportLocalModel(ctx, root, parameter, scale)
			})
		})
	}()
}
