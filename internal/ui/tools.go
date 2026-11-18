package ui

import (
	"context"
	"fmt"
	"path/filepath"
	"strings"

	myui "github.com/egoist/mygo/ui"
	"github.com/lobehub/lobe-mygo/pkg/lucide"

	"github.com/lllll081926i/imageflow-go/internal/batch"
	"github.com/lllll081926i/imageflow-go/internal/capabilities"
	"github.com/lllll081926i/imageflow-go/internal/engine"
)

// ConvertTool is the format-conversion page.
type ConvertTool struct {
	format      string
	quality     float64
	resize      int // 0 none, 1 percent, 2 pixels
	percent     float64
	width       float64
	height      float64
	maintain    bool
	keepMeta    bool
	icoSizes    string
	initialised bool
}

func (t *ConvertTool) ID() string      { return ToolConvert }
func (t *ConvertTool) Name() string    { return "格式转换" }
func (t *ConvertTool) Icon() *myui.SVG { return lucide.RefreshCw() }

func (t *ConvertTool) ensure() {
	if t.format == "" {
		t.format = "png"
	}
	if t.quality == 0 {
		t.quality = 92
	}
	if t.percent == 0 {
		t.percent = 100
	}
	if t.icoSizes == "" {
		t.icoSizes = "16,32,48,256"
	}
	if !t.initialised {
		// The original app keeps the aspect ratio locked until the user unlocks
		// it, so a zero-value tool must not silently stretch images.
		t.maintain = true
		t.initialised = true
	}
}

// convertFormats is the converter's format ladder. It is derived from the runtime
// capability table, so a format can never appear on the page without an encoder and
// the settings page's 编解码能力 list cannot disagree with this menu.
var convertFormats = capabilities.EncoderFormats()

func (t *ConvertTool) Panel(c *myui.Context, a *App) {
	t.ensure()
	sectionCard(c, "输出格式", func() {
		labels := make([]string, len(convertFormats))
		for i, f := range convertFormats {
			labels[i] = strings.ToUpper(f)
			if f == "webp" {
				labels[i] = "WebP"
			}
		}
		selected := indexOf(convertFormats, t.format)
		segmented(c, &selected, labels)
		if selected >= 0 && selected < len(convertFormats) {
			t.format = convertFormats[selected]
		}
		if engine.QualityFormats[t.format] {
			sliderRow(c, "质量", &t.quality, 1, 100, 1, "%.0f")
		}
		if !capabilities.SupportsExt(t.format).Encode {
			warnAlert(c, "当前格式不支持输出")
		}
		if t.format == "ico" {
			myui.Column(c).Gap(5).Children(func() {
				label(c, "ICO 尺寸（逗号分隔）")
				textField(c, &t.icoSizes, "16,32,48,256")
			})
		}
	})

	sectionCard(c, "尺寸", func() {
		selected := t.resize
		segmented(c, &selected, []string{"不缩放", "百分比", "像素"})
		t.resize = selected
		switch t.resize {
		case 1:
			sliderRow(c, "缩放比例", &t.percent, 1, 500, 1, "%.0f%%")
		case 2:
			grid2(c, func() {
				myui.Column(c).Gap(5).Children(func() {
					label(c, "宽度")
					numberField(c, &t.width, 0, 16000, 1, "自动")
				})
				myui.Column(c).Gap(5).Children(func() {
					label(c, "高度")
					numberField(c, &t.height, 0, 16000, 1, "自动")
				})
			})
			switchField(c, &t.maintain, "锁定宽高比")
		}
	})

	sectionCard(c, "高级", func() {
		switchField(c, &t.keepMeta, "保留 EXIF / XMP（JPG / PNG / WebP）")
	})
}

func (t *ConvertTool) PreviewCaption(a *App) string {
	caption := strings.ToUpper(t.format)
	if engine.QualityFormats[t.format] {
		caption += fmt.Sprintf(" · q%d", int(t.quality))
	}
	return caption
}

func (t *ConvertTool) Run(a *App) {
	t.ensure()
	// 控件字段由 UI 线程每帧读写，而闭包在 worker goroutine 中运行；必须在启动
	// goroutine 前把闭包会读到的 t.* 字段全部快照成局部值，否则会产生数据竞争。
	format := t.format
	quality, maintain, keepMeta := t.quality, t.maintain, t.keepMeta
	resize, percent, width, height := t.resize, t.percent, t.width, t.height
	icoSizes := t.icoSizes
	a.runOutputBatch("转换", func(index int, item batch.Item) (string, engine.Result) {
		ext := extOf(format)
		out, skipped := a.outputPathFor(item, "convert", ext, index+1)
		if skipped {
			return "", engine.Skip("目标已存在")
		}
		return out, engine.OK()
	}, func(_ context.Context, index int, item batch.Item, out string) engine.Result {
		spec := engine.ConvertRequest{
			InputPath:    item.Path,
			OutputPath:   out,
			Format:       format,
			Quality:      int(quality),
			MaintainAR:   maintain,
			KeepMetadata: keepMeta,
		}
		switch resize {
		case 1:
			spec.ResizeMode = "percent"
			spec.ScalePercent = percent
		case 2:
			// "fit" keeps the aspect ratio, "exact" stretches to both numbers.
			// Passing "fit" unconditionally made the aspect switch a no-op. With
			// only one side given there is nothing to stretch, so fit still
			// derives the other side.
			spec.Width = int(width)
			spec.Height = int(height)
			if maintain || spec.Width <= 0 || spec.Height <= 0 {
				spec.ResizeMode = "fit"
			} else {
				spec.ResizeMode = "exact"
			}
		}
		if format == "ico" {
			spec.IcoSizes = parseIcoSizes(icoSizes)
		}
		return engine.Convert(spec)
	})
}

func parseIcoSizes(text string) []int {
	var out []int
	for _, part := range strings.Split(text, ",") {
		var v int
		if _, err := fmt.Sscanf(strings.TrimSpace(part), "%d", &v); err == nil && v >= 8 && v <= 512 {
			out = append(out, v)
		}
	}
	if len(out) == 0 {
		out = []int{16, 32, 48, 256}
	}
	return out
}

func indexOf(list []string, value string) int {
	for i, v := range list {
		if strings.EqualFold(v, value) {
			return i
		}
	}
	return 0
}

// ResizeTool is the standalone resize page: one scale, or a list of preset
// boxes generated in one run.
type ResizeTool struct {
	mode      int
	percent   float64
	edge      float64
	format    string
	quality   float64
	preset    string
	fit       int
	presetBG  myui.Color
	presetHex string
}

func (t *ResizeTool) ID() string      { return ToolResize }
func (t *ResizeTool) Name() string    { return "尺寸调整" }
func (t *ResizeTool) Icon() *myui.SVG { return lucide.Scaling() }

