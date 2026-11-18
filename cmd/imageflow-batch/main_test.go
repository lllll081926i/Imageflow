package main

import (
	"bytes"
	"flag"
	"image"
	"image/png"
	"os"
	"os/exec"
	"path/filepath"
	"slices"
	"testing"
)

func TestBatchCommandHelper(t *testing.T) {
	separator := slices.Index(os.Args, "--")
	if separator < 0 {
		return
	}
	os.Args = append(os.Args[:1], os.Args[separator+1:]...)
	flag.CommandLine = flag.NewFlagSet(os.Args[0], flag.ExitOnError)
	main()
}

func TestBatchCommandPreservesExistingFiles(t *testing.T) {
	fixture := bytes.Buffer{}
	if err := png.Encode(&fixture, image.NewNRGBA(image.Rect(0, 0, 8, 8))); err != nil {
		t.Fatal(err)
	}
	for _, test := range []struct {
		tool, format, existing, renamed string
		stitch                          bool
	}{
		{"convert", "jpeg", "photo.jpg", "photo_1.jpg", false},
		{"resize", "tif", "photo.tiff", "photo_1.tiff", false},
		{"stitch", "png", "stitched.png", "stitched_1.png", true},
		{"pdf", "png", "document.pdf", "document_1.pdf", false},
	} {
		t.Run(test.tool, func(t *testing.T) {
			dir := t.TempDir()
			input := filepath.Join(dir, "photo.png")
			if err := os.WriteFile(input, fixture.Bytes(), 0600); err != nil {
				t.Fatal(err)
			}
			existing := filepath.Join(dir, test.existing)
			original := []byte("保留旧文件")
			if test.stitch {
				original = fixture.Bytes()
			}
			if err := os.WriteFile(existing, original, 0600); err != nil {
				t.Fatal(err)
			}
			args := []string{"-test.run=^TestBatchCommandHelper$", "--", "-tool", test.tool, "-format", test.format, "-out", dir, input}
			if test.stitch {
				args = append(args, existing)
			}
			if output, err := exec.Command(os.Args[0], args...).CombinedOutput(); err != nil {
				t.Fatalf("命令失败：%v\n%s", err, output)
			}
			current, err := os.ReadFile(existing)
			if err != nil || !bytes.Equal(current, original) {
				t.Fatalf("旧文件未保留：%v", err)
			}
			if _, err := os.Stat(filepath.Join(dir, test.renamed)); err != nil {
				t.Fatal(err)
			}
		})
	}
}
