package ui

import (
	"fmt"
	"image"
	"image/color"
	"image/draw"
	"math"
	"testing"

	myui "github.com/egoist/mygo/ui"
	"github.com/lllll081926i/imageflow-go/internal/collage"
)

func TestSmallPreviewContainsWholeImage(t *testing.T) {
	for _, size := range [][2]int{{40, 800}, {800, 40}} {
		app := newTestApp(t)
		defer app.CancelStudioWork()
		seedInputs(t, app, 1)
		app.openTool(app.toolIndex(ToolConvert))
		picture := image.NewNRGBA(image.Rect(0, 0, size[0], size[1]))
		draw.Draw(picture, picture.Bounds(), image.NewUniform(color.NRGBA{R: 240, A: 255}), image.Point{}, draw.Src)
		draw.Draw(picture, image.Rect(0, size[1]/2, size[0], size[1]), image.NewUniform(color.NRGBA{B: 240, A: 255}), image.Point{}, draw.Src)
		app.SetPreview(picture)
		tester := myui.NewTester(app.View, 960, 620)
		viewport, ok := tester.Find("预览图像")
		if !ok {
			t.Fatal("缺少预览图像")
		}
		fitted := previewFitRect(viewport, size[0], size[1])
		for index, fraction := range []float32{.25, .75} {
			pixel := color.NRGBAModel.Convert(tester.Image().At(int(fitted.X+fitted.W/2), int(fitted.Y+fitted.H*fraction))).(color.NRGBA)
			if index == 0 && pixel.R < 200 || index == 1 && pixel.B < 200 {
				t.Fatalf("%dx%d 图片没有完整适配预览：分区 %d 像素 %v", size[0], size[1], index, pixel)
			}
		}
	}
}

func TestExtremePreviewRatiosFitSmallWindow(t *testing.T) {
	for _, toolID := range []string{ToolConvert, ToolAnnotate, ToolCollage} {
		for _, size := range [][2]int{{40, 800}, {800, 40}} {
			t.Run(fmt.Sprintf("%s/%dx%d", toolID, size[0], size[1]), func(t *testing.T) {
				app := newTestApp(t)
				defer app.CancelStudioWork()
				seedInputs(t, app, 1)
				app.openTool(app.toolIndex(toolID))
				app.SetPreview(image.NewNRGBA(image.Rect(0, 0, size[0], size[1])))
				if tool, ok := app.ActiveTool().(*CollageTool); ok {
					tool.ensure()
					tool.aspectID = collage.CustomAspectID
					tool.customRatio = fmt.Sprintf("%d:%d", size[0], size[1])
				}
				tester := myui.NewTester(app.View, 960, 620)
				tester.Frame()
				frame, ok := tester.Find("预览边界")
				if !ok {
					t.Fatal("缺少预览边界")
				}
				for _, label := range []string{"图片预览", "标注画布", "拼图画布"} {
					if preview, exists := tester.Find(label); exists {
						if preview.W <= 0 || preview.H <= 0 || preview.X < frame.X || preview.Y < frame.Y || preview.X+preview.W > frame.X+frame.W+1 || preview.Y+preview.H > frame.Y+frame.H+1 {
							t.Fatalf("%s 没有完整适配：预览 %+v 边界 %+v", label, preview, frame)
						}
						imageRect := previewFitRect(preview, size[0], size[1])
						if imageRect.W <= 0 || imageRect.H <= 0 || math.Abs(float64(imageRect.W/imageRect.H)-float64(size[0])/float64(size[1])) > .01 {
							t.Fatalf("%s 图片比例变化：%+v", label, imageRect)
						}
					}
				}
				dumpImage(t, tester.Image(), fmt.Sprintf("preview-%s-%d", toolID, size[0]))
			})
		}
	}
}

func TestAllToolPreviewsStayInsideInputCard(t *testing.T) {
	app := newTestApp(t)
	defer app.CancelStudioWork()
	seedInputs(t, app, 8)
	for _, size := range [][2]int{{1440, 900}, {960, 620}} {
		for index, tool := range app.Tools {
			t.Run(fmt.Sprintf("%s/%d", tool.ID(), size[0]), func(t *testing.T) {
				app.openTool(index)
				tester := myui.NewTester(app.View, size[0], size[1])
				for mode := range 3 {
					app.fileLayout = mode
					tester.Frame()
					tester.Frame()
					card, ok := tester.Find("输入卡")
					if !ok {
						t.Fatal("缺少输入卡")
					}
					files, ok := tester.Find("拖放区")
					if !ok {
						t.Fatal("缺少图片列表")
					}
					left, right := files.X-card.X, card.X+card.W-files.X-files.W
					if math.Abs(float64(left-right)) > 1 || left < 15 {
						t.Fatalf("排列 %d 左右边距不一致：左 %g 右 %g，卡 %+v 列表 %+v", mode, left, right, card, files)
					}
					if frame, ok := tester.Find("预览边界"); ok {
						for _, label := range []string{"图片预览", "AI 对比画布", "拼图预览区", "拼图画布", "标注预览区", "标注画布", "对比区"} {
							if preview, exists := tester.Find(label); exists {
								if preview.X < frame.X || preview.Y < frame.Y || preview.X+preview.W > frame.X+frame.W+1 || preview.Y+preview.H > frame.Y+frame.H+1 || preview.Y+preview.H > files.Y+1 {
									t.Fatalf("排列 %d %s 越界：预览 %+v，边界 %+v，列表 %+v", mode, label, preview, frame, files)
								}
							}
						}
					}
				}
			})
		}
	}
}

