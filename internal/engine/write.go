package engine

// write.go holds the small writer helpers the newer engines share. Every one of
// them goes through output.Atomic, so a failed or cancelled write never leaves a
// half-finished file and never touches an existing one.

import (
	"os"

	"github.com/lllll081926i/imageflow-go/internal/output"
)

// writeBytes writes raw bytes to target through a sibling temporary file.
func writeBytes(target string, data []byte) error {
	return output.Atomic(target, func(f *os.File) error {
		_, err := f.Write(data)
		return err
	})
}
