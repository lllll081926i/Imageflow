package batch

import (
	"testing"

	"github.com/lllll081926i/imageflow-go/internal/settings"
)

func TestReorderFilesPreservesIdentityAndRejectsInvalidOrder(t *testing.T) {
	session := NewSession(settings.Default())
	first := &Item{Path: "first.png", id: 1, Status: "ok", Output: "result.png"}
	second := &Item{Path: "second.png", id: 2}
	session.Items = []*Item{first, second}
	if !session.ReorderFiles([]string{"second.png", "first.png"}) || session.Items[1] != first || session.Items[1].Output != "result.png" {
		t.Fatal("重排改变了文件身份或处理结果")
	}
	for _, order := range [][]string{{"first.png"}, {"first.png", "first.png"}, {"first.png", "unknown.png"}} {
		if session.ReorderFiles(order) || session.Items[0] != second {
			t.Fatalf("接受了非法或过期顺序：%v", order)
		}
	}
	session.Running = true
	if session.ReorderFiles([]string{"first.png", "second.png"}) {
		t.Fatal("运行中仍可重排")
	}
}
