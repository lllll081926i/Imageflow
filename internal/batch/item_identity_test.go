package batch

import (
	"testing"

	"github.com/lllll081926i/imageflow-go/internal/paths"
	"github.com/lllll081926i/imageflow-go/internal/settings"
)

func TestItemStatusRejectsObsoleteIdentity(t *testing.T) {
	for _, change := range []string{"删除", "清空", "新任务"} {
		t.Run(change, func(t *testing.T) {
			session := NewSession(settings.Default())
			files := []paths.InputFile{{Path: "first.png"}, {Path: "second.png"}}
			session.Add(files)
			session.ResetStatus()
			old := session.Snapshot()[0]
			switch change {
			case "删除":
				session.Remove(0)
			case "清空":
				session.Clear()
				session.Add(files[:1])
			case "新任务":
				session.ResetStatus()
			}
			session.SetItemStatus(old, "ok", "过期结果", "old.png")
			current := session.Snapshot()[0]
			if current.Status != "" || current.Output != "" {
				t.Fatalf("过期结果污染当前行：%+v", current)
			}
			session.SetItemStatus(current, "ok", "", "current.png")
			if session.Snapshot()[0].Output != "current.png" {
				t.Fatal("当前结果未提交")
			}
		})
	}
}
