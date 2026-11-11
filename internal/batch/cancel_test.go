package batch_test

import (
	"sync/atomic"
	"testing"
	"time"

	"github.com/lllll081926i/imageflow-go/internal/batch"
)

// TestCancelledRunNeverReportsMoreThanTotal pins the progress counters on a
// cancelled run: several workers observe the token at once, and each must claim
// its own index, otherwise Completed can exceed Total and the progress bar
// overflows.
func TestCancelledRunNeverReportsMoreThanTotal(t *testing.T) {
	const jobs = 64
	work := make([]batch.Job, jobs)
	var ran atomic.Int64
	for i := range work {
		work[i] = func(*batch.Cancel) error {
			ran.Add(1)
			time.Sleep(2 * time.Millisecond)
			return nil
		}
	}
	cancel := &batch.Cancel{}
	// Cancel while the workers are already busy.
	time.AfterFunc(time.Millisecond, cancel.Cancel)

	var maxSeen batch.Progress
	outcomes := batch.Run(work, 8, cancel, func(p batch.Progress) {
		if p.Completed > maxSeen.Completed {
			maxSeen = p
		}
		if p.Completed > p.Total {
			t.Errorf("progress %+v reports more completed than total", p)
		}
	})

	done, cancelled := 0, 0
	for _, o := range outcomes {
		switch o {
		case batch.Done:
			done++
		case batch.Cancelled:
			cancelled++
		}
	}
	if done+cancelled != jobs {
		t.Fatalf("done=%d cancelled=%d, want %d items accounted for", done, cancelled, jobs)
	}
	if done != int(ran.Load()) {
		t.Errorf("done=%d but %d jobs ran", done, ran.Load())
	}
	if cancelled == 0 {
		t.Error("the run was never marked cancelled")
	}
	if maxSeen.Completed > maxSeen.Total {
		t.Errorf("final progress %+v", maxSeen)
	}
}
