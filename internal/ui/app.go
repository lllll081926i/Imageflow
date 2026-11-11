// Package ui is the Lobe MyGo native interface of ImageFlow.
package ui

import (
	"context"
	"fmt"
	"image"
	"strings"
	"sync"
	"time"

	"github.com/egoist/mygo"
	myui "github.com/egoist/mygo/ui"
	"github.com/lobehub/lobe-mygo/pkg/lucide"
	"github.com/lobehub/lobe-mygo/pkg/theme"

	"github.com/lllll081926i/imageflow-go/internal/batch"
	"github.com/lllll081926i/imageflow-go/internal/settings"
	"github.com/lllll081926i/imageflow-go/internal/thumbs"
)

// Tool is one page of the workbench.
type Tool interface {
	// Name is the label in the side navigation and the page title.
	Name() string
	// Icon is the nav glyph.
	Icon() *myui.SVG
	// Panel builds the tool's parameters.
	Panel(c *myui.Context, a *App)
	// PreviewCaption describes what the preview will hold, shown in the preview
	// header. The preview itself is drawn once, by the input card.
	PreviewCaption(a *App) string
	// Run starts the batch for the current parameters.
	Run(a *App)
}

// App is the whole interface state.
//
// The batch goroutine updates Status, Progress, ProgressText and Running from
// outside the UI thread, so those four are guarded by mu; the view reads them
// through the accessors below.
type App struct {
	mu      sync.Mutex
	Session *batch.Session
	Thumbs  *thumbs.Cache
	Window  *mygo.Window

	Active       int
	ShowSettings bool
	Tools        []Tool

	Status       string
	Progress     float32
	ProgressText string
	Running      bool

	// planMu guards planned, the output paths already reserved by the batch in
	// flight, so two rows never resolve to the same file name.
	planMu  sync.Mutex
	planned []string

	// previewMu 保护预览图像、选中路径与请求版本；previewFocus 仅由界面线程使用。
	lastPreview         image.Image
	previewMu           sync.Mutex
	previewPath         string
	previewRevealPath   string
	previewVersion      uint64
	previewLoading      bool
	previewFocusPending bool
	previewLoadMu       sync.Mutex
	previewFocus        myui.Handle

	// quitting is set when the user asked to quit, so the close handler stops
	// hiding the window. quitWithoutTray makes close quit outright, for a
	// desktop with no tray entry to restore a hidden window from.
	quitting        bool
	quitWithoutTray bool

	// update holds the state of the last update check, and lastUpdate the found
	// release so Install can use it.
	update     updateState
	lastUpdate interface {
		Install(ctx context.Context, progress func(int64, int64)) error
	}

	// Settings holds the editable copy shown in the settings page, owned by the UI
	// thread. Background goroutines (a dialog, a drop) never write it: they record
	// their result in Session.Settings under that struct's lock, and the next frame
	// pulls the recent-folder lists across in applyPendingSettings.
	Settings      settings.Settings
	settingsBase  settings.Settings
	importMu      sync.Mutex
	importWorkMu  sync.Mutex
	importCancel  context.CancelFunc
	importContext context.Context
	importPending int

	// railCollapsed hides the tool labels and narrows the rail, as the original
	// sidebar's 收起侧栏 action did. The width animates, so the cards beside it
	// slide instead of jumping.
	railCollapsed bool

	// savedFlash and savedUntil drive the settings page's "已保存" note, which
	// fades out on its own instead of sitting there until the next save.
	savedFlash string
	savedUntil time.Time
	savedTimer *time.Timer

	// pendingPicker holds paths chosen by background file dialogs. A dialog
	// goroutine must never write a panel field directly: that field is bound to
	// a TextInput the UI thread reads every frame. The UI thread drains this
	// map in applyPendingSettings.
	pickerMu      sync.Mutex
	pendingPicker map[*string]string
}

// applyPendingSettings copies the folder history a background goroutine recorded
// into the editable settings. It runs on the UI thread and leaves unsaved edits to
// the other fields alone.
func (a *App) applyPendingSettings() {
	// Drain dialog results on the UI thread before the controls read their
	// bound fields, so no background goroutine writes view state directly.
	a.pickerMu.Lock()
	for dst, path := range a.pendingPicker {
		*dst = path
	}
	clear(a.pendingPicker)
	a.pickerMu.Unlock()

	a.Session.Mu.Lock()
	recentIn := append([]string(nil), a.Session.Settings.RecentInputDirs...)
	recentOut := append([]string(nil), a.Session.Settings.RecentOutputDirs...)
	outDir := a.Session.Settings.DefaultOutputDir
	a.Session.Mu.Unlock()
	a.Settings.RecentInputDirs = recentIn
	a.Settings.RecentOutputDirs = recentOut
	if a.Settings.DefaultOutputDir == a.settingsBase.DefaultOutputDir {
		a.Settings.DefaultOutputDir = outDir
	}
	a.settingsBase.DefaultOutputDir = outDir
	a.settingsBase.RecentInputDirs = append([]string(nil), recentIn...)
	a.settingsBase.RecentOutputDirs = append([]string(nil), recentOut...)
}

// SetStatus records a status line from any goroutine.
func (a *App) SetStatus(text string) {
	a.mu.Lock()
	a.Status = text
	a.mu.Unlock()
}

