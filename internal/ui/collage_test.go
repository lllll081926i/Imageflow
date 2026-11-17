package ui

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/egoist/mygo/transfer"
	myui "github.com/egoist/mygo/ui"

	"github.com/lllll081926i/imageflow-go/internal/collage"
	"github.com/lllll081926i/imageflow-go/internal/engine"
	"github.com/lllll081926i/imageflow-go/internal/settings"
)

// TestCollageToolDefaults pins the first-frame state: a template that matches the
// default picture count, a 1:1 canvas and the collage page owning the preview.
func TestCollageToolDefaults(t *testing.T) {
	a := newTestApp(t)
	tool, ok := useTool(t, a, "图片拼图").(*CollageTool)
	if !ok {
		t.Fatal("图片拼图 is not a *CollageTool")
	}
	tool.ensure()
	if tool.groupCount != 4 {
		t.Errorf("group count = %d, want 4", tool.groupCount)
	}
	if got := tool.slotCount(); got != 4 {
		t.Errorf("slots = %d, want 4 for the default template", got)
	}
	if tool.aspect().ID != "1:1" {
		t.Errorf("aspect = %q, want 1:1", tool.aspect().ID)
	}
	if _, ok := any(tool).(previewWidget); !ok {
		t.Error("the collage must draw its own preview")
	}
}

// TestCollageToolRendersPage asserts the page draws its whole panel and its own
// preview canvas once there are files.
func TestCollageToolRendersPage(t *testing.T) {
	a := newTestApp(t)
	seedInputs(t, a, 4)
	useTool(t, a, "图片拼图")
	tt := myui.NewTester(a.View, 1440, 900)
	for _, want := range []string{"布局", "自定义网格", "画布", "样式", "标注", "水印", "导出",
		"全部合成一张", "每 N 张一张",
	} {
		if !tt.HasText(want) {
			t.Errorf("%q missing from the collage panel: %v", want, tt.Texts())
		}
	}
	if !tt.HasText("拼图预览") {
		t.Error("the collage must own the preview area")
	}
	// The shared single-picture preview belongs to the other pages; the collage
	// draws its own canvas instead.
	if tt.HasText("点击选择文件，或拖入文件夹") {
		t.Error("the empty drop hint is still drawn next to the collage canvas")
	}
	dumpImage(t, tt.Image(), "collage-panel")
}

// TestCollageToolRunsEndToEnd pins the whole path: four inputs on the default
// 4-cell template produce one file the encoder can read.
func TestCollageToolRunsEndToEnd(t *testing.T) {
	a := newTestApp(t)
	seedInputs(t, a, 4)
	tool := useTool(t, a, "图片拼图").(*CollageTool)
	tool.presetID = "4-grid"
	tool.aspectID = "1:1"
	tool.longEdge = 600
	tool.marginTop, tool.marginRight, tool.marginBottom, tool.marginLeft = 16, 16, 16, 16
	tool.gap = 8
	tool.format = 0

	a.Start()
	waitForRun(t, a)
	assertAllOK(t, a)

	rows := a.Session.Snapshot()
	outputs := map[string]bool{}
	for _, row := range rows {
		outputs[row.Output] = true
	}
	if len(outputs) != 1 {
		t.Fatalf("collage produced %d outputs, want 1: %v", len(outputs), outputs)
	}
	for path := range outputs {
		if filepath.Ext(path) != ".png" {
			t.Errorf("output %q is not a PNG", path)
		}
		if _, err := os.Stat(path); err != nil {
			t.Fatalf("output missing: %v", err)
		}
	}
}

// TestCollageExtraRowsAreReported pins the short-template rule: pictures that do
// not fit one canvas are marked 已跳过 instead of claiming to be composed.
func TestCollageExtraRowsAreReported(t *testing.T) {
	a := newTestApp(t)
	seedInputs(t, a, 5)
	tool := useTool(t, a, "图片拼图").(*CollageTool)
	tool.presetID = "4-grid"
	tool.mode = collageAllInOne
	tool.longEdge = 400

	a.Start()
	waitForRun(t, a)
	got := statuses(a)
	ok, skipped := 0, 0
	for _, s := range got {
		switch s {
		case "ok":
			ok++
		case "skipped":
			skipped++
		}
	}
	if ok != 4 || skipped != 1 {
		t.Fatalf("statuses = %v, want 4 ok and 1 skipped", got)
	}
	if !strings.Contains(a.StatusLine(), "超出") {
		t.Errorf("status = %q, want it to report the dropped picture", a.StatusLine())
	}
}

