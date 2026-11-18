package batch

import (
	"errors"
	"sync/atomic"
	"testing"
	"time"
)

func TestRunExecutesEveryJob(t *testing.T) {
	var counter atomic.Int64
	jobs := make([]Job, 20)
	for i := range jobs {
		jobs[i] = func(_ *Cancel) error {
			counter.Add(1)
			return nil
		}
	}
	var last Progress
	results := Run(jobs, 4, &Cancel{}, func(p Progress) { last = p })
	if counter.Load() != 20 {
		t.Fatalf("ran %d jobs", counter.Load())
	}
	for i, r := range results {
		if r != Done {
			t.Errorf("job %d = %v", i, r)
		}
	}
	if last.Completed != 20 || last.Total != 20 {
		t.Errorf("progress = %+v", last)
	}
}

func TestRunReportsFailures(t *testing.T) {
	jobs := []Job{
		func(_ *Cancel) error { return nil },
		func(_ *Cancel) error { return errors.New("boom") },
		func(_ *Cancel) error { return nil },
	}
	results := Run(jobs, 2, &Cancel{}, nil)
	if results[0] != Done || results[1] != Failed || results[2] != Done {
		t.Fatalf("results = %v", results)
	}
}

func TestRunStopsTakingJobsWhenCancelled(t *testing.T) {
	cancel := &Cancel{}
	var started atomic.Int64
	jobs := make([]Job, 50)
	for i := range jobs {
		jobs[i] = func(_ *Cancel) error {
			n := started.Add(1)
			if n == 2 {
				cancel.Cancel()
			}
			time.Sleep(2 * time.Millisecond)
			return nil
		}
	}
	results := Run(jobs, 1, cancel, nil)
	var cancelled int
	for _, r := range results {
		if r == Cancelled {
			cancelled++
		}
	}
	if cancelled == 0 {
		t.Fatalf("nothing was cancelled: %v", results)
	}
	if started.Load() >= int64(len(jobs)) {
		t.Errorf("kept starting jobs after cancel: %d", started.Load())
	}
}

func TestRunHandlesEmptyInput(t *testing.T) {
	if got := Run(nil, 4, &Cancel{}, nil); len(got) != 0 {
		t.Fatalf("got %v", got)
	}
}

func TestCancelledBatchThrottlesProgress(t *testing.T) {
	cancel := &Cancel{}
	cancel.Cancel()
	jobs := make([]Job, 1000)
	var snapshots []Progress
	Run(jobs, 4, cancel, func(progress Progress) { snapshots = append(snapshots, progress) })
	if len(snapshots) == 0 || len(snapshots) > 32 {
		t.Fatalf("取消刷新次数异常：%d", len(snapshots))
	}
	last := snapshots[len(snapshots)-1]
	if last.Completed != len(jobs) || last.Cancelled != len(jobs) {
		t.Fatalf("取消后的最终进度不完整：%+v", last)
	}
}

func TestRunClampsConcurrency(t *testing.T) {
	var concurrent, peak atomic.Int64
	jobs := make([]Job, 12)
	for i := range jobs {
		jobs[i] = func(_ *Cancel) error {
			now := concurrent.Add(1)
			for {
				old := peak.Load()
				if now <= old || peak.CompareAndSwap(old, now) {
					break
				}
			}
			time.Sleep(time.Millisecond)
			concurrent.Add(-1)
			return nil
		}
	}
	Run(jobs, 3, &Cancel{}, nil)
	// A purely serial Run would satisfy "<= 3" while silently losing the
	// parallelism the UI relies on, so bound it from below too.
	if peak.Load() < 2 {
		t.Errorf("peak concurrency = %d, want at least 2 workers in parallel", peak.Load())
	}
	if peak.Load() > 3 {
		t.Errorf("peak concurrency = %d, want <= 3", peak.Load())
	}
}
