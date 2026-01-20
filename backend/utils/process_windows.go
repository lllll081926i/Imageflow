//go:build windows

package utils

import (
	"os/exec"
	"syscall"
)

func applyHideWindow(cmd *exec.Cmd) {
	cmd.SysProcAttr = &syscall.SysProcAttr{HideWindow: true}
}