func (t *ResizeTool) ensure() {
	if t.percent == 0 {
		t.percent = 100
	}
	if t.edge == 0 {
		t.edge = 1920
	}
	if t.quality == 0 {
		t.quality = 95
	}
	if t.preset == "" {
		t.preset = "1024x512, 256x128, 64x16"
	}
	if t.presetHex == "" {
		t.presetHex = "#FFFFFF"
		t.presetBG = colorOf(t.presetHex)
	}
}

var presetFits = []string{"等比留白", "等比裁切", "拉伸填满"}

func (t *ResizeTool) Panel(c *myui.Context, a *App) {
	t.ensure()
	sectionCard(c, "缩放方式", func() {
		selected := t.mode
		segmented(c, &selected, []string{"百分比", "长边", "短边", "多尺寸"})
		t.mode = selected
		switch selected {
		case 0:
			sliderRow(c, "缩放比例", &t.percent, 1, 500, 1, "%.0f%%")
		case 1, 2:
			myui.Column(c).Gap(5).Children(func() {
				label(c, "目标边长（像素）")
				numberField(c, &t.edge, 16, 16000, 16, "1920")
			})
		case 3:
			sizes := engine.ParsePresetSizes(t.preset)
			myui.Column(c).Gap(5).Children(func() {
				label(c, "目标尺寸（多个用逗号分隔）")
				textFieldState(c, &t.preset, "1024x512, 256x128, 64x16", len(sizes) == 0)
			})
			if len(sizes) == 0 {
				warnAlert(c, "没有解析出有效尺寸")
			} else {
				myui.Row(c).FillWidth().Wrap().Gap(6).Children(func() {
					for _, size := range sizes {
						statusPill(c, size.String(), styles(c).Brand)
					}
				})
			}
			myui.Column(c).Gap(6).Children(func() {
				label(c, "图片与目标比例不一致时")
				fit := t.fit
				segmented(c, &fit, presetFits)
				t.fit = fit
			})
			if t.fit == 0 {
				myui.Column(c).Gap(5).Children(func() {
					label(c, "留白颜色")
					before := t.presetBG
					colorField(c, &t.presetBG)
					if t.presetBG != before {
						t.presetHex = hexOf(t.presetBG)
					}
				})
			}
		}
	})
	sectionCard(c, "输出", func() {
		opts := []string{"原格式", "jpg", "png", "webp", "avif"}
		idx := 0
		if t.format != "" {
			idx = indexOf(opts, t.format)
		}
		selected := idx
		segmented(c, &selected, opts)
		if selected == 0 {
			t.format = ""
		} else if selected < len(opts) {
			t.format = opts[selected]
		}
		if qualityRelevant(a, t.format) && !(t.mode == 3 && t.format == "") {
			sliderRow(c, "质量", &t.quality, 1, 100, 1, "%.0f")
		}
	})
}

func (t *ResizeTool) PreviewCaption(a *App) string {
	if t.mode == 3 {
		return fmt.Sprintf("%d 个尺寸", len(engine.ParsePresetSizes(t.preset)))
	}
	return "缩放结果"
}

func (t *ResizeTool) Run(a *App) {
	t.ensure()
	if t.mode == 3 {
		sizes := engine.ParsePresetSizes(t.preset)
		if len(sizes) == 0 {
			a.SetStatus("目标尺寸为空")
			a.SetRunning(false)
			return
		}
		modes := []string{"contain", "cover", "stretch"}
		mode := "contain"
		if t.fit >= 0 && t.fit < len(modes) {
			mode = modes[t.fit]
		}
		format := t.format
		if format == "" {
			format = "png"
		}
		presetHex, quality := t.presetHex, t.quality
		a.runOutputBatch("多尺寸", func(index int, item batch.Item) (string, engine.Result) {
			dir, skipped := a.outputDirectoryFor(item, "_sizes")
			if skipped {
				return "", engine.Skip("目标目录已存在")
			}
			return dir, engine.OK()
		}, func(_ context.Context, index int, item batch.Item, dir string) engine.Result {
			return engine.ResizeToPresets(engine.PresetRequest{
				InputPath:    item.Path,
				OutputDir:    dir,
				BaseName:     baseNameOf(item.Path),
				Sizes:        sizes,
				Mode:         mode,
				Background:   presetHex,
				OutputFormat: format,
				Quality:      int(quality),
			})
		})
		return
	}
	format, quality := t.format, t.quality
	percent, edge, modeVal := t.percent, t.edge, t.mode
	a.runOutputBatch("调整", func(index int, item batch.Item) (string, engine.Result) {
		ext := extOf(format)
		if format == "" {
			ext = strings.TrimPrefix(strings.ToLower(itemPathExt(item.Path)), ".")
			if ext == "" {
				ext = "png"
			}
		}
		out, skipped := a.outputPathFor(item, "resize", ext, index+1)
		if skipped {
			return "", engine.Skip("目标已存在")
		}
		return out, engine.OK()
	}, func(_ context.Context, index int, item batch.Item, out string) engine.Result {
		mode := "percent"
		value := percent
		switch modeVal {
		case 1:
			mode = "long_edge"
			value = edge
		case 2:
			mode = "short_edge"
			value = edge
		}
		return engine.Resize(engine.ResizeRequest{
			InputPath:    item.Path,
			OutputPath:   out,
			Mode:         mode,
			Value:        value,
			OutputFormat: format,
			Quality:      int(quality),
		})
	})
}

// GridTool is the grid-cutter page.
type GridTool struct {
	grid    int
	format  string
	quality float64
}

func (t *GridTool) ID() string      { return ToolGrid }
func (t *GridTool) Name() string    { return "宫格切图" }
func (t *GridTool) Icon() *myui.SVG { return lucide.Grid3X3() }

func (t *GridTool) ensure() {
	if t.grid == 0 {
		t.grid = 3
	}
	if t.format == "" {
		t.format = "png"
	}
	if t.quality == 0 {
		t.quality = 95
	}
}

func (t *GridTool) Panel(c *myui.Context, a *App) {
	t.ensure()
	sectionCard(c, "切分", func() {
		// The ladder starts at 2×2, so the index is the grid size less two.
		selected := t.grid - 2
		if selected < 0 {
			selected = 0
		}
		segmented(c, &selected, []string{"2×2", "3×3", "4×4", "5×5"})
		t.grid = selected + 2
	})
	sectionCard(c, "输出", func() {
		opts := []string{"png", "jpg", "webp"}
		selected := indexOf(opts, t.format)
		segmented(c, &selected, opts)
		if selected < len(opts) {
			t.format = opts[selected]
		}
		if qualityRelevant(a, t.format) {
			sliderRow(c, "质量", &t.quality, 1, 100, 1, "%.0f")
		}
	})
}

func (t *GridTool) PreviewCaption(a *App) string {
	return fmt.Sprintf("%d×%d 宫格", t.grid, t.grid)
}

