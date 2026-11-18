package ui

// tools_studio.go holds the pages the v2.0 plan adds: basic editing, the free
// layout canvas, annotation, screenshot decoration, grading, filters, comparison,
// analysis, pixel art, stickers and the optional AI upscaler.
//
// Each page follows the same shape as the original fourteen: a Panel that edits the
// page's own parameters, an optional PreviewPanel for a page that composes its own
// picture, and a Run that drives the engine through the shared batch runner. None of
// them invents a second preview path or a second output path.

import (
	"context"
	"fmt"
	"image"
	"image/color"
	"math"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"

	myui "github.com/egoist/mygo/ui"
	"github.com/lobehub/lobe-mygo/pkg/lucide"

	"github.com/lllll081926i/imageflow-go/internal/batch"
	"github.com/lllll081926i/imageflow-go/internal/engine"
	"github.com/lllll081926i/imageflow-go/internal/history"
	"github.com/lllll081926i/imageflow-go/internal/output"
	"github.com/lllll081926i/imageflow-go/internal/settings"
	"github.com/lllll081926i/imageflow-go/internal/upscale"
)

// ---------------------------------------------------------------------------
// 基础编辑
// ---------------------------------------------------------------------------

// editSnapshot is one undoable step of the basic-edit page. Keeping the parameters
// in a comparable struct is what lets the page skip a duplicate push and lets the
// stack compare two steps cheaply.
type editSnapshot struct {
	Rotation  int
	FlipH     bool
	FlipV     bool
	Angle     float64
	AngleMode int
	CanvasW   float64
	CanvasH   float64
	Anchor    int
	BgHex     string
	Border    float64
	BorderHex string
	Radius    float64
	Shadow    bool
	Opacity   float64
	Blur      float64
	Sharpen   float64
	Grayscale bool
	Invert    bool
}

// EditTool is the 基础编辑 page.
type EditTool struct {
	rotation  int
	flipH     bool
	flipV     bool
	angle     float64
	angleMode int
	canvasW   float64
	canvasH   float64
	anchor    int
	bg        myui.Color
	bgHex     string
	border    float64
	borderCol myui.Color
	borderHex string
	radius    float64
	shadow    bool
	opacity   float64
	blur      float64
	sharpen   float64
	grayscale bool
	invert    bool
	format    int

	steps *history.Stack[editSnapshot]
	// initialised guards the defaults whose zero value is a real setting.
	initialised bool
}

func (t *EditTool) ID() string      { return ToolEdit }
func (t *EditTool) Name() string    { return "基础编辑" }
func (t *EditTool) Icon() *myui.SVG { return lucide.SlidersHorizontal() }

func (t *EditTool) ensure() {
	// The defaults come first: the history's baseline entry is the page's opening
	// state, so a default written after the baseline would be recorded as the user's
	// first edit and the page would open on "第 2 / 2 步" with 撤销 enabled.
	if !t.initialised {
		// Both values have a legitimate zero: 0% opacity and 0 px border are
		// choices. Writing the default whenever the field read zero made them
		// unreachable, so the default is seeded once.
		t.opacity = 100
		t.initialised = true
	}
	if t.bgHex == "" {
		t.bgHex = "#FFFFFF"
		t.bg = colorOf(t.bgHex)
	}
	if t.borderHex == "" {
		t.borderHex = "#E5E7EB"
		t.borderCol = colorOf(t.borderHex)
	}
	if t.steps == nil {
		t.steps = history.New[editSnapshot](40)
		t.steps.Reset(t.snapshot())
	}
}

// snapshot captures the current parameters.
func (t *EditTool) snapshot() editSnapshot {
	return editSnapshot{
		Rotation: t.rotation, FlipH: t.flipH, FlipV: t.flipV,
		Angle: t.angle, AngleMode: t.angleMode,
		CanvasW: t.canvasW, CanvasH: t.canvasH, Anchor: t.anchor, BgHex: t.bgHex,
		Border: t.border, BorderHex: t.borderHex, Radius: t.radius,
		Shadow: t.shadow, Opacity: t.opacity, Blur: t.blur, Sharpen: t.sharpen,
		Grayscale: t.grayscale, Invert: t.invert,
	}
}

// apply copies a snapshot back into the page.
func (t *EditTool) apply(s editSnapshot) {
	t.rotation, t.flipH, t.flipV = s.Rotation, s.FlipH, s.FlipV
	t.angle, t.angleMode = s.Angle, s.AngleMode
	t.canvasW, t.canvasH, t.anchor = s.CanvasW, s.CanvasH, s.Anchor
	t.bgHex, t.bg = s.BgHex, colorOf(s.BgHex)
	t.border, t.borderHex = s.Border, s.BorderHex
	t.borderCol = colorOf(s.BorderHex)
	t.radius, t.shadow, t.opacity = s.Radius, s.Shadow, s.Opacity
	t.blur, t.sharpen = s.Blur, s.Sharpen
	t.grayscale, t.invert = s.Grayscale, s.Invert
}

// record stores a step, but only when it differs from the current one: a frame
// that changed nothing must not grow the history.
func (t *EditTool) record() {
	t.ensure()
	t.steps.PushIfChanged(t.snapshot(), func(a, b editSnapshot) bool { return a == b })
}

func (t *EditTool) Panel(c *myui.Context, a *App) {
	t.ensure()
	st := styles(c)
	sectionCard(c, "历史", func() {
		myui.Row(c).FillWidth().Gap(8).AlignItems(myui.Center).Children(func() {
			step, total := t.steps.Position()
			label := "尚未编辑"
			if total > 0 {
				label = fmt.Sprintf("第 %d / %d 步", step, total)
			}
			myui.Text(c, label).FontSize(12).TextColor(st.TextSub)
			myui.Spacer(c).Grow(1)
			a.ghostButtonState(c, st, "撤销", lucide.Undo2(), !t.steps.CanUndo(), false, func() {
				if prev, ok := t.steps.Undo(); ok {
					t.apply(prev)
					a.Invalidate()
				}
			})
			a.ghostButtonState(c, st, "重做", lucide.Redo2(), !t.steps.CanRedo(), false, func() {
				if next, ok := t.steps.Redo(); ok {
					t.apply(next)
					a.Invalidate()
				}
			})
			a.ghostButton(c, st, "复位", lucide.RotateCcw(), func() {
				t.apply(editSnapshot{Opacity: 100, BgHex: "#FFFFFF", BorderHex: "#E5E7EB"})
				t.record()
				a.Invalidate()
			})
		})
	})

	sectionCard(c, "旋转与翻转", func() {
		labels := []string{"0°", "90°", "180°", "270°"}
		idx := t.rotation / 90
		if idx < 0 || idx >= len(labels) {
			idx = 0
		}
		selected := idx
		segmented(c, &selected, labels)
		t.rotation = selected * 90
		grid2(c, func() {
			flipH, flipV := t.flipH, t.flipV
			switchField(c, &flipH, "左右镜像")
			switchField(c, &flipV, "上下翻转")
			t.flipH, t.flipV = flipH, flipV
		})
		sliderRow(c, "任意角度", &t.angle, -180, 180, 1, "%.0f°")
		modeLabels := []string{"扩展画布", "裁去空白"}
		segmented(c, &t.angleMode, modeLabels)
		t.record()
	})

	sectionCard(c, "画布", func() {
		grid2(c, func() {
			myui.Column(c).Gap(5).Children(func() {
				label(c, "画布宽度（0 = 原尺寸）")
				numberField(c, &t.canvasW, 0, 16000, 8, "0")
			})
			myui.Column(c).Gap(5).Children(func() {
				label(c, "画布高度（0 = 原尺寸）")
				numberField(c, &t.canvasH, 0, 16000, 8, "0")
			})
		})
		myui.Column(c).Gap(5).Children(func() {
			label(c, "图片位置")
			anchorPicker(c, anchorPosition(t.anchor), func(position string) {
				t.anchor = indexOf(engine.CropAnchors, position)
			})
		})
		myui.Column(c).Gap(5).Children(func() {
			label(c, "空白填充色")
			before := t.bg
			colorField(c, &t.bg)
			if t.bg != before {
				t.bgHex = hexOf(t.bg)
			}
		})
		t.record()
	})

	sectionCard(c, "描边与圆角", func() {
		sliderRow(c, "边框宽度", &t.border, 0, 200, 1, "%.0f px")
		if t.border > 0 {
			myui.Column(c).Gap(5).Children(func() {
				label(c, "边框颜色")
				before := t.borderCol
				colorField(c, &t.borderCol)
				if t.borderCol != before {
					t.borderHex = hexOf(t.borderCol)
				}
			})
		}
		sliderRow(c, "圆角（占短边 %）", &t.radius, 0, 50, 1, "%.0f%%")
		switchField(c, &t.shadow, "投影")
		sliderRow(c, "不透明度", &t.opacity, 0, 100, 1, "%.0f%%")
		t.record()
	})

	sectionCard(c, "基础调整", func() {
		sliderRow(c, "模糊半径", &t.blur, 0, 32, 1, "%.0f px")
		sliderRow(c, "锐化强度", &t.sharpen, 0, 100, 1, "%.0f")
		switchField(c, &t.grayscale, "灰度")
		switchField(c, &t.invert, "反相")
		t.record()
	})

	sectionCard(c, "输出", func() {
		opts := []string{"png", "jpg", "webp"}
		segmented(c, &t.format, opts)
		if t.format != 0 {
			warnAlert(c, "非 PNG 输出将填充透明区域")
		}
	})
}

func (t *EditTool) PreviewCaption(a *App) string {
	parts := make([]string, 0, 4)
	if t.rotation != 0 {
		parts = append(parts, fmt.Sprintf("旋转 %d°", t.rotation))
	}
	if t.canvasW > 0 || t.canvasH > 0 {
		parts = append(parts, "改画布")
	}
	if t.radius > 0 {
		parts = append(parts, "圆角")
	}
	if len(parts) == 0 {
		return "原图"
	}
	return strings.Join(parts, " · ")
}

func (t *EditTool) request() engine.EditRequest {
	t.ensure()
	format := []string{"png", "jpg", "webp"}[t.format%3]
	angleModes := []string{"expand", "crop"}
	return engine.EditRequest{
		Rotation: t.rotation, FlipH: t.flipH, FlipV: t.flipV,
		Angle: t.angle, AngleMode: angleModes[t.angleMode%2],
		CanvasW: int(t.canvasW), CanvasH: int(t.canvasH),
		Anchor:     engine.CropAnchors[t.anchor%len(engine.CropAnchors)],
		Background: t.bgHex,
		Border:     int(t.border), BorderColor: t.borderHex, Radius: int(t.radius),
		Shadow: t.shadow, Opacity: int(t.opacity),
		Blur: int(t.blur), Sharpen: t.sharpen,
		Grayscale: t.grayscale, Invert: t.invert,
		OutputFormat: format,
	}
}

func (t *EditTool) Run(a *App) {
	req := t.request()
	format := req.OutputFormat
	a.runOutputBatch("基础编辑", func(index int, item batch.Item) (string, engine.Result) {
		out, skipped := a.outputPathFor(item, "edit", extOf(format), index+1)
		if skipped {
			return "", engine.Skip("目标已存在")
		}
		return out, engine.OK()
	}, func(_ context.Context, index int, item batch.Item, out string) engine.Result {
		one := req
		one.InputPath, one.OutputPath = item.Path, out
		return engine.BasicEdit(one)
	})
}

// ---------------------------------------------------------------------------
// 图片布局（自由画布）
// ---------------------------------------------------------------------------

// CollageStudioTool is the free-layer canvas page. It is a separate page from the
// batch collage: that one composes a template over many inputs, this one positions
// a few layers on one canvas.
type CollageStudioTool struct {
	width      float64
	height     float64
	preset     string
	bg         myui.Color
	bgHex      string
	layers     []engine.CanvasLayer
	selected   int
	step       float64
	previewKey string
	preview    image.Image
	previewErr string
	busy       bool
}

func (t *CollageStudioTool) ID() string      { return ToolStudio }
func (t *CollageStudioTool) Name() string    { return "自由布局" }
func (t *CollageStudioTool) Icon() *myui.SVG { return lucide.Layers() }

func (t *CollageStudioTool) ensure() {
	if t.width == 0 {
		t.width = 1200
	}
	if t.height == 0 {
		t.height = 1200
	}
	if t.bgHex == "" {
		t.bgHex = "#FFFFFF"
		t.bg = colorOf(t.bgHex)
	}
	if t.step == 0 {
		t.step = 10
	}
	if t.selected >= len(t.layers) {
		t.selected = len(t.layers) - 1
	}
}

