package mem

import (
	"fmt"
	"runtime"

	"github.com/bingoohuang/perf/plugins/internal"

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

	f := map[string]float64{
		"total":  internal.BytesToGiga(vm.Total),
		"avail":  internal.BytesToGiga(vm.Available),
		"used":   internal.BytesToGiga(vm.Used),
		"%used":  100 * float64(vm.Used) / float64(vm.Total),
		"%avail": 100 * float64(vm.Available) / float64(vm.Total),
	}

	switch ms.platform {
	case "darwin":
		f["active"] = internal.BytesToGiga(vm.Active)
		f["free"] = internal.BytesToGiga(vm.Free)
		f["inactive"] = internal.BytesToGiga(vm.Inactive)
		f["wired"] = internal.BytesToGiga(vm.Wired)
	case "openbsd":
		f["active"] = internal.BytesToGiga(vm.Active)
		f["cached"] = internal.BytesToGiga(vm.Cached)
		f["free"] = internal.BytesToGiga(vm.Free)
		f["inactive"] = internal.BytesToGiga(vm.Inactive)
		f["wired"] = internal.BytesToGiga(vm.Wired)
	case "freebsd":
		f["active"] = internal.BytesToGiga(vm.Active)
		f["buffered"] = internal.BytesToGiga(vm.Buffers)
		f["cached"] = internal.BytesToGiga(vm.Cached)
		f["free"] = internal.BytesToGiga(vm.Free)
		f["inactive"] = internal.BytesToGiga(vm.Inactive)
		f["wired"] = internal.BytesToGiga(vm.Wired)
	case "linux":
		f["active"] = internal.BytesToGiga(vm.Active)
		f["buffered"] = internal.BytesToGiga(vm.Buffers)
		f["cached"] = internal.BytesToGiga(vm.Cached)
		f["dirty"] = internal.BytesToGiga(vm.Dirty)
		f["free"] = internal.BytesToGiga(vm.Free)
	}

	return []interface{}{f["total"], f["avail"], f["used"], f["%used"], f["%avail"], f["active"], f["buffered"], f["cached"], f["free"]}, nil
}

func init() {
	plugins.Add("mem", func() plugins.Input {
		return &MemStats{}
	})
}
