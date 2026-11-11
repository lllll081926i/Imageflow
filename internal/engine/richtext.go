package engine

import (
	"errors"
	"fmt"
	"image"
	"image/color"
	"image/draw"
	"strconv"
	"strings"

	"github.com/disintegration/imaging"

	"github.com/lllll081926i/imageflow-go/internal/codec"
	"github.com/lllll081926i/imageflow-go/internal/output"
)

// RichTextBlock is one content block of the rich-text tool.
type RichTextBlock struct {
	Type         string   `json:"type"`
	Text         string   `json:"text"`
	Level        int      `json:"level"`
	Items        []string `json:"items"`
	Ordered      bool     `json:"ordered"`
	Path         string   `json:"path"`
	WidthPercent float64  `json:"width_percent"`
}

// RichTextRequest carries the canvas style and the blocks.
type RichTextRequest struct {
	Blocks       []RichTextBlock `json:"blocks"`
	OutputPath   string          `json:"output_path"`
	OutputFormat string          `json:"output_format"`
	Width        int             `json:"width"`
	Padding      int             `json:"padding"`
	Background   string          `json:"background"`
	TextColor    string          `json:"text_color"`
	HeadingColor string          `json:"heading_color"`
	AccentColor  string          `json:"accent_color"`
	FontSize     int             `json:"font_size"`
	LineHeight   float64         `json:"line_height"`
	Family       string          `json:"font_name"`
	Theme        string          `json:"theme"`
}

var headingScales = map[int]float64{1: 1.9, 2: 1.5, 3: 1.25}

// The four built-in colour themes of the rich-text canvas. Empty keeps the
// caller's explicit colours, which is what the UI sends.
const (
	RichTextThemeLight = ""
	RichTextThemeDark  = "dark"
	RichTextThemeSepia = "sepia"
	RichTextThemeOcean = "ocean"
)

// RichTextThemes lists the theme names the UI offers, in order.
var RichTextThemes = []string{"浅色", "深色", "暖褐", "海洋"}

// RichTextThemeKeys maps RichTextThemes to the Request values.
var RichTextThemeKeys = []string{RichTextThemeLight, RichTextThemeDark, RichTextThemeSepia, RichTextThemeOcean}

