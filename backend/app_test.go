package main

import (
	"strings"
	"testing"
)

func TestGetPreviewMaxBytes_Default(t *testing.T) {
	t.Setenv("IMAGEFLOW_PREVIEW_MAX_BYTES", "")
	got := getPreviewMaxBytes()
	if got != defaultPreviewMaxBytes {
		t.Fatalf("expected %d, got %d", defaultPreviewMaxBytes, got)
	}
}

func TestGetPreviewMaxBytes_Custom(t *testing.T) {
	t.Setenv("IMAGEFLOW_PREVIEW_MAX_BYTES", "12345")
	got := getPreviewMaxBytes()
	if got != 12345 {
		t.Fatalf("expected 12345, got %d", got)
	}
}

func TestDetectPreviewMimeType_Fallback(t *testing.T) {
	data := []byte{0x00, 0x01, 0x02}
	got := detectPreviewMimeType(data, "sample.png")
	if got != "image/png" {
		t.Fatalf("expected image/png, got %s", got)
	}
}

func TestBuildDataURL(t *testing.T) {
	data := []byte("abc")
	got := buildDataURL(data, "image/png")
	if !strings.HasPrefix(got, "data:image/png;base64,") {
		t.Fatalf("unexpected data url prefix: %s", got)
	}
}
