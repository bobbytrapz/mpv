// +build !linux,!windows,!darwin,!openbsd

package mpv

import (
	"os/exec"
)

func procOptions(c *exec.Cmd) (cmd *exec.Cmd) {
	return c
}