func (t *CollageStudioTool) Panel(c *myui.Context, a *App) {
	t.ensure()
	st := styles(c)
	sectionCard(c, "画布", func() {
		grid2(c, func() {
			myui.Column(c).Gap(5).Children(func() {
				label(c, "宽度")
				numberField(c, &t.width, 64, 6000, 10, "1200")
			})
			myui.Column(c).Gap(5).Children(func() {
				label(c, "高度")
				numberField(c, &t.height, 64, 6000, 10, "1200")
			})
		})
		myui.Column(c).Gap(5).Children(func() {
			label(c, "背景色")
			before := t.bg
			colorField(c, &t.bg)
			if t.bg != before {
				t.bgHex = hexOf(t.bg)
				t.previewKey = ""
			}
		})
		labels := make([]string, 0, len(engine.CanvasPresets)+1)
		keys := make([]string, 0, len(engine.CanvasPresets)+1)
		labels = append(labels, "自定义")
		keys = append(keys, "")
		for _, p := range engine.CanvasPresets {
			labels = append(labels, p.Label)
			keys = append(keys, p.Key)
		}
		idx := indexOf(keys, t.preset)
		segmented(c, &idx, labels)
		if idx > 0 && idx < len(keys) && keys[idx] != t.preset {
			t.preset = keys[idx]
			t.applyPreset(keys[idx])
			t.previewKey = ""
		} else if idx == 0 {
			t.preset = ""
		}
	})

	sectionCard(c, "图层", func() {
		if len(t.layers) == 0 {
			warnAlert(c, "暂无图层")
		}
		for i := range t.layers {
			index := i
			layer := &t.layers[i]
			row := myui.ButtonBase(c).FillWidth().Height(34).PaddingX(8).
				Radius(st.RadiusControl).AlignItems(myui.Center).Gap(8).
				Cursor(myui.CursorPointer).Label(layerLabel(*layer))
			if t.selected == index {
				row.Background(st.BrandSoft).Border(1, st.Brand)
			} else if row.Hovered() {
				row.Background(st.Recessed)
			}
			row.Children(func() {
				myui.Icon(c, layerIcon(*layer)).Width(14).Height(14).
					TextColor(featureTint(a.Active, st))
				myui.Text(c, truncate(layerLabel(*layer), 14)).FontSize(12).
					TextColor(st.Text).SingleLine()
				myui.Spacer(c).Grow(1)
				if layer.Locked {
					myui.Icon(c, lucide.Lock()).Width(12).Height(12).TextColor(st.TextDim)
				}
				if !layer.Visible {
					myui.Icon(c, lucide.EyeOff()).Width(12).Height(12).TextColor(st.TextDim)
				}
			})
			if row.Clicked() {
				t.selected = index
			}
		}
		myui.Row(c).FillWidth().Gap(6).Children(func() {
			a.ghostButton(c, st, "加图片", lucide.ImagePlus(), func() {
				t.addLayer(engine.LayerImage)
				t.layers[t.selected].Path = a.selectedPreviewPath()
			})
			a.ghostButton(c, st, "加文字", lucide.Type(), func() {
				t.addLayer(engine.LayerText)
			})
			a.ghostButton(c, st, "加形状", lucide.Square(), func() {
				t.addLayer(engine.LayerShape)
			})
		})
	})

	if t.selected >= 0 && t.selected < len(t.layers) {
		layer := &t.layers[t.selected]
		sectionCard(c, "选中图层", func() {
			grid2(c, func() {
				myui.Column(c).Gap(5).Children(func() {
					label(c, "X")
					numberField(c, &layer.X, -6000, 6000, 1, "0")
				})
				myui.Column(c).Gap(5).Children(func() {
					label(c, "Y")
					numberField(c, &layer.Y, -6000, 6000, 1, "0")
				})
			})
			grid2(c, func() {
				myui.Column(c).Gap(5).Children(func() {
					label(c, "宽")
					numberField(c, &layer.W, 1, 6000, 1, "400")
				})
				myui.Column(c).Gap(5).Children(func() {
					label(c, "高")
					numberField(c, &layer.H, 1, 6000, 1, "400")
				})
			})
			sliderRow(c, "旋转", &layer.Rotation, -180, 180, 1, "%.0f°")
			opacity := layer.Opacity * 100
			if opacity <= 0 {
				opacity = 100
			}
			sliderRow(c, "不透明度", &opacity, 1, 100, 1, "%.0f%%")
			layer.Opacity = opacity / 100
			switchField(c, &layer.Locked, "锁定图层")
			switchField(c, &layer.Visible, "显示图层")
			switch layer.Kind {
			case engine.LayerImage:
				myui.Column(c).Gap(5).Children(func() {
					label(c, "图片路径")
					myui.Row(c).FillWidth().Gap(8).AlignItems(myui.Center).Children(func() {
						myui.Box(c).Grow(1).MinWidth(0).Children(func() {
							textField(c, &layer.Path, "选择一张图片")
						})
						a.ghostButton(c, st, "选择", lucide.FolderOpen(), func() {
							a.pickImageInto(&layer.Path, func() { t.previewKey = "" })
						})
					})
				})
				fits := []string{"cover", "contain", "stretch"}
				fitIdx := indexOf(fits, layer.Fit)
				segmented(c, &fitIdx, []string{"裁切填满", "等比留白", "拉伸"})
				layer.Fit = fits[fitIdx]
				radius := float64(layer.Radius)
				sliderRow(c, "圆角", &radius, 0, 200, 1, "%.0f px")
				layer.Radius = int(radius)
			case engine.LayerText:
				myui.Column(c).Gap(5).Children(func() {
					label(c, "文字")
					multiLine(c, &layer.Text, 2, 5, "输入文字")
				})
				myui.Column(c).Gap(5).Children(func() {
					label(c, "字体")
					selectField(c, &layer.Font, fontOptions())
				})
				size := float64(layer.FontSize)
				sliderRow(c, "字号", &size, 8, 200, 1, "%.0f")
				layer.FontSize = int(size)
				switchField(c, &layer.Bold, "加粗")
				myui.Column(c).Gap(5).Children(func() {
					label(c, "颜色")
					col := colorOf(layer.Color)
					before := col
					colorField(c, &col)
					if col != before {
						layer.Color = hexOf(col)
					}
				})
				aligns := []string{"left", "center", "right"}
				alignIdx := indexOf(aligns, layer.Align)
				segmented(c, &alignIdx, []string{"左对齐", "居中", "右对齐"})
				layer.Align = aligns[alignIdx]
			case engine.LayerShape:
				shapes := []string{engine.ShapeRect, engine.ShapeEllipse, engine.ShapeLine}
				shapeIdx := indexOf(shapes, layer.Shape)
				segmented(c, &shapeIdx, []string{"矩形", "椭圆", "直线"})
				layer.Shape = shapes[shapeIdx]
				myui.Column(c).Gap(5).Children(func() {
					label(c, "填充")
					col := colorOf(layer.Fill)
					before := col
					colorField(c, &col)
					if col != before {
						layer.Fill = hexOf(col)
					}
				})
				stroke := float64(layer.StrokeWidth)
				sliderRow(c, "描边宽度", &stroke, 0, 40, 1, "%.0f px")
				layer.StrokeWidth = int(stroke)
				if layer.StrokeWidth > 0 {
					myui.Column(c).Gap(5).Children(func() {
						label(c, "描边颜色")
						col := colorOf(layer.Stroke)
						before := col
						colorField(c, &col)
						if col != before {
							layer.Stroke = hexOf(col)
						}
					})
				}
			}
			myui.Row(c).FillWidth().Gap(6).Children(func() {
				a.ghostButton(c, st, "上移一层", lucide.ArrowUp(), func() {
					t.moveLayer(t.selected, 1)
				})
				a.ghostButton(c, st, "下移一层", lucide.ArrowDown(), func() {
					t.moveLayer(t.selected, -1)
				})
				a.ghostButton(c, st, "删除", lucide.Trash2(), func() {
					t.removeLayer(t.selected)
				})
			})
		})
	}
	t.previewKey = ""
}

func (t *CollageStudioTool) PreviewCaption(a *App) string {
	return fmt.Sprintf("%d 个图层 · %d×%d", len(t.layers), int(t.width), int(t.height))
}

// request renders the current composition into t.preview.
func (t *CollageStudioTool) request(a *App) {
	a.requestStudioPreview(t)
}

func (t *CollageStudioTool) applyPreset(key string) {
	for _, p := range engine.CanvasPresets {
		if p.Key == key {
			t.layers = p.Build(int(t.width), int(t.height))
			t.selected = -1
			return
		}
	}
}

func (t *CollageStudioTool) addLayer(kind string) {
	t.ensure()
	layer := engine.CanvasLayer{
		Kind: kind, Visible: true, Opacity: 1,
		X: t.width / 4, Y: t.height / 4, W: t.width / 2, H: t.height / 2,
	}
	switch kind {
	case engine.LayerText:
		layer.Text = "输入文字"
		layer.FontSize = int(t.height / 16)
		layer.Font = defaultFontFamily()
		layer.Color = "#111827"
		layer.Align = "center"
		layer.H = layer.H / 3
	case engine.LayerShape:
		layer.Shape = engine.ShapeRect
		layer.Fill = "#007AFF"
	}
	t.layers = append(t.layers, layer)
	t.selected = len(t.layers) - 1
}

// moveLayer changes one layer's position in the stack, towards the front when
// direction is positive.
func (t *CollageStudioTool) moveLayer(index, direction int) {
	to := index + direction
	if index < 0 || index >= len(t.layers) || to < 0 || to >= len(t.layers) {
		return
	}
	t.layers[index], t.layers[to] = t.layers[to], t.layers[index]
	t.selected = to
}

func (t *CollageStudioTool) removeLayer(index int) {
	if index < 0 || index >= len(t.layers) {
		return
	}
	t.layers = append(t.layers[:index], t.layers[index+1:]...)
	t.selected = len(t.layers) - 1
}

// layerRadius is unused while the slider binds to a local copy; keep the helper out
// of the build until a caller needs it.

func layerLabel(layer engine.CanvasLayer) string {
	if strings.TrimSpace(layer.Name) != "" {
		return layer.Name
	}
	switch layer.Kind {
	case engine.LayerText:
		return "文字：" + truncate(layer.Text, 12)
	case engine.LayerShape:
		switch layer.Shape {
		case engine.ShapeRect:
			return "形状：矩形"
		case engine.ShapeEllipse:
			return "形状：椭圆"
		case engine.ShapeLine:
			return "形状：直线"
		}
		return "形状"
	}
	if layer.Path != "" {
		return filepath.Base(layer.Path)
	}
	return "图层"
}

func layerIcon(layer engine.CanvasLayer) *myui.SVG {
	switch layer.Kind {
	case engine.LayerText:
		return lucide.Type()
	case engine.LayerShape:
		return lucide.Square()
	}
	return lucide.Image()
}

func (t *CollageStudioTool) Run(a *App) {
	t.ensure()
	if len(t.layers) == 0 {
		a.finishRun()
		a.SetStatus("还没有图层")
		return
	}
	layers := append([]engine.CanvasLayer(nil), t.layers...)
	width, height, bg := int(t.width), int(t.height), t.bgHex
	if a.Session.Len() > 0 {
		a.runOutputBatch("布局", func(index int, row batch.Item) (string, engine.Result) {
			out, skipped := a.outputPathFor(row, "layout", "png", index+1)
			if skipped {
				return "", engine.Skip("目标已存在")
			}
			return out, engine.OK()
		}, func(ctx context.Context, _ int, _ batch.Item, out string) engine.Result {
			return engine.LayoutContext(ctx, engine.CanvasRequest{
				Width: width, Height: height, Background: bg, Layers: layers, OutputPath: out,
			})
		})
		return
	}
	if a.outputFolder() == "" {
		a.finishRun()
		a.SetStatus("先选择输出文件夹")
		return
	}
	cancel := a.beginWholeList()
	anchor := filepath.Join(a.outputFolder(), "layout.png")
	out, skipped := a.outputPathFor(batch.Item{Path: anchor}, "layout", "png", 1)
	if skipped {
		a.endWholeList(cancel, nil)
		a.finishRun()
		a.SetStatus("布局已跳过：目标文件已存在")
		return
	}
	go func() {
		defer a.finishRun()
		if cancel.Cancelled() {
			a.endWholeList(cancel, nil)
			return
		}
		res := engine.LayoutContext(cancel, engine.CanvasRequest{
			Width: width, Height: height, Background: bg, Layers: layers, OutputPath: out,
		})
		if a.endWholeList(cancel, nil, res) {
			return
		}
		a.reportWhole(res, ToolStudio)
		if res.Success && a.Session.SettingsSnapshot().OpenOutputOnComplete {
			a.openCompletedPath(res.OutputPath)
		}
	}()
}

