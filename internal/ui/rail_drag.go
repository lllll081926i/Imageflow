package ui

// rail_drag.go implements the sidebar's long-press reorder: a hold lifts a row,
// the rows it passes over glide out of the way on a spring, and the drop commits
// the new order.
//
// The order is stored as a permutation of tool indexes, so the active page, the
// selection pill and every tool's identity keep pointing at the same tool while
// only the positions move.

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

// ensureRailOrder fills the order with the identity permutation, and repairs one
// read from a settings file that no longer matches the tool list.
func (a *App) ensureRailOrder() {
	want := len(a.Tools)
	valid := len(a.railOrder) == want
	if valid {
		seen := make([]bool, want)
		for _, idx := range a.railOrder {
			if idx < 0 || idx >= want || seen[idx] {
				valid = false
				break
			}
			seen[idx] = true
		}
	}
	if valid {
		return
	}
	a.railOrder = make([]int, want)
	for i := range a.railOrder {
		a.railOrder[i] = i
	}
}

// railOrderSnapshot returns the row order: position → tool index.
func (a *App) railOrderSnapshot() []int {
	a.ensureRailOrder()
	return a.railOrder
}

// railPositionOf returns where a tool sits in the rail, or -1 when it is not in
// the list.
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
func (a *App) beginRailPress(c *myui.Context, tool, pos int, evY float32) {
	a.railPressTool = tool
	a.railPressAt = time.Time{}
	a.railPressMoved = 0
	a.railPressGrabY = evY
	// The pointer in list coordinates: the row's resting top plus its offset
	// inside the row.
	a.railPressListY = float32(pos)*railRowPitch + evY
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
	pos := a.railPositionOf(a.railPressTool)
	if pos < 0 {
		a.resetRailPress()
		return
	}
	a.railDragTool = a.railPressTool
	a.railDragFrom = pos
	a.railDragTo = pos
	a.railDragY = float32(pos) * railRowPitch
}

// railDragTargetFromPointer turns the carried row's position into the slot it
// would occupy.
//
// The travel is rounded, not truncated: half a row of movement is enough to mean
// the neighbour, which is what makes the gap open where the row is heading.
func (a *App) railDragTargetFromPointer() int {
	count := len(a.railOrderSnapshot())
	if count == 0 {
		return 0
	}
	travel := (a.railDragY - a.railPressListY + a.railPressGrabY) / railRowPitch
	pos := a.railDragFrom + int(math.Round(float64(travel)))
	if pos < 0 {
		pos = 0
	}
	if pos > count-1 {
		pos = count - 1
	}
	return pos
}

// commitRailDrag writes the drag's result into the order and persists it.
func (a *App) commitRailDrag() {
	from, to := a.railDragFrom, a.railDragTo
	a.railDragTool = -1
	a.railDragFrom, a.railDragTo = -1, -1
	a.resetRailPress()
	order := a.railOrderSnapshot()
	if from < 0 || from >= len(order) || to < 0 || to >= len(order) || from == to {
		return
	}
	moved := order[from]
	if from < to {
		// Moving down: everything between shifts up one place.
		copy(order[from:to], order[from+1:to+1])
	} else {
		// Moving up: everything between shifts down one place.
		copy(order[to+1:from+1], order[to:from])
	}
	order[to] = moved
	a.persistRailOrder()
	a.Invalidate()
}

// persistRailOrder saves the new order. A reorder is a rare, deliberate gesture,
// so it is written at once rather than debounced.
func (a *App) persistRailOrder() {
	order := append([]int(nil), a.railOrder...)
	a.Settings.RailOrder = order
	a.settingsBase.RailOrder = append([]int(nil), order...)
	if err := a.Session.UpdateSettings(func(next *settings.Settings) {
		next.RailOrder = append([]int(nil), order...)
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
