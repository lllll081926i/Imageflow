package upscale

import (
	"context"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"sync/atomic"
	"testing"
)

func TestDownloadArchiveStaysInModelsDirectory(t *testing.T) {
	root := t.TempDir()
	var found atomic.Bool
	proxy := httptest.NewServer(http.HandlerFunc(func(writer http.ResponseWriter, request *http.Request) {
		entries, _ := filepath.Glob(filepath.Join(root, ".imageflow-upscayl-*"))
		found.Store(len(entries) == 1)
		writer.WriteHeader(http.StatusBadGateway)
	}))
	defer proxy.Close()
	_, err := DownloadModel(context.Background(), root, DownloadableModels()[0].Name,
		DownloadOptions{Source: "github", ProxyMode: "custom", ProxyURL: proxy.URL}, nil)
	if err == nil || !found.Load() {
		t.Fatalf("下载归档没有放在模型目录：%v", err)
	}
	entries, err := os.ReadDir(root)
	if err != nil || len(entries) != 0 {
		t.Fatalf("下载失败后遗留临时归档：%v %v", entries, err)
	}
}

func TestDownloadRejectsUnwritableDirectoryWithoutFallback(t *testing.T) {
	root := filepath.Join(t.TempDir(), "occupied")
	if err := os.WriteFile(root, []byte("不得覆盖"), 0600); err != nil {
		t.Fatal(err)
	}
	_, err := DownloadModel(context.Background(), root, DownloadableModels()[0].Name, DownloadOptions{ProxyMode: "direct"}, nil)
	if err == nil || !strings.Contains(err.Error(), "模型目录不可写") {
		t.Fatalf("不可写目录必须明确报错，不得回退：%v", err)
	}
}
