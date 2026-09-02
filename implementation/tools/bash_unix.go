//go:build !windows
// +build !windows

package tools

import (
	"os/exec"
	"syscall"
	"time"
)

// configureBashCommand puts the bash child in its own process group and makes
// cancellation kill the whole group. Without this, exec.CommandContext signals
// only the direct child (bash); grandchildren spawned by pipelines, `&`,
// `nohup` etc. survive as orphans and keep running after a timeout or cancel.
func configureBashCommand(cmd *exec.Cmd) {
	cmd.SysProcAttr = &syscall.SysProcAttr{Setpgid: true}
	cmd.Cancel = func() error {
		if cmd.Process == nil {
			return nil
		}
		// Kill the entire process group (negative pid).
		return syscall.Kill(-cmd.Process.Pid, syscall.SIGKILL)
	}
	cmd.WaitDelay = 2 * time.Second
}
