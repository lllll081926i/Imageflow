//go:build !windows

package pngopt

import "os/exec"

func configure(cmd *exec.Cmd) {}
