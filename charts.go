package perf

import (
	"bytes"
	"embed"
	"fmt"
	"io"
	"log"
	"net"
	"strings"
	"text/template"
	"time"

	"github.com/bingoohuang/gg/pkg/osx"

	util2 "github.com/bingoohuang/perf/pkg/util"

	"github.com/bingoohuang/gg/pkg/mapp"
	"github.com/bingoohuang/jj"

	"github.com/bingoohuang/gg/pkg/ss"
	"github.com/bingoohuang/perf/plugins"

	_ "embed"

	cors "github.com/AdhityaRamadhanus/fasthttpcors"
	"github.com/go-echarts/go-echarts/v2/charts"
	"github.com/go-echarts/go-echarts/v2/components"
	"github.com/go-echarts/go-echarts/v2/opts"
	"github.com/go-echarts/go-echarts/v2/templates"
	"github.com/valyala/fasthttp"
)

//go:embed echarts.min.js jquery.min.js
var assetsFS embed.FS

//go:embed views_sync.js
var viewSyncJs string

const (
	assetsPath = "/echarts/statics/"
)

const (
	ViewTpl = `
$(function () {
{{ .SetInterval }}(views_sync, {{ .Interval }}); });
let views = {{.ViewsMap}};
{{.ViewSyncJS}}
`
	PageTpl = `
{{- define "page" }}
<!DOCTYPE html>
<html>
	{{- template "header" . }}
<body>
<p align="center">🚀 <a href="https://github.com/bingoohuang/perf"><b>Perf</b></a> %s</p>
<style> .box { justify-content:center; display:flex; flex-wrap:wrap } </style>
<div class="box"> {{- range .Charts }} {{ template "base" . }} {{- end }} </div>
</body>
</html>
{{ end }}
`
)

func (c *Views) genViewTemplate(viewChartsMap map[string]string) string {
	tpl, err := template.New("view").Parse(ViewTpl)
	if err != nil {
		panic("failed to parse template " + err.Error())
	}

	viewsMap := "{"
	for k, v := range viewChartsMap {
		viewsMap += k + ": goecharts_" + v + ","
	}
	viewsMap += "noop: null}"

	d := struct {
		Interval    int
		ViewsMap    string
		SetInterval string
		ViewSyncJS  string
	}{
		Interval:    int(time.Second.Milliseconds()),
		ViewsMap:    viewsMap,
		SetInterval: "setInterval",
		ViewSyncJS:  viewSyncJs,
	}

	if c.dryPlots {
		d.Interval = 1
		d.SetInterval = "setTimeout"
	}

	buf := bytes.Buffer{}
	if err := tpl.Execute(&buf, d); err != nil {
		panic("failed to execute template " + err.Error())
	}

	return buf.String()
}

type Views struct {
	viewChartsMap map[string]string
	size          util2.WidthHeight
	dryPlots      bool
	num           int
}

func NewViews(size string, dryPlots bool) *Views {
	return &Views{
		viewChartsMap: make(map[string]string),
		size:          util2.ParseWidthHeight(size, 500, 300),
		dryPlots:      dryPlots,
	}
}

func (c *Views) newBasicView(route string) *charts.Line {
	g := charts.NewLine()
	g.SetGlobalOptions(charts.WithTooltipOpts(opts.Tooltip{Show: true, Trigger: "axis"}),
		charts.WithXAxisOpts(opts.XAxis{Name: "Time"}),
		charts.WithInitializationOpts(opts.Initialization{Width: c.size.WidthPx(), Height: c.size.HeightPx()}),
		charts.WithDataZoomOpts(opts.DataZoom{Type: "slider", XAxisIndex: []int{0}}),
	)
	g.SetXAxis([]string{}).SetSeriesOptions(charts.WithLineChartOpts(opts.LineChart{Smooth: true}))

	c.viewChartsMap[route] = g.ChartID
	if len(c.viewChartsMap) == c.num {
		g.AddJSFuncs(c.genViewTemplate(c.viewChartsMap))
	}
	return g
}

var titleCn = map[string]string{
	"tps":               "吞吐量",
	"latency":           "延时",
	"latencypercentile": "百分位延时",
	"concurrent":        "并发",
	"procstat":          "进程",
	"mem":               "内存",
	"netstat":           "网络",
	"net":               "网卡",
	"system":            "系统",
}

func title(name string) string {
	if t := titleCn[strings.ToLower(name)]; t != "" {
		return t
	}

	return strings.Title(name)
}

func (c *Views) newView(name, unit string, series plugins.Series) components.Charter {
	selected := map[string]bool{}
	for _, p := range series.Series {
		selected[p] = len(series.Series) == 1 || ss.AnyOf(p, series.Selected...)
	}

	g := c.newBasicView(name)
	var axisLabel *opts.AxisLabel
	if unit != "" {
		axisLabel = &opts.AxisLabel{Formatter: "{value} " + unit}
	}
	g.SetGlobalOptions(charts.WithTitleOpts(opts.Title{Title: title(name)}),
		charts.WithYAxisOpts(opts.YAxis{Scale: true, AxisLabel: axisLabel}),
		charts.WithLegendOpts(opts.Legend{Show: true, Selected: selected}),
	)

	for _, p := range series.Series {
		g.AddSeries(p, []opts.LineData{})
	}
	return g
}

func (c *Views) newHardwareViews(charts *Charts) (charters []components.Charter) {
	for _, name := range charts.hardwaresNames {
		input := charts.hardwares[name]
		charters = append(charters, c.newView(name, "", input.Series()))
	}

	return
}

