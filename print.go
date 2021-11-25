package perf

import (
	"bytes"
	"fmt"
	"math"
	"os"
	"regexp"
	"sort"
	"strconv"
	"strings"
	"time"

	"github.com/mattn/go-isatty"
	"github.com/mattn/go-runewidth"
)

const (
	maxBarLen = 40
	barStart  = "|"
	barBody   = "■"
	barEnd    = "|"
)

var (
	barSpinner = []string{"|", "/", "-", "\\"}
	clearLine  = []byte("\r\033[K")
	isTerminal = isatty.IsTerminal(os.Stdout.Fd()) || isatty.IsCygwinTerminal(os.Stdout.Fd())
)

type Printer struct {
	maxNum      int64
	curNum      int64
	maxDuration time.Duration
	curDuration time.Duration
	pbInc       int64
	pbNumStr    string
	pbDurStr    string
	verbose     int
}

func (p *Printer) updateProgressValue(rs *SnapshotReport) {
	p.pbInc++
	if p.maxDuration > 0 {
		n := rs.Elapsed
		if n > p.maxDuration {
			n = p.maxDuration
		}
		p.curDuration = n
		barLen := int((p.curDuration*time.Duration(maxBarLen-2) + p.maxDuration/2) / p.maxDuration)
		p.pbDurStr = barStart + strings.Repeat(barBody, barLen) + strings.Repeat(" ", maxBarLen-2-barLen) + barEnd
	}
	if p.maxNum > 0 {
		p.curNum = rs.Count
		if p.maxNum > 0 {
			barLen := int((p.curNum*int64(maxBarLen-2) + p.maxNum/2) / p.maxNum)
			p.pbNumStr = barStart + strings.Repeat(barBody, barLen) + strings.Repeat(" ", maxBarLen-2-barLen) + barEnd
		} else {
			idx := p.pbInc % int64(len(barSpinner))
			p.pbNumStr = barSpinner[int(idx)]
		}
	}
}

func (p *Printer) PrintLoop(snapshot func() *SnapshotReport, interval time.Duration, useSeconds bool, doneChan <-chan struct{}, requests int) {
	stdout := os.Stdout

	var echo func(isFinal bool)
	buf := &bytes.Buffer{}

	var backCursor string
	echo = func(isFinal bool) {
		r := snapshot()
		p.updateProgressValue(r)
		stdout.WriteString(backCursor)

		buf.Reset()
		p.formatTableReports(buf, r, isFinal, useSeconds)

		n := printLines(buf.Bytes(), stdout)
		backCursor = fmt.Sprintf("\033[%dA", n)
		stdout.Sync()
	}

	if interval > 0 && requests != 1 {
		tick(interval, func() { echo(false) }, doneChan)
	} else {
		<-doneChan
	}

	if requests != 1 {
		echo(true)
	} else {
		r := snapshot()
		if p.printError(buf, r) {
			stdout.Write(buf.Bytes())
		}

		buf.Reset()
		var summary SummaryReport
		writeBulk(buf, p.buildSummary(r, true, &summary))
		stdout.Write(buf.Bytes())
	}
}

func tick(interval time.Duration, echo func(), doneChan <-chan struct{}) {
	ticker := time.NewTicker(interval)
	defer ticker.Stop()

	for {
		select {
		case <-ticker.C:
			echo()
		case <-doneChan:
			return
		}
	}
}

func printLines(result []byte, stdout *os.File) int {
	n := 0
	for ; ; n++ {
		i := bytes.IndexByte(result, '\n')
		if i < 0 {
			stdout.Write(clearLine)
			stdout.Write(result)
			break
		}
		stdout.Write(clearLine)
		stdout.Write(result[:i])
		stdout.Write([]byte("\n"))
		result = result[i+1:]
	}
	return n
}

const (
	FgBlackColor int = iota + 30
	FgRedColor
	FgGreenColor
	FgYellowColor
	FgBlueColor
	FgMagentaColor
	FgCyanColor
	FgWhiteColor
)

func colorize(s string, seq int) string {
	if !isTerminal {
		return s
	}
	return fmt.Sprintf("\033[%dm%s\033[0m", seq, s)
}

func durationToString(d time.Duration, useSeconds bool) string {
	d = d.Truncate(time.Microsecond)
	if useSeconds {
		return formatFloat64(d.Seconds())
	}
	return d.String()
}

