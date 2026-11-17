package history

import "testing"

type snap struct {
	A float64
	B int
}

func same(a, b snap) bool { return a == b }

func TestUndoRedoWalksTheStack(t *testing.T) {
	s := New[snap](10)
	s.Reset(snap{})
	for i := 1; i <= 3; i++ {
		s.Push(snap{A: float64(i)})
	}
	if step, total := s.Position(); step != 4 || total != 4 {
		t.Fatalf("position = %d/%d", step, total)
	}
	for want := 3; want >= 1; want-- {
		got, ok := s.Undo()
		if !ok || got.A != float64(want-1) {
			t.Fatalf("undo %d = %+v ok=%v", want, got, ok)
		}
	}
	if s.CanUndo() {
		t.Fatal("undo available at the start")
	}
	if _, ok := s.Undo(); ok {
		t.Fatal("undo at the start reported success")
	}
	for want := 1; want <= 3; want++ {
		got, ok := s.Redo()
		if !ok || got.A != float64(want) {
			t.Fatalf("redo %d = %+v ok=%v", want, got, ok)
		}
	}
	if s.CanRedo() {
		t.Fatal("redo available at the tip")
	}
}

func TestPushAfterUndoDropsTheRedoTail(t *testing.T) {
	s := New[snap](10)
	s.Reset(snap{})
	s.Push(snap{A: 1})
	s.Push(snap{A: 2})
	if _, ok := s.Undo(); !ok {
		t.Fatal("undo failed")
	}
	if !s.CanRedo() {
		t.Fatal("redo missing after undo")
	}
	s.Push(snap{A: 9})
	if s.CanRedo() {
		t.Fatal("a new edit must drop the redo branch")
	}
	if step, total := s.Position(); step != 3 || total != 3 {
		t.Fatalf("position = %d/%d", step, total)
	}
}

func TestBoundedStackDropsTheOldest(t *testing.T) {
	s := New[snap](3)
	s.Reset(snap{})
	for i := 1; i <= 10; i++ {
		s.Push(snap{A: float64(i)})
	}
	if s.Len() != 3 {
		t.Fatalf("len = %d", s.Len())
	}
	steps := 0
	for s.CanUndo() {
		s.Undo()
		steps++
	}
	if steps != 2 {
		t.Fatalf("undo steps = %d, want 2 bounded steps", steps)
	}
	cur, ok := s.Current()
	if !ok || cur.A != 8 {
		t.Fatalf("oldest kept = %+v", cur)
	}
}

func TestPushIfChangedIgnoresEqualSnapshots(t *testing.T) {
	s := New[snap](10)
	s.Reset(snap{A: 1, B: 2})
	if s.PushIfChanged(snap{A: 1, B: 2}, same) {
		t.Fatal("equal snapshot was pushed")
	}
	if !s.PushIfChanged(snap{A: 2, B: 2}, same) {
		t.Fatal("changed snapshot was not pushed")
	}
	if s.Len() != 2 {
		t.Fatalf("len = %d", s.Len())
	}
}

func TestZeroLimitFallsBackToDefault(t *testing.T) {
	s := New[snap](0)
	s.Reset(snap{})
	for i := 0; i < DefaultLimit+5; i++ {
		s.Push(snap{A: float64(i)})
	}
	if s.Len() != DefaultLimit {
		t.Fatalf("len = %d, want %d", s.Len(), DefaultLimit)
	}
}

func TestEmptyStackReportsNothing(t *testing.T) {
	s := New[snap](4)
	if _, ok := s.Current(); ok {
		t.Fatal("empty stack reported a current value")
	}
	if s.CanUndo() || s.CanRedo() {
		t.Fatal("empty stack reported history")
	}
	if step, total := s.Position(); step != 0 || total != 0 {
		t.Fatalf("position = %d/%d", step, total)
	}
}
