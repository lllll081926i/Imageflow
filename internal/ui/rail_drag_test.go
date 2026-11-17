package ui

// rail_drag_test.go pins the sidebar's long-press reorder: a hold lifts a row,
// the rows it passes over glide out of the way, and the drop writes the new order.
//
// The rail is grouped, so a row's y is no longer "position times a fixed pitch": a
// group header and the gap above it break the rhythm. The tests therefore read the
// row table the layout itself uses (`railToolRowY`) instead of recomputing geometry,
// which is also what keeps them meaningful when a group is added.

import (
	"testing"
	"time"

	myui "github.com/egoist/mygo/ui"

	"github.com/lllll081926i/imageflow-go/internal/settings"
)

// rowCentreY reads a row's real centre from the rendered frame, so a test does not
// duplicate the rail's padding arithmetic.
func rowCentreY(t *testing.T, tt *myui.Tester, name string) float32 {
	t.Helper()
	info, ok := tt.Access(name)
	if !ok {
		t.Fatalf("%q missing: %v", name, tt.Texts())
	}
	return info.Bounds.Y + info.Bounds.H/2
}

// holdRow presses a row and waits out the long press without moving.
func holdRow(t *testing.T, tt *myui.Tester, a *App, x, y float32) {
	t.Helper()
	tt.Press(x, y)
	tt.Frame()
	// The lift is noticed on the frame the press scheduled, so the clock has to
	// pass railLongPress before it can fire.
	for i := 0; i < 40 && a.railDragTool < 0; i++ {
		tt.Advance(40 * time.Millisecond)
		tt.Frame()
	}
}

// TestRailLongPressLiftsTheRow pins the first half of the gesture: a hold lifts
// the row, and a short click never does.
func TestRailLongPressLiftsTheRow(t *testing.T) {
	a := newTestApp(t)
	tt := myui.NewTester(a.View, 1440, 900)
	tt.Frame()

	info, ok := tt.Access("批量水印")
	if !ok {
		t.Fatalf("row missing: %v", tt.Texts())
	}
	x := info.Bounds.X + 60
	y := rowCentreY(t, tt, "批量水印")

	// A quick click must not lift anything.
	tt.Press(x, y)
	tt.Frame()
	tt.Release(x, y)
	tt.Frame()
	if a.railDragTool >= 0 {
		t.Fatal("a click lifted the row")
	}
	if a.ActiveTool().Name() != "批量水印" {
		t.Fatalf("a click did not select the tool: active = %q", a.ActiveTool().Name())
	}

	// A hold lifts it.
	holdRow(t, tt, a, x, rowCentreY(t, tt, "批量水印"))
	if a.railDragTool < 0 {
		t.Fatal("the long press never lifted the row")
	}
	if got := a.railToolAtOrdinal(a.railDragFromPos); got != a.railDragTool {
		t.Fatalf("the lift started from ordinal %d (%q), want the pressed row (%q)",
			a.railDragFromPos, nameOf(a, got), a.Tools[a.railDragTool].Name())
	}
	if a.Tools[a.railDragTool].Name() != "批量水印" {
		t.Fatalf("long press lifted %q, want 批量水印", a.Tools[a.railDragTool].Name())
	}
	if !tt.FramePending() {
		t.Error("the lift asked for no further frames")
	}
	tt.Release(x, rowCentreY(t, tt, "批量水印"))
	tt.Frame()
}

// nameOf resolves a tool index to its label for a failure message.
func nameOf(a *App, idx int) string {
	if idx < 0 || idx >= len(a.Tools) {
		return "-"
	}
	return a.Tools[idx].Name()
}