func alignBulk(bulk [][]string, aligns ...int) {
	maxLen := map[int]int{}
	for _, b := range bulk {
		for i, bb := range b {
			lbb := displayWidth(bb)
			if maxLen[i] < lbb {
				maxLen[i] = lbb
			}
		}
	}
	for _, b := range bulk {
		for i, ali := range aligns {
			if len(b) >= i+1 {
				if i == len(aligns)-1 && ali == AlignLeft {
					continue
				}
				b[i] = padString(b[i], " ", maxLen[i], ali)
			}
		}
	}
}

func writeBulkWith(w *bytes.Buffer, bulk [][]string, lineStart, sep, lineEnd string) {
	for _, b := range bulk {
		w.WriteString(lineStart)
		w.WriteString(b[0])
		for _, bb := range b[1:] {
			w.WriteString(sep)
			w.WriteString(bb)
		}
		w.WriteString(lineEnd)
	}
}

func writeBulk(w *bytes.Buffer, bulk [][]string) {
	writeBulkWith(w, bulk, "  ", "  ", "\n")
}

func formatFloat64(f float64) string {
	return strconv.FormatFloat(f, 'f', -1, 64)
}

type Report struct {
	SummaryReport    `json:"Summary"`
	StatsReport      `json:"Stats"`
	PercentileReport `json:"Percentile"`
}

func (p *Printer) formatTableReports(w *bytes.Buffer, r *SnapshotReport, isFinal bool, useSeconds bool) Report {
	w.WriteString("Summary:\n")
	report := Report{}
	writeBulk(w, p.buildSummary(r, isFinal, &report.SummaryReport))

	w.WriteString("\n")
	p.printError(w, r)
	writeBulkWith(w, p.buildStats(r, useSeconds, &report.StatsReport), "", "  ", "\n")

	w.WriteString("\n")
	w.WriteString("Latency Percentile:\n")
	report.PercentileReport = make(map[string]string)
	writeBulk(w, p.buildPercentile(r, useSeconds, report.PercentileReport))

	if p.verbose >= 1 {
		w.WriteString("\n")
		w.WriteString("Latency Histogram:\n")
		writeBulk(w, p.buildHistogram(r, useSeconds, isFinal))
	}
	return report
}

func (p *Printer) printError(w *bytes.Buffer, r *SnapshotReport) bool {
	if errorsBulks := p.buildErrors(r); errorsBulks != nil {
		w.WriteString("Error:\n")
		writeBulk(w, errorsBulks)
		w.WriteString("\n")
		return true
	}

	return false
}

func (p *Printer) buildHistogram(r *SnapshotReport, useSeconds bool, isFinal bool) [][]string {
	hisBulk := make([][]string, 0, 8)
	maxCount := 0
	hisSum := 0
	for _, bin := range r.Histograms {
		if maxCount < bin.Count {
			maxCount = bin.Count
		}
		hisSum += bin.Count
	}
	for _, bin := range r.Histograms {
		row := []string{durationToString(bin.Mean, useSeconds), strconv.Itoa(bin.Count)}
		if isFinal {
			row = append(row, fmt.Sprintf("%.2f%%", math.Floor(float64(bin.Count)*1e4/float64(hisSum)+0.5)/100.0))
		} else {
			barLen := 0
			if maxCount > 0 {
				barLen = (bin.Count*maxBarLen + maxCount/2) / maxCount
			}
			row = append(row, strings.Repeat(barBody, barLen))
		}
		hisBulk = append(hisBulk, row)
	}
	if isFinal {
		alignBulk(hisBulk, AlignLeft, AlignRight, AlignRight)
	} else {
		alignBulk(hisBulk, AlignLeft, AlignRight, AlignLeft)
	}
	return hisBulk
}

type PercentileReport map[string]string

func (p *Printer) buildPercentile(r *SnapshotReport, useSeconds bool, report PercentileReport) [][]string {
	percBulk := make([][]string, 2)
	percAligns := make([]int, 0, len(r.Percentiles))
	for _, percentile := range r.Percentiles {
		perc := formatFloat64(percentile.Percentile * 100)
		percBulk[0] = append(percBulk[0], "P"+perc)
		percValue := durationToString(percentile.Latency, useSeconds)

		report["P"+perc] = percValue
		percBulk[1] = append(percBulk[1], percValue)
		percAligns = append(percAligns, AlignCenter)
	}
	percAligns[0] = AlignLeft
	alignBulk(percBulk, percAligns...)
	return percBulk
}

type StatItem struct {
	Min, Max, StdDev, Mean string
}

type StatsReport struct {
	Latency StatItem
	RPS     StatItem
}