func (t *GridTool) Run(a *App) {
	t.ensure()
	grid, format, quality := t.grid, t.format, t.quality
	a.runOutputBatch("切图", func(index int, item batch.Item) (string, engine.Result) {
		dir, skipped := a.outputDirectoryFor(item, "_grid")
		if skipped {
			return "", engine.Skip("目标目录已存在")
		}
		return dir, engine.OK()
	}, func(_ context.Context, index int, item batch.Item, dir string) engine.Result {
		return engine.CutGrid(engine.GridRequest{
			InputPath:    item.Path,
			OutputDir:    dir,
			GridSize:     grid,
			OutputFormat: format,
			Quality:      int(quality),
			BaseName:     baseNameOf(item.Path),
		})
	})
}

// LayoutTool is the free rows×cols layout page.
type LayoutTool struct {
	rows    int
	cols    int
	square  bool
	format  string
	quality float64
}

func (t *LayoutTool) ID() string      { return ToolLayoutPage }
func (t *LayoutTool) Name() string    { return "图片布局" }
func (t *LayoutTool) Icon() *myui.SVG { return lucide.LayoutGrid() }

func (t *LayoutTool) ensure() {
	if t.rows == 0 {
		t.rows = 3
	}
	if t.cols == 0 {
		t.cols = 3
	}
	if t.format == "" {
		t.format = "png"
	}
	if t.quality == 0 {
		t.quality = 95
	}
}

func (t *LayoutTool) Panel(c *myui.Context, a *App) {
	t.ensure()
	sectionCard(c, "网格", func() {
		grid2(c, func() {
			myui.Column(c).Gap(5).Children(func() {
				label(c, "行")
				rf := float64(t.rows)
				numberField(c, &rf, 1, 50, 1, "3")
				t.rows = int(rf)
			})
			myui.Column(c).Gap(5).Children(func() {
				label(c, "列")
				cf := float64(t.cols)
				numberField(c, &cf, 1, 50, 1, "3")
				t.cols = int(cf)
			})
		})
		switchField(c, &t.square, "先裁成正方形再切分")
	})
	sectionCard(c, "输出", func() {
		opts := []string{"png", "jpg", "webp"}
		selected := indexOf(opts, t.format)
		segmented(c, &selected, opts)
		if selected < len(opts) {
			t.format = opts[selected]
		}
		if qualityRelevant(a, t.format) {
			sliderRow(c, "质量", &t.quality, 1, 100, 1, "%.0f")
		}
	})
}

func (t *LayoutTool) PreviewCaption(a *App) string {
	return fmt.Sprintf("%d 行 × %d 列", t.rows, t.cols)
}

func (t *LayoutTool) Run(a *App) {
	t.ensure()
	square, rows, cols, format, quality := t.square, t.rows, t.cols, t.format, t.quality
	a.runOutputBatch("布局", func(index int, item batch.Item) (string, engine.Result) {
		dir, skipped := a.outputDirectoryFor(item, "_layout")
		if skipped {
			return "", engine.Skip("目标目录已存在")
		}
		return dir, engine.OK()
	}, func(_ context.Context, index int, item batch.Item, dir string) engine.Result {
		mode := "cover"
		if square {
			mode = "square"
		}
		return engine.Split(engine.SplitRequest{
			InputPath:    item.Path,
			OutputDir:    dir,
			Rows:         rows,
			Cols:         cols,
			Mode:         mode,
			OutputFormat: format,
			Quality:      int(quality),
			BaseName:     baseNameOf(item.Path),
		})
	})
}

// StitchTool is the long-image stitching page.
type StitchTool struct {
	vertical   int
	target     float64
	gap        float64
	background string
	borderMode bool
	border     float64
	format     string
	quality    float64
	argb       myui.Color
	// initialised guards the border width: the slider offers 0 (no border), so
	// writing the default whenever it read 0 stopped the user from turning the
	// border off.
	initialised bool
}

func (t *StitchTool) ID() string      { return ToolStitch }
func (t *StitchTool) Name() string    { return "长图拼接" }
func (t *StitchTool) Icon() *myui.SVG { return lucide.Rows3() }

func (t *StitchTool) ensure() {
	if t.format == "" {
		t.format = "png"
	}
	if t.quality == 0 {
		t.quality = 95
	}
	if !t.initialised {
		t.border = 40
		t.initialised = true
	}
	if t.background == "" {
		t.background = "#FFFFFF"
	}
	if t.argb == (myui.Color{}) {
		t.argb = colorOf(t.background)
	}
}

func (t *StitchTool) Panel(c *myui.Context, a *App) {
	t.ensure()
	sectionCard(c, "方向与尺寸", func() {
		selected := t.vertical
		segmented(c, &selected, []string{"竖向拼接", "横向拼接"})
		t.vertical = selected
		grid2(c, func() {
			myui.Column(c).Gap(5).Children(func() {
				if t.vertical == 0 {
					label(c, "统一宽度（0 = 自动）")
				} else {
					label(c, "统一高度（0 = 自动）")
				}
				numberField(c, &t.target, 0, 16000, 16, "自动")
			})
			myui.Column(c).Gap(5).Children(func() {
				label(c, "间距")
				numberField(c, &t.gap, 0, 500, 1, "0")
			})
		})
		myui.Column(c).Gap(5).Children(func() {
			label(c, "背景色")
			before := t.argb
			colorField(c, &t.argb)
			if t.argb != before {
				t.background = hexOf(t.argb)
			}
		})
	})
	sectionCard(c, "边框", func() {
		switchField(c, &t.borderMode, "单张图加边框")
		if t.borderMode {
			sliderRow(c, "边框宽度", &t.border, 0, 200, 1, "%.0f px")
		}
	})
	sectionCard(c, "输出", func() {
		opts := []string{"png", "jpg", "webp"}
		selected := indexOf(opts, t.format)
		segmented(c, &selected, opts)
		if selected < len(opts) {
			t.format = opts[selected]
		}
		if qualityRelevant(a, t.format) {
			sliderRow(c, "质量", &t.quality, 1, 100, 1, "%.0f")
		}
	})
}

func (t *StitchTool) PreviewCaption(a *App) string {
	return fmt.Sprintf("%d 张", a.Session.Len())
}

