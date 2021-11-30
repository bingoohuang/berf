package main

import (
	"context"
	"github.com/bingoohuang/gg/pkg/ctl"
	"github.com/bingoohuang/gg/pkg/fla9"
	"github.com/bingoohuang/gg/pkg/osx"
	"github.com/bingoohuang/gg/pkg/randx"
	"github.com/bingoohuang/gg/pkg/sigx"
	"github.com/bingoohuang/perf"
	"time"
)

var (
	pVersion = fla9.Bool("version", false, "Show version and exit")
	pInit    = fla9.Bool("init", false, "Create initial ctl and exit")
)

func init() {
	fla9.Parse()
	ctl.Config{Initing: *pInit, PrintVersion: *pVersion}.ProcessInit()
}

func main() {
	sigx.RegisterSignalProfile()
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
	}

	return &perf.Result{Status: []string{"Noop"}}, nil
}
