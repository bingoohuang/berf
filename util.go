package perf

import (
	"context"
	"fmt"
	"net"
	"os"
	"time"
)

func SleepContext(ctx context.Context, duration time.Duration) {
	select {
	case <-ctx.Done():
	case <-time.After(duration):
	}
	return
}

func GetFreePortStart(port int) int {
	for i := 0; i < 100; i++ {
		if IsPortFree(port) {
			return port
		}
		port++
	}

	return 0
}

// IsPortFree tells whether the port is free or not
func IsPortFree(port int) bool {
	l, err := ListenPort(port)
	if err != nil {
		return false
	}

	_ = l.Close()
	return true
}

// ListenPort listens on port
func ListenPort(port int) (net.Listener, error) {
	return net.Listen("tcp", fmt.Sprintf(":%d", port))
}

func ExitIfErr(err error) {
	if err != nil {
		Exit(err.Error())
	}
}

func Exit(msg string) {
	fmt.Fprintln(os.Stderr, "blow: "+msg)
	os.Exit(1)
}

func MergeCodes(codes []string) string {
	n := 0
	last := ""
	merged := ""
	for _, code := range codes {
		if code != last {
			if last != "" {
				merged = mergeCodes(merged, n, last)
			}
			last = code
			n = 1
		} else {
			n++
		}
	}

	if n > 0 {
		merged = mergeCodes(merged, n, last)
	}

	return merged
}

func mergeCodes(merged string, n int, last string) string {
	if merged != "" {
		merged += ","
	}
	if n > 1 {
		merged += fmt.Sprintf("%sx%d", last, n)
	} else {
		merged += fmt.Sprintf("%s", last)
	}
	return merged
}