// flashSettingsSaved shows a short note in the save bar. It clears itself after
// a moment, and asks for the repaint that removes it.
func (a *App) flashSettingsSaved(text string) {
	const lifetime = 2400 * time.Millisecond
	a.mu.Lock()
	a.savedFlash = text
	a.savedUntil = time.Now().Add(lifetime)
	// Reuse one timer. Independent AfterFunc callbacks used to let an older
	// save erase a newer note before its own deadline.
	if a.savedTimer != nil {
		a.savedTimer.Stop()
	}
	a.savedTimer = time.AfterFunc(lifetime, func() {
		a.mu.Lock()
		// A callback that was already running when a newer flash reset the
		// timer must leave that newer note alone.
		if !time.Now().Before(a.savedUntil) {
			a.savedFlash = ""
			a.savedTimer = nil
		}
		a.mu.Unlock()
		a.Invalidate()
	})
	a.mu.Unlock()
}

// SettingsSavedFlash is the note the save bar shows, empty once it expired.
func (a *App) SettingsSavedFlash() string {
	a.mu.Lock()
	defer a.mu.Unlock()
	if time.Now().After(a.savedUntil) {
		return ""
	}
	return a.savedFlash
}

// StatusLine returns the current status line.
func (a *App) StatusLine() string {
	a.mu.Lock()
	defer a.mu.Unlock()
	return a.Status
}

// SetProgress records the batch progress from any goroutine.
func (a *App) SetProgress(percent float32, text string) {
	a.mu.Lock()
	a.Progress = percent
	a.ProgressText = text
	a.mu.Unlock()
}

// ProgressState returns the progress percentage and its caption.
func (a *App) ProgressState() (float32, string) {
	a.mu.Lock()
	defer a.mu.Unlock()
	return a.Progress, a.ProgressText
}

// SetRunning marks the shell as busy or idle.
func (a *App) SetRunning(on bool) {
	a.mu.Lock()
	a.Running = on
	a.mu.Unlock()
}

// IsRunning reports whether a batch is in flight.
func (a *App) IsRunning() bool {
	a.mu.Lock()
	defer a.mu.Unlock()
	return a.Running
}

// SetPreview replaces the previewed image.
func (a *App) SetPreview(img image.Image) {
	a.previewMu.Lock()
	a.previewVersion++
	a.previewLoading = false
	if img == nil {
		a.previewPath = ""
		a.previewRevealPath = ""
		a.previewFocusPending = false
	}
	a.lastPreview = img
	a.previewMu.Unlock()
}

// Preview returns the previewed image, if any.
func (a *App) Preview() image.Image {
	a.previewMu.Lock()
	defer a.previewMu.Unlock()
	return a.lastPreview
}

// NewApp builds the application shell with every tool registered.
func NewApp(s settings.Settings) *App {
	a := &App{
		Session:      batch.NewSession(s),
		Thumbs:       thumbs.New(),
		Settings:     s,
		Status:       "拖入图片或文件夹开始",
		ProgressText: "",
	}
	a.Tools = []Tool{
		&ConvertTool{},
		&ResizeTool{},
		&CropTool{},
		&GridTool{},
		&LayoutTool{},
		&StitchTool{},
		&WatermarkTool{},
		&PaletteTool{},
		&IconTool{},
		&PDFTool{},
		&GIFTool{},
		&RichTextTool{},
		&CompressTool{},
	}
	a.settingsBase = a.Session.SettingsSnapshot()
	// Fill each tool's defaults up front, so the panel and the batch runner agree
	// even when the page has not been drawn yet.
	for _, tool := range a.Tools {
		if init, ok := tool.(interface{ ensure() }); ok {
			init.ensure()
		}
	}
	return a
}

// View is the root view handed to the window.
//
// The shell is three floating cards over the window's own background: the tool
// rail, the input area and the settings panel. The window keeps its native title
// bar, so the app draws no second one.
func (a *App) View(c *myui.Context) {
	theme.Apply(c)
	a.applyPendingSettings()
	st := styles(c)
	// The design contract turns every animation off when the user asks for
	// reduced motion; the flag is read by the transition presets below.
	motionReduced = a.Settings.UIReducedMotion
	// Lobe's own primary is near-black; ImageFlow has one brand blue, so the
	// frame theme is re-pointed at it for every built-in widget.
	applyBrandTheme(c, st)

	myui.Column(c).Fill().Background(st.Shell).Padding(0, 16, 16, 16).Children(func() {
		// The rail's width is animated here, not on the rail itself: a sibling only
		// reflows when the value it reads changes, and a transition on the rail's
		// own Width repaints it after the row was already laid out. Feeding the
		// animated number to the layout every frame is what makes the cards beside
		// it glide instead of snapping once the animation ends.
		root := myui.Row(c).FillWidth().Grow(1).Gap(16).AlignItems(myui.Stretch)
		// The rail gives up its width before the cards do: on a window narrow enough
		// that 216 + 420 leaves the input card nothing, it shows as the icon strip
		// without the user having to collapse it.
		railWidth := a.railTargetWidth()
		if w, _ := c.Size(); w > 0 && w < 1080 {
			railWidth = railCollapsedWidth
		}
		if !motionReduced {
			railWidth = root.AnimateTo("rail-width", railWidth, railWidthSpring)
		}
		root.Children(func() {
			// The rail carries its own key. Keyed siblings are matched by key across
			// frames, and without one it was matched positionally against the page,
			// whose key changes on every switch — so the rail was rebuilt alongside
			// the page it sits next to, and its own animation state went with it.
			a.rail(c, st, railWidth)
			// The page container is keyed by what it shows, so switching tools or
			// opening the settings builds a new element and pageSwap's fade plays.
			page := myui.Row(c).Grow(1).FillHeight().Gap(16).
				AlignItems(myui.Stretch).Key(a.pageKey()).
				Transition(motion(pageSwap))
			page.Children(func() {
				if a.ShowSettings {
					a.settingsPage(c, st)
					return
				}
				tool := a.ActiveTool()
				if tool == nil {
					return
				}
				a.inputCard(c, st)
				a.settingsCard(c, st, tool)
			})
		})
	})
}

