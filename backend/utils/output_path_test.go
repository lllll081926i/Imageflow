package utils

import (
	"os"
	"path/filepath"
	"testing"
)

func TestResolveOutputPath_NoConflict(t *testing.T) {
	dir := t.TempDir()
	base := filepath.Join(dir, "image.png")
	got, err := ResolveOutputPath(base, nil)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if got != base {
		t.Fatalf("expected %s, got %s", base, got)
	}
}

func TestResolveOutputPath_ExistingFile(t *testing.T) {
	dir := t.TempDir()
	base := filepath.Join(dir, "image.png")
	if err := os.WriteFile(base, []byte("x"), 0o644); err != nil {
		t.Fatalf("failed to create file: %v", err)
	}
	got, err := ResolveOutputPath(base, nil)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	want := filepath.Join(dir, "image_01.png")
	if got != want {
		t.Fatalf("expected %s, got %s", want, got)
	}
}

func TestResolveOutputPath_Reserved(t *testing.T) {
	dir := t.TempDir()
	base := filepath.Join(dir, "image.png")
	reserved := map[string]struct{}{
		base:                               {},
		filepath.Join(dir, "image_01.png"): {},
	}
	got, err := ResolveOutputPath(base, reserved)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	want := filepath.Join(dir, "image_02.png")
	if got != want {
		t.Fatalf("expected %s, got %s", want, got)
	}
}

func TestResolveOutputPath_NoExt(t *testing.T) {
	dir := t.TempDir()
	base := filepath.Join(dir, "output")
	if err := os.WriteFile(base, []byte("x"), 0o644); err != nil {
		t.Fatalf("failed to create file: %v", err)
	}
	got, err := ResolveOutputPath(base, nil)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	want := filepath.Join(dir, "output_01")
	if got != want {
		t.Fatalf("expected %s, got %s", want, got)
	}
}