func (p *Printer) buildStats(r *SnapshotReport, useSeconds bool, stats *StatsReport) [][]string {
	st := r.Stats
	dts := func(d time.Duration) string { return durationToString(d, useSeconds) }
	stats.Latency.Min = dts(st.Min)
	stats.Latency.Mean = dts(st.Mean)
	stats.Latency.StdDev = dts(st.StdDev)
	stats.Latency.Max = dts(st.Max)
	statsBulk := [][]string{
		{"Statistics", "Min", "Mean", "StdDev", "Max"},
		{"  Latency", dts(st.Min), dts(st.Mean), dts(st.StdDev), dts(st.Max)},
	}
	rs := r.RpsStats
	if rs != nil {
		fft := func(v float64) string { return formatFloat64(math.Trunc(v*100) / 100.0) }
		stats.RPS.Min = fft(rs.Min)
		stats.RPS.Mean = fft(rs.Mean)
		stats.RPS.StdDev = fft(rs.StdDev)
		stats.RPS.Max = fft(rs.Max)
		statsBulk = append(statsBulk, []string{"  RPS", fft(rs.Min), fft(rs.Mean), fft(rs.StdDev), fft(rs.Max)})
	}
	alignBulk(statsBulk, AlignLeft, AlignCenter, AlignCenter, AlignCenter, AlignCenter)
	return statsBulk
}

func (p *Printer) buildErrors(r *SnapshotReport) [][]string {
	var errorsBulks [][]string
	for k, v := range r.Errors {
		vs := colorize(strconv.FormatInt(v, 10), FgRedColor)
		errorsBulks = append(errorsBulks, []string{vs, "\"" + k + "\""})
	}
	if errorsBulks != nil {
		sort.Slice(errorsBulks, func(i, j int) bool { return errorsBulks[i][1] < errorsBulks[j][1] })
	}
	alignBulk(errorsBulks, AlignLeft, AlignLeft)
	return errorsBulks
}

type SummaryReport struct {
	Count       int64
	Counting    int64
	Elapsed     string
	RPS         string
	ReadsWrites string
}

func (p *Printer) buildSummary(r *SnapshotReport, isFinal bool, sr *SummaryReport) [][]string {
	sr.Elapsed = r.Elapsed.Truncate(time.Millisecond).String()
	elapsedLine := []string{"Elapsed", sr.Elapsed}
	if p.maxDuration > 0 && !isFinal {
		elapsedLine = append(elapsedLine, p.pbDurStr)
	}
	sr.Count = r.Count
	countLine := []string{"Count/RPS", fmt.Sprintf("%d %.3f", r.Count, r.RPS)}
	if p.maxNum > 0 && !isFinal {
		countLine = append(countLine, p.pbNumStr)
	}
	summaryBulk := [][]string{elapsedLine, countLine}
	codesBulks := make([][]string, 0, len(r.Codes))
	for k, v := range r.Codes {
		vs := fmt.Sprintf("%d %.3f", v, float64(v)/r.ElapseInSec)
		if k != "200" {
			vs = colorize(vs, FgMagentaColor)
		}
		codesBulks = append(codesBulks, []string{"  " + k, vs})
	}
	sort.Slice(codesBulks, func(i, j int) bool { return codesBulks[i][0] < codesBulks[j][0] })
	summaryBulk = append(summaryBulk, codesBulks...)

	sr.RPS = fmt.Sprintf("%.3f", r.RPS)

	if r.ReadThroughput > 0 || r.WriteThroughput > 0 {
		sr.ReadsWrites = fmt.Sprintf("%.3f %.3f MiB/s", r.ReadThroughput, r.WriteThroughput)
		summaryBulk = append(summaryBulk, []string{"ReadWrite", sr.ReadsWrites})
	}

	sr.Counting = r.Counting
	if p.verbose >= 1 && sr.Counting > 1 {
		summaryBulk = append(summaryBulk, []string{"Counting", fmt.Sprintf("%d", r.Counting)})
	}

	alignBulk(summaryBulk, AlignLeft, AlignRight)
	return summaryBulk
}

var ansi = regexp.MustCompile("\033\\[(?:[0-9]{1,3}(?:;[0-9]{1,3})*)?[m|K]")

func displayWidth(str string) int {
	return runewidth.StringWidth(ansi.ReplaceAllLiteralString(str, ""))
}

const (
	AlignLeft = iota
	AlignRight
	AlignCenter
)

func padString(s, pad string, width, align int) string {
	if gap := width - displayWidth(s); gap > 0 {
		switch align {
		case AlignLeft:
			return s + strings.Repeat(pad, gap)
		case AlignRight:
			return strings.Repeat(pad, gap) + s
		case AlignCenter:
			gapLeft := gap / 2
			gapRight := gap - gapLeft
			return strings.Repeat(pad, gapLeft) + s + strings.Repeat(pad, gapRight)
		}
	}
	return s
}