// pageKey names what the content area is showing. Changing it is what makes the
// page hand over with an animation instead of swapping in place.
func (a *App) pageKey() string {
	if a.ShowSettings {
		return "page-settings"
	}
	return fmt.Sprintf("page-tool-%d", a.Active)
}

// pageSwap is the page handover: a plain fade, nothing else.
//
// Shadows is set only to opt out of MyGo's default: a Transition with no
// property flags animates *all* of them, including position and size. The page
// is the rail's sibling in the same row, so its x and width change on every
// frame of the rail's width animation. Animating those a second time, with a
// different curve, made the cards trail the rail by more than 150 DIP mid-way
// through a collapse. Shadows alone means the layout is followed immediately;
// Enter still fades because a Motion drives it, not the property flags.
//
// There is no Exit. A cross-fade needs a copy of the old page drawn under the
// new one, and two semi-transparent full-window layouts composited every frame
// flickered as their alphas crossed.
var pageSwap = myui.ElementTransition{
	Animation: &shellCurve,
	Shadows:   true,
	// Starting at 0 left the whole content area blank for the first frames of a
	// switch, which read as the window refreshing. A quiet dissolve from 72% is
	// the same handover without the blank frame, and it matches the selection
	// indicator fading out over the same curve.
	Enter: &myui.Motion{Opacity: 0.72},
}

// railTargetWidth is where the rail's spring is headed: railWidth open, railCollapsedWidth closed.
func (a *App) railTargetWidth() float32 {
	if a.railCollapsed {
		return railCollapsedWidth
	}
	return railWidth
}

// railWidth and railCollapsedWidth are the rail's two resting widths. The open
// rail is measured from what its content needs: a 32 DIP icon plate, 11 of gap
// and the longest label (「富文本制图」). 216 leaves that label room without the
// 16 DIP of slack the previous 232 showed as dead space beside every row.
const (
	railWidth          = 216
	railCollapsedWidth = 72
	// railItemPadding is the row's inner x padding. It is the same in both states
	// on purpose: the plate then sits on the same pixel whether the rail is open
	// or collapsed, and only the label changes beside it. The value is what
	// centres the 32 DIP plate in the collapsed strip (72 - 2*12 padding = 48 of
	// content, so 8 + 32 leaves 8 on the other side).
	railItemPadding = 8
	// The labels fade across a width interval instead of switching on at a
	// threshold. A boolean cut made every label vanish in one frame in the middle
	// of the collapse, out of step with the rail that was still narrowing.
	railLabelFadeFrom = railCollapsedWidth + 24
	railLabelFadeTo   = railWidth - 8
	// railRowHeight and railRowGap describe the tool rows. The selection pill lives
	// in the list's own coordinate space and is positioned with Absolute, so it
	// needs the same numbers the layout uses: a constant that drifted would park
	// the pill between two rows.
	railRowHeight = 48
	railRowGap    = 2
)

// The pill travels between rows on the shell's curve too. It used to be a 300 ms
// spring with a bounce while the page faded over 200 ms and the rail narrowed
// over 260 ms: three different durations in one window. One curve makes the
// whole shell read as a single movement.
var railPillCurve = shellCurve

// railWidthSpring is the shell's own curve, shared with the page handover so a
// collapse and a page switch that happen together settle on the same frame.
// No bounce: a 144 DIP width change with a bounce reads as a wobble across the
// whole window.
var railWidthSpring = shellCurve

// applyBrandTheme points the frame theme's accent at the brand colour.
//
// Every built-in widget (switch, slider, checkbox, focus ring, text selection)
// reads these tokens, so one assignment here is what keeps the whole window on
// the single brand blue instead of Lobe's near-black default.
func applyBrandTheme(c *myui.Context, st style) {
	base := theme.TokensOf(c).Theme()
	base.Accent = st.Brand
	base.AccentHover = st.BrandHover
	base.AccentPressed = st.BrandDeep
	base.AccentText = myui.RGB(255, 255, 255)
	base.Focus = myui.Color{R: st.Brand.R, G: st.Brand.G, B: st.Brand.B, A: 230}
	base.FocusRingWidth = 2
	base.FocusRingOffset = 1
	base.Selection = myui.RGBA(st.Brand.R, st.Brand.G, st.Brand.B, 0.24)
	base.Background = st.Shell
	base.Text = st.Text
	base.TextMuted = st.TextSub
	base.Border = st.Divider
	base.Surface = st.Recessed
	base.SurfaceHover = st.CardHover
	base.SurfacePressed = st.Recessed
	base.Scrollbar = st.TextDim
	// With InactiveAccent left zero, MyGo greys every filled control while it
	// believes the window is not the active one, which is what made switches,
	// sliders and progress bars grey until the pointer entered. This app has one
	// accent in every window state, so the two are pinned to the same blue.
	base.InactiveAccent = st.Brand
	base.Success = st.Success
	base.Warning = st.Warning
	base.Danger = st.Danger
	// Lobe's default control radius (8) is tighter than this system's 12, which
	// made inputs and switches read as a different app's widgets.
	base.Radius = st.RadiusControl
	c.SetTheme(base)
}

// statusLabel is the header's one-line state: what the batch is doing, or 就绪.
func (a *App) statusLabel() string {
	if a.IsRunning() {
		percent, _ := a.ProgressState()
		return fmt.Sprintf("处理中 %d%%", int(percent))
	}
	if a.Session.Len() == 0 {
		return "待添加"
	}
	return "就绪"
}