// reportWhole records a single-file result that is not attached to one input row.
func (a *App) reportWhole(res engine.Result, toolID string) {
	if !res.Success {
		a.SetStatus("处理失败：" + res.Error)
		a.SetProgress(0, "")
		a.Invalidate()
		return
	}
	if res.Skipped {
		a.SetStatus("已跳过：" + res.Message)
		a.SetProgress(0, "")
		a.Invalidate()
		return
	}
	a.rememberOutput(toolID, res.OutputPath)
	a.SetStatus("已生成：" + filepath.Base(res.OutputPath))
	a.SetProgress(100, "")
	a.Invalidate()
}

// ---------------------------------------------------------------------------
// 图片标注
// ---------------------------------------------------------------------------

// AnnotateTool is the annotation page. It draws its own preview because the marks
// are placed on the picture, not on a form.
type AnnotateTool struct {
	tool         int
	color        myui.Color
	colorHex     string
	width        float64
	text         string
	size         float64
	strength     float64
	fill         bool
	marks        []engine.AnnotateMark
	selected     int
	dragging     bool
	steps        *history.Stack[[]engine.AnnotateMark]
	canvasHandle myui.Handle
}

func (t *AnnotateTool) ID() string      { return ToolAnnotate }
func (t *AnnotateTool) Name() string    { return "图片标注" }
func (t *AnnotateTool) Icon() *myui.SVG { return lucide.PenTool() }

// `annotateTools` is the page's toolbar, in order.
//
// The icon is a function, not a value: a MyGo element or SVG is built per frame, so
// a table that cached one would hand a stale element to the next frame.
var annotateTools = []struct {
	Key   string
	Label string
	Icon  func() *myui.SVG
}{
	{engine.AnnotateRect, "矩形", lucide.Square},
	{engine.AnnotateEllipse, "圆形", lucide.Circle},
	{engine.AnnotateArrow, "箭头", lucide.MoveUpRight},
	{engine.AnnotateLine, "直线", lucide.Minus},
	{engine.AnnotateFree, "画笔", lucide.Pencil},
	{engine.AnnotateText, "文字", lucide.Type},
	{engine.AnnotateNumber, "编号", lucide.CircleDot},
	{engine.AnnotateHighlight, "高亮", lucide.Highlighter},
	{engine.AnnotateMosaic, "马赛克", lucide.Grid3X3},
	{engine.AnnotateBlur, "模糊", lucide.Droplets},
	{engine.AnnotateCover, "安全遮盖", lucide.EyeOff},
}

func (t *AnnotateTool) ensure() {
	if t.colorHex == "" {
		t.colorHex = "#EF4444"
		t.color = colorOf(t.colorHex)
	}
	if t.width == 0 {
		t.width = 4
	}
	if t.size == 0 {
		t.size = 48
	}
	if t.strength == 0 {
		t.strength = 12
	}
	if t.steps == nil {
		t.steps = history.New[[]engine.AnnotateMark](50)
		t.steps.Reset(nil)
	}
}

// marksEqual compares two mark lists by value. A history step stores the whole list,
// which is small: an annotation is four numbers and a label, not a picture.
func marksEqual(a, b []engine.AnnotateMark) bool {
	if len(a) != len(b) {
		return false
	}
	for i := range a {
		if a[i].Kind != b[i].Kind || a[i].X1 != b[i].X1 || a[i].Y1 != b[i].Y1 ||
			a[i].X2 != b[i].X2 || a[i].Y2 != b[i].Y2 || a[i].Text != b[i].Text ||
			a[i].Color != b[i].Color || a[i].Width != b[i].Width ||
			a[i].Size != b[i].Size || a[i].Strength != b[i].Strength || a[i].Fill != b[i].Fill {
			return false
		}
		if len(a[i].Points) != len(b[i].Points) {
			return false
		}
		for j := range a[i].Points {
			if a[i].Points[j] != b[i].Points[j] {
				return false
			}
		}
	}
	return true
}

// record stores the current mark list as one history step.
func (t *AnnotateTool) record() {
	snapshot := make([]engine.AnnotateMark, len(t.marks))
	copy(snapshot, t.marks)
	t.steps.PushIfChanged(snapshot, marksEqual)
}

func (t *AnnotateTool) Panel(c *myui.Context, a *App) {
	st := styles(c)
	sectionCard(c, "工具", func() {
		rows := (len(annotateTools) + 3) / 4
		for r := 0; r < rows; r++ {
			myui.Row(c).FillWidth().Gap(5).Children(func() {
				for i := r * 4; i < r*4+4 && i < len(annotateTools); i++ {
					index := i
					tool := annotateTools[i]
					active := t.tool == index
					btn := myui.ButtonBase(c).Grow(1).Height(46).Radius(st.RadiusControl).
						Justify(myui.Center).AlignItems(myui.Center).Gap(2).
						Cursor(myui.CursorPointer).Label(tool.Label)
					if active {
						btn.Background(st.BrandSoft).Border(1, st.Brand)
					} else {
						btn.Border(1, st.Divider).Background(st.Recessed)
					}
					btn.Children(func() {
						col := st.TextSub
						if active {
							col = st.Brand
						}
						myui.Icon(c, tool.Icon()).Width(15).Height(15).TextColor(col)
						myui.Text(c, tool.Label).FontSize(10).TextColor(col)
					})
					if btn.Clicked() {
						t.tool = index
					}
				}
			})
		}
		// The two distinct kinds of hiding must never read as the same control.
		warnAlert(c, "隐私遮挡请使用「安全遮盖」")
	})

	sectionCard(c, "样式", func() {
		myui.Column(c).Gap(5).Children(func() {
			label(c, "颜色")
			before := t.color
			colorField(c, &t.color)
			if t.color != before {
				t.colorHex = hexOf(t.color)
			}
		})
		kind := annotateTools[t.tool%len(annotateTools)].Key
		if kind != engine.AnnotateMosaic && kind != engine.AnnotateBlur {
			sliderRow(c, "线宽", &t.width, 1, 40, 1, "%.0f px")
		}
		if kind == engine.AnnotateText || kind == engine.AnnotateNumber {
			myui.Column(c).Gap(5).Children(func() {
				label(c, "文字")
				textField(c, &t.text, "标注内容")
			})
			sliderRow(c, "字号", &t.size, 10, 200, 1, "%.0f")
		}
		if kind == engine.AnnotateMosaic || kind == engine.AnnotateBlur {
			sliderRow(c, "强度", &t.strength, 2, 64, 1, "%.0f")
		}
		if kind == engine.AnnotateRect || kind == engine.AnnotateEllipse {
			switchField(c, &t.fill, "填充")
		}
	})

	sectionCard(c, "标注列表", func() {
		if len(t.marks) == 0 {
			warnAlert(c, "暂无标注")
		}
		for i, m := range t.marks {
			index := i
			row := myui.ButtonBase(c).FillWidth().Height(30).PaddingX(8).
				Radius(st.RadiusPill).AlignItems(myui.Center).Gap(6).
				Cursor(myui.CursorPointer).Label(m.Kind)
			if t.selected == index {
				row.Background(st.BrandSoft)
			} else if row.Hovered() {
				row.Background(st.Recessed)
			}
			row.Children(func() {
				myui.Text(c, fmt.Sprintf("%d. %s", index+1, annotateKindLabel(m.Kind))).
					FontSize(11.5).TextColor(st.Text).SingleLine()
				myui.Spacer(c).Grow(1)
				if engine.IsIrreversible(m.Kind) {
					statusPill(c, "不可逆", st.Success)
				}
			})
			if row.Clicked() {
				t.selected = index
			}
		}
		myui.Row(c).FillWidth().Gap(6).Children(func() {
			a.ghostButton(c, st, "删除选中", lucide.Trash2(), func() {
				if t.selected >= 0 && t.selected < len(t.marks) {
					t.marks = append(t.marks[:t.selected], t.marks[t.selected+1:]...)
					t.selected = -1
					t.record()
				}
			})
			a.ghostButton(c, st, "清空", lucide.Eraser(), func() {
				t.marks = nil
				t.selected = -1
				t.record()
			})
			a.ghostButtonState(c, st, "撤销", lucide.Undo2(), !t.steps.CanUndo(), false, func() {
				if prev, ok := t.steps.Undo(); ok {
					t.marks = append([]engine.AnnotateMark(nil), prev...)
					t.selected = len(t.marks) - 1
					a.Invalidate()
				}
			})
			a.ghostButtonState(c, st, "重做", lucide.Redo2(), !t.steps.CanRedo(), false, func() {
				if next, ok := t.steps.Redo(); ok {
					t.marks = append([]engine.AnnotateMark(nil), next...)
					t.selected = len(t.marks) - 1
					a.Invalidate()
				}
			})
		})
	})
}

// annotateKindLabel names an annotation kind. One label table serves the collage
// page's four mark types and the annotation toolbar's eleven, so the same mark never
// reads differently in two places.
func annotateKindLabel(kind string) string {
	switch kind {
	case "text":
		return "文字"
	case "arrow":
		return "箭头"
	case "rect":
		return "矩形"
	case "ellipse":
		return "椭圆"
	}
	for _, tool := range annotateTools {
		if tool.Key == kind {
			return tool.Label
		}
	}
	return kind
}

func (t *AnnotateTool) PreviewCaption(a *App) string {
	return fmt.Sprintf("%d 个标注", len(t.marks))
}

// PreviewPanel draws the picture with the marks and takes pointer input to place
// new ones.
func (t *AnnotateTool) PreviewPanel(c *myui.Context, a *App) {
	t.ensure()
	st := styles(c)
	img := a.Preview()
	if img == nil {
		row := myui.Row(c).Fill().Children(func() {})
		_ = row
		return
	}
	box := myui.Column(c).Fill().Padding(12).Gap(8).Radius(st.RadiusPanel).
		MinHeight(0).Clip().Background(st.Recessed).Label("标注预览区")
	box.Children(func() {
		myui.Row(c).FillWidth().AlignItems(myui.Center).Gap(6).Children(func() {
			myui.Icon(c, lucide.PenTool()).Width(13).Height(13).TextColor(st.TextDim)
			myui.Text(c, annotateKindLabel(annotateTools[t.tool%len(annotateTools)].Key)).
				FontSize(11).TextColor(st.TextSub)
			myui.Spacer(c).Grow(1)
			myui.Text(c, fmt.Sprintf("%d × %d", img.Bounds().Dx(), img.Bounds().Dy())).
				FontSize(11).TextColor(st.TextDim)
		})
		myui.Row(c).FillWidth().Grow(1).MinHeight(0).Clip().Justify(myui.Center).AlignItems(myui.Center).Children(func() {
			canvas := myui.Box(c).Fill().MinWidth(0).MinHeight(0).
				Radius(st.RadiusControl).Background(st.Card).Clip().
				Focusable().FocusRingColor(st.Brand).FocusRingOffset(2).
				Cursor(myui.CursorCrosshair).Label("标注画布").Bind(&t.canvasHandle)
			canvas.HandleInput(func(ev myui.InputEvent) bool {
				return t.pointer(c, a, ev, img)
			})
			canvas.Children(func() {
				myui.Image(c, myui.NewBitmap(img)).Fill().Fit(myui.Contain)
			})
		})
	})
}

// pointer places and sizes marks in source-pixel coordinates.
func (t *AnnotateTool) pointer(c *myui.Context, a *App, ev myui.InputEvent, img image.Image) bool {
	bounds := t.canvasHandle.Bounds(c)
	if bounds.W <= 0 || bounds.H <= 0 {
		return false
	}
	rect := previewFitRect(myui.Rect{X: 0, Y: 0, W: bounds.W, H: bounds.H},
		img.Bounds().Dx(), img.Bounds().Dy())
	toSource := func(x, y float32) (float64, float64) {
		if rect.W <= 0 || rect.H <= 0 {
			return 0, 0
		}
		return clampFloat64(float64((x-rect.X)/rect.W), 0, 1) * float64(img.Bounds().Dx()),
			clampFloat64(float64((y-rect.Y)/rect.H), 0, 1) * float64(img.Bounds().Dy())
	}
	kind := annotateTools[t.tool%len(annotateTools)].Key
	switch ev.Kind {
	case myui.InputPointerDown:
		if ev.Button != 0 || ev.X < rect.X || ev.X > rect.X+rect.W || ev.Y < rect.Y || ev.Y > rect.Y+rect.H {
			return false
		}
		x, y := toSource(ev.X, ev.Y)
		mark := engine.AnnotateMark{
			Kind: kind, X1: x, Y1: y, X2: x, Y2: y,
			Color: t.colorHex, Width: int(t.width), Size: int(t.size),
			Text: t.text, Fill: t.fill, Strength: int(t.strength),
		}
		if kind == engine.AnnotateFree {
			mark.Points = []engine.AnnotatePoint{{X: x, Y: y}}
		}
		if kind == engine.AnnotateNumber && mark.Text == "" {
			mark.Text = fmt.Sprint(len(t.marks) + 1)
		}
		t.marks = append(t.marks, mark)
		t.selected = len(t.marks) - 1
		t.dragging = true
		return true
	case myui.InputPointerMove:
		if !t.dragging || len(t.marks) == 0 {
			return false
		}
		x, y := toSource(ev.X, ev.Y)
		last := &t.marks[len(t.marks)-1]
		if last.Kind == engine.AnnotateFree {
			last.Points = append(last.Points, engine.AnnotatePoint{X: x, Y: y})
		} else {
			last.X2, last.Y2 = x, y
		}
		return true
	case myui.InputPointerUp:
		if !t.dragging {
			return false
		}
		t.dragging = false
		// A click without a drag gets a default size, so a stamp or a text label
		// appears where the user pressed instead of being invisible.
		if len(t.marks) > 0 {
			last := &t.marks[len(t.marks)-1]
			if last.Kind != engine.AnnotateFree && last.Kind != engine.AnnotateText && last.Kind != engine.AnnotateNumber {
				if math.Abs(last.X2-last.X1) < 4 && math.Abs(last.Y2-last.Y1) < 4 {
					last.X2 = math.Min(float64(img.Bounds().Dx()), last.X1+float64(img.Bounds().Dx())/6)
					last.Y2 = math.Min(float64(img.Bounds().Dy()), last.Y1+float64(img.Bounds().Dy())/8)
				}
			}
		}
		t.record()
		a.Invalidate()
		return true
	}
	return false
}

