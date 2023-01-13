package berf

import (
	"context"
	"errors"
	"fmt"
	"net"
	"os"
	"reflect"
	"runtime"
	"sync"
	"time"

	"github.com/bingoohuang/berf/pkg/util"
	_ "github.com/bingoohuang/berf/plugins/all"
	"github.com/bingoohuang/gg/pkg/filex"
	"github.com/bingoohuang/gg/pkg/fla9"
	"github.com/bingoohuang/gg/pkg/netx/freeport"
	"github.com/bingoohuang/gg/pkg/osx"
	"github.com/bingoohuang/gg/pkg/ss"
)

var (
	pf = os.Getenv("BERF_PRE")

	pN          = fla9.Int(pf+"n", 0, "Total number of requests")
	pDuration   = fla9.Duration(pf+"d", 0, "Duration of test, examples: -d10s -d3m")
	pGoMaxProcs = fla9.Int(pf+"t", runtime.GOMAXPROCS(0), "Number of GOMAXPROCS")
	pGoroutines = fla9.Int(pf+"c", 100, "Number of goroutines")
	pGoIncr     = fla9.String(pf+"ci", "", "Goroutines incremental mode. empty: none; 1: up by step 1 to max every 1m; 1:10s: up to max by step 1 by n every 10s; 1:10s:1 up to max then down to 0 by step1 every 10s.")
	pQps        = fla9.Float64(pf+"qps", 0, "QPS rate limit")
	pFeatures   = fla9.String(pf+"f", "", "Customized features, e.g. a,b,c, specifically nop to run no benchmarking job for collect hardware metrics only")
	pPlotsFile  = fla9.String(pf+"plots", "", "Plots filename, append :dry to show exists plots in dry mode")
	pVerbose    = fla9.Count(pf+"v", 0, "Verbose level, e.g. -v -vv")
	pThinkTime  = fla9.String(pf+"think", "", "Think time among requests, eg. 1s, 10ms, 10-20ms and etc. (unit ns, us/µs, ms, s, m, h)")
	pPort       = fla9.Int(pf+"port", 28888, "Listen port for serve Web UI")
	pName       = fla9.String(pf+"name", "", "Name for this benchmarking test")
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

	Desc string
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
	Status     []string
	Counting   []string
	Cost       time.Duration
}

type BenchOption struct {
	NoReport bool
}

type Benchable interface {
	Name(context.Context, *Config) string
	Init(context.Context, *Config) (*BenchOption, error)
	Invoke(context.Context, *Config) (*Result, error)
	Final(context.Context, *Config) error
}

type BenchEmpty struct{}

// ErrNoop means there is no operation defined.
var ErrNoop = errors.New("noop")

func (f *BenchEmpty) Name(context.Context, *Config) string             { return "bench" }
func (f *BenchEmpty) Init(context.Context, *Config) error              { return nil }
func (f *BenchEmpty) Final(context.Context, *Config) error             { return nil }
func (f *BenchEmpty) Invoke(context.Context, *Config) (*Result, error) { return nil, ErrNoop }

type F func(context.Context, *Config) (*Result, error)

func (f F) Name(context.Context, *Config) string                   { return reflect.ValueOf(f).Type().Name() }
func (f F) Init(context.Context, *Config) (*BenchOption, error)    { return &BenchOption{}, nil }
func (f F) Final(context.Context, *Config) error                   { return nil }
func (f F) Invoke(ctx context.Context, c *Config) (*Result, error) { return f(ctx, c) }

// Demo tells the underlying benchmark is only a demo.
var Demo = false