// statusTint colours that state: brand while working, muted otherwise.
func (a *App) statusTint(st style) myui.Color {
	if a.IsRunning() {
		return st.Brand
	}
	return st.TextDim
}

// rail is the tool navigation card: bare coloured icons, one gradient pill for
// the active tool, exactly as the original sidebar. It collapses to a strip of
// icons, which is the original's 收起侧栏 action.
func (a *App) rail(c *myui.Context, st style, width float32) {
	// A fixed key: the rail keeps its element (and therefore its pill's animation
	// channels and the width it animates) across page switches, which is what stops
	// it from being rebuilt alongside the page beside it.
	rail := myui.Column(c).Width(width).Shrink(0).FillHeight().Key("rail").
		Padding(10, 12, 10, 12).Gap(0).
		Radius(st.RadiusCard).Background(st.Sidebar).Border(1, st.Divider).
		Shadow(0, 2, 12, 0, myui.RGBA(0, 0, 0, 0.04)).Clip().Label("工具卡")

	// Label visibility is derived from the animated width rather than a boolean
	// threshold: the text fades on the very frames the rail narrows, with no
	// second animation to fall out of step, and no one-frame jump mid-collapse.
	labelAlpha := railLabelAlpha(width)

	rail.Children(func() {
		// No "工具箱" caption: the icons and their labels already say what the
		// column is, and the heading pushed the first tool down for nothing.

		myui.Scroll(c).FillWidth().Grow(1).Children(func() {
			a.railList(c, st, labelAlpha)
		})

		myui.Column(c).FillWidth().Gap(2).PaddingY(8).Children(func() {
			myui.Row(c).FillWidth().Height(1).Background(st.Divider).Margin(0, 4, 6, 4)
			// 全局设置 toggles: selecting it while the settings page shows returns
			// to the tool page, which is why the settings header carries no second
			// back button.
			a.railFooterItem(c, st, "rail-settings", lucide.Settings(), "全局设置", a.ShowSettings, labelAlpha, func() {
				a.ShowSettings = !a.ShowSettings
			})
			// The collapse action the original sidebar had. Its icon flips, so the
			// same button both opens and closes the rail.
			icon := lucide.PanelLeft()
			label := "收起侧栏"
			if a.railCollapsed {
				icon = lucide.PanelLeftOpen()
				label = "展开侧栏"
			}
			a.railFooterItem(c, st, "rail-collapse", icon, label, false, labelAlpha, func() {
				a.railCollapsed = !a.railCollapsed
			})
		})
	})
}

// railLabelAlpha maps the animated rail width to how visible the row labels are.
// It reaches 0 before the label would have no room, and 1 only near full width.
func railLabelAlpha(width float32) float32 {
	if width <= railLabelFadeFrom {
		return 0
	}
	if width >= railLabelFadeTo {
		return 1
	}
	return (width - railLabelFadeFrom) / (railLabelFadeTo - railLabelFadeFrom)
}

// railList is the tool rows with one selection pill behind them.
//
// The pill is a single element that travels between rows instead of each row
// painting its own highlight. Per-row highlights cannot animate: the old row loses
// its gradient in the same frame the new row gains one, so the selection jumps. A
// single pill has an identity across frames, so its Top is a value the framework
// can interpolate.
//
// The pill is always built, including on the settings page where it fades out.
// MyGo identifies an unkeyed child by its position among its siblings; removing
// the first child used to renumber all thirteen rows, so opening the settings
// rebuilt the entire rail, reset every row's hover/focus state and restarted the
// pill's travel from the top. Keeping the child (and keying every row) makes the
// list's shape constant across every page switch.
func (a *App) railList(c *myui.Context, st style, labelAlpha float32) {
	// activeRow is which row the pill rests on. On the settings page the pill
	// stays where it was and only fades, so returning to a tool travels from the
	// last selected row instead of restarting at the top.
	activeRow := a.Active
	if activeRow < 0 || activeRow >= len(a.Tools) {
		activeRow = 0
	}
	visible := !a.ShowSettings && a.Active >= 0 && a.Active < len(a.Tools)

	myui.Column(c).FillWidth().Gap(railRowGap).Children(func() {
		target := float32(activeRow * (railRowHeight + railRowGap))
		// ZIndex(-1) puts the pill behind the rows: the rows paint no background
		// of their own, so the pill shows through, and an Absolute child would
		// otherwise paint above its later siblings.
		pill := myui.Row(c).FillWidth().Height(railRowHeight).
			Radius(st.RadiusControl).Absolute().Top(0).Left(0).ZIndex(-1).
			Key("rail-pill").
			Gradient(st.Brand, st.BrandGradientTo, 90).
			Shadow(0, 4, 12, 0, st.BrandGlow).Label("选中指示")
		// AnimateTo runs while the list is built, and its value drives this
		// frame's Top: the pill therefore travels instead of jumping. The same
		// curve as the rail width and the page handover keeps the fade in step.
		y := target
		alpha := float32(0)
		if visible {
			alpha = 1
		}
		if !motionReduced {
			y = pill.AnimateTo("rail-pill-top", target, railPillCurve)
			alpha = pill.AnimateTo("rail-pill-alpha", alpha, shellCurve)
		}
		pill.Top(y).Opacity(alpha)
		for i, tool := range a.Tools {
			// The selected row's palette fades with the indicator. Switching it on
			// the first frame of the settings page left white text on a gradient
			// that was still fading out, which read as the rail being repainted.
			rowAlpha := float32(0)
			if i == activeRow {
				rowAlpha = alpha
			}
			a.railItem(c, st, i, tool, labelAlpha, rowAlpha)
		}
	})
}