func (t *AnnotateTool) Run(a *App) {
	t.ensure()
	marks := append([]engine.AnnotateMark(nil), t.marks...)
	a.runOutputBatch("标注", func(index int, item batch.Item) (string, engine.Result) {
		ext := strings.TrimPrefix(strings.ToLower(itemPathExt(item.Path)), ".")
		if ext == "" {
			ext = "png"
		}
		out, skipped := a.outputPathFor(item, "annotate", ext, index+1)
		if skipped {
			return "", engine.Skip("目标已存在")
		}
		return out, engine.OK()
	}, func(_ context.Context, index int, item batch.Item, out string) engine.Result {
		return engine.Annotate(engine.AnnotateRequest{
			InputPath: item.Path, OutputPath: out, Marks: marks, StripMetadata: true,
		})
	})
}

// ---------------------------------------------------------------------------
// 截图美化
// ---------------------------------------------------------------------------

// ScreenshotTool decorates a screenshot with a frame.
type ScreenshotTool struct {
	initialised bool
	padding     float64
	radius      float64
	shadow      float64
	border      float64
	borderHex   string
	bg1         myui.Color
	bg1Hex      string
	bg2         myui.Color
	bg2Hex      string
	useGrad     bool
	gradient    float64
	aspect      int
	format      int
}

func (t *ScreenshotTool) ID() string      { return ToolScreenshot }
func (t *ScreenshotTool) Name() string    { return "截图美化" }
func (t *ScreenshotTool) Icon() *myui.SVG { return lucide.MonitorSmartphone() }

var screenshotAspects = []struct {
	Label string
	W, H  float64
}{{"自动", 0, 0}, {"1:1", 1, 1}, {"4:3", 4, 3}, {"16:9", 16, 9}, {"9:16", 9, 16}, {"3:2", 3, 2}}

func (t *ScreenshotTool) ensure() {
	if !t.initialised {
		t.padding = 72
		t.radius = 18
		t.gradient = 135
		t.initialised = true
	}
	if t.bg1Hex == "" {
		t.bg1Hex = "#F5F5F7"
		t.bg1 = colorOf(t.bg1Hex)
	}
	if t.bg2Hex == "" {
		t.bg2Hex = "#E5E7EB"
		t.bg2 = colorOf(t.bg2Hex)
	}
	if t.borderHex == "" {
		t.borderHex = "#E5E7EB"
	}
}

func (t *ScreenshotTool) Panel(c *myui.Context, a *App) {
	t.ensure()
	sectionCard(c, "画布", func() {
		sliderRow(c, "留白", &t.padding, 0, 400, 4, "%.0f px")
		sliderRow(c, "图片圆角", &t.radius, 0, 120, 1, "%.0f px")
		sliderRow(c, "阴影", &t.shadow, 0, 80, 1, "%.0f px")
		labels := make([]string, 0, len(screenshotAspects))
		for _, aspect := range screenshotAspects {
			labels = append(labels, aspect.Label)
		}
		segmented(c, &t.aspect, labels)
	})

	sectionCard(c, "背景", func() {
		switchField(c, &t.useGrad, "使用渐变")
		myui.Column(c).Gap(5).Children(func() {
			label(c, "起始色")
			before := t.bg1
			colorField(c, &t.bg1)
			if t.bg1 != before {
				t.bg1Hex = hexOf(t.bg1)
			}
		})
		if t.useGrad {
			myui.Column(c).Gap(5).Children(func() {
				label(c, "结束色")
				before := t.bg2
				colorField(c, &t.bg2)
				if t.bg2 != before {
					t.bg2Hex = hexOf(t.bg2)
				}
			})
			sliderRow(c, "渐变角度", &t.gradient, 0, 360, 5, "%.0f°")
		}
	})

	sectionCard(c, "描边", func() {
		sliderRow(c, "宽度", &t.border, 0, 20, 1, "%.0f px")
		if t.border > 0 {
			myui.Column(c).Gap(5).Children(func() {
				label(c, "颜色")
				col := colorOf(t.borderHex)
				before := col
				colorField(c, &col)
				if col != before {
					t.borderHex = hexOf(col)
				}
			})
		}
	})

	sectionCard(c, "输出", func() {
		segmented(c, &t.format, []string{"png", "jpg", "webp"})
	})
}

func (t *ScreenshotTool) PreviewCaption(a *App) string {
	return fmt.Sprintf("留白 %.0f px · 圆角 %.0f px", t.padding, t.radius)
}

func (t *ScreenshotTool) request() engine.ScreenshotRequest {
	t.ensure()
	aspect := screenshotAspects[t.aspect%len(screenshotAspects)]
	req := engine.ScreenshotRequest{
		Padding: int(t.padding), Radius: int(t.radius),
		ShadowDistance: int(t.shadow), ShadowOpacity: 0.22,
		BorderWidth: int(t.border), BorderColor: t.borderHex,
		Background: t.bg1Hex, GradientAngle: t.gradient,
		AspectW: aspect.W, AspectH: aspect.H,
	}
	if t.useGrad {
		req.BackgroundTo = t.bg2Hex
	}
	return req
}

func (t *ScreenshotTool) Run(a *App) {
	req := t.request()
	format := []string{"png", "jpg", "webp"}[t.format%3]
	a.runOutputBatch("截图美化", func(index int, item batch.Item) (string, engine.Result) {
		out, skipped := a.outputPathFor(item, "screenshot", extOf(format), index+1)
		if skipped {
			return "", engine.Skip("目标已存在")
		}
		return out, engine.OK()
	}, func(_ context.Context, index int, item batch.Item, out string) engine.Result {
		one := req
		one.InputPath, one.OutputPath = item.Path, out
		return engine.Screenshot(one)
	})
}

// ---------------------------------------------------------------------------
// 专业调色
// ---------------------------------------------------------------------------

// GradeTool is the professional grading page.
type GradeTool struct {
	lutJob        studioJob[*engine.LUT3D]
	initialised   bool
	appliedPreset int
	preset        int
	exposure      float64
	brightness    float64
	contrast      float64
	highlights    float64
	shadows       float64
	whites        float64
	blacks        float64
	saturation    float64
	vibrance      float64
	temperature   float64
	tint          float64
	inputBlack    float64
	inputGamma    float64
	inputWhite    float64
	curveMiddle   float64
	lutPath       string
	lutStrength   float64
	lut           *engine.LUT3D
	lutErr        string
	format        int
	steps         *history.Stack[engine.GradeParams]
}

func (t *GradeTool) ID() string      { return ToolGrade }
func (t *GradeTool) Name() string    { return "专业调色" }
func (t *GradeTool) Icon() *myui.SVG { return lucide.Wand2() }

func (t *GradeTool) ensure() {
	if !t.initialised {
		t.inputWhite = 255
		t.inputGamma = 1
		t.curveMiddle = 128
		t.lutStrength = 100
		t.initialised = true
	}
	if t.steps == nil {
		t.steps = history.New[engine.GradeParams](60)
		t.steps.Reset(t.params())
	}
}

// params assembles the engine parameter set from the panel fields.
func (t *GradeTool) params() engine.GradeParams {
	t.ensure()
	p := engine.GradeParams{
		Exposure: t.exposure, Brightness: t.brightness, Contrast: t.contrast,
		Highlights: t.highlights, Shadows: t.shadows, Whites: t.whites, Blacks: t.blacks,
		Saturation: t.saturation, Vibrance: t.vibrance,
		Temperature: t.temperature, Tint: t.tint,
		InputBlack: t.inputBlack, InputGamma: t.inputGamma, InputWhite: t.inputWhite,
	}
	if t.curveMiddle != 128 {
		p.Curve = engine.Curve{Points: []engine.CurvePoint{
			{X: 0, Y: 0}, {X: 128, Y: t.curveMiddle}, {X: 255, Y: 255},
		}}
	}
	if t.lut != nil {
		p.LUT, p.LUTStrength = t.lut, t.lutStrength
	}
	return p
}

func (t *GradeTool) Panel(c *myui.Context, a *App) {
	t.ensure()
	st := styles(c)
	sectionCard(c, "预设", func() {
		labels := make([]string, 0, len(engine.GradePresets))
		for _, p := range engine.GradePresets {
			labels = append(labels, p.Label)
		}
		choiceGrid(c, &t.preset, labels, 4)
		if t.preset != t.appliedPreset && t.preset >= 0 && t.preset < len(engine.GradePresets) {
			t.appliedPreset = t.preset
			t.applyPreset(engine.GradePresets[t.preset].Params)
		}
	})

	sectionCard(c, "曝光与对比", func() {
		sliderRow(c, "曝光 EV", &t.exposure, -3, 3, 0.05, "%.2f")
		sliderRow(c, "亮度", &t.brightness, -100, 100, 1, "%.0f")
		sliderRow(c, "对比度", &t.contrast, -100, 100, 1, "%.0f")
	})

	sectionCard(c, "明暗区域", func() {
		sliderRow(c, "高光", &t.highlights, -100, 100, 1, "%.0f")
		sliderRow(c, "阴影", &t.shadows, -100, 100, 1, "%.0f")
		sliderRow(c, "白色色阶", &t.whites, -100, 100, 1, "%.0f")
		sliderRow(c, "黑色色阶", &t.blacks, -100, 100, 1, "%.0f")
	})

	sectionCard(c, "颜色", func() {
		sliderRow(c, "饱和度", &t.saturation, -100, 100, 1, "%.0f")
		sliderRow(c, "自然饱和度", &t.vibrance, -100, 100, 1, "%.0f")
		sliderRow(c, "色温", &t.temperature, -100, 100, 1, "%.0f")
		sliderRow(c, "色调", &t.tint, -100, 100, 1, "%.0f")
	})

	sectionCard(c, "色阶与曲线", func() {
		sliderRow(c, "输入黑点", &t.inputBlack, 0, 200, 1, "%.0f")
		sliderRow(c, "输入 Gamma", &t.inputGamma, 0.2, 3, 0.05, "%.2f")
		sliderRow(c, "输入白点", &t.inputWhite, 60, 255, 1, "%.0f")
		sliderRow(c, "曲线中点", &t.curveMiddle, 0, 255, 1, "%.0f")
	})

	sectionCard(c, "LUT", func() {
		myui.Row(c).FillWidth().Gap(8).AlignItems(myui.Center).Children(func() {
			myui.Text(c, truncate(filepath.Base(t.lutPath), 22)).FontSize(11).
				TextColor(st.TextSub).SingleLine()
			myui.Spacer(c).Grow(1)
			a.ghostButton(c, st, "导入 .cube", lucide.Upload(), func() {
				a.pickLUT(&t.lutPath, func() {
					t.lut = nil
					t.lutErr = ""
					a.Invalidate()
				})
			})
			if t.lutPath != "" {
				a.ghostButton(c, st, "清除", lucide.Eraser(), func() {
					t.lutPath = ""
					t.lut = nil
					t.lutErr = ""
				})
			}
		})
		if t.lutPath != "" {
			path := t.lutPath
			lut, busy, err := t.lutJob.request(a, path, func(context.Context) (*engine.LUT3D, error) { return engine.LoadLUT(path) })
			if busy {
				myui.Text(c, "正在读取 LUT").FontSize(11).TextColor(st.TextDim)
			} else {
				t.lut, t.lutErr = lut, ""
				if err != nil {
					t.lutErr = err.Error()
				}
			}
		}
		if t.lutErr != "" {
			warnAlert(c, t.lutErr)
		}
		if t.lut != nil {
			myui.Text(c, fmt.Sprintf("%d³ 表 · %s", t.lut.Size, truncate(t.lut.Title, 20))).
				FontSize(10.5).TextColor(st.TextDim)
			sliderRow(c, "强度", &t.lutStrength, 0, 100, 1, "%.0f%%")
		}
	})

	sectionCard(c, "历史", func() {
		myui.Row(c).FillWidth().Gap(8).AlignItems(myui.Center).Children(func() {
			step, total := t.steps.Position()
			myui.Text(c, fmt.Sprintf("第 %d / %d 步", step, total)).FontSize(12).TextColor(st.TextSub)
			myui.Spacer(c).Grow(1)
			a.ghostButtonState(c, st, "撤销", lucide.Undo2(), !t.steps.CanUndo(), false, func() {
				if prev, ok := t.steps.Undo(); ok {
					t.applyParams(prev)
				}
			})
			a.ghostButtonState(c, st, "重做", lucide.Redo2(), !t.steps.CanRedo(), false, func() {
				if next, ok := t.steps.Redo(); ok {
					t.applyParams(next)
				}
			})
			a.ghostButton(c, st, "复位", lucide.RotateCcw(), func() {
				t.applyParams(engine.GradeParams{})
			})
		})
	})

	sectionCard(c, "输出", func() {
		segmented(c, &t.format, []string{"png", "jpg", "webp"})
	})
	// Every frame records the parameter set; duplicates are dropped by the
	// comparison, so a still slider does not grow the history.
	t.steps.PushIfChanged(t.params(), gradeParamsEqual)
}

