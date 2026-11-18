package ui

import (
	"testing"

	myui "github.com/egoist/mygo/ui"
)

// TestCollagePageRendersEmpty covers the page with an empty list: the panel still
// draws its sections, and the input card falls back to the shared dashed drop
// target, because there is nothing to compose yet.
func TestCollagePageRendersEmpty(t *testing.T) {
	a := newTestApp(t)
	useTool(t, a, "图片拼图")
	tt := myui.NewTester(a.View, 1440, 900)
	for _, want := range []string{"布局", "画布", "样式", "导出拼图"} {
		if !tt.HasText(want) {
			t.Errorf("%q missing from the empty collage page: %v", want, tt.Texts())
		}
	}
	if _, found := tt.Access("拖放区"); !found || !tt.HasText(workflowFor(a.ActiveTool()).input) {
		t.Error("an empty list must still offer the drop target")
	}
	dumpImage(t, tt.Image(), "collage-empty")
}