func (t *StitchTool) Run(a *App) {
	t.ensure()
	rows := a.Session.Snapshot()
	if len(rows) == 0 {
		a.SetStatus("先添加图片")
		return
	}
	a.SetRunning(true)
	cancel := a.beginWholeList()
	rows = a.Session.Snapshot()
	// 裸 goroutine 同样要先把 t.* 快照成局部值，避免与 UI 线程并发读写产生数据竞争。
	vertical, format := t.vertical, t.format
	gap, background, borderMode, border := t.gap, t.background, t.borderMode, t.border
	target, quality := t.target, t.quality
	go func() {
		defer a.finishRun()
		if cancel.Cancelled() {
			a.endWholeList(cancel, rows)
			return
		}
		paths := make([]string, 0, len(rows))
		for _, r := range rows {
			paths = append(paths, r.Path)
		}
		direction := "vertical"
		if vertical == 1 {
			direction = "horizontal"
		}
		ext := extOf(format)
		out, skipped := a.outputPathFor(rows[0], "stitch", ext, 1)
		if skipped {
			if a.endWholeList(cancel, rows) {
				return
			}
			a.SetStatus("拼接已跳过：目标文件已存在")
			for i := range rows {
				a.Session.SetItemStatus(rows[i], "skipped", "目标已存在", "")
			}
			a.Invalidate()
			return
		}
		req := engine.StitchRequest{
			InputPaths:   paths,
			OutputPath:   out,
			Direction:    direction,
			Gap:          int(gap),
			Background:   background,
			BorderMode:   borderMode,
			Border:       int(border),
			OutputFormat: format,
			Quality:      int(quality),
		}
		if direction == "vertical" {
			req.TargetWidth = int(target)
		} else {
			req.TargetHeight = int(target)
		}
		res := engine.StitchContext(cancel, req)
		if a.endWholeList(cancel, rows, res) {
			return
		}
		if res.Success {
			a.SetStatus(fmt.Sprintf("拼接完成：%d 张，%d×%d", res.Count, res.Width, res.Height))
			for i := range rows {
				a.Session.SetItemStatus(rows[i], "ok", "", res.OutputPath)
			}
			a.SetProgress(100, a.StatusLine())
		} else {
			a.SetStatus("拼接失败：" + res.Error)
			for i := range rows {
				a.Session.SetItemStatus(rows[i], "failed", res.Error, "")
			}
			a.SetProgress(0, a.StatusLine())
		}
		if res.Success && a.Session.SettingsSnapshot().OpenOutputOnComplete {
			a.openCompletedPath(res.OutputPath)
		}
		a.Invalidate()
	}()
}

// WatermarkTool is the batch watermark page.
type WatermarkTool struct {
	text      string
	fontName  string
	sizeMode  int
	fontSize  float64
	color     myui.Color
	colorHex  string
	opacity   float64
	rotation  float64
	position  string
	margin    float64
	tiled     bool
	tileGapX  float64
	tileGapY  float64
	stroke    float64
	strokeCol myui.Color
	strokeHex string
	shadow    bool
	quality   float64
	// initialised guards the fields whose zero value is a legitimate setting, so a
	// later choice of 0 is not overwritten with the default on the next frame.
	// The margin is one: 0 puts the watermark flush against the edge.
	initialised bool
}

func (t *WatermarkTool) ID() string      { return ToolWatermark }
func (t *WatermarkTool) Name() string    { return "批量水印" }
func (t *WatermarkTool) Icon() *myui.SVG { return lucide.Stamp() }

func (t *WatermarkTool) ensure() {
	if !t.initialised && t.text == "" {
		t.text = "ImageFlow"
	}
	if t.fontName == "" {
		// The original panel offered a fixed family list and started on 微软雅黑,
		// which AvailableFonts resolves to the first installed candidate.
		t.fontName = defaultFontFamily()
	}
	if t.fontSize == 0 {
		t.fontSize = 32
	}
	if t.opacity == 0 {
		t.opacity = 60
	}
	if t.position == "" {
		t.position = "br"
	}
	if t.colorHex == "" {
		t.colorHex = "#FFFFFF"
	}
	if t.color == (myui.Color{}) {
		t.color = colorOf(t.colorHex)
	}
	if t.strokeHex == "" {
		t.strokeHex = "#000000"
	}
	if t.strokeCol == (myui.Color{}) {
		t.strokeCol = colorOf(t.strokeHex)
	}
	if t.quality == 0 {
		t.quality = 95
	}
	if t.tileGapX == 0 {
		t.tileGapX = 120
	}
	if t.tileGapY == 0 {
		t.tileGapY = 120
	}
	if !t.initialised {
		// Only the first frame seeds the margin: the slider offers 0, so writing
		// the default whenever the value is 0 made flush-to-edge unreachable.
		t.margin = 24
		t.initialised = true
	}
}

func (t *WatermarkTool) Panel(c *myui.Context, a *App) {
	t.ensure()
	sectionCard(c, "文字", func() {
		myui.Column(c).Gap(5).Children(func() {
			label(c, "内容（支持换行）")
			multiLine(c, &t.text, 3, 6, "输入水印文字")
		})
		myui.Column(c).Gap(5).Children(func() {
			label(c, "字体")
			selectField(c, &t.fontName, fontOptions())
		})
		grid2(c, func() {
			myui.Column(c).Gap(5).Children(func() {
				label(c, "字号模式")
				selected := t.sizeMode
				segmented(c, &selected, []string{"像素", "百分比"})
				t.sizeMode = selected
			})
			myui.Column(c).Gap(5).Children(func() {
				label(c, "字号")
				numberField(c, &t.fontSize, 6, 512, 1, "32")
			})
		})
	})

	sectionCard(c, "样式", func() {
		grid2(c, func() {
			myui.Column(c).Gap(5).Children(func() {
				label(c, "颜色")
				before := t.color
				colorField(c, &t.color)
				if t.color != before {
					t.colorHex = hexOf(t.color)
				}
			})
			myui.Column(c).Gap(5).Children(func() {
				label(c, "描边颜色")
				before := t.strokeCol
				colorField(c, &t.strokeCol)
				if t.strokeCol != before {
					t.strokeHex = hexOf(t.strokeCol)
				}
			})
		})
		sliderRow(c, "不透明度", &t.opacity, 1, 100, 1, "%.0f%%")
		grid2(c, func() {
			sliderRow(c, "旋转", &t.rotation, -180, 180, 1, "%.0f°")
			sliderRow(c, "描边宽度", &t.stroke, 0, 20, 1, "%.0f")
		})
		switchField(c, &t.shadow, "投影")
	})

	sectionCard(c, "位置", func() {
		anchorPicker(c, t.position, func(position string) { t.position = position })
		sliderRow(c, "边距", &t.margin, 0, 500, 1, "%.0f px")
		switchField(c, &t.tiled, "平铺重复")
		if t.tiled {
			grid2(c, func() {
				sliderRow(c, "横向间隔", &t.tileGapX, 16, 600, 1, "%.0f")
				sliderRow(c, "纵向间隔", &t.tileGapY, 16, 600, 1, "%.0f")
			})
		}
	})

	if qualityRelevant(a, "") {
		sectionCard(c, "输出", func() {
			sliderRow(c, "质量", &t.quality, 1, 100, 1, "%.0f")
		})
	}
}

func positionGlyph(p string) string {
	switch p {
	case "tl":
		return "↖"
	case "tc":
		return "↑"
	case "tr":
		return "↗"
	case "cl":
		return "←"
	case "c":
		return "·"
	case "cr":
		return "→"
	case "bl":
		return "↙"
	case "bc":
		return "↓"
	default:
		return "↘"
	}
}

func anchorPosition(index int) string {
	if index < 0 || index >= len(engine.CropAnchors) {
		return "c"
	}
	return engine.CropAnchors[index]
}

