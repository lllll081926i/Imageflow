package brand

// marks.go embeds the pre-rendered squares.
//
// The set is fixed at build time: every entry is a file the generator wrote, and
// the generator reads MarkSizes from this package, so a new size cannot exist as a
// declaration only. Embedding one file per size means the switch below is the whole
// lookup, with no directory walk at startup.
import (
	_ "embed"
	"fmt"
)

// The marks the shell draws, and their neighbours. Each is a corrected render of
// the artwork's tile: transparent surround, the tile's own rounded corner, and the
// small sizes' contrast and sharpen pass. Order matches MarkSizes.
var (
	//go:embed marks/mark-16x16.png
	mark16 []byte
	//go:embed marks/mark-24x24.png
	mark24 []byte
	//go:embed marks/mark-32x32.png
	mark32 []byte
	//go:embed marks/mark-36x36.png
	mark36 []byte
	//go:embed marks/mark-40x40.png
	mark40 []byte
	//go:embed marks/mark-48x48.png
	mark48 []byte
	//go:embed marks/mark-64x64.png
	mark64 []byte
)

// markFile returns the embedded bytes for one size.
func markFile(size int) ([]byte, bool) {
	switch size {
	case 16:
		return mark16, true
	case 24:
		return mark24, true
	case 32:
		return mark32, true
	case 36:
		return mark36, true
	case 40:
		return mark40, true
	case 48:
		return mark48, true
	case 64:
		return mark64, true
	}
	return nil, false
}

// errNoMark reports a size this build does not carry.
func errNoMark(size int) error {
	return fmt.Errorf("brand: no mark for size %d, want one of %v", size, MarkSizes)
}
