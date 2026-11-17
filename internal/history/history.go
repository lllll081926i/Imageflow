// Package history keeps the undo/redo stack of one editing session.
//
// The PRD's SYS-03 requires that editing never destroys the source and that the
// current session can undo and redo its parameter changes. This package stores
// parameter snapshots — not pixels — so a slider drag costs a few words of
// memory instead of an image copy. Each page owns one Stack and renders from
// whatever Current returns.
package history

// DefaultLimit is how many parameter snapshots one session keeps. A hundred steps
// is far more than a person reaches for, and the snapshots are small structs.
const DefaultLimit = 100

// Stack is a bounded undo/redo history over immutable snapshots.
//
// The zero value is not usable; build one with New. The type is not safe for
// concurrent use: every page edits it on the UI thread only.
type Stack[T any] struct {
	limit   int
	entries []T
	// cursor indexes the current entry. -1 means the stack holds nothing.
	cursor int
}

// New returns an empty stack that keeps at most limit snapshots. A limit below 1
// falls back to DefaultLimit.
func New[T any](limit int) *Stack[T] {
	if limit < 1 {
		limit = DefaultLimit
	}
	return &Stack[T]{limit: limit, cursor: -1}
}

// Reset starts a new history whose only entry is v: a page calls it when the
// source image changes, because the previous steps described another picture.
func (s *Stack[T]) Reset(v T) {
	s.entries = []T{v}
	s.cursor = 0
}

// Push records v as the new current state and drops the redo tail.
//
// Pushing the same snapshot twice would make Undo look broken (one press appears
// to do nothing), so callers that may build identical frames should use
// PushIfChanged with their own equality.
func (s *Stack[T]) Push(v T) {
	if s.cursor < 0 {
		s.Reset(v)
		return
	}
	// Drop everything after the cursor: a new edit invalidates the redo branch.
	s.entries = s.entries[:s.cursor+1]
	s.entries = append(s.entries, v)
	if len(s.entries) > s.limit {
		drop := len(s.entries) - s.limit
		s.entries = append(s.entries[:0], s.entries[drop:]...)
	}
	s.cursor = len(s.entries) - 1
}

// PushIfChanged records v only when equal reports a difference from the current
// snapshot.
func (s *Stack[T]) PushIfChanged(v T, equal func(a, b T) bool) bool {
	if current, ok := s.Current(); ok && equal != nil && equal(current, v) {
		return false
	}
	s.Push(v)
	return true
}

// Undo moves back one step and reports the restored snapshot. When there is
// nothing to undo it returns the zero value and false.
func (s *Stack[T]) Undo() (T, bool) {
	var zero T
	if s.cursor <= 0 {
		return zero, false
	}
	s.cursor--
	return s.entries[s.cursor], true
}

// Redo moves forward one step and reports the snapshot it lands on.
func (s *Stack[T]) Redo() (T, bool) {
	var zero T
	if s.cursor < 0 || s.cursor >= len(s.entries)-1 {
		return zero, false
	}
	s.cursor++
	return s.entries[s.cursor], true
}

// Current returns the snapshot in effect.
func (s *Stack[T]) Current() (T, bool) {
	var zero T
	if s.cursor < 0 || s.cursor >= len(s.entries) {
		return zero, false
	}
	return s.entries[s.cursor], true
}

// CanUndo reports whether Undo would move.
func (s *Stack[T]) CanUndo() bool { return s.cursor > 0 }

// CanRedo reports whether Redo would move.
func (s *Stack[T]) CanRedo() bool {
	return s.cursor >= 0 && s.cursor < len(s.entries)-1
}

// Len reports how many snapshots the stack holds in total, both directions.
func (s *Stack[T]) Len() int { return len(s.entries) }

// Position reports the cursor as a one-based step number and the total, which is
// what an "第 3 / 12 步" caption shows.
func (s *Stack[T]) Position() (step, total int) {
	if s.cursor < 0 {
		return 0, 0
	}
	return s.cursor + 1, len(s.entries)
}
