package ui

// collage.go is the拼图 (collage) page: a template picker, the canvas and style
// controls, the annotation tools, and an interactive canvas where a cell can be
// reframed with the pointer.
//
// Where a picture lands is decided by internal/collage and internal/engine; this
// file only edits the parameters and draws the composed result.

import (
	"context"
	"fmt"
	"image"
	"math"
	"path/filepath"
	"strings"
	"sync"
	"time"

	"github.com/egoist/mygo/transfer"
	myui "github.com/egoist/mygo/ui"
	"github.com/lobehub/lobe-mygo/pkg/lucide"

	"github.com/lllll081926i/imageflow-go/internal/batch"
	"github.com/lllll081926i/imageflow-go/internal/codec"
	"github.com/lllll081926i/imageflow-go/internal/collage"
	"github.com/lllll081926i/imageflow-go/internal/engine"
	"github.com/lllll081926i/imageflow-go/internal/settings"
)

// Collage grouping modes.
const (
	// collageAllInOne puts every input on one canvas.
	collageAllInOne = 0
	// collagePerSlot produces one canvas per filled template.
	collagePerSlot = 1
)

// Collage seam modes.
const (
	// collageSeamless butts the cells against each other with no gap and square
	// corners, which is what a collage is expected to look like by default.
	collageSeamless = 0
	// collageSeamed shows the background between the cells.
	collageSeamed = 1
)

// collageMarkTools are the annotation kinds the toolbar offers, in order.
var collageMarkTools = []struct {
	Kind  string
	Label string
	Icon  func() *myui.SVG
}{
	{"text", "文字", lucide.Type},
	{"arrow", "箭头", lucide.MoveUpRight},
	{"rect", "方框", lucide.Square},
	{"ellipse", "圆圈", lucide.Circle},
}

// collageMarkColors is the palette a new annotation takes from.
var collageMarkColors = []string{"#EF4444", "#F59E0B", "#22C55E", "#3B82F6", "#A855F7", "#FFFFFF"}

// collageGroupRanges are the picture-count ranges the picker offers, four
// counts per range. Sixteen counts as sixteen pills would take four rows of
// pills before the templates even start.
var collageGroupRanges = [][2]int{
	{1, 4}, {5, 8}, {9, 12}, {13, collage.MaxPresetImages},
}

// collageRangeLabels are the range pills' labels.
var collageRangeLabels = []string{"1–4 张", "5–8 张", "9–12 张", "13–16 张"}

// collageFrame is one cell's framing: a cover zoom, a pan and the two mirrors.
type collageFrame struct {
	Scale   float64
	OffsetX float64
	OffsetY float64
	FlipH   bool
	FlipV   bool
}

// CollageTool is the拼图 page.
type CollageTool struct {
	// groupCount is which image count's templates the picker shows.
	groupCount int
	// presetID is the chosen template; ignored while useCustom is set.
	presetID  string
	useCustom bool
	// grid is the custom editor's state, mergeAnchor the first corner picked.
	grid        *collage.MergeGrid
	mergeAnchor int

	aspectID    string
	customRatio string
	longEdge    float64
	// aspectRow remembers which pill each ratio row shows, so a row that does not
	// hold the active ratio keeps its own selection.
	aspectRow [3]int

	marginTop    float64
	marginRight  float64
	marginBottom float64
	marginLeft   float64
	// seam is 无接缝 / 有接缝. A seamless collage ignores gap and radius: either
	// one punches background through a joint, which is exactly what a seam is.
	seam   int
	gap    float64
	radius float64

	background myui.Color
	bgHex      string
	bgPath     string
	bgDim      float64
	fit        int

	mode    int
	format  int
	quality float64
	// maxBatch is a per-run guard rather than a preference, so it is not
	// persisted: a stored value could silently refuse a later, larger run.
	maxBatch float64

	marks     []engine.CollageMark
	markTool  int
	markColor int
	// drawMode lets a drag on the canvas draw the active mark instead of panning
	// a cell. The reference tool works this way, and it is the only way to put an
	// arrow where it belongs without guessing coordinates.
	drawMode  bool
	draftMark *engine.CollageMark
	markText  string
	// selectedMark is the annotation the panel edits: its colour swatch, its text
	// and the delete action act on it. -1 means none.
	selectedMark int
	// markDrag is the mark a press is moving, with the point it was grabbed at.
	markDrag int
	markFrom [2]float64
	// copying guards the clipboard action: composing a 4K collage takes long
	// enough that a second press would queue another one.
	copying    bool
	copyCancel context.CancelFunc
	copyGen    uint64
	// services is the frame's persistent service handle, captured while the panel
	// is built. The clipboard write has to happen on the UI thread, but this tool
	// is not the frame's owner, so it keeps the handle instead of the Context.
	services myui.Services

	watermark        bool
	watermarkText    string
	watermarkSize    float64
	watermarkOpacity float64
	watermarkAngle   float64
	watermarkDensity int

	// frames holds the per-cell framing; a cell without an entry keeps the
	// default centred cover crop.
	frames map[int]collageFrame
	// overrides replaces a slot's picture with another file, which is what a
	// double click on the canvas does. The export and the preview both read it, so
	// a replaced slot is the picture that gets written.
	overrides map[int]string
	// pendingCells holds slot replacements chosen by a background file dialog.
	// Only the UI thread moves them into overrides.
	pendingCells map[int]string
	// randState seeds the 随机 button's generator.
	randState uint32

	// The preview canvas: the composed bitmap, the signature it was composed
	// from, and the pointer state of an interactive canvas.
	previewBitmap *myui.Bitmap
	previewKey    string
	previewErr    string
	previewBusy   bool
	previewGen    uint64
	previewCancel context.CancelFunc
	previewWork   *sync.Mutex

	selectedCell int
	dragCell     int
	dragFrom     [2]float64
	showGuides   bool
	lastClick    time.Time
	lastClickAt  int

	handle      myui.Handle
	initialised bool
}

func (t *CollageTool) ID() string      { return ToolCollage }
func (t *CollageTool) Name() string    { return "图片拼图" }
func (t *CollageTool) Icon() *myui.SVG { return lucide.LayoutTemplate() }

// ensure fills the tool's defaults. The拼图 page's parameters live in the
// persisted settings, so a fresh tool is seeded from them once: a template, a
// canvas ratio and a set of annotations take minutes to assemble, and losing them
// on every launch made the page expensive to use.
func (t *CollageTool) ensure() {
	if t.grid == nil {
		t.grid = collage.NewMergeGrid(3, 3)
	}
	if !t.initialised {
		t.mergeAnchor = -1
		t.selectedCell = -1
		t.selectedMark = -1
		t.markDrag = -1
		t.dragCell = -1
		t.lastClickAt = -1
		t.aspectRow = [3]int{0, 0, 0}
		t.maxBatch = 8
		t.applyPersisted(settings.DefaultCollage())
		t.initialised = true
	}
	if t.frames == nil {
		t.frames = map[int]collageFrame{}
	}
	if t.overrides == nil {
		t.overrides = map[int]string{}
	}
	if t.presetID == "" && !t.useCustom {
		if list := collage.PresetsFor(t.groupCount); len(list) > 0 {
			t.presetID = list[0].ID
		}
	}
}

// applyPersisted copies a persisted collage block into the tool.
func (t *CollageTool) applyPersisted(c settings.Collage) {
	if c.GroupCount <= 0 {
		c = settings.DefaultCollage()
	}
	t.groupCount = c.GroupCount
	t.presetID = c.PresetID
	t.useCustom = c.UseCustom
	t.grid = collage.NewMergeGrid(c.GridCols, c.GridRows)
	if len(c.GridParents) > 0 {
		t.grid.Restore(c.GridCols, c.GridRows, c.GridParents)
	}
	t.aspectID = c.AspectID
	t.customRatio = c.CustomRatio
	t.longEdge = float64(c.LongEdge)
	t.marginTop = float64(c.MarginTop)
	t.marginRight = float64(c.MarginRight)
	t.marginBottom = float64(c.MarginBottom)
	t.marginLeft = float64(c.MarginLeft)
	t.seam = c.Seam
	t.gap = float64(c.Gap)
	t.radius = float64(c.Radius)
	t.fit = c.Fit
	t.bgHex = c.Background
	t.background = colorOf(t.bgHex)
	t.bgDim = float64(c.BackgroundDim)
	t.mode = c.Mode
	t.format = c.Format
	t.quality = float64(c.Quality)
	t.watermark = c.Watermark
	t.watermarkText = c.WatermarkText
	t.watermarkSize = float64(c.WatermarkSize)
	t.watermarkOpacity = c.WatermarkOpacity
	t.watermarkAngle = c.WatermarkAngle
	t.watermarkDensity = c.WatermarkDensity
	t.marks = nil
	for _, m := range c.Marks {
		t.marks = append(t.marks, engine.CollageMark{
			Kind: m.Kind, Text: m.Text, X1: m.X1, Y1: m.Y1, X2: m.X2, Y2: m.Y2,
			Color: m.Color, Width: m.Width,
		})
	}
	if t.markText == "" {
		t.markText = "说明文字"
	}
}

