package codec

import (
	"os"

	"github.com/lllll081926i/imageflow-go/internal/output"
)

// writeFile runs write against target. It goes through a sibling temporary when
// the target already exists, so a failed encode keeps the previous file; the
// implementation lives in the output package, which owns the same guarantee for
// the batch runners.
func writeFile(target string, write func(*os.File) error) error {
	return output.Commit(target, write)
}