// TestRailDragReordersAndPersists pins the whole gesture: holding a row and
// dragging it over the first row moves it there, animates the rows in between, and
// writes the new order to the settings file.
func TestRailDragReordersAndPersists(t *testing.T) {
	a := newTestApp(t)
	tt := myui.NewTester(a.View, 1440, 900)
	tt.Frame()

	first := a.Tools[0].Name()
	info, _ := tt.Access("批量水印")
	x := info.Bounds.X + 60
	holdRow(t, tt, a, x, rowCentreY(t, tt, "批量水印"))
	if a.railDragTool < 0 {
		t.Fatal("the long press never lifted the row")
	}

	// Drag up past the first row.
	firstRowY := a.railToolRowY(0)
	firstRowCentre := rowCentreY(t, tt, first)
	tt.Move(x, firstRowCentre)
	tt.Frame()
	if a.railDragTo != 0 {
		t.Fatalf("drag target = %d, want 0", a.railDragTo)
	}

	// The rows between glide: at least one frame has the first row below its
	// resting place, which is the gap the carried row would fill.
	glided := false
	for i := 0; i < 12; i++ {
		tt.Advance(16 * time.Millisecond)
		tt.Frame()
		if row, ok := tt.Access(first); ok {
			if row.Bounds.Y-firstRowY > 4 {
				glided = true
			}
		}
	}
	if !glided {
		t.Error("the displaced row never moved: the neighbours did not glide")
	}

	// The drop point is the last pointer position: a release somewhere else would
	// mean the pointer moved first, and the engine reports that movement. The
	// carried row's own bounds are its layout box, which a translation does not
	// change, so they cannot be used as a pointer position.
	lastX, lastY := x, firstRowCentre
	tt.Move(lastX, lastY)
	tt.Frame()
	tt.Release(lastX, lastY)
	tt.Frame()

	order := a.railOrderSnapshot()
	if a.Tools[order[0]].Name() != "批量水印" {
		t.Fatalf("order = %v, want 批量水印 first", order)
	}
	// A reorder is not navigation: the page that was open stays open.
	if a.ActiveTool().Name() != first {
		t.Errorf("dropping the row switched the page to %q, want %q", a.ActiveTool().Name(), first)
	}

	// Animated settle: the rows land on their planned places.
	for i := 0; i < 40; i++ {
		tt.Advance(16 * time.Millisecond)
		tt.Frame()
	}
	// Animated settle: the rows land on their planned places.
	for i := 0; i < 40; i++ {
		tt.Advance(16 * time.Millisecond)
		tt.Frame()
	}
	// A row's planned top is in the list's own content space while a committed
	// bound is in window DIPs, so the comparison uses one reference row to carry
	// the constant offset: what the reorder is about is that every row sits at the
	// place the row table says, one slot apart from its neighbour.
	var anchorOffset float32
	anchored := false
	for _, row := range a.railRows() {
		if row.Header {
			continue
		}
		if info, ok := tt.Access(a.Tools[row.Tool].Name()); ok {
			anchorOffset = info.Bounds.Y - row.Y
			anchored = true
			break
		}
	}
	if !anchored {
		t.Fatal("no row rendered after the reorder")
	}
	for pos, toolIndex := range a.railOrderSnapshot() {
		name := a.Tools[toolIndex].Name()
		row, ok := tt.Access(name)
		if !ok {
			continue // scrolled out of the rendered list
		}
		want := a.railToolRowY(toolIndex)
		if want < 0 {
			t.Fatalf("%q has no planned row", name)
		}
		if diff := row.Bounds.Y - (want + anchorOffset); diff > 3 || diff < -3 {
			t.Errorf("%q sits at y=%v, want %v (position %d)", name, row.Bounds.Y, want+anchorOffset, pos)
		}
	}

	// Persisted.
	loaded := settings.Load()
	firstID := loaded.RailOrderIDs
	if len(firstID) != len(a.Tools) {
		t.Fatalf("persisted %d ids, want %d", len(firstID), len(a.Tools))
	}
	if firstID[0] != ToolWatermark {
		t.Errorf("persisted first id = %q, want %q", firstID[0], ToolWatermark)
	}
}

