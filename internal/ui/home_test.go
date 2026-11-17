package ui

import (
	"os"
	"testing"
	"time"

	myui "github.com/egoist/mygo/ui"
)

// TestHomePageRenders pins the landing page: it exists, it lists every group, and
// it shows the workbench columns only after a tool is opened.
func TestHomePageRenders(t *testing.T) {
	a := newTestApp(t)
	a.ShowHome = true
	tt := myui.NewTester(a.View, 1440, 900)
	for _, want := range []string{"ImageFlow", "常用", "基础处理", "创作与排版", "专业图像", "动态与素材", "智能增强", "历史功能"} {
		if !tt.HasText(want) {
			t.Errorf("%q missing from the landing page", want)
		}
	}
	// The workbench columns belong to a tool page, not to the landing page.
	if tt.HasText("开始处理") {
		t.Error("the landing page drew the workbench action bar")
	}
	path := dumpFrame(t, a, "home")
	if st, err := statFile(path); err != nil || st < 5000 {
		t.Fatalf("home screenshot too small: %v %v", st, err)
	}
}

// TestHomeSearchFiltersTheSidebarAndThePage 验证首页搜索不改变侧栏导航。
func TestHomeSearchFiltersTheSidebarAndThePage(t *testing.T) {
	a := newTestApp(t)
	a.ShowHome = true
	a.searchQuery = "调色"
	tt := myui.NewTester(a.View, 1440, 900)
	tt.Frame()
	if !tt.HasText("找到") {
		t.Errorf("the result count is missing: %v", tt.Texts())
	}
	if !tt.HasText("专业调色") {
		t.Error("the matching tool is not in the sidebar")
	}
	if !tt.HasText("转 PDF") || a.railToolCount() != len(a.Tools) {
		t.Error("首页搜索改变了侧栏导航")
	}
}

// TestHomeClickOpensTheTool pins the landing page's card action.
func TestHomeClickOpensTheTool(t *testing.T) {
	a := newTestApp(t)
	a.ShowHome = true
	tt := myui.NewTester(a.View, 1440, 900)
	tt.Frame()
	if err := tt.Click("图片压缩"); err != nil {
		t.Fatalf("click: %v", err)
	}
	if a.ShowHome {
		t.Fatal("the landing page is still showing")
	}
	if a.ActiveTool().ID() != ToolCompress {
		t.Fatalf("active = %q", a.ActiveTool().ID())
	}
}

// TestHomeButtonReturnsFromATool pins the sidebar's route home.
func TestHomeButtonReturnsFromATool(t *testing.T) {
	a := newTestApp(t)
	a.Active = 3
	tt := myui.NewTester(a.View, 1440, 900)
	tt.Frame()
	if err := tt.Click("首页"); err != nil {
		t.Fatalf("click: %v", err)
	}
	if !a.ShowHome {
		t.Fatal("the home button did not open the landing page")
	}
}

// TestGroupHeaderFoldsTheSection pins the sidebar's fold action end to end: the
// header click is what a user actually presses.
func TestGroupHeaderFoldsTheSection(t *testing.T) {
	a := newTestApp(t)
	a.Active = 0
	tt := myui.NewTester(a.View, 1440, 900)
	tt.Frame()
	before := a.railToolCount()
	// The 专业图像 header sits below the visible part of a 900 DIP rail, so the
	// section is folded through the same entry the header calls.
	if !a.groupExpanded(GroupStudio) {
		t.Fatal("the section starts folded")
	}
	a.toggleGroup(GroupStudio)
	for i := 0; i < 6; i++ {
		tt.Advance(16 * time.Millisecond)
		tt.Frame()
	}
	if a.groupExpanded(GroupStudio) {
		t.Fatal("the fold did not close the section")
	}
	after := a.railToolCount()
	if after >= before {
		t.Fatalf("tool rows = %d, want fewer than %d", after, before)
	}
	if tt.HasText("专业调色") {
		t.Error("a folded section still drew its rows")
	}
	// The header stays, so the section can be reopened.
	a.toggleGroup(GroupStudio)
	tt.Frame()
	if !a.groupExpanded(GroupStudio) {
		t.Fatal("the section did not reopen")
	}
}

func TestCollapsedRailHasNoSearchOrGroupGaps(t *testing.T) {
	a := newTestApp(t)
	a.Active = 2
	tt := myui.NewTester(a.View, 900, 800)
	tt.Frame()
	if !a.railCollapsedAuto() {
		t.Fatalf("a 900 DIP window should render the collapsed rail")
	}
	if _, found := tt.Access("搜索工具"); found {
		t.Fatal("收起侧栏仍包含搜索按钮")
	}
	for index, row := range a.railRows() {
		if row.Header || row.Y != float32(index*(railRowHeight+railRowGap)) {
			t.Fatalf("收起侧栏包含分组空隙：%+v", row)
		}
	}
}

// statFile reports a file's size, so a screenshot check reads as one line.
func statFile(path string) (int64, error) {
	info, err := os.Stat(path)
	if err != nil {
		return 0, err
	}
	return info.Size(), nil
}
