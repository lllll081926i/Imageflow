package ui

import (
	"fmt"
	"image/png"
	"os"
	"path/filepath"
	"strings"
	"testing"

	myui "github.com/egoist/mygo/ui"
	"github.com/lllll081926i/imageflow-go/internal/engine"
)

func acceptanceTypeSearch(t *testing.T, tester *myui.Tester, query string) {
	t.Helper()
	for _, field := range tester.AccessAll() {
		if strings.HasPrefix(field.Placeholder, "搜索工具") && !field.Offscreen {
			tester.ClickAt(field.Bounds.X+field.Bounds.W/2, field.Bounds.Y+field.Bounds.H/2)
			tester.Key(myui.Ctrl, myui.KeyA)
			tester.Type(query)
			tester.Frame()
			return
		}
	}
	t.Fatal("未找到可操作的工具搜索框")
}

func TestReleaseSearchAndNavigateEveryTool(t *testing.T) {
	app := newTestApp(t)
	app.ShowHome = true
	app.Settings.UIReducedMotion = true
	tester := myui.NewTester(app.View, 1440, 900)
	for _, tool := range app.Tools {
		acceptanceTypeSearch(t, tester, tool.Name())
		if err := tester.Click(tool.Name()); err != nil {
			t.Fatalf("%s 无法从搜索结果打开：%v", tool.Name(), err)
		}
		tester.Frame()
		if app.ShowHome || app.ShowSettings || app.ActiveTool().ID() != tool.ID() {
			t.Fatalf("搜索后切换到错误工具：%s", tool.Name())
		}
	}
	acceptanceTypeSearch(t, tester, "不存在的工具abc123")
	if tester.HasText("专业调色") {
		t.Fatal("无结果搜索仍保留无关工具")
	}
	if err := tester.Click("首页"); err != nil {
		t.Fatal(err)
	}
	acceptanceTypeSearch(t, tester, "打码")
	if err := tester.Click("图片标注"); err != nil {
		t.Fatal(err)
	}
	if app.ActiveTool().ID() != ToolAnnotate {
		t.Fatal("同义词搜索没有打开图片标注")
	}
	app.CancelStudioWork()
}

func TestReleaseResponsivePages(t *testing.T) {
	for _, size := range []struct{ width, height int }{{960, 620}, {1440, 900}} {
		for _, dark := range []bool{false, true} {
			t.Run(fmt.Sprintf("%dx%d-dark-%t", size.width, size.height, dark), func(t *testing.T) {
				app := newTestApp(t)
				app.Settings.UIReducedMotion = true
				defer app.CancelStudioWork()
				tester := myui.NewTester(app.View, size.width, size.height)
				tester.SetDark(dark)
				for index, tool := range app.Tools {
					app.Active = index
					tester.Frame()
					button, found := tester.AccessByRole(myui.RoleButton, "开始处理")
					if !found || button.Offscreen || button.Bounds.W < 32 || button.Bounds.H < 20 ||
						button.Bounds.X < 0 || button.Bounds.Y < 0 ||
						button.Bounds.X+button.Bounds.W > float32(size.width)+1 ||
						button.Bounds.Y+button.Bounds.H > float32(size.height)+1 {
						t.Fatalf("%s 的开始按钮不可见或越界：%+v", tool.Name(), button)
					}
					if directory := os.Getenv("IMAGEFLOW_ACCEPTANCE_SHOTS"); directory != "" {
						path := filepath.Join(directory, fmt.Sprintf("page-%02d-%dx%d-dark-%t.png", index, size.width, size.height, dark))
						file, err := os.Create(path)
						if err != nil {
							t.Fatal(err)
						}
						encodeErr := png.Encode(file, tester.Image())
						closeErr := file.Close()
						if encodeErr != nil || closeErr != nil {
							t.Fatalf("截图失败：%v %v", encodeErr, closeErr)
						}
					}
				}
			})
		}
	}
}

func acceptancePanel(app *App, tool Tool) *myui.Tester {
	return myui.NewTester(func(c *myui.Context) {
		myui.Column(c).Width(440).Gap(10).Children(func() { tool.Panel(c, app) })
	}, 460, 2600)
}

