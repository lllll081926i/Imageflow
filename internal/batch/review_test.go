package batch

import (
	"path/filepath"
	"sync"
	"testing"

	"github.com/lllll081926i/imageflow-go/internal/settings"
)

func TestWorkerPanicIsSingleFailure(t *testing.T) {
	var final Progress
	results := Run([]Job{func(*Cancel) error { panic("codec failed") }, func(*Cancel) error { return nil }}, 2, nil, func(p Progress) { final = p })
	if results[0] != Failed || results[1] != Done || final.Failed != 1 || final.Completed != 2 {
		t.Fatalf("异常隔离失效：%v %+v", results, final)
	}
}

func TestSettingsTransactionsPreserveConcurrentEdits(t *testing.T) {
	t.Setenv("IMAGEFLOW_SETTINGS_FILE", filepath.Join(t.TempDir(), "settings.json"))
	s := NewSession(settings.Default())
	base := s.SettingsSnapshot()
	edited := base
	edited.OutputPrefix = "USER"
	var wg sync.WaitGroup
	errors := make(chan error, 2)
	wg.Add(2)
	go func() {
		defer wg.Done()
		errors <- s.UpdateSettings(func(next *settings.Settings) {
			next.DefaultOutputDir = "new-output"
			next.RecentInputDirs = []string{"new-input"}
		})
	}()
	go func() { defer wg.Done(); errors <- s.MergeSettings(base, edited) }()
	wg.Wait()
	close(errors)
	for err := range errors {
		if err != nil {
			t.Fatal(err)
		}
	}
	next := s.SettingsSnapshot()
	disk := settings.Load()
	if next.OutputPrefix != "USER" || next.DefaultOutputDir != "new-output" || len(next.RecentInputDirs) != 1 {
		t.Fatalf("字段更新丢失：%+v", next)
	}
	if disk.OutputPrefix != next.OutputPrefix || disk.DefaultOutputDir != next.DefaultOutputDir {
		t.Fatal("内存与落盘状态不一致")
	}
	copy := s.SettingsSnapshot()
	copy.RecentInputDirs[0] = "changed"
	if s.SettingsSnapshot().RecentInputDirs[0] != "new-input" {
		t.Fatal("设置快照共享了切片")
	}
}

func TestSettingsSaveFailureKeepsMemory(t *testing.T) {
	t.Setenv("IMAGEFLOW_SETTINGS_FILE", t.TempDir())
	s := NewSession(settings.Default())
	if err := s.UpdateSettings(func(next *settings.Settings) { next.OutputPrefix = "FAILED" }); err == nil {
		t.Fatal("应当报告落盘失败")
	}
	if s.SettingsSnapshot().OutputPrefix == "FAILED" {
		t.Fatal("落盘失败后发布了内存状态")
	}
}

func TestOutputTemplateStaysInDirectory(t *testing.T) {
	s := settings.Default()
	s.DefaultOutputDir = t.TempDir()
	s.OutputTemplate = "../../{prefix}/{basename}"
	s.OutputPrefix = "../outside"
	out := BuildOutputPath(s, filepath.Join(t.TempDir(), "image.png"), "../../escape/image.png", "convert", "png", 1)
	rel, err := filepath.Rel(s.DefaultOutputDir, out)
	if out == "" || err != nil || !filepath.IsLocal(rel) || filepath.Dir(out) != s.DefaultOutputDir {
		t.Fatalf("路径越界：%q", out)
	}
}
