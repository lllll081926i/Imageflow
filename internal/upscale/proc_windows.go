//go:build windows

package upscale

import (
	"os/exec"
	"strconv"
	"syscall"
)

// hideWindow stops a console window from flashing when the engine is spawned from
// a GUI process, and puts the child in its own process group so a cancellation can
// address the whole tree.
func hideWindow(cmd *exec.Cmd) {
	if cmd.SysProcAttr == nil {
		cmd.SysProcAttr = &syscall.SysProcAttr{}
	}
	cmd.SysProcAttr.HideWindow = true
	cmd.SysProcAttr.CreationFlags |= syscall.CREATE_NEW_PROCESS_GROUP
}

// killTree terminates the child and anything it spawned.
//
// taskkill addresses the whole tree: a launcher process may spawn the real
// inference process in its own group, and killing only the parent leaves the GPU
// busy while the page already reports a cancellation.
func killTree(cmd *exec.Cmd) {
	if cmd == nil || cmd.Process == nil {
		return
	}
	_ = exec.Command("taskkill", "/T", "/F", "/PID", strconv.Itoa(cmd.Process.Pid)).Run()
	_ = cmd.Process.Kill()
}
