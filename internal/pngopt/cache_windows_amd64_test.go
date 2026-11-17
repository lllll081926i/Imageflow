package pngopt

import (
	"bytes"
	"context"
	"image"
	"image/png"
	"os"
	"testing"

	"github.com/lllll081926i/imageflow-go/internal/appcache"
)

func TestOptimizeRestoresToolAfterCacheCleanup(t *testing.T) {
	root := t.TempDir()
	t.Setenv("LOCALAPPDATA", root)
	var source bytes.Buffer
	if err := png.Encode(&source, image.NewNRGBA(image.Rect(0, 0, 16, 16))); err != nil {
		t.Fatal(err)
	}
	if _, err := Optimize(source.Bytes()); err != nil {
		t.Fatal(err)
	}
	path, err := executable()
	if err != nil {
		t.Fatal(err)
	}
	report, err := appcache.Inspect(context.Background(), "", root, nil, true)
	if err != nil || report.Removed == 0 {
		t.Fatalf("未清理内置压缩工具缓存：%+v %v", report, err)
	}
	if _, err := os.Stat(path); !os.IsNotExist(err) {
		t.Fatal("压缩工具缓存仍存在")
	}
	if _, err := Optimize(source.Bytes()); err != nil {
		t.Fatalf("清理缓存后压缩失效：%v", err)
	}
}
