package diskio

import (
	"fmt"
	"log"
	"strings"
	"time"

	"github.com/bingoohuang/berf/pkg/filter"
	"github.com/bingoohuang/berf/pkg/util"
	"github.com/bingoohuang/berf/plugins"
	"github.com/bingoohuang/berf/plugins/internal"
	"github.com/bingoohuang/berf/plugins/system"
	"github.com/shirou/gopsutil/v3/disk"
)

type DiskIO struct {
	ps system.PS

	Devices []string

	infoCache     map[string]diskInfoCache
	deviceFilter  filter.Filter
	names         map[string]bool
	gCurNetStats  []ioStat
	gPrevNetStats []ioStat
	lastStatsTime time.Time
}

func (d *DiskIO) Series() plugins.Series {
	ps := plugins.Series{}

	diskio, err := d.diskIO()
	if err != nil {
		log.Printf("E! error getting disk io info: %s", err.Error())
		return ps
	}

	d.names = make(map[string]bool)
	for _, io := range diskio {
		if len(diskio) > 1 && !d.match(io.Name) {
			continue
		}

		d.names[io.Name] = true
	}

	for _, io := range diskio {
		pp := io.Name
		if !d.names[io.Name] {
			continue
		}

		if len(d.names) == 1 {
			pp = ""
		} else {
			pp += ":"
		}

		ps.Series = append(ps.Series,
			pp+"reads",
			pp+"writes",
			pp+"read_bytes",
			pp+"write_bytes",
			pp+"read_time",
			pp+"write_time",
			pp+"io_time",
			pp+"weighted_io_time",
			pp+"iops_in_progress",
			pp+"merged_reads",
			pp+"merged_writes")

		ps.Selected = append(ps.Selected,
			pp+"read_bytes",
			pp+"write_bytes")
	}

	return ps
}

// hasMeta reports whether s contains any special glob characters.
func hasMeta(s string) bool {
	return strings.ContainsAny(s, "*?[")
}

func (d *DiskIO) Init() error {
	for _, device := range d.Devices {
		if hasMeta(device) {
			deviceFilter, err := filter.Compile(d.Devices)
			if err != nil {
				return fmt.Errorf("error compiling device pattern: %s", err.Error())
			}
			d.deviceFilter = deviceFilter
		}
	}
	return nil
}

func (d *DiskIO) Gather() ([]interface{}, error) {
	diskio, err := d.diskIO()
	if err != nil {
		return nil, err
	}

	var stats []ioStat
	for _, io := range diskio {
		if !d.names[io.Name] {
			continue
		}

		stats = append(stats, ioStat{
			Name:             io.Name,
			ReadCount:        io.ReadCount,
			WriteCount:       io.WriteCount,
			ReadBytes:        io.ReadBytes,
			WriteBytes:       io.WriteBytes,
			ReadTime:         io.ReadTime,
			WriteTime:        io.WriteTime,
			IoTime:           io.IoTime,
			WeightedIO:       io.WeightedIO,
			IopsInProgress:   io.IopsInProgress,
			MergedReadCount:  io.MergedReadCount,
			MergedWriteCount: io.MergedWriteCount,
		})
	}

	return d.diff(stats), nil
}

func (d *DiskIO) diskIO() (map[string]disk.IOCountersStat, error) {
	var devices []string
	if d.deviceFilter == nil {
		devices = d.Devices
	}

	diskio, err := d.ps.DiskIO(devices)
	if err != nil {
		return nil, fmt.Errorf("error getting disk io info: %s", err.Error())
	}
	return diskio, nil
}

func (d *DiskIO) match(name string) bool {
	if d.deviceFilter != nil && d.deviceFilter.Match(name) {
		return true
	}
	return d.deviceFilter == nil
}

func (d *DiskIO) diff(stat []ioStat) []interface{} {
	d.gPrevNetStats = d.gCurNetStats
	d.gCurNetStats = stat

	du := time.Since(d.lastStatsTime)
	d.lastStatsTime = time.Now()

	var points []interface{}
	for _, ns := range d.gCurNetStats {
		di := getStatDiff(ns, d.gPrevNetStats)
		points = append(points,
			util.NumberToRate(di.ReadCount, du),
			util.NumberToRate(di.WriteCount, du),
			util.BytesToMBS(di.ReadBytes, du),
			util.BytesToMBS(di.WriteBytes, du),
			util.Float64(di.ReadTime),
			util.Float64(di.WriteTime),
			util.Float64(di.IoTime),
			util.Float64(di.WeightedIO),
			util.Float64(di.IopsInProgress),
			util.Float64(di.MergedReadCount),
			util.Float64(di.MergedWriteCount),
		)
	}

	return points
}

func getStatDiff(cur ioStat, pre []ioStat) ioStat {
	for _, p := range pre {
		if p.Name == cur.Name {
			cur.ReadCount = internal.Adjust(cur.ReadCount, p.ReadCount)
			cur.WriteCount = internal.Adjust(cur.WriteCount, p.WriteCount)
			cur.ReadBytes = internal.Adjust(cur.ReadBytes, p.ReadBytes)
			cur.WriteBytes = internal.Adjust(cur.WriteBytes, p.WriteBytes)
			cur.ReadTime = internal.Adjust(cur.ReadTime, p.ReadTime)
			cur.WriteTime = internal.Adjust(cur.WriteTime, p.WriteTime)
			cur.IoTime = internal.Adjust(cur.IoTime, p.IoTime)
			cur.WeightedIO = internal.Adjust(cur.WeightedIO, p.WeightedIO)
			cur.IopsInProgress = internal.Adjust(cur.IopsInProgress, p.IopsInProgress)
			cur.MergedReadCount = internal.Adjust(cur.MergedReadCount, p.MergedReadCount)
			cur.MergedWriteCount = internal.Adjust(cur.MergedWriteCount, p.MergedWriteCount)
			break
		}
	}
	return cur
}

type ioStat struct {
	Name string

	ReadCount        uint64
	WriteCount       uint64
	ReadBytes        uint64
	WriteBytes       uint64
	ReadTime         uint64
	WriteTime        uint64
	IoTime           uint64
	WeightedIO       uint64
	IopsInProgress   uint64
	MergedReadCount  uint64
	MergedWriteCount uint64
}

func init() {
	plugins.Add("diskio", func() plugins.Input {
		return &DiskIO{
			ps:      system.NewSystemPS(),
			Devices: []string{"sda", "sdb", "vd*"},
		}
	})
}
