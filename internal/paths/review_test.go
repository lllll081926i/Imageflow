package paths

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestSafeNames(t *testing.T) {
	for _, value := range []string{"../outside", `C:\folder\image`, "CON", "NUL.png", "COM1.txt", "...", "a\x00b", strings.Repeat("名", 200)} {
		name := SafeName(value)
		if name == "" || strings.ContainsAny(name, `<>:"/\|?*`) || name == "CON" || name == "NUL.png" || len([]rune(name)) > 101 {
			t.Fatalf("不安全的文件名：%q", name)
		}
	}
}

func TestContainmentRejectsSymlinkEscape(t *testing.T) {
	root, outside := t.TempDir(), t.TempDir()
	link := filepath.Join(root, "link")
	if err := os.Symlink(outside, link); err != nil {
		t.Skipf("当前系统不允许创建符号链接：%v", err)
	}
	if err := CheckContained(root, filepath.Join(link, "nested", "image.png")); err == nil {
		t.Fatal("符号链接越界未被拒绝")
	}
	if err := CheckContained(root, filepath.Join(root, "safe", "image.png")); err != nil {
		t.Fatal(err)
	}
}

func TestCanceledExpansionStopsBeforeTraversal(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	if result := ExpandContext(ctx, []string{t.TempDir()}); len(result.Files) != 0 || len(result.InputDirs) != 0 {
		t.Fatal("取消后仍继续展开目录")
	}
}