// RenderRichTextImage lays the blocks out and returns the canvas.
func RenderRichTextImage(req RichTextRequest) (image.Image, error) {
	// The zero value must mean "use the default", so the default is applied before
	// the clamp: clamping first turned 0 into the lower bound and made every
	// default below unreachable, so a zero-value request produced a 200 px canvas
	// with a 16 px margin instead of the documented 800 / 48.
	width := req.Width
	if width == 0 {
		width = 800
	}
	width = clampInt(width, 200, 4000)
	padding := req.Padding
	if padding == 0 {
		padding = 48
	}
	padding = clampInt(padding, 16, 400)
	base := req.FontSize
	if base == 0 {
		base = 18
	}
	base = clampInt(base, 10, 120)
	lineHeight := req.LineHeight
	if lineHeight < 1.2 {
		lineHeight = 1.2
	}
	if lineHeight > 2.5 {
		lineHeight = 2.5
	}
	family := req.Family
	bg := hexToColor(req.Background, 255)
	if strings.TrimSpace(req.Background) == "" {
		bg = color.NRGBA{R: 255, G: 255, B: 255, A: 255}
	}
	textColor := hexToColor(req.TextColor, 255)
	if strings.TrimSpace(req.TextColor) == "" {
		textColor = color.NRGBA{R: 31, G: 41, B: 55, A: 255}
	}
	headingColor := hexToColor(req.HeadingColor, 255)
	if strings.TrimSpace(req.HeadingColor) == "" {
		headingColor = color.NRGBA{R: 17, G: 24, B: 39, A: 255}
	}
	accent := hexToColor(req.AccentColor, 255)
	if strings.TrimSpace(req.AccentColor) == "" {
		accent = color.NRGBA{R: 0, G: 122, B: 255, A: 255}
	}

	switch strings.ToLower(strings.TrimSpace(req.Theme)) {
	case RichTextThemeDark:
		bg = color.NRGBA{R: 17, G: 24, B: 39, A: 255}
		textColor = color.NRGBA{R: 229, G: 231, B: 235, A: 255}
		headingColor = color.NRGBA{R: 249, G: 250, B: 251, A: 255}
		accent = color.NRGBA{R: 96, G: 165, B: 250, A: 255}
	case RichTextThemeSepia:
		bg = color.NRGBA{R: 253, G: 246, B: 227, A: 255}
		textColor = color.NRGBA{R: 87, G: 83, B: 78, A: 255}
		headingColor = color.NRGBA{R: 60, G: 56, B: 54, A: 255}
		accent = color.NRGBA{R: 193, G: 126, B: 63, A: 255}
	case RichTextThemeOcean:
		bg = color.NRGBA{R: 236, G: 246, B: 255, A: 255}
		textColor = color.NRGBA{R: 30, G: 58, B: 95, A: 255}
		headingColor = color.NRGBA{R: 12, G: 40, B: 74, A: 255}
		accent = color.NRGBA{R: 2, G: 132, B: 199, A: 255}
	}

	contentWidth := float32(width - padding*2)
	type line struct {
		text   string
		size   int
		color  color.NRGBA
		bold   bool
		gap    float32
		bar    bool
		indent float32
		img    image.Image
	}
	var lines []line
	imagePixels := int64(0)

	addWrapped := func(text string, size int, col color.NRGBA, bold bool, indent float32, bar bool) error {
		face, err := FontFace(family, size, 72, bold)
		if err != nil {
			return err
		}
		wrapped := WrapText(text, face, contentWidth-indent)
		for _, w := range wrapped {
			lines = append(lines, line{text: w, size: size, color: col, bold: bold, indent: indent, bar: bar})
		}
		return nil
	}

	for _, block := range req.Blocks {
		switch strings.ToLower(strings.TrimSpace(block.Type)) {
		case "heading", "":
			level := clampInt(block.Level, 1, 3)
			if level == 0 {
				level = 1
			}
			size := int(float64(base) * headingScales[level])
			lines = append(lines, line{gap: float32(base) * 0.6})
			if err := addWrapped(block.Text, size, headingColor, true, 0, false); err != nil {
				return nil, err
			}
		case "paragraph":
			lines = append(lines, line{gap: float32(base) * 0.5})
			if err := addWrapped(block.Text, base, textColor, false, 0, false); err != nil {
				return nil, err
			}
		case "list":
			for i, item := range block.Items {
				marker := "• "
				indent := float32(base) * 1.4
				if block.Ordered {
					marker = strconv.Itoa(i+1) + ". "
				}
				face, err := FontFace(family, base, 72, false)
				if err != nil {
					return nil, err
				}
				wrapped := WrapText(item, face, contentWidth-indent)
				for j, w := range wrapped {
					prefix := ""
					if j == 0 {
						prefix = marker
					}
					lines = append(lines, line{
						text: prefix + w, size: base, color: textColor,
						indent: indent, bar: false,
					})
				}
			}
		case "quote":
			lines = append(lines, line{gap: float32(base) * 0.4})
			if err := addWrapped(block.Text, int(float64(base)*1.05), textColor, false, float32(base)*1.2, true); err != nil {
				return nil, err
			}
		case "image":
			if strings.TrimSpace(block.Path) == "" {
				continue
			}
			opened, err := codec.OpenSRGB(block.Path)
			if err != nil {
				return nil, fmt.Errorf("富文本图片读取失败：%w", err)
			}
			pct := block.WidthPercent
			if pct < 10 || pct > 100 {
				pct = 100
			}
			targetW := int(float64(contentWidth) * pct / 100)
			if targetW < 1 {
				continue
			}
			targetH := maxInt(1, int(float64(opened.Height)*float64(targetW)/float64(opened.Width)))
			if err := output.CheckSize(targetW, targetH); err != nil {
				return nil, err
			}
			imagePixels += int64(targetW) * int64(targetH)
			if imagePixels > output.MaxOutputPixels {
				return nil, fmt.Errorf("富文本图片累计面积超过 %d 像素限制", output.MaxOutputPixels)
			}
			// Scale by the requested fraction of the content width, keeping the
			// source aspect ratio; a zero height would keep the source size.
			thumb := imaging.Resize(opened.Image, targetW, 0, imaging.Lanczos)
			drawnW := thumb.Bounds().Dx()
			drawnH := thumb.Bounds().Dy()
			lines = append(lines, line{gap: float32(base) * 0.5})
			lines = append(lines, line{text: "\x00image\x00", size: drawnH, indent: float32(drawnW), img: thumb})
		}
	}
	if len(lines) == 0 {
		return nil, errors.New("内容为空")
	}
	// A block's leading gap separates it from what came before; before the first
	// block there is nothing to separate from, and keeping it made the top margin
	// visibly deeper than the padding on the other three sides.
	lines[0].gap = 0

	// Measure: a dry pass over line heights, with images carrying their height.
	// totalHeight already carries the padding on both sides, so the canvas is
	// exactly that: adding another padding here left a visibly deeper bottom
	// margin than the top one.
	totalHeight := float32(padding * 2)
	for _, l := range lines {
		if strings.HasPrefix(l.text, "\x00image\x00") {
			totalHeight += float32(l.size) + l.gap
			continue
		}
		size := l.size
		if size == 0 {
			size = base
		}
		totalHeight += float32(float64(size) * lineHeight)
		totalHeight += l.gap
	}
	height := int(totalHeight)
	if height < 1 {
		height = 1
	}
	// A long document can outgrow the shared canvas budget; reject it before the
	// allocation instead of letting the runtime try a multi-gigabyte image.
	if err := output.CheckSize(width, height); err != nil {
		return nil, err
	}

	canvas := image.NewNRGBA(image.Rect(0, 0, width, height))
	draw.Draw(canvas, canvas.Bounds(), &image.Uniform{bg}, image.Point{}, draw.Src)

	cursorY := float32(padding)
	for _, l := range lines {
		cursorY += l.gap
		if strings.HasPrefix(l.text, "\x00image\x00") {
			draw.Draw(canvas, image.Rect(padding, int(cursorY), padding+l.img.Bounds().Dx(), int(cursorY)+l.img.Bounds().Dy()), l.img, l.img.Bounds().Min, draw.Over)
			cursorY += float32(l.img.Bounds().Dy())
			continue
		}
		size := l.size
		if size == 0 {
			size = base
		}
		baseline := cursorY + float32(float64(size)*0.82)
		if l.bar {
			draw.Draw(canvas, image.Rect(padding, int(cursorY), padding+4, int(cursorY)+int(float64(size)*lineHeight)), &image.Uniform{accent}, image.Point{}, draw.Src)
		}
		_, err := DrawText(canvas, l.text, float32(padding)+l.indent, baseline, TextOptions{
			Family: family, Size: size, Color: l.color, Bold: l.bold,
		})
		if err != nil {
			return nil, err
		}
		cursorY += float32(float64(size) * lineHeight)
	}
	return canvas, nil
}
