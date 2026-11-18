package ui

import (
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/lllll081926i/imageflow-go/internal/settings"
)

func TestBatchSameNameFollowsListOrder(t *testing.T) {
	for _, strategy := range []settings.ConflictStrategy{settings.ConflictRename, settings.ConflictSkip, settings.ConflictOverwrite} {
		t.Run(string(strategy), func(t *testing.T) {
			app := newTestApp(t)
			seedInputs(t, app, 3)
			config := app.Session.SettingsSnapshot()
			config.DefaultOutputDir = t.TempDir()
			config.OutputTemplate = "same"
			config.MaxConcurrency = 3
			config.ConflictStrategy = strategy
			config.OpenOutputOnComplete = false
			if err := app.Session.SetSettings(config); err != nil {
				t.Fatal(err)
			}
			tool := app.ActiveTool().(*ConvertTool)
			tool.ensure()
			tool.format = "png"
			app.Start()
			deadline := time.Now().Add(10 * time.Second)
			for app.IsRunning() && time.Now().Before(deadline) {
				time.Sleep(time.Millisecond)
			}
			if app.IsRunning() {
				t.Fatal("批处理未结束")
			}
			for index, row := range app.Session.Snapshot() {
				if strategy == settings.ConflictSkip && index > 0 {
					if row.Status != "skipped" {
						t.Fatalf("第 %d 行未跳过：%+v", index+1, row)
					}
					continue
				}
				want := []string{"same.png", "same_1.png", "same_2.png"}[index]
				if row.Status != "ok" || row.Output != filepath.Join(config.DefaultOutputDir, want) {
					t.Fatalf("第 %d 行输出不是 %s：%+v", index+1, want, row)
				}
				if _, err := os.Stat(row.Output); err != nil {
					t.Fatal(err)
				}
			}
		})
	}
}
