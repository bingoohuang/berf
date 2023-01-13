package main

import (
	"context"
	"fmt"
	"os"
	"syscall"
	"time"

	"github.com/bingoohuang/berf"
	"github.com/bingoohuang/berf/pkg/blow"
	"github.com/bingoohuang/gg/pkg/ctl"
	"github.com/bingoohuang/gg/pkg/fla9"
	"github.com/bingoohuang/gg/pkg/osx"
	"github.com/bingoohuang/gg/pkg/randx"
	"github.com/bingoohuang/gg/pkg/sigx"
	"github.com/mattn/go-isatty"
	"golang.org/x/crypto/ssh/terminal"
)

var (
	pVersion = fla9.Bool("version", false, "Show version and exit")
	pInit    = fla9.Bool("init", false, "Create initial ctl and exit")
)

func init() {
	fla9.Parse()
	ctl.Config{Initing: *pInit, PrintVersion: *pVersion}.ProcessInit()
	sigx.RegisterSignalProfile()
}

func main() {
	if blow.TryStartAsBlow() {
		return
	}

	berf.Demo = true
	berf.StartBench(context.Background(), berf.F(demo), berf.WithOkStatus("200"))
}

func demo(ctx context.Context, conf *berf.Config) (*berf.Result, error) {
	if conf.Has("demo") {
		if randx.IntN(100) >= 90 {
			return &berf.Result{Status: []string{"500"}}, nil
		}

		d := time.Duration(10 + randx.IntN(10))
		osx.SleepContext(ctx, d*time.Millisecond)
		return &berf.Result{Status: []string{"200"}}, nil
	} else if conf.Has("tty") {
		checkStdin()
		os.Exit(0)
	}

	return &berf.Result{Status: []string{"Noop"}}, nil
}

// https://mozillazg.com/2016/03/go-let-cli-support-pipe-read-data-from-stdin.html
// https://www.socketloop.com/tutorials/golang-check-if-os-stdin-input-data-is-piped-or-from-terminal
func checkStdin() {
	//  Stdin is a tty, not a pipe
	fmt.Println("Stdin Is  terminal.Terminal:", terminal.IsTerminal(syscall.Stdin))
	fmt.Println("Stdout Is  terminal.Terminal:", terminal.IsTerminal(syscall.Stdout))

	fi, _ := os.Stdin.Stat()
	fo, _ := os.Stdout.Stat()

	fmt.Println("Is Stdin isatty.Terminal:", isatty.IsTerminal(os.Stdin.Fd()))
	fmt.Println("Is Stdout isatty.Terminal:", isatty.IsTerminal(os.Stdout.Fd()))
	fmt.Println("Is isatty.IsCygwinTerminal: ", isatty.IsCygwinTerminal(os.Stdout.Fd()))
	fmt.Println("Is Stdin device: ", (fi.Mode()&os.ModeCharDevice) == os.ModeCharDevice) // D: device file
	fmt.Println("Is Stdout device: ", (fo.Mode()&os.ModeCharDevice) == os.ModeCharDevice)
}