// TestCollageGroupOutputNamesFollowTheGroup pins the per-group naming: every group
// is named after its own first picture, so the files can be told apart, and with a
// {seq} template the group number is visible.
func TestCollageGroupOutputNamesFollowTheGroup(t *testing.T) {
	a := newTestApp(t)
	seedInputs(t, a, 8)
	tool := useTool(t, a, "图片拼图").(*CollageTool)
	tool.presetID = "4-grid"
	tool.mode = collagePerSlot
	tool.longEdge = 300
	tool.maxBatch = 16

	settings := a.Session.Settings
	settings.OutputTemplate = "{basename}"
	if err := a.Session.SetSettings(settings); err != nil {
		t.Fatal(err)
	}
	a.Start()
	waitForRun(t, a)

	rows := a.Session.Snapshot()
	if rows[0].Output == "" || rows[4].Output == "" {
		t.Fatalf("rows have no output: %q / %q", rows[0].Output, rows[4].Output)
	}
	if filepath.Base(rows[0].Output) != "img00.png" {
		t.Errorf("first group = %q, want img00.png", filepath.Base(rows[0].Output))
	}
	if filepath.Base(rows[4].Output) != "img04.png" {
		t.Errorf("second group = %q, want img04.png", filepath.Base(rows[4].Output))
	}
	if rows[0].Output == rows[4].Output {
		t.Error("both groups wrote the same file")
	}
}

// TestCollageGroupOutputUsesTheSequence pins the other half: with a {seq}
// template the group number reaches the name.
func TestCollageGroupOutputUsesTheSequence(t *testing.T) {
	a := newTestApp(t)
	seedInputs(t, a, 8)
	tool := useTool(t, a, "图片拼图").(*CollageTool)
	tool.presetID = "4-grid"
	tool.mode = collagePerSlot
	tool.longEdge = 300
	tool.maxBatch = 16

	settings := a.Session.Settings
	settings.OutputTemplate = "{basename}_{seq}"
	if err := a.Session.SetSettings(settings); err != nil {
		t.Fatal(err)
	}
	a.Start()
	waitForRun(t, a)

	rows := a.Session.Snapshot()
	if got := filepath.Base(rows[0].Output); got != "img00_1.png" {
		t.Errorf("first group = %q, want img00_1.png", got)
	}
	if got := filepath.Base(rows[4].Output); got != "img04_2.png" {
		t.Errorf("second group = %q, want img04_2.png", got)
	}
}

// TestCollageToolGroupsBySlot pins the second grouping mode: eight inputs on a
// four-cell template produce two files, and each row reports its own group.
func TestCollageToolGroupsBySlot(t *testing.T) {
	a := newTestApp(t)
	seedInputs(t, a, 8)
	tool := useTool(t, a, "图片拼图").(*CollageTool)
	tool.presetID = "4-grid"
	tool.mode = collagePerSlot
	tool.longEdge = 400
	tool.maxBatch = 16

	a.Start()
	waitForRun(t, a)
	assertAllOK(t, a)

	rows := a.Session.Snapshot()
	outputs := map[string]bool{}
	for _, row := range rows {
		outputs[row.Output] = true
	}
	if len(outputs) != 2 {
		t.Fatalf("two groups produced %d outputs, want 2: %v", len(outputs), outputs)
	}
	if rows[0].Output == rows[7].Output {
		t.Error("the last row still reports the first group's output")
	}
}

// TestCollageToolRejectsTooManyGroups pins the guard: a batch that would write
// more files than the limit is refused with a message instead of running.
func TestCollageToolRejectsTooManyGroups(t *testing.T) {
	a := newTestApp(t)
	seedInputs(t, a, 8)
	tool := useTool(t, a, "图片拼图").(*CollageTool)
	tool.presetID = "1-full"
	tool.mode = collagePerSlot
	tool.maxBatch = 2

	a.Start()
	if a.IsRunning() {
		t.Fatal("the run started despite the group limit")
	}
	if !strings.Contains(a.StatusLine(), "分组过多") {
		t.Fatalf("status = %q, want the group-limit message", a.StatusLine())
	}
}

