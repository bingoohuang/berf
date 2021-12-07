package disk

import (
	"log"
	"sort"

	"github.com/bingoohuang/berf/pkg/util"
	"github.com/bingoohuang/berf/plugins"
	"github.com/bingoohuang/berf/plugins/system"
)

type DiskStats struct {
	ps   system.PS
	path string

	MountPoints []string
	IgnoreFS    []string
}

func (ds *DiskStats) Series() plugins.Series {
	ps := plugins.Series{}
	disks, _, err := ds.ps.DiskUsage(ds.MountPoints, ds.IgnoreFS)
	if err != nil {
		log.Printf("E! error getting disk usage info: %s", err)
		return ps
	}

	var paths []string
	for _, du := range disks {
		if du.Total == 0 { // Skip dummy filesystem (procfs, cgroupfs, ...)
			continue
		}

		paths = append(paths, du.Path)
	}

	sort.Strings(paths)
	if len(paths) > 0 {
		ds.path = paths[0]
	}

	for _, p := range paths {
		pp := p + "-"

		ps.Series = append(ps.Series,
			pp+"total",
			pp+"free",
			pp+"used",
			pp+"used_percent",
			pp+"inodes_total",
			pp+"inodes_free",
			pp+"inodes_used")

		ps.Selected = append(ps.Selected,
			pp+"free",
			pp+"used")

		break
	}

	return ps
}

func (ds *DiskStats) Gather() ([]interface{}, error) {
	disks, _, err := ds.ps.DiskUsage(ds.MountPoints, ds.IgnoreFS)
	if err != nil {
		log.Printf("E! error getting disk usage info: %s", err)
		return nil, err
	}

	var points []interface{}

	for _, du := range disks {
		if du.Total == 0 { // Skip dummy filesystem (procfs, cgroupfs, ...)
			continue
		}

		if du.Path != ds.path {
			continue
		}

		var usedPercent float64
		if du.Used+du.Free > 0 {
			usedPercent = float64(du.Used) / float64(du.Used+du.Free) * 100
		}

		points = append(points,
			util.BytesToGiga(du.Total),
			util.BytesToGiga(du.Free),
			util.BytesToGiga(du.Used),
			util.Float64(usedPercent),
			du.InodesTotal,
			du.InodesFree,
			du.InodesUsed,
		)
	}

	return points, nil
}

func init() {
	plugins.Add("disk", func() plugins.Input {
		return &DiskStats{
			ps:       system.NewSystemPS(),
			IgnoreFS: []string{"tmpfs", "devtmpfs", "devfs", "iso9660", "overlay", "aufs", "squashfs"},
		}
	})
}
