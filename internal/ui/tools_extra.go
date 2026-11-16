package ui

// tools_extra.go adds the two tools the reference toolbox had and this one did
// not: a crop tool with ratio presets, and an icon bundle generator.

import (
	"fmt"

	myui "github.com/egoist/mygo/ui"
	"github.com/lobehub/lobe-mygo/pkg/lucide"

	"github.com/lllll081926i/imageflow-go/internal/batch"
	"github.com/lllll081926i/imageflow-go/internal/engine"
)

// CropTool cuts every input to one aspect ratio.
type CropTool struct {
	ratio      int
	customText string
	anchor     int
	scale      int
	outWidth   float64
	outHeight  float64
	radius     float64
	background myui.Color
	bgHex      string
	format     string
	quality    float64
}

func (t *CropTool) Name() string    { return "图片裁剪" }
func (t *CropTool) Icon() *myui.SVG { return lucide.Crop() }

var cropAnchorLabels = []string{"↖", "↑", "↗", "←", "·", "→", "↙", "↓", "↘"}

func (t *CropTool) ensure() {
	if t.customText == "" {
		t.customText = "5:4"
	}
	if t.outWidth == 0 {
		t.outWidth = 1024
	}
	if t.outHeight == 0 {
		t.outHeight = 1024
	}
	if t.quality == 0 {
		t.quality = 95
	}
	if t.format == "" {
		t.format = "png"
	}
	if t.bgHex == "" {
		t.bgHex = "#FFFFFF"
		t.background = colorOf(t.bgHex)
	}
}

// ratio is the ratio the panel currently resolves to.
func (t *CropTool) ratioValue() engine.CropRatio {
	if t.ratio >= 0 && t.ratio < len(engine.CropRatios) {
		return engine.CropRatios[t.ratio]
	}
	if parsed, ok := engine.ParseCropRatio(t.customText); ok {
		return parsed
	}
	return engine.CropRatios[0]
}

