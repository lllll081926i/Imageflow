package ui

// rail_drag.go implements the sidebar's long-press reorder: a hold lifts a row,
// the rows it passes over glide out of the way on a spring, and the drop commits
// the new order.
//
// The order is stored as a list of stable tool identifiers, so the active page, the
// selection pill and every tool's identity keep pointing at the same tool while
// only the positions move. A file written before identifiers existed stores
// positions instead; that list is migrated once, through the legacy index-to-ID
// table below, and then never written again.

import (
	"fmt"
	"math"
	"time"

	myui "github.com/egoist/mygo/ui"

	"github.com/lllll081926i/imageflow-go/internal/settings"
)

// railRowPitch is the distance between two row origins: the row plus the gap
// below it. The pill and the drag are positioned in the list's coordinate space,
// so they need the number the layout would otherwise derive from the children.
const railRowPitch = railRowHeight + railRowGap

// railLongPress is how long a press must hold still before the row lifts. 380 ms
// is long enough that a click never turns into a drag, and short enough that a
// deliberate hold does not feel stuck.
const railLongPress = 380 * time.Millisecond

// railLongPressSlop is how far the pointer may wander before the long press is
// abandoned. Without it, a hand that shakes two pixels while pressing would never
// start a drag.
const railLongPressSlop = 12

var (
	// railReorderSpring is the curve the other rows take out of the way. It has a
	// little bounce on purpose: a spring that leaves no overshoot reads as a jump
	// that was merely eased, while a visible settle says the row was displaced by
	// something.
	railReorderSpring = myui.Animation{
		Spring: myui.SpringOptions{Duration: 300 * time.Millisecond, Bounce: 0.22},
	}
	// railDragFollow is the lifted row's own curve. A spring following a moving
	// target oscillates, so the dragged row trails the pointer on a short ease
	// instead: it reads as a heavy card being carried, not as a lag.
	railDragFollow = myui.Animation{
		Duration: 110 * time.Millisecond,
		Ease:     myui.EaseOut,
	}
)

// ensureRailOrder fills the order with the natural order, and repairs one read
// from a settings file that no longer matches the tool list.
//
// The ids are the source of truth. A stale id (a page that was removed) is dropped,
// a missing tool is appended in its natural place, and a duplicate is ignored: a
// repaired order is still a permutation of the tools the build actually has.
func (a *App) ensureRailOrder() {
	want := len(a.Tools)
	byID := make(map[string]int, want)
	for i, tool := range a.Tools {
		byID[tool.ID()] = i
	}
	seen := make([]bool, want)
	order := make([]int, 0, want)
	for _, id := range a.railOrderIDs {
		idx, ok := byID[id]
		if !ok || seen[idx] {
			continue
		}
		seen[idx] = true
		order = append(order, idx)
	}
	for i := 0; i < want; i++ {
		if !seen[i] {
			order = append(order, i)
		}
	}
	a.railOrder = order
}

// migrateRailOrderIDs turns a pre-identifier saved order into identifiers.
//
// A file written by v3.1.0 stores positions into that version's tool list. The
// legacy table below is that list in its original order; a position is mapped to
// the identifier of the tool that used to sit there, so a user's arrangement
// survives the upgrade instead of being reinterpreted against the new list. A
// position outside the table (a hand-edited or truncated file) is dropped, which is
// why this must never panic on a bad file.
func migrateRailOrderIDs(legacy []int) []string {
	if len(legacy) == 0 {
		return nil
	}
	out := make([]string, 0, len(legacy))
	seen := map[string]bool{}
	for _, pos := range legacy {
		if pos < 0 || pos >= len(legacyToolIDs) {
			continue
		}
		id := legacyToolIDs[pos]
		if id == "" || seen[id] {
			continue
		}
		seen[id] = true
		out = append(out, id)
	}
	return out
}

