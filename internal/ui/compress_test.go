package ui

import (
	"os"
	"testing"
)

func TestRunCompressEndToEnd(t *testing.T) {
	a := newTestApp(t)
	seedInputs(t, a, 3)
	found := false
	for i, tool := range a.Tools {
		if _, ok := tool.(*CompressTool); ok {
			a.Active = i
			found = true
			break
		}
	}
	if !found {
		t.Fatal("侧边栏缺少图片压缩页")
	}
	a.Start()
	waitForRun(t, a)
	assertAllOK(t, a)
	for _, row := range a.Session.Snapshot() {
		in, err := os.Stat(row.Path)
		if err != nil {
			t.Fatal(err)
		}
		out, err := os.Stat(row.Output)
		if err != nil {
			t.Fatal(err)
		}
		if out.Size() > in.Size() || row.Message == "" {
			t.Fatalf("压缩大小或结果说明异常：%+v", row)
		}
	}
}
