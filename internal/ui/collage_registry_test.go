package ui

import (
	"reflect"
	"testing"
)

// TestEveryToolHasItsOwnTint pins the rail's colour ladder: one entry per tool.
// Two tools sharing a tint is what makes the sidebar read as an unordered menu.
func TestEveryToolHasItsOwnTint(t *testing.T) {
	a := newTestApp(t)
	st := newStyle(false)
	seen := map[uint32]int{}
	for i, tool := range a.Tools {
		c := featureTint(i, st)
		key := uint32(c.R)<<16 | uint32(c.G)<<8 | uint32(c.B)
		if other, ok := seen[key]; ok {
			t.Errorf("%q and %q share the tint %v", a.Tools[other].Name(), tool.Name(), c)
		}
		seen[key] = i
	}
}

// TestCollagePageIsRegisteredBeforeStitch pins the rail's grouping: the collage
// sits next to the other layout tools, not at the end of the list.
func TestCollagePageIsRegisteredBeforeStitch(t *testing.T) {
	a := newTestApp(t)
	names := make([]string, 0, len(a.Tools))
	for _, tool := range a.Tools {
		names = append(names, tool.Name())
	}
	// The first fourteen rows are the v3.1.0 ladder in its original order: an
	// upgrade must not move a row the user already knows. The pages the v2.0 plan
	// adds follow, so no existing row shifts until the user reorders it.
	legacy := []string{
		"格式转换", "尺寸调整", "图片裁剪", "宫格切图", "图片布局",
		"图片拼图", "长图拼接", "批量水印", "色彩提取", "图标生成",
		"转 PDF", "GIF 工具", "富文本制图", "图片压缩",
	}
	if len(names) < len(legacy) {
		t.Fatalf("tools = %v, want at least the %d legacy rows", names, len(legacy))
	}
	if !reflect.DeepEqual(names[:len(legacy)], legacy) {
		t.Fatalf("legacy rows = %v, want %v", names[:len(legacy)], legacy)
	}
	for i, tool := range a.Tools {
		if tool.ID() == "" {
			t.Errorf("tool %d (%s) has no stable id", i, tool.Name())
		}
	}
}
