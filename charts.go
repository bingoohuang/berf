package perf

import (
	"bytes"
	"embed"
	"fmt"
	"log"
	"net"
	"strings"
	"text/template"
	"time"

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

const (
	assetsPath = "/echarts/statics/"
	dataPath   = "/data/"

	latencyView           = "latency"
	latencyPercentileView = "latencyPercentile"
	rpsView               = "rps"
	concurrentView        = "concurrent"
)

var viewSeriesNum = map[string]int{latencyView: 4, latencyPercentileView: 7, rpsView: 1, concurrentView: 1}

const (
	ViewTpl = `
$(function () {
{{ .SetInterval }}(views_sync, {{ .Interval }}); });
let views = {{.ViewsMap}};
function views_sync() {
	$.ajax({
		type: "GET",
		url: "{{ .APIPath }}",
		dataType: "json",
		success: function (dictArr) {
			for (let j = 0; j < dictArr.length; j++) {
				let dict = dictArr[j];
				for (let key in dict.values) {
					let view = views[key];
					if (!view) {
						continue;
					}
					let opt = view.getOption();
	
					let x = opt.xAxis[0].data;
					x.push(dict.time);
					opt.xAxis[0].data = x;
					
					let arr = dict.values[key];
					for (let i = 0; i < arr.length; i++) {
						let y = opt.series[i].data;
						y.push({ value: arr[i]});
						opt.series[i].data = y;
					}
					view.setOption(opt);
				}
			}
		}
	});
}`
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

func (c *Views) genViewTemplate(routerChartsMap map[string]string) string {
	tpl, err := template.New("view").Parse(ViewTpl)
	if err != nil {
		panic("failed to parse template " + err.Error())
	}

	viewsMap := "{"
	for k, v := range routerChartsMap {
		viewsMap += k + ": goecharts_" + v + ","
	}
	viewsMap += "noop: 123}"

	d := struct {
		Interval    int
		APIPath     string
		ViewsMap    string
		SetInterval string
	}{
		Interval:    int(time.Second.Milliseconds()),
		APIPath:     dataPath,
		ViewsMap:    viewsMap,
		SetInterval: "setInterval",
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
	routerChartsMap map[string]string
	num             int
	size            WidthHeight
	dryPlots        bool
}

func NewViews(num int, size string, dryPlots bool) *Views {
	return &Views{
		num:             num,
		routerChartsMap: make(map[string]string),
		size:            ParseWidthHeight(size, 800, 400),
		dryPlots:        dryPlots,
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

	c.routerChartsMap[route] = g.ChartID
	if len(c.routerChartsMap) == c.num {
		g.AddJSFuncs(c.genViewTemplate(c.routerChartsMap))
	}
	return g
}

func (c *Views) newLatencyView() components.Charter {
	g := c.newBasicView(latencyView)
	g.SetGlobalOptions(charts.WithTitleOpts(opts.Title{Title: "Latency"}),
		charts.WithYAxisOpts(opts.YAxis{Scale: true, AxisLabel: &opts.AxisLabel{Formatter: "{value} ms"}}),
		charts.WithLegendOpts(opts.Legend{Show: true, Selected: map[string]bool{
			"Min": false, "Max": false, "StdDev": false,
		}}),
	)

	for _, p := range []string{"Min", "Mean", "StdDev", "Max"} {
		g.AddSeries(p, []opts.LineData{})
	}
	return g
}

func (c *Views) newLatencyPercentileView() components.Charter {
	g := c.newBasicView(latencyPercentileView)
	g.SetGlobalOptions(
		charts.WithTitleOpts(opts.Title{Title: "Latency Percentile"}),
		charts.WithYAxisOpts(opts.YAxis{Scale: true, AxisLabel: &opts.AxisLabel{Formatter: "{value} ms"}}),
		charts.WithLegendOpts(opts.Legend{Show: true, Selected: map[string]bool{
			"P75": false, "P95": false, "P99.9": false, "P99.99": false,
		}}),
	)

	for _, p := range []string{"P50", "P75", "P90", "P95", "P99", "P99.9", "P99.99"} {
		g.AddSeries(p, []opts.LineData{})
	}
	return g
}

func (c *Views) newConcurrentView() components.Charter {
	g := c.newBasicView(concurrentView)
	g.SetGlobalOptions(charts.WithTitleOpts(opts.Title{Title: "Concurrent"}), charts.WithYAxisOpts(opts.YAxis{Scale: true}))
	g.AddSeries("Concurrent", []opts.LineData{})
	return g
}

func (c *Views) newRPSView() components.Charter {
	g := c.newBasicView(rpsView)
	g.SetGlobalOptions(charts.WithTitleOpts(opts.Title{Title: "TPS"}), charts.WithYAxisOpts(opts.YAxis{Scale: true}))
	g.AddSeries("RPS", []opts.LineData{})
	return g
}

type Charts struct {
	ln         net.Listener
	chartsData chan []byte
	config     *Config
}

func NewCharts(ln net.Listener, chartsData chan []byte, desc string, config *Config) *Charts {
	templates.PageTpl = fmt.Sprintf(PageTpl, desc)
	return &Charts{ln: ln, chartsData: chartsData, config: config}
}

func (c *Charts) Handler(ctx *fasthttp.RequestCtx) {
	switch path := string(ctx.Path()); {
	case path == dataPath:
		ctx.SetContentType(`application/json; charset=utf-8`)

		if c.config.IsDryPlots() {
			data := c.config.PlotsHandle.ReadAll()
			if len(data) > 2 {
				ctx.WriteString("[" + string(data[:len(data)-2]) + "]")
			} else {
				ctx.WriteString("[]")
			}
			return
		}

		select {
		case data := <-c.chartsData:
			ctx.WriteString("[" + string(data) + "]")
		default:
			ctx.WriteString("[]")
		}
	case path == "/":
		ctx.SetContentType("text/html")
		viewNum := 3
		if !c.config.Incr.IsEmpty() {
			viewNum++
		}
		v := NewViews(viewNum, string(ctx.QueryArgs().Peek("size")), c.config.IsDryPlots())
		page := components.NewPage()
		page.PageTitle = "perf"
		page.AssetsHost = assetsPath
		page.Assets.JSAssets.Add("jquery.min.js")
		page.AddCharts(v.newLatencyView(), v.newRPSView(), v.newLatencyPercentileView())
		if !c.config.Incr.IsEmpty() {
			page.AddCharts(v.newConcurrentView())
		}
		_ = page.Render(ctx)
	case strings.HasPrefix(path, assetsPath):
		ap := path[len(assetsPath):]
		if f, err := assetsFS.Open(ap); err != nil {
			ctx.Error(err.Error(), 404)
		} else {
			ctx.SetBodyStream(f, -1)
		}
	default:
		ctx.Error("NotFound", fasthttp.StatusNotFound)
	}
}

func (c *Charts) Serve(port int) {
	server := fasthttp.Server{
		Handler: cors.DefaultHandler().CorsMiddleware(c.Handler),
	}

	if c.config.IsDryPlots() {
		log.Printf("Running in dry mode for %s", c.config.PlotsFile)
		go OpenBrowser(fmt.Sprintf("http://127.0.0.1:%d", port))
		ExitIfErr(server.Serve(c.ln))
		return
	}

	go func() {
		time.Sleep(3 * time.Second) // 3s之后再弹出，避免运行时间过短，程序已经退出
		go OpenBrowser(fmt.Sprintf("http://127.0.0.1:%d", port))
		ExitIfErr(server.Serve(c.ln))
	}()
}
