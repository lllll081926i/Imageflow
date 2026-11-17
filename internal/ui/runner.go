package ui

import (
	"context"
	"fmt"
	"strings"

	"github.com/lllll081926i/imageflow-go/internal/batch"
	"github.com/lllll081926i/imageflow-go/internal/engine"
	"github.com/lllll081926i/imageflow-go/internal/paths"
	"github.com/lllll081926i/imageflow-go/internal/settings"
)

// runBatch executes one job per row and updates the shell state.
//
// runOne handles a single row and returns the engine envelope; the wrapper
// resolves the row status, keeps the counters and repaints the window.
func (a *App) runBatch(verb string, runOne func(index int, item batch.Item) engine.Result) {
	a.runBatchContext(verb, func(_ context.Context, index int, item batch.Item) engine.Result { return runOne(index, item) })
}

func (a *App) runBatchContext(verb string, runOne func(context.Context, int, batch.Item) engine.Result) {
	a.Session.ResetStatus()
	a.resetPlan()
	cancel := &batch.Cancel{}
	rows := a.Session.Snapshot()
	a.Session.Mu.Lock()
	a.Session.Cancel = cancel
	a.Session.Mu.Unlock()
	jobs := make([]batch.Job, len(rows))
	for i := range rows {
		index := i
		item := rows[i]
		jobs[i] = func(_ *batch.Cancel) error {
			var res engine.Result
			if err := batch.Execute(func(_ *batch.Cancel) error { res = runOne(cancel, index, item); return nil }, nil); err != nil {
				res = engine.Fail("TASK_PANIC", err.Error())
			}
			if !res.Success {
				if res.ErrorCode == "CANCELLED" {
					a.Session.SetItemStatus(item, "cancelled", "已取消", "")
					return context.Canceled
				}
				status, msg := batch.FailFromResult(res)
				a.Session.SetItemStatus(item, status, msg, "")
				return fmt.Errorf("%s", msg)
			}
			if res.Skipped {
				// The destination was protected by the conflict strategy; nothing was
				// written, so the row says so and leaves both counters alone.
				a.Session.SetItemStatus(item, "skipped", res.Message, "")
				return batch.ErrSkipped
			}
			// A tool that writes several files reports their folder; the row keeps
			// the first path so opening it still lands on something real.
			out := res.OutputPath
			if out == "" && len(res.Outputs) > 0 {
				out = res.Outputs[0]
			}
			a.Session.SetItemStatus(item, "ok", res.Message, out)
			return nil
		}
	}

	go func() {
		defer a.finishRun()
		results := batch.Run(jobs, a.Session.SettingsSnapshot().MaxConcurrency, cancel, func(p batch.Progress) {
			a.Session.Mu.Lock()
			a.Session.Progress = p
			a.Session.Mu.Unlock()
			percent := float32(0)
			if p.Total > 0 {
				percent = float32(p.Completed) / float32(p.Total) * 100
			}
			a.SetProgress(percent, fmt.Sprintf("%s %d/%d，失败 %d", verb, p.Completed, p.Total, p.Failed))
			a.Invalidate()
		})
		var ok, failed, skipped, cancelled int
		for i, r := range results {
			switch r {
			case batch.Done:
				ok++
			case batch.Failed:
				failed++
			case batch.Skipped:
				skipped++
			case batch.Cancelled:
				cancelled++
				// batch.Run never invokes the job for a cancelled row, so the
				// row's status has to be written here; otherwise it stays blank
				// and the cancellation is invisible in the file list.
				a.Session.SetItemStatus(rows[i], "cancelled", "已取消", "")
			}
		}
		a.Session.Mu.Lock()
		a.Session.Running = false
		a.Session.Cancel = nil
		a.Session.Mu.Unlock()
		switch {
		case cancelled > 0:
			// A cancelled run must not be reported as "完成": none of the
			// cancelled rows were ever processed.
			a.SetStatus(fmt.Sprintf("%s已取消：完成 %d 张，取消 %d 张", verb, ok, cancelled))
		case failed == 0 && skipped == 0:
			a.SetStatus(fmt.Sprintf("%s完成：%d 张", verb, ok))
		case failed == 0:
			a.SetStatus(fmt.Sprintf("%s完成：%d 张，跳过 %d", verb, ok, skipped))
		case skipped == 0:
			a.SetStatus(fmt.Sprintf("%s结束：成功 %d，失败 %d", verb, ok, failed))
		default:
			a.SetStatus(fmt.Sprintf("%s结束：成功 %d，失败 %d，跳过 %d", verb, ok, failed, skipped))
		}
		if a.Session.SettingsSnapshot().OpenOutputOnComplete && ok > 0 {
			a.openOutputFolder()
		}
		a.Invalidate()
	}()
}

