package system

import (
	"fmt"
	"math"
	"strings"
	"sync"

	"github.com/bingoohuang/berf/pkg/util"
	"github.com/bingoohuang/berf/plugins"
	"github.com/shirou/gopsutil/v3/cpu"
	"github.com/shirou/gopsutil/v3/host"
	"github.com/shirou/gopsutil/v3/load"
)

type SystemStats struct{}

func (s *SystemStats) Init() error {
	_, _ = CPUBusyPercent()
	return nil
}

func (s *SystemStats) Series() plugins.Series {
	return plugins.Series{
		Series:   []string{"load1", "load5", "load15", "n_cpus", "n_users", "p_cpu"},
		Selected: []string{"p_cpu"},
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

	pcpus, _ := CPUBusyPercent()
	pcpu := float64(0)
	if len(pcpus) == 1 {
		pcpu = pcpus[0]
	}
	return []interface{}{
		util.Float64(loadavg.Load1), util.Float64(loadavg.Load5), util.Float64(loadavg.Load15),
		numCPUs, len(users), util.Float64(pcpu),
	}, nil
}

func init() {
	plugins.Add("system", func() plugins.Input {
		return &SystemStats{}
	})
}

// 以下代码拷贝自 github.com/shirou/gopsutil/v3/cpu/cpu.go 中

// CPUBusyPercent calculates the percentage of cpu used either per CPU or combined.
// If an interval of 0 is given it will compare the current cpu times against the last call.
// Returns one value per cpu, or a single value if percpu is set to false.
func CPUBusyPercent() ([]float64, error) {
	cpuTimes, err := cpu.Times(false)
	if err != nil {
		return nil, err
	}
	lastCPUPercent.Lock()
	defer lastCPUPercent.Unlock()
	lastTimes := lastCPUPercent.lastCPUTimes
	lastCPUPercent.lastCPUTimes = cpuTimes

	if lastTimes == nil {
		return nil, fmt.Errorf("error getting times for cpu percent. lastTimes was nil")
	}
	return calculateAllBusy(lastTimes, cpuTimes)
}

type lastPercent struct {
	lastCPUTimes    []cpu.TimesStat
	lastPerCPUTimes []cpu.TimesStat
	sync.Mutex
}

var lastCPUPercent lastPercent

func calculateAllBusy(t1, t2 []cpu.TimesStat) ([]float64, error) {
	// Make sure the CPU measurements have the same length.
	if len(t1) != len(t2) {
		return nil, fmt.Errorf("received two CPU counts: %d != %d", len(t1), len(t2))
	}

	ret := make([]float64, len(t1))
	for i, t := range t2 {
		ret[i] = calculateBusy(t1[i], t)
	}
	return ret, nil
}

func getAllBusy(t cpu.TimesStat) (float64, float64) {
	busy := t.User + t.System + t.Nice + t.Iowait + t.Irq + t.Softirq + t.Steal
	return busy + t.Idle, busy
}

func calculateBusy(t1, t2 cpu.TimesStat) float64 {
	t1All, t1Busy := getAllBusy(t1)
	t2All, t2Busy := getAllBusy(t2)

	if t2Busy <= t1Busy {
		return 0
	}
	if t2All <= t1All {
		return 100
	}
	return math.Min(100, math.Max(0, (t2Busy-t1Busy)/(t2All-t1All)*100))
}