// persisted is the tool's state in the form the settings file holds.
func (t *CollageTool) persisted() settings.Collage {
	t.ensure()
	out := settings.Collage{
		GroupCount:       t.groupCount,
		PresetID:         t.presetID,
		UseCustom:        t.useCustom,
		GridCols:         t.grid.Cols,
		GridRows:         t.grid.Rows,
		GridParents:      t.grid.Parents(),
		AspectID:         t.aspectID,
		CustomRatio:      t.customRatio,
		LongEdge:         int(t.longEdge),
		MarginTop:        int(t.marginTop),
		MarginRight:      int(t.marginRight),
		MarginBottom:     int(t.marginBottom),
		MarginLeft:       int(t.marginLeft),
		Seam:             t.seam,
		Gap:              int(t.gap),
		Radius:           int(t.radius),
		Fit:              t.fit,
		Background:       t.bgHex,
		BackgroundDim:    int(t.bgDim),
		Mode:             t.mode,
		Format:           t.format,
		Quality:          int(t.quality),
		Watermark:        t.watermark,
		WatermarkText:    t.watermarkText,
		WatermarkSize:    int(t.watermarkSize),
		WatermarkOpacity: t.watermarkOpacity,
		WatermarkAngle:   t.watermarkAngle,
		WatermarkDensity: t.watermarkDensity,
	}
	for _, m := range t.marks {
		out.Marks = append(out.Marks, settings.CollageMark{
			Kind: m.Kind, Text: m.Text, X1: m.X1, Y1: m.Y1, X2: m.X2, Y2: m.Y2,
			Color: m.Color, Width: m.Width,
		})
	}
	return out
}

// persist hands the current parameters to the app, which writes them after a
// short quiet period.
func (t *CollageTool) persist(a *App) {
	if a != nil {
		a.SaveCollage(t.persisted())
	}
}

// blocks is the layout the page is editing.
func (t *CollageTool) blocks() []collage.Rect {
	t.ensure()
	if t.useCustom {
		return t.grid.Blocks()
	}
	if preset, ok := collage.PresetByID(t.presetID); ok && len(preset.Cells) > 0 {
		return preset.Cells
	}
	if list := collage.PresetsFor(t.groupCount); len(list) > 0 {
		return list[0].Cells
	}
	return []collage.Rect{{X: 0, Y: 0, W: 1, H: 1}}
}

// slotCount is how many pictures the current layout takes.
func (t *CollageTool) slotCount() int { return len(t.blocks()) }

// aspect resolves the canvas ratio.
func (t *CollageTool) aspect() collage.Aspect {
	t.ensure()
	if t.aspectID == collage.CustomAspectID {
		if a, ok := collage.ParseAspect(t.customRatio); ok {
			a.Label = t.customRatio
			return a
		}
		return collage.Aspect{ID: collage.CustomAspectID, Label: "1:1", W: 1, H: 1}
	}
	if a, ok := collage.AspectByID(t.aspectID); ok {
		return a
	}
	return collage.Aspect{ID: "1:1", Label: "1:1", W: 1, H: 1}
}

// fitName resolves the fit ladder to the engine's constants.
func (t *CollageTool) fitName() string {
	switch t.fit {
	case 1:
		return engine.CollageContain
	case 2:
		return engine.CollageStretch
	}
	return engine.CollageCover
}

// outputFormat is the encoder the export uses.
func (t *CollageTool) outputFormat() string {
	switch t.format {
	case 1:
		return "jpg"
	case 2:
		return "webp"
	}
	return "png"
}

func (t *CollageTool) margins() collage.Margins {
	return collage.Margins{
		Top: int(t.marginTop), Right: int(t.marginRight),
		Bottom: int(t.marginBottom), Left: int(t.marginLeft),
	}
}

// effectiveGap is the gap the renderer uses. A seamless collage has none,
// whatever the slider last held; switching modes back restores the last value
// instead of losing it.
func (t *CollageTool) effectiveGap() int {
	if t.seam == collageSeamed {
		return int(t.gap)
	}
	return 0
}

// effectiveRadius is the corner radius the renderer uses. A rounded corner cuts
// the picture and shows the canvas through it, so a seamless collage keeps
// square corners.
func (t *CollageTool) effectiveRadius() int {
	if t.seam == collageSeamed {
		return int(t.radius)
	}
	return 0
}

// groups splits the session rows into the canvases this run produces.
func (t *CollageTool) groups(rows []batch.Item) [][]batch.Item {
	count := t.slotCount()
	if count < 1 {
		count = 1
	}
	if t.mode == collagePerSlot {
		var out [][]batch.Item
		for start := 0; start < len(rows); start += count {
			end := start + count
			if end > len(rows) {
				end = len(rows)
			}
			out = append(out, rows[start:end])
		}
		return out
	}
	return [][]batch.Item{rows}
}

// cellsFor turns one group of rows into engine cells, in reading order.
//
// A group with more pictures than the template has slots keeps the first ones and
// drops the rest: a slot holds one picture, and silently replacing an earlier
// picture with a later one made it impossible to tell where an input went. The run
// reports the dropped count instead.
func cellsFor(rows []batch.Item, slot int) []engine.CollageCell {
	if slot < 1 {
		slot = 1
	}
	cells := make([]engine.CollageCell, slot)
	for i := 0; i < len(rows) && i < slot; i++ {
		cells[i] = engine.CollageCell{Path: rows[i].Path}
	}
	return cells
}

// droppedRows is how many pictures of a group do not fit the template.
func droppedRows(rows int, slot int) int {
	if slot < 1 || rows <= slot {
		return 0
	}
	return rows - slot
}

// buildRequest assembles the engine payload shared by the preview and the run.
func (t *CollageTool) buildRequest(cells []engine.CollageCell, outputPath string) engine.CollageRequest {
	t.ensure()
	a := t.aspect()
	m := t.margins()
	marks := make([]engine.CollageMark, len(t.marks))
	copy(marks, t.marks)
	if t.draftMark != nil {
		// The mark being dragged is part of what the canvas shows, so it is part of
		// what the engine composes for the preview.
		marks = append(marks, *t.draftMark)
	}
	return engine.CollageRequest{
		Cells:          cells,
		Rects:          t.blocks(),
		OutputPath:     outputPath,
		AspectW:        a.W,
		AspectH:        a.H,
		LongEdge:       int(t.longEdge),
		MarginTop:      m.Top,
		MarginRight:    m.Right,
		MarginBottom:   m.Bottom,
		MarginLeft:     m.Left,
		Gap:            t.effectiveGap(),
		Radius:         t.effectiveRadius(),
		Background:     t.bgHex,
		BackgroundPath: t.bgPath,
		BackgroundDim:  t.bgDim,
		Fit:            t.fitName(),
		Quality:        int(t.quality),
		Format:         t.outputFormat(),
		Marks:          marks,
		Watermark: engine.CollageWatermark{
			Enabled: t.watermark,
			Text:    t.watermarkText,
			Size:    int(t.watermarkSize),
			Opacity: t.watermarkOpacity,
			Angle:   t.watermarkAngle,
			Density: t.watermarkDensity,
		},
	}
}

// applyFrames copies the per-cell framing and picture overrides into the engine
// cells. Both the preview and the export go through it, so the canvas and the
// written file cannot disagree about which picture sits in a slot.
func (t *CollageTool) applyFrames(cells []engine.CollageCell) []engine.CollageCell {
	out := make([]engine.CollageCell, len(cells))
	copy(out, cells)
	for i := range out {
		if path, ok := t.overrides[i]; ok && path != "" {
			out[i].Path = path
		}
		if frame, ok := t.frames[i]; ok {
			out[i].Scale = frame.Scale
			out[i].OffsetX = frame.OffsetX
			out[i].OffsetY = frame.OffsetY
			out[i].FlipH = frame.FlipH
			out[i].FlipV = frame.FlipV
		}
	}
	return out
}

func (t *CollageTool) PreviewCaption(a *App) string {
	t.ensure()
	aspect := t.aspect()
	mode := "全部合成一张"
	if t.mode == collagePerSlot {
		mode = fmt.Sprintf("每 %d 张一张", t.slotCount())
	}
	return fmt.Sprintf("%s · %s · 长边 %d px", aspect.Label, mode, int(t.longEdge))
}

func (t *CollageTool) Run(a *App) {
	t.ensure()
	rows := a.Session.Snapshot()
	if len(rows) == 0 {
		a.SetStatus("先添加图片")
		a.SetRunning(false)
		return
	}
	groups := t.groups(rows)
	if limit := int(t.maxBatch); t.mode == collagePerSlot && limit > 0 && len(groups) > limit {
		a.SetRunning(false)
		a.SetStatus(fmt.Sprintf("分组过多：%d 组，超过上限 %d，请改用「全部合成一张」或减少图片", len(groups), limit))
		return
	}
	slot := t.slotCount()
	a.SetRunning(true)
	cancel := a.beginWholeList()
	groups = t.groups(a.Session.Snapshot())
	requests := make([]engine.CollageRequest, len(groups))
	for index, group := range groups {
		requests[index] = t.buildRequest(t.applyFrames(cellsFor(group, slot)), "")
	}
	go a.runCollageGroups(groups, requests, cancel, engine.CollageContext)
}

