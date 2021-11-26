package perf

import (
	"context"
	"fmt"
	"io/ioutil"
	"log"
	"net"
	"os"
	"os/exec"
	"runtime"
	"strings"
	"time"
)

// OpenBrowser go/src/cmd/internal/browser/browser.go
func OpenBrowser(url string) bool {
	var cmds [][]string
	if exe := os.Getenv("BROWSER"); exe != "" {
		cmds = append(cmds, []string{exe})
	}
	switch runtime.GOOS {
	case "darwin":
		cmds = append(cmds, []string{"/usr/bin/open"})
	case "windows":
		cmds = append(cmds, []string{"cmd", "/c", "start"})
	default:
		if os.Getenv("DISPLAY") != "" {
			// xdg-open is only for use in a desktop environment.
			cmds = append(cmds, []string{"xdg-open"})
		}
	}
	cmds = append(cmds, []string{"chrome"}, []string{"google-chrome"}, []string{"chromium"}, []string{"firefox"})
	for _, args := range cmds {
		cmd := exec.Command(args[0], append(args[1:], url)...)
		if cmd.Start() == nil && WaitTimeout(cmd, 3*time.Second) {
			return true
		}
	}
	return false
}

// WaitTimeout reports whether the command appears to have run successfully.
// If the command runs longer than the timeout, it's deemed successful.
// If the command runs within the timeout, it's deemed successful if it exited cleanly.
func WaitTimeout(cmd *exec.Cmd, timeout time.Duration) bool {
	ch := make(chan error, 1)
	go func() {
		ch <- cmd.Wait()
	}()

	select {
	case <-time.After(timeout):
		return true
	case err := <-ch:
		return err == nil
	}
}

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

// ParseFileArg parse an argument which represents a string content,
// or @file to represents the file's content.
func ParseFileArg(arg string) []byte {
	if strings.HasPrefix(arg, "@") {
		f := (arg)[1:]
		if v, err := ioutil.ReadFile(f); err != nil {
			log.Fatalf("failed to read file %s, error: %v", f, err)
			return nil
		} else {
			return v
		}
	}

	return []byte(arg)
}
