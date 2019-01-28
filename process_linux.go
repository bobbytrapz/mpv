package mpv

import (
	"os/exec"
	"syscall"
)

func procOptions(c *exec.Cmd) (cmd *exec.Cmd) {
	if c == nil {
		return nil
	}

	cmd = &exec.Cmd{}
	*cmd = *c
	cmd.SysProcAttr = &syscall.SysProcAttr{
		Setpgid: true,
	}

	return cmd
}