func anchorPicker(c *myui.Context, selected string, onSelect func(string)) {
	st := styles(c)
	names := []string{"左上", "上中", "右上", "左中", "居中", "右中", "左下", "下中", "右下"}
	myui.Grid(c).Width(132).Columns(3).Gap(4).Children(func() {
		for index, position := range engine.CropAnchors {
			c.Key(position)
			button := myui.ButtonBase(c).FillWidth().Height(32).Radius(st.RadiusControl).
				Label(names[index]).Tooltip(names[index]).Cursor(myui.CursorPointer).FocusRingColor(st.Brand)
			color := st.TextSub
			if position == selected {
				button.Background(st.Brand)
				color = myui.RGB(255, 255, 255)
			} else {
				button.Background(st.Recessed).Border(1, st.Divider)
			}
			button.Center().Children(func() {
				myui.Text(c, positionGlyph(position)).FontSize(13).TextColor(color)
			})
			button.OnClick(func() { onSelect(position) })
		}
	})
}

func (t *WatermarkTool) PreviewCaption(a *App) string {
	return truncate(strings.SplitN(t.text, "\n", 2)[0], 18)
}

func (t *WatermarkTool) Run(a *App) {
	t.ensure()
	request := t.request()
	a.runOutputBatch("水印", func(index int, item batch.Item) (string, engine.Result) {
		ext := strings.TrimPrefix(strings.ToLower(itemPathExt(item.Path)), ".")
		if ext == "" {
			ext = "png"
		}
		out, skipped := a.outputPathFor(item, "watermark", ext, index+1)
		if skipped {
			return "", engine.Skip("目标已存在")
		}
		return out, engine.OK()
	}, func(_ context.Context, index int, item batch.Item, out string) engine.Result {
		req := request
		req.InputPath, req.OutputPath = item.Path, out
		return engine.Watermark(req)
	})
}

func (t *WatermarkTool) request() engine.WatermarkRequest {
	sizeMode := "px"
	if t.sizeMode == 1 {
		sizeMode = "percent"
	}
	return engine.WatermarkRequest{
		Text: t.text, FontName: t.fontName, SizeMode: sizeMode,
		FontSize: int(t.fontSize), Color: t.colorHex, Opacity: int(t.opacity),
		Rotation: int(t.rotation), Position: t.position, Margin: int(t.margin),
		Tiled: t.tiled, TileGapX: int(t.tileGapX), TileGapY: int(t.tileGapY),
		StrokeWide: int(t.stroke), StrokeColr: t.strokeHex, Shadow: t.shadow, Quality: int(t.quality),
	}
}

// PaletteTool extracts colours.
type PaletteTool struct {
	swatchHeight float64
	count        float64
	includeText  bool
	format       string
	initialised  bool
}

func (t *PaletteTool) ID() string      { return ToolPalette }
func (t *PaletteTool) Name() string    { return "色彩提取" }
func (t *PaletteTool) Icon() *myui.SVG { return lucide.Pipette() }

func (t *PaletteTool) ensure() {
	if t.swatchHeight == 0 {
		t.swatchHeight = 96
	}
	if t.count == 0 {
		t.count = 10
	}
	if t.format == "" {
		t.format = "png"
	}
	if !t.initialised {
		// The original palette panel labels every swatch with its HEX value.
		t.includeText = true
		t.initialised = true
	}
}

func (t *PaletteTool) Panel(c *myui.Context, a *App) {
	t.ensure()
	sectionCard(c, "色板", func() {
		sliderRow(c, "色块高度", &t.swatchHeight, 24, 600, 4, "%.0f px")
		sliderRow(c, "色块数量", &t.count, 2, 16, 1, "%.0f 个")
		switchField(c, &t.includeText, "在色块上标注 HEX")
		opts := []string{"png", "jpg"}
		selected := indexOf(opts, t.format)
		segmented(c, &selected, opts)
		if selected < len(opts) {
			t.format = opts[selected]
		}
	})
}

func (t *PaletteTool) PreviewCaption(a *App) string { return "色板" }

func (t *PaletteTool) Run(a *App) {
	t.ensure()
	format, swatchHeight, includeText, count := t.format, t.swatchHeight, t.includeText, t.count
	a.runOutputBatch("取色", func(index int, item batch.Item) (string, engine.Result) {
		ext := extOf(format)
		out, skipped := a.outputPathFor(item, "palette", ext, index+1)
		if skipped {
			return "", engine.Skip("目标已存在")
		}
		return out, engine.OK()
	}, func(_ context.Context, index int, item batch.Item, out string) engine.Result {
		res, palette := engine.ExtractPalette(engine.PaletteRequest{
			InputPath:    item.Path,
			OutputPath:   out,
			SwatchHeight: int(swatchHeight),
			IncludeText:  includeText,
			OutputFormat: format,
			Colors:       int(count),
		})
		if res.Success {
			res.Message = palette.ThemeColor
			if img, err := engine.PaletteImage(
				append([]string{palette.ThemeColor}, palette.Colors...), 96,
			); err == nil {
				a.SetPreview(img)
			}
		}
		return res
	})
}

// PDFTool builds a PDF from the list.
type PDFTool struct {
	layout   string
	rows     float64
	cols     float64
	pageSize string
	margin   float64
	title    string
	author   string
	// orientation is 0 for portrait and 1 for landscape. A bool could not
	// distinguish "not chosen yet" from "landscape", and the original tool
	// defaults to portrait, so a bool defaulting to false silently rotated
	// every page.
	orientation int
	fitMode     string
	password    string
	// initialised guards the margin: 0 is a legitimate value (no margin), so the
	// default must be seeded once instead of on every frame.
	initialised bool
}

var paperOptions = []string{"A4", "A3", "A5", "Letter", "Legal", "B5", "B4", "Tabloid"}

func (t *PDFTool) ID() string      { return ToolPDF }
func (t *PDFTool) Name() string    { return "转 PDF" }
func (t *PDFTool) Icon() *myui.SVG { return lucide.FileText() }

func (t *PDFTool) ensure() {
	if t.layout == "" {
		t.layout = "single"
	}
	if t.pageSize == "" {
		t.pageSize = "A4"
	}
	if !t.initialised {
		t.margin = 36
		t.initialised = true
	}
	if t.rows == 0 {
		t.rows = 2
	}
	if t.cols == 0 {
		t.cols = 2
	}
	if t.fitMode == "" {
		t.fitMode = "contain"
	}
}

