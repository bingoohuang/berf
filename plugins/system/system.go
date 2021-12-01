package system

import (
	"strings"

	"github.com/bingoohuang/berf/pkg/util"

	"github.com/bingoohuang/berf/plugins"
	"github.com/shirou/gopsutil/v3/cpu"
	"github.com/shirou/gopsutil/v3/host"
	"github.com/shirou/gopsutil/v3/load"
)

type SystemStats struct{}

func (s *SystemStats) Series() plugins.Series {
	return plugins.Series{
		Series:   []string{"load1", "load5", "load15", "n_cpus", "n_users"},
		Selected: []string{"load1", "load5", "load15"},
	}
}

func (s *SystemStats) Gather() ([]interface{}, error) {
	loadavg, err := load.Avg()
	if err != nil && !strings.Contains(err.Error(), "not implemented") {
		return nil, err
	}

	numCPUs, err := cpu.Counts(true)
	if err != nil {
		return nil, err
	}

	users, _ := host.Users()
	return []interface{}{util.Float64(loadavg.Load1), util.Float64(loadavg.Load5), util.Float64(loadavg.Load15), numCPUs, len(users)}, nil
}

func init() {
	plugins.Add("system", func() plugins.Input {
		return &SystemStats{}
	})
}