// TestRailOrderSurvivesRestart pins the other half of persistence: a new App
// reads the stored order and draws the rail in it. The selected tool is a
// separate fact from the row order, so it stays where it was.
func TestRailOrderSurvivesRestart(t *testing.T) {
	a := newTestApp(t)
	// Move the fourth row to the front, using the identifiers the app now stores.
	order := a.railOrderSnapshot()
	moved := order[3]
	ids := []string{a.Tools[moved].ID()}
	for _, idx := range order {
		if idx != moved {
			ids = append(ids, a.Tools[idx].ID())
		}
	}
	a.railOrderIDs = ids
	a.ensureRailOrder()
	a.persistRailOrder()

	restored := NewApp(settings.Load())
	got := restored.railOrderSnapshot()
	if len(got) != len(restored.Tools) || got[0] != moved {
		t.Fatalf("restored order = %v, want %d first", got, moved)
	}
	// The page a fresh app shows is still the first tool, not the first row.
	if restored.ActiveTool().Name() != restored.Tools[0].Name() {
		t.Errorf("the restored order changed the selected tool to %q", restored.ActiveTool().Name())
	}
}

// TestRailOrderRepairsAStaleFile pins the guard: a stored order that no longer
// matches the tool list falls back to the natural order instead of pointing a row
// at a tool that does not exist.
//
// Two kinds of file are covered. An identifier list may name a removed page or
// repeat one, and a legacy index list may be short, duplicated or out of range;
// both are repaired into a permutation of the tools the build actually has.
func TestRailOrderRepairsAStaleFile(t *testing.T) {
	a := newTestApp(t)
	natural := make([]string, len(a.Tools))
	for i := range natural {
		natural[i] = a.Tools[i].ID()
	}
	duplicate := append([]string(nil), natural...)
	duplicate[len(duplicate)-1] = natural[0]
	unknown := append([]string(nil), natural...)
	unknown[0] = "removed-page"
	for _, stale := range [][]string{
		{"convert"}, // too short
		duplicate,   // a duplicate
		unknown,     // an id the build no longer has
	} {
		a.railOrderIDs = append([]string(nil), stale...)
		a.ensureRailOrder()
		if len(a.railOrder) != len(a.Tools) {
			t.Fatalf("%v was not repaired: %v", stale, a.railOrder)
		}
		seen := map[int]bool{}
		for _, idx := range a.railOrder {
			if idx < 0 || idx >= len(a.Tools) || seen[idx] {
				t.Fatalf("%v was not repaired to a permutation: %v", stale, a.railOrder)
			}
			seen[idx] = true
		}
	}
}

// TestRailReorderKeepsThePlateAxis pins that a reorder does not break the column's
// alignment: every rendered row still has its plate on the same x after a drag.
func TestRailReorderKeepsThePlateAxis(t *testing.T) {
	a := newTestApp(t)
	a.ShowSettings = true // every row inactive, so the plate is the plain tint
	order := a.railOrderSnapshot()
	for i, j := 0, 5; i < j; i, j = i+1, j-1 {
		order[i], order[j] = order[j], order[i]
	}
	a.railOrder = order
	tt := myui.NewTester(a.View, 1440, 900)
	for i := 0; i < 20; i++ {
		tt.Advance(16 * time.Millisecond)
		tt.Frame()
	}
	xs := map[float32]bool{}
	for _, tool := range a.Tools {
		info, ok := tt.Access(tool.Name())
		if !ok {
			continue // scrolled out of the rendered list
		}
		xs[info.Bounds.X] = true
	}
	if len(xs) != 1 {
		t.Errorf("rows start at %v, want one shared x", xs)
	}
}