func (t *PDFTool) Panel(c *myui.Context, a *App) {
	t.ensure()
	sectionCard(c, "版式", func() {
		layouts := []string{"单页", "两栏", "2×2", "3×3", "自定义"}
		keys := []string{"single", "two", "2x2", "3x3", "custom"}
		selected := indexOf(keys, t.layout)
		segmented(c, &selected, layouts)
		if selected < len(keys) {
			t.layout = keys[selected]
		}
		if t.layout == "custom" {
			grid2(c, func() {
				myui.Column(c).Gap(5).Children(func() {
					label(c, "行")
					numberField(c, &t.rows, 1, 10, 1, "2")
				})
				myui.Column(c).Gap(5).Children(func() {
					label(c, "列")
					numberField(c, &t.cols, 1, 10, 1, "2")
				})
			})
		}
		grid2(c, func() {
			myui.Column(c).Gap(5).Children(func() {
				label(c, "纸张")
				selectField(c, &t.pageSize, paperOptions)
			})
			myui.Column(c).Gap(5).Children(func() {
				label(c, "边距（pt）")
				numberField(c, &t.margin, 0, 200, 4, "36")
			})
		})
		orientation := t.orientation
		segmented(c, &orientation, []string{"纵向", "横向"})
		t.orientation = orientation
		fitKeys := []string{"contain", "cover"}
		sel := indexOf(fitKeys, t.fitMode)
		segmented(c, &sel, []string{"完整显示", "裁切填满"})
		if sel < len(fitKeys) {
			t.fitMode = fitKeys[sel]
		}
	})

	sectionCard(c, "元数据与加密", func() {
		grid2(c, func() {
			myui.Column(c).Gap(5).Children(func() {
				label(c, "标题")
				textField(c, &t.title, "可选")
			})
			myui.Column(c).Gap(5).Children(func() {
				label(c, "作者")
				textField(c, &t.author, "可选")
			})
		})
		myui.Column(c).Gap(5).Children(func() {
			label(c, "打开密码（留空不加密）")
			input := myui.TextInput(c, &t.password).Password().Label("打开密码").
				FillWidth().Height(38).PaddingX(12).Radius(styles(c).RadiusControl).FontSize(13).Placeholder("可选")
			styleFieldInput(c, input, false)
		})
	})
}

func (t *PDFTool) PreviewCaption(a *App) string {
	// The caption is read by a person, so it uses the same wording as the panel
	// instead of the internal request key: "A4 · single" was a leaked enum.
	layout := "单页"
	for i, key := range []string{"single", "two", "2x2", "3x3", "custom"} {
		if key == t.layout {
			layout = []string{"单页", "两栏", "2×2", "3×3", "自定义"}[i]
			break
		}
	}
	return t.pageSize + " · " + layout
}

func (t *PDFTool) Run(a *App) {
	t.ensure()
	rows := a.Session.Snapshot()
	if len(rows) == 0 {
		a.SetStatus("先添加图片")
		return
	}
	a.SetRunning(true)
	cancel := a.beginWholeList()
	rows = a.Session.Snapshot()
	// 裸 goroutine 同样要先把 t.* 快照成局部值，避免与 UI 线程并发读写产生数据竞争。
	layout, pageSize := t.layout, t.pageSize
	customRows, customCols := t.rows, t.cols
	margin, title, author := t.margin, t.title, t.author
	orientation, fitMode, password := t.orientation, t.fitMode, t.password
	go func() {
		defer a.finishRun()
		if cancel.Cancelled() {
			a.endWholeList(cancel, rows)
			return
		}
		paths := make([]string, 0, len(rows))
		for _, r := range rows {
			paths = append(paths, r.Path)
		}
		out, skipped := a.outputPathFor(rows[0], "document", "pdf", 1)
		if skipped {
			if a.endWholeList(cancel, rows) {
				return
			}
			a.SetStatus("PDF 已跳过：目标文件已存在")
			for i := range rows {
				a.Session.SetItemStatus(rows[i], "skipped", "目标已存在", "")
			}
			a.Invalidate()
			return
		}
		res := engine.GeneratePDFContext(cancel, engine.PDFRequest{
			ImagePaths: paths,
			OutputPath: out,
			Layout:     layout,
			CustomRows: int(customRows),
			CustomCols: int(customCols),
			PageSize:   pageSize,
			Margin:     margin,
			Title:      title,
			Author:     author,
			Landscape:  orientation == 1,
			FitMode:    fitMode,
			UserPasswd: password,
		})
		if a.endWholeList(cancel, rows, res) {
			return
		}
		if res.Success {
			a.SetStatus(fmt.Sprintf("PDF 已生成：%d 张图片", res.Count))
			for i := range rows {
				a.Session.SetItemStatus(rows[i], "ok", "", res.OutputPath)
			}
			a.SetProgress(100, a.StatusLine())
		} else {
			a.SetStatus("PDF 生成失败：" + res.Error)
			for i := range rows {
				a.Session.SetItemStatus(rows[i], "failed", res.Error, "")
			}
			a.SetProgress(0, a.StatusLine())
		}
		if res.Success && a.Session.SettingsSnapshot().OpenOutputOnComplete {
			a.openCompletedPath(res.OutputPath)
		}
		a.Invalidate()
	}()
}

// GIFTool handles animated images.
type GIFTool struct {
	action      int
	loop        float64
	speed       float64
	quality     float64
	width       float64
	height      float64
	frameStart  float64
	frameEnd    float64
	maintain    bool
	format      string
	initialised bool
}

func (t *GIFTool) ID() string      { return ToolGIF }
func (t *GIFTool) Name() string    { return "GIF 工具" }
func (t *GIFTool) Icon() *myui.SVG { return lucide.Film() }

var gifActions = []string{"倒放", "变速", "压缩", "缩放", "导出帧"}
var gifActionKeys = []string{"reverse", "change_speed", "compress", "resize", "export_frames"}

func (t *GIFTool) ensure() {
	if t.action < 0 || t.action >= len(gifActions) {
		t.action = 0
	}
	if t.speed == 0 {
		t.speed = 1
	}
	if t.quality == 0 {
		t.quality = 80
	}
	if t.format == "" {
		t.format = "gif"
	}
	if t.width == 0 {
		t.width = 480
	}
	if !t.initialised {
		// Resizing an animation must keep its aspect ratio by default, as the
		// original GIF panel did.
		t.maintain = true
		t.initialised = true
	}
}

func (t *GIFTool) Panel(c *myui.Context, a *App) {
	t.ensure()
	sectionCard(c, "操作", func() {
		selected := t.action
		segmented(c, &selected, gifActions)
		t.action = selected
		switch gifActionKeys[selected] {
		case "change_speed":
			sliderRow(c, "倍速", &t.speed, 0.1, 5, 0.1, "%.1f×")
		case "compress":
			sliderRow(c, "质量", &t.quality, 1, 100, 1, "%.0f")
		case "resize":
			grid2(c, func() {
				myui.Column(c).Gap(5).Children(func() {
					label(c, "宽度")
					numberField(c, &t.width, 16, 4096, 16, "480")
				})
				myui.Column(c).Gap(5).Children(func() {
					label(c, "高度")
					numberField(c, &t.height, 0, 4096, 16, "自动")
				})
			})
			switchField(c, &t.maintain, "锁定比例")
		case "export_frames":
			grid2(c, func() {
				myui.Column(c).Gap(5).Children(func() {
					label(c, "起始帧（从 0 计）")
					numberField(c, &t.frameStart, 0, 9999, 1, "0")
				})
				myui.Column(c).Gap(5).Children(func() {
					label(c, "结束帧（不含，0 = 末尾）")
					numberField(c, &t.frameEnd, 0, 9999, 1, "0")
				})
			})
			if t.frameEnd > 0 && t.frameEnd <= t.frameStart {
				warnAlert(c, "结束帧必须大于起始帧")
			}
		}
		// 导出帧 writes still PNGs, so neither the loop count nor the container
		// applies; showing them promised settings the action cannot honour.
		if gifActionKeys[selected] != "export_frames" {
			myui.Column(c).Gap(5).Children(func() {
				label(c, "循环次数（0 = 无限）")
				numberField(c, &t.loop, 0, 100, 1, "0")
			})
		}
	})

	if gifActionKeys[t.action] == "export_frames" {
		return
	}

	sectionCard(c, "输出格式", func() {
		opts := capabilities.AnimationTargets()
		labels := make([]string, len(opts))
		for i, target := range opts {
			labels[i] = strings.ToUpper(target)
		}
		selected := indexOf(opts, t.format)
		segmented(c, &selected, labels)
		if selected >= 0 && selected < len(opts) {
			t.format = opts[selected]
		}
	})
}