// applyPreset copies a built-in look into the editable fields.
func (t *GradeTool) applyPreset(p engine.GradeParams) {
	t.applyParams(p)
}

// applyParams copies a parameter set into the panel fields.
func (t *GradeTool) applyParams(p engine.GradeParams) {
	t.exposure, t.brightness, t.contrast = p.Exposure, p.Brightness, p.Contrast
	t.highlights, t.shadows = p.Highlights, p.Shadows
	t.whites, t.blacks = p.Whites, p.Blacks
	t.saturation, t.vibrance = p.Saturation, p.Vibrance
	t.temperature, t.tint = p.Temperature, p.Tint
	t.inputBlack, t.inputGamma, t.inputWhite = p.InputBlack, p.InputGamma, p.InputWhite
	if p.InputGamma == 0 {
		t.inputGamma = 1
	}
	if p.InputWhite == 0 {
		t.inputWhite = 255
	}
	t.curveMiddle = 128
	for _, pt := range p.Curve.Points {
		if pt.X == 128 {
			t.curveMiddle = pt.Y
		}
	}
}

// gradeParamsEqual compares two parameter sets. The LUT is compared by identity,
// which is right: two parameters that reference the same table grade the same.
func gradeParamsEqual(a, b engine.GradeParams) bool {
	if a.Exposure != b.Exposure || a.Brightness != b.Brightness || a.Contrast != b.Contrast ||
		a.Highlights != b.Highlights || a.Shadows != b.Shadows ||
		a.Whites != b.Whites || a.Blacks != b.Blacks ||
		a.Saturation != b.Saturation || a.Vibrance != b.Vibrance ||
		a.Temperature != b.Temperature || a.Tint != b.Tint ||
		a.InputBlack != b.InputBlack || a.InputGamma != b.InputGamma || a.InputWhite != b.InputWhite {
		return false
	}
	if (a.LUT == nil) != (b.LUT == nil) || a.LUTStrength != b.LUTStrength {
		return false
	}
	if len(a.Curve.Points) != len(b.Curve.Points) {
		return false
	}
	for i := range a.Curve.Points {
		if a.Curve.Points[i] != b.Curve.Points[i] {
			return false
		}
	}
	return true
}

func (t *GradeTool) PreviewCaption(a *App) string {
	name := engine.GradePresets[t.clampPreset()].Label
	if !t.params().Identity() {
		name = "已调整"
	}
	return name
}

func (t *GradeTool) clampPreset() int {
	if t.preset < 0 || t.preset >= len(engine.GradePresets) {
		return 0
	}
	return t.preset
}

func (t *GradeTool) Run(a *App) {
	t.ensure()
	params := t.params()
	format := []string{"png", "jpg", "webp"}[t.format%3]
	a.runOutputBatch("调色", func(index int, item batch.Item) (string, engine.Result) {
		out, skipped := a.outputPathFor(item, "grade", extOf(format), index+1)
		if skipped {
			return "", engine.Skip("目标已存在")
		}
		return out, engine.OK()
	}, func(_ context.Context, index int, item batch.Item, out string) engine.Result {
		opened, err := openSRGB(item.Path)
		if err != nil {
			return engine.Fail("READ_FAILED", err.Error())
		}
		img := engine.Grade(opened, params)
		if err := saveImage(img, out, 95); err != nil {
			return engine.Fail("WRITE_FAILED", err.Error())
		}
		res := engine.OK()
		res.OutputPath = out
		res.Width = img.Bounds().Dx()
		res.Height = img.Bounds().Dy()
		return res
	})
}

// ---------------------------------------------------------------------------
// 滤镜效果
// ---------------------------------------------------------------------------

// FilterTool is the filter library page.
type FilterTool struct {
	kind     string
	strength float64
	amount   float64
	radius   float64
	levels   float64
	angle    float64
	format   int
	// initialised guards the strength default, because 0 is a real value.
	initialised bool
}

func (t *FilterTool) ID() string      { return ToolFilter }
func (t *FilterTool) Name() string    { return "滤镜效果" }
func (t *FilterTool) Icon() *myui.SVG { return lucide.Sparkles() }

func (t *FilterTool) ensure() {
	if t.kind == "" {
		t.kind = "grayscale"
	}
	if !t.initialised && t.amount == 0 {
		t.amount = 50
	}
	if !t.initialised {
		// 0 is a legitimate strength (it means "leave the picture alone"), so the
		// default is seeded once instead of whenever the value reads zero. Writing
		// it every frame made 0% unreachable: the slider snapped back to 100.
		t.strength = 100
		t.initialised = true
	}
	if t.radius == 0 {
		t.radius = 4
	}
	if t.levels == 0 {
		t.levels = 6
	}
}

func (t *FilterTool) Panel(c *myui.Context, a *App) {
	t.ensure()
	st := styles(c)
	sectionCard(c, "滤镜库", func() {
		groups := []string{"基础", "纹理", "算法", "风格"}
		for _, group := range groups {
			items := make([]engine.FilterKind, 0, 4)
			for _, k := range engine.FilterCatalog {
				if k.Group == group {
					items = append(items, k)
				}
			}
			if len(items) == 0 {
				continue
			}
			label(c, group)
			// Four per row: a filter label is two to four characters, and seven in
			// one 420 DIP row squeezed every label to an unreadable width.
			rows := (len(items) + 3) / 4
			for r := 0; r < rows; r++ {
				start, end := r*4, minInt(r*4+4, len(items))
				myui.Row(c).FillWidth().Gap(5).Children(func() {
					for i := start; i < end; i++ {
						key := items[i].Key
						name := items[i].Label
						active := t.kind == key
						btn := myui.ButtonBase(c).Grow(1).Height(34).Radius(st.RadiusPill).
							Justify(myui.Center).AlignItems(myui.Center).
							Cursor(myui.CursorPointer).Label(name)
						if active {
							btn.Gradient(st.Brand, st.BrandGradientTo, 90)
						} else {
							btn.Border(1, st.Divider).Background(st.Recessed)
						}
						btn.Children(func() {
							col := st.TextSub
							if active {
								col = myui.RGB(255, 255, 255)
							}
							myui.Text(c, name).FontSize(10.5).TextColor(col).NoWrap()
						})
						if btn.Clicked() {
							t.kind = key
						}
					}
				})
			}
		}
	})

	sectionCard(c, "参数", func() {
		sliderRow(c, "强度", &t.strength, 0, 100, 1, "%.0f%%")
		switch t.kind {
		case "grain", "sharpen":
			sliderRow(c, "幅度", &t.amount, 0, 100, 1, "%.0f")
		case "vignette":
			sliderRow(c, "暗角", &t.amount, 0, 100, 1, "%.0f")
		case "blur", "pixelate":
			sliderRow(c, "半径", &t.radius, 1, 64, 1, "%.0f px")
		case "motion":
			sliderRow(c, "半径", &t.radius, 1, 64, 1, "%.0f px")
			sliderRow(c, "角度", &t.angle, 0, 360, 5, "%.0f°")
		case "posterize":
			sliderRow(c, "色阶数", &t.levels, 2, 16, 1, "%.0f")
		}
	})

	sectionCard(c, "输出", func() {
		segmented(c, &t.format, []string{"png", "jpg", "webp"})
	})
}

func (t *FilterTool) PreviewCaption(a *App) string {
	return fmt.Sprintf("%s · 强度 %.0f%%", engine.FilterLabel(t.kind), t.strength)
}

func (t *FilterTool) request() engine.FilterRequest {
	t.ensure()
	return engine.FilterRequest{
		Kind: t.kind, Strength: t.strength, Amount: t.amount,
		Radius: t.radius, Levels: int(t.levels), Angle: t.angle,
	}
}

func (t *FilterTool) Run(a *App) {
	req := t.request()
	format := []string{"png", "jpg", "webp"}[t.format%3]
	a.runOutputBatch("滤镜", func(index int, item batch.Item) (string, engine.Result) {
		out, skipped := a.outputPathFor(item, "filter", extOf(format), index+1)
		if skipped {
			return "", engine.Skip("目标已存在")
		}
		return out, engine.OK()
	}, func(_ context.Context, index int, item batch.Item, out string) engine.Result {
		opened, err := openSRGB(item.Path)
		if err != nil {
			return engine.Fail("READ_FAILED", err.Error())
		}
		img := engine.ApplyFilter(opened, req)
		if err := saveImage(img, out, 95); err != nil {
			return engine.Fail("WRITE_FAILED", err.Error())
		}
		res := engine.OK()
		res.OutputPath = out
		res.Width = img.Bounds().Dx()
		res.Height = img.Bounds().Dy()
		return res
	})
}

// ---------------------------------------------------------------------------
// 图片对比
// ---------------------------------------------------------------------------

// CompareTool is the shared comparison page. Its page state is the pair of paths
// and the alignment mode; the metrics are recomputed on demand, not every frame.
type CompareTool struct {
	referencePreview studioJob[image.Image]
	resultPreview    studioJob[image.Image]
	reference        string
	result           string
	align            int
	gain             float64
	showDiff         bool
	metrics          engine.CompareResult
	diff             image.Image
	busy             bool
	err              string
}

func (t *CompareTool) ID() string      { return ToolCompare }
func (t *CompareTool) Name() string    { return "图片对比" }
func (t *CompareTool) Icon() *myui.SVG { return lucide.ScanEye() }

var compareAligns = []engine.AlignMode{engine.AlignNone, engine.AlignStretch, engine.AlignFit}

func (t *CompareTool) ensure() {
	if t.align < 0 || t.align >= len(compareAligns) {
		t.align = 0
	}
	if t.gain == 0 {
		t.gain = 2
	}
}

