package perf

import (
	"bytes"
	"embed"
	"encoding/json"
	"fmt"
	"log"
	"net"
	"os/exec"
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

//go:embed echarts.min.js
//go:embed jquery.min.js
var assetsFS embed.FS

const (
	assetsPath            = "/echarts/statics/"
	dataPath              = "/data/"
	apiPath               = "/api/"
	latencyView           = "latency"
	latencyPercentileView = "latencyPercentile"
	rpsView               = "rps"
	concurrentView        = "concurrent"
	timeFormat            = "15:04:05"
	refreshInterval       = time.Second
)

const (
	ViewTpl = `
$(function () { setInterval({{ .ViewID }}_sync, {{ .Interval }}); });
function {{ .ViewID }}_sync() {
    $.ajax({
        type: "GET",
        url: "{{ .APIPath }}{{ .Route }}",
        dataType: "json",
        success: function (result) {
            let opt = goecharts_{{ .ViewID }}.getOption();
            let x = opt.xAxis[0].data;
            x.push(result.time);
            opt.xAxis[0].data = x;
            for (let i = 0; i < result.values.length; i++) {
                let y = opt.series[i].data;
                y.push({ value: result.values[i] });
                opt.series[i].data = y;
                goecharts_{{ .ViewID }}.setOption(opt);
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
<p align="center">🚀 <a href="https://github.com/bingoohuang/blow"><b>Blow</b></a> %s</p>
<style> .box { justify-content:center; display:flex; flex-wrap:wrap } </style>
<div class="box"> {{- range .Charts }} {{ template "base" . }} {{- end }} </div>
</body>
</html>
{{ end }}
`
)

func (c *Charts) genViewTemplate(vid, route string) string {
	tpl, err := template.New("view").Parse(ViewTpl)
	if err != nil {
		panic("failed to parse template " + err.Error())
	}

	d := struct {
		Interval int
		APIPath  string
		Route    string
		ViewID   string
	}{
		Interval: int(refreshInterval.Milliseconds()),
		APIPath:  dataPath,
		Route:    route,
		ViewID:   vid,
	}

	buf := bytes.Buffer{}
	if err := tpl.Execute(&buf, d); err != nil {
		panic("failed to execute template " + err.Error())
	}

	return buf.String()
}

func (c *Charts) newBasicView(route string) *charts.Line {
	g := charts.NewLine()
	g.SetGlobalOptions(charts.WithTooltipOpts(opts.Tooltip{Show: true, Trigger: "axis"}),
		charts.WithXAxisOpts(opts.XAxis{Name: "Time"}),
		charts.WithInitializationOpts(opts.Initialization{Width: "800px", Height: "400px"}),
		charts.WithDataZoomOpts(opts.DataZoom{Type: "slider", XAxisIndex: []int{0}}),
	)
	g.SetXAxis([]string{}).SetSeriesOptions(charts.WithLineChartOpts(opts.LineChart{Smooth: true}))
	g.AddJSFuncs(c.genViewTemplate(g.ChartID, route))
	return g
}

func (c *Charts) newLatencyView() components.Charter {
	g := c.newBasicView(latencyView)
	g.SetGlobalOptions(charts.WithTitleOpts(opts.Title{Title: "Latency"}),
		charts.WithYAxisOpts(opts.YAxis{Scale: true, AxisLabel: &opts.AxisLabel{Formatter: "{value} ms"}}),
		charts.WithLegendOpts(opts.Legend{Show: true, Selected: map[string]bool{
			"Min": false, "Max": false, "StdDev": false,
		}}),
	)
	g.AddSeries("Min", []opts.LineData{}).AddSeries("Mean", []opts.LineData{}).
		AddSeries("StdDev", []opts.LineData{}).AddSeries("Max", []opts.LineData{})
	return g
}

func (c *Charts) newLatencyPercentileView() components.Charter {
	g := c.newBasicView(latencyPercentileView)
	g.SetGlobalOptions(
		charts.WithTitleOpts(opts.Title{Title: "Latency Percentile"}),
		charts.WithYAxisOpts(opts.YAxis{Scale: true, AxisLabel: &opts.AxisLabel{Formatter: "{value} ms"}}),
		charts.WithLegendOpts(opts.Legend{Show: true, Selected: map[string]bool{
			"P75": false, "P95": false, "P99.9": false, "P99.99": false,
		}}),
	)
	g.AddSeries("P50", []opts.LineData{}).AddSeries("P75", []opts.LineData{}).
		AddSeries("P90", []opts.LineData{}).AddSeries("P95", []opts.LineData{}).
		AddSeries("P99", []opts.LineData{}).AddSeries("P99.9", []opts.LineData{}).
		AddSeries("P99.99", []opts.LineData{})
	return g
}

func (c *Charts) newConcurrentView() components.Charter {
	g := c.newBasicView(concurrentView)
	g.SetGlobalOptions(charts.WithTitleOpts(opts.Title{Title: "Concurrent"}), charts.WithYAxisOpts(opts.YAxis{Scale: true}))
	g.AddSeries("Concurrent", []opts.LineData{})
	return g
}

func (c *Charts) newRPSView() components.Charter {
	g := c.newBasicView(rpsView)
	g.SetGlobalOptions(charts.WithTitleOpts(opts.Title{Title: "TPS"}), charts.WithYAxisOpts(opts.YAxis{Scale: true}))
	g.AddSeries("RPS", []opts.LineData{})
	return g
}

type Metrics struct {
	Values []interface{} `json:"values"`
	Time   string        `json:"time"`
}

type Charts struct {
	page     *components.Page
	ln       net.Listener
	dataFunc func() *ChartsReport
}

func NewCharts(ln net.Listener, dataFunc func() *ChartsReport, desc string) (*Charts, error) {
	templates.PageTpl = fmt.Sprintf(PageTpl, desc)

	c := &Charts{ln: ln, dataFunc: dataFunc}
	c.page = components.NewPage()
	c.page.PageTitle = "blow"
	c.page.AssetsHost = assetsPath
	c.page.Assets.JSAssets.Add("jquery.min.js")
	c.page.AddCharts(c.newLatencyView(), c.newRPSView(), c.newLatencyPercentileView(), c.newConcurrentView())

	return c, nil
}

func (c *Charts) Handler(ctx *fasthttp.RequestCtx) {
	switch path := string(ctx.Path()); {
	case strings.HasPrefix(path, dataPath):
		m := &Metrics{Time: time.Now().Format(timeFormat)}
		rd := c.dataFunc()
		switch view := path[len(dataPath):]; view {
		case latencyPercentileView:
			if rd != nil {
				m.Values = append(m.Values, rd.LatencyPercentiles...)
			} else {
				m.Values = append(m.Values, nil, nil, nil, nil, nil, nil, nil)
			}
		case latencyView:
			if rd != nil {
				m.Values = append(m.Values, rd.Latency...)
			} else {
				m.Values = append(m.Values, nil, nil, nil, nil)
			}
		case concurrentView:
			if rd != nil {
				m.Values = append(m.Values, rd.Concurrent)
			} else {
				m.Values = append(m.Values, nil)
			}
		case rpsView:
			if rd != nil {
				m.Values = append(m.Values, rd.RPS)
			} else {
				m.Values = append(m.Values, nil)
			}
		}
		_ = json.NewEncoder(ctx).Encode(m)
	case path == "/":
		ctx.SetContentType("text/html")
		_ = c.page.Render(ctx)
	case strings.HasPrefix(path, assetsPath):
		ap := path[len(assetsPath):]
		if f, err := assetsFS.Open(ap); err != nil {
			ctx.Error(err.Error(), 404)
		} else {
			ctx.SetBodyStream(f, -1)
		}
	case strings.HasPrefix(path, apiPath):
		if n, err := ctx.Write(ctx.Request.Body()); err != nil {
			log.Println(err)
		} else if n == 0 {
			ctx.SetContentType(`application/json; charset=utf-8`)
			_, _ = ctx.Write([]byte(`{"status": 200, "message": "OK"}`))
		}
	default:
		ctx.Error("NotFound", fasthttp.StatusNotFound)
	}
}

func (c *Charts) Serve(port int) {
	server := fasthttp.Server{
		Handler: cors.DefaultHandler().CorsMiddleware(c.Handler),
	}

	time.Sleep(3 * time.Second) // 3s之后再弹出，避免运行时间过短，程序已经退出

	go OpenBrowser(fmt.Sprintf("http://127.0.0.1:%d", port))

	err := server.Serve(c.ln)
	ExitIfErr(err)
}

// appearsSuccessful reports whether the command appears to have run successfully.
// If the command runs longer than the timeout, it's deemed successful.
// If the command runs within the timeout, it's deemed successful if it exited cleanly.
func appearsSuccessful(cmd *exec.Cmd, timeout time.Duration) bool {
	errc := make(chan error, 1)
	go func() {
		errc <- cmd.Wait()
	}()

	select {
	case <-time.After(timeout):
		return true
	case err := <-errc:
		return err == nil
	}
}
