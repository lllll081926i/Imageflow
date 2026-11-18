package ui

// home.go is the landing page.
//
// Twenty-five rows in a column is a list to read, not a place to start. The landing
// page answers "what can this app do" with the four things a user actually needs at
// that moment: a search box, the tools they open most, the tools grouped by task,
// and the state of the optional components. It shows the same groups, in the same
// order, as the sidebar — one information architecture, two ways into it.

import (
	"fmt"
	"strings"

	myui "github.com/egoist/mygo/ui"
	"github.com/lobehub/lobe-mygo/pkg/lucide"
)

// homeFavourites is the quick row: the pages an ordinary user opens to get one task
// done. It is a fixed list rather than a frequency counter on purpose — a counter
// with no data would show an arbitrary six, and inventing one is worse than
// choosing six that cover the common jobs.
var homeFavourites = []string{
	ToolCompress, ToolConvert, ToolCrop, ToolCollage, ToolScreenshot, ToolGrade,
}

// homePage is the landing view.
func (a *App) homePage(c *myui.Context, st style) {
	card := myui.Column(c).Grow(1).FillHeight().Radius(st.RadiusCard).
		Background(st.Card).Border(1, st.Divider).
		Shadow(0, 2, 12, 0, myui.RGBA(0, 0, 0, 0.04)).Clip().
		Label("首页").Key("home")
	card.Children(func() {
		myui.Scroll(c).FillWidth().Grow(1).Children(func() {
			myui.Column(c).FillWidth().Padding(24).Gap(18).Children(func() {
				a.homeHeader(c, st)
				a.homeSearch(c, st)
				if strings.TrimSpace(a.searchQuery) != "" {
					a.homeSearchResults(c, st)
					return
				}
				a.homeFavouriteRow(c, st)
				a.homeGroups(c, st)
				a.homeComponentState(c, st)
			})
		})
	})
}

// homeHeader 显示产品名称与工具数量。
func (a *App) homeHeader(c *myui.Context, st style) {
	myui.Column(c).FillWidth().Gap(6).Children(func() {
		myui.Row(c).FillWidth().AlignItems(myui.Center).Gap(10).Children(func() {
			if mark := logoBitmap(logoHomeSize); mark != nil {
				myui.Image(c, mark).Size(logoHomeSize, logoHomeSize).Fit(myui.Contain)
			}
			myui.Column(c).Gap(2).Children(func() {
				myui.Text(c, "ImageFlow").FontSize(22).FontWeight(700).TextColor(st.Text)
			})
			myui.Spacer(c).Grow(1)
			statusPill(c, fmt.Sprintf("%d 个工具", len(a.Tools)), st.Brand)
		})
	})
}

// homeSearch 仅搜索首页工具卡片。
func (a *App) homeSearch(c *myui.Context, st style) {
	row := myui.Row(c).FillWidth().Height(48).PaddingX(14).Gap(10).
		AlignItems(myui.Center).Radius(st.RadiusControl).
		Background(st.Recessed).Border(1, st.Divider)
	row.Children(func() {
		myui.Icon(c, lucide.Search()).Width(17).Height(17).TextColor(st.TextDim)
		input := myui.TextInput(c, &a.searchQuery).Grow(1).Height(44).
			Border(0, myui.RGBA(0, 0, 0, 0)).Background(myui.RGBA(0, 0, 0, 0)).FontSize(14).
			Placeholder("搜索工具")
		input.PlaceholderColor(st.TextDim).FocusRing(false)
		if strings.TrimSpace(a.searchQuery) != "" {
			a.ghostButton(c, st, "清除", lucide.X(), func() {
				a.searchQuery = ""
				a.Invalidate()
			})
		}
	})
}

// homeSearchResults lists the tools a query matches, ranked.
func (a *App) homeSearchResults(c *myui.Context, st style) {
	matches := a.matchedTools()
	if len(matches) == 0 {
		stateAlert(c, "没有匹配的工具", st.Warning, lucide.Info())
		return
	}
	myui.Text(c, fmt.Sprintf("找到 %d 个工具", len(matches))).FontSize(12).
		FontWeight(600).TextColor(st.TextSub)
	myui.Grid(c).FillWidth().Columns(a.homeColumns()).GapX(16).GapY(16).Children(func() {
		for _, idx := range matches {
			a.homeToolCard(c, st, idx)
		}
	})
}

// homeFavouriteRow is the quick row of the six most common jobs.
func (a *App) homeFavouriteRow(c *myui.Context, st style) {
	sectionTitle(c, st, "常用")
	myui.Grid(c).FillWidth().Columns(a.homeColumns()).GapX(16).GapY(16).Children(func() {
		for _, id := range homeFavourites {
			idx := a.toolIndex(id)
			if idx < 0 {
				continue
			}
			a.homeToolCard(c, st, idx)
		}
	})
}

// homeGroups renders every group with its tools, matching the sidebar's sections.
func (a *App) homeGroups(c *myui.Context, st style) {
	for _, group := range toolGroups {
		tools := a.toolsInGroup(group.Key)
		if len(tools) == 0 {
			continue
		}
		sectionTitleIcon(c, st, group.Icon(), group.Label)
		myui.Grid(c).FillWidth().Columns(a.homeColumns()).GapX(16).GapY(16).Children(func() {
			for _, idx := range tools {
				a.homeToolCard(c, st, idx)
			}
		})
	}
}