// TestCollageToolCustomGrid pins the merge editor: merging two cells changes the
// layout the page sends to the engine.
func TestCollageToolCustomGrid(t *testing.T) {
	a := newTestApp(t)
	seedInputs(t, a, 2)
	tool := useTool(t, a, "图片拼图").(*CollageTool)
	tool.useCustom = true
	tool.grid.SetSize(2, 2)
	if got := tool.slotCount(); got != 4 {
		t.Fatalf("custom 2x2 slots = %d, want 4", got)
	}
	tool.grid.Merge(0, 0, 1, 0)
	if got := tool.slotCount(); got != 3 {
		t.Fatalf("after a merge slots = %d, want 3", got)
	}
	req := tool.buildRequest(nil, "")
	if len(req.Rects) != 3 {
		t.Fatalf("request carries %d cells, want 3", len(req.Rects))
	}
	if r := req.Rects[0]; r.W < 0.99 || r.H < 0.32 {
		t.Errorf("merged cell = %+v, want a full-width top row", r)
	}
}

// TestCollageToolPreviewComposes asserts the interactive canvas composes a
// bitmap from the selected template.
func TestCollageToolPreviewComposes(t *testing.T) {
	a := newTestApp(t)
	seedInputs(t, a, 3)
	tool := useTool(t, a, "图片拼图").(*CollageTool)
	tool.presetID = "3-v"
	tt := myui.NewTester(a.View, 1440, 900)
	waitForCollagePreview(t, tool, tt)
	if tool.previewBitmap == nil {
		t.Fatalf("no preview bitmap: %s", tool.previewErr)
	}
}

// waitForCollagePreview drives frames until the asynchronous preview lands.
func waitForCollagePreview(t *testing.T, tool *CollageTool, tt *myui.Tester) {
	t.Helper()
	deadline := time.Now().Add(10 * time.Second)
	for time.Now().Before(deadline) {
		tt.Advance(16 * time.Millisecond)
		tt.Frame()
		if tool.previewBitmap != nil || tool.previewErr != "" {
			return
		}
		time.Sleep(5 * time.Millisecond)
	}
	t.Fatalf("preview never composed: %s", tool.previewErr)
}

// TestCollageMarkToolsAddMarks pins the annotation toolbar: each tool drops one
// mark of its own kind once the canvas drag ends, and the canvas reports a draft
// while the press lasts.
func TestCollageMarkToolsAddMarks(t *testing.T) {
	a := newTestApp(t)
	seedInputs(t, a, 2)
	tool := useTool(t, a, "图片拼图").(*CollageTool)
	tool.presetID = "2-h"
	tt := myui.NewTester(a.View, 1440, 3300)
	waitForCollagePreview(t, tool, tt)

	if _, ok := tt.Access("拼图画布"); !ok {
		t.Fatalf("the preview canvas has no accessible label: %v", tt.Texts())
	}

	for i := range collageMarkTools {
		if err := tt.Click(collageMarkTools[i].Label); err != nil {
			t.Fatalf("click %s: %v", collageMarkTools[i].Label, err)
		}
		tt.Frame()
		if !tool.drawMode {
			t.Fatalf("%s did not arm the canvas for drawing", collageMarkTools[i].Label)
		}
		// Arming a text tool reveals its text field, which moves the canvas down:
		// the press has to re-read the box on every iteration.
		box, ok := tt.Access("拼图画布")
		if !ok {
			t.Fatalf("%s: the canvas vanished", collageMarkTools[i].Label)
		}
		// Each tool draws in its own corner: a press on an existing mark grabs that
		// mark instead of starting a new one, which is the feature being tested
		// elsewhere.
		frac := [2]float64{0.12 + 0.2*float64(i), 0.12 + 0.16*float64(i)}
		if frac[0] > 0.6 {
			frac[0], frac[1] = 0.1, 0.75
		}
		aspect := tool.aspect()
		canvasWidth, canvasHeight := collage.CanvasSize(aspect.W, aspect.H, 1000)
		picture := previewFitRect(box.Bounds, canvasWidth, canvasHeight)
		fromX := picture.X + picture.W*float32(frac[0])
		fromY := picture.Y + picture.H*float32(frac[1])
		toX := fromX + 40
		toY := fromY + 30
		tt.Press(fromX, fromY)
		tt.Move(toX, toY)
		if tool.draftMark == nil {
			t.Fatalf("%s: no mark is being dragged", collageMarkTools[i].Label)
		}
		tt.Release(toX, toY)
		tt.Frame()
	}
	if len(tool.marks) != len(collageMarkTools) {
		t.Fatalf("marks = %d, want %d", len(tool.marks), len(collageMarkTools))
	}
	seen := map[string]bool{}
	for _, m := range tool.marks {
		seen[m.Kind] = true
		if m.X2 <= m.X1 && m.Y2 <= m.Y1 {
			t.Errorf("%s mark has no extent: %+v", m.Kind, m)
		}
	}
	for _, want := range collageMarkTools {
		if !seen[want.Kind] {
			t.Errorf("no %s mark", want.Kind)
		}
	}
}

