package output

import (
	"bytes"
	"fmt"
	"image"
	"image/color"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"testing"
)

func TestCheckSizeBounds(t *testing.T) {
	if err := CheckSize(100, 100); err != nil {
		t.Fatalf("valid size rejected: %v", err)
	}
	for _, c := range [][2]int{{0, 10}, {10, 0}, {-1, 5}, {MaxOutputSide + 1, 10}, {10, MaxOutputSide + 1}} {
		if err := CheckSize(c[0], c[1]); err == nil {
			t.Errorf("%dx%d accepted", c[0], c[1])
		}
	}
	if err := CheckSize(9000, 8000); err == nil {
		t.Error("area over budget accepted")
	}
}

func TestCheckImageRejectsNil(t *testing.T) {
	if err := CheckImage(nil); err == nil {
		t.Error("nil image accepted")
	}
	img := image.NewNRGBA(image.Rect(0, 0, 10, 10))
	if err := CheckImage(img); err != nil {
		t.Errorf("valid image rejected: %v", err)
	}
}

func TestAtomicKeepsOldFileOnFailure(t *testing.T) {
	dir := t.TempDir()
	target := filepath.Join(dir, "out.png")
	if err := os.WriteFile(target, []byte("original"), 0o644); err != nil {
		t.Fatal(err)
	}
	err := Atomic(target, func(f *os.File) error {
		if _, werr := f.Write([]byte("partial")); werr != nil {
			return werr
		}
		return os.ErrInvalid
	})
	if err == nil {
		t.Fatal("expected the writer error")
	}
	got, _ := os.ReadFile(target)
	if string(got) != "original" {
		t.Fatalf("target clobbered: %q", got)
	}
	entries, _ := os.ReadDir(dir)
	if len(entries) != 1 {
		t.Errorf("temporary file left behind: %v", entries)
	}
}

func TestAtomicReplacesOnSuccess(t *testing.T) {
	dir := t.TempDir()
	target := filepath.Join(dir, "out.bin")
	if err := Atomic(target, func(f *os.File) error {
		_, err := f.Write([]byte("new content"))
		return err
	}); err != nil {
		t.Fatal(err)
	}
	got, _ := os.ReadFile(target)
	if string(got) != "new content" {
		t.Fatalf("got %q", got)
	}
}

func TestCommitWritesNewFileDirectly(t *testing.T) {
	dir := t.TempDir()
	target := filepath.Join(dir, "fresh", "out.bin")
	if err := Commit(target, func(f *os.File) error {
		_, err := f.Write([]byte("hello"))
		return err
	}); err == nil {
		t.Fatal("missing parent directory should fail without MkdirAll")
	}
	if err := os.MkdirAll(filepath.Dir(target), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := Commit(target, func(f *os.File) error {
		_, err := f.Write([]byte("hello"))
		return err
	}); err != nil {
		t.Fatal(err)
	}
	got, _ := os.ReadFile(target)
	if string(got) != "hello" {
		t.Fatalf("got %q", got)
	}
}

// TestCommitConcurrentSameTarget pins that two workers writing the same new
// path cannot interleave into one file or delete each other's result. Commit
// used to os.Create the target directly when Stat said it did not exist, and
// the loser's deferred Remove erased the winner's output.
func TestCommitConcurrentSameTarget(t *testing.T) {
	dir := t.TempDir()
	target := filepath.Join(dir, "out.bin")

	payloads := make([][]byte, 8)
	for i := range payloads {
		payloads[i] = []byte(strings.Repeat(fmt.Sprintf("payload-%02d;", i), 64))
	}
	var wg sync.WaitGroup
	errs := make([]error, len(payloads))
	for i, p := range payloads {
		wg.Add(1)
		go func(i int, p []byte) {
			defer wg.Done()
			errs[i] = Commit(target, func(f *os.File) error {
				_, err := f.Write(p)
				return err
			})
		}(i, p)
	}
	wg.Wait()
	for i, err := range errs {
		if err != nil {
			t.Errorf("worker %d: %v", i, err)
		}
	}

	got, err := os.ReadFile(target)
	if err != nil {
		t.Fatalf("target missing after concurrent commits: %v", err)
	}
	matched := false
	for _, p := range payloads {
		if bytes.Equal(got, p) {
			matched = true
			break
		}
	}
	if !matched {
		t.Errorf("target holds %d bytes of interleaved/garbage data, want one complete payload", len(got))
	}
	entries, err := os.ReadDir(dir)
	if err != nil {
		t.Fatal(err)
	}
	if len(entries) != 1 {
		t.Errorf("temporary files left behind: %d entries", len(entries))
	}
}

var _ = color.RGBA{}
