//go:build !windows

package utils

import "os/exec"

func applyHideWindow(_ *exec.Cmd) {}
