package perf

import (
	"fmt"
	"net"
	"runtime"
	"strings"
	"time"

	"github.com/bingoohuang/gg/pkg/fla9"
)

var (
	pN          = fla9.Int("n", 0, "Total number of requests")
	pDuration   = fla9.Duration("d", 0, "Duration of test, examples: -d10s -d3m")
	pGoMaxProcs = fla9.Int("t", runtime.GOMAXPROCS(0), "Number of GOMAXPROCS")
	pGoroutines = fla9.Int("g", 300, "Number of goroutines")
	pQps        = fla9.Float64("qps", 0, "QPS per worker")
	pFeatures   = fla9.String("f", "", "features, e.g. a,b,c")
	pVerbose    = fla9.Count("v", 0, "verbose level")
	thinkTime   = fla9.String("think", "", "Think time among requests, eg. 1s, 10ms, 10-20ms and etc. (unit ns, us/µs, ms, s, m, h)")
	pPort       = fla9.Int("p", 28888, "Listen port for serve Web UI")
)

// Config defines the bench configuration.
type Config struct {
	N          int
	Duration   time.Duration
	Goroutines int
	GoMaxProcs int
	Features   string
	Verbose    int

	FeatureMap
	GoroutinesTimes int
}

type ConfigFn func(*Config)

func WithConfig(v *Config) ConfigFn {
	return func(c *Config) {
		*c = *v
	}
}

type FnResult struct {
	ReadBytes  int64
	WriteBytes int64
	Status     string
	Counting   string
}

type Fn func() (*FnResult, error)

// StartBench starts a benchmark.
func StartBench(fn Fn, fns ...ConfigFn) {
	c := &Config{
		N:          *pN,
		Duration:   *pDuration,
		Goroutines: *pGoroutines,
		GoMaxProcs: *pGoMaxProcs,
		Features:   *pFeatures,
		Verbose:    *pVerbose,
	}

	for _, f := range fns {
		f(c)
	}

	c.Setup()

	requester, err := NewRequester(c.Goroutines, c.Verbose, int64(c.N), c.Duration, fn)
	ExitIfErr(err)

	var ln net.Listener

	// description
	desc := "Benchmarking "
	if c.N > 0 {
		desc += fmt.Sprintf(" with %d request(s)", c.N)
	}
	if c.Duration > 0 {
		desc += fmt.Sprintf(" for %s", c.Duration)
	}
	desc += fmt.Sprintf(" using %d connection(s), %d GoMaxProcs", c.Goroutines, c.GoMaxProcs)
	if c.Features != "" {
		desc += fmt.Sprintf(" with feature: %s", c.Features)
	}
	desc += "."

	fmt.Println(desc)

	// charts listener
	if *pPort > 0 && c.N != 1 {
		*pPort = GetFreePortStart(*pPort)
	}

	if *pPort > 0 && c.N != 1 && *pVerbose >= 1 {
		addr := fmt.Sprintf(":%d", *pPort)
		if ln, err = net.Listen("tcp", addr); err != nil {
			ExitIfErr(err)
		}
		fmt.Printf("@ Real-time charts is listening on http://127.0.0.1:%d\n", *pPort)
	}
	fmt.Printf("\n")

	go requester.Run()

	// metrics collection
	report := NewStreamReport()

	maxResult := c.Goroutines * 100
	if maxResult > 8192 {
		maxResult = 8192
	}
	go report.Collect(requester.recordChan)

	if ln != nil {
		// serve charts data
		charts, err := NewCharts(ln, report.Charts, desc)
		ExitIfErr(err)
		go charts.Serve(*pPort)
	}

	// terminal printer
	p := &Printer{maxNum: int64(c.N), maxDuration: c.Duration, verbose: c.Verbose}
	p.PrintLoop(report.Snapshot, 200*time.Millisecond, false, report.Done(), c.N)
}

// Setup setups the environment by the config.
func (c *Config) Setup() {
	if c.Goroutines < 0 {
		c.Goroutines = 100
	}
	if c.GoMaxProcs < 0 {
		c.GoMaxProcs = int(2.5 * float64(runtime.GOMAXPROCS(0)))
	}

	if c.N == 1 {
		c.Verbose = 2
	}

	if c.N > 0 && c.N < c.Goroutines {
		c.Goroutines = c.N
	}

	runtime.GOMAXPROCS(c.GoMaxProcs)

	c.GoroutinesTimes = c.N / c.Goroutines

	if c.FeatureMap == nil {
		c.FeatureMap = make(map[string]bool)
		c.FeatureMap.Setup(c.Features)
	}
}

// FeatureMap defines a feature map.
type FeatureMap map[string]bool

// Setup sets up a feature map by features string, which separates feature names by comma.
func (f *FeatureMap) Setup(features string) {
	for _, feature := range strings.Split(strings.ToLower(features), ",") {
		if v := strings.TrimSpace(feature); v != "" {
			(*f)[v] = true
		}
	}
}

// HasFeature tells the feature map contains a features.
func (f *FeatureMap) HasFeature(feature string) bool {
	return (*f)[feature] || (*f)[strings.ToLower(feature)]
}

// HasAny tells the feature map contains any of the features.
func (f *FeatureMap) HasAny(features ...string) bool {
	for _, feature := range features {
		if f.HasFeature(feature) {
			return true
		}
	}

	return false
}