// mixColor blends two straight-alpha colours. The selection indicator and the
// selected row's icon, label and plate fade on the same value so the palette
// never snaps a frame ahead of the gradient behind it.
func mixColor(from, to myui.Color, k float32) myui.Color {
	k = maxFloat(0, minFloat(1, k))
	ch := func(a, b uint8) uint8 {
		return uint8(float32(a) + (float32(b)-float32(a))*k + 0.5)
	}
	return myui.Color{
		R: ch(from.R, to.R), G: ch(from.G, to.G),
		B: ch(from.B, to.B), A: ch(from.A, to.A),
	}
}

func (a *App) railItem(c *myui.Context, st style, index int, tool Tool, labelAlpha, selectAlpha float32) {
	active := a.Active == index && !a.ShowSettings
	item := myui.ButtonBase(c).Key(fmt.Sprintf("rail-tool-%d", index)).
		FillWidth().Height(railRowHeight).Gap(11).
		AlignItems(myui.Center).Radius(st.RadiusControl).
		Cursor(myui.CursorPointer).Label(tool.Name()).
		FocusRingColor(st.Brand).FocusRingOffset(1).
		Transition(motion(springTap))
	// The item keeps one alignment in every state: the plate sits at one x whether
	// the rail is open or collapsed, and only the label appears beside it.
	item = item.Justify(myui.Start).PaddingX(railItemPadding)
	// The row itself paints no selection: the pill behind it does, and it slides
	// between rows. Only the press and hover states belong to the row.
	switch {
	case item.Pressed():
		item.Background(st.DividerTop).Scale(0.99, 0.99)
	case item.Hovered() && !active:
		item.Background(st.Recessed)
	}
	item.Children(func() {
		// The coloured plate is what makes the rail read as a set of tools rather
		// than a text menu: the original sidebar tinted a square behind every icon.
		// The plate keeps one fixed size in every state: growing it on selection
		// pushed the label right, so the active row read as misaligned.
		// Palette changes ride selectAlpha, the indicator's own fade, so opening
		// the settings page cannot leave white glyphs on a vanishing gradient.
		plateTint := mixColor(featureTintPlate(index, st),
			myui.Color{R: 255, G: 255, B: 255, A: 56}, selectAlpha)
		myui.Row(c).Size(featurePlateSize, featurePlateSize).Radius(st.RadiusPill).
			Justify(myui.Center).AlignItems(myui.Center).
			Background(plateTint).Children(func() {
			if icon := tool.Icon(); icon != nil {
				myui.Icon(c, icon).Width(17).Height(17).
					TextColor(mixColor(featureTint(index, st), myui.RGB(255, 255, 255), selectAlpha))
			}
		})
		// The label is always built, and only fades as the rail narrows. Building
		// it conditionally shifted the row's own children between frames too.
		label := myui.Text(c, tool.Name()).FontSize(13.5).NoWrap().Opacity(labelAlpha).
			TextColor(mixColor(st.Text, myui.RGB(255, 255, 255), selectAlpha))
		// Weight has no in-between value; switch it once the gradient is already
		// half faded, so no bold text is left standing alone on the first frame.
		if selectAlpha > 0.5 {
			label.FontWeight(600)
		} else {
			label.FontWeight(500)
		}
	})
	if item.Clicked() {
		a.Active = index
		a.ShowSettings = false
	}
}

func (a *App) railFooterItem(c *myui.Context, st style, key string, icon *myui.SVG, text string, active bool, labelAlpha float32, onClick func()) {
	item := myui.ButtonBase(c).Key(key).FillWidth().Height(44).Gap(11).AlignItems(myui.Center).
		Radius(st.RadiusControl).Cursor(myui.CursorPointer).
		Label(text).FocusRingColor(st.Brand).FocusRingOffset(1).
		Transition(motion(springTap))
	// The same fixed alignment as the tool rows, so the footer's icons land on the
	// tool plates' axis and stay there while the rail animates.
	item = item.Justify(myui.Start).PaddingX(railItemPadding)

	// The settings row's highlight fades in on the shell curve while the tool
	// indicator fades out on the same curve. Switching it on instantly made two
	// blue rows appear at once for a frame, which read as the rail repainting.
	target := float32(0)
	if active {
		target = 1
	}
	highlight := target
	if !motionReduced {
		highlight = item.AnimateTo("footer-highlight", target, shellCurve)
	}
	switch {
	case item.Pressed():
		item.Background(st.DividerTop).Scale(0.99, 0.99)
	case item.Hovered() && highlight < 0.5:
		item.Background(st.Recessed)
	}
	item.Children(func() {
		// The gradient is an overlay so its opacity can be animated without
		// changing the element's identity or its layout box.
		myui.Row(c).Fill().Absolute().Radius(st.RadiusControl).ZIndex(-1).
			Key(key+"-highlight").Opacity(highlight).
			Gradient(st.Brand, st.BrandGradientTo, 90)
		myui.Row(c).Size(featurePlateSize, featurePlateSize).
			Justify(myui.Center).AlignItems(myui.Center).Children(func() {
			myui.Icon(c, icon).Width(19).Height(19).
				TextColor(mixColor(st.TextSub, myui.RGB(255, 255, 255), highlight))
		})
		label := myui.Text(c, text).FontSize(13.5).NoWrap().Opacity(labelAlpha).
			TextColor(mixColor(st.Text, myui.RGB(255, 255, 255), highlight))
		if highlight > 0.5 {
			label.FontWeight(600)
		} else {
			label.FontWeight(500)
		}
		if badge := a.updateBadge(); badge != "" && key == "rail-settings" {
			myui.Spacer(c).Grow(1)
			myui.Row(c).Opacity(labelAlpha).Children(func() {
				statusPill(c, badge, st.Brand)
			})
		}
	})
	if item.Clicked() {
		onClick()
	}
}