// toolsInGroup lists one group's tools in the saved sidebar order.
func (a *App) toolsInGroup(key string) []int {
	out := make([]int, 0, 8)
	for _, idx := range a.railOrderSnapshot() {
		if idx < 0 || idx >= len(a.Tools) {
			continue
		}
		if toolGroupKey(a.Tools[idx]) == key {
			out = append(out, idx)
		}
	}
	return out
}

func (a *App) homeColumns() int {
	if a.windowWidth > 0 && a.windowWidth < 1180 {
		return 2
	}
	return 3
}

// homeToolCard 显示可点击的工具图标与名称。
func (a *App) homeToolCard(c *myui.Context, st style, idx int) {
	tool := a.Tools[idx]
	card := myui.ButtonBase(c).Key("home-card-" + tool.ID()).FillWidth().MinHeight(132).
		Padding(20).Gap(14).Radius(st.RadiusPanel).
		Cursor(myui.CursorPointer).Label(tool.Name()).
		FocusRingColor(st.Brand).FocusRingOffset(2).
		Transition(motion(springTap))
	card.Column().AlignItems(myui.Start)
	switch {
	case card.Pressed():
		card.Background(st.Recessed).Border(1, st.Brand).Scale(0.99, 0.99)
	case card.Hovered():
		card.Background(st.Recessed).Border(1, st.Brand)
	default:
		card.Background(st.Card).Border(1, st.Divider)
	}
	card.Children(func() {
		myui.Column(c).FillWidth().Gap(14).Children(func() {
			myui.Row(c).Size(44, 44).Shrink(0).Radius(st.RadiusControl).
				Justify(myui.Center).AlignItems(myui.Center).
				Background(featureTintPlate(idx, st)).Children(func() {
				if icon := tool.Icon(); icon != nil {
					myui.Icon(c, icon).Width(24).Height(24).
						TextColor(featureTint(idx, st))
				}
			})
			myui.Text(c, tool.Name()).FontSize(16).FontWeight(600).
				TextColor(st.Text).SingleLine()
		})
	})
	if card.Clicked() {
		a.openTool(idx)
	}
}

// openTool switches to a page. The landing page closes, the search clears, and the
// sidebar's group for that page opens, so the user can see where they landed.
func (a *App) openTool(idx int) {
	if idx < 0 || idx >= len(a.Tools) {
		return
	}
	a.Active = idx
	a.ShowHome = false
	a.ShowSettings = false
	a.searchQuery = ""
	group := toolGroupKey(a.Tools[idx])
	if a.collapsedGroups[group] {
		a.collapsedGroups[group] = false
		a.persistCollapsedGroups()
	}
	a.railRevealTool = idx
	a.Invalidate()
}

// homeComponentState states, on the landing page, whether the optional AI component
// is installed. It is the honest counterpart of the AI card below it: a user who
// clicks 放大 and finds "组件未安装" should have been told here first.
func (a *App) homeComponentState(c *myui.Context, st style) {
	tint := st.TextDim
	text := "AI 高清放大：尚未检查"
	for _, tool := range a.Tools {
		if page, ok := tool.(*UpscaleTool); ok && page.detected {
			text = "AI 高清放大：未就绪"
			if page.busy {
				text = "AI 高清放大：正在检查"
			}
		}
	}
	if a.upscaleAvailable() {
		tint = st.Success
		text = "AI 高清放大：引擎可用"
	}
	myui.Row(c).FillWidth().Padding(12).Gap(8).AlignItems(myui.Center).
		Radius(st.RadiusPill).Background(st.Recessed).Children(func() {
		myui.Icon(c, lucide.Sparkles()).Width(14).Height(14).TextColor(tint)
		myui.Text(c, text).FontSize(11).TextColor(st.TextSub)
		myui.Spacer(c).Grow(1)
		a.ghostButton(c, st, "打开", lucide.ArrowUpRight(), func() { a.openTool(a.toolIndex(ToolUpscale)) })
	})
}

// upscaleAvailable reports whether the AI page found a usable engine. It reads the
// page's own cached probe so the landing page never spawns a process.
func (a *App) upscaleAvailable() bool {
	for _, tool := range a.Tools {
		if page, ok := tool.(*UpscaleTool); ok {
			return page.env.Available
		}
	}
	return false
}

// sectionTitle 显示分组标题。
func sectionTitle(c *myui.Context, st style, title string) {
	myui.Column(c).FillWidth().Gap(3).Children(func() {
		myui.Text(c, title).FontSize(14).FontWeight(700).TextColor(st.Text)
	})
}

// sectionTitleIcon is the same heading with a glyph, used for the group sections.
func sectionTitleIcon(c *myui.Context, st style, icon *myui.SVG, title string) {
	myui.Row(c).FillWidth().AlignItems(myui.Center).Gap(9).Children(func() {
		myui.Row(c).Size(26, 26).Radius(st.RadiusPill).
			Justify(myui.Center).AlignItems(myui.Center).
			Background(st.BrandSoft).Children(func() {
			myui.Icon(c, icon).Width(14).Height(14).TextColor(st.Brand)
		})
		myui.Column(c).Grow(1).Gap(2).Children(func() {
			myui.Text(c, title).FontSize(14).FontWeight(700).TextColor(st.Text)
		})
	})
}

// homeEntry is the sidebar's route back to the landing page.
func (a *App) homeEntry(c *myui.Context, st style, labelAlpha float32) {
	a.railFooterItem(c, st, "rail-home", lucide.House(), "首页", a.ShowHome && !a.ShowSettings, labelAlpha, func() {
		a.ShowHome = true
		a.ShowSettings = false
		a.searchQuery = ""
	})
}