func (a *App) runCollageGroups(groups [][]batch.Item, requests []engine.CollageRequest, cancel *batch.Cancel,
	render func(context.Context, engine.CollageRequest) engine.Result) {
	defer a.finishRun()
	defer func() {
		a.Session.Mu.Lock()
		a.Session.Cancel = nil
		a.Session.Mu.Unlock()
	}()
	var completed, failed, skipped, cancelled, outputs, dropped int
	for index, group := range groups {
		req := requests[index]
		res := engine.Fail("CANCELLED", "已取消")
		if !cancel.Cancelled() {
			out, exists := a.outputPathFor(group[0], "collage", extOf(req.Format), index+1)
			if exists {
				res = engine.Skip("目标已存在")
			} else {
				req.OutputPath = out
				if err := batch.Execute(func(_ *batch.Cancel) error { res = render(cancel, req); return nil }, cancel); err != nil {
					res = engine.Fail("TASK_PANIC", err.Error())
				}
			}
		}
		if res.Success && !res.Skipped {
			outputs++
		}
		for position, item := range group {
			switch {
			case res.ErrorCode == "CANCELLED":
				a.Session.SetItemStatus(item, "cancelled", "已取消", "")
				cancelled++
			case !res.Success:
				a.Session.SetItemStatus(item, "failed", res.Error, "")
				failed++
			case res.Skipped:
				a.Session.SetItemStatus(item, "skipped", res.Message, "")
				skipped++
			case position >= len(req.Cells):
				a.Session.SetItemStatus(item, "skipped", "超出一张画布的格数", "")
				skipped++
				dropped++
			default:
				a.Session.SetItemStatus(item, "ok", fmt.Sprintf("%d 格", len(req.Cells)), res.OutputPath)
				completed++
			}
		}
		a.SetProgress(float32(index+1)*100/float32(len(groups)), fmt.Sprintf("拼图 %d/%d", index+1, len(groups)))
		a.Invalidate()
	}
	switch {
	case cancelled > 0:
		a.SetStatus(fmt.Sprintf("拼图已取消：完成 %d 张，输出 %d 张，取消 %d 张，失败 %d，跳过 %d", completed, outputs, cancelled, failed, skipped))
	case failed > 0:
		a.SetStatus(fmt.Sprintf("拼图结束：完成 %d 张，输出 %d 张，失败 %d，跳过 %d", completed, outputs, failed, skipped))
	case outputs == 0:
		a.SetStatus(fmt.Sprintf("拼图已跳过：%d 张", skipped))
	default:
		a.SetStatus(fmt.Sprintf("拼图完成：%d 张合成 %d 张，跳过 %d，超出格数 %d", completed, outputs, skipped, dropped))
	}
	a.SetProgress(100, a.StatusLine())
}

// PreviewPanel is the tool's own preview widget: an interactive canvas that
// replaces the shared single-picture preview on this page only.
func (t *CollageTool) PreviewPanel(c *myui.Context, a *App) {
	t.ensure()
	t.requestPreview(c, a)
	st := styles(c)
	rows := a.Session.Snapshot()
	box := myui.Column(c).Fill().Padding(12).Gap(8).Radius(st.RadiusPanel).
		MinHeight(0).Clip().Background(st.Recessed).Label("拼图预览区")
	box.Children(func() {
		myui.Row(c).FillWidth().AlignItems(myui.Center).Gap(6).Children(func() {
			myui.Icon(c, lucide.LayoutTemplate()).Width(13).Height(13).TextColor(st.TextDim)
			myui.Text(c, "拼图预览").FontSize(11).FontWeight(500).TextColor(st.TextSub)
			if t.previewBusy {
				myui.Text(c, "合成中…").FontSize(11).TextColor(st.TextDim)
			}
			myui.Row(c).Width(1).Height(10).Background(st.Divider).MarginX(2)
			myui.Text(c, t.PreviewCaption(a)).FontSize(11).TextColor(st.TextDim).SingleLine()
			myui.Spacer(c).Grow(1)
			if t.selectedCell >= 0 {
				a.ghostButton(c, st, "换图", lucide.ImagePlus(), func() {
					t.replaceCell(a, t.selectedCell)
				})
				if _, replaced := t.overrides[t.selectedCell]; replaced {
					a.ghostButton(c, st, "还原", lucide.Undo2(), func() {
						t.resetCell(t.selectedCell)
					})
				}
				a.ghostButton(c, st, "左右镜像", lucide.FlipHorizontal(), func() {
					t.toggleFlip(t.selectedCell, true)
				})
				a.ghostButton(c, st, "上下翻转", lucide.FlipVertical(), func() {
					t.toggleFlip(t.selectedCell, false)
				})
				a.ghostButton(c, st, "居中当前格", lucide.Focus(), func() {
					delete(t.frames, t.selectedCell)
					t.previewKey = ""
				})
			}
			a.ghostButton(c, st, "辅助线", lucide.Grid2X2(), func() { t.showGuides = !t.showGuides })
		})
		if len(rows) == 0 {
			// The input card sends the shared drop target instead of the canvas when
			// the list is empty; this branch only keeps the widget total for a caller
			// that draws it anyway.
			myui.Row(c).Fill().Children(func() {})
			return
		}
		if dropped := droppedRows(len(rows), t.slotCount()); dropped > 0 && t.mode == collageAllInOne {
			// One canvas holds one picture per slot. Saying so here is what makes the
			// grouping switch discoverable instead of looking like a lost file.
			warnAlert(c, fmt.Sprintf("当前布局只放得下 %d 张，多余 %d 张不会上画；改用「每 N 张一张」可全部输出",
				t.slotCount(), dropped))
		}
		aspect := t.aspect()
		// The canvas element takes the canvas's own ratio and sits centred in the
		// room the card has: a full-boxed picture with Contain letterboxing read as
		// a picture floating in a large empty card once the window grew.
		myui.Row(c).FillWidth().Grow(1).MinHeight(0).Clip().Justify(myui.Center).
			AlignItems(myui.Center).Children(func() {
			canvas := myui.Box(c).Fill().MinWidth(0).MinHeight(0).
				Radius(st.RadiusControl).Background(st.Card).Clip().
				Focusable().FocusRingColor(st.Brand).FocusRingOffset(2).
				Cursor(myui.CursorPointer).Label("拼图画布").Bind(&t.handle)
			t.canvasInput(c, a, canvas, aspect)
			canvas.Children(func() {
				if t.previewBitmap != nil {
					myui.Image(c, t.previewBitmap).Fill().Fit(myui.Contain)
				}
			})
			if t.showGuides {
				canvas.DrawOver(func(p *myui.Painter, r myui.Rect) {
					cw, ch := collage.CanvasSize(aspect.W, aspect.H, 1000)
					box := previewFitRect(r, cw, ch)
					px := collage.Layout(t.blocks(), cw, ch, t.margins(), t.effectiveGap())
					sx, sy := box.W/float32(cw), box.H/float32(ch)
					for i, cell := range px {
						tint := st.Divider
						if i == t.selectedCell {
							tint = st.Brand
						}
						p.Stroke(myui.Rect{
							X: box.X + float32(cell.X)*sx, Y: box.Y + float32(cell.Y)*sy,
							W: float32(cell.W) * sx, H: float32(cell.H) * sy,
						}, tint, 0, 1)
					}
				})
			}
			// The selected annotation gets a dashed marquee, drawn over the picture
			// rather than inside it: the export has no selection, and a solid frame
			// would read as part of the design.
			if t.selectedMark >= 0 && t.selectedMark < len(t.marks) {
				m := t.marks[t.selectedMark]
				canvas.DrawOver(func(p *myui.Painter, r myui.Rect) {
					cw, ch := collage.CanvasSize(aspect.W, aspect.H, 1000)
					box := previewFitRect(r, cw, ch)
					sx, sy := box.W/float32(cw), box.H/float32(ch)
					x1, y1 := box.X+float32(m.X1)*float32(cw)*sx, box.Y+float32(m.Y1)*float32(ch)*sy
					x2, y2 := box.X+float32(m.X2)*float32(cw)*sx, box.Y+float32(m.Y2)*float32(ch)*sy
					// A text mark has no explicit box: its marquee covers the glyphs, which
					// is why it grows to the right and down from its anchor.
					if m.Kind == "text" {
						x2 = x1 + box.W*0.5
						y2 = y1 + box.H*0.12
					}
					pad := float32(4)
					p.StrokeDashed(myui.Rect{
						X: minFloat32(x1, x2) - pad, Y: minFloat32(y1, y2) - pad,
						W: absFloat32(x2-x1) + 2*pad, H: absFloat32(y2-y1) + 2*pad,
					}, st.Brand, 3, 1.5)
				})
			}
		})
		if t.previewErr != "" {
			warnAlert(c, "预览失败："+t.previewErr)
		}
		hint := "取景"
		if t.drawMode {
			hint = collageMarkTools[t.markTool].Label
		}
		myui.Row(c).FillWidth().Gap(6).AlignItems(myui.Center).Children(func() {
			myui.Icon(c, lucide.MousePointerClick()).Width(12).Height(12).TextColor(st.TextDim)
			myui.Text(c, hint).FontSize(10.5).TextColor(st.TextDim).SingleLine()
		})
	})
}

// previewFitRect maps the canvas into the preview box the way Image with
// Fit(Contain) does: the composed picture is centred and letterboxed, so a point
// has to be mapped through the picture's own rect, not through the whole box.
// Without this, a click in the letterbox band selected a cell that was not under
// the pointer and the guides were drawn beside the picture rather than over it.
//
// An input event's coordinates are relative to the element's own box, so callers
// pass a rect at the origin; DrawOver passes the box's own rect and gets that
// back.
func previewFitRect(b myui.Rect, canvasW, canvasH int) myui.Rect {
	if canvasW <= 0 || canvasH <= 0 || b.W <= 0 || b.H <= 0 {
		return b
	}
	sx := b.W / float32(canvasW)
	sy := b.H / float32(canvasH)
	s := sx
	if sy < s {
		s = sy
	}
	w := float32(canvasW) * s
	h := float32(canvasH) * s
	return myui.Rect{
		X: b.X + (b.W-w)/2,
		Y: b.Y + (b.H-h)/2,
		W: w, H: h,
	}
}

// markAt returns the topmost annotation under a canvas point, or -1.
//
// A mark is hit by its own shape: a box and an ellipse by their outline band, an
// arrow by its line, a text by the box its glyphs occupy. A generous band means a
// thin annotation is still grabbable with a mouse.
func (t *CollageTool) markAt(cx, cy int) int {
	// Later marks paint over earlier ones, so the search runs backwards.
	for i := len(t.marks) - 1; i >= 0; i-- {
		if t.markHit(t.marks[i], cx, cy) {
			return i
		}
	}
	return -1
}

