//go:build windows
// +build windows

package tools

import (
	"os/exec"
	"time"
)

// configureBashCommand prepares a bash command for cancellation on Windows,
// where process groups are not available. The direct child process is killed
// via os.Process.Kill; the WaitDelay guards against a process that ignores the
// kill.
func configureBashCommand(cmd *exec.Cmd) {
	cmd.Cancel = func() error {
		if cmd.Process == nil {
			return nil
		}
		return cmd.Process.Kill()
	}
	cmd.WaitDelay = 2 * time.Second
}