func TestReleaseEditFlipInteraction(t *testing.T) {
	app := newTestApp(t)
	tool := useTool(t, app, "基础编辑").(*EditTool)
	tester := acceptancePanel(app, tool)
	if err := tester.Click("左右镜像"); err != nil {
		t.Fatal(err)
	}
	tester.Frame()
	if !tool.flipH {
		t.Fatal("点击左右镜像后参数未保留")
	}
	if err := tester.Click("90°"); err != nil {
		t.Fatal(err)
	}
	tester.Frame()
	if tool.rotation != 90 {
		t.Fatal("旋转分段选择未保留")
	}
	if err := tester.Click("撤销"); err != nil {
		t.Fatal(err)
	}
	tester.Frame()
	if tool.rotation != 0 || !tool.flipH {
		t.Fatal("撤销没有恢复上一步参数")
	}
	if err := tester.Click("重做"); err != nil {
		t.Fatal(err)
	}
	tester.Frame()
	if tool.rotation != 90 || !tool.flipH {
		t.Fatal("重做没有恢复旋转与翻转")
	}
}

func TestReleaseCanvasLayerActions(t *testing.T) {
	app := newTestApp(t)
	tool := useTool(t, app, "自由布局").(*CollageStudioTool)
	tester := acceptancePanel(app, tool)
	for _, action := range []string{"加形状", "加文字", "下移一层"} {
		if err := tester.Click(action); err != nil {
			t.Fatal(err)
		}
		tester.Frame()
	}
	if len(tool.layers) != 2 || tool.selected != 0 || tool.layers[0].Kind != engine.LayerText {
		t.Fatalf("图层新增或换序错误：%+v", tool.layers)
	}
	if err := tester.Click("删除"); err != nil {
		t.Fatal(err)
	}
	tester.Frame()
	if len(tool.layers) != 1 || tool.layers[0].Kind != engine.LayerShape {
		t.Fatalf("图层删除错误：%+v", tool.layers)
	}
}

func TestReleaseIndependentToolStartButton(t *testing.T) {
	for _, name := range []string{"自由布局", "图片对比", "图像分析"} {
		t.Run(name, func(t *testing.T) {
			app := newTestApp(t)
			useTool(t, app, name)
			defer app.CancelStudioWork()
			tester := myui.NewTester(app.View, 1440, 900)
			button, found := tester.AccessByRole(myui.RoleButton, "开始处理")
			if !found || button.Disabled {
				t.Fatal("独立工具的开始按钮不应要求共享导入列表非空")
			}
			if err := tester.Click("开始处理"); err != nil {
				t.Fatal(err)
			}
			if app.IsRunning() {
				t.Fatal("缺少独立输入时没有释放运行状态")
			}
		})
	}
}

func TestReleasePixelColorTyping(t *testing.T) {
	app := newTestApp(t)
	tool := useTool(t, app, "像素画 / 点阵").(*PixelTool)
	tester := acceptancePanel(app, tool)
	for _, field := range tester.AccessAll() {
		if field.Placeholder == "#RRGGBB" {
			tester.ClickAt(field.Bounds.X+field.Bounds.W/2, field.Bounds.Y+field.Bounds.H/2)
			tester.Key(myui.Ctrl, myui.KeyA)
			tester.Type("#123456")
			tester.Frame()
			if tool.blackHex != "#123456" {
				t.Fatalf("黑色输入丢失：%s", tool.blackHex)
			}
			return
		}
	}
	t.Fatal("未找到黑色色值输入框")
}

func TestReleaseCanvasDerivedSliderInteraction(t *testing.T) {
	app := newTestApp(t)
	tool := useTool(t, app, "自由布局").(*CollageStudioTool)
	tool.layers = []engine.CanvasLayer{{Kind: engine.LayerText, W: 100, H: 100, Text: "测试", FontSize: 24, Opacity: 1, Visible: true}}
	tool.selected = 0
	tester := acceptancePanel(app, tool)
	for _, field := range tester.AccessAll() {
		if field.Role == myui.RoleSlider && field.Min == 8 && field.Max == 200 {
			tester.ClickAt(field.Bounds.X+field.Bounds.W*0.7, field.Bounds.Y+field.Bounds.H/2)
			tester.Frame()
			if tool.layers[0].FontSize <= 24 {
				t.Fatalf("字号滑块参数丢失：%d", tool.layers[0].FontSize)
			}
			return
		}
	}
	t.Fatal("未找到字号滑块")
}
