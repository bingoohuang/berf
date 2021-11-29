package mem

import (
	"fmt"
	"runtime"

	"github.com/bingoohuang/perf/pkg/util"

	"github.com/bingoohuang/perf/plugins"
	"github.com/bingoohuang/perf/plugins/system"
)

type MemStats struct {
	ps       system.PS
	platform string
}

func (ms *MemStats) Init() error {
	ms.platform = runtime.GOOS
	ms.ps = system.NewSystemPS()
	return nil
}

func (ms *MemStats) Series() plugins.Series {
	return plugins.Series{
		Series:   []string{"total", "avail", "used", "%used", "%avail", "active", "buffered", "cached", "free"},
		Selected: []string{"avail", "used"},
	}
}

func (ms *MemStats) Gather() ([]interface{}, error) {
	vm, err := ms.ps.VMStat()
	if err != nil {
		return nil, fmt.Errorf("error getting virtual memory info: %s", err)
	}

	f := map[string]util.Float64{
		"total":  util.BytesToGiga(vm.Total),
		"avail":  util.BytesToGiga(vm.Available),
		"used":   util.BytesToGiga(vm.Used),
		"%used":  util.Float64(100 * float64(vm.Used) / float64(vm.Total)),
		"%avail": util.Float64(100 * float64(vm.Available) / float64(vm.Total)),
	}

	switch ms.platform {
	case "darwin":
		f["active"] = util.BytesToGiga(vm.Active)
		f["free"] = util.BytesToGiga(vm.Free)
		f["inactive"] = util.BytesToGiga(vm.Inactive)
		f["wired"] = util.BytesToGiga(vm.Wired)
	case "openbsd":
		f["active"] = util.BytesToGiga(vm.Active)
		f["cached"] = util.BytesToGiga(vm.Cached)
		f["free"] = util.BytesToGiga(vm.Free)
		f["inactive"] = util.BytesToGiga(vm.Inactive)
		f["wired"] = util.BytesToGiga(vm.Wired)
	case "freebsd":
		f["active"] = util.BytesToGiga(vm.Active)
		f["buffered"] = util.BytesToGiga(vm.Buffers)
		f["cached"] = util.BytesToGiga(vm.Cached)
		f["free"] = util.BytesToGiga(vm.Free)
		f["inactive"] = util.BytesToGiga(vm.Inactive)
		f["wired"] = util.BytesToGiga(vm.Wired)
	case "linux":
		f["active"] = util.BytesToGiga(vm.Active)
		f["buffered"] = util.BytesToGiga(vm.Buffers)
		f["cached"] = util.BytesToGiga(vm.Cached)
		f["dirty"] = util.BytesToGiga(vm.Dirty)
		f["free"] = util.BytesToGiga(vm.Free)
	}

	return []interface{}{f["total"], f["avail"], f["used"], f["%used"], f["%avail"], f["active"], f["buffered"], f["cached"], f["free"]}, nil
}

func init() {
	plugins.Add("mem", func() plugins.Input {
		return &MemStats{}
	})
}