func TestFileBrowserLayoutsSelectAndRemove(t *testing.T) {
	for mode, name := range []string{"列表", "网格", "紧凑"} {
		t.Run(name, func(t *testing.T) {
			app := newTestApp(t)
			defer app.CancelStudioWork()
			seedInputs(t, app, 80)
			app.openTool(app.toolIndex(ToolConvert))
			tester := myui.NewTester(app.View, 1440, 900)
			if err := tester.Click(name); err != nil {
				t.Fatal(err)
			}
			tester.Frame()
			if app.fileLayout != mode {
				t.Fatal("排列未切换")
			}
			rows := app.Session.Snapshot()
			if err := tester.Click("预览 " + rows[1].Name); err != nil {
				t.Fatal(err)
			}
			tester.Frame()
			if app.selectedPreviewPath() != rows[1].Path {
				t.Fatal("图片选择未更新")
			}
			tester.Key(0, myui.KeyEnd)
			tester.Frame()
			if app.selectedPreviewPath() != rows[len(rows)-1].Path {
				t.Fatal("图片列表未响应键盘跳转末尾")
			}
			tester.Key(0, myui.KeyHome)
			tester.Frame()
			if err := tester.Click("移除 " + rows[1].Name); err != nil {
				t.Fatal(err)
			}
			tester.Frame()
			if app.Session.Len() != 79 {
				t.Fatal("图片移除未更新")
			}
			app.selectPreview(rows[70].Path)
			tester.Frame()
			tester.Frame()
			if err := tester.Click("移除 " + rows[70].Name); err != nil {
				t.Fatalf("跳转后目标图片不可操作：%v", err)
			}
			tester.Frame()
			if app.Session.Len() != 78 {
				t.Fatal("跳转后未能移除目标图片")
			}
			if mode == 1 {
				dumpImage(t, tester.Image(), "file-browser-grid")
			}
		})
	}
}

func TestPreviewSpaceMatchesToolTask(t *testing.T) {
	for _, toolID := range []string{ToolPDF, ToolGIF, ToolAnalysis, ToolConvert, ToolResize, ToolCompress, ToolIcon, ToolGrid, ToolPalette} {
		t.Run(toolID, func(t *testing.T) {
			app := newTestApp(t)
			defer app.CancelStudioWork()
			seedInputs(t, app, 3)
			app.openTool(app.toolIndex(toolID))
			tester := myui.NewTester(app.View, 1440, 900)
			preview, exists := tester.Find("图片预览")
			if toolPreviewMode(toolID) == previewNone {
				if exists {
					t.Fatal("不需要静态原图的页面仍显示预览")
				}
			} else if !exists || preview.H > 256 {
				t.Fatalf("批处理预览未缩小：%+v", preview)
			}
			files, ok := tester.Find("拖放区")
			if !ok || files.H < 400 {
				t.Fatalf("图片列表未获得主要空间：%+v", files)
			}
			if toolID == ToolAnalysis {
				dumpImage(t, tester.Image(), "analysis-file-browser")
			}
		})
	}
}

func TestPreviewKeepsSpaceAndFileRemovalHasAnExplicitAction(t *testing.T) {
	app := newTestApp(t)
	defer app.CancelStudioWork()
	seedInputs(t, app, 3)
	app.openTool(app.toolIndex(ToolEdit))
	tester := myui.NewTester(app.View, 1440, 900)
	preview, ok := tester.Access("图片预览")
	if !ok || preview.Bounds.H < 350 {
		t.Fatalf("预览未获得主要操作空间：%+v", preview)
	}
	files, ok := tester.Access("拖放区")
	if !ok || files.Bounds.H > 200 {
		t.Fatalf("文件列表挤占预览空间：%+v", files)
	}
	rows := app.Session.Snapshot()
	if err := tester.Click("移除 " + rows[0].Name); err != nil {
		t.Fatal(err)
	}
	tester.Frame()
	if app.Session.Len() != 2 {
		t.Fatal("显式移除操作没有更新列表")
	}
}
