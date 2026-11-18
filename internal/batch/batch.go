package batch

import (
	"context"
	"errors"
	"fmt"
	"sync"
	"sync/atomic"
	"time"
)

// Progress reports how many items finished so far.
type Progress struct {
	Completed int
	Total     int
	Failed    int
	Skipped   int
	Cancelled int
}

// Job is one unit of work in a batch.
type Job func(cancel *Cancel) error

// Cancel is a cooperative cancellation token shared by one batch run.
type Cancel struct {
	flag      atomic.Bool
	once      sync.Once
	done      chan struct{}
	closeOnce sync.Once
}

// Cancel marks the run as cancelled; workers check Cancelled between items.
func (c *Cancel) Cancel() {
	c.flag.Store(true)
	c.Done()
	c.closeOnce.Do(func() { close(c.done) })
}

// Done、Err、Deadline 和 Value 使取消令牌可直接传入支持 context 的引擎。
func (c *Cancel) Done() <-chan struct{} {
	c.once.Do(func() { c.done = make(chan struct{}) })
	return c.done
}
func (c *Cancel) Err() error {
	if c.Cancelled() {
		return context.Canceled
	}
	return nil
}
func (c *Cancel) Deadline() (time.Time, bool) { return time.Time{}, false }
func (c *Cancel) Value(any) any               { return nil }

// Cancelled reports whether the run was cancelled.
func (c *Cancel) Cancelled() bool { return c.flag.Load() }

// Outcome is the result of one job.
type Outcome int

const (
	Done Outcome = iota
	Failed
	Skipped
	Cancelled
)

// ErrSkipped is what a job returns when the conflict strategy left the row
// alone. It is not a failure: nothing was attempted, so the counters and the row
// status must say 已跳过 instead of 失败.
var ErrSkipped = errors.New("已跳过")

// Run executes jobs with at most concurrency workers and reports progress
// through onProgress, throttled to avoid flooding the UI thread.
//
// Cancellation is cooperative: workers stop taking new jobs, and jobs already
// running finish what they started so no partial file is left behind.
func Run(jobs []Job, concurrency int, cancel *Cancel, onProgress func(Progress)) []Outcome {
	results := make([]Outcome, len(jobs))
	if len(jobs) == 0 {
		return results
	}
	if concurrency < 1 {
		concurrency = 1
	}
	if concurrency > 32 {
		concurrency = 32
	}
	if concurrency > len(jobs) {
		concurrency = len(jobs)
	}

	var mu sync.Mutex
	progress := Progress{Total: len(jobs)}
	var next atomic.Int64
	var wg sync.WaitGroup

	// reportMu guards lastReport, which several workers touch; mu guards the
	// counters and the results slice.
	var reportMu sync.Mutex
	lastReport := time.Now()
	report := func(force bool) {
		if onProgress == nil {
			return
		}
		reportMu.Lock()
		defer reportMu.Unlock()
		now := time.Now()
		if !force && now.Sub(lastReport) < 80*time.Millisecond {
			return
		}
		lastReport = now
		mu.Lock()
		snapshot := progress
		mu.Unlock()
		onProgress(snapshot)
	}

	for w := 0; w < concurrency; w++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for {
				index := int(next.Add(1) - 1)
				if index >= len(jobs) {
					return
				}
				// Cancellation is claimed one index at a time. Marking "everything
				// from here on" from several workers at once double-counted
				// Completed and could outrun Total, because each worker saw a
				// different index.
				if cancel != nil && cancel.Cancelled() {
					mu.Lock()
					results[index] = Cancelled
					progress.Cancelled++
					progress.Completed++
					mu.Unlock()
					report(false)
					continue
				}
				err := Execute(jobs[index], cancel)
				mu.Lock()
				switch {
				case errors.Is(err, context.Canceled):
					results[index] = Cancelled
					progress.Cancelled++
				case errors.Is(err, ErrSkipped):
					results[index] = Skipped
					progress.Skipped++
				case err != nil:
					results[index] = Failed
					progress.Failed++
				default:
					results[index] = Done
				}
				progress.Completed++
				mu.Unlock()
				report(false)
			}
		}()
	}
	wg.Wait()
	report(true)
	return results
}

// Execute 将第三方编解码器的可恢复异常转换为单项失败。
func Execute(job Job, cancel *Cancel) (err error) {
	defer func() {
		if recovered := recover(); recovered != nil {
			err = fmt.Errorf("任务异常：%v", recovered)
		}
	}()
	return job(cancel)
}