// beginWholeList prepares the session for a tool that consumes every row in one
// engine call (拼接 / PDF). Those tools bypass runBatch, so they must still
// reset the reserved output paths and install a cancel token: without this, a
// previous batch's planned paths could make the conflict strategy skip the job,
// and 停止处理 had no token to signal at all.
func (a *App) beginWholeList() *batch.Cancel {
	a.Session.ResetStatus()
	a.resetPlan()
	cancel := &batch.Cancel{}
	a.Session.Mu.Lock()
	a.Session.Cancel = cancel
	a.Session.Mu.Unlock()
	return cancel
}

// endWholeList 释放取消令牌；只有未提交输出的取消结果才覆盖行状态。
func (a *App) endWholeList(cancel *batch.Cancel, rows []batch.Item, result ...engine.Result) bool {
	a.Session.Mu.Lock()
	a.Session.Cancel = nil
	a.Session.Mu.Unlock()
	if cancel == nil || !cancel.Cancelled() {
		return false
	}
	// 已提交的输出必须显示实际结果，不能被稍后到达的取消覆盖。
	if len(result) > 0 && result[0].ErrorCode != "CANCELLED" {
		return false
	}
	a.SetStatus("已取消：完成 0 张，取消 " + fmt.Sprint(len(rows)) + " 张")
	for _, item := range rows {
		a.Session.SetItemStatus(item, "cancelled", "已取消", "")
	}
	a.SetProgress(0, "")
	a.Invalidate()
	return true
}

// outputPathFor builds the destination for one row under the current settings,
// resolving name collisions against everything already planned in this batch and
// against the filesystem, per the configured conflict strategy.
//
// The second result reports that the row must be left alone because the target
// exists and the strategy is 跳过. The caller returns engine.Skip for it; before
// this the resolved path went straight to the encoder, so the 跳过 strategy wrote
// over the file it was supposed to protect.
func (a *App) outputPathFor(item batch.Item, op, ext string, seq int) (string, bool) {
	s := a.Session.SettingsSnapshot()
	base := batch.BuildOutputPath(s, item.Path, item.RelPath, op, ext, seq)
	return a.reserveOutput(base, s)
}

func (a *App) outputDirectoryFor(item batch.Item, suffix string) (string, bool) {
	s := a.Session.SettingsSnapshot()
	return a.reserveOutput(outputDirFor(s, item, suffix), s)
}

func (a *App) reserveOutput(base string, s settings.Settings) (string, bool) {
	if base == "" {
		return "", false
	}
	strategy := string(s.ConflictStrategy)
	a.planMu.Lock()
	defer a.planMu.Unlock()
	resolved, exists, err := paths.ResolveOutput(base, a.planned, strategy)
	if err != nil {
		return "", false
	}
	if strategy == string(settings.ConflictSkip) && exists {
		return resolved, true
	}
	a.planned = append(a.planned, resolved)
	return resolved, false
}

func (a *App) finishRun() {
	a.SetRunning(false)
	a.Invalidate()
}

// resetPlan clears the reserved paths before a batch starts.
func (a *App) resetPlan() {
	a.planMu.Lock()
	a.planned = a.planned[:0]
	a.planMu.Unlock()
}

func extOf(outputFormat string) string {
	switch strings.ToLower(strings.TrimSpace(outputFormat)) {
	case "jpg", "jpeg":
		return "jpg"
	case "png":
		return "png"
	case "webp":
		return "webp"
	case "avif":
		return "avif"
	case "tiff", "tif":
		return "tiff"
	case "bmp":
		return "bmp"
	case "ico":
		return "ico"
	case "gif":
		return "gif"
	case "apng":
		return "apng"
	case "pdf":
		return "pdf"
	}
	return "png"
}
