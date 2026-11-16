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
	want := []string{
		"格式转换", "尺寸调整", "图片裁剪", "宫格切图", "图片布局",
		"图片拼图", "长图拼接", "批量水印", "色彩提取", "图标生成",
		"转 PDF", "GIF 工具", "富文本制图", "图片压缩",
	}
	if !reflect.DeepEqual(names, want) {
		t.Fatalf("tools = %v, want %v", names, want)
	}
}
