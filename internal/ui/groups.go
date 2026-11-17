package ui

// groups.go defines where each page sits in the navigation, what it is for, and
// how a search finds it.
//
// The tool list itself is flat: a tool's position in App.Tools is its identity
// index, and the saved sidebar order is a permutation of those indexes. Grouping is
// a presentation rule layered on top, so adding a page never rewrites the order a
// user already arranged. The rules are:
//
//   - A tool belongs to exactly one group. That membership is fixed; a drag
//     reorders the tools *inside* their group.
//   - Groups appear in a fixed order (foundation first, optional components last),
//     because the group order is a statement about the product, not a preference.
//   - A search hides the grouping entirely: a filtered list has one order, the one
//     the user is looking for.

import (
	"fmt"
	"sort"
	"strings"

	myui "github.com/egoist/mygo/ui"
	"github.com/lobehub/lobe-mygo/pkg/lucide"

	"github.com/lllll081926i/imageflow-go/internal/settings"
)

// toolGroup is one section of the navigation and of the landing page.
type toolGroup struct {
	Key string
	// Label is the section heading.
	Label string
	// Hint is the one-line description the landing page shows under the heading.
	Hint string
	// Icon is the section glyph.
	Icon func() *myui.SVG
}

// Group keys. They are identifiers, not labels: a heading may be reworded without
// touching saved state.
const (
	GroupCore    = "core"
	GroupCreator = "creator"
	GroupStudio  = "studio"
	GroupMedia   = "media"
	GroupAI      = "ai"
	GroupLegacy  = "legacy"
)

// toolGroups is the navigation order. The foundation comes first because it holds
// the tools an ordinary user opens most; optional and frozen features come last so
// they never push a daily tool below the fold.
var toolGroups = []toolGroup{
	{Key: GroupCore, Label: "基础处理", Hint: "日常最高频的转换、尺寸、裁剪、压缩与基础编辑",
		Icon: lucide.Scaling},
	{Key: GroupCreator, Label: "创作与排版", Hint: "宫格、拼图、长图、水印、文字制图、标注与截图美化",
		Icon: lucide.LayoutTemplate},
	{Key: GroupStudio, Label: "专业图像", Hint: "调色、滤镜、像素级对比与直方图分析",
		Icon: lucide.SlidersHorizontal},
	{Key: GroupMedia, Label: "动态与素材", Hint: "动图、图标、取色、点阵与表情包",
		Icon: lucide.Film},
	{Key: GroupAI, Label: "智能增强", Hint: "可选的本地 AI 放大；未安装组件时不影响其它工具",
		Icon: lucide.Sparkles},
	{Key: GroupLegacy, Label: "历史功能", Hint: "保留的旧入口，冻结维护、不再扩展",
		Icon: lucide.FileText},
}

// groupByKey resolves a group key.
func groupByKey(key string) (toolGroup, bool) {
	for _, g := range toolGroups {
		if g.Key == key {
			return g, true
		}
	}
	return toolGroup{}, false
}

// toolGroupKey is one tool's group, resolved through the membership table.
//
// Grouping is a table rather than a method on Tool on purpose: it keeps the page
// implementations free of navigation concerns, and it keeps the whole information
// architecture readable in one place. A page missing from the table fails
// TestEveryToolHasAGroup, so the table cannot silently fall out of date.
func toolGroupKey(tool Tool) string {
	if group, ok := toolGroupOf[tool.ID()]; ok {
		return group
	}
	// An unregistered page still has to appear somewhere: the foundation group is
	// the least surprising home, and the test above is what actually catches it.
	return GroupCore
}

// toolGroupOf is the ID → group table. It is the single place membership is
// declared; `TestEveryToolHasAGroup` fails the build if a page is missing.
var toolGroupOf = map[string]string{
	ToolConvert:  GroupCore,
	ToolResize:   GroupCore,
	ToolCrop:     GroupCore,
	ToolCompress: GroupCore,
	ToolEdit:     GroupCore,

	ToolGrid:       GroupCreator,
	ToolLayoutPage: GroupCreator,
	ToolCollage:    GroupCreator,
	ToolStitch:     GroupCreator,
	ToolWatermark:  GroupCreator,
	ToolRichText:   GroupCreator,
	ToolAnnotate:   GroupCreator,
	ToolScreenshot: GroupCreator,
	ToolStudio:     GroupCreator,

	ToolGrade:    GroupStudio,
	ToolFilter:   GroupStudio,
	ToolCompare:  GroupStudio,
	ToolAnalysis: GroupStudio,

	ToolGIF:     GroupMedia,
	ToolIcon:    GroupMedia,
	ToolPalette: GroupMedia,
	ToolPixel:   GroupMedia,
	ToolEmote:   GroupMedia,

	ToolUpscale: GroupAI,

	ToolPDF: GroupLegacy,
}

