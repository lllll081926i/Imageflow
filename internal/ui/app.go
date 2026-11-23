// Package ui is the Lobe MyGo native interface of ImageFlow.
package ui

import (
	"context"
	"fmt"
	"image"
	"strings"
	"sync"
	"sync/atomic"
	"time"

	"github.com/egoist/mygo"
	myui "github.com/egoist/mygo/ui"
	"github.com/lobehub/lobe-mygo/pkg/lucide"
	"github.com/lobehub/lobe-mygo/pkg/theme"

	"github.com/lllll081926i/imageflow-go/internal/appcache"
	"github.com/lllll081926i/imageflow-go/internal/batch"
	"github.com/lllll081926i/imageflow-go/internal/paths"
	"github.com/lllll081926i/imageflow-go/internal/settings"
	"github.com/lllll081926i/imageflow-go/internal/thumbs"
)

// Tool is one page of the workbench.
type Tool interface {
	// ID is the tool's stable identifier. It survives a rename, a reorder and an
	// insertion, and it is what the saved sidebar order stores: an index would be
	// reinterpreted as soon as a page is added.
	ID() string
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

// previewWidget is implemented by a tool whose preview is more than one picture:
// a collage, for instance, composes several inputs and receives pointer input.
// The input card asks for it instead of drawing the shared single-file preview.
type previewWidget interface {
	// PreviewPanel draws the tool's own preview inside the input card. The
	// Element it builds is the whole preview area; the tool sizes it itself.
	PreviewPanel(c *myui.Context, a *App)
}

// App is the whole interface state.
//
// The batch goroutine updates Status, Progress, ProgressText and Running from
// outside the UI thread, so those four are guarded by mu; the view reads them
// through the accessors below.
type App struct {
	clipboardCache appcache.Session
	cacheJob       studioJob[appcache.Report]
	cacheVersion   uint64
	cacheNextScan  time.Time
	cacheCleaning  atomic.Bool
	mu             sync.Mutex
	Session        *batch.Session
	Thumbs         *thumbs.Cache
	Window         *mygo.Window

	Active       int
	ShowSettings bool
	// ShowHome draws the landing page instead of a tool. It is the app's start
	// state: twenty-five rows in a column is a list to read, not a place to start.
	ShowHome bool
	// searchQuery 保存首页工具搜索，不改变侧栏导航。
	searchQuery string
	// collapsedGroups holds the sidebar groups the user folded away, keyed by
	// group. A missing key means open.
	collapsedGroups map[string]bool
	Tools           []Tool

	Status        string
	Progress      float32
	ProgressText  string
	Running       bool
	canvasOutputs map[string]string
	rowResultTool string

	// planMu guards planned, the output paths already reserved by the batch in
	// flight, so two rows never resolve to the same file name.
	planMu     sync.Mutex
	planned    paths.OutputReservations
	fileThumbs fileThumbnailCache

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
	fileLayout          int
	fileSelection       int
	studioPreview       studioJob[image.Image]
	studioOutput        image.Image
	studioSourceVersion uint64
	studioToolID        string
	studioPreviewError  string
	studioPreviewBusy   bool

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
	// pendingDone holds the completion callbacks queued with those paths. They run
	// on the UI thread after the fields are applied, so a page starts its work on
	// the value it can actually see.
	pendingDone []func()

	// collage saves the拼图 page's parameters in the background. The UI thread
	// hands a snapshot to collageMu, and a single timer writes it, so a slider
	// drag does not turn into one file write per frame.
	collageMu      sync.Mutex
	collagePending *settings.Collage
	collageTimer   *time.Timer
	collageSig     string

	// railOrder is the sidebar's row order: position → index into Tools. It is a
	// permutation, so a tool keeps its identity, its page and its selection while
	// only its position changes. Zero length means "the natural order".
	railOrder []int
	// railOrderIDs is the same order as stable identifiers. It is the persistent
	// form; railOrder is the per-frame index form derived from it.
	railOrderIDs []string
	// railDragPos and railDragFromPos are ordinals among the *visible* tool rows.
	// A group header is not a row: a drag moves a tool past other tools, and the
	// group standing between them is a label, not a slot.
	railDragPos     int
	railDragFromPos int
	// railRevealTool asks the rail to scroll one row into view on the next frame.
	// Selecting a page from the landing page or from a search result must bring its
	// row into sight, otherwise the sidebar shows a selection nobody can see.
	railRevealTool int
	// windowWidth is the frame's window width, read once per frame. The rail's
	// auto-collapse rule reads it, so the app answers the question instead of every
	// caller measuring the window again.
	windowWidth float32
	// railRowsCache and railRowsKeyValue memoise the row table for one frame shape.
	railRowsCache    []railRow
	railRowsKeyValue string
	// The long-press and drag state. railPressTool is the tool whose row is being
	// pressed; railDragTool is the one lifted, which is set once the press has
	// lasted long enough to be a drag.
	railPressTool  int
	railPressAt    time.Time
	railPressMoved float32
	railPressGrabY float32
	railPressListY float32
	railDragTool   int
	railDragFrom   int
	railDragTo     int
	railDragY      float32
	// railSuppressClick swallows the click the engine reports for the release that
	// ended a reorder drag, so dropping a row does not also switch to its page.
	railSuppressClick bool
}

// applyPendingSettings copies the folder history a background goroutine recorded
// into the editable settings. It runs on the UI thread and leaves unsaved edits to
// the other fields alone.
func (a *App) applyPendingSettings() {
	// Drain dialog results on the UI thread before the controls read their
	// bound fields, so no background goroutine writes view state directly.
	a.pickerMu.Lock()
	picked := len(a.pendingPicker) > 0
	for dst, path := range a.pendingPicker {
		*dst = path
	}
	clear(a.pendingPicker)
	done := a.pendingDone
	a.pendingDone = nil
	a.pickerMu.Unlock()
	// The completion callbacks run after every field was applied, so a page that
	// reloads a picture on selection reads the path it just received.
	for _, fn := range done {
		fn()
	}
	// A new background picture changes the collage preview, which is keyed by its
	// inputs: dropping the key here is what makes the next frame recompose.
	if picked {
		if tool, ok := a.ActiveTool().(*CollageTool); ok && tool != nil {
			tool.previewKey = ""
		}
	}
	// Slot replacements chosen by a double click come through their own queue on
	// the tool, because they are keyed by slot rather than by a bound field.
	if tool, ok := a.ActiveTool().(*CollageTool); ok && tool != nil {
		a.pickerMu.Lock()
		for cell, path := range tool.pendingCells {
			if tool.overrides == nil {
				tool.overrides = map[int]string{}
			}
			tool.overrides[cell] = path
			tool.previewKey = ""
		}
		clear(tool.pendingCells)
		a.pickerMu.Unlock()
	}

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

// collageSignature summarises a collage snapshot, so an unchanged frame does not
// schedule a settings write.
func collageSignature(c settings.Collage) string {
	var b strings.Builder
	fmt.Fprintf(&b, "%d|%s|%v|%d|%d|%v|", c.GroupCount, c.PresetID, c.UseCustom, c.GridCols, c.GridRows, c.GridParents)
	fmt.Fprintf(&b, "%s|%s|%d|%d,%d,%d,%d|%d|%d|%d|%d|", c.AspectID, c.CustomRatio, c.LongEdge,
		c.MarginTop, c.MarginRight, c.MarginBottom, c.MarginLeft, c.Seam, c.Gap, c.Radius, c.Fit)
	fmt.Fprintf(&b, "%s|%d|%d|%d|%d|", c.Background, c.BackgroundDim, c.Mode, c.Format, c.Quality)
	fmt.Fprintf(&b, "%v|%s|%d|%.3f|%.1f|%d|", c.Watermark, c.WatermarkText, c.WatermarkSize,
		c.WatermarkOpacity, c.WatermarkAngle, c.WatermarkDensity)
	for _, m := range c.Marks {
		fmt.Fprintf(&b, "%+v;", m)
	}
	return b.String()
}

// SetStatus records a status line from any goroutine.
func (a *App) SetStatus(text string) {
	a.mu.Lock()
	a.Status = text
	a.mu.Unlock()
}

// SaveCollage schedules a write of the collage page's parameters.
//
// The values are copied on the UI thread and written by a single debounced timer
// 600 ms later: every slider drag or drag of a cell changes them dozens of times a
// second, and one file write per frame would make a cheap edit expensive.
func (a *App) SaveCollage(c settings.Collage) {
	a.collageMu.Lock()
	// A signature check keeps a frame that changed nothing from scheduling a
	// write; equality is by value, so two identical snapshots skip the timer.
	sig := collageSignature(c)
	if sig == a.collageSig {
		a.collageMu.Unlock()
		return
	}
	a.collageSig = sig
	copyOf := c
	copyOf.Marks = append([]settings.CollageMark(nil), c.Marks...)
	copyOf.GridParents = append([]int(nil), c.GridParents...)
	a.collagePending = &copyOf
	if a.collageTimer != nil {
		a.collageTimer.Stop()
	}
	a.collageTimer = time.AfterFunc(600*time.Millisecond, a.flushCollage)
	a.collageMu.Unlock()
}

// flushCollage writes the pending collage parameters, if any.
func (a *App) flushCollage() {
	a.collageMu.Lock()
	pending := a.collagePending
	a.collagePending = nil
	a.collageTimer = nil
	a.collageMu.Unlock()
	if pending == nil {
		return
	}
	value := *pending
	if err := a.Session.UpdateSettings(func(next *settings.Settings) {
		next.Collage = value
	}); err != nil {
		a.SetStatus("保存拼图设置失败：" + err.Error())
		a.Invalidate()
		return
	}
	a.collageMu.Lock()
	a.settingsBase.Collage = value
	a.Settings.Collage = value
	a.collageMu.Unlock()
}

// FlushCollage writes the pending collage parameters now, for shutdown.
func (a *App) FlushCollage() {
	a.collageMu.Lock()
	if a.collageTimer != nil {
		a.collageTimer.Stop()
	}
	a.collageMu.Unlock()
	a.flushCollage()
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
	if tool := a.ActiveTool(); tool != nil && tool.ID() == a.studioToolID && a.studioSourceVersion == a.previewVersion && a.studioOutput != nil {
		return a.studioOutput
	}
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
		// The app opens on the landing page: twenty-five rows in a column is a list
		// to read, not a place to start, and a first-time user has no reason to know
		// which row they want.
		ShowHome: true,
		// -1 means "no drag": the zero value would name tool 0 as the row being
		// carried, and every frame would draw it lifted.
		railDragTool:   -1,
		railDragFrom:   -1,
		railDragTo:     -1,
		railPressTool:  -1,
		railRevealTool: -1,
	}
	a.Tools = []Tool{
		&ConvertTool{},
		&ResizeTool{},
		&CropTool{},
		&GridTool{},
		&LayoutTool{},
		&CollageTool{},
		&StitchTool{},
		&WatermarkTool{},
		&PaletteTool{},
		&IconTool{},
		&PDFTool{},
		&GIFTool{},
		&RichTextTool{},
		&CompressTool{},
		// The pages below are the增量 deliveries of the v2.0 plan: they append to
		// the existing list so no existing row moves until the user reorders it.
		&EditTool{},
		&CollageStudioTool{},
		&AnnotateTool{},
		&ScreenshotTool{},
		&GradeTool{},
		&FilterTool{},
		&CompareTool{},
		&AnalysisTool{},
		&PixelTool{},
		&EmoteTool{},
		&UpscaleTool{engine: s.AIEnginePath, models: s.AIModelsDir},
	}
	a.settingsBase = a.Session.SettingsSnapshot()
	// The row order is validated against the tool list, so it is seeded only after
	// that list exists: seeding it first repaired every stored permutation to the
	// natural order.
	//
	// A file written by v3.1.0 stores legacy indexes; those are migrated to
	// identifiers once, here, so the user's arrangement survives the upgrade and
	// every later save writes identifiers only.
	if len(s.RailOrderIDs) == 0 && len(s.RailOrder) > 0 {
		a.railOrderIDs = migrateRailOrderIDs(s.RailOrder)
		a.Settings.RailOrderIDs = append([]string(nil), a.railOrderIDs...)
		a.Settings.RailOrder = nil
	} else {
		a.railOrderIDs = append([]string(nil), s.RailOrderIDs...)
	}
	// The folded sections are restored before the first frame, so the sidebar opens
	// the way the user left it.
	a.collapsedGroups = map[string]bool{}
	for _, key := range s.RailCollapsedGroups {
		if _, known := groupByKey(key); known {
			a.collapsedGroups[key] = true
		}
	}
	a.ensureRailOrder()
	// Fill each tool's defaults up front, so the panel and the batch runner agree
	// even when the page has not been drawn yet.
	for _, tool := range a.Tools {
		if init, ok := tool.(interface{ ensure() }); ok {
			init.ensure()
		}
	}
	// The collage page's parameters live in the settings file; seed the tool from
	// them so a restored template is what the first frame draws and exports.
	if s.Collage.GroupCount > 0 {
		if tool, ok := a.toolByNameInternal("图片拼图").(*CollageTool); ok && tool != nil {
			tool.applyPersisted(s.Collage)
		}
	}
	return a
}

// Tool identifiers. A tool's identifier is permanent: it is what a saved sidebar
// order stores, so renaming a page or renaming a label never reinterprets a
// stored arrangement.
const (
	ToolConvert    = "convert"
	ToolResize     = "resize"
	ToolCrop       = "crop"
	ToolGrid       = "grid"
	ToolLayoutPage = "layout"
	ToolCollage    = "collage"
	ToolStitch     = "stitch"
	ToolWatermark  = "watermark"
	ToolPalette    = "palette"
	ToolIcon       = "icon"
	ToolPDF        = "pdf"
	ToolGIF        = "gif"
	ToolRichText   = "richtext"
	ToolCompress   = "compress"

	ToolEdit       = "edit"
	ToolStudio     = "studio"
	ToolAnnotate   = "annotate"
	ToolScreenshot = "screenshot"
	ToolGrade      = "grade"
	ToolFilter     = "filter"
	ToolCompare    = "compare"
	ToolAnalysis   = "analysis"
	ToolPixel      = "pixel"
	ToolEmote      = "emote"
	ToolUpscale    = "upscale"
)

// toolByNameInternal finds a registered tool by its label without a *testing.T.
func (a *App) toolByNameInternal(name string) Tool {
	for _, tool := range a.Tools {
		if tool.Name() == name {
			return tool
		}
	}
	return nil
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
		// One read per frame: the layout below and the rail's width rule both need it.
		a.windowWidth, _ = c.Size()
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
		if a.railCollapsedAuto() {
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
			// The reveal request is consumed by the rail's own build: one frame asks for
			// the scroll, and the next frame's layout is already there.
			a.railRevealTool = -1
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
				if a.ShowHome {
					a.homePage(c, st)
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
	if a.ShowHome {
		return "page-home"
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

// railCollapsedAuto reports whether a window of the given width renders the rail as
// the icon strip. The rule lives here so a test can assert the behaviour instead of
// copying the threshold.
func (a *App) railCollapsedAuto() bool {
	return a.railCollapsed || (a.windowWidth > 0 && a.windowWidth < railAutoCollapseWidth)
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
	// railAutoCollapseWidth is the window width below which the rail shows as the
	// icon strip: 216 + 420 of cards leaves nothing for the preview under this.
	railAutoCollapseWidth = 1080
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
		myui.Scroll(c).Key("rail-scroll").FillWidth().Grow(1).Scrollbars(false).Children(func() {
			a.railList(c, st, labelAlpha)
		})

		myui.Column(c).FillWidth().Gap(2).PaddingY(8).Children(func() {
			myui.Row(c).FillWidth().Height(1).Background(st.Divider).Margin(0, 4, 6, 4)
			// 首页 returns to the landing page. It sits with the settings row because
			// both are shell destinations rather than tools.
			a.homeEntry(c, st, labelAlpha)
			// 全局设置 toggles: selecting it while the settings page shows returns
			// to the tool page, which is why the settings header carries no second
			// back button.
			a.railFooterItem(c, st, "rail-settings", lucide.Settings(), "全局设置", a.ShowSettings, labelAlpha, func() {
				a.ShowSettings = !a.ShowSettings
				a.ShowHome = false
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

// ObserveInput (not HandleInput) is enough because the rows keep their normal click
// handling: the observer only tracks the gesture, and a plain click reaches the row
// as it always did. The gestures themselves have two phases — a press must hold
// still for railLongPress before anything lifts — which is why Dragged, available
// only as it moves, cannot express them.
// The pill is a single element that travels between rows instead of each row
// painting its own highlight. Per-row highlights cannot animate: the old row loses
// its gradient in the same frame the new row gains one, so the selection jumps. A
// single pill has an identity across frames, so its Top is a value the framework
// can interpolate.
//
// The list itself observes the pointer. A row cannot: the carried row is the thing
// being moved, so a pointer position measured inside it changes as the row slides,
// and a drag driven by those coordinates stalls or runs away. The list's own box
// does not move, so its local coordinates are a stable frame of reference for the
// whole gesture.
//
// The pill is always built, including on the settings page where it fades out.
// MyGo identifies an unkeyed child by its position among its siblings; removing
// the first child used to renumber all thirteen rows, so opening the settings
// rebuilt the entire rail, reset every row's hover/focus state and restarted the
// pill's travel from the top. Keeping the child (and keying every row) makes the
// list's shape constant across every page switch.
// railList draws the grouped navigation.
//
// A group header is a row of its own: clicking it folds the group away, which is
// what keeps twenty-five tools readable in a 216 DIP column. The rows inside a
// group follow the saved permutation, so a user's own order is preserved inside
// whatever section a tool belongs to.
//
// The selection pill is placed at the active tool's planned row, not at a computed
// pitch times an index: a header and a group gap break the fixed pitch, so the
// list's own row table is the only source of truth for a y coordinate.
func (a *App) railList(c *myui.Context, st style, labelAlpha float32) {
	rows := a.railRows()
	visible := !a.ShowSettings && !a.ShowHome

	list := myui.Column(c).FillWidth().Gap(railRowGap)
	list.Children(func() {
		// The pill is always built, even with no active row to point at: MyGo
		// identifies an unkeyed child by its position among its siblings, so
		// removing the first child renumbers every row after it.
		pill := myui.Row(c).FillWidth().Height(railRowHeight).
			Radius(st.RadiusControl).Absolute().Top(0).Left(0).ZIndex(-1).
			Key("rail-pill").Gradient(st.Brand, st.BrandGradientTo, 90).
			Shadow(0, 4, 12, 0, st.BrandGlow).Label("选中指示")
		target, alpha := float32(0), float32(0)
		if visible {
			if y := a.railToolRowY(a.Active); y >= 0 {
				target, alpha = y, 1
			}
		}
		y := target
		if !motionReduced {
			y = pill.AnimateTo("rail-pill-top", target, railPillCurve)
			alpha = pill.AnimateTo("rail-pill-alpha", alpha, shellCurve)
		}
		pill.Top(y).Opacity(alpha)

		// Header and rows are laid out in one flow, so the y the pill uses and the
		// y the layout produces stay the same number. A header's own height is
		// written explicitly, which is what makes the two agree across the group
		// gap as well.
		for _, row := range rows {
			if row.Header {
				a.railGroupHeader(c, st, row.Group, labelAlpha)
				continue
			}
			tool := a.Tools[row.Tool]
			rowAlpha := float32(0)
			if row.Tool == a.Active {
				rowAlpha = alpha
			}
			a.railItem(c, st, row, tool, labelAlpha, rowAlpha)
		}
	})
}

// railGroupHeader draws one section heading: its glyph, its label, and a chevron
// that says whether the section is open.
//
// 收起侧栏时不构建标题或分组占位。
func (a *App) railGroupHeader(c *myui.Context, st style, key string, labelAlpha float32) {
	group, ok := groupByKey(key)
	if !ok {
		return
	}
	open := a.groupExpanded(key)
	if labelAlpha <= 0.02 {
		return
	}
	row := myui.ButtonBase(c).Key("rail-group-" + key).FillWidth().
		Height(railHeaderHeight).Gap(6).AlignItems(myui.Center).
		Radius(st.RadiusPill).Cursor(myui.CursorPointer).
		Label(group.Label).Expanded(open).FocusRingColor(st.Brand).FocusRingOffset(1)
	row = row.Justify(myui.Start).PaddingX(railItemPadding)
	if row.Hovered() {
		row.Background(st.Recessed)
	}
	if !open && !a.ShowHome && !a.ShowSettings && a.ActiveTool() != nil && toolGroupKey(a.ActiveTool()) == key {
		row.Background(st.BrandSoft)
	}
	row.Opacity(labelAlpha)
	row.Children(func() {
		myui.Icon(c, group.Icon()).Width(12).Height(12).TextColor(st.TextDim)
		myui.Text(c, group.Label).FontSize(11).FontWeight(600).
			LetterSpacing(0.2).TextColor(st.TextDim).NoWrap()
		myui.Spacer(c).Grow(1)
		icon := lucide.ChevronDown()
		if !open {
			icon = lucide.ChevronRight()
		}
		myui.Icon(c, icon).Width(12).Height(12).TextColor(st.TextDim)
	})
	if row.Clicked() {
		a.toggleGroup(key)
	}
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

// railItem draws one tool row.
//
// The rows keep the flow layout that aligns their plates to one axis, and a
// reorder moves them with a translation: a carried row follows the pointer, is
// scaled up and takes a shadow, which is the only cue that says the pointer is now
// carrying it rather than pressing it. The gesture itself lives on the list
// (railPointerInput), so it has a coordinate frame that does not move.
func (a *App) railItem(c *myui.Context, st style, row railRow, tool Tool, labelAlpha, selectAlpha float32) {
	toolIndex := row.Tool
	rest := row.Y
	carried := a.railDragTool == toolIndex
	// A press outside the open sections can no longer be carried: the row it was
	// pointing at is not visible, so the list has no slot to move it to.
	visible := carried || a.railToolRowY(toolIndex) >= 0
	if !visible {
		return
	}
	active := a.Active == toolIndex && !a.ShowSettings && !a.ShowHome
	item := myui.ButtonBase(c).Key(railRowKey(toolIndex) + "-row").
		FillWidth().Height(railRowHeight).Gap(11).
		AlignItems(myui.Center).Radius(st.RadiusControl).
		Cursor(myui.CursorPointer).Label(tool.Name()).
		FocusRingColor(st.Brand).FocusRingOffset(1).
		Transition(motion(springTap))
	if labelAlpha <= 0.02 {
		item.Tooltip(tool.Name())
	}

	// Where the row belongs, and where the drag has taken it. Positions are
	// ordinals among the visible tool rows: a group header is a label, not a slot,
	// so it never counts as a step of travel.
	target := rest
	displaced := false
	switch {
	case carried:
		target = a.railDragY
	case a.railDragTool >= 0:
		from, to := a.railDragFromPos, a.railDragTo
		pos := a.railToolOrdinal(toolIndex)
		pitch := float32(railRowHeight + railRowGap)
		switch {
		case pos < 0:
		case pos == from:
			// The carried row's own gap travels to its future slot.
			target = rest + float32(to-from)*pitch
			displaced = true
		case from < to && pos > from && pos <= to:
			// Rows between the old and the new slot step up one place.
			target = rest - pitch
			displaced = true
		case from > to && pos >= to && pos < from:
			// Rows between step down one place.
			target = rest + pitch
			displaced = true
		}
	}

	top := target
	if !motionReduced {
		switch {
		case carried:
			// A spring following a moving target oscillates, so the carried row
			// trails the pointer on a short ease: it reads as a heavy card being
			// carried rather than as a lag.
			top = item.AnimateTo(railRowKey(toolIndex), target, railDragFollow)
		default:
			// A displaced row, and a row coming back, settle on the reorder spring:
			// a small overshoot is what says a row was pushed out of the way.
			top = item.AnimateTo(railRowKey(toolIndex), target, railReorderSpring)
		}
	}
	// A flow child is moved by a translation, not by Top: Top is for an absolutely
	// placed element, so setting it here would shift the row relative to the
	// column's padding box on top of the flow position and push every later row
	// further down. The translation is the offset from the row's own place, which
	// is what makes it animation-safe and layout-neutral.
	item.Translate(0, top-rest)

	if carried {
		// The carried row paints above the ones it passes over. Its lift is a
		// pose, applied below: a static Scale would snap between two sizes in
		// the frame the long press fires.
		item.ZIndex(3)
		item.Shadow(0, 8, 20, 0, myui.RGBA(0, 0, 0, 0.22))
	} else if displaced {
		item.ZIndex(1)
	}
	// The item keeps one alignment in every state: the plate sits at one x whether
	// the rail is open or collapsed, and only the label appears beside it.
	item = item.Justify(myui.Start).PaddingX(railItemPadding)
	// The row itself paints no selection: the pill behind it does, and it slides
	// between rows. Only the press and hover states belong to the row.
	switch {
	case carried:
		item.Background(st.Card).Border(1, st.Divider)
	case item.Pressed():
		item.Background(st.DividerTop)
	case item.Hovered() && !active:
		item.Background(st.Recessed)
	}
	if carried {
		// The lift is what says the pointer is carrying the row. It is a pose, on
		// the same short curve the row's translation uses.
		item = pressFeedback(item, true, 1.04)
	} else {
		item = pressFeedback(item, item.Pressed(), 0.99)
	}
	item.Children(func() {
		// The coloured plate is what makes the rail read as a set of tools rather
		// than a text menu: the original sidebar tinted a square behind every icon.
		// The plate keeps one fixed size in every state: growing it on selection
		// pushed the label right, so the active row read as misaligned.
		// Palette changes ride selectAlpha, the indicator's own fade, so opening
		// the settings page cannot leave white glyphs on a vanishing gradient.
		plateTint := mixColor(featureTintPlate(toolIndex, st),
			myui.Color{R: 255, G: 255, B: 255, A: 56}, selectAlpha)
		myui.Row(c).Size(featurePlateSize, featurePlateSize).Radius(st.RadiusPill).
			Justify(myui.Center).AlignItems(myui.Center).
			Background(plateTint).Children(func() {
			if icon := tool.Icon(); icon != nil {
				myui.Icon(c, icon).Width(17).Height(17).
					TextColor(mixColor(featureTint(toolIndex, st), myui.RGB(255, 255, 255), selectAlpha))
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
	// Activation stays on Clicked: the engine has already decided what counts as a
	// click (in bounds, no drag), so re-deriving it here would disagree with the
	// accessibility tree and with the tester. A drop sets railSuppressClick, which
	// swallows the click the engine reports for the release that ended the drag.
	a.railRowGesture(c, item, toolIndex)
	if a.railRevealTool == toolIndex {
		item.ScrollIntoView()
	}
	if item.Clicked() {
		if a.railSuppressClick {
			a.railSuppressClick = false
		} else {
			a.Active = toolIndex
			a.ShowSettings = false
			a.ShowHome = false
			a.searchQuery = ""
		}
	}
}

// railRowGesture runs one row's long press and drag while it is being built.
//
// It is called every frame the row exists, which is what makes a held press keep
// asking for frames until it either lifts or ends. The pointer's movement comes
// from Dragged, which reports the pointer's travel in the window's axes for this
// frame: a delta survives the row's own translation, where an absolute local
// position would feed the row's movement back into its target and stall.
func (a *App) railRowGesture(c *myui.Context, item myui.Element, toolIndex int) {
	item.HandleInput(func(ev myui.InputEvent) bool {
		switch ev.Kind {
		case myui.InputPointerDown:
			a.beginRailPress(c, toolIndex, ev.Y)
			// Not consumed: the engine still records the press, so a plain click
			// reaches the row as it always did.
			return false
		case myui.InputPointerUp:
			if a.railDragTool != toolIndex {
				a.resetRailPress()
				return false
			}
			// The row is dropped where it was carried to, not where the pointer
			// happened to be inside it.
			a.commitRailDrag()
			// A drop is not a click: the row travelled, so opening its page would
			// fight the gesture that just ended.
			a.railSuppressClick = true
			return true
		}
		return false
	})

	// Accumulate the pointer's travel while the press lasts.
	if a.railPressTool == toolIndex {
		if _, dy, ok := item.Dragged(); ok {
			a.railPressMoved += absFloat32(dy)
			if a.railDragTool == toolIndex {
				a.railDragY += dy
				// The travel is bounded by the visible tool rows, not by the whole
				// list: a group folded away has no slots to drop into.
				lastPos := a.railToolCount() - 1
				maxY := a.railDragYOfOrdinal(lastPos)
				minY := a.railDragYOfOrdinal(0)
				if a.railDragY < minY {
					a.railDragY = minY
				}
				if a.railDragY > maxY {
					a.railDragY = maxY
				}
				if next := a.railDragTargetFromPointer(); next != a.railDragTo {
					a.railDragTo = next
				}
			}
		}
		if item.Pressed() {
			a.noticeRailLift(c, true)
		}
	}
}

// railFooterItem draws one of the rail's own actions, below the tool list.
func (a *App) railFooterItem(c *myui.Context, st style, key string, icon *myui.SVG, text string, active bool, labelAlpha float32, onClick func()) {
	item := myui.ButtonBase(c).Key(key).FillWidth().Height(44).Gap(11).AlignItems(myui.Center).
		Radius(st.RadiusControl).Cursor(myui.CursorPointer).
		Label(text).FocusRingColor(st.Brand).FocusRingOffset(1).
		Transition(motion(springTap))
	if labelAlpha <= 0.02 {
		item.Tooltip(text)
	}
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
		item.Background(st.DividerTop)
	case item.Hovered() && highlight < 0.5:
		item.Background(st.Recessed)
	}
	item = pressFeedback(item, item.Pressed(), 0.99)
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
		Focusable().Bind(&a.previewFocus)
	// The arrow keys step through the file list, which is what the shared preview
	// shows. A tool that draws its own preview binds them itself: leaving these on
	// meant the arrows silently switched a preview nobody can see and the canvas
	// never received the nudge it advertised.
	tool := a.ActiveTool()
	if upscaleTool, ok := tool.(*UpscaleTool); ok {
		services := c.Services()
		card.OnShortcut(myui.Ctrl, myui.KeyV, func() { upscaleTool.pasteClipboard(a, services) })
	}
	if _, ownPreview := tool.(previewWidget); !ownPreview || a.Session.Len() == 0 {
		card = card.
			OnShortcut(0, myui.KeyLeft, func() { a.movePreview(-1) }).
			OnShortcut(0, myui.KeyUp, func() { a.movePreview(-1) }).
			OnShortcut(0, myui.KeyRight, func() { a.movePreview(1) }).
			OnShortcut(0, myui.KeyDown, func() { a.movePreview(1) })
	}
	a.previewMu.Lock()
	focus := a.previewFocusPending
	a.previewFocusPending = false
	a.previewMu.Unlock()
	if focus {
		a.previewFocus.Focus(c)
	}
	card.Children(func() {
		a.inputHeader(c, st)
		if toolPreviewMode(tool.ID()) == previewNone {
			myui.Row(c).FillWidth().Grow(1).MinHeight(0).Padding(4, 16, 16, 16).Clip().AlignItems(myui.Stretch).Children(func() {
				a.dropArea(c, st)
			})
			return
		}
		a.requestStudioPreview(tool)
		if a.studioPreviewError != "" && a.studioToolID == tool.ID() {
			warnAlert(c, "预览失败："+a.studioPreviewError)
		}
		// A tool that composes several inputs draws its own preview area: the
		// single-picture preview below shows one file, which is the wrong question
		// for a collage. It is chosen by the tool's type, not by whether a picture
		// happens to be decoded: a failed decode must not drop the canvas. An empty
		// list keeps the shared dashed target: with nothing to compose, the canvas
		// would be a large empty frame next to a hint saying what to do.
		if widget, ok := tool.(previewWidget); ok && widget != nil && (a.Session.Len() > 0 || tool.ID() == ToolCompare) {
			myui.Row(c).FillWidth().Grow(1).MinHeight(0).Padding(16, 16, 0, 16).Clip().AlignItems(myui.Stretch).Label("预览边界").
				Key(fmt.Sprintf("preview-%d", a.Active)).
				Transition(motion(springSoft)).Children(func() {
				widget.PreviewPanel(c, a)
			})
			myui.Row(c).FillWidth().Padding(12, 16, 16, 16).
				AlignItems(myui.Stretch).Children(func() {
				a.dropAreaCompact(c, st)
			})
			return
		}
		if img := a.Preview(); img != nil {
			// Padding on a wrapper keeps the preview inside the card's edges;
			// a margin on a growing element is ignored by the row layout. The
			// preview takes the room the drag target does not: it grows, so the
			// picture stays large while the drop area keeps its minimum. The key
			// is the tool index so a switch lifts the new frame in.
			preview := myui.Row(c).FillWidth().MinHeight(0).Padding(16, 16, 0, 16).Clip().AlignItems(myui.Stretch).Label("预览边界").
				Key(fmt.Sprintf("preview-%d", a.Active)).Transition(motion(springSoft))
			compact := toolPreviewMode(tool.ID()) == previewCompact
			if compact {
				_, height := c.Size()
				preview.Height(min(256, height*.3))
			} else {
				preview.Grow(1)
			}
			preview.Children(func() {
				caption := ""
				if tool := a.ActiveTool(); tool != nil {
					caption = tool.PreviewCaption(a)
				}
				a.previewArea(c, st, img, caption)
			})
			if (tool.ID() == ToolRichText || tool.ID() == ToolStudio) && a.Session.Len() == 0 {
				return
			}
			files := myui.Row(c).FillWidth().Padding(12, 16, 16, 16).AlignItems(myui.Stretch)
			if compact {
				files.Grow(1).MinHeight(0).Clip()
			}
			files.Children(func() {
				if compact {
					a.dropArea(c, st)
				} else {
					a.dropAreaCompact(c, st)
				}
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

type previewMode uint8

const (
	previewLarge previewMode = iota
	previewCompact
	previewNone
)

func toolPreviewMode(toolID string) previewMode {
	switch toolID {
	case ToolPDF, ToolGIF, ToolAnalysis:
		return previewNone
	case ToolConvert, ToolResize, ToolCompress, ToolIcon, ToolGrid, ToolLayoutPage, ToolStitch, ToolPalette:
		return previewCompact
	default:
		return previewLarge
	}
}

// dropAreaCompact is the file list under a tool that draws its own preview:
// the list keeps its rows and its add action, but gives the canvas the room.
func (a *App) dropAreaCompact(c *myui.Context, st style) {
	items := a.visibleInputRows()
	height := float32(168)
	if a.fileLayout != 0 {
		height = 228
	}
	area := myui.Column(c).FillWidth().Height(height).MinHeight(0).Gap(8).Padding(12).Clip().
		Radius(st.RadiusPanel).Border(1, st.Divider).Background(st.Recessed).
		Label("拖放区")
	area.Children(func() {
		myui.Row(c).FillWidth().Wrap().Gap(8).AlignItems(myui.Center).Children(func() {
			myui.Text(c, fmt.Sprintf("%d 个文件", len(items))).FontSize(11).
				FontWeight(500).TextColor(st.TextDim)
			myui.Spacer(c).Grow(1)
			myui.Box(c).Width(156).Key("file-layout").Children(func() {
				segmented(c, &a.fileLayout, []string{"列表", "网格", "紧凑"})
			})
			a.ghostButton(c, st, "添加更多", lucide.Plus(), a.pickFiles)
			a.ghostButton(c, st, "选择文件夹", lucide.FolderOpen(), a.pickDirectory)
		})
		a.fileBrowser(c, st, items)
	})
	if files := area.DroppedFiles(); len(files) > 0 {
		a.AddPaths(files)
	}
}

func (a *App) inputHeader(c *myui.Context, st style) {
	myui.Row(c).FillWidth().MinHeight(52).Wrap().Padding(10, 18).Gap(10).AlignItems(myui.Center).
		Children(func() {
			title := "输入项"
			if (a.ActiveTool().ID() == ToolRichText || a.ActiveTool().ID() == ToolStudio) && a.Session.Len() == 0 {
				title = "画布"
			}
			myui.Text(c, title).FontSize(14).FontWeight(600).TextColor(st.Text).NoWrap()
			count := fmt.Sprintf("%d", a.Session.Len())
			if title == "输入项" {
				myui.Row(c).Height(20).PaddingX(7).Radius(999).
					Justify(myui.Center).AlignItems(myui.Center).
					Background(st.Recessed).Children(func() {
					myui.Text(c, count).FontSize(11).FontWeight(600).TextColor(st.TextSub)
				})
			}
			myui.Spacer(c).Grow(1)
			a.ghostButtonState(c, st, "选择输出位置", lucide.FolderOpen(), a.IsRunning(), false, a.pickOutputDirectory)
			if tool, ok := a.ActiveTool().(*UpscaleTool); ok {
				services := c.Services()
				a.ghostButtonState(c, st, "粘贴图片", lucide.ClipboardPaste(), a.IsRunning(), false, func() { tool.pasteClipboard(a, services) })
			}
			if a.Session.Len() > 0 {
				a.ghostButtonState(c, st, "清空", lucide.Trash2(), a.IsRunning(), false, a.clearInputs)
			}
		})
}

// ghostButtonOf is the shared secondary action, for a caller that stores its actions
// in a table. It is the same control the App method builds, so a table-driven row
// cannot end up looking like a second design.
func ghostButtonOf(c *myui.Context, st style, text string, icon *myui.SVG, onClick func()) myui.Element {
	return ghostButtonIn(c, st, text, icon, onClick)
}

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
		btn.Background(st.DividerTop).Border(1, st.BrandDeep)
	case btn.Hovered():
		btn.Background(st.Recessed).Border(1, st.Brand)
	}
	if !unavailable {
		btn = pressFeedback(btn, btn.Pressed(), 0.98)
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
	box := myui.Column(c).Fill().Grow(1).Padding(12).Gap(8).
		MinHeight(0).Clip().Radius(st.RadiusPanel).Background(st.Recessed).Label("图片预览").
		OnClick(func() { a.previewFocus.Focus(c) })
	box.Children(func() {
		myui.Row(c).FillWidth().AlignItems(myui.Center).Gap(6).Children(func() {
			myui.Icon(c, lucide.Image()).Width(13).Height(13).TextColor(st.TextDim)
			myui.Text(c, "预览").FontSize(11).FontWeight(500).TextColor(st.TextSub)
			a.previewMu.Lock()
			loading := a.previewLoading
			updating := a.studioPreviewBusy && a.studioToolID == a.ActiveTool().ID()
			a.previewMu.Unlock()
			if loading {
				myui.Text(c, "加载中…").FontSize(11).TextColor(st.TextDim)
			} else if updating {
				myui.Text(c, "更新中…").FontSize(11).TextColor(st.TextDim)
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
		myui.Image(c, myui.NewBitmap(img)).FillWidth().Grow(1).MinHeight(0).Clip().
			Radius(st.RadiusControl).Fit(myui.Contain).Label("预览图像")
	})
}

// dropArea is the large dashed target: it holds the empty hint, or the file rows
// once the tool has inputs.
func (a *App) dropArea(c *myui.Context, st style) {
	items := a.visibleInputRows()
	// The frame is drawn by hand: the framework's dashes are 3:1 and thin out at
	// the corners, which on a panel this tall reads as a broken line.
	area := myui.Column(c).Fill().Grow(1).MinHeight(0).Clip().Gap(10).Padding(18).
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
				myui.Text(c, workflowFor(a.ActiveTool()).input).FontSize(13.5).
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
		myui.Row(c).FillWidth().Wrap().Gap(8).AlignItems(myui.Center).PaddingY(2).Children(func() {
			myui.Text(c, fmt.Sprintf("%d 个文件", len(items))).FontSize(11).
				FontWeight(500).TextColor(st.TextDim)
			myui.Spacer(c).Grow(1)
			myui.Box(c).Width(156).Key("file-layout").Children(func() {
				segmented(c, &a.fileLayout, []string{"列表", "网格", "紧凑"})
			})
			a.ghostButton(c, st, "添加更多", lucide.Plus(), a.pickFiles)
		})
		a.fileBrowser(c, st, items)
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
	if text == "拖入图片或文件夹开始" && (a.ActiveTool().ID() == ToolRichText || a.ActiveTool().ID() == ToolStudio) {
		return
	}
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
	importing := a.IsImporting()
	running := a.IsRunning() || importing
	cancelLabel := "停止处理"
	if importing {
		cancelLabel = "取消导入"
	}

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
				if tool, ok := a.ActiveTool().(*UpscaleTool); ok && a.IsRunning() {
					if name, percent, _ := tool.runState.snapshot(); name != "" {
						progress := fmt.Sprintf("%s · %d%%", name, percent)
						myui.Text(c, progress).FontSize(11).TextColor(st.TextSub).SingleLine().Tooltip(progress)
					}
				}
			})
		}
		a.operationFeedback(c, st)

		myui.Row(c).FillWidth().Gap(8).AlignItems(myui.Center).Children(func() {
			if running {
				run := myui.ButtonBase(c).Grow(1).Height(46).Radius(st.RadiusControl).Gap(8).
					Justify(myui.Center).AlignItems(myui.Center).Cursor(myui.CursorPointer).
					Label(cancelLabel).Transition(motion(springTap)).Border(1, st.Divider)
				switch {
				case run.Pressed():
					run.Background(st.DividerTop).Border(1, st.Danger)
				case run.Hovered():
					run.Background(myui.RGBA(st.Danger.R, st.Danger.G, st.Danger.B, 0.10)).
						Border(1, st.Danger)
				default:
					run.Background(st.Recessed)
				}
				run = pressFeedback(run, run.Pressed(), 0.985)
				run.Children(func() {
					myui.Icon(c, lucide.X()).Width(16).Height(16).TextColor(st.Danger)
					myui.Text(c, cancelLabel).FontSize(14).FontWeight(600).TextColor(st.Text)
				})
				if run.Clicked() {
					a.Cancel()
				}
				return
			}
			tool := a.ActiveTool()
			label, icon, disabled, action := a.pageAction(tool)
			primaryButton(c, st, label, icon, disabled, false, action).
				Grow(1).Height(46)
			if a.completedOutputPath() != "" {
				a.ghostButton(c, st, "查看结果", lucide.FolderOpen(), a.openCompletedOutput)
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
