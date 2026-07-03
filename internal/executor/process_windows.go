//go:build windows

package executor

import (
	"os"
	"os/exec"
)

func configureCommand(cmd *exec.Cmd) {}

func terminateCommand(cmd *exec.Cmd) error {
	if cmd.Process == nil {
		return os.ErrProcessDone
	}
	return cmd.Process.Kill()
}