// TestCollageSlotOverrideReachesTheEngine pins the double-click replacement: a
// slot pointed at another file is the picture the request carries, both for the
// preview and for the export.
func TestCollageSlotOverrideReachesTheEngine(t *testing.T) {
	a := newTestApp(t)
	paths := seedInputs(t, a, 2)
	tool := useTool(t, a, "图片拼图").(*CollageTool)
	tool.presetID = "2-h"
	tool.overrides = map[int]string{1: paths[0]}
	cells := tool.applyFrames(cellsFor(a.Session.Snapshot(), tool.slotCount()))
	if len(cells) != 2 {
		t.Fatalf("cells = %d, want 2", len(cells))
	}
	if cells[1].Path != paths[0] {
		t.Errorf("slot 1 = %q, want the override %q", cells[1].Path, paths[0])
	}
	tool.resetCell(1)
	if len(tool.overrides) != 0 {
		t.Error("resetCell left the override in place")
	}
}

// TestCollageSeamModeDefaultsToSeamless pins the default the page ships with and
// the two effective values it resolves to.
func TestCollageSeamModeDefaultsToSeamless(t *testing.T) {
	a := newTestApp(t)
	tool := useTool(t, a, "图片拼图").(*CollageTool)
	tool.ensure()
	if tool.seam != collageSeamless {
		t.Fatalf("seam mode = %d, want the seamless default", tool.seam)
	}
	if tool.effectiveGap() != 0 || tool.effectiveRadius() != 0 {
		t.Fatalf("seamless resolves to gap %d radius %d, want 0 and 0",
			tool.effectiveGap(), tool.effectiveRadius())
	}
	// Switching to a seamed collage supplies a visible seam, and switching back
	// keeps the values the user chose.
	tool.seam = collageSeamed
	tool.gap, tool.radius = 14, 9
	tool.seam = collageSeamless
	if tool.effectiveGap() != 0 {
		t.Error("a seamless collage still reports a gap")
	}
	tool.seam = collageSeamed
	if tool.effectiveGap() != 14 || tool.effectiveRadius() != 9 {
		t.Errorf("seamed resolves to gap %d radius %d, want the saved 14 and 9",
			tool.effectiveGap(), tool.effectiveRadius())
	}
}

// TestCollageResetRestoresDefaults pins 恢复默认设置: every field goes back to the
// shipping state, the parts that must survive (the service handle) survive, and
// the panel keeps rendering.
func TestCollageResetRestoresDefaults(t *testing.T) {
	a := newTestApp(t)
	seedInputs(t, a, 4)
	tool := useTool(t, a, "图片拼图").(*CollageTool)
	// Dirty every field the reset is responsible for.
	tool.presetID = "4-grid"
	tool.useCustom = true
	tool.aspectID = "16:9"
	tool.longEdge = 3000
	tool.marginTop, tool.marginLeft = 40, 40
	tool.seam = collageSeamed
	tool.gap, tool.radius = 30, 20
	tool.fit = 2
	tool.bgHex = "#101010"
	tool.bgPath = "x.png"
	tool.mode = collagePerSlot
	tool.format = 2
	tool.quality = 40
	tool.watermark = true
	tool.marks = []engine.CollageMark{{Kind: "rect"}}
	tool.frames[0] = collageFrame{Scale: 2}
	tool.overrides[1] = "y.png"

	tool.resetToDefaults()

	if tool.useCustom || tool.aspectID != "1:1" || tool.longEdge != 1600 {
		t.Errorf("layout fields survived the reset: %+v", tool)
	}
	if tool.seam != collageSeamless || tool.effectiveGap() != 0 || tool.effectiveRadius() != 0 {
		t.Errorf("reset left a seam: seam=%d gap=%d radius=%d",
			tool.seam, tool.effectiveGap(), tool.effectiveRadius())
	}
	if len(tool.marks) != 0 || len(tool.frames) != 0 || len(tool.overrides) != 0 {
		t.Errorf("reset left edits behind: marks=%d frames=%d overrides=%d",
			len(tool.marks), len(tool.frames), len(tool.overrides))
	}
	if tool.mode != collageAllInOne || tool.format != 0 {
		t.Errorf("reset left the export mode at %d/%d", tool.mode, tool.format)
	}
	if tool.watermark || tool.bgPath != "" {
		t.Errorf("reset left decorations on: watermark=%v bg=%q", tool.watermark, tool.bgPath)
	}

	// The panel still draws: a reset that leaves the page blank would be worse
	// than the state it replaces.
	tt := myui.NewTester(a.View, 1440, 900)
	if !tt.HasText("恢复默认设置") || !tt.HasText("无接缝") {
		t.Errorf("the panel did not survive the reset: %v", tt.Texts())
	}
}

