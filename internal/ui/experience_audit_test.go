package ui

import (
	"fmt"
	"image"
	"image/color"
	"path/filepath"
	"strings"
	"testing"
	"time"

	myui "github.com/egoist/mygo/ui"
	"github.com/lllll081926i/imageflow-go/internal/codec"
	"github.com/lllll081926i/imageflow-go/internal/engine"
)

func TestAuditCropRatioCrossGroupSelection(t *testing.T) {
	app := newTestApp(t)
	defer app.CancelStudioWork()
	tool := useTool(t, app, "图片裁剪").(*CropTool)
	tester := myui.NewTester(app.View, 960, 620)
	for _, index := range []int{6, 1, len(engine.CropRatios), 0} {
		label := "自定义"
		if index < len(engine.CropRatios) {
			label = engine.CropRatios[index].Label()
		}
		if err := tester.Click(label); err != nil {
			t.Fatal(err)
		}
		tester.Frame()
		if tool.ratio != index {
			t.Fatalf("点击 %s 后比例为 %d，期望 %d", label, tool.ratio, index)
		}
	}
}

func TestAuditRichTextEditsSurviveFramesAndExport(t *testing.T) {
	app := newTestApp(t)
	defer app.CancelStudioWork()
	seedInputs(t, app, 1)
	tool := useTool(t, app, "富文本制图").(*RichTextTool)
	tool.ensure()
	tool.theme = "dark"
	tool.applyRichTextTheme()
	tool.heading, tool.body = "", ""
	tool.bgHex, tool.background = "#E02030", colorOf("#E02030")
	tester := myui.NewTester(app.View, 1440, 900)
	for range 4 {
		tester.Frame()
	}
	if tool.heading != "" || tool.body != "" || tool.bgHex != "#E02030" {
		t.Fatal("刷新覆盖了空内容或自定义颜色")
	}
	app.Start()
	waitForRun(t, app)
	rows := app.Session.Snapshot()
	if rows[0].Status != "ok" {
		t.Fatalf("富文本导出失败：%+v", rows[0])
	}
	result, err := codec.Open(rows[0].Output)
	if err != nil {
		t.Fatal(err)
	}
	pixel := color.NRGBAModel.Convert(result.Image.At(0, 0)).(color.NRGBA)
	if pixel.R != 224 || pixel.G != 32 || pixel.B != 48 {
		t.Fatalf("主题覆盖了导出颜色：%v", pixel)
	}
}

func TestAuditAnnotationIgnoresLetterbox(t *testing.T) {
	app := newTestApp(t)
	defer app.CancelStudioWork()
	seedInputs(t, app, 1)
	tool := useTool(t, app, "图片标注").(*AnnotateTool)
	app.SetPreview(image.NewNRGBA(image.Rect(0, 0, 40, 800)))
	tester := myui.NewTester(app.View, 960, 620)
	box, found := tester.Find("标注画布")
	if !found {
		t.Fatal("缺少标注画布")
	}
	tester.ClickAt(box.X+2, box.Y+box.H/2)
	tester.Frame()
	if len(tool.marks) != 0 {
		t.Fatal("留白区创建了标注")
	}
	tester.Press(box.X+box.W/2, box.Y+box.H/2)
	tester.Move(box.X+box.W-2, box.Y+box.H-2)
	tester.Release(box.X+box.W-2, box.Y+box.H-2)
	tester.Frame()
	if len(tool.marks) != 1 || tool.marks[0].X2 > 40 || tool.marks[0].Y2 > 800 {
		t.Fatalf("图片内绘制或边界限制错误：%+v", tool.marks)
	}
	tester.Press(box.X+box.W/2, box.Y+box.H*.8)
	tester.Move(box.X+box.W/2-3, box.Y+box.H*.2)
	tester.Release(box.X+box.W/2-3, box.Y+box.H*.2)
	tester.Frame()
	if len(tool.marks) != 2 || tool.marks[1].Y2 >= tool.marks[1].Y1 {
		t.Fatal("反向拖动被误判为单击")
	}
}