// markHit reports whether a canvas point is on an annotation.
func (t *CollageTool) markHit(m engine.CollageMark, cx, cy int) bool {
	aspect := t.aspect()
	cw, ch := collage.CanvasSize(aspect.W, aspect.H, 1000)
	x1, y1 := int(m.X1*float64(cw)), int(m.Y1*float64(ch))
	x2, y2 := int(m.X2*float64(cw)), int(m.Y2*float64(ch))
	// A band wide enough to grab a thin line, and never narrower than the mark's
	// own stroke.
	band := 10
	if m.Width*3 > band {
		band = m.Width * 3
	}
	near := func(v, target, tol int) bool {
		d := v - target
		return d <= tol && d >= -tol
	}
	switch m.Kind {
	case "arrow":
		// Distance from the segment, which is the only shape an arrow has.
		dx, dy := float64(x2-x1), float64(y2-y1)
		length := math.Hypot(dx, dy)
		if length < 1 {
			return near(cx, x1, band) && near(cy, y1, band)
		}
		t0 := (float64(cx-x1)*dx + float64(cy-y1)*dy) / (length * length)
		t0 = clampFloat64(t0, 0, 1)
		px, py := float64(x1)+dx*t0, float64(y1)+dy*t0
		return math.Hypot(float64(cx)-px, float64(cy)-py) <= float64(band)
	case "rect", "ellipse":
		if cx < minInt(x1, x2)-band || cx > maxInt(x1, x2)+band ||
			cy < minInt(y1, y2)-band || cy > maxInt(y1, y2)+band {
			return false
		}
		// On the outline, not the middle: a filled box would swallow the picture it
		// is drawn over. The middle still counts when the box is tiny.
		if near(cx, x1, band) || near(cx, x2, band) || near(cy, y1, band) || near(cy, y2, band) {
			return true
		}
		return absInt(x2-x1) < 4*band && absInt(y2-y1) < 4*band
	case "text":
		// Text is drawn from its anchor to the right and down: a rectangle of a
		// generous half canvas width and an eighth of its height holds it.
		w := int(float64(cw) * 0.5)
		h := int(float64(ch) * 0.12)
		return cx >= x1-band && cx <= x1+w+band && cy >= y1-band && cy <= y1+h+band
	}
	return false
}

// askRedraw marks the composed preview stale, so the next frame recomposes it.
func (t *CollageTool) askRedraw() {
	t.previewKey = ""
}

func minInt(a, b int) int {
	if a < b {
		return a
	}
	return b
}

func maxInt(a, b int) int {
	if a > b {
		return a
	}
	return b
}

func absInt(v int) int {
	if v < 0 {
		return -v
	}
	return v
}

// canvasInput makes the canvas interactive: click to select a cell, drag to pan
// inside it, arrows to nudge, Ctrl with the wheel to zoom, double click to
// replace the picture.
func (t *CollageTool) canvasInput(c *myui.Context, a *App, canvas myui.Element, aspect collage.Aspect) {
	canvas.HandleInput(func(ev myui.InputEvent) bool {
		bounds := t.handle.Bounds(c)
		if bounds.W <= 0 || bounds.H <= 0 {
			bounds = canvas.Bounds()
		}
		if bounds.W <= 0 || bounds.H <= 0 {
			return false
		}
		cw, ch := collage.CanvasSize(aspect.W, aspect.H, 1000)
		// InputEvent coordinates are relative to the element's own box while the
		// committed bounds are in window DIPs: the mapping has to use a rect at the
		// origin with the same size, or every click lands off by the element's own
		// position.
		local := myui.Rect{X: 0, Y: 0, W: bounds.W, H: bounds.H}
		box := previewFitRect(local, cw, ch)
		px := collage.Layout(t.blocks(), cw, ch, t.margins(), t.effectiveGap())
		sx, sy := box.W/float32(cw), box.H/float32(ch)
		// canvasPoint maps an element-local point onto the canvas pixels.
		canvasPoint := func(x, y float32) (int, int) {
			return int((x - box.X) / sx), int((y - box.Y) / sy)
		}
		hit := func(x, y float32) int {
			cx, cy := canvasPoint(x, y)
			for i, cell := range px {
				if cell.Contains(cx, cy) {
					return i
				}
			}
			return -1
		}
		switch ev.Kind {
		case myui.InputPointerDown:
			if ev.Button != 0 || ev.X < box.X || ev.X > box.X+box.W || ev.Y < box.Y || ev.Y > box.Y+box.H {
				return false
			}
			// The canvas takes the keyboard on a press: its arrow keys and Delete act
			// on the selected cell, and the input card around it no longer binds the
			// arrows to the file preview.
			t.handle.Focus(c)
			// A press on an existing annotation grabs it, before either drawing a new
			// mark or starting a picture drag: editing what is already on the canvas
			// is the reason the marks are drawn there at all.
			if mark := t.markAt(canvasPoint(ev.X, ev.Y)); mark >= 0 {
				t.selectedMark = mark
				t.drawMode = false
				t.markDrag = mark
				t.markFrom = [2]float64{float64(ev.X), float64(ev.Y)}
				t.previewKey = ""
				a.Invalidate()
				return true
			}
			t.selectedMark = -1
			if t.drawMode {
				// Drawing a mark: the press starts it at the pointer, the move sizes
				// it, and the release commits it.
				cx, cy := canvasPoint(ev.X, ev.Y)
				x := float64(cx) / float64(cw)
				y := float64(cy) / float64(ch)
				t.draftMark = &engine.CollageMark{
					Kind:  collageMarkTools[t.markTool].Kind,
					Text:  t.markText,
					X1:    clampFloat64(x, 0, 1),
					Y1:    clampFloat64(y, 0, 1),
					X2:    clampFloat64(x, 0, 1),
					Y2:    clampFloat64(y, 0, 1),
					Color: collageMarkColors[t.markColor%len(collageMarkColors)],
					Width: 7,
				}
				a.Invalidate()
				return true
			}
			cell := hit(ev.X, ev.Y)
			double := cell >= 0 && t.lastClickAt == cell && time.Since(t.lastClick) < 400*time.Millisecond
			t.selectedCell = cell
			t.dragCell = cell
			t.dragFrom = [2]float64{float64(ev.X), float64(ev.Y)}
			t.lastClick = time.Now()
			t.lastClickAt = cell
			if double {
				t.replaceCell(a, cell)
			}
			a.Invalidate()
			return true
		case myui.InputPointerMove:
			if t.markDrag >= 0 {
				// Moving a mark shifts both ends by the same fraction, so its size and
				// shape stay whatever they were.
				dx := (float64(ev.X) - t.markFrom[0]) / float64(box.W)
				dy := (float64(ev.Y) - t.markFrom[1]) / float64(box.H)
				t.markFrom = [2]float64{float64(ev.X), float64(ev.Y)}
				m := &t.marks[t.markDrag]
				wantX := clampFloat64(m.X1+dx, 0, 1)
				wantY := clampFloat64(m.Y1+dy, 0, 1)
				dx, dy = wantX-m.X1, wantY-m.Y1
				m.X1, m.X2 = m.X1+dx, m.X2+dx
				m.Y1, m.Y2 = m.Y1+dy, m.Y2+dy
				t.previewKey = ""
				return true
			}
			if t.drawMode && t.draftMark != nil {
				cx, cy := canvasPoint(ev.X, ev.Y)
				t.draftMark.X2 = clampFloat64(float64(cx)/float64(cw), 0, 1)
				t.draftMark.Y2 = clampFloat64(float64(cy)/float64(ch), 0, 1)
				t.previewKey = ""
				a.Invalidate()
				return true
			}
			if t.dragCell < 0 || ev.Button != 0 {
				return false
			}
			// The pan is relative to the cell's own size, so the picture follows
			// the pointer one for one whatever the preview is scaled to.
			dx := (float64(ev.X) - t.dragFrom[0]) / float64(box.W)
			dy := (float64(ev.Y) - t.dragFrom[1]) / float64(box.H)
			t.dragFrom = [2]float64{float64(ev.X), float64(ev.Y)}
			frame := t.frames[t.dragCell]
			if frame.Scale <= 0 {
				frame.Scale = 1
			}
			frame.OffsetX = clampFloat64(frame.OffsetX-dx*2, -1, 1)
			frame.OffsetY = clampFloat64(frame.OffsetY-dy*2, -1, 1)
			t.frames[t.dragCell] = frame
			t.previewKey = ""
			a.Invalidate()
			return true
		case myui.InputPointerUp:
			if t.markDrag >= 0 {
				t.markDrag = -1
				t.previewKey = ""
				return true
			}
			if t.drawMode && t.draftMark != nil {
				m := *t.draftMark
				t.draftMark = nil
				// A click without a drag would leave an invisible zero-sized mark, so
				// a press that did not move gets a default size around the point.
				if math.Abs(m.X2-m.X1) < 0.01 && math.Abs(m.Y2-m.Y1) < 0.01 {
					m.X2 = clampFloat64(m.X1+0.2, 0, 1)
					m.Y2 = clampFloat64(m.Y1+0.15, 0, 1)
				}
				t.marks = append(t.marks, m)
				t.previewKey = ""
				a.Invalidate()
				return true
			}
			t.dragCell = -1
			return true
		case myui.InputScroll:
			if ev.Mods&myui.Ctrl == 0 || t.selectedCell < 0 {
				return false
			}
			frame := t.frames[t.selectedCell]
			if frame.Scale <= 0 {
				frame.Scale = 1
			}
			frame.Scale = clampFloat64(frame.Scale-float64(ev.DY)/400, 1, 5)
			t.frames[t.selectedCell] = frame
			t.previewKey = ""
			a.Invalidate()
			return true
		case myui.InputKeyDown:
			if t.selectedCell < 0 {
				return false
			}
			frame := t.frames[t.selectedCell]
			if frame.Scale <= 0 {
				frame.Scale = 1
			}
			step := 0.02
			if ev.Mods&myui.Shift != 0 {
				step = 0.1
			}
			switch ev.Key {
			case myui.KeyLeft:
				frame.OffsetX = clampFloat64(frame.OffsetX-step*2, -1, 1)
			case myui.KeyRight:
				frame.OffsetX = clampFloat64(frame.OffsetX+step*2, -1, 1)
			case myui.KeyUp:
				frame.OffsetY = clampFloat64(frame.OffsetY-step*2, -1, 1)
			case myui.KeyDown:
				frame.OffsetY = clampFloat64(frame.OffsetY+step*2, -1, 1)
			case myui.KeyBackspace, myui.KeyDelete:
				// Delete clears the framing, not the cell: a cell's picture belongs to
				// the file list, and removing a row from a preview press would surprise.
				delete(t.frames, t.selectedCell)
				t.previewKey = ""
				a.Invalidate()
				return true
			default:
				return false
			}
			t.frames[t.selectedCell] = frame
			t.previewKey = ""
			a.Invalidate()
			return true
		}
		return false
	})
}

