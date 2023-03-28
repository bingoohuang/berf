package diskio

import (
	"fmt"
	"log"
	"os"
	"strings"
	"time"

	"github.com/bingoohuang/berf/pkg/filter"
	"github.com/bingoohuang/berf/pkg/util"
	"github.com/bingoohuang/berf/plugins"
	"github.com/bingoohuang/berf/plugins/internal"
	"github.com/bingoohuang/berf/plugins/system"
	"github.com/bingoohuang/gg/pkg/ss"
	"github.com/shirou/gopsutil/v3/disk"
)

type DiskIO struct {
	lastStatsTime time.Time

	ps system.PS

	deviceFilter filter.Filter

	infoCache map[string]diskInfoCache
	names     map[string]bool
	Devices   []string

	gCurNetStats  []ioStat
	gPrevNetStats []ioStat
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
		)

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
			Name:       io.Name,
			ReadCount:  io.ReadCount,
			WriteCount: io.WriteCount,
			ReadBytes:  io.ReadBytes,
			WriteBytes: io.WriteBytes,
			ReadTime:   io.ReadTime,
			WriteTime:  io.WriteTime,
			IoTime:     io.IoTime,
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

/*
root@bjca-PC:~/bingoohuang# lsblk -f
NAME               FSTYPE      LABEL     UUID                                   FSAVAIL FSUSE% MOUNTPOINT
sda
├─sda1             vfat        EFI       B29D-D8B4                                 299M     0% /boot/efi
├─sda2             ext4        Boot      ab774015-5261-44d5-94b0-b8d124693652      1.2G    12% /boot
├─sda3             ext4        Roota     7c3eb2ff-ae7b-4b6c-bc62-3075a6248371    129.4G     7% /
├─sda4             ext4        _dde_data e585ad0c-4c32-449e-baee-21ff25e24151    861.5G     2% /data
├─sda5             ext4        Backup    68de0a20-364b-49a7-8d24-26885af87275         0   100% /recovery
└─sda6             swap        SWAP      a8be7006-f049-4ccf-a112-b502bd4580ec                  [SWAP]
sdb
├─sdb1             vfat                  4939-432F
├─sdb2             xfs                   26b1f940-ea98-4cd8-9341-6048516fdb4a
└─sdb3             LVM2_member           gz5CWb-9daZ-xDpp-p6jQ-DSl7-XUlu-SifASi
sdc
├─sdc1             vfat                  6813-2B66
├─sdc2             ext4                  26de7823-c690-41cd-9fce-601002e8e253
└─sdc3             LVM2_member           VBcgeb-JwHk-zTZJ-5pAx-eZ17-Qk0c-xnuAjT
  ├─openeuler-swap swap                  fcd0cc39-e927-48b1-91a9-cf69657e28e6
  ├─openeuler-home ext4                  2aa00937-7212-4e35-9ede-8c4567a94811
  └─openeuler-root ext4                  7c800ccc-afbf-46d5-8b4d-b5a66fd2cc1d
sdd
*/

func init() {
	plugins.Add("diskio", func() plugins.Input {
		devices := []string{"sda", "sdb", "vd*"}
		if s := os.Getenv("BERF_DISK"); s != "" {
			devices = ss.Split(s)
		}
		return &DiskIO{
			Devices: devices,
			ps:      system.NewSystemPS(),
		}
	})
}