// toolSummary is the one-line description a landing-page card shows. It says what
// the page does in the user's words, not the engine's.
var toolSummary = map[string]string{
	ToolConvert:  "转成 JPG、PNG、WebP、AVIF、TIFF、BMP 或 ICO，可选保留元数据",
	ToolResize:   "按比例、长边、短边缩放，或一次导出多种尺寸",
	ToolCrop:     "常用比例与九宫取景，可加圆角与填充色",
	ToolCompress: "质量优先、体积优先或无损优化，保持像素尺寸",
	ToolEdit:     "旋转、翻转、画布、边框、圆角、投影与基础调色，可撤销",

	ToolGrid:       "把一张图切成 2×2 到 5×5 的等分宫格",
	ToolLayoutPage: "按行列切分，可先裁成正方形",
	ToolCollage:    "1–16 张模板拼图，逐格取景、标注与水印",
	ToolStitch:     "横向或纵向拼成长图，可设间距、背景与边框",
	ToolWatermark:  "文字水印，九宫位置、平铺、描边与投影",
	ToolRichText:   "标题、正文、列表与配图排版成一张图片",
	ToolAnnotate:   "箭头、形状、文字、马赛克与不可逆安全遮盖",
	ToolScreenshot: "留白、圆角、投影与渐变背景的截图美化",
	ToolStudio:     "图片 / 文字 / 形状图层画布，位置尺寸与层级可调",

	ToolGrade:    "曝光、明暗、颜色、色阶、曲线与 3D LUT 调色",
	ToolFilter:   "黑白、复古、模糊、锐化、浮雕、边缘与像素化",
	ToolCompare:  "PSNR、SSIM、通道差与差异图，可选对齐方式",
	ToolAnalysis: "RGB、亮度与 Alpha 直方图，过曝与透明像素统计",

	ToolGIF:     "帧导出、倒放、变速、缩放与 GIF/APNG/WebP 互转",
	ToolIcon:    "多尺寸 PNG、ICO、favicon 与内嵌 PNG 的 SVG",
	ToolPalette: "主题色与 2–16 色调色板提取",
	ToolPixel:   "严格黑白与抖动点阵，可选真正的 1-bit PNG",
	ToolEmote:   "上下字幕、居中大字与对话气泡，动图可按帧加字",

	ToolUpscale: "可选的本地推理放大；需另装引擎与模型",

	ToolPDF: "纸张、方向、边距与网格拼版（冻结维护）",
}

// toolKeywords are the extra search terms a page answers to. They are what a user
// types instead of the product's own wording: 「改大小」 for 尺寸调整, 「打码」 for
// 标注. The search never looks at the user's pictures.
var toolKeywords = map[string][]string{
	ToolConvert:  {"格式", "转换", "转格式", "jpg", "png", "webp", "avif", "heic"},
	ToolResize:   {"缩放", "改大小", "尺寸", "分辨率", "批量尺寸"},
	ToolCrop:     {"裁剪", "剪切", "头像", "比例", "裁剪框"},
	ToolCompress: {"压缩", "变小", "体积", "kb", "画质"},
	ToolEdit:     {"编辑", "旋转", "翻转", "画布", "圆角", "边框", "投影", "模糊", "锐化"},

	ToolGrid:       {"宫格", "九宫格", "切九宫", "切图", "朋友圈"},
	ToolLayoutPage: {"布局", "行列", "切分", "排版"},
	ToolCollage:    {"拼图", "多图", "模板", "拼贴"},
	ToolStitch:     {"长图", "拼接", "接图", "滚动截图"},
	ToolWatermark:  {"水印", "logo", "版权", "平铺"},
	ToolRichText:   {"文字", "富文本", "排版", "卡片", "图文"},
	ToolAnnotate:   {"标注", "打码", "马赛克", "遮盖", "箭头", "圈", "涂鸦"},
	ToolScreenshot: {"截图", "美化", "投影", "圆角", "样机"},
	ToolStudio:     {"图层", "自由布局", "画布", "排版"},

	ToolGrade:    {"调色", "曲线", "色阶", "lut", "白平衡", "曝光"},
	ToolFilter:   {"滤镜", "特效", "复古", "黑白", "模糊", "锐化", "素描"},
	ToolCompare:  {"对比", "比较", "差异", "psnr", "ssim"},
	ToolAnalysis: {"分析", "直方图", "统计", "过曝", "色值"},

	ToolGIF:     {"动图", "gif", "apng", "webp", "帧", "倒放", "变速"},
	ToolIcon:    {"图标", "ico", "favicon", "应用图标"},
	ToolPalette: {"取色", "配色", "色板", "主色", "拾色"},
	ToolPixel:   {"像素画", "点阵", "黑白", "抖动", "1bit", "像素艺术"},
	ToolEmote:   {"表情包", "表情", "梗图", "贴纸", "动图加字"},

	ToolUpscale: {"ai", "放大", "超分", "高清", "upscale", "增强"},

	ToolPDF: {"pdf", "文档", "导出pdf"},
}

