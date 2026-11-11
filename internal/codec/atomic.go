package codec

import (
	"os"

	"github.com/lllll081926i/imageflow-go/internal/output"
)

// WriteAtomic writes target through a sibling temporary file, replacing the
// target only once the write and the flush succeeded. A caller that supplies a
// function which fails leaves the previous target untouched, even when the
// target did not exist before: the animation writers use this so a failed
// encode never leaves a truncated container behind.
func WriteAtomic(target string, write func(*os.File) error) error {
	return output.Atomic(target, write)
}
