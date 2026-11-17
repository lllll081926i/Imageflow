package settings

import (
	"encoding/json"
	"os"
	"path/filepath"
	"testing"
)

func TestModelDownloadDefaultsAndLegacyMigration(t *testing.T) {
	path := filepath.Join(t.TempDir(), "settings.json")
	t.Setenv("IMAGEFLOW_SETTINGS_FILE", path)
	if err := os.WriteFile(path, []byte(`{"output_prefix":"OLD"}`), 0600); err != nil {
		t.Fatal(err)
	}
	configuration := Load()
	if configuration.AIModelSource != "github" || configuration.AIModelProxyMode != "system" || configuration.AIModelProxyURL != "http://127.0.0.1:7890" || configuration.AIModelMirror != "https://ghfast.top/" {
		t.Fatalf("旧设置未补齐下载默认值：%+v", configuration)
	}
	configuration.AIModelSource, configuration.AIModelProxyMode = "mirror", "custom"
	configuration.AIModelProxyURL = "socks5://127.0.0.1:1080"
	configuration.AIEnginePath = filepath.Join(t.TempDir(), "engine.exe")
	configuration.AIModelsDir = filepath.Join(t.TempDir(), "models")
	if err := Save(configuration); err != nil {
		t.Fatal(err)
	}
	loaded := Load()
	if loaded.AIModelSource != configuration.AIModelSource || loaded.AIModelProxyMode != configuration.AIModelProxyMode || loaded.AIModelProxyURL != configuration.AIModelProxyURL || loaded.AIModelsDir != configuration.AIModelsDir || loaded.AIEnginePath != configuration.AIEnginePath {
		t.Fatal("下载设置未完整持久化")
	}
}

func TestModelsStayBesideExecutable(t *testing.T) {
	base := t.TempDir()
	configuration := filepath.Join(base, "user-data", "settings.json")
	custom := filepath.Join(base, "chosen-models")
	for _, folder := range []string{"installed", "portable", "moved-portable"} {
		executable := filepath.Join(base, folder, "ImageFlow.exe")
		want := filepath.Join(base, folder, "models")
		for _, saved := range []string{"", "models", filepath.Join(filepath.Dir(configuration), "models")} {
			if actual := resolveModelDirectory(saved, executable, configuration); actual != want {
				t.Fatalf("模型目录未跟随程序：%s，期望 %s", actual, want)
			}
		}
		if actual := resolveModelDirectory(custom, executable, configuration); actual != custom {
			t.Fatal("不得覆盖用户自选目录")
		}
	}
	t.Setenv("IMAGEFLOW_SETTINGS_FILE", configuration)
	if err := Save(Default()); err != nil {
		t.Fatal(err)
	}
	raw, err := os.ReadFile(configuration)
	if err != nil {
		t.Fatal(err)
	}
	var stored Settings
	if err := json.Unmarshal(raw, &stored); err != nil || stored.AIModelsDir != "models" {
		t.Fatalf("默认路径必须相对程序保存，以便移动便携目录：%s %v", stored.AIModelsDir, err)
	}
	if actual := Load().AIModelsDir; actual != Default().AIModelsDir {
		t.Fatalf("相对路径读取失败：%s", actual)
	}
}