// TestCollageRandomMarksStayInsideTheCanvas pins the 随机 button: each press adds
// one mark of a known kind, and every mark stays inside the canvas so it is
// visible in the export.
func TestCollageRandomMarksStayInsideTheCanvas(t *testing.T) {
	a := newTestApp(t)
	tool := useTool(t, a, "图片拼图").(*CollageTool)
	tool.resetToDefaults()
	seen := map[string]bool{}
	for i := 0; i < 40; i++ {
		tool.addRandomMark()
	}
	if len(tool.marks) != 40 {
		t.Fatalf("marks = %d, want 40", len(tool.marks))
	}
	kinds := map[string]bool{}
	for _, toolDef := range collageMarkTools {
		kinds[toolDef.Kind] = true
	}
	for _, m := range tool.marks {
		if !kinds[m.Kind] {
			t.Errorf("unknown kind %q", m.Kind)
		}
		seen[m.Kind] = true
		for _, v := range []float64{m.X1, m.Y1, m.X2, m.Y2} {
			if v < 0 || v > 1 {
				t.Errorf("%s mark escapes the canvas: %+v", m.Kind, m)
			}
		}
		if m.X2 <= m.X1 || m.Y2 <= m.Y1 {
			t.Errorf("%s mark has no extent: %+v", m.Kind, m)
		}
		if m.Color == "" {
			t.Errorf("%s mark has no colour", m.Kind)
		}
	}
	if len(seen) < 2 {
		t.Errorf("the random button only ever drew %v", seen)
	}
}

// TestCollageCopyToClipboard pins the clipboard action end to end: the composed
// collage lands on the clipboard as a PNG, the status line reports its size, and
// the shortcut writes no file.
func TestCollageCopyToClipboard(t *testing.T) {
	a := newTestApp(t)
	seedInputs(t, a, 4)
	tool := useTool(t, a, "图片拼图").(*CollageTool)
	tool.presetID = "4-grid"
	tool.longEdge = 600
	tt := myui.NewTester(a.View, 1440, 3000)
	waitForCollagePreview(t, tool, tt)
	if err := tt.Click("复制到剪贴板"); err != nil {
		t.Fatal(err)
	}
	deadline := time.Now().Add(10 * time.Second)
	for time.Now().Before(deadline) {
		tt.Advance(16 * time.Millisecond)
		tt.Frame()
		data := tt.ClipboardData()
		if format, ok := data.Preferred(transfer.PNG); ok {
			raw, err := data.Read(format)
			if err != nil {
				t.Fatal(err)
			}
			if len(raw) < 100 {
				t.Fatalf("clipboard PNG is %d bytes", len(raw))
			}
			if !strings.Contains(a.StatusLine(), "剪贴板") {
				t.Errorf("status = %q, want it to mention the clipboard", a.StatusLine())
			}
			return
		}
	}
	t.Fatalf("clipboard stayed empty: %s", a.StatusLine())
}

