package ui

import (
	"context"
	"fmt"
	"os"
	"time"

	myui "github.com/egoist/mygo/ui"
	"github.com/lllll081926i/imageflow-go/internal/appcache"
	"github.com/lobehub/lobe-mygo/pkg/lucide"
)

func (a *App) cacheSection(c *myui.Context, st style) {
	if time.Now().After(a.cacheNextScan) {
		if _, busy, _ := a.cacheJob.snapshot(); !busy {
			a.requestCache(false)
		}
	}
	report, busy, err := a.cacheJob.snapshot()
	settingsSection(c, st, lucide.Trash2(), "缓存", func() {
		metricRow(c, st, "磁盘缓存", humanSize(report.Total()))
		metricRow(c, st, "缩略图", humanSize(report.Thumbnails))
		metricRow(c, st, "临时原图", humanSize(report.Clipboard))
		metricRow(c, st, "压缩工具", humanSize(report.Tools))
		metricRow(c, st, "下载暂存", humanSize(report.Staging))
		if report.InUse > 0 {
			metricRow(c, st, "使用中", humanSize(report.InUse))
		}
		if busy {
			myui.Text(c, "正在处理缓存").FontSize(11).TextColor(st.TextDim)
		}
		if err != nil {
			warnAlert(c, "缓存处理失败："+err.Error())
		}
		myui.Row(c).Gap(8).Children(func() {
			a.ghostButtonState(c, st, "一键清理缓存", lucide.Trash2(), busy || a.cacheOperationBlocked(), false, func() { a.requestCache(true) })
			a.ghostButtonState(c, st, "刷新", lucide.RefreshCw(), busy, false, func() { a.requestCache(false) })
		})
	})
}

func (a *App) cacheOperationBlocked() bool {
	if a.IsRunning() || a.IsImporting() {
		return true
	}
	for _, tool := range a.Tools {
		if current, ok := tool.(*UpscaleTool); ok {
			if _, busy, _ := current.clipboardImport.snapshot(); busy {
				return true
			}
			if _, busy, _ := current.modelDownload.snapshot(); busy {
				return true
			}
			if _, busy, _ := current.modelImport.snapshot(); busy {
				return true
			}
		}
	}
	return false
}

func (a *App) requestCache(clean bool) {
	if _, busy, _ := a.cacheJob.snapshot(); busy || clean && a.cacheOperationBlocked() {
		return
	}
	a.cacheVersion++
	a.cacheNextScan = time.Now().Add(10 * time.Second)
	root := a.Session.SettingsSnapshot().AIModelsDir
	userCache, _ := os.UserCacheDir()
	if clean {
		a.cacheCleaning.Store(true)
	}
	a.cacheJob.request(a, fmt.Sprint(a.cacheVersion), func(ctx context.Context) (appcache.Report, error) {
		if clean {
			defer a.cacheCleaning.Store(false)
		}
		protected := a.clipboardCache.Protected()
		report, err := appcache.Inspect(ctx, root, userCache, protected, clean)
		if clean {
			a.Thumbs.ClearMemory()
			if err == nil {
				a.SetStatus("已清理缓存：" + humanSize(report.Removed))
			} else {
				a.SetStatus("缓存未完全清理：" + err.Error())
			}
		}
		return report, err
	})
}
