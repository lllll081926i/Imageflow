//go:build !windows || !amd64

package pngopt

import (
	"fmt"
	"os/exec"
)

func executable() (string, error) {
	path, err := exec.LookPath("oxipng")
	if err != nil {
		return "", fmt.Errorf("当前平台需要安装 OxiPNG 并加入 PATH：%w", err)
	}
	return path, nil
}