// TestCollageCanvasArrowKeysNudgeTheSelectedCell pins the canvas keyboard
// handling: a press focuses the canvas, the arrows nudge the framing instead of
// stepping the file list, and Delete clears the framing.
func TestCollageCanvasArrowKeysNudgeTheSelectedCell(t *testing.T) {
	a := newTestApp(t)
	seedInputs(t, a, 4)
	tool := useTool(t, a, "图片拼图").(*CollageTool)
	tool.presetID = "4-grid"
	tool.gap = 0
	tt := myui.NewTester(a.View, 1440, 900)
	waitForCollagePreview(t, tool, tt)

	box, ok := tt.Access("拼图画布")
	if !ok {
		t.Fatalf("no canvas: %v", tt.Texts())
	}
	x, y := box.Bounds.X+box.Bounds.W*0.25, box.Bounds.Y+box.Bounds.H*0.25
	tt.Press(x, y)
	tt.Release(x, y)
	tt.Frame()
	before := a.selectedPreviewPath()

	tt.Key(0, myui.KeyRight)
	tt.Frame()
	if a.selectedPreviewPath() != before {
		t.Fatal("the arrow key stepped the file list instead of nudging the cell")
	}
	frame := tool.frames[tool.selectedCell]
	if frame.OffsetX <= 0 {
		t.Fatalf("offset X = %v, want it nudged right", frame.OffsetX)
	}

	tt.Key(0, myui.KeyDelete)
	tt.Frame()
	if _, still := tool.frames[tool.selectedCell]; still {
		t.Error("Delete did not clear the framing")
	}
}

// TestCollagePersistsAcrossApps pins the persistence end to end: a template, a
// canvas ratio, a seam and an annotation survive into the settings file and come
// back in a new App.
func TestCollagePersistsAcrossApps(t *testing.T) {
	a := newTestApp(t)
	tool := useTool(t, a, "图片拼图").(*CollageTool)
	tool.presetID = "3-t1b2"
	tool.useCustom = false
	tool.groupCount = 3
	tool.aspectID = "16:9"
	tool.longEdge = 2400
	tool.seam = collageSeamed
	tool.gap, tool.radius = 14, 9
	tool.marginTop = 24
	tool.watermark = true
	tool.watermarkText = "样板"
	tool.addRandomMark()

	// The panel is what schedules the write, so build one frame.
	tt := myui.NewTester(a.View, 1440, 900)
	tt.Frame()
	a.FlushCollage()

	loaded := settings.Load()
	if loaded.Collage.PresetID != "3-t1b2" || loaded.Collage.GroupCount != 3 {
		t.Fatalf("persisted template = %q / %d", loaded.Collage.PresetID, loaded.Collage.GroupCount)
	}
	if loaded.Collage.AspectID != "16:9" || loaded.Collage.LongEdge != 2400 {
		t.Fatalf("persisted canvas = %q / %d", loaded.Collage.AspectID, loaded.Collage.LongEdge)
	}
	if loaded.Collage.Seam != collageSeamed || loaded.Collage.Gap != 14 {
		t.Fatalf("persisted seam = %d / %d", loaded.Collage.Seam, loaded.Collage.Gap)
	}
	if len(loaded.Collage.Marks) != 1 {
		t.Fatalf("persisted marks = %d, want 1", len(loaded.Collage.Marks))
	}
	if !loaded.Collage.Watermark || loaded.Collage.WatermarkText != "样板" {
		t.Errorf("persisted watermark = %v / %q", loaded.Collage.Watermark, loaded.Collage.WatermarkText)
	}

	// A fresh app reads it back.
	restored := NewApp(loaded)
	tool = restored.toolByNameInternal("图片拼图").(*CollageTool)
	if tool.presetID != "3-t1b2" || tool.aspectID != "16:9" {
		t.Fatalf("restored tool = %q / %q", tool.presetID, tool.aspectID)
	}
	if tool.seam != collageSeamed || tool.effectiveGap() != 14 {
		t.Fatalf("restored seam = %d / %d", tool.seam, tool.effectiveGap())
	}
	if len(tool.marks) != 1 {
		t.Errorf("restored marks = %d, want 1", len(tool.marks))
	}
}

// TestCollagePersistsTheCustomGrid pins the merge editor's state: a hand-drawn
// layout is part of the template and must survive too.
func TestCollagePersistsTheCustomGrid(t *testing.T) {
	a := newTestApp(t)
	tool := useTool(t, a, "图片拼图").(*CollageTool)
	tool.useCustom = true
	tool.grid.SetSize(3, 2)
	tool.grid.Merge(0, 0, 1, 0)
	want := tool.slotCount()

	tt := myui.NewTester(a.View, 1440, 900)
	tt.Frame()
	a.FlushCollage()

	loaded := settings.Load()
	if !loaded.Collage.UseCustom || len(loaded.Collage.GridParents) != 6 {
		t.Fatalf("persisted grid = %v / %v", loaded.Collage.UseCustom, loaded.Collage.GridParents)
	}
	restored := NewApp(loaded).toolByNameInternal("图片拼图").(*CollageTool)
	if got := restored.slotCount(); got != want {
		t.Errorf("restored blocks = %d, want %d", got, want)
	}
}

