package pngopt

import (
	"crypto/sha256"
	_ "embed"
	"fmt"
	"os"
	"path/filepath"
	"sync"

	"github.com/lllll081926i/imageflow-go/internal/output"
)

//go:embed assets/oxipng.exe
var bundled []byte

const executableSHA256 = "421ce9013342db4a2d9ee6c595adc673d4fe0c69bb0b5c6b9a8794c83226093d"

var initExecutable = sync.OnceValues(func() (string, error) {
	if fmt.Sprintf("%x", sha256.Sum256(bundled)) != executableSHA256 {
		return "", fmt.Errorf("内置 OxiPNG 校验失败")
	}
	cache, err := os.UserCacheDir()
	if err != nil {
		return "", err
	}
	path := filepath.Join(cache, "imageflow", "tools", "oxipng-9.1.5-"+executableSHA256[:12], "oxipng.exe")
	if data, err := os.ReadFile(path); err == nil && sha256.Sum256(data) == sha256.Sum256(bundled) {
		return path, nil
	}
	err = output.Atomic(path, func(f *os.File) error { _, err := f.Write(bundled); return err })
	return path, err
})

func executable() (string, error) { return initExecutable() }