func (t *GIFTool) PreviewCaption(a *App) string {
	t.ensure()
	return gifActions[t.action]
}

func (t *GIFTool) Run(a *App) {
	t.ensure()
	action := gifActionKeys[t.action]
	if action == "export_frames" {
		frameStart, frameEnd := t.frameStart, t.frameEnd
		if frameEnd > 0 && frameEnd <= frameStart {
			a.finishRun()
			a.SetStatus("结束帧必须大于起始帧")
			return
		}
		a.runOutputBatch("导出帧", func(index int, item batch.Item) (string, engine.Result) {
			dir, skipped := a.outputDirectoryFor(item, "_frames")
			if skipped {
				return "", engine.Skip("目标目录已存在")
			}
			return dir, engine.OK()
		}, func(_ context.Context, index int, item batch.Item, dir string) engine.Result {
			return engine.RunGIF(engine.GIFRequest{
				Action:       "export_frames",
				InputPath:    item.Path,
				OutputDir:    dir,
				OutputFormat: "png",
				Quality:      95,
				FrameStart:   int(frameStart),
				FrameEnd:     int(frameEnd),
			})
		})
		return
	}
	format, speed, quality := t.format, t.speed, t.quality
	width, height, maintain, loop := t.width, t.height, t.maintain, t.loop
	a.runOutputBatch("动图处理", func(index int, item batch.Item) (string, engine.Result) {
		ext := extOf(format)
		out, skipped := a.outputPathFor(item, action, ext, index+1)
		if skipped {
			return "", engine.Skip("目标已存在")
		}
		return out, engine.OK()
	}, func(_ context.Context, index int, item batch.Item, out string) engine.Result {
		return engine.RunGIF(engine.GIFRequest{
			Action:       action,
			InputPath:    item.Path,
			OutputPath:   out,
			OutputFormat: format,
			SpeedFactor:  speed,
			Quality:      int(quality),
			Width:        int(width),
			Height:       int(height),
			MaintainAR:   maintain,
			Loop:         int(loop),
			// The panel always owns the loop count: 0 means 无限, so the value must
			// be written even when it is zero.
			ApplyLoop: true,
		})
	})
}

// RichTextTool renders structured content to an image.
type RichTextTool struct {
	initialised bool
	width       float64
	padding     float64
	fontSize    float64
	lineHeight  float64
	theme       string
	fontName    string
	background  myui.Color
	bgHex       string
	textColor   myui.Color
	textHex     string
	headingCol  myui.Color
	headingHex  string
	accentCol   myui.Color
	accentHex   string
	heading     string
	body        string
	quote       string
	listItems   string
	imagePath   string
	imagePct    float64
}

func (t *RichTextTool) ID() string      { return ToolRichText }
func (t *RichTextTool) Name() string    { return "富文本制图" }
func (t *RichTextTool) Icon() *myui.SVG { return lucide.Type() }

func (t *RichTextTool) ensure() {
	if t.initialised {
		return
	}
	t.initialised = true
	if t.width == 0 {
		t.width = 800
	}
	if t.padding == 0 {
		t.padding = 48
	}
	if t.fontSize == 0 {
		t.fontSize = 18
	}
	if t.lineHeight == 0 {
		t.lineHeight = 1.6
	}
	if t.bgHex == "" {
		t.bgHex = "#FFFFFF"
	}
	if t.background == (myui.Color{}) {
		t.background = colorOf(t.bgHex)
	}
	if t.textHex == "" {
		t.textHex = "#1F2937"
	}
	if t.textColor == (myui.Color{}) {
		t.textColor = colorOf(t.textHex)
	}
	if t.headingHex == "" {
		t.headingHex = "#111827"
	}
	if t.headingCol == (myui.Color{}) {
		t.headingCol = colorOf(t.headingHex)
	}
	if t.accentHex == "" {
		t.accentHex = "#007AFF"
	}
	if t.accentCol == (myui.Color{}) {
		t.accentCol = colorOf(t.accentHex)
	}
	if t.heading == "" {
		t.heading = "ImageFlow"
	}
	if t.body == "" {
		t.body = "记录灵感，分享此刻。"
	}
	if t.imagePct == 0 {
		t.imagePct = 100
	}
	if t.fontName == "" {
		t.fontName = defaultFontFamily()
	}
}

