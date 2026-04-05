package utils

import (
	"os"
	"path/filepath"
	"strings"
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

func TestNormalizeUserSuppliedPath_NormalizesSafeRelativePath(t *testing.T) {
	wd, err := os.Getwd()
	if err != nil {
		t.Fatalf("failed to get working directory: %v", err)
	}

	got, err := NormalizeUserSuppliedPath(filepath.Join("testdata", "sample.png"))
	if err != nil {
		t.Fatalf("expected safe relative path to be allowed, got %v", err)
	}
	want := filepath.Join(wd, "testdata", "sample.png")
	if got != want {
		t.Fatalf("expected normalized path %q, got %q", want, got)
	}
}

func TestNormalizeUserSuppliedPath_RejectsParentTraversal(t *testing.T) {
	_, err := NormalizeUserSuppliedPath(filepath.Join("..", "secret.png"))
	if err == nil {
		t.Fatal("expected parent traversal path to be rejected")
	}
	if !strings.Contains(err.Error(), "父级目录") {
		t.Fatalf("expected parent traversal error, got %q", err.Error())
	}
}

func TestNormalizeUserSuppliedPath_RejectsNULByte(t *testing.T) {
	_, err := NormalizeUserSuppliedPath("bad\x00path.png")
	if err == nil {
		t.Fatal("expected NUL byte path to be rejected")
	}
	if !strings.Contains(err.Error(), "空字符") {
		t.Fatalf("expected NUL byte error, got %q", err.Error())
	}
}
