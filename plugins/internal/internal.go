package internal

import (
	"errors"
	"os/exec"
	"syscall"
	"time"
)

var (
	ErrorNotImplemented = errors.New("not implemented yet")
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

type SizeUnit int

const (
	KILO SizeUnit = 1000
	MEGA          = 1000 * KILO
	GIGA          = 1000 * MEGA
	TERA          = 1000 * GIGA
)

func BytesToGiga(bytes uint64) float64 {
	return float64(bytes) / float64(GIGA)
}

func BytesToMEGA(bytes uint64) float64 {
	return float64(bytes) / float64(MEGA)
}

func BytesToBPS(bytes uint64, d time.Duration) float64 {
	return float64(bytes*8) / float64(MEGA) / d.Seconds()
}

func NumberToRate(num uint64, d time.Duration) float64 {
	return float64(num) / d.Seconds()
}
