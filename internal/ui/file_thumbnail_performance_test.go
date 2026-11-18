package ui

import (
	"bytes"
	"fmt"
	"image"
	"image/png"
	"testing"

	myui "github.com/egoist/mygo/ui"
	"github.com/lllll081926i/imageflow-go/internal/batch"
)

func testThumbnail(test testing.TB, size int) string {
	test.Helper()
	var buffer bytes.Buffer
	if err := png.Encode(&buffer, image.NewRGBA(image.Rect(0, 0, size, size))); err != nil {
		test.Fatal(err)
	}
	return buffer.String()
}

func TestFileThumbnailCacheReuseAndLimit(test *testing.T) {
	var cache fileThumbnailCache
	item := batch.Item{Path: "a.png", Thumb: testThumbnail(test, 64)}
	first := cache.bitmap(&item)
	if first == nil || cache.bitmap(&item) != first {
		test.Fatal("相同缩略图未复用位图")
	}
	item.Thumb = testThumbnail(test, 32)
	if updated := cache.bitmap(&item); updated == nil || updated == first {
		test.Fatal("原图更新后未更换位图")
	}
	for index := range fileThumbnailLimit {
		cache.bitmap(&batch.Item{Path: fmt.Sprintf("other-%d.png", index), Thumb: item.Thumb})
	}
	if _, found := cache.index[item.Path]; found || len(cache.index) != fileThumbnailLimit {
		test.Fatal("缩略图缓存未按容量淘汰旧项")
	}
}

func BenchmarkFileThumbnailRepaint(bench *testing.B) {
	item := batch.Item{Path: "photo.png", Thumb: testThumbnail(bench, 256)}
	bench.Run("重复解码", func(bench *testing.B) {
		for bench.Loop() {
			if _, err := myui.DecodeBitmap([]byte(item.Thumb)); err != nil {
				bench.Fatal(err)
			}
		}
	})
	bench.Run("复用位图", func(bench *testing.B) {
		var cache fileThumbnailCache
		cache.bitmap(&item)
		for bench.Loop() {
			if cache.bitmap(&item) == nil {
				bench.Fatal("未找到位图")
			}
		}
	})
}