// inputCard is the middle column: the tool's output location, its preview and the
// large drop area that holds the file list once it has files.
func (a *App) inputCard(c *myui.Context, st style) {
	c.Key("input-card")
	card := myui.Column(c).Grow(1).FillHeight().Radius(st.RadiusCard).
		Background(st.Card).Border(1, st.Divider).
		Shadow(0, 2, 12, 0, myui.RGBA(0, 0, 0, 0.04)).Clip().Label("输入卡").
		Focusable().Bind(&a.previewFocus).
		OnShortcut(0, myui.KeyLeft, func() { a.movePreview(-1) }).
		OnShortcut(0, myui.KeyUp, func() { a.movePreview(-1) }).
		OnShortcut(0, myui.KeyRight, func() { a.movePreview(1) }).
		OnShortcut(0, myui.KeyDown, func() { a.movePreview(1) })
	a.previewMu.Lock()
	focus := a.previewFocusPending
	a.previewFocusPending = false
	a.previewMu.Unlock()
	if focus {
		a.previewFocus.Focus(c)
	}
	card.Children(func() {
		a.inputHeader(c, st)
		if img := a.Preview(); img != nil {
			// Padding on a wrapper keeps the preview inside the card's edges;
			// a margin on a growing element is ignored by the row layout. The
			// preview takes the room the drag target does not: it grows, so the
			// picture stays large while the drop area keeps its minimum. The key
			// is the tool index so a switch lifts the new frame in.
			myui.Row(c).FillWidth().Padding(16, 16, 0, 16).
				Key(fmt.Sprintf("preview-%d", a.Active)).
				Transition(motion(springSoft)).Children(func() {
				caption := ""
				if tool := a.ActiveTool(); tool != nil {
					caption = tool.PreviewCaption(a)
				}
				a.previewArea(c, st, img, caption)
			})
			myui.Row(c).FillWidth().Grow(1).Padding(16, 0, 16, 16).
				AlignItems(myui.Stretch).Children(func() {
				a.dropArea(c, st)
			})
			return
		}
		// The header is 52 DIP tall on its own, so the target needs only a hair of
		// space under it: the same 16 on all four sides left the top edge looking
		// empty.
		myui.Row(c).FillWidth().Grow(1).Padding(4, 16, 16, 16).
			AlignItems(myui.Stretch).Children(func() {
			a.dropArea(c, st)
		})
	})
}

func (a *App) inputHeader(c *myui.Context, st style) {
	myui.Row(c).FillWidth().Height(52).PaddingX(18).Gap(10).AlignItems(myui.Center).
		Children(func() {
			tool := a.ActiveTool()
			myui.Text(c, "输入项").FontSize(14).FontWeight(600).TextColor(st.Text)
			count := fmt.Sprintf("%d", a.Session.Len())
			myui.Row(c).Height(20).PaddingX(7).Radius(999).
				Justify(myui.Center).AlignItems(myui.Center).
				Background(st.Recessed).Children(func() {
				myui.Text(c, count).FontSize(11).FontWeight(600).TextColor(st.TextSub)
			})
			if tool != nil {
				myui.Row(c).Width(1).Height(16).Background(st.Divider).MarginX(4)
				if icon := tool.Icon(); icon != nil {
					myui.Icon(c, icon).Width(15).Height(15).TextColor(featureTint(a.Active, st))
				}
				myui.Text(c, tool.Name()).FontSize(12.5).FontWeight(500).TextColor(st.TextSub)
			}
			myui.Spacer(c).Grow(1)
			statusPill(c, a.statusLabel(), a.statusTint(st))
			a.ghostButton(c, st, "选择输出位置", lucide.FolderOpen(), a.pickOutputDirectory)
			if a.Session.Len() > 0 {
				a.ghostButton(c, st, "清空", lucide.Trash2(), func() {
					a.CancelImport()
					a.Session.Clear()
					a.SetPreview(nil)
					a.SetStatus("已清空")
				})
			}
		})
}

// ghostButton draws a secondary action with the same keyboard, pressed,
// disabled and loading semantics as the primary action.
func (a *App) ghostButton(c *myui.Context, st style, text string, icon *myui.SVG, onClick func()) myui.Element {
	return a.ghostButtonState(c, st, text, icon, false, false, onClick)
}

func (a *App) ghostButtonState(c *myui.Context, st style, text string, icon *myui.SVG, disabled, loading bool, onClick func()) myui.Element {
	unavailable := disabled || loading
	btn := myui.ButtonBase(c).Height(32).PaddingX(11).Radius(st.RadiusPill).Gap(5).
		AlignItems(myui.Center).Cursor(myui.CursorPointer).Label(text).
		Transition(motion(springTap)).Border(1, st.Divider).Background(st.Card)
	switch {
	case unavailable:
		btn.Disabled(true).Cursor(myui.CursorNotAllowed).Background(st.Recessed)
	case btn.Pressed():
		btn.Background(st.DividerTop).Border(1, st.BrandDeep).Scale(0.98, 0.98)
	case btn.Hovered():
		btn.Background(st.Recessed).Border(1, st.Brand)
	}
	foreground := st.Text
	secondary := st.TextSub
	if unavailable {
		foreground = st.TextDim
		secondary = st.TextDim
	}
	btn.Children(func() {
		if loading {
			loadingIcon(c, 13, secondary)
		} else if icon != nil {
			myui.Icon(c, icon).Width(13).Height(13).TextColor(secondary)
		}
		myui.Text(c, text).FontSize(12).TextColor(foreground).NoWrap()
	})
	if !unavailable && btn.Clicked() && onClick != nil {
		onClick()
	}
	return btn
}

