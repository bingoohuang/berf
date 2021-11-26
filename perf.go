package perf

import (
	"context"
	"fmt"
	"github.com/bingoohuang/gg/pkg/ss"
	"net"
	"os"
	"runtime"
	"strings"
	"time"

	"github.com/bingoohuang/gg/pkg/fla9"
)

var (
	pf = os.Getenv("PERF_PRE")

	pN          = fla9.Int(pf+"n", 0, "Total number of requests")
	pDuration   = fla9.Duration(pf+"d", 0, "Duration of test, examples: -d10s -d3m")
	pGoMaxProcs = fla9.Int(pf+"t", runtime.GOMAXPROCS(0), "Number of GOMAXPROCS")
	pGoroutines = fla9.Int(pf+"c", 100, "Number of goroutines")
	pGoIncr     = fla9.Int(pf+"ci", 0, "Goroutines incremental mode. 0: none, n: incr by n up to max then down to")
	pGoIncrDur  = fla9.Duration(pf+"cd", time.Minute, "Interval among goroutines number change")
	pQps        = fla9.Float64(pf+"qps", 0, "QPS rate limit")
	pFeatures   = fla9.String(pf+"f", "", "Features, e.g. a,b,c")
	pVerbose    = fla9.Count(pf+"v", 0, "Verbose level, e.g. -v -vv")
	pThinkTime  = fla9.String(pf+"think", "", "Think time among requests, eg. 1s, 10ms, 10-20ms and etc. (unit ns, us/µs, ms, s, m, h)")
	pPort       = fla9.Int(pf+"p", 28888, "Listen port for serve Web UI")
)

// Config defines the bench configuration.
type Config struct {
	N          int
	Goroutines int
	Duration   time.Duration

	GoIncr    int
	GoIncrDur time.Duration

	GoMaxProcs int
	Qps        float64
	Features   string
	Verbose    int
	ThinkTime  string
	ChartPort  int

	FeatureMap
	CountingName string
	OkStatus     string
}

type ConfigFn func(*Config)

// WithCounting with customized config.
func WithCounting(name string) ConfigFn { return func(c *Config) { c.CountingName = name } }

// WithOkStatus set the status which represents OK.
func WithOkStatus(okStatus string) ConfigFn { return func(c *Config) { c.OkStatus = okStatus } }

// WithConfig with customized config.
func WithConfig(v *Config) ConfigFn { return func(c *Config) { *c = *v } }

type Result struct {
	ReadBytes  int64
	WriteBytes int64
	Status     string
	Counting   string
}

type F func(context.Context, *Config) (*Result, error)

// StartBench starts a benchmark.
func StartBench(fn F, fns ...ConfigFn) {
	c := &Config{
		N: *pN, Duration: *pDuration, Goroutines: *pGoroutines, GoMaxProcs: *pGoMaxProcs,
		GoIncr: *pGoIncr, GoIncrDur: *pGoIncrDur,
		Qps: *pQps, Features: *pFeatures, Verbose: *pVerbose, ThinkTime: *pThinkTime, ChartPort: *pPort,
	}
	for _, f := range fns {
		f(c)
	}

	c.Setup()

	requester, err := c.NewRequester(fn)
	ExitIfErr(err)

	desc := c.Description()
	fmt.Println(desc)

	report := NewStreamReport(requester)
	c.serveCharts(report, desc)

	go requester.Run()
	go report.Collect(requester.recordChan)

	p := c.createTerminalPrinter(&requester.concurrent)
	p.PrintLoop(report.Snapshot, 500*time.Millisecond, report.Done(), c.N)
}

func (c *Config) serveCharts(report *StreamReport, desc string) {
	if c.ChartPort > 0 && c.N != 1 && c.Verbose >= 1 {
		addr := fmt.Sprintf(":%d", c.ChartPort)
		ln, err := net.Listen("tcp", addr)
		ExitIfErr(err)
		fmt.Printf("@Real-time charts is on http://127.0.0.1:%d\n", c.ChartPort)

		// serve charts data
		charts, err := NewCharts(ln, report.Charts, desc)
		ExitIfErr(err)
		go charts.Serve(c.ChartPort)
	}
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

	if c.ChartPort > 0 && c.N != 1 {
		c.ChartPort = GetFreePortStart(c.ChartPort)
	}

	if c.FeatureMap == nil {
		c.FeatureMap = make(map[string]bool)
		c.FeatureMap.Setup(c.Features)
	}
}

func (c *Config) Description() string {
	desc := "Benchmarking"
	if c.Features != "" {
		desc += fmt.Sprintf(" %s", c.Features)
	}
	if c.N > 0 {
		desc += fmt.Sprintf(" with %d request(s)", c.N)
	}
	if c.Duration > 0 {
		desc += fmt.Sprintf(" for %s", c.Duration)
	}

	return desc + fmt.Sprintf(" using %s%d goroutine(s), %d GoMaxProcs.",
		ss.If(c.GoIncr > 0, "max ", ""),
		c.Goroutines, c.GoMaxProcs)
}

func (c *Config) createTerminalPrinter(concurrent *int64) *Printer {
	return &Printer{
		maxNum: int64(c.N), maxDuration: c.Duration, verbose: c.Verbose, config: c,
		concurrent: concurrent,
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
