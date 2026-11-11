package ui

import (
	"image"
	"image/color"
	"path/filepath"
	"testing"
	"time"

	myui "github.com/egoist/mygo/ui"
)

func waitForPreview(t *testing.T, a *App, path string) {
	t.Helper()
	deadline := time.Now().Add(5 * time.Second)
	for time.Now().Before(deadline) {
		a.previewMu.Lock()
		ready := a.previewPath == path && !a.previewLoading && a.lastPreview != nil
		a.previewMu.Unlock()
		if ready {
			return
		}
		time.Sleep(time.Millisecond)
	}
	t.Fatalf("预览未切换到 %s：%s", path, a.StatusLine())
}

func TestPreviewClickAndArrowKeys(t *testing.T) {
	a := newTestApp(t)
	paths := seedInputs(t, a, 3)
	tt := myui.NewTester(a.View, 1440, 900)
	if err := tt.Click("预览 " + filepath.Base(paths[1])); err != nil {
		t.Fatal(err)
	}
	waitForPreview(t, a, paths[1])
	tt.Frame()
	if got := color.NRGBAModel.Convert(a.Preview().At(0, 0)).(color.NRGBA); got != (color.NRGBA{20, 10, 90, 255}) {
		t.Fatalf("点击后仍显示旧图片：%v", got)
	}
	for _, step := range []struct {
		key   myui.Key
		index int
	}{
		{myui.KeyRight, 2}, {myui.KeyRight, 2}, {myui.KeyLeft, 1},
		{myui.KeyUp, 0}, {myui.KeyUp, 0}, {myui.KeyDown, 1},
	} {
		tt.Key(0, step.key)
		waitForPreview(t, a, paths[step.index])
		tt.Frame()
	}
	if err := tt.Click("图片预览"); err != nil {
		t.Fatal(err)
	}
	tt.Key(0, myui.KeyRight)
	waitForPreview(t, a, paths[2])
	tt.Frame()
	dumpImage(t, tt.Image(), "preview-navigation")
}

func TestPreviewKeysDoNotStealTextInput(t *testing.T) {
	a := newTestApp(t)
	paths := seedInputs(t, a, 2)
	value := "参数"
	tt := myui.NewTester(func(c *myui.Context) {
		myui.Row(c).Fill().Children(func() {
			a.inputCard(c, newStyle(false))
			myui.TextInput(c, &value).Width(200).Height(40).Label("参数文本")
		})
	}, 1000, 800)
	if err := tt.Click("参数文本"); err != nil {
		t.Fatal(err)
	}
	tt.Key(0, myui.KeyLeft)
	tt.Key(0, myui.KeyRight)
	tt.Type("内容")
	if a.selectedPreviewPath() != paths[0] {
		t.Fatal("编辑参数时方向键切换了预览")
	}
	if value == "参数" {
		t.Fatal("参数输入失去焦点")
	}
}

func TestPreviewRejectsStaleResultsAndRemoval(t *testing.T) {
	a := newTestApp(t)
	paths := seedInputs(t, a, 3)
	old := a.beginPreview(paths[0])
	current := a.beginPreview(paths[2])
	newImage := image.NewNRGBA(image.Rect(0, 0, 2, 2))
	a.finishPreview(current, newImage, nil)
	a.finishPreview(old, image.NewNRGBA(image.Rect(0, 0, 1, 1)), nil)
	if a.Preview() != newImage {
		t.Fatal("旧解码结果覆盖当前预览")
	}
	a.removeByPath(paths[2])
	waitForPreview(t, a, paths[1])
	a.removeByPath(paths[1])
	waitForPreview(t, a, paths[0])
	a.removeByPath(paths[0])
	if a.Preview() != nil || a.selectedPreviewPath() != "" {
		t.Fatal("删除全部图片后预览未清空")
	}
	a.finishPreview(current, newImage, nil)
	if a.Preview() != nil {
		t.Fatal("清空后旧预览重新出现")
	}
}

func TestPreviewRapidSelectionKeepsLatest(t *testing.T) {
	a := newTestApp(t)
	paths := seedInputs(t, a, 3)
	a.previewLoadMu.Lock()
	a.selectPreview(paths[1])
	a.selectPreview(paths[0])
	a.selectPreview(paths[2])
	a.previewLoadMu.Unlock()
	waitForPreview(t, a, paths[2])
	if got := color.NRGBAModel.Convert(a.Preview().At(0, 0)).(color.NRGBA); got != (color.NRGBA{40, 20, 120, 255}) {
		t.Fatalf("快速切换显示了旧图片：%v", got)
	}
}