// previewArea shows the image the tool produced, as the original watermark page
// showed a live preview above its drop zone. The image sits on a chequerboard so
// a transparent PNG reads as transparent instead of as a white picture.
func (a *App) previewArea(c *myui.Context, st style, img image.Image, caption string) {
	box := myui.Column(c).FillWidth().Grow(1).Padding(12).Gap(8).
		Radius(st.RadiusPanel).Background(st.Recessed).Label("图片预览").
		OnClick(func() { a.previewFocus.Focus(c) })
	box.Children(func() {
		myui.Row(c).FillWidth().AlignItems(myui.Center).Gap(6).Children(func() {
			myui.Icon(c, lucide.Image()).Width(13).Height(13).TextColor(st.TextDim)
			myui.Text(c, "预览").FontSize(11).FontWeight(500).TextColor(st.TextSub)
			a.previewMu.Lock()
			loading := a.previewLoading
			a.previewMu.Unlock()
			if loading {
				myui.Text(c, "加载中…").FontSize(11).TextColor(st.TextDim)
			}
			if caption != "" {
				myui.Row(c).Width(1).Height(10).Background(st.Divider).MarginX(2)
				myui.Text(c, caption).FontSize(11).TextColor(st.TextDim).SingleLine()
			}
			myui.Spacer(c).Grow(1)
			b := img.Bounds()
			myui.Text(c, fmt.Sprintf("%d × %d", b.Dx(), b.Dy())).FontSize(11).
				TextColor(st.TextDim)
		})
		// Contain keeps the whole picture visible instead of cropping it to the
		// box, which is what a preview must do.
		myui.Image(c, myui.NewBitmap(img)).FillWidth().Grow(1).
			Radius(st.RadiusControl).Fit(myui.Contain)
	})
}

// dropArea is the large dashed target: it holds the empty hint, or the file rows
// once the tool has inputs.
func (a *App) dropArea(c *myui.Context, st style) {
	items := a.Session.Snapshot()
	// The frame is drawn by hand: the framework's dashes are 3:1 and thin out at
	// the corners, which on a panel this tall reads as a broken line.
	area := myui.Column(c).FillWidth().Grow(1).Gap(10).Padding(18).
		Radius(st.RadiusPanel).Label("拖放区").Transition(motion(springSoft))
	dragging := area.FileDragOver()
	if len(items) > 0 || dragging {
		area.Border(1, st.Divider)
	}
	if len(items) > 0 && !dragging {
		// With files in the list the target turns into a plain panel: a dashed
		// frame around a scrollable list reads as a placeholder that was never
		// filled, which is the opposite of what it holds.
		area.Background(st.Recessed)
	}
	if dragging {
		area.Background(st.BrandSoft).Border(1, st.Brand)
	}
	if len(items) == 0 && !dragging {
		area.Draw(func(p *myui.Painter, r myui.Rect) {
			dashedFrame(p, myui.Rect{X: r.X + 0.5, Y: r.Y + 0.5, W: r.W - 1, H: r.H - 1},
				st.RadiusPanel, 1.5, st.Divider)
		})
	}
	area.Children(func() {
		if len(items) == 0 {
			empty := myui.Column(c).Fill().Gap(10).AlignItems(myui.Center).
				Justify(myui.Center)
			empty.Children(func() {
				myui.Row(c).Size(64, 64).Radius(999).Justify(myui.Center).
					AlignItems(myui.Center).Background(st.BrandSoft).Children(func() {
					myui.Icon(c, lucide.Upload()).Width(28).Height(28).TextColor(st.Brand)
				})
				myui.Text(c, "点击选择文件，或拖入文件夹").FontSize(13.5).
					FontWeight(500).TextColor(st.TextSub)
				myui.Row(c).Gap(8).PaddingY(6).Children(func() {
					a.ghostButton(c, st, "添加文件", lucide.Plus(), a.pickFiles)
					a.ghostButton(c, st, "添加文件夹", lucide.FolderOpen(), a.pickDirectory)
				})
			})
			return
		}
		// With files present the header row carries the count and the add action;
		// the rows below need no repeated hint because right-click removal is
		// discoverable on hover.
		myui.Row(c).FillWidth().Gap(8).AlignItems(myui.Center).PaddingY(2).Children(func() {
			myui.Text(c, fmt.Sprintf("%d 个文件", len(items))).FontSize(11).
				FontWeight(500).TextColor(st.TextDim)
			myui.Spacer(c).Grow(1)
			a.ghostButton(c, st, "添加更多", lucide.Plus(), a.pickFiles)
		})
		myui.Scroll(c).FillWidth().Grow(1).Children(func() {
			myui.Column(c).FillWidth().Gap(2).Children(func() {
				for i := range items {
					a.fileRow(c, st, &items[i])
				}
			})
		})
	})
	if files := area.DroppedFiles(); len(files) > 0 {
		a.AddPaths(files)
	}
	if len(items) == 0 && area.Clicked() {
		a.pickFiles()
	}
}