// TestCollageResetAlsoClearsThePersistedState pins the interaction between the two:
// 恢复默认设置 must not be undone by the next launch.
func TestCollageResetAlsoClearsThePersistedState(t *testing.T) {
	a := newTestApp(t)
	tool := useTool(t, a, "图片拼图").(*CollageTool)
	tool.presetID = "9-grid3x3"
	tool.groupCount = 9
	tool.aspectID = "16:9"
	tt := myui.NewTester(a.View, 1440, 900)
	tt.Frame()
	a.FlushCollage()

	tool.resetToDefaults()
	tt.Frame()
	a.FlushCollage()

	loaded := settings.Load()
	if loaded.Collage.AspectID != "1:1" || loaded.Collage.GroupCount != 4 {
		t.Fatalf("settings still hold the old collage: %+v", loaded.Collage)
	}
}

// TestCollageMarkSelectionAndDrag pins the editing half of the annotations: a
// press on a mark selects it instead of starting a cell drag, and the drag moves
// it without changing its size.
func TestCollageMarkSelectionAndDrag(t *testing.T) {
	a := newTestApp(t)
	seedInputs(t, a, 2)
	tool := useTool(t, a, "图片拼图").(*CollageTool)
	tool.presetID = "2-h"
	tool.seam = collageSeamless
	tool.marks = []engine.CollageMark{{
		Kind: "rect", X1: 0.3, Y1: 0.3, X2: 0.5, Y2: 0.5, Color: "#EF4444", Width: 8,
	}}
	tt := myui.NewTester(a.View, 1440, 900)
	waitForCollagePreview(t, tool, tt)

	box, ok := tt.Access("拼图画布")
	if !ok {
		t.Fatalf("no canvas: %v", tt.Texts())
	}
	// The mark's top-left corner in window coordinates. The preview letterboxes the
	// canvas into the box, so the fraction has to be mapped through the fitted
	// rect, not through the whole card.
	cw, ch := collage.CanvasSize(tool.aspect().W, tool.aspect().H, 1000)
	fit := previewFitRect(myui.Rect{X: box.Bounds.X, Y: box.Bounds.Y, W: box.Bounds.W, H: box.Bounds.H}, cw, ch)
	hitX := fit.X + fit.W*0.3
	hitY := fit.Y + fit.H*0.3
	tt.Press(hitX, hitY)
	tt.Frame()
	if tool.selectedMark != 0 {
		t.Fatalf("pressing the mark selected %d, want 0", tool.selectedMark)
	}
	if tool.dragCell >= 0 {
		t.Error("pressing the mark also grabbed the cell behind it")
	}

	tt.Move(hitX+60, hitY+40)
	tt.Release(hitX+60, hitY+40)
	tt.Frame()
	m := tool.marks[0]
	if m.X1 <= 0.3 {
		t.Errorf("mark X1 = %v, want it moved right", m.X1)
	}
	if got := m.X2 - m.X1; got < 0.199 || got > 0.201 {
		t.Errorf("mark width = %v, want the original 0.2", got)
	}
	if tool.selectedMark != 0 {
		t.Error("the mark lost its selection after the drag")
	}

	// Outside every mark, a press selects the cell instead.
	away := previewFitRect(myui.Rect{X: box.Bounds.X, Y: box.Bounds.Y, W: box.Bounds.W, H: box.Bounds.H}, cw, ch)
	tt.Press(away.X+away.W*0.85, away.Y+away.H*0.85)
	tt.Release(away.X+away.W*0.85, away.Y+away.H*0.85)
	tt.Frame()
	if tool.selectedMark != -1 {
		t.Errorf("a press away from the marks kept mark %d selected", tool.selectedMark)
	}
}

