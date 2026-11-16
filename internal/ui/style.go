package ui

import (
	myui "github.com/egoist/mygo/ui"
)

// style holds ImageFlow's visual contract: the single brand blue, the neutral
// surface ladder, radii and springs, taken from docs/DESIGN.md of the original
// project so the Go build keeps the same feel.
type style struct {
	// Dark reports the current appearance.
	Dark bool

	// Brand: the one accent colour, used for selection, primary actions and
	// progress. PrimaryGradientTo is the darker end of the button gradient.
	Brand            myui.Color
	BrandDeep        myui.Color
	BrandHover       myui.Color
	BrandSoft        myui.Color
	BrandGlow        myui.Color
	BrandGradientTo  myui.Color
	BrandGradientHov myui.Color

	// Shell and surfaces, in the order content sits on them.
	Shell      myui.Color
	Sidebar    myui.Color
	Card       myui.Color
	CardHover  myui.Color
	Recessed   myui.Color
	Divider    myui.Color
	DividerTop myui.Color

	// Text ladder.
	Text    myui.Color
	TextSub myui.Color
	TextDim myui.Color

	// Semantic states.
	Success myui.Color
	Warning myui.Color
	Danger  myui.Color

	// Radii, from the design ladder.
	RadiusCard    float32 // 16
	RadiusPanel   float32 // 12
	RadiusControl float32 // 10
	RadiusPill    float32 // 7
}

func newStyle(dark bool) style {
	if dark {
		return style{
			Dark:             true,
			Brand:            myui.RGB(10, 132, 255),
			BrandDeep:        myui.RGB(0, 90, 220),
			BrandHover:       myui.RGB(64, 156, 255),
			BrandSoft:        myui.RGBA(10, 132, 255, 0.16),
			BrandGlow:        myui.RGBA(10, 132, 255, 0.35),
			BrandGradientTo:  myui.RGB(88, 86, 214),
			BrandGradientHov: myui.RGB(84, 130, 255),

			Shell:      myui.RGB(28, 28, 30),
			Sidebar:    myui.RGB(38, 38, 40),
			Card:       myui.RGB(44, 44, 46),
			CardHover:  myui.RGB(54, 54, 56),
			Recessed:   myui.RGB(34, 34, 36),
			Divider:    myui.RGB(60, 60, 63),
			DividerTop: myui.RGB(52, 52, 55),

			Text:    myui.RGB(245, 245, 247),
			TextSub: myui.RGB(174, 174, 178),
			TextDim: myui.RGB(124, 124, 128),

			Success: myui.RGB(48, 209, 88),
			Warning: myui.RGB(255, 159, 10),
			Danger:  myui.RGB(255, 69, 58),

			// Radii sit one step inside the window's own corner radius, so a card
			// never fights the frame around it.
			RadiusCard:    16,
			RadiusPanel:   12,
			RadiusControl: 10,
			RadiusPill:    7,
		}
	}
	return style{
		Brand:            myui.RGB(0, 122, 255),
		BrandDeep:        myui.RGB(0, 85, 255),
		BrandHover:       myui.RGB(0, 68, 221),
		BrandSoft:        myui.RGBA(0, 122, 255, 0.10),
		BrandGlow:        myui.RGBA(0, 122, 255, 0.25),
		BrandGradientTo:  myui.RGB(0, 85, 255),
		BrandGradientHov: myui.RGB(88, 86, 214),

		Shell:      myui.RGB(245, 245, 247),
		Sidebar:    myui.RGB(255, 255, 255),
		Card:       myui.RGB(255, 255, 255),
		CardHover:  myui.RGB(250, 250, 252),
		Recessed:   myui.RGB(246, 247, 249),
		Divider:    myui.RGB(231, 232, 236),
		DividerTop: myui.RGB(238, 239, 242),

		Text:    myui.RGB(17, 24, 39),
		TextSub: myui.RGB(75, 85, 99),
		TextDim: myui.RGB(156, 163, 175),

		Success: myui.RGB(34, 197, 94),
		Warning: myui.RGB(245, 158, 11),
		Danger:  myui.RGB(239, 68, 68),

		RadiusCard:    16,
		RadiusPanel:   12,
		RadiusControl: 10,
		RadiusPill:    7,
	}
}

// styles returns the design tokens for the current appearance.
func styles(c *myui.Context) style {
	return newStyle(c.Theme().Dark)
}

// featureTint is the per-tool icon colour, mirroring FEATURES[].color in the
// original constants so each tool keeps its identity.
//
// The ladder carries one entry per tool. It used to hold ten, so the last two
// tools fell back to the brand blue and their rail plates were identical — which
// is the one thing a per-tool tint must not be.
func featureTint(index int, st style) myui.Color {
	palette := []myui.Color{
		myui.RGB(59, 130, 246),  // blue-500   格式转换
		myui.RGB(6, 182, 212),   // cyan-500   尺寸调整
		myui.RGB(168, 85, 247),  // purple-500 图片裁剪
		myui.RGB(139, 92, 246),  // violet-500 宫格切图
		myui.RGB(20, 184, 166),  // teal-500   图片布局
		myui.RGB(236, 72, 153),  // pink-500   长图拼接
		myui.RGB(249, 115, 22),  // orange-500 批量水印
		myui.RGB(217, 119, 6),   // amber-600  色彩提取
		myui.RGB(239, 68, 68),   // red-500    图标生成
		myui.RGB(99, 102, 241),  // indigo-500 转 PDF
		myui.RGB(16, 185, 129),  // emerald-500 GIF 工具
		myui.RGB(100, 116, 139), // slate-500  富文本制图
		myui.RGB(34, 197, 94),   // 图片压缩
		myui.RGB(244, 63, 94),   // rose-500   图片拼图
	}
	if index < 0 || index >= len(palette) {
		return st.Brand
	}
	return palette[index]
}

// featureTintPlate is the tinted plate behind a tool's icon.
//
// The original sidebar gave every row a soft coloured square, and that plate is
// what makes the rail read as a tool list rather than a text menu. Alpha is
// per theme: a 12% wash on the white sidebar, 22% on the dark one, where the
// same alpha would vanish.
func featureTintPlate(index int, st style) myui.Color {
	tint := featureTint(index, st)
	alpha := uint8(38) // ~15%: a visible tile on the white sidebar
	if st.Dark {
		alpha = 64 // ~25%: the same wash would vanish on 262628
	}
	return myui.Color{R: tint.R, G: tint.G, B: tint.B, A: alpha}
}

// featurePlateSize is the square the rail's icon plate occupies.
const featurePlateSize = 32