// replaceCell points one slot at another file, which is what a double click on
// the canvas asks for. The picked file is stored as a slot override rather than
// appended to the session: the list is the batch's input, and a slot replacement
// is a collage edit.
func (t *CollageTool) replaceCell(a *App, cell int) {
	if cell < 0 {
		return
	}
	t.selectedCell = cell
	a.pickCollageCell(t, cell)
}

// toggleFlip mirrors one cell, keeping its framing. FlipH mirrors left to right,
// FlipV top to bottom; a second press takes the mirror back.
func (t *CollageTool) toggleFlip(cell int, horizontal bool) {
	if cell < 0 {
		return
	}
	frame := t.frames[cell]
	if frame.Scale <= 0 {
		frame.Scale = 1
	}
	if horizontal {
		frame.FlipH = !frame.FlipH
	} else {
		frame.FlipV = !frame.FlipV
	}
	t.frames[cell] = frame
	t.previewKey = ""
}

// resetCell drops a slot's override, returning it to the session's picture.
func (t *CollageTool) resetCell(cell int) {
	if _, ok := t.overrides[cell]; ok {
		delete(t.overrides, cell)
		t.previewKey = ""
	}
}

// requestPreview rebuilds the composed preview when its inputs changed.
func (t *CollageTool) requestPreview(c *myui.Context, a *App) {
	rows := a.Session.Snapshot()
	if len(rows) == 0 {
		if t.previewCancel != nil {
			t.previewCancel()
		}
		t.previewGen++
		t.previewBusy = false
		t.previewBitmap = nil
		t.previewKey = ""
		return
	}
	cells := t.applyFrames(cellsFor(rows, t.slotCount()))
	key := t.previewSignature(cells)
	if key == t.previewKey {
		return
	}
	t.previewKey = key
	req := t.buildRequest(cells, "")
	// The preview composes at a smaller long edge than the export, so the lengths
	// that live in canvas pixels (margins, gap, corner radius) must shrink with
	// it: a 20 px margin on a 1600 px canvas is 12.8 px on a 1024 px one, and
	// leaving it at 20 made the preview look tighter than the file it stands for.
	req.LongEdge = collagePreviewEdge()
	scale := float64(req.LongEdge) / math.Max(t.longEdge, 1)
	if scale <= 0 || scale > 1 {
		scale = 1
	}
	req.MarginTop = scaledLength(req.MarginTop, scale)
	req.MarginRight = scaledLength(req.MarginRight, scale)
	req.MarginBottom = scaledLength(req.MarginBottom, scale)
	req.MarginLeft = scaledLength(req.MarginLeft, scale)
	req.Gap = scaledLength(req.Gap, scale)
	req.Radius = scaledLength(req.Radius, scale)
	var publish func(func())
	if a.Window != nil {
		publish = a.Window.Update
	}
	t.startPreview(a, req, publish)
}

func (t *CollageTool) startPreview(a *App, req engine.CollageRequest, publish func(func())) {
	if t.previewCancel != nil {
		t.previewCancel()
	}
	ctx, cancel := context.WithCancel(context.Background())
	t.previewCancel = cancel
	t.previewGen++
	gen := t.previewGen
	t.previewBusy = true
	if t.previewWork == nil {
		t.previewWork = &sync.Mutex{}
	}
	work := t.previewWork
	render := func() {
		work.Lock()
		defer work.Unlock()
		if ctx.Err() != nil {
			return
		}
		img, res := renderCollageSafely(ctx, req)
		commit := func() {
			defer cancel()
			if gen != t.previewGen || ctx.Err() != nil {
				return
			}
			t.previewBusy = false
			if !res.Success {
				t.previewErr = res.Error
				t.previewBitmap = nil
			} else {
				t.previewBitmap = myui.NewBitmap(img)
				t.previewErr = ""
			}
			a.Invalidate()
		}
		if publish == nil {
			commit()
		} else if ctx.Err() == nil {
			publish(commit)
		}
	}
	if publish == nil {
		render()
	} else {
		go render()
	}
}

func renderCollageSafely(ctx context.Context, req engine.CollageRequest) (image.Image, engine.Result) {
	var img image.Image
	var res engine.Result
	if err := batch.Execute(func(_ *batch.Cancel) error {
		img, res = engine.RenderCollage(ctx, req)
		return nil
	}, nil); err != nil {
		return nil, engine.Fail("TASK_PANIC", err.Error())
	}
	return img, res
}

// collagePreviewEdge is the long edge the preview composes at: enough to judge
// the layout, small enough to stay instant.
func collagePreviewEdge() int { return 1024 }

// scaledLength rounds a canvas length after scaling it for the preview.
func scaledLength(v int, scale float64) int {
	if v <= 0 {
		return 0
	}
	out := int(float64(v)*scale + 0.5)
	if out < 1 {
		out = 1
	}
	return out
}

// previewSignature describes everything that changes the composed picture.
func (t *CollageTool) previewSignature(cells []engine.CollageCell) string {
	var b strings.Builder
	for _, cell := range cells {
		if cell.Path == "" {
			continue
		}
		info, err := filepath.Abs(cell.Path)
		if err != nil {
			info = cell.Path
		}
		fmt.Fprintf(&b, "%s|%.3f|%.3f|%.3f;", info, cell.Scale, cell.OffsetX, cell.OffsetY)
	}
	for _, r := range t.blocks() {
		fmt.Fprintf(&b, "%.4f,%.4f,%.4f,%.4f;", r.X, r.Y, r.W, r.H)
	}
	aspect := t.aspect()
	fmt.Fprintf(&b, "|%s|%.1f,%.1f,%.1f,%.1f|%.0f|%.0f|%s|%s|%.0f|%s",
		aspect.Label, t.marginTop, t.marginRight, t.marginBottom, t.marginLeft,
		t.gap, t.radius, t.bgHex, t.bgPath, t.bgDim, t.fitName())
	fmt.Fprintf(&b, "|seam%d", t.seam)
	for _, m := range t.marks {
		fmt.Fprintf(&b, "|%+v", m)
	}
	if t.draftMark != nil {
		// The mark being dragged is composed as part of the preview, so it is part
		// of the preview's signature too: without it the canvas would only catch up
		// after the press ended.
		fmt.Fprintf(&b, "|draft%+v", *t.draftMark)
	}
	fmt.Fprintf(&b, "|%v|%s|%.0f|%.2f|%.0f|%d", t.watermark, t.watermarkText,
		t.watermarkSize, t.watermarkOpacity, t.watermarkAngle, t.watermarkDensity)
	return b.String()
}

// Panel builds the parameter column.
func (t *CollageTool) Panel(c *myui.Context, a *App) {
	t.ensure()
	// The clipboard write needs a persistent service handle, and a Context may not
	// be kept past the frame; take the handle while the panel is being built.
	t.services = c.Services()
	t.resetSection(c, a)
	t.layoutSection(c, a)
	t.canvasSection(c)
	t.styleSection(c, a)
	t.markSection(c, a)
	t.watermarkSection(c)
	t.exportSection(c, a)
	// Every parameter the panel edits is persisted, and the write is debounced and
	// skipped when nothing changed (App.SaveCollage). Calling it here instead of in
	// two dozen callbacks keeps one source of truth: what the panel shows is what
	// the next launch restores.
	t.persist(a)
}

// resetSection is the panel's 恢复默认设置 action.
func (t *CollageTool) resetSection(c *myui.Context, a *App) {
	st := styles(c)
	myui.Row(c).FillWidth().AlignItems(myui.Center).Children(func() {
		myui.Spacer(c).Grow(1)
		a.ghostButton(c, st, "恢复默认设置", lucide.RotateCcw(), func() {
			t.resetToDefaults()
			a.SetStatus("拼图设置已恢复默认")
		})
	})
}

