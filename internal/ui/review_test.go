package ui

import (
	"testing"
	"time"

	myui "github.com/egoist/mygo/ui"
	"github.com/lllll081926i/imageflow-go/internal/batch"
	"github.com/lllll081926i/imageflow-go/internal/engine"
)

func TestImportCancellationRejectsQueuedResults(t *testing.T) {
	a := newTestApp(t)
	seedInputs(t, a, 1)
	path := a.Session.Snapshot()[0].Path
	a.Session.Clear()
	queue := make(chan func(), 8)
	a.importPaths([]string{path}, func(fn func()) { queue <- fn })
	var queued []func()
	for len(queued) < 2 {
		select {
		case fn := <-queue:
			queued = append(queued, fn)
		case <-time.After(10 * time.Second):
			t.Fatal("后台导入没有提交结果和完成通知")
		}
	}
	if !a.IsImporting() || a.Session.Len() != 0 {
		t.Fatal("窗口提交前导入状态提前结束")
	}
	a.CancelImport()
	for _, fn := range queued {
		fn()
	}
	if a.Session.Len() != 0 || a.IsImporting() {
		t.Fatal("取消后旧导入结果仍写入列表")
	}
}

func TestPanicUpdatesFailedRow(t *testing.T) {
	a := newTestApp(t)
	seedInputs(t, a, 1)
	a.SetRunning(true)
	a.runBatch("测试", func(int, batch.Item) engine.Result { panic("codec failed") })
	waitForRun(t, a)
	row := a.Session.Snapshot()[0]
	if row.Status != "failed" || row.Message == "" {
		t.Fatalf("异常未更新失败行：%+v", row)
	}
}

func TestCompressionPageHasNoInstructionsCard(t *testing.T) {
	a := newTestApp(t)
	for i, tool := range a.Tools {
		if _, ok := tool.(*CompressTool); ok {
			a.Active = i
			break
		}
	}
	tt := myui.NewTester(a.View, 1440, 900)
	if tt.HasText("处理说明") {
		t.Fatal("压缩页不应显示处理说明卡片")
	}
	if !tt.HasText("压缩设置") {
		t.Fatal("压缩控件未显示")
	}
}

func TestWholeListCancellationKeepsCommittedResult(t *testing.T) {
	a := newTestApp(t)
	seedInputs(t, a, 1)
	cancel := a.beginWholeList()
	cancel.Cancel()
	if a.endWholeList(cancel, 1, engine.Result{Success: true, OutputPath: "written.pdf"}) {
		t.Fatal("已提交输出被标记为取消")
	}
	if row := a.Session.Snapshot()[0]; row.Status == "cancelled" {
		t.Fatal("已完成的行被取消覆盖")
	}
	cancel = a.beginWholeList()
	cancel.Cancel()
	if !a.endWholeList(cancel, 1, engine.Fail("CANCELLED", "已取消")) {
		t.Fatal("未提交输出的取消结果未处理")
	}
	if row := a.Session.Snapshot()[0]; row.Status != "cancelled" {
		t.Fatalf("行状态=%s", row.Status)
	}
}
