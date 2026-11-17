package batch

import (
	"encoding/json"
	"os"
	"path/filepath"
	"testing"

	"github.com/lllll081926i/imageflow-go/internal/settings"
)

func TestSettingsWritesStayWithTheirSession(t *testing.T) {
	firstPath := filepath.Join(t.TempDir(), "first.json")
	secondPath := filepath.Join(t.TempDir(), "second.json")
	t.Setenv("IMAGEFLOW_SETTINGS_FILE", firstPath)
	first := NewSession(settings.Default())
	t.Setenv("IMAGEFLOW_SETTINGS_FILE", secondPath)
	second := NewSession(settings.Default())
	if err := second.UpdateSettings(func(value *settings.Settings) { value.BackgroundRunning = true }); err != nil {
		t.Fatal(err)
	}
	if err := first.UpdateSettings(func(value *settings.Settings) { value.BackgroundRunning = false }); err != nil {
		t.Fatal(err)
	}
	for path, expected := range map[string]bool{firstPath: false, secondPath: true} {
		raw, err := os.ReadFile(path)
		if err != nil {
			t.Fatal(err)
		}
		var saved settings.Settings
		if err := json.Unmarshal(raw, &saved); err != nil {
			t.Fatal(err)
		}
		if saved.BackgroundRunning != expected {
			t.Fatalf("会话写入了其他设置文件：%s", path)
		}
	}
}