// settingsCard is the right column: the tool's parameters and the action bar.
func (a *App) settingsCard(c *myui.Context, st style, tool Tool) {
	card := myui.Column(c).Width(420).FillHeight().Radius(st.RadiusCard).
		Background(st.Card).Border(1, st.Divider).
		Shadow(0, 2, 12, 0, myui.RGBA(0, 0, 0, 0.04)).Clip().Label("参数卡")
	card.Children(func() {
		// The panel is keyed by tool so a switch lifts the new parameters in
		// rather than swapping the text in place.
		myui.Scroll(c).FillWidth().Grow(1).
			Key(fmt.Sprintf("panel-%d", a.Active)).
			Transition(motion(springSoft)).Children(func() {
			myui.Column(c).FillWidth().Padding(18).Gap(14).Children(func() {
				tool.Panel(c, a)
			})
		})
		a.actionBar(c, st)
	})
}

// operationFeedback gives App.Status a visible, accessible outlet. The icon and
// wording carry the state as well as the colour; long errors stay available as
// a tooltip instead of stretching the action bar.
func (a *App) operationFeedback(c *myui.Context, st style) {
	text := a.StatusLine()
	if text == "" {
		return
	}
	tint := st.TextSub
	icon := lucide.Info()
	busy := strings.Contains(text, "正在")
	switch {
	case strings.Contains(text, "失败"):
		tint = st.Danger
		icon = lucide.CircleX()
	case busy:
		tint = st.Brand
	case strings.Contains(text, "没有找到") || strings.Contains(text, "先添加") || strings.Contains(text, "为空"):
		tint = st.Warning
		icon = lucide.TriangleAlert()
	case strings.Contains(text, "完成") || strings.Contains(text, "已保存") ||
		strings.Contains(text, "已更新") || strings.Contains(text, "已生成") ||
		strings.Contains(text, "已添加") || strings.Contains(text, "已清空"):
		tint = st.Success
		icon = lucide.CircleCheck()
	}
	row := myui.Row(c).FillWidth().MinHeight(30).Padding(6, 8, 6, 8).Gap(7).
		AlignItems(myui.Center).Radius(st.RadiusPill).Background(st.Recessed).
		Role(myui.RoleStatus).Label(text).Tooltip(text)
	row.Children(func() {
		if busy {
			loadingIcon(c, 13, tint)
		} else {
			myui.Icon(c, icon).Width(13).Height(13).TextColor(tint)
		}
		myui.Text(c, text).FontSize(11.5).TextColor(tint).SingleLine()
	})
}

// actionBar is the primary action, the cancel action and the progress.
func (a *App) actionBar(c *myui.Context, st style) {
	percent, caption := a.ProgressState()
	running := a.IsRunning() || a.IsImporting()

	myui.Column(c).FillWidth().Gap(10).Padding(14, 18, 16, 18).Children(func() {
		if running || percent > 0 {
			myui.Column(c).FillWidth().Gap(5).Children(func() {
				track := myui.Row(c).FillWidth().Height(5).Radius(999).Background(st.Recessed)
				track.Children(func() {
					myui.Row(c).Height(5).Radius(999).
						Gradient(st.Brand, st.BrandGradientTo, 90).
						WidthPercent(maxFloat(2, minFloat(100, percent)))
				})
				if caption != "" {
					myui.Text(c, caption).FontSize(11).TextColor(st.TextDim).SingleLine()
				}
			})
		}
		a.operationFeedback(c, st)

		myui.Row(c).FillWidth().Gap(8).AlignItems(myui.Center).Children(func() {
			if running {
				run := myui.ButtonBase(c).Grow(1).Height(46).Radius(st.RadiusControl).Gap(8).
					Justify(myui.Center).AlignItems(myui.Center).Cursor(myui.CursorPointer).
					Label("停止处理").Transition(motion(springTap)).Border(1, st.Divider)
				switch {
				case run.Pressed():
					run.Background(st.DividerTop).Border(1, st.Danger).Scale(0.985, 0.985)
				case run.Hovered():
					run.Background(myui.RGBA(st.Danger.R, st.Danger.G, st.Danger.B, 0.10)).
						Border(1, st.Danger)
				default:
					run.Background(st.Recessed)
				}
				run.Children(func() {
					myui.Icon(c, lucide.X()).Width(16).Height(16).TextColor(st.Danger)
					myui.Text(c, "停止处理").FontSize(14).FontWeight(600).TextColor(st.Text)
				})
				if run.Clicked() {
					a.Cancel()
				}
				return
			}
			primaryButton(c, st, "开始处理", lucide.Play(), a.Session.Len() == 0, false, a.Start).
				Grow(1).Height(46)
			if a.Session.Len() > 0 {
				a.ghostButton(c, st, "取消", lucide.X(), a.Cancel)
			}
		})
	})
}

// ActiveTool returns the tool whose page is showing.
func (a *App) ActiveTool() Tool {
	if a.Active < 0 || a.Active >= len(a.Tools) {
		return nil
	}
	return a.Tools[a.Active]
}

// Invalidate asks the window for a new frame.
func (a *App) Invalidate() {
	if a.Window != nil {
		a.Window.Invalidate()
	}
}

func maxFloat(a, b float32) float32 {
	if a > b {
		return a
	}
	return b
}

func minFloat(a, b float32) float32 {
	if a < b {
		return a
	}
	return b
}

// hexOf turns a UI colour into the "#RRGGBB" form the engines take.
func hexOf(c myui.Color) string {
	return fmt.Sprintf("#%02X%02X%02X", c.R, c.G, c.B)
}

// colorOf parses "#RRGGBB" into a UI colour.
func colorOf(value string) myui.Color {
	value = strings.TrimSpace(value)
	if value == "" {
		return myui.RGB(0, 122, 255)
	}
	return myui.Hex(value)
}