func (t *CompareTool) Panel(c *myui.Context, a *App) {
	t.ensure()
	st := styles(c)
	rows := a.Session.Snapshot()
	sectionCard(c, "对比对象", func() {
		fileRowActions(c, st,
			"参考图（原图）", pathLabel(t.reference, "尚未选择"),
			[]rowAction{
				{Label: "用首张", Icon: lucide.Image(), OnClick: func() {
					if len(rows) > 0 {
						t.reference = rows[0].Path
						t.recompute(a)
					}
				}},
				{Label: "选择", Icon: lucide.FolderOpen(), OnClick: func() {
					a.pickImageInto(&t.reference, func() { t.recompute(a) })
				}},
			})
		fileRowActions(c, st,
			"结果图（处理后）", pathLabel(t.result, "尚未选择"),
			[]rowAction{
				{Label: "用第二张", Icon: lucide.Image(), OnClick: func() {
					if len(rows) > 1 {
						t.result = rows[1].Path
						t.recompute(a)
					}
				}},
				{Label: "选择", Icon: lucide.FolderOpen(), OnClick: func() {
					a.pickImageInto(&t.result, func() { t.recompute(a) })
				}},
			})
		myui.Column(c).Gap(5).Children(func() {
			label(c, "尺寸不一致时")
			previous := t.align
			segmented(c, &t.align, []string{"不对齐", "拉伸对齐", "等比适配"})
			if previous != t.align {
				t.recompute(a)
			}
		})
		myui.Row(c).FillWidth().Gap(8).Children(func() {
			missing := t.reference == "" || t.result == ""
			a.ghostButtonState(c, st, "交换两图", lucide.ArrowLeftRight(), missing, false, func() {
				t.reference, t.result = t.result, t.reference
				t.recompute(a)
			})
			a.ghostButtonState(c, st, "计算指标", lucide.Activity(), missing || t.busy, false, func() { t.recompute(a) })
		})
	})

	sectionCard(c, "指标", func() {
		if t.busy {
			myui.Text(c, "正在计算…").FontSize(11).TextColor(st.TextDim)
			return
		}
		if t.err != "" {
			warnAlert(c, t.err)
		}
		if !t.metrics.Computable {
			reason := t.metrics.Reason
			if reason == "" {
				reason = "选择两张图片后计算"
			}
			warnAlert(c, reason)
			return
		}
		metricRow(c, st, "PSNR", psnrText(t.metrics))
		metricRow(c, st, "SSIM", fmt.Sprintf("%.4f", t.metrics.SSIM))
		metricRow(c, st, "最大通道差", fmt.Sprintf("%d", t.metrics.MaxChannelDelta))
		metricRow(c, st, "平均通道差", fmt.Sprintf("%.2f", t.metrics.MeanDelta))
		metricRow(c, st, "对齐方式", compareAlignLabel(t.metrics.Align))
		previous := t.gain
		sliderRow(c, "差异放大", &t.gain, 1, 10, 1, "%.0f×")
		if previous != t.gain {
			t.recompute(a)
		}
		switchField(c, &t.showDiff, "显示差异图")
	})
}

// pathLabel names a file for a one-line readout, saying so when none was chosen yet.
func pathLabel(path, empty string) string {
	if strings.TrimSpace(path) == "" {
		return empty
	}
	return truncate(filepath.Base(path), 22)
}

// rowAction is one secondary action on a file row.
type rowAction struct {
	Label   string
	Icon    *myui.SVG
	OnClick func()
}

// fileRowActions is the labelled file row the pages that pick their own sources
// share: a caption, the chosen file's name, and the row's actions.
//
// The name goes in its own box under the caption rather than beside the buttons: a
// path and two buttons in one 420 DIP row left the label squeezed to nothing, and a
// squeezed file name is exactly the information the row exists to show.
func fileRowActions(c *myui.Context, st style, caption, value string, actions []rowAction) {
	myui.Column(c).FillWidth().Gap(6).Children(func() {
		myui.Text(c, caption).FontSize(12).FontWeight(500).TextColor(st.TextSub)
		myui.Row(c).FillWidth().Gap(8).AlignItems(myui.Center).Children(func() {
			myui.Row(c).Grow(1).Height(32).PaddingX(10).Radius(st.RadiusControl).
				AlignItems(myui.Center).Background(st.Recessed).
				Border(1, st.Divider).Children(func() {
				myui.Text(c, value).FontSize(11.5).TextColor(st.TextSub).SingleLine()
			})
			for _, action := range actions {
				action := action
				ghostButtonOf(c, st, action.Label, action.Icon, action.OnClick)
			}
		})
	})
}

func metricRow(c *myui.Context, st style, name, value string) {
	myui.Row(c).FillWidth().Height(24).AlignItems(myui.Center).Children(func() {
		myui.Text(c, name).FontSize(11.5).TextColor(st.TextDim)
		myui.Spacer(c).Grow(1)
		myui.Text(c, value).FontSize(11.5).FontWeight(600).TextColor(st.Text)
	})
}

func psnrText(r engine.CompareResult) string {
	if !r.Computable {
		return "—"
	}
	if r.MaxChannelDelta == 0 {
		return "∞ dB（完全一致）"
	}
	return fmt.Sprintf("%.2f dB", r.PSNR)
}

func compareAlignLabel(mode engine.AlignMode) string {
	switch mode {
	case engine.AlignStretch:
		return "拉伸对齐"
	case engine.AlignFit:
		return "等比适配"
	default:
		return "不对齐"
	}
}

// recompute loads both pictures and measures them off the UI thread.
func (t *CompareTool) recompute(a *App) {
	t.ensure()
	t.metrics = engine.CompareResult{}
	t.diff = nil
	t.err = ""
	if t.busy {
		return
	}
	ref, res := t.reference, t.result
	if ref == "" || res == "" {
		t.err = "需要两张图片"
		return
	}
	align := compareAligns[t.align%len(compareAligns)]
	gain := t.gain
	t.busy = true
	go func() {
		refImg, err := openSRGB(ref)
		var resImg image.Image
		if err == nil {
			resImg, err = openSRGB(res)
		}
		req := engine.CompareRequest{Reference: refImg, Result: resImg, Align: align}
		var metrics engine.CompareResult
		var diff image.Image
		if err == nil {
			metrics = engine.Compare(req)
			if metrics.Computable {
				diff, _ = engine.DiffImage(req, gain)
			}
		}
		publish := func() {
			t.busy = false
			if ref != t.reference || res != t.result || align != compareAligns[t.align%len(compareAligns)] || gain != t.gain {
				t.recompute(a)
				return
			}
			if err != nil {
				t.err = err.Error()
			} else if !metrics.Computable {
				t.err = metrics.Reason
			} else {
				t.err = ""
			}
			t.metrics = metrics
			t.diff = diff
			a.Invalidate()
		}
		a.publishStudio(publish)
	}()
}

func (t *CompareTool) PreviewCaption(a *App) string {
	if t.reference == "" || t.result == "" {
		return "选择两张图片"
	}
	return "对比结果"
}

// PreviewPanel shows the two pictures; the diff replaces the result when asked.
func (t *CompareTool) PreviewPanel(c *myui.Context, a *App) {
	t.ensure()
	st := styles(c)
	box := myui.Column(c).Fill().Padding(12).Gap(8).Radius(st.RadiusPanel).
		MinHeight(0).Clip().Background(st.Recessed).Label("对比区")
	box.Children(func() {
		myui.Row(c).FillWidth().AlignItems(myui.Center).Gap(6).Children(func() {
			myui.Icon(c, lucide.ScanEye()).Width(13).Height(13).TextColor(st.TextDim)
			myui.Text(c, "左：参考图 · 右：结果图").FontSize(11).TextColor(st.TextSub)
			myui.Spacer(c).Grow(1)
			if t.busy {
				loadingIcon(c, 12, st.Brand)
			}
		})
		grid := myui.Grid(c).FillWidth().Grow(1).MinHeight(0).Clip().Columns(2).GapX(8).Children(func() {
			t.showTile(c, a, st, "参考图", t.reference, &t.referencePreview)
			if t.showDiff && t.diff != nil {
				t.showImage(c, st, "差异图", t.diff)
			} else {
				t.showTile(c, a, st, "结果图", t.result, &t.resultPreview)
			}
		})
		_ = grid
	})
}

// showTile loads and shows one path in a letterboxed box.
func (t *CompareTool) showTile(c *myui.Context, a *App, st style, caption, path string, job *studioJob[image.Image]) {
	holder := myui.Column(c).Fill().MinHeight(0).Clip().Gap(6).Radius(st.RadiusControl).
		Background(st.Card).Padding(8).Children(func() {
		myui.Text(c, caption).FontSize(10.5).TextColor(st.TextDim)
		if path == "" {
			myui.Spacer(c).Grow(1)
			return
		}
		img, busy, err := job.request(a, path, func(context.Context) (image.Image, error) { return loadPreviewImage(path) })
		if img != nil {
			myui.Image(c, myui.NewBitmap(img)).FillWidth().Grow(1).MinHeight(0).Clip().Fit(myui.Contain).
				Radius(st.RadiusPill)
		} else if busy {
			myui.Text(c, "正在读取").FontSize(11).TextColor(st.TextDim)
		} else if err != nil {
			myui.Text(c, "无法读取").FontSize(11).TextColor(st.Danger)
		}
	})
	_ = holder
}

// showImage shows an already-decoded picture in a labelled box.
func (t *CompareTool) showImage(c *myui.Context, st style, caption string, img image.Image) {
	myui.Column(c).Fill().MinHeight(0).Clip().Gap(6).Radius(st.RadiusControl).Background(st.Card).
		Padding(8).Children(func() {
		myui.Text(c, caption).FontSize(10.5).TextColor(st.TextDim)
		myui.Image(c, myui.NewBitmap(img)).FillWidth().Grow(1).MinHeight(0).Clip().Fit(myui.Contain).
			Radius(st.RadiusPill)
	})
}

func (t *CompareTool) Run(a *App) {
	defer a.finishRun()
	t.ensure()
	if t.busy {
		a.SetStatus("正在计算指标，请稍后导出")
		return
	}
	// The comparison page writes its own report as a text file, which is the one
	// artefact that belongs to it: the metrics are the result.
	if !t.metrics.Computable {
		a.SetStatus("先计算指标")
		return
	}
	dir := a.outputFolder()
	if dir == "" {
		dir = filepath.Dir(t.reference)
	}
	a.resetPlan()
	path, skipped := a.reserveOutput(filepath.Join(dir, "compare-report.txt"), a.Session.SettingsSnapshot())
	if skipped {
		a.SetStatus("对比报告已跳过：目标已存在")
		return
	}
	report := fmt.Sprintf(
		"参考图：%s\n结果图：%s\n尺寸：%d×%d\n对齐：%s\nPSNR：%s\nSSIM：%.4f\n最大通道差：%d\n平均通道差：%.2f\n",
		t.reference, t.result, t.metrics.Width, t.metrics.Height, compareAlignLabel(t.metrics.Align),
		psnrText(t.metrics), t.metrics.SSIM, t.metrics.MaxChannelDelta, t.metrics.MeanDelta)
	if err := output.Atomic(path, func(file *os.File) error { _, err := file.WriteString(report); return err }); err != nil {
		a.SetStatus("写入报告失败：" + err.Error())
		return
	}
	a.rememberOutput(ToolCompare, path)
	a.SetStatus("对比报告已保存：" + filepath.Base(path))
	a.SetProgress(100, "")
}

// ---------------------------------------------------------------------------
// 图像分析
// ---------------------------------------------------------------------------

// AnalysisTool shows the histograms and the sampled statistics.
type AnalysisTool struct {
	sample      image.Image
	result      engine.AnalysisResult
	path        string
	logY        bool
	showRGB     bool
	probeX      float64
	probeY      float64
	busy        bool
	err         string
	initialised bool
}

func (t *AnalysisTool) ID() string      { return ToolAnalysis }
func (t *AnalysisTool) Name() string    { return "图像分析" }
func (t *AnalysisTool) Icon() *myui.SVG { return lucide.ChartNoAxesColumn() }

func (t *AnalysisTool) ensure() {
	// The RGB channels are on by default: a histogram with only luminance hides the
	// colour cast a user is actually looking for.
	if !t.initialised {
		t.showRGB = true
		t.initialised = true
	}
}