// expandedGroups is the sidebar's open/closed state, keyed by group. A missing key
// means "open": the grouped list is browsable by default, and collapsing is an
// explicit action.
func (a *App) groupExpanded(key string) bool {
	if a.collapsedGroups == nil {
		return true
	}
	// The group that holds the current page is always open: hiding the row of the
	// page you are on leaves the selection pill with nothing to point at.
	for i, tool := range a.Tools {
		if toolGroupKey(tool) == key && i == a.Active {
			return true
		}
	}
	return !a.collapsedGroups[key]
}

// toggleGroup flips one group's open state.
//
// The group holding the current page cannot be folded: its rows must stay visible or
// the selection pill would point at nothing. The click is refused rather than
// recorded, so the section does not suddenly collapse later when the user switches
// pages.
func (a *App) toggleGroup(key string) {
	for i, tool := range a.Tools {
		if toolGroupKey(tool) == key && i == a.Active {
			return
		}
	}
	if a.collapsedGroups == nil {
		a.collapsedGroups = map[string]bool{}
	}
	a.collapsedGroups[key] = !a.collapsedGroups[key]
	a.persistCollapsedGroups()
	a.Invalidate()
}

// persistCollapsedGroups writes the folded sections once, on the deliberate click a
// fold is. It reuses the same settings transaction the sidebar order uses, so two
// rapid folds cannot lose one of the two lists.
func (a *App) persistCollapsedGroups() {
	keys := make([]string, 0, len(a.collapsedGroups))
	for key, folded := range a.collapsedGroups {
		if folded {
			keys = append(keys, key)
		}
	}
	sort.Strings(keys)
	a.Settings.RailCollapsedGroups = keys
	a.settingsBase.RailCollapsedGroups = append([]string(nil), keys...)
	if err := a.Session.UpdateSettings(func(next *settings.Settings) {
		next.RailCollapsedGroups = append([]string(nil), keys...)
	}); err != nil {
		a.SetStatus("保存侧栏分组状态失败：" + err.Error())
	}
}

// toolIndex resolves a tool ID to its index in App.Tools, or -1.
func (a *App) toolIndex(id string) int {
	for i, tool := range a.Tools {
		if tool.ID() == id {
			return i
		}
	}
	return -1
}

// railRow is one line of the navigation. A header has Tool -1; a tool row carries
// its index. Y is the row's top inside the scrolling list's content, which is what
// the selection pill and the drag gesture both position against.
type railRow struct {
	Header bool
	Group  string
	Tool   int
	Y      float32
}

// Row metrics. A header is shorter than a tool row because it carries no plate: the
// heading plus the column's gap is enough separation between two sections, and an
// extra spacer would be a second source of y arithmetic.
const railHeaderHeight = 28

// railRows computes the visible navigation as a list of positioned rows.
//
// The order inside a group follows the saved sidebar permutation, so a reorder the
// user performed survives; the group order is fixed. A search result list has no
// headers at all: it is one flat answer.
//
// The y values are the *layout's* y values, not an approximation: every child of the
// list is a flow child separated by railRowGap, so this table adds that same gap
// between every pair. The selection pill and the drag both position against these
// numbers, which is why they cannot be derived from "index times a pitch".
//
// The result is cached per frame shape: the rail, the pill and the drag all ask for
// it several times per frame, and rebuilding a slice of thirty rows on every ask
// would allocate for nothing.
func (a *App) railRows() []railRow {
	key := a.railRowsKey()
	if a.railRowsCache != nil && a.railRowsKeyValue == key {
		return a.railRowsCache
	}
	rows := a.buildRailRows()
	a.railRowsCache = rows
	a.railRowsKeyValue = key
	return rows
}

// railRowsKey summarises everything the row table depends on.
func (a *App) railRowsKey() string {
	expanded := make([]byte, 0, len(toolGroups))
	for _, g := range toolGroups {
		if a.groupExpanded(g.Key) {
			expanded = append(expanded, '1')
		} else {
			expanded = append(expanded, '0')
		}
	}
	return fmt.Sprintf("%s|%s|%v|%d|%v", a.searchQuery, string(expanded), a.railOrderIDs, a.Active, sortedFoldKeys(a.collapsedGroups))
}