// layoutSection is the template picker and the custom merge editor.
func (t *CollageTool) layoutSection(c *myui.Context, a *App) {
	st := styles(c)
	sectionCard(c, "布局", func() {
		// Two ladders, not sixteen pills in a row: the range narrows the picture
		// count, then the second row picks the exact count inside it.
		rangePick := 0
		for i, r := range collageGroupRanges {
			if t.groupCount >= r[0] && t.groupCount <= r[1] {
				rangePick = i
			}
		}
		segmented(c, &rangePick, collageRangeLabels)
		from, to := collageGroupRanges[rangePick][0], collageGroupRanges[rangePick][1]
		labels := make([]string, 0, to-from+1)
		countPick := 0
		for n := from; n <= to; n++ {
			labels = append(labels, fmt.Sprintf("%d 张", n))
			if n == t.groupCount {
				countPick = n - from
			}
		}
		segmented(c, &countPick, labels)
		if want := from + countPick; want != t.groupCount {
			t.groupCount = want
			t.frames = map[int]collageFrame{}
			t.useCustom = false
			if list := collage.PresetsFor(want); len(list) > 0 {
				t.presetID = list[0].ID
			}
			t.previewKey = ""
		}

		if t.useCustom {
			t.mergeEditor(c, st, a)
			myui.Row(c).FillWidth().Children(func() {
				a.ghostButton(c, st, "返回预设", lucide.LayoutGrid(), func() {
					t.useCustom = false
					t.mergeAnchor = -1
					t.frames = map[int]collageFrame{}
					t.previewKey = ""
				})
			})
			return
		}

		presets := collage.PresetsFor(t.groupCount)
		for row := 0; row*2 < len(presets); row++ {
			myui.Row(c).FillWidth().Gap(8).AlignItems(myui.Stretch).Children(func() {
				for i := row * 2; i < row*2+2 && i < len(presets); i++ {
					t.presetThumb(c, st, presets[i])
				}
				if row*2+1 >= len(presets) {
					// Keep the last row's single template at the same width as a
					// pair, so the column does not reflow when the count changes.
					myui.Row(c).Grow(1)
				}
			})
		}
		myui.Row(c).FillWidth().Children(func() {
			a.ghostButton(c, st, "自定义网格", lucide.Grid2X2(), func() {
				t.useCustom = true
				t.mergeAnchor = -1
				t.frames = map[int]collageFrame{}
				t.previewKey = ""
			})
		})
	})
}

// presetThumb draws one template as a miniature of its own cells.
func (t *CollageTool) presetThumb(c *myui.Context, st style, p collage.Preset) {
	active := !t.useCustom && t.presetID == p.ID
	col := myui.Column(c).Grow(1).Gap(5).Padding(6).Radius(st.RadiusPill).
		Border(1, st.Divider).Cursor(myui.CursorPointer).Label(p.Name).
		Transition(motion(springTap))
	switch {
	case active:
		col.Background(st.BrandSoft).Border(1, st.Brand)
	case col.Hovered():
		col.Background(st.Recessed)
	}
	col.Children(func() {
		thumb := myui.Box(c).FillWidth().AspectRatio(1).Radius(4).
			Background(st.Recessed).Clip()
		thumb.DrawOver(func(pt *myui.Painter, r myui.Rect) {
			for _, cell := range p.Cells {
				pt.Fill(myui.Rect{
					X: r.X + float32(cell.X)*r.W + 1,
					Y: r.Y + float32(cell.Y)*r.H + 1,
					W: float32(cell.W)*r.W - 2,
					H: float32(cell.H)*r.H - 2,
				}, st.TextDim, 2)
			}
		})
		myui.Text(c, p.Name).FontSize(10.5).TextColor(st.TextSub).NoWrap().SingleLine()
	})
	if col.Clicked() {
		t.presetID = p.ID
		t.useCustom = false
		t.frames = map[int]collageFrame{}
		t.previewKey = ""
	}
}

// mergeEditor is the custom grid editor: pick two cells to merge their bounding
// box, right-click a block to split it again.
func (t *CollageTool) mergeEditor(c *myui.Context, st style, a *App) {
	sectionCard(c, "自定义网格", func() {
		grid2(c, func() {
			myui.Column(c).Gap(5).Children(func() {
				label(c, "列")
				cols := float64(t.grid.Cols)
				numberField(c, &cols, float64(collage.MinCustomCols), float64(collage.MaxCustomCols), 1, "3")
				if int(cols) != t.grid.Cols {
					t.grid.SetSize(int(cols), t.grid.Rows)
					t.mergeAnchor = -1
					t.frames = map[int]collageFrame{}
					t.previewKey = ""
				}
			})
			myui.Column(c).Gap(5).Children(func() {
				label(c, "行")
				rows := float64(t.grid.Rows)
				numberField(c, &rows, float64(collage.MinCustomRows), float64(collage.MaxCustomRows), 1, "3")
				if int(rows) != t.grid.Rows {
					t.grid.SetSize(t.grid.Cols, int(rows))
					t.mergeAnchor = -1
					t.frames = map[int]collageFrame{}
					t.previewKey = ""
				}
			})
		})
		board := myui.Box(c).FillWidth().
			AspectRatio(float32(t.grid.Cols) / float32(t.grid.Rows)).
			Radius(st.RadiusPill).Background(st.Recessed).Clip().
			Cursor(myui.CursorPointer).Label("网格编辑器")
		board.DrawOver(func(p *myui.Painter, r myui.Rect) {
			cw := r.W / float32(t.grid.Cols)
			ch := r.H / float32(t.grid.Rows)
			for row := 0; row < t.grid.Rows; row++ {
				for col := 0; col < t.grid.Cols; col++ {
					p.Stroke(myui.Rect{
						X: r.X + float32(col)*cw, Y: r.Y + float32(row)*ch,
						W: cw, H: ch,
					}, st.Divider, 0, 1)
				}
			}
			for i, block := range t.grid.Blocks() {
				tint := st.Brand
				if i%2 == 1 {
					tint = st.Success
				}
				rect := myui.Rect{
					X: r.X + float32(block.X)*r.W + 2,
					Y: r.Y + float32(block.Y)*r.H + 2,
					W: float32(block.W)*r.W - 4,
					H: float32(block.H)*r.H - 4,
				}
				p.Fill(rect, Tint(tint, 0.18), 3)
				p.Stroke(rect, tint, 3, 1)
			}
		})
		board.OnClick(func() {
			col, row, ok := t.gridCellAt(c, board)
			if !ok {
				return
			}
			if t.mergeAnchor < 0 {
				t.mergeAnchor = row*t.grid.Cols + col
				return
			}
			ax, ay := t.mergeAnchor%t.grid.Cols, t.mergeAnchor/t.grid.Cols
			t.grid.Merge(ax, ay, col, row)
			t.mergeAnchor = -1
			t.frames = map[int]collageFrame{}
			t.previewKey = ""
		})
		board.RightClicked()
		board.ContextMenu(func(m *myui.Menu) {
			if m.Item("拆分区块").Chosen() {
				col, row, ok := t.gridCellAt(c, board)
				if ok && t.grid.SplitAt(col, row) {
					t.frames = map[int]collageFrame{}
					t.previewKey = ""
				}
			}
		})
		myui.Row(c).FillWidth().Gap(6).AlignItems(myui.Center).Children(func() {
			hint := "合并区块"
			if t.mergeAnchor >= 0 {
				hint = "选择目标区块"
			}
			myui.Text(c, hint).FontSize(11).TextColor(st.TextDim).SingleLine()
			myui.Spacer(c).Grow(1)
			statusPill(c, fmt.Sprintf("%d 块", len(t.grid.Blocks())), st.Brand)
		})
		myui.Row(c).FillWidth().Gap(6).Children(func() {
			a.ghostButton(c, st, "重置网格", lucide.RotateCcw(), func() {
				t.grid.Reset()
				t.mergeAnchor = -1
				t.frames = map[int]collageFrame{}
				t.previewKey = ""
			})
			a.ghostButton(c, st, "左右翻转", lucide.FlipHorizontal(), func() {
				t.grid.Mirror()
				t.frames = map[int]collageFrame{}
				t.previewKey = ""
			})
		})
	})
}

// gridCellAt maps the pointer on the editor board to a grid cell.
func (t *CollageTool) gridCellAt(c *myui.Context, board myui.Element) (int, int, bool) {
	x, y, ok := board.PointerPosition()
	if !ok {
		return 0, 0, false
	}
	b := board.Bounds()
	if b.W <= 0 || b.H <= 0 {
		return 0, 0, false
	}
	col := int(x / b.W * float32(t.grid.Cols))
	row := int(y / b.H * float32(t.grid.Rows))
	if col < 0 || col >= t.grid.Cols || row < 0 || row >= t.grid.Rows {
		return 0, 0, false
	}
	return col, row, true
}

