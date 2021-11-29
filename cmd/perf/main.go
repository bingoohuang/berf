package main

import (
	"context"
	"fmt"
	"os"
	"syscall"
	"time"

	"github.com/bingoohuang/gg/pkg/ss"
	"github.com/bingoohuang/perf/pkg/blow"

	"github.com/bingoohuang/gg/pkg/ctl"

	"golang.org/x/crypto/ssh/terminal"

	"github.com/bingoohuang/gg/pkg/osx"

	"github.com/bingoohuang/gg/pkg/fla9"
	"github.com/bingoohuang/gg/pkg/randx"
	"github.com/bingoohuang/perf"
	"github.com/mattn/go-isatty"
)

var (
	pBlow    = fla9.Bool("blow", false, "Blow as a high-performance HTTP benchmarking tool")
	pVersion = fla9.Bool("version", false, "Show version and exit")
	pInit    = fla9.Bool("init", false, "Create initial ctl and exit")
)

func init() {
	fla9.Parse()
	ctl.Config{Initing: *pInit, PrintVersion: *pVersion}.ProcessInit()
}

func main() {
	if *pBlow || blow.IsBlowEnv() {
		perf.StartBench(context.Background(), &blow.BlowBench{},
			perf.WithOkStatus(ss.Or(blow.StatusName(), "200")),
			perf.WithCounting("Connections"))
		return
	}

	perf.StartBench(context.Background(), perf.F(demo), perf.WithOkStatus("200"))
}

func demo(ctx context.Context, conf *perf.Config) (*perf.Result, error) {
	if conf.Has("demo") {
		if randx.IntN(100) >= 90 {
			return &perf.Result{Status: []string{"500"}}, nil
		}

		d := time.Duration(10 + randx.IntN(10))
		osx.SleepContext(ctx, d*time.Millisecond)
		return &perf.Result{Status: []string{"200"}}, nil
	} else if conf.Has("tty") {
		checkStdin()
		os.Exit(0)
	}

	return &perf.Result{Status: []string{"Noop"}}, nil
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