func (t *CropTool) Panel(c *myui.Context, a *App) {
	t.ensure()
	sectionCard(c, "裁剪比例", func() {
		labels := make([]string, 0, len(engine.CropRatios)+1)
		for _, ratio := range engine.CropRatios {
			labels = append(labels, ratio.Label())
		}
		labels = append(labels, "自定义")
		selected := t.ratio
		if selected < 0 || selected >= len(labels) {
			selected = 0
		}
		// Two rows of pills rather than one: eleven options in a single row
		// squeeze the labels to two characters each.
		half := (len(labels) + 1) / 2
		first := selected
		segmented(c, &first, labels[:half])
		if selected < half {
			t.ratio = first
			selected = first
		}
		if len(labels) > half {
			second := selected - half
			if second < 0 {
				second = 0
			}
			segmented(c, &second, labels[half:])
			if selected >= half {
				t.ratio = second + half
			}
		}
		if t.ratio < 0 || t.ratio >= len(labels) {
			t.ratio = 0
		}
		if t.ratio == len(engine.CropRatios) {
			_, valid := engine.ParseCropRatio(t.customText)
			myui.Column(c).Gap(5).Children(func() {
				label(c, "自定义比例（宽:高）")
				textFieldState(c, &t.customText, "5:4", !valid)
			})
			if !valid {
				warnAlert(c, "比例格式无效，使用 1:1")
			}
		}
	})

	sectionCard(c, "取景位置", func() {
		myui.Row(c).FillWidth().Gap(6).Children(func() {
			for i, glyph := range cropAnchorLabels {
				index := i
				active := t.anchor == index
				btn := myui.ButtonBase(c).Grow(1).Height(30).Radius(styles(c).RadiusPill).
					Justify(myui.Center).AlignItems(myui.Center).
					Cursor(myui.CursorPointer).Label(engine.CropAnchors[index]).
					FocusRingColor(styles(c).Brand).FocusRingOffset(1).
					Transition(motion(springTap))
				st := styles(c)
				switch {
				case active && btn.Pressed():
					btn.Gradient(st.BrandDeep, st.BrandGradientTo, 90).Scale(0.96, 0.96)
				case active:
					btn.Gradient(st.Brand, st.BrandGradientTo, 90)
				case btn.Pressed():
					btn.Border(1, st.BrandDeep).Background(st.DividerTop).Scale(0.96, 0.96)
				case btn.Hovered():
					btn.Border(1, st.Brand).Background(st.Card)
				default:
					btn.Border(1, st.Divider).Background(st.Recessed)
				}
				btn.Children(func() {
					col := st.TextSub
					if active {
						col = myui.RGB(255, 255, 255)
					}
					myui.Text(c, glyph).FontSize(12).TextColor(col)
				})
				if btn.Clicked() {
					t.anchor = index
				}
			}
		})
	})

	sectionCard(c, "输出尺寸", func() {
		fit := t.scale
		segmented(c, &fit, []string{"保持裁剪像素", "缩放到固定尺寸"})
		t.scale = fit
		if t.scale == 1 {
			grid2(c, func() {
				myui.Column(c).Gap(5).Children(func() {
					label(c, "宽")
					numberField(c, &t.outWidth, 8, 16000, 8, "1024")
				})
				myui.Column(c).Gap(5).Children(func() {
					label(c, "高")
					numberField(c, &t.outHeight, 8, 16000, 8, "1024")
				})
			})
		}
		sliderRow(c, "圆角（占短边 %）", &t.radius, 0, 50, 1, "%.0f%%")
		if t.radius > 0 {
			myui.Column(c).Gap(5).Children(func() {
				label(c, "圆角外颜色")
				before := t.background
				colorField(c, &t.background)
				if t.background != before {
					t.bgHex = hexOf(t.background)
				}
			})
		}
		opts := []string{"png", "jpg", "webp"}
		selected := indexOf(opts, t.format)
		segmented(c, &selected, opts)
		if selected >= 0 && selected < len(opts) {
			t.format = opts[selected]
		}
		if t.format != "png" {
			sliderRow(c, "质量", &t.quality, 1, 100, 1, "%.0f")
		}
	})
}

func (t *CropTool) PreviewCaption(a *App) string {
	return t.ratioValue().Label() + " 裁剪"
}

func (t *CropTool) Run(a *App) {
	t.ensure()
	ratio := t.ratioValue()
	anchor := "c"
	if t.anchor >= 0 && t.anchor < len(engine.CropAnchors) {
		anchor = engine.CropAnchors[t.anchor]
	}
	scale, w, h, radius, bg, format, quality := t.scale, int(t.outWidth), int(t.outHeight), int(t.radius), t.bgHex, t.format, int(t.quality)
	a.runBatch("裁剪", func(index int, item batch.Item) engine.Result {
		ext := extOf(format)
		out, skipped := a.outputPathFor(item, "crop", ext, index+1)
		if skipped {
			return engine.Skip("目标已存在")
		}
		req := engine.CropRequest{
			InputPath: item.Path, OutputPath: out,
			RatioW: ratio.W, RatioH: ratio.H, Anchor: anchor,
			Radius: radius, Background: bg, OutputFormat: format, Quality: quality,
		}
		if scale == 1 {
			req.ScaledWidth, req.ScaledHeight = w, h
		}
		return engine.CropBox(req)
	})
}

// IconTool generates an application icon bundle.
type IconTool struct {
	padding     float64
	radius      float64
	background  myui.Color
	bgHex       string
	withICO     bool
	withFav     bool
	withSVG     bool
	initialised bool
}

func (t *IconTool) Name() string    { return "图标生成" }
func (t *IconTool) Icon() *myui.SVG { return lucide.AppWindow() }