func TestAuditCollageIgnoresLetterbox(t *testing.T) {
	app := newTestApp(t)
	defer app.CancelStudioWork()
	seedInputs(t, app, 4)
	tool := useTool(t, app, "图片拼图").(*CollageTool)
	tool.ensure()
	tool.drawMode = true
	tester := myui.NewTester(app.View, 1440, 900)
	waitForCollagePreview(t, tool, tester)
	box, found := tester.Find("拼图画布")
	if !found {
		t.Fatal("缺少拼图画布")
	}
	tester.ClickAt(box.X+2, box.Y+box.H/2)
	tester.Frame()
	if len(tool.marks) != 0 || tool.draftMark != nil {
		t.Fatal("留白区创建了拼图标注")
	}
	tester.ClickAt(box.X+box.W/2, box.Y+box.H/2)
	tester.Frame()
	if len(tool.marks) != 1 {
		t.Fatal("图片内无法创建拼图标注")
	}
}

func TestAuditCropAndWatermarkPreviewMatchExport(t *testing.T) {
	for _, name := range []string{"图片裁剪", "批量水印"} {
		t.Run(name, func(t *testing.T) {
			app := newTestApp(t)
			defer app.CancelStudioWork()
			inputs := seedInputs(t, app, 1)
			source, err := codec.OpenSRGB(inputs[0])
			if err != nil {
				t.Fatal(err)
			}
			tool := useTool(t, app, name)
			switch current := tool.(type) {
			case *CropTool:
				current.ensure()
				current.ratio, current.anchor, current.scale = 3, 8, 1
				current.outWidth, current.outHeight, current.radius = 64, 40, 20
			case *WatermarkTool:
				current.ensure()
				current.text, current.fontSize, current.rotation = "验收", 16, -20
				current.margin, current.tiled = 0, true
			}
			app.SetPreview(source.Image)
			preview := waitStudioImage(t, app, tool)
			app.Start()
			waitForRun(t, app)
			assertAllOK(t, app)
			result, err := codec.Open(app.Session.Snapshot()[0].Output)
			if err != nil {
				t.Fatal(err)
			}
			if preview.Bounds() != result.Image.Bounds() {
				t.Fatalf("预览与导出尺寸不一致：%v / %v", preview.Bounds(), result.Image.Bounds())
			}
			for row := range preview.Bounds().Dy() {
				for column := range preview.Bounds().Dx() {
					before := color.NRGBAModel.Convert(preview.At(column, row))
					after := color.NRGBAModel.Convert(result.Image.At(column, row))
					if before != after {
						t.Fatalf("预览与导出像素不一致：%d,%d", column, row)
					}
				}
			}
		})
	}
}

func TestAuditRichTextRunsWithoutInput(t *testing.T) {
	app := newTestApp(t)
	defer app.CancelStudioWork()
	seedInputs(t, app, 0)
	tool := useTool(t, app, "富文本制图").(*RichTextTool)
	tool.ensure()
	preview := waitStudioImage(t, app, tool)
	if preview.Bounds().Dx() != 800 {
		t.Fatal("无输入时未生成富文本预览")
	}
	app.Start()
	waitForRun(t, app)
	outputs, err := filepath.Glob(filepath.Join(app.outputFolder(), "*.png"))
	if err != nil || len(outputs) != 1 || !strings.Contains(app.StatusLine(), "已生成") {
		t.Fatalf("无输入时导出失败：%v，%s，%v", outputs, app.StatusLine(), err)
	}
	if app.Session.Len() != 0 {
		t.Fatal("纯文本导出污染了输入队列")
	}
}

func TestAuditRichTextNeedsExplicitOutputWithoutInput(t *testing.T) {
	app := newTestApp(t)
	defer app.CancelStudioWork()
	useTool(t, app, "富文本制图")
	app.Start()
	if app.IsRunning() || app.StatusLine() != "先选择输出文件夹" {
		t.Fatal("无输出位置时没有停止导出")
	}
}

func TestAuditQualityControlsFollowEncoder(t *testing.T) {
	for _, name := range []string{"图片压缩", "尺寸调整", "宫格切图", "图片布局", "长图拼接", "批量水印"} {
		t.Run(name, func(t *testing.T) {
			app := newTestApp(t)
			defer app.CancelStudioWork()
			seedInputs(t, app, 1)
			useTool(t, app, name)
			tester := myui.NewTester(app.View, 1440, 900)
			if tester.HasText("质量") {
				t.Fatal("PNG 输出仍显示无效的质量控件")
			}
			if !qualityRelevant(app, "jpg") || qualityRelevant(app, "png") {
				t.Fatal("质量控件与编码器能力不一致")
			}
		})
	}
}