// canvasSection is the ratio, the output size and the margins.
func (t *CollageTool) canvasSection(c *myui.Context) {
	sectionCard(c, "画布", func() {
		rows := [][]string{
			{"1:1", "16:9", "9:16", "16:10", "4:3", "3:4"},
			{"6寸", "6寸横", "7寸", "7寸横", "8寸", "8寸横"},
			{"A4", "A4横"},
		}
		// The row that holds the active ratio shows it selected; every other row
		// keeps its own pick. A row only changes the ratio when the user clicks a
		// different pill, so nothing re-keys the preview on a passive frame.
		applied := false
		for ri, row := range rows {
			selected := -1
			for i, label := range row {
				if preset, ok := collage.AspectByLabel(label); ok && preset.ID == t.aspectID {
					selected = i
				}
			}
			pick := selected
			if pick < 0 {
				if t.aspectRow[ri] >= 0 && t.aspectRow[ri] < len(row) {
					pick = t.aspectRow[ri]
				} else {
					pick = 0
				}
			}
			last := t.aspectRow[ri]
			segmented(c, &pick, row)
			t.aspectRow[ri] = pick
			if applied || pick == last || pick == selected {
				continue
			}
			if preset, ok := collage.AspectByLabel(row[pick]); ok {
				t.aspectID = preset.ID
				t.previewKey = ""
				applied = true
			}
		}
		myui.Row(c).FillWidth().AlignItems(myui.Center).Gap(6).Children(func() {
			label(c, "自定义比例")
			myui.Spacer(c).Grow(1)
			custom := 0
			if t.aspectID == collage.CustomAspectID {
				custom = 1
			}
			active := custom == 1
			btn := myui.ButtonBase(c).Height(28).PaddingX(10).Radius(styles(c).RadiusPill).
				Border(1, styles(c).Divider).Background(styles(c).Recessed).
				Label("自定义比例").Transition(motion(springTap))
			if active {
				btn.Border(1, styles(c).Brand).Background(styles(c).BrandSoft)
			}
			btn.Children(func() {
				myui.Text(c, "自定义比例").FontSize(11.5).TextColor(styles(c).TextSub)
			})
			if btn.Clicked() {
				if active {
					t.aspectID = "1:1"
				} else {
					t.aspectID = collage.CustomAspectID
				}
				t.previewKey = ""
			}
		})
		if t.aspectID == collage.CustomAspectID {
			myui.Column(c).Gap(5).Children(func() {
				label(c, "比例（宽:高）")
				textField(c, &t.customRatio, "16:9")
			})
			if _, ok := collage.ParseAspect(t.customRatio); !ok {
				warnAlert(c, "比例格式无效，预览按 1:1 处理")
			}
		}
		sliderRow(c, "长边像素", &t.longEdge, 480, 8000, 80, "%.0f")
		grid2(c, func() {
			myui.Column(c).Gap(5).Children(func() {
				label(c, "上边距")
				numberField(c, &t.marginTop, 0, 800, 1, "0")
			})
			myui.Column(c).Gap(5).Children(func() {
				label(c, "下边距")
				numberField(c, &t.marginBottom, 0, 800, 1, "0")
			})
			myui.Column(c).Gap(5).Children(func() {
				label(c, "左边距")
				numberField(c, &t.marginLeft, 0, 800, 1, "0")
			})
			myui.Column(c).Gap(5).Children(func() {
				label(c, "右边距")
				numberField(c, &t.marginRight, 0, 800, 1, "0")
			})
		})
	})
}

// styleSection is the seam mode, the gap, the radius, the fit and the background.
func (t *CollageTool) styleSection(c *myui.Context, a *App) {
	st := styles(c)
	sectionCard(c, "样式", func() {
		// The seam is the first choice because it changes what the other two
		// controls mean: a seamless collage has no gap and square corners by
		// definition.
		seam := t.seam
		segmented(c, &seam, []string{"无接缝", "有接缝"})
		if seam != t.seam && seam == collageSeamed && t.gap <= 0 && t.radius <= 0 {
			// Switching to a seamed collage for the first time must show a seam,
			// not the same picture the seamless mode drew.
			t.gap = 10
			t.radius = 8
		}
		t.seam = seam
		if t.seam == collageSeamed {
			sliderRow(c, "间距", &t.gap, 0, 80, 1, "%.0f px")
			sliderRow(c, "圆角", &t.radius, 0, 120, 1, "%.0f px")
		}
		fit := t.fit
		segmented(c, &fit, []string{"裁切填满", "完整显示", "拉伸"})
		t.fit = fit
		myui.Column(c).Gap(5).Children(func() {
			label(c, "背景色")
			before := t.background
			colorField(c, &t.background)
			if t.background != before {
				t.bgHex = hexOf(t.background)
			}
		})
		myui.Row(c).FillWidth().AlignItems(myui.Center).Gap(8).Children(func() {
			myui.Text(c, "背景图片").FontSize(12).FontWeight(500).TextColor(st.TextSub)
			myui.Spacer(c).Grow(1)
			name := "未选择"
			if t.bgPath != "" {
				name = filepath.Base(t.bgPath)
			}
			statusPill(c, name, st.TextDim)
		})
		myui.Row(c).FillWidth().Gap(6).Children(func() {
			a.ghostButton(c, st, "上传背景图", lucide.ImagePlus(), func() {
				a.pickCollageBackground(t)
			})
			if t.bgPath != "" {
				a.ghostButton(c, st, "移除", lucide.Trash2(), func() {
					t.bgPath = ""
					t.previewKey = ""
				})
			}
		})
		if t.bgPath != "" {
			sliderRow(c, "背景暗化", &t.bgDim, 0, 100, 1, "%.0f%%")
		}
	})
}

// markSection is the annotation toolbar.
func (t *CollageTool) markSection(c *myui.Context, a *App) {
	st := styles(c)
	sectionCard(c, "标注", func() {
		myui.Row(c).FillWidth().Gap(6).Children(func() {
			for i, tool := range collageMarkTools {
				index := i
				active := t.drawMode && t.markTool == index
				btn := myui.ButtonBase(c).Grow(1).Height(30).Radius(st.RadiusPill).
					Justify(myui.Center).AlignItems(myui.Center).Gap(4).
					Cursor(myui.CursorPointer).Label(tool.Label).
					Transition(motion(springTap))
				switch {
				case active:
					btn.Gradient(st.Brand, st.BrandGradientTo, 90)
				case btn.Hovered():
					btn.Background(st.Recessed).Border(1, st.Brand)
				default:
					btn.Background(st.Recessed).Border(1, st.Divider)
				}
				btn.Children(func() {
					tint := st.TextSub
					if active {
						tint = myui.RGB(255, 255, 255)
					}
					myui.Icon(c, tool.Icon()).Width(13).Height(13).TextColor(tint)
					myui.Text(c, tool.Label).FontSize(11).TextColor(tint)
				})
				if btn.Clicked() {
					// The tool turns the canvas into a drawing surface: pressing it again
					// puts the canvas back to framing a cell, which is the state most of
					// the work happens in.
					if t.drawMode && t.markTool == index {
						t.drawMode = false
					} else {
						t.markTool = index
						t.drawMode = true
					}
				}
			}
		})
		if t.drawMode {
			if collageMarkTools[t.markTool].Kind == "text" {
				myui.Column(c).Gap(5).Children(func() {
					label(c, "标注文字")
					textField(c, &t.markText, "说明文字")
				})
			}
		}
		myui.Row(c).FillWidth().Gap(8).AlignItems(myui.Center).Children(func() {
			for i, hex := range collageMarkColors {
				index := i
				swatch := myui.Row(c).Size(20, 20).Radius(999).Background(colorOf(hex)).
					Cursor(myui.CursorPointer).Label("标注颜色 " + hex).
					Transition(motion(springTap))
				if t.markColor%len(collageMarkColors) == index {
					swatch.Border(2, st.Text)
				}
				if swatch.Clicked() {
					t.markColor = index
					// A selected mark takes the new colour; with nothing selected the
					// swatch only decides what the next mark looks like. Repainting every
					// mark made the toolbar read as a global paint bucket, which is not
					// what someone editing one arrow expects.
					if t.selectedMark >= 0 && t.selectedMark < len(t.marks) {
						t.marks[t.selectedMark].Color = collageMarkColors[index]
					}
					t.previewKey = ""
				}
			}
			myui.Spacer(c).Grow(1)
			status := "未选中标注"
			if t.selectedMark >= 0 && t.selectedMark < len(t.marks) {
				status = fmt.Sprintf("已选 %s", annotateKindLabel(t.marks[t.selectedMark].Kind))
			}
			statusPill(c, status, st.TextDim)
		})
		if t.selectedMark >= 0 && t.selectedMark < len(t.marks) {
			// Editing the selected mark: its text, its size and its removal, in one
			// row so the panel does not grow a form for every annotation.
			m := &t.marks[t.selectedMark]
			if m.Kind == "text" {
				myui.Column(c).Gap(5).Children(func() {
					label(c, "标注文字")
					before := m.Text
					textField(c, &m.Text, "说明文字")
					if m.Text != before {
						t.previewKey = ""
					}
				})
			}
			width := float64(m.Width)
			sliderRow(c, "线宽", &width, 1, 24, 1, "%.0f px")
			if int(width) != m.Width {
				m.Width = int(width)
				t.previewKey = ""
			}
		}
		// The actions get their own row: six swatches and three buttons do not
		// fit one 384 DIP column, and the last label was clipped to "清除 (".
		myui.Row(c).FillWidth().Gap(6).Children(func() {
			a.ghostButton(c, st, "随机", lucide.Dices(), func() {
				t.addRandomMark()
			})
			a.ghostButton(c, st, "撤销", lucide.Undo2(), func() {
				if len(t.marks) > 0 {
					t.marks = t.marks[:len(t.marks)-1]
					if t.selectedMark >= len(t.marks) {
						t.selectedMark = -1
					}
					t.previewKey = ""
				}
			})
			a.ghostButtonState(c, st, "删除选中", lucide.Trash2(),
				t.selectedMark < 0, false, func() {
					t.deleteSelectedMark()
				})
			a.ghostButton(c, st, "清除", lucide.Eraser(), func() {
				t.marks = nil
				t.selectedMark = -1
				t.previewKey = ""
			})
		})
	})
}

