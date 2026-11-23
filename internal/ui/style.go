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
			Dark: true,
			// The dark theme's accent is the same periwinkle family, and each value
			// holds the luminance its predecessor had: #0A84FF, #005ADC and
			// #409CFF. Only the hue moved (211 -> 232), so every contrast pair the
			// shell already relied on keeps its ratio.
			//
			// Note the one pre-existing gap, unchanged by this switch: accent text
			// on a dark card is 3.85:1, under the 4.5 a body label wants. It is the
			// same 3.82 the old #0A84FF gave, tracked in docs/DESIGN.md §7 rather
			// than introduced here.
			Brand:           myui.RGB(0x69, 0x7C, 0xF5),
			BrandDeep:       myui.RGB(0x34, 0x4E, 0xF2),
			BrandHover:      myui.RGB(0x83, 0x92, 0xF7),
			BrandSoft:       myui.RGBA(0x69, 0x7C, 0xF5, 0.16),
			BrandGlow:       myui.RGBA(0x69, 0x7C, 0xF5, 0.35),
			BrandGradientTo: myui.RGB(0x58, 0x56, 0xD6),
			// BrandGradientHov is a hover end, so it can be darker than the resting
			// one: #6A66DA keeps a white 13.5 px label at 4.64:1, where the
			// previous #5482FF gave 3.7.
			BrandGradientHov: myui.RGB(0x6A, 0x66, 0xDA),

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
		Brand:      myui.RGB(0x35, 0x5C, 0xE8),
		BrandDeep:  myui.RGB(0x25, 0x4B, 0xD6),
		BrandHover: myui.RGB(0x2A, 0x4E, 0xCF),
		BrandSoft:  myui.RGBA(0x35, 0x5C, 0xE8, 0.10),
		BrandGlow:  myui.RGBA(0x35, 0x5C, 0xE8, 0.25),
		// GradientTo stays a *blue*, not the dark theme's violet: docs/DESIGN.md §1
		// keeps one emphasis colour, so a filled control cannot drift to a second
		// hue half-way along its gradient. GradientHov is the one violet step, so
		// hover still reads as a change rather than as more of the same.
		BrandGradientTo:  myui.RGB(0x2A, 0x4E, 0xCF),
		BrandGradientHov: myui.RGB(0x58, 0x56, 0xD6),

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
		myui.RGB(59, 130, 246),  // blue-500     格式转换
		myui.RGB(6, 182, 212),   // cyan-500     尺寸调整
		myui.RGB(168, 85, 247),  // purple-500   图片裁剪
		myui.RGB(139, 92, 246),  // violet-500   宫格切图
		myui.RGB(20, 184, 166),  // teal-500     图片布局
		myui.RGB(244, 63, 94),   // rose-500     图片拼图
		myui.RGB(236, 72, 153),  // pink-500     长图拼接
		myui.RGB(249, 115, 22),  // orange-500   批量水印
		myui.RGB(217, 119, 6),   // amber-600    色彩提取
		myui.RGB(239, 68, 68),   // red-500      图标生成
		myui.RGB(99, 102, 241),  // indigo-500   转 PDF
		myui.RGB(16, 185, 129),  // emerald-500  GIF 工具
		myui.RGB(100, 116, 139), // slate-500    富文本制图
		myui.RGB(34, 197, 94),   // green-500    图片压缩
		myui.RGB(14, 165, 233),  // sky-500      基础编辑
		myui.RGB(180, 83, 9),    // amber-700    自由布局
		myui.RGB(190, 24, 93),   // pink-700     图片标注
		myui.RGB(13, 148, 136),  // teal-600     截图美化
		myui.RGB(147, 51, 234),  // purple-600   专业调色
		myui.RGB(2, 132, 199),   // sky-600      滤镜效果
		myui.RGB(75, 85, 99),    // gray-600     图片对比
		myui.RGB(120, 53, 15),   // amber-900    图像分析
		myui.RGB(22, 163, 74),   // green-600    像素画 / 点阵
		myui.RGB(219, 39, 119),  // pink-600     表情包制作
		myui.RGB(79, 70, 229),   // indigo-600   AI 高清放大
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
