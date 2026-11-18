package paths

import (
	"fmt"
	"os"
	"path/filepath"
	"runtime"
	"testing"
	"time"
)

func BenchmarkBatchOutputPlanning(bench *testing.B) {
	dir := bench.TempDir()
	targets := make([]string, 1000)
	for index := range targets {
		targets[index] = filepath.Join(dir, fmt.Sprintf("image-%04d.png", index))
	}
	bench.Run("逐项扫描", func(bench *testing.B) {
		for bench.Loop() {
			reserved := make([]string, 0, len(targets))
			for _, target := range targets {
				resolved, _, err := ResolveOutput(target, reserved, "rename")
				if err != nil {
					bench.Fatal(err)
				}
				reserved = append(reserved, resolved)
			}
		}
	})
	bench.Run("集合查询", func(bench *testing.B) {
		for bench.Loop() {
			var reserved OutputReservations
			for _, target := range targets {
				if _, _, err := reserved.Reserve(target, "rename"); err != nil {
					bench.Fatal(err)
				}
			}
		}
	})
}

func TestNormalizeRejectsBadPaths(t *testing.T) {
	if _, err := Normalize("   "); err == nil {
		t.Error("blank path accepted")
	}
	if _, err := Normalize("a\x00b"); err == nil {
		t.Error("NUL accepted")
	}
	if _, err := Normalize("../etc/passwd"); err == nil {
		t.Error("parent traversal accepted")
	}
	got, err := Normalize("  ./out/x.png  ")
	if err != nil {
		t.Fatalf("normal path rejected: %v", err)
	}
	if got != filepath.Clean("./out/x.png") {
		t.Errorf("got %q", got)
	}
}

func TestExpandWalksDirectoriesAndSkipsOthers(t *testing.T) {
	dir := t.TempDir()
	if err := os.MkdirAll(filepath.Join(dir, "sub"), 0o755); err != nil {
		t.Fatal(err)
	}
	write := func(name string) string {
		p := filepath.Join(dir, name)
		if err := os.WriteFile(p, []byte("x"), 0o644); err != nil {
			t.Fatal(err)
		}
		return p
	}
	a := write("a.png")
	b := write(filepath.Join("sub", "b.jpg"))
	bad := write("c.txt")

	res := Expand([]string{dir})
	if len(res.Files) != 2 {
		t.Fatalf("files = %+v", res.Files)
	}
	names := map[string]bool{}
	for _, f := range res.Files {
		names[f.Name] = true
		if f.Size != 1 {
			t.Errorf("%s size = %d", f.Name, f.Size)
		}
		if f.ModTimeNS == 0 {
			t.Errorf("%s has no mod time", f.Name)
		}
	}
	if !names["a.png"] || !names["b.jpg"] {
		t.Errorf("names = %v", names)
	}
	if len(res.InputDirs) != 1 || res.InputDirs[0] != dir {
		t.Errorf("dirs = %v", res.InputDirs)
	}
	// A file found inside the dropped folder keeps its path inside that folder,
	// which is what preserve_folder_structure rebuilds in the output tree.
	for _, f := range res.Files {
		switch f.Name {
		case "a.png":
			if f.RelPath != "a.png" {
				t.Errorf("a.png rel = %q", f.RelPath)
			}
		case "b.jpg":
			want := filepath.Join("sub", "b.jpg")
			if f.RelPath != want {
				t.Errorf("b.jpg rel = %q, want %q", f.RelPath, want)
			}
		}
	}
	if len(res.Skipped) != 1 || res.Skipped[0] != bad {
		t.Errorf("skipped = %v", res.Skipped)
	}
	_ = a
	_ = b
}

func TestExpandDedupesExplicitAndWalkedPaths(t *testing.T) {
	dir := t.TempDir()
	file := filepath.Join(dir, "one.png")
	if err := os.WriteFile(file, []byte("x"), 0o644); err != nil {
		t.Fatal(err)
	}
	res := Expand([]string{file, dir, file})
	if len(res.Files) != 1 {
		t.Fatalf("duplicates kept: %+v", res.Files)
	}
}

func TestNamingTemplate(t *testing.T) {
	n := Naming{Prefix: "IF", Template: "{prefix}_{basename}_{op}_{date:YYYYMMDD}_{seq:3}"}
	got := n.Render("/tmp/photo.png", "convert", 7, 2026, 11, 9, 13, 5, 9)
	want := "IF_photo_convert_20261109_007"
	if got != want {
		t.Fatalf("got %q want %q", got, want)
	}
}

func TestNamingEmptyTemplateFallsBackToBase(t *testing.T) {
	n := Naming{Prefix: "", Template: "   "}
	if got := n.Render("/tmp/photo.png", "op", 1, 2026, 1, 2, 3, 4, 5); got != "photo" {
		t.Fatalf("got %q", got)
	}
}

func TestResolveOutputStrategies(t *testing.T) {
	dir := t.TempDir()
	base := filepath.Join(dir, "out.png")
	if err := os.WriteFile(base, []byte("x"), 0o644); err != nil {
		t.Fatal(err)
	}

	if got, exists, err := ResolveOutput(base, nil, "overwrite"); err != nil || got != base || !exists {
		t.Fatalf("overwrite: %q %v %v", got, exists, err)
	}
	if got, exists, err := ResolveOutput(base, nil, "skip"); err != nil || got != base || !exists {
		t.Fatalf("skip: %q %v %v", got, exists, err)
	}
	got, exists, err := ResolveOutput(base, nil, "rename")
	if err != nil {
		t.Fatal(err)
	}
	if exists {
		t.Error("rename reported the file exists")
	}
	if got == base {
		t.Fatal("rename reused the occupied path")
	}
	if filepath.Ext(got) != ".png" {
		t.Errorf("extension lost: %q", got)
	}

	// A path reserved earlier in the same batch must be treated as taken.
	taken, _, err := ResolveOutput(base, []string{filepath.Join(dir, "out_1.png")}, "rename")
	if err != nil {
		t.Fatal(err)
	}
	if filepath.Base(taken) != "out_2.png" {
		t.Errorf("reserved path not honoured: %q", filepath.Base(taken))
	}
}

func TestSamePath(t *testing.T) {
	dir := t.TempDir()
	f := filepath.Join(dir, "a.png")
	if err := os.WriteFile(f, []byte("x"), 0o644); err != nil {
		t.Fatal(err)
	}
	if !SamePath(f, f) {
		t.Error("identical paths differ")
	}
	if SamePath(f, filepath.Join(dir, "b.png")) {
		t.Error("different paths match")
	}
	if runtime.GOOS == "windows" && !SamePath(f, stringsToUpper(f)) {
		t.Error("case-insensitive comparison failed on Windows")
	}
}

func stringsToUpper(s string) string {
	b := []byte(s)
	for i := range b {
		if b[i] >= 'a' && b[i] <= 'z' {
			b[i] -= 32
		}
	}
	return string(b)
}

var _ = time.Now