// addRandomMark drops a mark of a random kind, colour and place. The reference
// tool's 随机 button exists because picking a colour and dragging a box is three
// gestures for a decoration that is meant to look arbitrary anyway.
func (t *CollageTool) addRandomMark() {
	kind := collageMarkTools[t.randIntn(len(collageMarkTools))].Kind
	color := collageMarkColors[t.randIntn(len(collageMarkColors))]
	// Keep the mark inside the canvas with a margin, so it never lands half off
	// the picture where it would be invisible in the export.
	const pad = 0.08
	w := 0.14 + float64(t.randIntn(14))/100
	h := 0.10 + float64(t.randIntn(12))/100
	x := pad + float64(t.randIntn(60))/100
	y := pad + float64(t.randIntn(60))/100
	if x+w > 1-pad {
		x = 1 - pad - w
	}
	if y+h > 1-pad {
		y = 1 - pad - h
	}
	mark := engine.CollageMark{
		Kind: kind, X1: x, Y1: y, X2: x + w, Y2: y + h,
		Color: color, Width: 6 + t.randIntn(3),
	}
	switch kind {
	case "text":
		mark.Text = t.markText
		if strings.TrimSpace(mark.Text) == "" {
			mark.Text = "说明文字"
		}
		mark.Width = 6 + t.randIntn(4)
	case "arrow":
		// An arrow is a line with a head, not a box: the drag goes to its tip.
		mark.X2 = clampFloat64(mark.X1+w, 0, 1)
		mark.Y2 = clampFloat64(mark.Y1+h, 0, 1)
		offsetX := 0.05 + float64(t.randIntn(10))/100
		offsetY := 0.05 + float64(t.randIntn(10))/100
		mark.X1 = clampFloat64(mark.X2-offsetX, 0, 1)
		mark.Y1 = clampFloat64(mark.Y2-offsetY, 0, 1)
	}
	t.marks = append(t.marks, mark)
	t.previewKey = ""
}

// randIntn is a small generator for the 随机 button.
//
// math/rand's global source would be fine for a decoration, but the panel also
// runs under the tester, where a fixed sequence keeps a rendered frame stable and
// therefore comparable between runs. The state is seeded once, from the clock, and
// every call advances it.
func (t *CollageTool) randIntn(n int) int {
	if n <= 0 {
		return 0
	}
	if t.randState == 0 {
		t.seedRandom()
	}
	// xorshift32: an LCG's low bits have a short period, and reading those made
	// every press draw the same kind.
	t.randState ^= t.randState << 13
	t.randState ^= t.randState >> 17
	t.randState ^= t.randState << 5
	return int(uint64(t.randState) * uint64(n) >> 32)
}

// seedRandom re-seeds the generator from the clock, so two sessions draw
// different marks.
func (t *CollageTool) seedRandom() {
	t.randState = uint32(time.Now().UnixNano())
	if t.randState == 0 {
		t.randState = 0x9E3779B9
	}
}

// resetToDefaults puts every panel field back to what a fresh install shows and
// clears the marks, the framing and the slot overrides. It is the 恢复默认设置
// action of the reference tool: after a long session it is the only way back to a
// known state without restarting the app.
func (t *CollageTool) resetToDefaults() {
	grid := t.grid
	services := t.services
	if t.previewCancel != nil {
		t.previewCancel()
	}
	if t.copyCancel != nil {
		t.copyCancel()
	}
	*t = CollageTool{grid: grid, services: services, previewWork: t.previewWork, previewGen: t.previewGen + 1, copyGen: t.copyGen + 1}
	t.applyPersisted(settings.DefaultCollage())
	t.initialised = true
	t.maxBatch = 8
	t.selectedMark = -1
	t.markDrag = -1
}

// deleteSelectedMark removes the selected annotation.
func (t *CollageTool) deleteSelectedMark() {
	if t.selectedMark < 0 || t.selectedMark >= len(t.marks) {
		return
	}
	t.marks = append(t.marks[:t.selectedMark], t.marks[t.selectedMark+1:]...)
	t.selectedMark = -1
	t.markDrag = -1
	t.previewKey = ""
}

// annotateKindLabel resolves an annotation kind's label. It lives in
// tools_studio.go with the annotation toolbar, which is the table that covers every
// kind the app can draw; the collage page's four marks are a subset of it.

func (t *CollageTool) watermarkSection(c *myui.Context) {
	sectionCard(c, "水印", func() {
		switchField(c, &t.watermark, "平铺文字水印")
		if !t.watermark {
			return
		}
		myui.Column(c).Gap(5).Children(func() {
			label(c, "水印文字")
			textField(c, &t.watermarkText, "ImageFlow")
		})
		sliderRow(c, "字号", &t.watermarkSize, 10, 120, 1, "%.0f px")
		sliderRow(c, "透明度", &t.watermarkOpacity, 0.05, 1, 0.05, "%.2f")
		sliderRow(c, "旋转角度", &t.watermarkAngle, -90, 90, 5, "%.0f°")
		density := t.watermarkDensity
		segmented(c, &density, []string{"稀疏", "中等", "密集"})
		t.watermarkDensity = density
	})
}

// exportSection is the grouping mode, the encoder and the clipboard action.
func (t *CollageTool) exportSection(c *myui.Context, a *App) {
	st := styles(c)
	sectionCard(c, "导出", func() {
		mode := t.mode
		segmented(c, &mode, []string{"全部合成一张", "每 N 张一张"})
		t.mode = mode
		myui.Text(c, fmt.Sprintf("当前布局每张画布放 %d 张", t.slotCount())).
			FontSize(11).TextColor(st.TextDim)
		if t.mode == collagePerSlot {
			sliderRow(c, "最多输出组数", &t.maxBatch, 1, 64, 1, "%.0f")
		}
		format := t.format
		segmented(c, &format, []string{"PNG", "JPG", "WebP"})
		t.format = format
		if t.format != 0 {
			sliderRow(c, "质量", &t.quality, 1, 100, 1, "%.0f")
		}
		// Copying is a separate action from 开始处理: it writes no file, so it
		// must not create (or overwrite) anything in the output directory.
		myui.Row(c).FillWidth().Children(func() {
			a.ghostButtonState(c, st, "复制到剪贴板", lucide.ClipboardCopy(),
				a.Session.Len() == 0 || t.copying, t.copying, func() {
					t.copyToClipboard(a)
				})
		})
	})
}

// copyToClipboard composes the collage at the export size and puts its PNG on the
// clipboard.
//
// The compose runs on a goroutine; the clipboard write has to happen on the UI
// thread, so the encoded bytes travel back through Window.Update and only the
// write touches the system clipboard. A JPG or WebP export still copies PNG: a
// clipboard consumer expects a lossless picture, and the format ladder here
// chooses a file encoder, not a clipboard one.
func (t *CollageTool) copyToClipboard(a *App) {
	t.ensure()
	rows := a.Session.Snapshot()
	if len(rows) == 0 {
		a.SetStatus("先添加图片")
		return
	}
	if t.copying {
		return
	}
	t.copying = true
	cells := t.applyFrames(cellsFor(rows, t.slotCount()))
	req := t.buildRequest(cells, "")
	// Capture the service handle now: a Context may not outlive the frame, but the
	// handle may, and the clipboard write has to run on the UI thread.
	services := t.services
	a.SetStatus("正在合成到剪贴板…")
	var publish func(func())
	if a.Window != nil {
		publish = a.Window.Update
	}
	t.startCopy(a, req, services, publish)
}

func (t *CollageTool) startCopy(a *App, req engine.CollageRequest, services myui.Services, publish func(func())) {
	if t.copyCancel != nil {
		t.copyCancel()
	}
	t.copying = true
	ctx, cancel := context.WithCancel(context.Background())
	t.copyCancel = cancel
	t.copyGen++
	gen := t.copyGen
	compose := func() {
		img, res := renderCollageSafely(ctx, req)
		var encoded []byte
		var encodeErr error
		if res.Success && ctx.Err() == nil {
			encoded, encodeErr = encodeClipboardPNG(img)
		}
		commit := func() {
			defer cancel()
			if gen != t.copyGen || ctx.Err() != nil {
				return
			}
			if res.Success && encodeErr == nil && len(encoded) > 0 {
				services.WriteClipboardData(transfer.New(
					transfer.NewItem(transfer.Bytes(transfer.PNG, encoded))))
				a.SetStatus(fmt.Sprintf("已复制到剪贴板：%d × %d", res.Width, res.Height))
			} else {
				msg := res.Error
				if encodeErr != nil {
					msg = encodeErr.Error()
				}
				if msg == "" {
					msg = "编码失败"
				}
				a.SetStatus("复制失败：" + msg)
			}
			t.copying = false
			a.Invalidate()
		}
		if publish == nil {
			commit()
		} else {
			publish(commit)
		}
	}
	if publish == nil {
		compose()
	} else {
		go compose()
	}
}

func (a *App) CancelCollageWork() {
	for _, tool := range a.Tools {
		if collageTool, ok := tool.(*CollageTool); ok {
			if collageTool.previewCancel != nil {
				collageTool.previewCancel()
			}
			if collageTool.copyCancel != nil {
				collageTool.copyCancel()
			}
			collageTool.previewGen++
			collageTool.copyGen++
			collageTool.previewBusy = false
			collageTool.copying = false
		}
	}
}

// encodeClipboardPNG encodes the composed canvas for the clipboard.
func encodeClipboardPNG(img image.Image) ([]byte, error) {
	return codec.EncodeBytes(img, "png", codec.SaveOptions{SRGB: true})
}

// Tint returns c with its alpha scaled by k, which the editor uses for a merged
// block's translucent plate.
func Tint(c myui.Color, k float32) myui.Color {
	if k < 0 {
		k = 0
	}
	if k > 1 {
		k = 1
	}
	return myui.Color{R: c.R, G: c.G, B: c.B, A: uint8(float32(c.A) * k)}
}

func minFloat32(a, b float32) float32 {
	if a < b {
		return a
	}
	return b
}

func absFloat32(v float32) float32 {
	if v < 0 {
		return -v
	}
	return v
}

func clampFloat64(v, lo, hi float64) float64 {
	if v < lo {
		return lo
	}
	if v > hi {
		return hi
	}
	return v
}
