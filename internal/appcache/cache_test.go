package appcache

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

func writeCacheFile(t *testing.T, path, content string) {
	t.Helper()
	if err := os.MkdirAll(filepath.Dir(path), 0o700); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, []byte(content), 0o600); err != nil {
		t.Fatal(err)
	}
}

func TestCacheStatsAndCleanupPreserveInputsModelsOutputsAndUnknownFiles(t *testing.T) {
	root, userCache := t.TempDir(), t.TempDir()
	thumb := filepath.Join(userCache, "imageflow", "thumb_cache", strings.Repeat("a", 64)+".png")
	tool := filepath.Join(userCache, "imageflow", "tools", "oxipng-9.1.5-421ce9013342", "oxipng.exe")
	old := filepath.Join(root, ".clipboard", "image-123.png")
	for _, path := range []string{thumb, tool, old} {
		writeCacheFile(t, path, "1234")
	}
	staging := filepath.Join(root, ".imageflow-upscayl-42", "model", "pending.bin")
	writeCacheFile(t, staging, "12345")
	var session Session
	current, err := session.Use(root, func(directory string) (string, error) {
		path := filepath.Join(directory, "image-456.png")
		writeCacheFile(t, path, "123456")
		return path, nil
	})
	if err != nil {
		t.Fatal(err)
	}
	defer session.Close()
	protected := []string{filepath.Join(root, "upscayl-standard-4x", "model.bin"),
		filepath.Join(root, ".trash", "legacy", "old.bin"), filepath.Join(root, ".clipboard", "IFimage-123.png"),
		filepath.Join(root, ".clipboard", "image-123_1.png"), filepath.Join(root, ".clipboard", "unknown.txt"),
		filepath.Join(userCache, "imageflow", "thumb_cache", "unknown.png"),
		filepath.Join(root, ".clipboard", "session-foreign", "image-789.png")}
	for _, path := range protected {
		writeCacheFile(t, path, "保留")
	}
	report, err := Inspect(context.Background(), root, userCache, session.Protected(), false)
	if err != nil || report.Total() != 23 || report.InUse != 6 || report.Files != 5 || report.Staging != 5 {
		t.Fatalf("缓存统计错误：%+v %v", report, err)
	}
	report, err = Inspect(context.Background(), root, userCache, session.Protected(), true)
	if err != nil || report.Removed != 17 || report.Total() != 6 || report.InUse != 6 {
		t.Fatalf("缓存清理错误：%+v %v", report, err)
	}
	for _, path := range append(protected, current) {
		if _, err := os.Stat(path); err != nil {
			t.Fatalf("清理删除了受保护文件：%s %v", path, err)
		}
	}
	if err := session.Close(); err != nil {
		t.Fatal(err)
	}
	if _, err := os.Stat(current); !os.IsNotExist(err) {
		t.Fatal("退出后仍保留临时原图")
	}
	if _, err := session.Use(root, func(string) (string, error) { return "", nil }); err == nil {
		t.Fatal("退出后仍接受缓存写入")
	}
}

func TestCacheExpiryAndSessionIsolation(t *testing.T) {
	root := t.TempDir()
	old := filepath.Join(root, ".clipboard", "image-1.png")
	fresh := filepath.Join(root, ".clipboard", "image-2.png")
	writeCacheFile(t, old, "old")
	writeCacheFile(t, fresh, "fresh")
	now := time.Now()
	if err := os.Chtimes(old, now.Add(-25*time.Hour), now.Add(-25*time.Hour)); err != nil {
		t.Fatal(err)
	}
	if err := PruneExpired(context.Background(), root, now); err != nil {
		t.Fatal(err)
	}
	if _, err := os.Stat(old); !os.IsNotExist(err) {
		t.Fatal("未清理过期临时原图")
	}
	var first, second Session
	create := func(session *Session) string {
		path, err := session.Use(root, func(directory string) (string, error) {
			path := filepath.Join(directory, "image-3.png")
			writeCacheFile(t, path, "缓存")
			return path, nil
		})
		if err != nil {
			t.Fatal(err)
		}
		return path
	}
	firstPath, secondPath := create(&first), create(&second)
	defer second.Close()
	if err := first.Close(); err != nil {
		t.Fatal(err)
	}
	if _, err := os.Stat(firstPath); !os.IsNotExist(err) {
		t.Fatal("本会话临时图片未清理")
	}
	for _, path := range []string{secondPath, fresh} {
		if _, err := os.Stat(path); err != nil {
			t.Fatalf("清理了其他会话或新鲜缓存：%s %v", path, err)
		}
	}
}

func TestCacheRejectsLinkedDirectoriesAndHonorsCancellation(t *testing.T) {
	root, outside := t.TempDir(), t.TempDir()
	outsideFile := filepath.Join(outside, "image-1.png")
	writeCacheFile(t, outsideFile, "保留")
	if err := os.Symlink(outside, filepath.Join(root, ".clipboard")); err == nil {
		if _, err := Inspect(context.Background(), root, "", nil, true); err == nil {
			t.Fatal("接受了符号链接缓存目录")
		}
		if _, err := os.Stat(outsideFile); err != nil {
			t.Fatal("符号链接目录外的文件被删除")
		}
	}
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	if _, err := Inspect(ctx, t.TempDir(), "", nil, true); err == nil {
		t.Fatal("缓存清理未响应取消")
	}
}