func (t *AnalysisTool) Panel(c *myui.Context, a *App) {
	t.ensure()
	st := styles(c)
	sectionCard(c, "取样", func() {
		fileRowActions(c, st, "图片来源", pathLabel(t.path, "尚未选择图片"), []rowAction{
			{Label: "分析选中", Icon: lucide.Image(), OnClick: func() {
				if path := a.selectedPreviewPath(); path != "" {
					t.analyse(a, path)
				}
			}},
			{Label: "选择", Icon: lucide.FolderOpen(), OnClick: func() {
				a.pickImageInto(&t.path, func() { t.analyse(a, t.path) })
			}},
		})
		switchField(c, &t.logY, "对数纵轴")
		switchField(c, &t.showRGB, "显示 RGB 通道")
	})

	sectionCard(c, "直方图", func() {
		if t.busy {
			myui.Text(c, "正在分析…").FontSize(11).TextColor(st.TextDim)
			return
		}
		if t.err != "" {
			warnAlert(c, t.err)
			return
		}
		if t.result.Width == 0 {
			warnAlert(c, "选择一张图片后分析")
			return
		}
		label(c, "亮度")
		histogramBars(c, st, t.result.Luma, st.Brand, t.logY)
		if t.showRGB {
			label(c, "红")
			histogramBars(c, st, t.result.Red, myui.RGB(239, 68, 68), t.logY)
			label(c, "绿")
			histogramBars(c, st, t.result.Green, myui.RGB(34, 197, 94), t.logY)
			label(c, "蓝")
			histogramBars(c, st, t.result.Blue, myui.RGB(59, 130, 246), t.logY)
		}
		if t.result.HasAlpha {
			label(c, "Alpha")
			histogramBars(c, st, t.result.Alpha, st.TextSub, t.logY)
		}
	})

	sectionCard(c, "像素取样", func() {
		if t.busy || t.err != "" {
			return
		}
		if t.result.Width == 0 {
			warnAlert(c, "先分析一张图片")
			return
		}
		maxX := float64(t.result.Width - 1)
		maxY := float64(t.result.Height - 1)
		if t.probeX > maxX {
			t.probeX = maxX
		}
		if t.probeY > maxY {
			t.probeY = maxY
		}
		sliderRow(c, "X", &t.probeX, 0, maxX, 1, "%.0f")
		sliderRow(c, "Y", &t.probeY, 0, maxY, 1, "%.0f")
		img := t.sample
		if img != nil {
			if probe, ok := engine.SamplePixel(img, int(t.probeX), int(t.probeY)); ok {
				swatch := myui.RGBA(probe.R, probe.G, probe.B, 1)
				myui.Row(c).FillWidth().AlignItems(myui.Center).Gap(8).Children(func() {
					myui.ColorWell(c, &swatch).Size(28, 28).
						Radius(st.RadiusPill).Border(1, st.Divider)
					myui.Text(c, fmt.Sprintf("#%02X%02X%02X · RGBA(%d,%d,%d,%d)",
						probe.R, probe.G, probe.B, probe.R, probe.G, probe.B, probe.A)).
						FontSize(11).TextColor(st.TextSub).SingleLine()
				})
				metricRow(c, st, "叠加白底后对比度",
					fmt.Sprintf("%.2f:1", engine.ContrastRatio(probe, color.NRGBA{R: 255, G: 255, B: 255, A: 255})))
			}
		}
	})

	sectionCard(c, "统计", func() {
		if t.result.Width == 0 {
			return
		}
		scope := "全量统计"
		if t.result.Sampled {
			scope = "采样统计"
		}
		metricRow(c, st, "统计范围", scope)
		metricRow(c, st, "尺寸", fmt.Sprintf("%d × %d", t.result.Width, t.result.Height))
		metricRow(c, st, "取样像素", fmt.Sprintf("%d", t.result.SampledPixels))
		metricRow(c, st, "平均亮度", fmt.Sprintf("%.1f", t.result.MeanLuma))
		metricRow(c, st, "过曝像素", fmt.Sprintf("%d", t.result.OverexposedPixels))
		metricRow(c, st, "纯黑像素", fmt.Sprintf("%d", t.result.UnderexposedPixels))
		metricRow(c, st, "半透明像素", fmt.Sprintf("%d", t.result.TransparentPixels))
		unique := fmt.Sprintf("%d", t.result.UniqueColors)
		if t.result.UniqueColors < 0 {
			unique = "100 万+"
		}
		metricRow(c, st, "不同颜色数", unique)
	})
}

// histogramBars draws one 256-bin histogram as a row of bars.
func histogramBars(c *myui.Context, st style, hist engine.Histogram, tint myui.Color, logY bool) {
	data := hist
	if logY {
		data = hist.LogScaled()
	}
	peak := data.HistogramPeak()
	myui.Row(c).FillWidth().Height(56).Radius(st.RadiusPill).
		Background(st.Recessed).Clip().Children(func() {
		// The bars are painted, not built as 256 elements: a row of 256 boxes
		// would cost a layout pass per frame for a picture that never changes.
		for i := 0; i < 256; i += 4 {
			start := i
			myui.Box(c).Grow(1).FillHeight().Draw(func(p *myui.Painter, r myui.Rect) {
				height := float32(0)
				if peak > 0 {
					// A block covers four bins; its height is the tallest of them.
					best := 0
					for k := start; k < start+4 && k < 256; k++ {
						if data[k] > best {
							best = data[k]
						}
					}
					height = float32(best) / float32(peak) * r.H
				}
				if height < 0.5 {
					height = 0.5
				}
				p.Fill(myui.Rect{X: r.X, Y: r.Y + r.H - height, W: r.W, H: height}, tint, 0)
			})
		}
	})
}

// analyse measures one picture off the UI thread.
func (t *AnalysisTool) analyse(a *App, path string) {
	if path == "" {
		return
	}
	t.path = path
	t.err = ""
	t.result = engine.AnalysisResult{}
	t.sample = nil
	if t.busy {
		return
	}
	t.busy = true
	a.SetStatus("正在分析：" + filepath.Base(path))
	go func() {
		img, err := openSRGB(path)
		var res engine.AnalysisResult
		if err == nil {
			res = engine.Analyze(engine.AnalysisRequest{Image: img, MaxSamples: 4_000_000})
		}
		publish := func() {
			t.busy = false
			if path != t.path {
				t.analyse(a, t.path)
				return
			}
			if err != nil {
				t.err = err.Error()
				a.SetStatus("分析失败：" + err.Error())
			} else {
				t.err = ""
				t.result = res
				t.sample = img
				a.SetStatus("已分析：" + filepath.Base(path))
			}
			a.Invalidate()
		}
		a.publishStudio(publish)
	}()
}

func (t *AnalysisTool) PreviewCaption(a *App) string {
	return "直方图与统计"
}

func (t *AnalysisTool) Run(a *App) {
	defer a.finishRun()
	if t.path == "" {
		t.path = a.selectedPreviewPath()
	}
	if t.path == "" {
		a.SetStatus("先选择一张图片")
		return
	}
	t.analyse(a, t.path)
}

// ---------------------------------------------------------------------------
// 像素画 / 点阵图
// ---------------------------------------------------------------------------

// PixelTool is the pixel-art page.
type PixelTool struct {
	initialised   bool
	appliedPreset int
	preset        int
	width         float64
	height        float64
	fit           int
	colors        int
	gray          bool
	strict        bool
	trueBit       bool
	dither        int
	strength      float64
	threshold     float64
	invert        bool
	blackHex      string
	whiteHex      string
	bgHex         string
	format        int
}

func (t *PixelTool) ID() string      { return ToolPixel }
func (t *PixelTool) Name() string    { return "像素画 / 点阵" }
func (t *PixelTool) Icon() *myui.SVG { return lucide.Grid2X2() }

var pixelPresets = []struct {
	Label string
	W, H  int
}{{"400×300", 400, 300}, {"4:5", 400, 500}, {"1:1", 512, 512}, {"16:9", 640, 360}, {"自定义", 0, 0}}

func (t *PixelTool) ensure() {
	if t.width == 0 {
		t.width = 400
	}
	if t.height == 0 {
		t.height = 300
	}
	if t.colors == 0 {
		t.colors = 2
	}
	if !t.initialised {
		t.strength = 100
		t.threshold = 128
		t.strict, t.gray = true, true
		t.initialised = true
	}
	if t.blackHex == "" {
		t.blackHex = "#000000"
	}
	if t.whiteHex == "" {
		t.whiteHex = "#FFFFFF"
	}
	if t.bgHex == "" {
		t.bgHex = "#FFFFFF"
	}
}

func (t *PixelTool) Panel(c *myui.Context, a *App) {
	t.ensure()
	sectionCard(c, "预设", func() {
		labels := make([]string, 0, len(pixelPresets))
		for _, p := range pixelPresets {
			labels = append(labels, p.Label)
		}
		choiceGrid(c, &t.preset, labels, 3)
		if t.preset != t.appliedPreset && t.preset >= 0 && t.preset < len(pixelPresets) {
			t.appliedPreset = t.preset
			p := pixelPresets[t.preset]
			if p.W > 0 {
				t.width, t.height = float64(p.W), float64(p.H)
			}
			if t.preset == 0 {
				t.strict, t.gray = true, true
			}
		}
	})

	sectionCard(c, "尺寸与适配", func() {
		grid2(c, func() {
			myui.Column(c).Gap(5).Children(func() {
				label(c, "宽度")
				numberField(c, &t.width, 8, 4000, 8, "400")
			})
			myui.Column(c).Gap(5).Children(func() {
				label(c, "高度")
				numberField(c, &t.height, 8, 4000, 8, "300")
			})
		})
		segmented(c, &t.fit, []string{"裁切填满", "等比留白", "拉伸"})
		if t.width*4 > 4000 && t.height*4 > 4000 {
			warnAlert(c, "目标尺寸较大时预览会变慢")
		}
	})

	sectionCard(c, "颜色与抖动", func() {
		switchField(c, &t.strict, "严格黑白")
		if !t.strict {
			levels := float64(t.colors)
			sliderRow(c, "色数", &levels, 2, 16, 1, "%.0f")
			t.colors = int(levels)
			switchField(c, &t.gray, "灰阶调色板")
		}
		sliderRow(c, "强度", &t.strength, 0, 100, 1, "%.0f%%")
		labels := make([]string, 0, len(engine.DitherLabels))
		labels = append(labels, engine.DitherLabels...)
		choiceGrid(c, &t.dither, labels, 2)
		sliderRow(c, "亮度阈值", &t.threshold, 0, 255, 1, "%.0f")
		switchField(c, &t.invert, "反相")
	})

	sectionCard(c, "黑白编码", func() {
		switchField(c, &t.trueBit, "1-bit PNG")
		if t.trueBit && !t.strict {
			warnAlert(c, "1-bit 编码需要严格黑/白模式")
		}
		myui.Column(c).Gap(5).Children(func() {
			label(c, "黑色")
			col := colorOf(t.blackHex)
			before := col
			colorField(c, &col)
			if col != before {
				t.blackHex = hexOf(col)
			}
		})
		myui.Column(c).Gap(5).Children(func() {
			label(c, "白色")
			col := colorOf(t.whiteHex)
			before := col
			colorField(c, &col)
			if col != before {
				t.whiteHex = hexOf(col)
			}
		})
	})

	sectionCard(c, "透明合成与输出", func() {
		myui.Column(c).Gap(5).Children(func() {
			label(c, "透明像素先合成到")
			col := colorOf(t.bgHex)
			before := col
			colorField(c, &col)
			if col != before {
				t.bgHex = hexOf(col)
			}
		})
		segmented(c, &t.format, []string{"png", "gif", "webp"})
		if t.trueBit && t.format != 0 {
			warnAlert(c, "1-bit 编码只能写 PNG")
		}
	})
}

func (t *PixelTool) PreviewCaption(a *App) string {
	mode := "多色"
	if t.strict {
		mode = "严格黑白"
	}
	return fmt.Sprintf("%d×%d · %s", int(t.width), int(t.height), mode)
}

func (t *PixelTool) request() engine.PixelRequest {
	t.ensure()
	fitModes := []string{"cover", "contain", "stretch"}
	return engine.PixelRequest{
		Width: int(t.width), Height: int(t.height), FitMode: fitModes[t.fit%3],
		Colors: t.colors, Gray: t.gray, StrictBlackWhite: t.strict,
		Black: t.blackHex, White: t.whiteHex, TrueBitDepth: t.trueBit,
		Dither:   engine.DitherModes[t.dither%len(engine.DitherModes)],
		Strength: t.strength, Threshold: int(t.threshold), Invert: t.invert,
		Background: t.bgHex,
	}
}

func (t *PixelTool) Run(a *App) {
	req := t.request()
	format := []string{"png", "gif", "webp"}[t.format%3]
	a.runOutputBatch("像素化", func(index int, item batch.Item) (string, engine.Result) {
		out, skipped := a.outputPathFor(item, "pixel", extOf(format), index+1)
		if skipped {
			return "", engine.Skip("目标已存在")
		}
		return out, engine.OK()
	}, func(_ context.Context, index int, item batch.Item, out string) engine.Result {
		one := req
		one.InputPath, one.OutputPath = item.Path, out
		res := engine.Pixelate(one)
		if res.Success && res.BitDepth == 1 {
			res.Message = "1-bit PNG"
		}
		return res.Result
	})
}

// ---------------------------------------------------------------------------
// 表情包制作
// ---------------------------------------------------------------------------

// EmoteTool is the sticker page.
type EmoteTool struct {
	initialised bool
	template    int
	top         string
	bottom      string
	center      string
	font        string
	size        float64
	color       myui.Color
	colorHex    string
	stroke      float64
	strokeCol   myui.Color
	strokeHex   string
	shadow      bool
	bubble      myui.Color
	bubbleHex   string
	start       float64
	end         float64
	applyLoop   bool
	loop        float64
	format      int
}

func (t *EmoteTool) ID() string      { return ToolEmote }
func (t *EmoteTool) Name() string    { return "表情包制作" }
func (t *EmoteTool) Icon() *myui.SVG { return lucide.SmilePlus() }

var emoteTemplates = []struct {
	Label string
	Key   string
}{{"上下字幕", engine.EmoteTopBottom}, {"居中大字", engine.EmoteCenter}, {"对话气泡", engine.EmoteBubble}}

func (t *EmoteTool) ensure() {
	if !t.initialised {
		t.bottom = "ImageFlow"
		t.stroke = 3
		t.initialised = true
	}
	if t.colorHex == "" {
		t.colorHex = "#FFFFFF"
		t.color = colorOf(t.colorHex)
	}
	if t.strokeHex == "" {
		t.strokeHex = "#000000"
		t.strokeCol = colorOf(t.strokeHex)
	}
	if t.bubbleHex == "" {
		t.bubbleHex = "#FFFFFF"
		t.bubble = colorOf(t.bubbleHex)
	}
	if t.font == "" {
		t.font = defaultFontFamily()
	}
}