func TestAuditPDFPasswordIsMasked(t *testing.T) {
	app := newTestApp(t)
	defer app.CancelStudioWork()
	useTool(t, app, "转 PDF")
	tester := myui.NewTester(app.View, 1440, 900)
	password, found := tester.Access("打开密码")
	if !found || !password.Password {
		t.Fatal("PDF 密码仍以明文显示")
	}
}

func TestAuditAnalysisKeepsLatestSelection(t *testing.T) {
	app := newTestApp(t)
	defer app.CancelStudioWork()
	inputs := seedInputs(t, app, 2)
	tool := useTool(t, app, "图像分析").(*AnalysisTool)
	tool.analyse(app, inputs[0])
	tool.analyse(app, inputs[1])
	tester := myui.NewTester(app.View, 1440, 900)
	deadline := time.Now().Add(10 * time.Second)
	for tool.busy && time.Now().Before(deadline) {
		tester.Frame()
		time.Sleep(time.Millisecond)
	}
	if tool.busy || tool.path != inputs[1] || tool.result.Width == 0 || !strings.Contains(app.StatusLine(), filepath.Base(inputs[1])) {
		t.Fatalf("快速换图后没有分析最后选择：%s，%s", tool.path, app.StatusLine())
	}
}

func TestAuditReorderInputsByDragging(t *testing.T) {
	for mode := range 3 {
		t.Run(fmt.Sprint(mode), func(t *testing.T) {
			app := newTestApp(t)
			defer app.CancelStudioWork()
			inputs := seedInputs(t, app, 8)
			app.openTool(app.toolIndex(ToolPDF))
			app.fileLayout = mode
			tester := myui.NewTester(app.View, 1440, 900)
			from, found := tester.Find("预览 img00.png")
			if !found {
				t.Fatal("找不到拖动源")
			}
			to, found := tester.Find("预览 img02.png")
			if !found {
				t.Fatal("找不到拖动目标")
			}
			tester.Press(from.X+10, from.Y+10)
			tester.Move(from.X+22, from.Y+10)
			tester.Move(to.X+to.W*.75, to.Y+to.H*.75)
			tester.Release(to.X+to.W*.75, to.Y+to.H*.75)
			tester.Frame()
			rows := app.Session.Snapshot()
			if rows[0].Path != inputs[1] || rows[2].Path != inputs[0] {
				t.Fatalf("拖动没有更新队列顺序：%v", rows)
			}
			app.SetRunning(true)
			app.reorderInputs(rows, []int{2}, 0)
			app.SetRunning(false)
			if app.Session.Snapshot()[0].Path != inputs[1] {
				t.Fatal("处理中仍允许重排")
			}
		})
	}
}

func TestAuditPagesWithInputsAndScrolledParameters(t *testing.T) {
	app := newTestApp(t)
	defer app.CancelStudioWork()
	inputs := seedInputs(t, app, 4)
	for index, tool := range app.Tools {
		t.Run(tool.ID(), func(t *testing.T) {
			app.openTool(index)
			if compare, ok := tool.(*CompareTool); ok {
				compare.reference, compare.result = inputs[0], inputs[1]
				compare.recompute(app)
			}
			tester := myui.NewTester(app.View, 1440, 900)
			switch tool.(type) {
			case *CropTool, *WatermarkTool, *RichTextTool, *EditTool, *ScreenshotTool, *GradeTool, *FilterTool, *PixelTool, *EmoteTool, *CollageStudioTool:
				waitStudioImage(t, app, tool)
			case *CollageTool:
				waitForCollagePreview(t, tool.(*CollageTool), tester)
			case *AnalysisTool:
				analysis := tool.(*AnalysisTool)
				analysis.analyse(app, inputs[0])
				deadline := time.Now().Add(10 * time.Second)
				for analysis.busy && time.Now().Before(deadline) {
					tester.Frame()
					time.Sleep(time.Millisecond)
				}
				if analysis.busy || analysis.result.Width == 0 {
					t.Fatal("分析页未生成统计结果")
				}
			}
			tester.Frame()
			dumpImage(t, tester.Image(), "audit-"+tool.ID()+"-top")
			panel, found := tester.Find("参数卡")
			if !found {
				t.Fatal("缺少参数卡")
			}
			for range 12 {
				tester.Scroll(panel.X+panel.W/2, panel.Y+200, 0, 400)
				tester.Frame()
			}
			button, found := tester.Access(app.actionLabel())
			if !found || button.Offscreen || button.Bounds.Y+button.Bounds.H > 900 {
				t.Fatal("滚动参数后开始处理按钮不可见")
			}
			dumpImage(t, tester.Image(), "audit-"+tool.ID()+"-bottom")
		})
	}
}

