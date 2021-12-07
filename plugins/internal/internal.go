package internal

import (
	"errors"
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

var ErrorNotImplemented = errors.New("not implemented yet")

func Adjust(cur, prev uint64) uint64 {
	if cur >= prev {
		return cur - prev
	}

	return cur + ^uint64(0) - prev
}
