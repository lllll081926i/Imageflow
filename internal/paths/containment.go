package paths

import (
	"fmt"
	"os"
	"path/filepath"
)

// CheckContained 同时核验词法路径和已有符号链接的实际目标。
func CheckContained(root, target string) error {
	absRoot, err := filepath.Abs(root)
	if err != nil {
		return err
	}
	absTarget, err := filepath.Abs(target)
	if err != nil {
		return err
	}
	inside := func(base, path string) bool {
		rel, err := filepath.Rel(base, path)
		return err == nil && (rel == "." || filepath.IsLocal(rel))
	}
	if !inside(absRoot, absTarget) {
		return fmt.Errorf("输出路径超出所选目录")
	}
	resolve := func(path string) (string, error) {
		var suffix []string
		for {
			resolved, err := filepath.EvalSymlinks(path)
			if err == nil {
				for i := len(suffix) - 1; i >= 0; i-- {
					resolved = filepath.Join(resolved, suffix[i])
				}
				return resolved, nil
			}
			if !os.IsNotExist(err) {
				return "", err
			}
			parent := filepath.Dir(path)
			if parent == path {
				return "", err
			}
			suffix = append(suffix, filepath.Base(path))
			path = parent
		}
	}
	realRoot, err := resolve(absRoot)
	if err != nil {
		return err
	}
	realTarget, err := resolve(absTarget)
	if err != nil {
		return err
	}
	if !inside(realRoot, realTarget) {
		return fmt.Errorf("输出路径的链接指向所选目录之外")
	}
	return nil
}
