// Command imageflow-marks pre-renders the squares the window draws.
//
// internal/brand embeds the finished squares; this command is what produces them
// from the artwork. Running it is a build step after a logo change, not something
// the app does: that is why the resampling lives in internal/iconkit and not in
// the binary the installer ships.
//
//	go run ./cmd/imageflow-marks               # rewrite internal/brand/marks/
//	go run ./cmd/imageflow-marks -out temp    # write somewhere else
package main

import (
	"flag"
	"fmt"
	"os"
	"path/filepath"

	"github.com/lllll081926i/imageflow-go/internal/iconkit"
)

func main() {
	out := flag.String("out", filepath.Join("internal", "brand", "marks"),
		"directory to write the mark squares into")
	flag.Parse()

	src, err := iconkit.Source()
	if err != nil {
		fmt.Fprintln(os.Stderr, "decode artwork:", err)
		os.Exit(1)
	}
	if err := os.MkdirAll(*out, 0o755); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}

	for _, size := range iconkit.MarkSizes {
		img := iconkit.RenderCorrected(src, size)
		if img == nil {
			fmt.Fprintf(os.Stderr, "%d: 渲染失败\n", size)
			os.Exit(1)
		}
		name := filepath.Join(*out, fmt.Sprintf("mark-%dx%d.png", size, size))
		if err := writeAtomic(name, func(f *os.File) error {
			_, err := f.Write(iconkit.EncodePNG(img))
			return err
		}); err != nil {
			fmt.Fprintln(os.Stderr, err)
			os.Exit(1)
		}
		fmt.Printf("wrote %s\n", name)
	}
}

// writeAtomic writes through a sibling temporary, so an interrupted run never
// leaves a half-written PNG for the embed to pick up.
func writeAtomic(target string, write func(*os.File) error) error {
	tmp, err := os.CreateTemp(filepath.Dir(target), ".mark-*"+filepath.Ext(target))
	if err != nil {
		return err
	}
	name := tmp.Name()
	defer os.Remove(name)
	if err := write(tmp); err != nil {
		tmp.Close()
		return err
	}
	if err := tmp.Sync(); err != nil {
		tmp.Close()
		return err
	}
	if err := tmp.Close(); err != nil {
		return err
	}
	return os.Rename(name, target)
}
