package internal

import (
	"os/exec"
	"syscall"
)

// ExitStatus status takes the error from exec.Command
// and returns the exit status and true
// if error is not exit status, will return 0 and false
func ExitStatus(err error) (int, bool) {
	if exiterr, ok := err.(*exec.ExitError); ok {
		if status, ok := exiterr.Sys().(syscall.WaitStatus); ok {
			return status.ExitStatus(), true
		}
	}
	return 0, false
}