func (t *RichTextTool) Panel(c *myui.Context, a *App) {
	t.ensure()
	st := styles(c)
	sectionCard(c, "内容", func() {
		myui.Column(c).Gap(5).Children(func() {
			label(c, "标题")
			textField(c, &t.heading, "标题")
		})
		myui.Column(c).Gap(5).Children(func() {
			label(c, "正文")
			multiLine(c, &t.body, 3, 6, "")
		})
		myui.Column(c).Gap(5).Children(func() {
			label(c, "引用")
			textField(c, &t.quote, "可选")
		})
		myui.Column(c).Gap(5).Children(func() {
			label(c, "列表（每行一项）")
			multiLine(c, &t.listItems, 3, 6, "")
		})
		myui.Column(c).Gap(5).Children(func() {
			label(c, "配图路径（可选）")
			myui.Row(c).FillWidth().Gap(8).AlignItems(myui.Center).Children(func() {
				myui.Box(c).Grow(1).MinWidth(0).Children(func() {
					textField(c, &t.imagePath, "一张图片的完整路径")
				})
				a.ghostButton(c, st, "选择", lucide.Image(), func() {
					a.pickRichTextImage(&t.imagePath)
				})
			})
		})
		if t.imagePath != "" {
			sliderRow(c, "配图宽度", &t.imagePct, 10, 100, 5, "%.0f%%")
		}
	})

	sectionCard(c, "排版", func() {
		grid2(c, func() {
			myui.Column(c).Gap(5).Children(func() {
				label(c, "画布宽度")
				numberField(c, &t.width, 200, 4000, 20, "800")
			})
			myui.Column(c).Gap(5).Children(func() {
				label(c, "内边距")
				numberField(c, &t.padding, 16, 400, 4, "48")
			})
		})
		grid2(c, func() {
			myui.Column(c).Gap(5).Children(func() {
				label(c, "字号")
				numberField(c, &t.fontSize, 10, 120, 1, "18")
			})
			sliderRow(c, "行高", &t.lineHeight, 1.2, 2.5, 0.1, "%.1f")
		})
		myui.Column(c).Gap(5).Children(func() {
			label(c, "字体")
			selectField(c, &t.fontName, fontOptions())
		})
	})

	sectionCard(c, "配色", func() {
		themes := engine.RichTextThemes
		selected := indexOf(engine.RichTextThemeKeys, t.theme)
		segmented(c, &selected, themes)
		if selected >= 0 && selected < len(engine.RichTextThemeKeys) && t.theme != engine.RichTextThemeKeys[selected] {
			t.theme = engine.RichTextThemeKeys[selected]
			t.applyRichTextTheme()
		}
		grid2(c, func() {
			myui.Column(c).Gap(5).Children(func() {
				label(c, "背景")
				before := t.background
				colorField(c, &t.background)
				if t.background != before {
					t.bgHex = hexOf(t.background)
				}
			})
			myui.Column(c).Gap(5).Children(func() {
				label(c, "正文")
				before := t.textColor
				colorField(c, &t.textColor)
				if t.textColor != before {
					t.textHex = hexOf(t.textColor)
				}
			})
		})
		grid2(c, func() {
			myui.Column(c).Gap(5).Children(func() {
				label(c, "标题")
				before := t.headingCol
				colorField(c, &t.headingCol)
				if t.headingCol != before {
					t.headingHex = hexOf(t.headingCol)
				}
			})
			myui.Column(c).Gap(5).Children(func() {
				label(c, "强调")
				before := t.accentCol
				colorField(c, &t.accentCol)
				if t.accentCol != before {
					t.accentHex = hexOf(t.accentCol)
				}
			})
		})
	})
}

func (t *RichTextTool) PreviewCaption(a *App) string { return "富文本画布" }

// fontOptions lists the font families this machine can actually render with, so
// the selector never offers a family the engine would have to substitute.
func fontOptions() []string {
	families := engine.AvailableFonts()
	if len(families) == 0 {
		return []string{"系统默认"}
	}
	return families
}

// defaultFontFamily is the family a fresh panel starts on: the platform's
// preferred installed family, so a Chinese watermark does not open on Arial.
func defaultFontFamily() string {
	return engine.PreferredFont()
}

// richTextPalettes maps the theme keys to the four colour wells the panel edits.
var richTextPalettes = map[string][4]string{
	"":      {"#FFFFFF", "#1F2937", "#111827", "#007AFF"},
	"dark":  {"#111827", "#E5E7EB", "#F9FAFB", "#60A5FA"},
	"sepia": {"#FDF6E3", "#57534E", "#3C3836", "#C17E3F"},
	"ocean": {"#ECF6FF", "#1E3A5F", "#0C284A", "#0284C7"},
}

// applyRichTextTheme copies the theme's colours into the editable fields, so the
// colour wells and the rendered canvas never disagree.
func (t *RichTextTool) applyRichTextTheme() {
	pal, ok := richTextPalettes[t.theme]
	if !ok {
		return
	}
	t.bgHex, t.textHex, t.headingHex, t.accentHex = pal[0], pal[1], pal[2], pal[3]
	t.background = colorOf(t.bgHex)
	t.textColor = colorOf(t.textHex)
	t.headingCol = colorOf(t.headingHex)
	t.accentCol = colorOf(t.accentHex)
}

func (t *RichTextTool) blocks() []engine.RichTextBlock {
	out := []engine.RichTextBlock{{Type: "heading", Level: 1, Text: t.heading}}
	if t.body != "" {
		out = append(out, engine.RichTextBlock{Type: "paragraph", Text: t.body})
	}
	if t.quote != "" {
		out = append(out, engine.RichTextBlock{Type: "quote", Text: t.quote})
	}
	if t.listItems != "" {
		var items []string
		for _, line := range strings.Split(t.listItems, "\n") {
			line = strings.TrimSpace(line)
			if line != "" {
				items = append(items, line)
			}
		}
		if len(items) > 0 {
			out = append(out, engine.RichTextBlock{Type: "list", Items: items})
		}
	}
	if strings.TrimSpace(t.imagePath) != "" {
		out = append(out, engine.RichTextBlock{
			Type: "image", Path: strings.TrimSpace(t.imagePath), WidthPercent: t.imagePct,
		})
	}
	return out
}

func (t *RichTextTool) Run(a *App) {
	t.ensure()
	request := t.request()
	if a.Session.Len() == 0 {
		if a.outputFolder() == "" {
			a.finishRun()
			a.SetStatus("先选择输出文件夹")
			return
		}
		cancel := a.beginWholeList()
		anchor := filepath.Join(a.outputFolder(), "richtext.png")
		out, skipped := a.outputPathFor(batch.Item{Path: anchor}, "richtext", "png", 1)
		if skipped {
			if !a.endWholeList(cancel, nil) {
				a.SetStatus("制图已跳过：目标文件已存在")
			}
			a.finishRun()
			return
		}
		request.OutputPath = out
		go func() {
			defer a.finishRun()
			if cancel.Cancelled() {
				a.endWholeList(cancel, nil)
				return
			}
			result := engine.RenderRichTextContext(cancel, request)
			if a.endWholeList(cancel, nil, result) {
				return
			}
			a.reportWhole(result, ToolRichText)
			if result.Success && a.Session.SettingsSnapshot().OpenOutputOnComplete {
				a.openCompletedPath(result.OutputPath)
			}
		}()
		return
	}
	a.runOutputBatch("制图", func(index int, item batch.Item) (string, engine.Result) {
		out, skipped := a.outputPathFor(item, "richtext", "png", index+1)
		if skipped {
			return "", engine.Skip("目标已存在")
		}
		return out, engine.OK()
	}, func(ctx context.Context, _ int, _ batch.Item, out string) engine.Result {
		req := request
		req.OutputPath = out
		return engine.RenderRichTextContext(ctx, req)
	})
}

func (t *RichTextTool) request() engine.RichTextRequest {
	return engine.RichTextRequest{
		Blocks: t.blocks(), OutputFormat: "png", Width: int(t.width), Padding: int(t.padding),
		Background: t.bgHex, TextColor: t.textHex, HeadingColor: t.headingHex, AccentColor: t.accentHex,
		FontSize: int(t.fontSize), LineHeight: t.lineHeight, Family: t.fontName,
	}
}

func qualityRelevant(a *App, format string) bool {
	if format != "" {
		return engine.QualityFormats[format]
	}
	rows := a.Session.Snapshot()
	if len(rows) == 0 {
		return true
	}
	for _, row := range rows {
		if engine.QualityFormats[strings.TrimPrefix(strings.ToLower(filepath.Ext(row.Path)), ".")] {
			return true
		}
	}
	return false
}
