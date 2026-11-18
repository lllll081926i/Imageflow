package ui

import (
	"strings"
	"testing"

	myui "github.com/egoist/mygo/ui"
	"github.com/lllll081926i/imageflow-go/internal/settings"
	"github.com/lllll081926i/imageflow-go/internal/upscale"
)

func TestModelMenuCarriesThePurposeInTheName(t *testing.T) {
	// Every registry model is trained for one picture family, and that family is the
	// only fact that tells two 4× models apart. It must be part of the row's own text,
	// because the page no longer prints a second caption below the menu.
	for _, model := range upscale.DownloadableModels() {
		label := modelLabel(model.Name)
		if !strings.HasPrefix(label, model.Name) {
			t.Fatalf("%s: label %q does not start with the model name", model.Name, label)
		}
		if !strings.Contains(label, model.Purpose) {
			t.Fatalf("%s: label %q does not carry the purpose %q", model.Name, label, model.Purpose)
		}
	}
	// An unknown name (a hand-installed model) passes through untouched, so the menu
	// never shows an empty bracket.
	if got := modelLabel("custom-local-4x"); got != "custom-local-4x" {
		t.Fatalf("custom label = %q, want the bare name", got)
	}
}

func TestModelChoicesKeepTheIdentifiersTheEngineRuns(t *testing.T) {
	tool := &UpscaleTool{downloadModel: upscale.DownloadableModels()[0].Name}
	tool.env.Models = []upscale.ModelStatus{
		{Model: upscale.Model{Name: "custom-local-4x"}},
		{Model: upscale.Model{Name: upscale.DownloadableModels()[0].Name}},
	}
	choices := tool.modelChoices()
	seen := make(map[string]int, len(choices))
	for _, choice := range choices {
		seen[choice.Name]++
		if choice.Name == "" || choice.Label == "" {
			t.Fatalf("empty row: %+v", choice)
		}
	}
	if seen[upscale.DownloadableModels()[0].Name] != 1 {
		t.Fatalf("a model installed on disk was listed twice: %v", seen)
	}
	if seen["custom-local-4x"] != 1 {
		t.Fatalf("a hand-installed model was not offered: %v", seen)
	}
	// The decorated label is the widget's value, so the page must be able to recover the
	// identifier the engine runs from the label it wrote back.
	for _, choice := range choices {
		found := false
		for _, other := range choices {
			if other.Label == choice.Label {
				found = true
			}
		}
		if !found {
			t.Fatalf("%s: its own label is not in the menu", choice.Name)
		}
	}
}

// TestChoosingAModelKeepsTheIdentifier pins the round trip through the widget: the
// menu shows decorated labels, so the page must translate the choice back to the
// model name. If it stored the label, the downloader would ask the registry for
// "hfa2k-4x（照片 / 保留细节）" and fail.
func TestChoosingAModelKeepsTheIdentifier(t *testing.T) {
	tool := &UpscaleTool{models: t.TempDir()}
	tool.ensure()
	app := newTestApp(t)
	defer app.CancelStudioWork()
	tester := myui.NewTester(func(c *myui.Context) {
		tool.downloadPanel(c, app, newStyle(false))
	}, 720, 900)
	first := upscale.DownloadableModels()[0].Name
	if tool.downloadModel != first {
		t.Fatalf("the panel did not open on the registry's first model: %q", tool.downloadModel)
	}
	if !tester.HasText(modelLabel(first)) {
		t.Fatalf("the trigger does not show the decorated name %q", modelLabel(first))
	}
	// Open the menu and take the second model.
	target := upscale.DownloadableModels()[1]
	if err := tester.Click(modelLabel(first)); err != nil {
		t.Fatal(err)
	}
	tester.Frame()
	if err := tester.Click(modelLabel(target.Name)); err != nil {
		t.Fatal(err)
	}
	tester.Frame()
	if tool.downloadModel != target.Name {
		t.Fatalf("the menu stored %q, want the identifier %q", tool.downloadModel, target.Name)
	}
	if tool.modelName != target.Name {
		t.Fatalf("the run state kept %q", tool.modelName)
	}
	if !tester.HasText(modelLabel(target.Name)) {
		t.Fatal("the trigger did not follow the choice")
	}
}

func TestModelDownloadSettingsControlsPersist(t *testing.T) {
	app := newTestApp(t)
	tester := myui.NewTester(func(c *myui.Context) {
		app.modelDownloadSettings(c, newStyle(false), &app.Settings)
	}, 1000, 1200)
	for _, text := range []string{"AI 模型下载", "系统代理", "代理地址（默认端口 7890）"} {
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
	for _, choice := range []struct{ label, source string }{{"国内加速直连", "mirror"}, {"普通 GitHub", "github"}, {"国内加速直连", "mirror"}} {
		if err := tester.Click(choice.label); err != nil {
			t.Fatal(err)
		}
		tester.Frame()
		if app.Settings.AIModelSource != choice.source {
			t.Fatal("下载源不能反复切换")
		}
	}
	if err := settings.Save(app.Settings); err != nil {
		t.Fatal(err)
	}
	loaded := settings.Load()
	if loaded.AIModelSource != "mirror" || loaded.AIModelProxyMode != "system" {
		t.Fatal("界面设置未持久化")
	}
}
