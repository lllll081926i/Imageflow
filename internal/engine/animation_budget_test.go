package engine

import (
	"encoding/binary"
	"testing"
)

func TestAnimationBudgetChecksBeforeDecode(t *testing.T) {
	if err := animationSize(8000, 8000, 2); err == nil {
		t.Fatal("累计像素预算未生效")
	}
	if err := animationSize(10, 10, MaxAnimationFrames+1); err == nil {
		t.Fatal("帧数预算未生效")
	}
	if err := animationSize(32, 32, 10); err != nil {
		t.Fatal(err)
	}
	header := make([]byte, 13)
	binary.BigEndian.PutUint32(header, 32)
	binary.BigEndian.PutUint32(header[4:], 32)
	header[8], header[9] = 8, 6
	count := make([]byte, 8)
	binary.BigEndian.PutUint32(count, MaxAnimationFrames+1)
	raw := append([]byte("\x89PNG\r\n\x1a\n"), pngChunk("IHDR", header)...)
	raw = append(raw, pngChunk("acTL", count)...)
	if err := checkAnimation(raw); err == nil {
		t.Fatal("APNG 声明超量帧未被拒绝")
	}
	gif := []byte("GIF89a\x20\x4e\x20\x4e\x00\x00\x00\x3b")
	if err := checkAnimation(gif); err == nil {
		t.Fatal("GIF 超大画布未被拒绝")
	}
}