// legacyToolIDs is the v3.1.0 rail order as identifiers, index for index. It is
// the migration table M0 requires: the list must never be edited, because a stored
// position refers to it and to nothing else.
var legacyToolIDs = []string{
	ToolConvert, ToolResize, ToolCrop, ToolGrid, ToolLayoutPage, ToolCollage,
	ToolStitch, ToolWatermark, ToolPalette, ToolIcon, ToolPDF, ToolGIF,
	ToolRichText, ToolCompress,
}

// railOrderSnapshot returns the row order: position → tool index.
func (a *App) railOrderSnapshot() []int {
	a.ensureRailOrder()
	return a.railOrder
}

// railPositionOf and railToolAt are the permutation lookups the order is built
// from. They are kept for the tests and for any caller that thinks in positions
// rather than in visible ordinals.

// railPositionOf returns where a tool sits in the saved order, or -1.
func (a *App) railPositionOf(tool int) int {
	for pos, idx := range a.railOrderSnapshot() {
		if idx == tool {
			return pos
		}
	}
	return -1
}

// railToolAt returns the tool a position holds, or -1.
func (a *App) railToolAt(pos int) int {
	order := a.railOrderSnapshot()
	if pos < 0 || pos >= len(order) {
		return -1
	}
	return order[pos]
}

// railToolCount is how many tool rows are currently visible.
func (a *App) railToolCount() int {
	count := 0
	for _, row := range a.railRows() {
		if !row.Header {
			count++
		}
	}
	return count
}

// railToolOrdinal is a tool's index among the visible tool rows, headers excluded.
func (a *App) railToolOrdinal(tool int) int {
	ordinal := 0
	for _, row := range a.railRows() {
		if row.Header {
			continue
		}
		if row.Tool == tool {
			return ordinal
		}
		ordinal++
	}
	return -1
}

// railToolAtOrdinal is the tool at one visible ordinal, or -1.
func (a *App) railToolAtOrdinal(ordinal int) int {
	index := 0
	for _, row := range a.railRows() {
		if row.Header {
			continue
		}
		if index == ordinal {
			return row.Tool
		}
		index++
	}
	return -1
}

// railDragYOfOrdinal is the planned top of one visible ordinal, used to clamp the
// carried row to the ends of the list.
func (a *App) railDragYOfOrdinal(ordinal int) float32 {
	index := 0
	for _, row := range a.railRows() {
		if row.Header {
			continue
		}
		if index == ordinal {
			return row.Y
		}
		index++
	}
	return 0
}

// resetRailPress forgets a press that will not become a drag.
func (a *App) resetRailPress() {
	a.railPressTool = -1
	a.railPressMoved = 0
}

// beginRailPress starts the long press for one row.
//
// No timestamp is taken here: an input handler sees the event outside a build, and
// a Context's clock is only meaningful for the frame it belongs to. The first build
// that sees the press stamps it instead (noticeRailLift).
func (a *App) beginRailPress(c *myui.Context, tool int, evY float32) {
	a.railPressTool = tool
	a.railPressAt = time.Time{}
	a.railPressMoved = 0
	a.railPressGrabY = evY
	// The pointer in list coordinates: the row's planned top plus its offset
	// inside the row. The planned top comes from the same table the layout uses,
	// so a group header between two rows is already accounted for.
	a.railPressListY = a.railToolRowY(tool) + evY
	// One frame has to arrive soon, so the press is stamped and, later, the long
	// press is noticed without waiting for an unrelated event.
	c.After(railLongPress)
}

// noticeRailLift turns a long enough press into a drag. It runs once per build of
// the pressed row, which keeps the state on the UI thread: the scheduled frame is
// only a wake-up.
func (a *App) noticeRailLift(c *myui.Context, pressed bool) {
	if a.railPressTool < 0 || a.railDragTool >= 0 {
		return
	}
	if !pressed {
		return
	}
	if a.railPressAt.IsZero() {
		// The first build that sees the press stamps it.
		a.railPressAt = c.Now()
		c.After(railLongPress)
		return
	}
	if c.Now().Sub(a.railPressAt) < railLongPress || a.railPressMoved > railLongPressSlop {
		// Not long enough yet, or the pointer wandered: keep watching.
		c.After(16 * time.Millisecond)
		return
	}
	pos := a.railToolOrdinal(a.railPressTool)
	if pos < 0 {
		a.resetRailPress()
		return
	}
	a.railDragTool = a.railPressTool
	a.railDragFromPos = pos
	a.railDragPos = pos
	a.railDragTo = pos
	a.railDragY = a.railDragYOfOrdinal(pos)
}

