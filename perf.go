package perf

import (
	"context"
	"fmt"
	"net"
	"os"
	"runtime"
	"time"

	"github.com/bingoohuang/perf/pkg/util"

	"github.com/bingoohuang/gg/pkg/ss"

	"github.com/bingoohuang/gg/pkg/fla9"
	"github.com/bingoohuang/gg/pkg/netx/freeport"
	_ "github.com/bingoohuang/perf/plugins/all"
)

var (
	pf = os.Getenv("PERF_PRE")

	pN          = fla9.Int(pf+"n", 0, "Total number of requests")
	pDuration   = fla9.Duration(pf+"d", 0, "Duration of test, examples: -d10s -d3m")
	pGoMaxProcs = fla9.Int(pf+"t", runtime.GOMAXPROCS(0), "Number of GOMAXPROCS")
	pGoroutines = fla9.Int(pf+"c", 100, "Number of goroutines")
	pGoIncr     = fla9.String(pf+"ci", "", "Goroutines incremental mode. empty: none; 1: up by step 1 to max every 1m; 1:10s: up to max by step 1 by n every 10s; 1:10s:1 up to max then down to 0 by step1 every 10s.")
	pQps        = fla9.Float64(pf+"qps", 0, "QPS rate limit")
	pFeatures   = fla9.String(pf+"f", "", "Features, e.g. a,b,c")
	pPlotsFile  = fla9.String(pf+"plots", "", "Plots filename, append :dry to show exists plots in dry mode")
	pVerbose    = fla9.Count(pf+"v", 0, "Verbose level, e.g. -v -vv")
	pThinkTime  = fla9.String(pf+"think", "", "Think time among requests, eg. 1s, 10ms, 10-20ms and etc. (unit ns, us/µs, ms, s, m, h)")
	pPort       = fla9.Int(pf+"p", 28888, "Listen port for serve Web UI")
	PVersion    = fla9.Bool("version", false, "Show version and exit")
	PInit       = fla9.Bool("init", false, "Create initial ctl and exit")
)

// Config defines the bench configuration.
type Config struct {
	N          int
	Goroutines int
	Duration   time.Duration
	Incr       util.GoroutineIncr

	GoMaxProcs   int
	Qps          float64
	FeaturesConf string
	Verbose      int
	ThinkTime    string
	ChartPort    int

	util.Features
	CountingName string
	OkStatus     string
	PlotsFile    string
	PlotsHandle  *util.JSONLogFile
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
		Incr: util.ParseGoIncr(*pGoIncr), PlotsFile: *pPlotsFile,
		Qps: *pQps, FeaturesConf: *pFeatures, Verbose: *pVerbose, ThinkTime: *pThinkTime, ChartPort: *pPort,
	}
	for _, f := range fns {
		f(c)
	}

	c.Setup()

	requester, err := c.NewRequester(fn)
	util.ExitIfErr(err)

	desc := c.Description()
	if !c.IsDryPlots() {
		fmt.Println(desc)
	}

	report := NewStreamReport(requester)
	c.serveCharts(report, desc)

	if c.IsNop() {
		<-requester.ctx.Done()
	}

	go requester.Run()
	go report.Collect(requester.recordChan)

	p := c.createTerminalPrinter(&requester.concurrent)
	p.PrintLoop(report.Snapshot, 500*time.Millisecond, report.Done(), c.N)
}

func (c *Config) serveCharts(report *StreamReport, desc string) {
	charts := NewCharts(report.Charts, desc, c)

	go c.collectChartData(report.requester.ctx, report.Charts, charts)

	if c.IsDryPlots() || c.ChartPort > 0 && c.N != 1 && c.Verbose >= 1 {
		addr := fmt.Sprintf(":%d", c.ChartPort)
		ln, err := net.Listen("tcp", addr)
		util.ExitIfErr(err)
		fmt.Printf("@Real-time charts is on http://127.0.0.1:%d\n", c.ChartPort)

		charts.Serve(ln, c.ChartPort)
	}
}

func (c *Config) collectChartData(ctx context.Context, chartsFn func() *ChartsReport, charts *Charts) {
	tickDur := 15 * time.Second
	tickConf := ss.Or(os.Getenv("PERF_TICK"), "5s")
	if v, _ := time.ParseDuration(tickConf); v > 0 {
		tickDur = v
	}

	ticker := time.NewTicker(tickDur)
	defer ticker.Stop()

	c.PlotsHandle = util.NewJsonLogFile(c.PlotsFile)
	defer c.PlotsHandle.Close()

	if c.PlotsHandle.IsDry() {
		return
	}

	for ctx.Err() == nil {
		<-ticker.C
		rd := chartsFn()
		plots := createMetrics(rd, c.IsNop())
		plots = charts.mergeHardwareMetrics(plots)
		if rd != nil {
			c.PlotsHandle.WriteJSON(plots)
		}
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
		c.ChartPort = freeport.PortStart(c.ChartPort)
	}

	if c.Features == nil {
		c.Features = util.NewFeatures(c.FeaturesConf)
	}
}

func (c *Config) Description() string {
	if c.IsNop() {
		return "Perf is starting to collect hardware metrics."
	}

	desc := "Benchmarking"
	if c.FeaturesConf != "" {
		desc += fmt.Sprintf(" %s", c.FeaturesConf)
	}
	if c.N > 0 {
		desc += fmt.Sprintf(" with %d request(s)", c.N)
	}
	if c.Duration > 0 {
		desc += fmt.Sprintf(" for %s", c.Duration)
	}

	return desc + fmt.Sprintf(" using %s%d goroutine(s), %d GoMaxProcs.", c.Incr.Modifier(), c.Goroutines, c.GoMaxProcs)
}

func (c *Config) createTerminalPrinter(concurrent *int64) *Printer {
	return &Printer{
		maxNum: int64(c.N), maxDuration: c.Duration, verbose: c.Verbose, config: c,
		concurrent: concurrent,
	}
}

func (c *Config) IsDryPlots() bool { return util.IsDrySuffix(c.PlotsFile) }