// StartBench starts a benchmark.
func StartBench(ctx context.Context, fn Benchable, fns ...ConfigFn) {
	setupPlotsFile()

	c := &Config{
		N: *pN, Duration: *pDuration, Goroutines: *pGoroutines, GoMaxProcs: *pGoMaxProcs,
		Incr: util.ParseGoIncr(*pGoIncr), PlotsFile: *pPlotsFile,
		Qps: *pQps, FeaturesConf: *pFeatures, Verbose: *pVerbose, ThinkTime: *pThinkTime, ChartPort: *pPort,
	}
	for _, f := range fns {
		f(c)
	}

	c.Setup()

	benchOption, err := fn.Init(ctx, c)
	osx.ExitIfErr(err)

	c.Desc = c.Description(fn.Name(ctx, c))
	if !c.IsDryPlots() && c.N != 1 {
		fmt.Println("Berf" + c.Desc)
	}

	requester := c.NewRequester(ctx, fn)
	report := NewStreamReport(requester)
	wg := &sync.WaitGroup{}
	c.serveCharts(report, wg)

	if c.IsNop() {
		<-requester.ctx.Done()
	}

	go requester.run()
	go report.Collect(requester.recordChan)

	p := c.createTerminalPrinter(&requester.concurrent, benchOption)
	p.PrintLoop(report.Snapshot, report.Done(), c.N)

	wg.Wait()
	osx.ExitIfErr(fn.Final(ctx, c))
}

func setupPlotsFile() {
	if *pPlotsFile == "" && len(fla9.Args()) == 1 {
		if filex.Exists(util.TrimDrySuffix(fla9.Args()[0])) {
			*pPlotsFile = fla9.Args()[0]
		}
	}
}

func (c *Config) serveCharts(report *StreamReport, wg *sync.WaitGroup) {
	charts := NewCharts(report.Charts, c)

	wg.Add(1)
	go c.collectChartData(report.requester.ctx, report.Charts, charts, wg)

	if c.IsDryPlots() || c.ChartPort > 0 && c.N != 1 && c.Verbose >= 1 {
		addr := fmt.Sprintf(":%d", c.ChartPort)
		ln, err := net.Listen("tcp", addr)
		osx.ExitIfErr(err)
		fmt.Printf("@Real-time charts is on http://127.0.0.1:%d\n", c.ChartPort)
		charts.Serve(ln, c.ChartPort)
	}
}

func (c *Config) collectChartData(ctx context.Context, chartsFn func() *ChartsReport, charts *Charts, wg *sync.WaitGroup) {
	defer wg.Done()

	ticker := time.NewTicker(util.ParseEnvDuration("BERF_TICK", 15*time.Second))
	defer ticker.Stop()

	c.PlotsHandle = util.NewJsonLogFile(c.PlotsFile)
	defer c.PlotsHandle.Close()

	if c.PlotsHandle.IsDry() {
		return
	}

	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
		}

		rd := chartsFn()
		plots := createMetrics(rd, c.IsNop())
		plots = charts.mergeHardwareMetrics(plots)
		if rd != nil {
			_ = c.PlotsHandle.WriteJSON(plots)
		}
	}
}

// Setup setups the environment by the config.
func (c *Config) Setup() {
	c.Goroutines = ss.Ifi(c.Goroutines < 0, 100, c.Goroutines)
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

func (c *Config) Description(benchableName string) string {
	if c.Desc != "" {
		return c.Desc
	}

	if c.IsDryPlots() {
		return fmt.Sprintf(" showing metrics from existing plots file %s.", util.TrimDrySuffix(c.PlotsFile))
	}

	if c.IsNop() {
		return " starting to collect hardware metrics."
	}

	desc := " benchmarking"
	if benchableName != "" {
		desc += " " + benchableName
	}
	if *pName != "" {
		desc += " " + *pName
	}
	if c.FeaturesConf != "" {
		desc += fmt.Sprintf(" (%s)", c.FeaturesConf)
	}
	if c.N > 0 {
		desc += fmt.Sprintf(" with %d request(s)", c.N)
	}
	if c.Duration > 0 {
		desc += fmt.Sprintf(" for %s", c.Duration)
	}

	return desc + fmt.Sprintf(" using %s%d goroutine(s), %d GoMaxProcs.", c.Incr.Modifier(), c.Goroutines, c.GoMaxProcs)
}

func (c *Config) createTerminalPrinter(concurrent *int64, benchOption *BenchOption) *Printer {
	return &Printer{
		maxNum: int64(c.N), maxDuration: c.Duration, verbose: c.Verbose, config: c,
		concurrent:  concurrent,
		benchOption: benchOption,
	}
}

func (c *Config) IsDryPlots() bool { return util.IsDrySuffix(c.PlotsFile) }
