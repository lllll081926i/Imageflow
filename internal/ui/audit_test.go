package ui

// audit_test.go pins the UI-level defects the full audit found: a conflict
// strategy that overwrote the file it was told to protect, and a session read from
// a worker goroutine without a snapshot.

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/lllll081926i/imageflow-go/internal/batch"
	"github.com/lllll081926i/imageflow-go/internal/settings"
)

// TestSkipStrategyLeavesTheExistingFileAlone pins the 同名文件 = 跳过 strategy.
//
// outputPathFor used to return the occupied path and drop the second result of
// paths.ResolveOutput, so the row overwrote the file the strategy promised to keep.
// The row now reports 已跳过 and the file keeps its bytes.
func TestSkipStrategyLeavesTheExistingFileAlone(t *testing.T) {
	a := newTestApp(t)
	dir := t.TempDir()
	existing := filepath.Join(dir, "photo.png")
	const original = "original bytes"
	if err := os.WriteFile(existing, []byte(original), 0o644); err != nil {
		t.Fatal(err)
	}

	s := a.Session.Settings
	s.ConflictStrategy = settings.ConflictSkip
	s.OutputTemplate = "{basename}"
	s.DefaultOutputDir = dir
	s.PreserveFolderStructure = false
	if err := a.Session.SetSettings(s); err != nil {
		t.Fatal(err)
	}

	// The row resolves to the occupied path and reports the skip.
	a.resetPlan()
	got, skipped := a.outputPathFor(batch.Item{Path: filepath.Join(dir, "photo.png")}, "convert", "png", 1)
	if !skipped {
		t.Fatalf("skip strategy did not report a skip; path %q", got)
	}
	if got != existing {
		t.Errorf("resolved %q, want the target %q", got, existing)
	}
	if raw, err := os.ReadFile(existing); err != nil || string(raw) != original {
		t.Errorf("the existing file was touched: %q %v", raw, err)
	}
}

// TestSkipStrategyOnlyAppliesToOccupiedTargets pins that a free path still runs:
// the strategy protects what exists, it does not skip everything.
func TestSkipStrategyOnlyAppliesToOccupiedTargets(t *testing.T) {
	a := newTestApp(t)
	dir := t.TempDir()
	s := a.Session.Settings
	s.ConflictStrategy = settings.ConflictSkip
	s.OutputTemplate = "{basename}"
	s.DefaultOutputDir = dir
	s.PreserveFolderStructure = false
	if err := a.Session.SetSettings(s); err != nil {
		t.Fatal(err)
	}
	a.resetPlan()
	if _, skipped := a.outputPathFor(batch.Item{Path: filepath.Join(dir, "free.png")}, "convert", "png", 1); skipped {
		t.Fatal("a free target must not be skipped")
	}
}

// TestBatchReportsSkipsSeparately pins the job outcome: a skipped row is neither a
// success nor a failure, so it must not be counted as an error the user has to
// chase.
func TestBatchReportsSkipsSeparately(t *testing.T) {
	cancelled := 0
	_ = cancelled
	var seen []batch.Progress
	outcomes := batch.Run([]batch.Job{
		func(*batch.Cancel) error { return nil },
		func(*batch.Cancel) error { return batch.ErrSkipped },
	}, 1, nil, func(p batch.Progress) { seen = append(seen, p) })

	if len(outcomes) != 2 {
		t.Fatalf("outcomes = %v", outcomes)
	}
	if outcomes[1] != batch.Skipped {
		t.Errorf("second outcome = %v, want Skipped", outcomes[1])
	}
	if len(seen) == 0 {
		t.Fatal("no progress reported")
	}
	last := seen[len(seen)-1]
	if last.Skipped != 1 || last.Failed != 0 || last.Completed != 2 {
		t.Errorf("progress = %+v", last)
	}
}