func (t *EmoteTool) Panel(c *myui.Context, a *App) {
	t.ensure()
	sectionCard(c, "模板", func() {
		labels := make([]string, 0, len(emoteTemplates))
		for _, tpl := range emoteTemplates {
			labels = append(labels, tpl.Label)
		}
		segmented(c, &t.template, labels)
		key := emoteTemplates[t.template%len(emoteTemplates)].Key
		switch key {
		case engine.EmoteTopBottom:
			myui.Column(c).Gap(5).Children(func() {
				label(c, "上字幕")
				textField(c, &t.top, "可选")
			})
			myui.Column(c).Gap(5).Children(func() {
				label(c, "下字幕")
				textField(c, &t.bottom, "可选")
			})
		case engine.EmoteCenter:
			myui.Column(c).Gap(5).Children(func() {
				label(c, "居中文字")
				textField(c, &t.center, "输入文字")
			})
		case engine.EmoteBubble:
			myui.Column(c).Gap(5).Children(func() {
				label(c, "气泡文字")
				textField(c, &t.center, "输入台词")
			})
			myui.Column(c).Gap(5).Children(func() {
				label(c, "气泡底色")
				before := t.bubble
				colorField(c, &t.bubble)
				if t.bubble != before {
					t.bubbleHex = hexOf(t.bubble)
				}
			})
		}
	})

	sectionCard(c, "文字样式", func() {
		myui.Column(c).Gap(5).Children(func() {
			label(c, "字体")
			selectField(c, &t.font, fontOptions())
		})
		myui.Column(c).Gap(5).Children(func() {
			label(c, "填充色")
			before := t.color
			colorField(c, &t.color)
			if t.color != before {
				t.colorHex = hexOf(t.color)
			}
		})
		myui.Column(c).Gap(5).Children(func() {
			label(c, "描边色")
			before := t.strokeCol
			colorField(c, &t.strokeCol)
			if t.strokeCol != before {
				t.strokeHex = hexOf(t.strokeCol)
			}
		})
		sliderRow(c, "描边", &t.stroke, 0, 20, 1, "%.0f")
		switchField(c, &t.shadow, "投影")
	})

	sectionCard(c, "动图帧区间", func() {
		grid2(c, func() {
			myui.Column(c).Gap(5).Children(func() {
				label(c, "起始帧（0 = 第一帧）")
				numberField(c, &t.start, 0, 9999, 1, "0")
			})
			myui.Column(c).Gap(5).Children(func() {
				label(c, "结束帧（0 = 末帧）")
				numberField(c, &t.end, 0, 9999, 1, "0")
			})
		})
		switchField(c, &t.applyLoop, "设置循环次数")
		if t.applyLoop {
			myui.Column(c).Gap(5).Children(func() {
				label(c, "循环次数（0 = 无限）")
				numberField(c, &t.loop, 0, 100, 1, "0")
			})
		}
	})

	sectionCard(c, "输出", func() {
		segmented(c, &t.format, []string{"gif", "apng", "webp"})
	})
}

func (t *EmoteTool) PreviewCaption(a *App) string {
	return emoteTemplates[t.template%len(emoteTemplates)].Label
}

func (t *EmoteTool) request() engine.EmoteRequest {
	t.ensure()
	return engine.EmoteRequest{
		Template: emoteTemplates[t.template%len(emoteTemplates)].Key,
		TopText:  t.top, BottomText: t.bottom, CenterText: t.center,
		Font: t.font, FontSize: int(t.size), Color: t.colorHex,
		Stroke: int(t.stroke), StrokeColor: t.strokeHex, Shadow: t.shadow,
		BubbleColor: t.bubbleHex,
		FrameStart:  int(t.start), FrameEnd: int(t.end),
		Loop: int(t.loop), ApplyLoop: t.applyLoop,
	}
}

func (t *EmoteTool) Run(a *App) {
	req := t.request()
	formats := []string{"gif", "apng", "webp"}
	format := formats[t.format%len(formats)]
	a.runOutputBatch("表情制作", func(index int, item batch.Item) (string, engine.Result) {
		out, skipped := a.outputPathFor(item, "emote", extOf(format), index+1)
		if skipped {
			return "", engine.Skip("目标已存在")
		}
		return out, engine.OK()
	}, func(ctx context.Context, index int, item batch.Item, out string) engine.Result {
		one := req
		one.InputPath, one.OutputPath = item.Path, out
		one.OutputFormat = format
		return engine.Emote(ctx, one)
	})
}

// ---------------------------------------------------------------------------
// AI 高清放大
// ---------------------------------------------------------------------------

// UpscaleTool is the optional AI page. Everything on it degrades to an explanation
// when the component is absent: the core build must stay fully usable.
type UpscaleTool struct {
	clipboardImport                       studioJob[string]
	pasteVersion                          uint64
	pasteApplied                          uint64
	results                               sync.Map
	runState                              upscaleRunState
	resultPreview                         studioJob[upscalePair]
	comparisonHandle                      myui.Handle
	comparisonPath                        string
	zoom, split, centerX, centerY         float64
	draggingComparison, panningComparison bool
	pointerX, pointerY                    float64
	preferencesLoaded                     bool
	preferencesMu                         sync.Mutex
	preferencesTimer                      *time.Timer
	lastPreferences                       settings.Upscale
	pendingPreferences                    *settings.Upscale
	gpuIDs                                string
	skipPreview                           bool
	notifyComplete                        bool
	advanced                              bool
	modelImport                           studioJob[upscale.Model]
	importVersion                         uint64
	importApplied                         uint64
	importScale                           int
	double                                bool
	customWidth                           float64
	useCustomWidth                        bool
	compression                           float64
	modelDownload                         studioJob[upscale.Model]
	downloadVersion                       uint64
	downloadApplied                       uint64
	downloadProgress                      modelDownloadProgress
	downloadModel                         string
	modelName                             string
	probe                                 studioJob[upscale.Environment]
	probeVersion                          uint64
	initialised                           bool
	env                                   upscale.Environment
	detected                              bool
	engine                                string
	models                                string
	model                                 int
	scale                                 int
	tile                                  float64
	tta                                   bool
	gpu                                   float64
	format                                int
	busy                                  bool
}

func (t *UpscaleTool) ID() string      { return ToolUpscale }
func (t *UpscaleTool) Name() string    { return "AI 高清放大" }
func (t *UpscaleTool) Icon() *myui.SVG { return lucide.Sparkles() }

func (t *UpscaleTool) ensure() {
	if !t.initialised {
		t.gpu = -1
		t.initialised = true
	}
	if t.scale == 0 {
		t.scale = 4
	}
}

func (t *UpscaleTool) Panel(c *myui.Context, a *App) {
	t.ensure()
	t.loadPreferences(a)
	t.applyClipboard(a)
	t.applyModelDownload(a)
	t.models = a.Settings.AIModelsDir
	if t.downloadModel == "" {
		t.downloadModel = upscale.UpscaylModels()[0].Name
	}
	t.engine = upscale.EnginePathForModel(t.models, upscale.UpscaylModels()[0])
	st := styles(c)
	if !t.detected {
		t.detected = true
		t.probeVersion++
	}
	enginePath, modelsRoot := t.engine, t.models
	env, probing, _ := t.probe.request(a, fmt.Sprintf("%d/%s/%s", t.probeVersion, enginePath, modelsRoot),
		func(ctx context.Context) (upscale.Environment, error) {
			return upscale.DetectContext(ctx, enginePath, modelsRoot), nil
		})
	t.busy = probing
	if !probing {
		t.env = env
	}
	t.downloadPanel(c, a, st)

	sectionCard(c, "放大尺寸", func() {
		switchField(c, &t.double, "双重放大（推理两次）")
		switchField(c, &t.useCustomWidth, "按输出宽度")
		if !t.useCustomWidth || t.double {
			factor := float64(t.scale)
			caption := "放大倍率"
			if t.double && t.useCustomWidth {
				caption = "首轮倍率"
			} else if t.double {
				caption = "每轮倍率"
			}
			sliderRow(c, caption, &factor, 1, 16, 1, "%.0f×")
			t.scale = int(factor)
		}
		if t.useCustomWidth {
			if t.customWidth < 1 {
				t.customWidth = 1920
			}
			label(c, "输出宽度（px）")
			numberField(c, &t.customWidth, 1, 16000, 1, "1920")
		}
		if path := a.selectedPreviewPath(); path != "" {
			for _, item := range a.Session.Snapshot() {
				if item.Path != path || item.Width <= 0 || item.Height <= 0 {
					continue
				}
				customWidth := 0
				if t.useCustomWidth {
					customWidth = int(t.customWidth)
				}
				width, height, err := upscale.ExpectedOutputSize(item.Width, item.Height, t.scale, customWidth, t.double)
				if err != nil {
					warnAlert(c, "当前图片的输出尺寸超限，请降低倍率或宽度")
				} else {
					metricRow(c, st, "选中图片预计输出", fmt.Sprintf("%d × %d px", width, height))
				}
				break
			}
		}
	})

	sectionCard(c, "输出", func() {
		segmented(c, &t.format, []string{"png", "jpg", "webp"})
		sliderRow(c, "压缩", &t.compression, 0, 100, 1, "%.0f%%")
	})
	t.advancedPanel(c, a, st)
	t.persistPreferences(a)
}

func (t *UpscaleTool) PreviewCaption(a *App) string {
	if !t.env.Available {
		return "组件未安装"
	}
	return fmt.Sprintf("本地推理 · %d×", t.scale)
}

func (t *UpscaleTool) Run(a *App) {
	t.ensure()
	t.loadPreferences(a)
	t.persistPreferences(a)
	if blocked := t.upscaleBlock(a); blocked != "" {
		a.SetStatus(blocked)
		a.finishRun()
		return
	}
	modelName := t.downloadModel
	format := []string{"png", "jpg", "webp"}[t.format%3]
	req := upscale.Request{EnginePath: t.engine, ModelsRoot: t.models, Model: modelName,
		Scale: t.scale, Tile: int(t.tile), TTA: t.tta, GPU: int(t.gpu), Format: format,
		Double: t.double, Compression: int(t.compression), GPUs: t.gpuIDs}
	if t.useCustomWidth {
		req.CustomWidth = int(t.customWidth)
	}
	batchSize := a.Session.Len()
	t.runState.start("")
	a.runOutputBatchWithConcurrency("AI 放大", 1, func(index int, item batch.Item) (string, engine.Result) {
		out, skipped := a.outputPathFor(item, "upscale", extOf(format), index+1)
		if skipped {
			return "", engine.Skip("目标已存在")
		}
		return out, engine.OK()
	}, func(ctx context.Context, index int, item batch.Item, out string) engine.Result {
		one := req
		one.InputPath, one.OutputPath = item.Path, out
		info, statErr := os.Stat(item.Path)
		if statErr != nil {
			return engine.Fail("INPUT_MISSING", statErr.Error())
		}
		t.results.Delete(item.Path)
		t.runState.start(fmt.Sprintf("%d/%d · %s", index+1, batchSize, item.Name))
		a.Invalidate()
		one.Progress = func(percent int, line string) {
			if t.runState.record(percent, line) {
				a.Invalidate()
			}
		}
		report, err := upscale.Run(ctx, one)
		if err != nil {
			code := "ENGINE_FAILED"
			if e, ok := err.(*upscale.Error); ok {
				code = e.Code
			}
			return engine.Fail(code, err.Error())
		}
		res := engine.OK()
		t.runState.record(100, "")
		t.results.Store(item.Path, upscaleResult{report: *report, sourceSize: info.Size(), sourceMod: info.ModTime().UnixNano()})
		statsErr := a.Session.UpdateSettings(func(next *settings.Settings) {
			next.UpscaleStats.Images++
			if batchSize > 1 {
				next.UpscaleStats.BatchImages++
			}
			if req.Double {
				next.UpscaleStats.DoubleImages++
			}
			next.UpscaleStats.LastMillis = report.Elapsed.Milliseconds()
			next.UpscaleStats.TotalMillis += report.Elapsed.Milliseconds()
			next.UpscaleStats.LastUsed = time.Now().Format("2006-01-02 15:04")
		})
		if statsErr != nil {
			report.Warnings = append(report.Warnings, "本地统计保存失败："+statsErr.Error())
		}
		res.OutputPath = out
		res.Width, res.Height = report.Width, report.Height
		if len(report.Warnings) > 0 {
			res.Warning = strings.Join(report.Warnings, "；")
		}
		res.Message = report.Model
		return res
	})
}