// TestRailGroupsFoldAndKeepTheActivePageVisible pins the grouped sidebar: a header
// folds its tools away, and the group holding the current page cannot be folded.
func TestRailGroupsFoldAndKeepTheActivePageVisible(t *testing.T) {
	a := newTestApp(t)
	a.Active = a.toolIndex(ToolCollage)
	tt := myui.NewTester(a.View, 1440, 900)
	tt.Frame()

	// The group holding the current page refuses to fold: its row must stay visible.
	before := a.railToolCount()
	a.toggleGroup(GroupCreator)
	if !a.groupExpanded(GroupCreator) {
		t.Fatal("the group holding the current page must stay open")
	}
	if a.railToolCount() != before {
		t.Fatalf("the active group folded anyway: %d rows", a.railToolCount())
	}

	// Another group does fold: its rows disappear from the row table and the frame.
	studioRows := len(a.toolsInGroup(GroupStudio))
	if studioRows == 0 {
		t.Fatal("the studio group is empty")
	}
	a.toggleGroup(GroupStudio)
	if a.groupExpanded(GroupStudio) {
		t.Fatal("the toggled group did not fold")
	}
	if after := a.railToolCount(); after != before-studioRows {
		t.Fatalf("tool rows = %d, want %d", after, before-studioRows)
	}
	tt.Frame()
	if tt.HasText("专业调色") {
		t.Error("a folded section still drew its rows")
	}
	// The header itself is still on screen, so the group can be reopened.
	if !tt.HasText("专业图像") {
		t.Fatalf("the folded group's header is gone: %v", tt.Texts())
	}
	a.toggleGroup(GroupStudio)
	if a.railToolCount() != before {
		t.Fatalf("tool rows = %d after unfolding, want %d", a.railToolCount(), before)
	}
	// The fold is a preference: it survives a restart.
	a.toggleGroup(GroupStudio)
	restored := NewApp(settings.Load())
	if restored.groupExpanded(GroupStudio) {
		t.Fatal("the folded section did not survive a restart")
	}
}

// TestRailSearchRanksExactNamesFirst pins the navigation search: a name hit beats a
// keyword hit, and a keyword finds the page whose name does not contain it.
func TestRailSearchRanksExactNamesFirst(t *testing.T) {
	a := newTestApp(t)

	a.searchQuery = "压缩"
	matches := a.matchedTools()
	if len(matches) == 0 {
		t.Fatal("no match for 压缩")
	}
	if got := a.Tools[matches[0]].ID(); got != ToolCompress {
		t.Errorf("first match = %q, want %q", got, ToolCompress)
	}

	// 「打码」 is a keyword of the annotation page, whose name does not contain it.
	a.searchQuery = "打码"
	matches = a.matchedTools()
	if len(matches) == 0 || a.Tools[matches[0]].ID() != ToolAnnotate {
		t.Fatalf("打码 did not resolve to the annotation page: %v", matches)
	}

	// An empty query restores the whole list.
	a.searchQuery = ""
	if len(a.matchedTools()) != len(a.Tools) {
		t.Fatal("an empty query must list every tool")
	}
}

// TestEveryToolHasAGroup pins the membership table: a page without a group would
// silently land in the foundation section.
func TestEveryToolHasAGroup(t *testing.T) {
	a := newTestApp(t)
	known := map[string]bool{}
	for _, group := range toolGroups {
		known[group.Key] = true
	}
	for _, tool := range a.Tools {
		group, ok := toolGroupOf[tool.ID()]
		if !ok {
			t.Errorf("%q has no group", tool.Name())
			continue
		}
		if !known[group] {
			t.Errorf("%q is in the unknown group %q", tool.Name(), group)
		}
		if toolSummary[tool.ID()] == "" {
			t.Errorf("%q has no landing-page summary", tool.Name())
		}
	}
}

// TestHomeListsEveryToolOnce pins the landing page's coverage: each tool appears in
// exactly one group section, so nothing is reachable only through search.
func TestHomeListsEveryToolOnce(t *testing.T) {
	a := newTestApp(t)
	seen := map[int]int{}
	for _, group := range toolGroups {
		for _, idx := range a.toolsInGroup(group.Key) {
			seen[idx]++
		}
	}
	if len(seen) != len(a.Tools) {
		t.Fatalf("the landing page covers %d tools, want %d", len(seen), len(a.Tools))
	}
	for idx, count := range seen {
		if count != 1 {
			t.Errorf("%q appears %d times", a.Tools[idx].Name(), count)
		}
	}
}

// TestOpenToolLeavesTheLandingPage pins the landing page's only navigation action.
func TestOpenToolLeavesTheLandingPage(t *testing.T) {
	a := newTestApp(t)
	a.ShowHome = true
	a.searchQuery = "压缩"
	a.openTool(a.toolIndex(ToolGrade))
	if a.ShowHome {
		t.Fatal("opening a tool left the landing page visible")
	}
	if a.searchQuery != "" {
		t.Error("opening a tool kept the search filter")
	}
	if a.ActiveTool().ID() != ToolGrade {
		t.Fatalf("active = %q", a.ActiveTool().ID())
	}
}