// railDragTargetFromPointer turns the carried row's position into the visible slot
// it would occupy.
//
// The travel is rounded, not truncated: half a row of movement is enough to mean
// the neighbour, which is what makes the gap open where the row is heading.
func (a *App) railDragTargetFromPointer() int {
	count := a.railToolCount()
	if count == 0 {
		return 0
	}
	travel := (a.railDragY - a.railPressListY + a.railPressGrabY) / railRowPitch
	pos := a.railDragFromPos + int(math.Round(float64(travel)))
	if pos < 0 {
		pos = 0
	}
	if pos > count-1 {
		pos = count - 1
	}
	return pos
}

// commitRailDrag writes the drag's result into the order and persists it.
//
// The move happens between two tool identities, not between two screen positions:
// the ordinals are resolved back to tools first, and the permutation is rewritten by
// removing the carried tool and inserting it before its target. That keeps the
// operation correct when the two tools belong to different groups, which is what a
// drag across a header actually means.
func (a *App) commitRailDrag() {
	from, to := a.railDragFromPos, a.railDragTo
	movedTool := a.railToolAtOrdinal(from)
	targetTool := a.railToolAtOrdinal(to)
	a.railDragTool = -1
	a.railDragFrom, a.railDragTo = -1, -1
	a.railDragFromPos, a.railDragPos = -1, -1
	a.resetRailPress()
	if movedTool < 0 || targetTool < 0 {
		return
	}
	order := append([]int(nil), a.railOrderSnapshot()...)
	// Remove the carried tool from its place.
	without := make([]int, 0, len(order))
	for _, idx := range order {
		if idx != movedTool {
			without = append(without, idx)
		}
	}
	// Insert it immediately before the tool it was dropped on.
	out := make([]int, 0, len(order))
	inserted := false
	for _, idx := range without {
		if idx == targetTool && !inserted {
			out = append(out, movedTool)
			inserted = true
		}
		out = append(out, idx)
	}
	if !inserted {
		out = append(out, movedTool)
	}
	a.railOrder = out
	a.persistRailOrder()
	a.Invalidate()
}

// persistRailOrder saves the new order. A reorder is a rare, deliberate gesture,
// so it is written at once rather than debounced.
func (a *App) persistRailOrder() {
	ids := make([]string, 0, len(a.railOrder))
	for _, idx := range a.railOrder {
		if idx < 0 || idx >= len(a.Tools) {
			continue
		}
		ids = append(ids, a.Tools[idx].ID())
	}
	a.railOrderIDs = append([]string(nil), ids...)
	a.Settings.RailOrderIDs = append([]string(nil), ids...)
	// The legacy index list is cleared: from this save on, the identifiers are the
	// only order that exists, and a stale index list would be replayed by a later
	// migration over the new order.
	a.Settings.RailOrder = nil
	a.settingsBase.RailOrderIDs = append([]string(nil), ids...)
	a.settingsBase.RailOrder = nil
	if err := a.Session.UpdateSettings(func(next *settings.Settings) {
		next.RailOrderIDs = append([]string(nil), ids...)
		next.RailOrder = nil
	}); err != nil {
		a.SetStatus("保存侧栏顺序失败：" + err.Error())
	}
}

// railRowKey names a row's animation channel. It follows the tool, not the
// position: a row that moves keeps its channel, so it glides instead of jumping
// to a new element's start value.
func railRowKey(tool int) string {
	return fmt.Sprintf("rail-row-top-%d", tool)
}
