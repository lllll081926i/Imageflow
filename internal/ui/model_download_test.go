package ui

import (
	"testing"

	myui "github.com/egoist/mygo/ui"
	"github.com/lllll081926i/imageflow-go/internal/settings"
)

func TestModelDownloadSettingsControlsPersist(t *testing.T) {
	app := newTestApp(t)
	tester := myui.NewTester(func(c *myui.Context) {
		app.modelDownloadSettings(c, newStyle(false), &app.Settings)
	}, 1000, 1200)
	for _, text := range []string{"AI 模型下载", "系统代理", "代理地址（默认端口 7890）", "模型保存目录", "本地推理引擎"} {
		if !tester.HasText(text) {
			t.Fatalf("设置项缺失：%s", text)
		}
	}
	for _, choice := range []struct{ label, mode string }{{"自定义代理", "custom"}, {"不使用代理", "direct"}, {"系统代理", "system"}} {
		if err := tester.Click(choice.label); err != nil {
			t.Fatal(err)
		}
		tester.Frame()
		if app.Settings.AIModelProxyMode != choice.mode {
			t.Fatalf("代理选择未保存：%s", choice.label)
		}
	}
	if err := tester.Click("国内加速直连"); err != nil {
		t.Fatal(err)
	}
	tester.Frame()
	if app.Settings.AIModelSource != "mirror" {
		t.Fatal("下载源选择未保存")
	}
	if err := settings.Save(app.Settings); err != nil {
		t.Fatal(err)
	}
	loaded := settings.Load()
	if loaded.AIModelSource != "mirror" || loaded.AIModelProxyMode != "system" {
		t.Fatal("界面设置未持久化")
	}
}