func (c *Views) newLatencyView() components.Charter {
	return c.newView("latency", "ms", plugins.Series{
		Series: []string{"Min", "Mean", "StdDev", "Max"}, Selected: []string{"Mean"},
	})
}

func (c *Views) newLatencyPercentileView() components.Charter {
	return c.newView("latencyPercentile", "ms", plugins.Series{
		Series: []string{"P50", "P75", "P90", "P95", "P99", "P99.9", "P99.99"}, Selected: []string{"P50", "P90", "P99"},
	})
}

func (c *Views) newConcurrentView() components.Charter {
	return c.newView("concurrent", "", plugins.Series{Series: []string{"Concurrent"}})
}

func (c *Views) newTPSView() components.Charter {
	return c.newView("tps", "", plugins.Series{Series: []string{"TPS"}})
}

type Charts struct {
	chartsData func() *ChartsReport
	config     *Config

	hardwaresNames []string
	hardwares      map[string]plugins.Input
}

func NewCharts(chartsData func() *ChartsReport, desc string, config *Config) *Charts {
	templates.PageTpl = fmt.Sprintf(PageTpl, desc)
	c := &Charts{chartsData: chartsData, config: config}
	c.initHardwareCollectors()
	return c
}

func (c *Charts) initHardwareCollectors() {
	c.hardwaresNames = mapp.KeysSorted(plugins.Inputs)
	c.hardwares = map[string]plugins.Input{}
	for _, name := range c.hardwaresNames {
		inputFn := plugins.Inputs[name]
		input := inputFn()
		if init, ok := input.(plugins.Initializer); ok {
			init.Init()
		}

		c.hardwares[name] = input
	}
}

func (c *Charts) Handler(ctx *fasthttp.RequestCtx) {
	switch path := string(ctx.Path()); {
	case path == "/data/":
		ctx.SetContentType(`application/json; charset=utf-8`)
		ctx.Write(c.handleData())
	case path == "/":
		ctx.SetContentType("text/html")
		size := ctx.QueryArgs().Peek("size")
		views := ctx.QueryArgs().Peek("views")
		c.renderCharts(ctx, string(size), string(views))
	case strings.HasPrefix(path, assetsPath):
		ap := path[len(assetsPath):]
		if f, err := assetsFS.Open(ap); err != nil {
			ctx.Error(err.Error(), 404)
		} else {
			ctx.SetBodyStream(f, -1)
		}
	default:
		ctx.Error("NotFound", 404)
	}
}

func (c *Charts) mergeHardwareMetrics(s []byte) []byte {
	for _, name := range c.hardwaresNames {
		if d, err := c.hardwares[name].Gather(); err != nil {
			log.Printf("E! failed to gather %s error: %v", name, err)
		} else {
			if s, err = jj.SetBytes(s, "values."+name, d); err != nil {
				log.Printf("E! failed to set %s error: %v", name, err)
			}
		}
	}

	return s
}

func (c *Charts) handleData() []byte {
	if c.config.IsDryPlots() {
		if d := c.config.PlotsHandle.ReadAll(); len(d) > 0 {
			return d
		}
		return []byte("[]")
	}

	rd := c.chartsData()
	plots := createMetrics(rd, c.config.IsNop())
	plots = c.mergeHardwareMetrics(plots)

	return []byte(("[" + string(plots) + "]"))
}

func (c *Charts) renderCharts(w io.Writer, size, viewsArg string) {
	v := NewViews(size, c.config.IsDryPlots())
	var fns []func() components.Charter

	if !c.config.IsNop() {
		if views := util2.NewFeatures(viewsArg); len(views) == 0 {
			fns = append(fns, v.newLatencyView, v.newTPSView, v.newLatencyPercentileView)
			if !c.config.Incr.IsEmpty() || c.config.IsDryPlots() {
				fns = append(fns, v.newConcurrentView)
			}
		} else {
			if views.HasAny("latency", "l") {
				fns = append(fns, v.newLatencyView)
			}
			if views.HasAny("tps", "r") {
				fns = append(fns, v.newTPSView)
			}
			if views.HasAny("latencypercentile", "lp") {
				fns = append(fns, v.newLatencyPercentileView)
			}
			if views.HasAny("concurrent", "c") {
				fns = append(fns, v.newConcurrentView)
			}
		}
	}

	v.num = len(fns) + len(plugins.Inputs)
	p := components.NewPage()
	p.PageTitle = "perf"
	p.AssetsHost = assetsPath
	p.Assets.JSAssets.Add("jquery.min.js")

	for _, vf := range fns {
		p.AddCharts(vf())
	}
	p.AddCharts(v.newHardwareViews(c)...)
	_ = p.Render(w)
}

func (c *Charts) Serve(ln net.Listener, port int) {
	server := fasthttp.Server{
		Handler: cors.DefaultHandler().CorsMiddleware(c.Handler),
	}

	if c.config.IsDryPlots() {
		log.Printf("Running in dry mode for %s", c.config.PlotsFile)
		go osx.OpenBrowser(fmt.Sprintf("http://127.0.0.1:%d", port))
		util2.ExitIfErr(server.Serve(ln))
		return
	}

	go func() {
		time.Sleep(3 * time.Second) // 3s之后再弹出，避免运行时间过短，程序已经退出
		go osx.OpenBrowser(fmt.Sprintf("http://127.0.0.1:%d", port))
		util2.ExitIfErr(server.Serve(ln))
	}()
}
