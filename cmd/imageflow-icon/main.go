// Command imageflow-icon writes the application's icon files from the embedded
// artwork.
//
// The packager reads resources/icon.png, and that file must be the same mark the
// window draws. Rendering it here, from internal/brand, means a change to the
// artwork cannot leave a stale PNG behind for the installer to paint.
//
//	go run ./cmd/imageflow-icon            # rewrite resources/
//	go run ./cmd/imageflow-icon -out . -sizes 16,32,256
package main

import (
	"flag"
	"fmt"
	"image"
	"image/png"
	"os"
	"path/filepath"
	"strconv"
	"strings"

	ico "github.com/sergeymakinen/go-ico"

	"github.com/lllll081926i/imageflow-go/internal/brand"
)

// run returns the process exit code. Keeping os.Exit in main only means every
// failure path still runs the deferred cleanup, which is what used to leave
// .icon-*.png siblings behind.
func run() int {
	out := flag.String("out", "resources", "directory to write the icon files into")
	ladder := flag.String("sizes", "", "comma-separated PNG sizes (default: the brand ladder)")
	flag.Parse()

	// Fail before writing anything if the embedded artwork cannot be decoded: the
	// rest of the run renders from it, and a corrupt asset would otherwise leave a
	// half-updated resources/ behind.
	if _, err := brand.Logo(); err != nil {
		fmt.Fprintln(os.Stderr, "decode logo:", err)
		return 1
	}
	if *out == "" {
		fmt.Fprintln(os.Stderr, "no output directory")
		return 1
	}
	icons := filepath.Join(*out, "icons")
	for _, dir := range []string{*out, icons} {
		if err := os.MkdirAll(dir, 0o755); err != nil {
			fmt.Fprintln(os.Stderr, err)
			return 1
		}
	}

	// The packager's own icon: the tile at 1024, which is the largest size MyGo
	// packs into a Windows icon group and the size the macOS bundle derives its
	// .icns from.
	if err := writePNG(filepath.Join(*out, "icon.png"), brand.MarkAt(1024)); err != nil {
		fmt.Fprintln(os.Stderr, err)
		return 1
	}

	sizes, err := parseSizes(*ladder)
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		return 1
	}
	for _, size := range sizes {
		target := filepath.Join(icons, fmt.Sprintf("icon-%dx%d.png", size, size))
		if err := writePNG(target, brand.MarkAt(size)); err != nil {
			fmt.Fprintln(os.Stderr, err)
			return 1
		}
	}

	// The Windows multi-resolution icon and the favicon. Both are written from the
	// same render path at the sizes Windows and a browser tab ask for.
	if err := writeICO(filepath.Join(*out, "icon.ico"), brand.ICOIconSizes); err != nil {
		fmt.Fprintln(os.Stderr, err)
		return 1
	}
	if err := writeICO(filepath.Join(*out, "favicon.ico"), brand.FaviconSizes); err != nil {
		fmt.Fprintln(os.Stderr, err)
		return 1
	}

	fmt.Printf("wrote %s (1024), %s, %s, and %d PNG sizes into %s\n",
		filepath.Join(*out, "icon.png"), filepath.Join(*out, "icon.ico"),
		filepath.Join(*out, "favicon.ico"), len(sizes), icons)
	return 0
}

// parseSizes reads the -sizes flag, falling back to the brand ladder.
func parseSizes(value string) ([]int, error) {
	value = strings.TrimSpace(value)
	if value == "" {
		return brand.IconLadder, nil
	}
	var out []int
	for _, part := range strings.Split(value, ",") {
		part = strings.TrimSpace(part)
		if part == "" {
			continue
		}
		size, err := strconv.Atoi(part)
		if err != nil || size < 8 || size > 1024 {
			return nil, fmt.Errorf("尺寸 %q 无效：需要 8–1024 之间的整数", part)
		}
		out = append(out, size)
	}
	if len(out) == 0 {
		return nil, fmt.Errorf("没有可用的尺寸")
	}
	return out, nil
}

// writePNG writes one image through a sibling temporary, so an interrupted run
// never leaves a half-written PNG for the packager to embed.
func writePNG(target string, img image.Image) error {
	if img == nil {
		return fmt.Errorf("%s: 渲染失败", target)
	}
	return writeAtomic(target, func(f *os.File) error { return png.Encode(f, img) })
}

// writeICO writes a multi-resolution icon. Each entry is rendered through the
// same MarkAt the interface and the PNG ladder use, so a 16 px entry and the
// 16x16.png beside it are the same picture rather than two independent
// drawings. Entries above 256 are refused by the format and skipped.
func writeICO(target string, sizes []int) error {
	images := make([]image.Image, 0, len(sizes))
	for _, size := range sizes {
		if size > 256 {
			continue
		}
		img := brand.MarkAt(size)
		if img == nil {
			return fmt.Errorf("%s: %d×%d 渲染失败", target, size, size)
		}
		images = append(images, img)
	}
	if len(images) == 0 {
		return fmt.Errorf("%s: 没有可写入的尺寸", target)
	}
	return writeAtomic(target, func(f *os.File) error { return ico.EncodeAll(f, images) })
}

// writeAtomic writes through a sibling temporary and renames it into place.
func writeAtomic(target string, write func(*os.File) error) error {
	dir := filepath.Dir(target)
	tmp, err := os.CreateTemp(dir, ".icon-*"+filepath.Ext(target))
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

func main() {
	os.Exit(run())
}