// sortedFoldKeys renders the folded set in a stable order, so the memo key does
// not change because a map iterated differently.
func sortedFoldKeys(folded map[string]bool) []string {
	keys := make([]string, 0, len(folded))
	for key, on := range folded {
		if on {
			keys = append(keys, key)
		}
	}
	sort.Strings(keys)
	return keys
}

// buildRailRows is the uncached table.
func (a *App) buildRailRows() []railRow {
	// Every child of the list is a flow child separated by railRowGap, so the y of a
	// row is the sum of the heights before it plus one gap for every pair. Tracking
	// that sum here is what makes the table the layout's own answer rather than an
	// approximation of it.
	var y float32
	added := 0
	push := func(rows []railRow, row railRow, height float32) []railRow {
		if added > 0 {
			y += railRowGap
		}
		row.Y = y
		y += height
		added++
		return append(rows, row)
	}

	if strings.TrimSpace(a.searchQuery) != "" {
		rows := make([]railRow, 0, len(a.Tools))
		for _, idx := range a.matchedTools() {
			rows = push(rows, railRow{Tool: idx, Group: toolGroupKey(a.Tools[idx])}, railRowHeight)
		}
		return rows
	}
	// The tools of one group, in the saved order.
	byGroup := map[string][]int{}
	for _, idx := range a.railOrderSnapshot() {
		if idx < 0 || idx >= len(a.Tools) {
			continue
		}
		group := toolGroupKey(a.Tools[idx])
		byGroup[group] = append(byGroup[group], idx)
	}
	rows := make([]railRow, 0, len(a.Tools)+len(toolGroups))
	for _, g := range toolGroups {
		tools := byGroup[g.Key]
		if len(tools) == 0 {
			continue
		}
		rows = push(rows, railRow{Header: true, Group: g.Key, Tool: -1}, railHeaderHeight)
		if !a.groupExpanded(g.Key) {
			continue
		}
		for _, idx := range tools {
			rows = push(rows, railRow{Tool: idx, Group: g.Key}, railRowHeight)
		}
	}
	return rows
}

// railToolRowY returns a tool row's planned top, or -1 when the row is not visible
// (its group is collapsed, or a search filters it out).
func (a *App) railToolRowY(tool int) float32 {
	for _, row := range a.railRows() {
		if !row.Header && row.Tool == tool {
			return row.Y
		}
	}
	return -1
}

// railToolAtY returns the tool whose row contains y, or the nearest tool row.
func (a *App) railToolAtY(y float32) int {
	rows := a.railRows()
	best, bestDist := -1, float32(1e9)
	for _, row := range rows {
		if row.Header {
			continue
		}
		centre := row.Y + railRowHeight/2
		dist := centre - y
		if dist < 0 {
			dist = -dist
		}
		if dist < bestDist {
			best, bestDist = row.Tool, dist
		}
	}
	return best
}

// matchedTools filters the tool list by the search box. A tool matches on its name,
// its summary and its keywords; the comparison is case-insensitive so 「AI」 and
// 「ai」 find the same page.
func (a *App) matchedTools() []int {
	query := strings.ToLower(strings.TrimSpace(a.searchQuery))
	out := make([]int, 0, len(a.Tools))
	if query == "" {
		order := a.railOrderSnapshot()
		return append(out, order...)
	}
	for _, idx := range a.railOrderSnapshot() {
		if idx < 0 || idx >= len(a.Tools) {
			continue
		}
		tool := a.Tools[idx]
		if toolMatches(tool, query) {
			out = append(out, idx)
		}
	}
	// A search result is ranked: an exact prefix beats a keyword hit, so typing
	// 「压缩」 puts 图片压缩 first even though 格式转换 mentions compression in its
	// summary.
	sort.SliceStable(out, func(i, j int) bool {
		return matchScore(a.Tools[out[i]], query) > matchScore(a.Tools[out[j]], query)
	})
	return out
}

// toolMatches reports whether one tool answers to a lower-cased query.
func toolMatches(tool Tool, query string) bool {
	return matchScore(tool, query) > 0
}

// matchScore ranks one tool against a query. 0 means "no match".
func matchScore(tool Tool, query string) int {
	name := strings.ToLower(tool.Name())
	switch {
	case name == query:
		return 1000
	case strings.HasPrefix(name, query):
		return 900
	case strings.Contains(name, query):
		return 800
	}
	for _, keyword := range toolKeywords[tool.ID()] {
		keyword = strings.ToLower(keyword)
		if keyword == query {
			return 700
		}
		if strings.HasPrefix(keyword, query) {
			return 600
		}
		if strings.Contains(keyword, query) {
			return 500
		}
	}
	if strings.Contains(strings.ToLower(toolSummary[tool.ID()]), query) {
		return 300
	}
	return 0
}
