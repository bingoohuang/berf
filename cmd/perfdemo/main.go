package main

import (
	"time"

	"github.com/bingoohuang/gg/pkg/fla9"
	"github.com/bingoohuang/gg/pkg/randx"
	"github.com/bingoohuang/perf"
)

func main() {
	fla9.Parse()

	perf.StartBench(demo)
}

func demo(*perf.Config) (*perf.FnResult, error) {
	d := time.Duration(10 + randx.IntN(10))
	time.Sleep(d * time.Millisecond)
	return &perf.FnResult{
		Status: "200",
	}, nil
}