func TestAuditStudioKeepsLastPreviewWhileUpdating(t *testing.T) {
	app := newTestApp(t)
	defer app.CancelStudioWork()
	tool := useTool(t, app, "基础编辑").(*EditTool)
	tool.ensure()
	tool.rotation = 90
	app.SetPreview(image.NewNRGBA(image.Rect(0, 0, 120, 90)))
	previous := waitStudioImage(t, app, tool)
	tool.rotation = 180
	app.requestStudioPreview(tool)
	if app.Preview() != previous {
		t.Fatal("调整参数时闪回了原图")
	}
	if result := waitStudioImage(t, app, tool); result.Bounds().Dx() != 120 {
		t.Fatal("保留旧预览后没有更新结果")
	}
}

func TestAuditPSNRZeroIsAValidMeasurement(t *testing.T) {
	result := engine.CompareResult{Computable: true, PSNR: 0, MaxChannelDelta: 255}
	if psnrText(result) != "0.00 dB" {
		t.Fatal("0 dB 被误显示为没有结果")
	}
}

func TestAuditCompareUpdatesAlignmentAndBlocksStaleReport(t *testing.T) {
	app := newTestApp(t)
	defer app.CancelStudioWork()
	inputs := seedInputs(t, app, 2)
	tool := useTool(t, app, "图片对比").(*CompareTool)
	tool.reference, tool.result = inputs[0], inputs[1]
	tool.recompute(app)
	app.Start()
	if !strings.Contains(app.StatusLine(), "正在计算指标") {
		t.Fatal("计算中仍允许导出旧报告")
	}
	tester := myui.NewTester(app.View, 1440, 900)
	wait := func() {
		t.Helper()
		deadline := time.Now().Add(10 * time.Second)
		for tool.busy && time.Now().Before(deadline) {
			tester.Frame()
			time.Sleep(time.Millisecond)
		}
		if tool.busy || !tool.metrics.Computable {
			t.Fatalf("对比未完成：%s", tool.err)
		}
	}
	wait()
	if err := tester.Click("等比适配"); err != nil {
		t.Fatal(err)
	}
	tester.Frame()
	wait()
	if tool.metrics.Align != engine.AlignFit || compareAlignLabel(tool.metrics.Align) != "等比适配" {
		t.Fatal("对齐选项未自动更新指标")
	}
}

func TestAuditAnchorPickerMatchesImageDirections(t *testing.T) {
	names := []string{"左上", "上中", "右上", "左中", "居中", "右中", "左下", "下中", "右下"}
	for _, name := range []string{"图片裁剪", "基础编辑", "批量水印"} {
		t.Run(name, func(t *testing.T) {
			app := newTestApp(t)
			defer app.CancelStudioWork()
			tool := useTool(t, app, name)
			tester := myui.NewTester(app.View, 1440, 1800)
			for index, direction := range names {
				if err := tester.Click(direction); err != nil {
					t.Fatal(err)
				}
				tester.Frame()
				selected := ""
				switch current := tool.(type) {
				case *CropTool:
					selected = anchorPosition(current.anchor)
				case *EditTool:
					selected = anchorPosition(current.anchor)
				case *WatermarkTool:
					selected = current.position
				}
				if selected != engine.CropAnchors[index] {
					t.Fatalf("%s 对应了错误取景位置：%s", direction, selected)
				}
			}
			left, _ := tester.Find("左上")
			right, _ := tester.Find("右下")
			if right.X <= left.X || right.Y <= left.Y {
				t.Fatal("取景按钮没有按图片方向排列为九宫格")
			}
		})
	}
}
