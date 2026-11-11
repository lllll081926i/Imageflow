package output

import (
	"errors"
	"fmt"
	"image"
	"os"
	"path/filepath"
	"time"
)

// Budgets mirror the safety limits of the Python backend.
const (
	MaxOutputSide   = 16000
	MaxOutputPixels = 64_000_000
)

// ErrSizeExceeded reports an output larger than the shared budget.
var ErrSizeExceeded = errors.New("输出尺寸超出安全限制")

// CheckSize rejects an output canvas before it is allocated.
func CheckSize(width, height int) error {
	if width <= 0 || height <= 0 {
		return fmt.Errorf("%w: %dx%d", ErrSizeExceeded, width, height)
	}
	if width > MaxOutputSide || height > MaxOutputSide {
		return fmt.Errorf("%w: 单边 %d/%d 超过 %dpx", ErrSizeExceeded, width, height, MaxOutputSide)
	}
	if width*height > MaxOutputPixels {
		return fmt.Errorf("%w: 面积 %d 超过 %dpx", ErrSizeExceeded, width*height, MaxOutputPixels)
	}
	return nil
}

// CheckImage rejects an output image before it is written.
func CheckImage(img image.Image) error {
	if img == nil {
		return errors.New("输出图像为空")
	}
	b := img.Bounds()
	return CheckSize(b.Dx(), b.Dy())
}

// Atomic writes a file through a sibling temporary, replacing the target only
// once the write succeeded, so a failure keeps the previous file intact.
func Atomic(target string, write func(*os.File) error) error {
	dir := filepath.Dir(target)
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return err
	}
	tmp, err := os.CreateTemp(dir, ".imageflow-*"+filepath.Ext(target))
	if err != nil {
		return err
	}
	name := tmp.Name()
	committed := false
	defer func() {
		if !committed {
			_ = os.Remove(name)
		}
	}()
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
	// On Windows, replacing a target while another goroutine is renaming into
	// the same path can fail with a transient sharing violation. Retrying a few
	// times lets concurrent writers to one destination all finish with a whole
	// file instead of one of them reporting a spurious failure.
	var renameErr error
	for attempt := 0; attempt < 8; attempt++ {
		if renameErr = os.Rename(name, target); renameErr == nil {
			break
		}
		time.Sleep(time.Duration(attempt+1) * time.Millisecond)
	}
	if renameErr != nil {
		return renameErr
	}
	committed = true
	return nil
}

// Commit always writes through a sibling temporary and renames it into place.
//
// It used to write directly with os.Create when the target did not exist. That
// was a Stat→Create TOCTOU window: two workers aiming at the same new path
// could both pass the Stat, interleave their writes into one file, and the
// loser's deferred Remove deleted the winner's output. A dangling symlink was
// followed out of the output directory as well. Always using Atomic keeps a
// failed encode from leaving a half-written file and makes the final rename
// the single point at which the target appears.
func Commit(target string, write func(*os.File) error) error {
	// Keep the established contract: the caller must have prepared the output
	// directory. Atomic would create it, and silently creating a destination
	// the user never chose is a different (and surprising) behaviour.
	if _, err := os.Stat(filepath.Dir(target)); err != nil {
		return err
	}
	return Atomic(target, write)
}