// TestCollageDeleteSelectedMark pins the panel's delete action.
func TestCollageDeleteSelectedMark(t *testing.T) {
	a := newTestApp(t)
	tool := useTool(t, a, "图片拼图").(*CollageTool)
	tool.marks = []engine.CollageMark{
		{Kind: "rect", X1: 0.1, Y1: 0.1, X2: 0.3, Y2: 0.3, Color: "#EF4444", Width: 6},
		{Kind: "arrow", X1: 0.5, Y1: 0.5, X2: 0.7, Y2: 0.7, Color: "#22C55E", Width: 6},
	}
	tool.selectedMark = 0
	tool.deleteSelectedMark()
	if len(tool.marks) != 1 || tool.marks[0].Kind != "arrow" {
		t.Fatalf("marks = %+v, want only the arrow", tool.marks)
	}
	if tool.selectedMark != -1 {
		t.Errorf("selection = %d, want none", tool.selectedMark)
	}
}

// TestCollageEngineRequestMatchesThePanel pins that the request the page builds is
// one the engine accepts: a mismatched field name here is what makes the preview
// differ from the export.
func TestCollageEngineRequestMatchesThePanel(t *testing.T) {
	a := newTestApp(t)
	seedInputs(t, a, 2)
	tool := useTool(t, a, "图片拼图").(*CollageTool)
	tool.presetID = "2-h"
	tool.watermark = true
	tool.marks = []engine.CollageMark{{Kind: "rect", X1: 0.1, Y1: 0.1, X2: 0.5, Y2: 0.5, Color: "#FF0000", Width: 4}}
	cells := []engine.CollageCell{{Path: "a.png"}, {Path: "b.png"}}
	req := tool.buildRequest(cells, "")
	if len(req.Rects) != 2 {
		t.Fatalf("rects = %d, want 2", len(req.Rects))
	}
	if !req.Watermark.Enabled || req.Watermark.Text == "" {
		t.Error("the watermark switch did not reach the request")
	}
	if len(req.Marks) != 1 {
		t.Errorf("marks = %d, want 1", len(req.Marks))
	}
	if req.AspectW != 1 || req.AspectH != 1 {
		t.Errorf("aspect = %v:%v, want 1:1", req.AspectW, req.AspectH)
	}
}

// TestCollageCanvasTakesPointerInput pins the interactive canvas: a press selects
// a cell and a drag pans the picture inside it.
func TestCollageCanvasTakesPointerInput(t *testing.T) {
	a := newTestApp(t)
	seedInputs(t, a, 4)
	tool := useTool(t, a, "图片拼图").(*CollageTool)
	tool.presetID = "4-grid"
	tool.gap = 0
	tt := myui.NewTester(a.View, 1440, 900)
	waitForCollagePreview(t, tool, tt)

	box, ok := tt.Access("拼图画布")
	if !ok {
		t.Fatalf("the preview canvas has no accessible label: %v", tt.Texts())
	}
	x, y := box.Bounds.X+box.Bounds.W*0.25, box.Bounds.Y+box.Bounds.H*0.25
	tt.Press(x, y)
	tt.Release(x, y)
	tt.Frame()
	if tool.selectedCell != 0 {
		t.Fatalf("clicked cell = %d, want 0", tool.selectedCell)
	}

	tt.Press(x, y)
	tt.Move(x+40, y+20)
	tt.Release(x+40, y+20)
	tt.Frame()
	frame, tracked := tool.frames[0]
	if !tracked {
		t.Fatal("dragging did not record a framing change")
	}
	if frame.OffsetX >= 0 {
		t.Errorf("offset X = %v, want it moved opposite to the drag", frame.OffsetX)
	}
}

// TestCollageToolDarkModeRenders asserts the page keeps its surfaces in dark mode:
// a light card behind the canvas is what a missing token looks like.
func TestCollageToolDarkModeRenders(t *testing.T) {
	a := newTestApp(t)
	seedInputs(t, a, 2)
	tool := useTool(t, a, "图片拼图").(*CollageTool)
	tool.presetID = "2-h"
	tt := myui.NewTester(a.View, 1440, 900)
	tt.SetDark(true)
	waitForCollagePreview(t, tool, tt)
	img := tt.Image()
	if img == nil || hashes(img) < 3 {
		t.Fatal("dark collage frame is blank")
	}
	if luminance(img) > 120 {
		t.Fatalf("dark collage frame luminance = %.0f", luminance(img))
	}
	dumpImage(t, img, "collage-dark")
}