func (t *IconTool) ensure() {
	// An application icon wants a transparent background by default: a white
	// square shows as a white plate on a dock or taskbar. A chosen colour is what
	// the 填充色 field is for.
	if !t.initialised {
		t.bgHex = ""
		t.withICO = true
		t.withFav = true
		t.withSVG = true
		t.initialised = true
	}
}

func (t *IconTool) Panel(c *myui.Context, a *App) {
	t.ensure()
	st := styles(c)
	sectionCard(c, "画布", func() {
		sliderRow(c, "内边距", &t.padding, 0, 30, 1, "%.0f%%")
		sliderRow(c, "圆角", &t.radius, 0, 50, 1, "%.0f%%")
		// A fill only matters once the canvas has a margin or a rounded corner to
		// fill; with neither, the switch would offer a colour that changes nothing.
		if t.padding > 0 || t.radius > 0 {
			if t.bgHex == "" {
				// A blank background has no colour to show. Rendering the field anyway
				// painted #000000, which read as black rather than as transparent.
				myui.Row(c).FillWidth().AlignItems(myui.Center).Gap(8).Children(func() {
					myui.Text(c, "边距填充色").FontSize(12).TextColor(st.TextSub)
					myui.Spacer(c).Grow(1)
					statusPill(c, "透明", st.TextDim)
				})
				myui.Row(c).FillWidth().Children(func() {
					a.ghostButton(c, st, "指定填充色", lucide.Palette(), func() {
						t.bgHex = "#FFFFFF"
						t.background = colorOf(t.bgHex)
					})
				})
			} else {
				myui.Column(c).Gap(5).Children(func() {
					label(c, "边距填充色")
					before := t.background
					colorField(c, &t.background)
					if t.background != before {
						t.bgHex = hexOf(t.background)
					}
				})
				myui.Row(c).FillWidth().Children(func() {
					a.ghostButton(c, st, "用透明背景", lucide.Eraser(), func() {
						t.bgHex = ""
						t.background = myui.Color{}
					})
				})
			}
		}
	})
	sectionCard(c, "输出文件", func() {
		// The ladder wraps four to a row: sixteen pills in one row run off the
		// panel, which is what the first version did.
		rows := (len(engine.IconLadder) + 3) / 4
		for r := 0; r < rows; r++ {
			myui.Row(c).FillWidth().Gap(5).Children(func() {
				for i := r * 4; i < r*4+4 && i < len(engine.IconLadder); i++ {
					cell := myui.Row(c).Grow(1).Height(24).Radius(styles(c).RadiusPill).
						Justify(myui.Center).AlignItems(myui.Center).
						Background(styles(c).Recessed).Children(func() {
						myui.Text(c, fmt.Sprintf("%d", engine.IconLadder[i])).
							FontSize(10.5).FontWeight(500).
							TextColor(styles(c).TextDim).NoWrap()
					})
					_ = cell
				}
			})
		}
		switchRow(c, styles(c), "icon.ico", &t.withICO)
		switchRow(c, styles(c), "favicon.ico", &t.withFav)
		switchRow(c, styles(c), "icon.svg", &t.withSVG)
	})
}

func (t *IconTool) PreviewCaption(a *App) string { return "图标源图" }

func (t *IconTool) Run(a *App) {
	t.ensure()
	padding, radius, bg := int(t.padding), int(t.radius), t.bgHex
	withICO, withFav, withSVG := t.withICO, t.withFav, t.withSVG
	a.runBatch("图标", func(index int, item batch.Item) engine.Result {
		dir, skipped := a.outputDirectoryFor(item, "_icons")
		if skipped {
			return engine.Skip("目标已存在")
		}
		return engine.BuildIconSet(engine.IconRequest{
			InputPath:   item.Path,
			OutputDir:   dir,
			Padding:     padding,
			Radius:      radius,
			Background:  bg,
			Sizes:       engine.IconLadder,
			WithICO:     withICO,
			WithFavicon: withFav,
			WithSVG:     withSVG,
		})
	})
}
