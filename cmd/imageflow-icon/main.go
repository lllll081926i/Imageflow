// Command imageflow-icon writes the application icon files from the compiled-in
// logo document.
//
// It exists because the packaging step reads resources/icon.png, and that file
// must be the same mark the window draws. Rendering it here, from
// internal/brand, means a change to the logo cannot leave a stale PNG behind for
// the installer to paint.
//
//	go run ./cmd/imageflow-icon          # rewrite resources/icon.png
//	go run ./cmd/imageflow-icon -out .   # write elsewhere
package main

import (
	"flag"
	"fmt"
	"image/png"
	"os"
	"path/filepath"

	"github.com/lllll081926i/imageflow-go/internal/brand"
	"github.com/lllll081926i/imageflow-go/internal/svg"
)

// iconEdge is the size the packaging step expects: a square PNG, 1024 is the
// largest size MyGo packs into a Windows icon group.
const iconEdge = 1024

// run returns the process exit code. Keeping os.Exit in main only means the
// deferred cleanup of the sibling temporary file runs on every failure path;
// calling os.Exit inside run skipped defers and left .icon-*.png files behind.
func run() int {
	out := flag.String("out", "resources", "directory to write icon.png into")
	flag.Parse()

	img, err := svg.RenderBytes([]byte(brand.IconSVG), iconEdge, iconEdge)
	if err != nil {
		fmt.Fprintln(os.Stderr, "render logo:", err)
		return 1
	}
	if err := os.MkdirAll(*out, 0o755); err != nil {
		fmt.Fprintln(os.Stderr, err)
		return 1
	}
	target := filepath.Join(*out, "icon.png")

	// Write through a sibling temporary so an interrupted run never leaves a
	// half-written PNG for the packager to embed.
	tmp, err := os.CreateTemp(*out, ".icon-*.png")
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		return 1
	}
	name := tmp.Name()
	defer os.Remove(name)
	if err := png.Encode(tmp, img); err != nil {
		tmp.Close()
		fmt.Fprintln(os.Stderr, "encode:", err)
		return 1
	}
	if err := tmp.Sync(); err != nil {
		tmp.Close()
		fmt.Fprintln(os.Stderr, err)
		return 1
	}
	if err := tmp.Close(); err != nil {
		fmt.Fprintln(os.Stderr, err)
		return 1
	}
	if err := os.Rename(name, target); err != nil {
		fmt.Fprintln(os.Stderr, err)
		return 1
	}

	info, err := os.Stat(target)
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		return 1
	}
	fmt.Printf("wrote %s  %dx%d  %d KB\n", target, iconEdge, iconEdge, info.Size()/1024)
	return 0
}

func main() {
	os.Exit(run())
}
