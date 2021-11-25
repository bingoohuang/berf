package main

import (
	"context"
	"time"

	"github.com/bingoohuang/gg/pkg/fla9"
	"github.com/bingoohuang/gg/pkg/randx"
	"github.com/bingoohuang/perf"
)

func init() {
	fla9.Parse()
}

func main() {
	perf.StartBench(demo, perf.WithOkStatus("200"))
}

func demo(ctx context.Context, conf *perf.Config) (*perf.Result, error) {
	if conf.HasFeature("demo") {
		if randx.IntN(100) >= 90 {
			return &perf.Result{Status: "500"}, nil
		}

		d := time.Duration(10 + randx.IntN(10))
		perf.SleepContext(ctx, d*time.Millisecond)
		return &perf.Result{Status: "200"}, nil
	}

	return &perf.Result{Status: "Noop"}, nil
}
