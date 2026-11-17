package ui

import (
	"context"
	"image/color"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/disintegration/imaging"
	myui "github.com/egoist/mygo/ui"
	"github.com/lllll081926i/imageflow-go/internal/codec"
	"github.com/lllll081926i/imageflow-go/internal/settings"
)

func TestSettingsCacheCountsAndCleansWithoutDeletingCurrentClipboardOrOutput(t *testing.T) {
	userCache := t.TempDir()
	t.Setenv("LOCALAPPDATA", userCache)
	t.Setenv("XDG_CACHE_HOME", userCache)
	t.Setenv("HOME", userCache)
	app := newTestApp(t)
	defer app.CancelStudioWork()
	app.Settings.AIModelsDir = filepath.Join(t.TempDir(), "models")
	if err := app.Session.UpdateSettings(func(next *settings.Settings) { next.AIModelsDir = app.Settings.AIModelsDir }); err != nil {
		t.Fatal(err)
	}
	input, err := app.clipboardCache.Use(app.Settings.AIModelsDir, func(directory string) (string, error) {
		path := filepath.Join(directory, "image-123.png")
		return path, codec.Save(imaging.New(20, 10, color.White), path, codec.SaveOptions{})
	})
	if err != nil {
		t.Fatal(err)
	}
	app.AddPaths([]string{input})
	output, skipped := app.outputPathFor(app.Session.Snapshot()[0], "upscale", "png", 1)
	if skipped || filepath.Dir(output) != filepath.Join(filepath.Dir(app.Settings.AIModelsDir), "outputs") {
		t.Fatalf("粘贴图片结果落入缓存：%s", output)
	}
	if err := os.MkdirAll(filepath.Dir(output), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := codec.Save(imaging.New(40, 20, color.White), output, codec.SaveOptions{}); err != nil {
		t.Fatal(err)
	}
	thumbDirectory := filepath.Join(userCache, "imageflow", "thumb_cache")
	if err := os.MkdirAll(thumbDirectory, 0o700); err != nil {
		t.Fatal(err)
	}
	thumb := filepath.Join(thumbDirectory, strings.Repeat("b", 64)+".png")
	if err := os.WriteFile(thumb, []byte("1234"), 0o600); err != nil {
		t.Fatal(err)
	}
	app.ShowHome, app.ShowSettings = false, true
	tester := myui.NewTester(app.View, 1440, 2200)
	wait := func() {
		deadline := time.Now().Add(5 * time.Second)
		for {
			tester.Frame()
			if _, busy, err := app.cacheJob.snapshot(); !busy {
				if err != nil {
					t.Fatal(err)
				}
				return
			}
			if time.Now().After(deadline) {
				t.Fatal("缓存统计未完成")
			}
			time.Sleep(10 * time.Millisecond)
		}
	}
	wait()
	if !tester.HasText("磁盘缓存") || !tester.HasText("临时原图") || !tester.HasText("使用中") {
		t.Fatal("设置页缺少缓存分类与容量")
	}
	report, _, _ := app.cacheJob.snapshot()
	if report.Thumbnails < 4 || report.InUse <= 0 {
		t.Fatalf("缓存容量统计错误：%+v", report)
	}
	if err := tester.Click("一键清理缓存"); err != nil {
		t.Fatal(err)
	}
	wait()
	if _, err := os.Stat(thumb); !os.IsNotExist(err) {
		t.Fatal("一键清理未删除缩略图")
	}
	if _, err := os.Stat(input); err != nil {
		t.Fatal("清理删除了当前输入")
	}
	app.CancelStudioWork()
	if _, err := os.Stat(input); !os.IsNotExist(err) {
		t.Fatal("退出未清理临时输入")
	}
	if _, err := os.Stat(output); err != nil {
		t.Fatal("退出清理删除了导出图片")
	}
}

func TestCacheClearIsBlockedDuringInference(t *testing.T) {
	app := newTestApp(t)
	defer app.CancelStudioWork()
	app.SetRunning(true)
	app.requestCache(true)
	if app.cacheVersion != 0 || app.cacheCleaning.Load() {
		t.Fatal("推理期间启动了缓存清理")
	}
	app.SetRunning(false)
	app.cacheCleaning.Store(true)
	app.Start()
	if app.IsRunning() {
		t.Fatal("缓存清理期间启动了处理")
	}
	_, err := saveClipboardInput(context.Background(), t.TempDir(), []byte("invalid"))
	if err == nil {
		t.Fatal("接受了损坏的粘贴图片")
	}
}
