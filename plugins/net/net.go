package net

import (
	"fmt"
	"github.com/bingoohuang/perf/plugins/internal"
	"net"
	"sort"
	"time"

	"github.com/bingoohuang/perf/pkg/filter"
	"github.com/bingoohuang/perf/plugins"
	"github.com/bingoohuang/perf/plugins/system"
)

type NetIOStats struct {
	filter filter.Filter
	ps     system.PS

	Interfaces []string

	interfacesByName map[string]bool
	interfacesName   []string

	gPrevNetStats netStat
	gCurNetStats  netStat

	lastStatsTime time.Time
}

func (n *NetIOStats) Init() (err error) {
	if n.filter, err = filter.Compile(n.Interfaces); err != nil {
		return fmt.Errorf("error compiling filter: %s", err)
	}

	interfaces, err := net.Interfaces()
	if err != nil {
		return fmt.Errorf("error getting list of interfaces: %s", err)
	}
	n.interfacesByName = map[string]bool{}
	for _, iface := range interfaces {
		if iface.Flags&net.FlagLoopback == net.FlagLoopback || iface.Flags&net.FlagUp == 0 {
			continue
		}

		if n.filter.Match(iface.Name) {
			n.interfacesByName[iface.Name] = true
			n.interfacesName = append(n.interfacesName, iface.Name)
		}
	}

	sort.Strings(n.interfacesName)
	return nil
}

func (n *NetIOStats) Series() plugins.Series {
	s := plugins.Series{}
	for _, v := range n.interfacesName {
		s.Series = append(s.Series, v+"_Tx", v+"_Rx", v+"_TxP", v+"_RxP")
		s.Selected = append(s.Selected, v+"_Tx", v+"_Rx")
	}

	return s
}

func (n *NetIOStats) Gather() ([]interface{}, error) {
	netio, err := n.ps.NetIO()
	if err != nil {
		return nil, fmt.Errorf("error getting net io info: %s", err)
	}

	stat := netStat{}

	for _, io := range netio {
		if !n.interfacesByName[io.Name] {
			continue
		}

		stat.netDevStats = append(stat.netDevStats,
			netDevStat{
				interfaceName: io.Name,
				rxBytes:       io.BytesRecv,
				txBytes:       io.BytesSent,
				rxPkts:        io.PacketsRecv,
				txPkts:        io.PacketsSent,
			})
	}

	sort.Slice(stat.netDevStats, func(i, j int) bool {
		return stat.netDevStats[i].interfaceName < stat.netDevStats[j].interfaceName
	})

	return n.diff(stat), nil
}

func init() {
	plugins.Add("net", func() plugins.Input {
		return &NetIOStats{ps: system.NewSystemPS(),
			Interfaces: []string{"eth*", "en0"}}
	})
}

func (n *NetIOStats) diff(stat netStat) []interface{} {
	n.gPrevNetStats = n.gCurNetStats
	n.gCurNetStats = stat

	d := time.Since(n.lastStatsTime)
	n.lastStatsTime = time.Now()

	var points []interface{}
	for _, ns := range n.gCurNetStats.netDevStats {
		nsDiff := getNetDevStatDiff(ns, n.gPrevNetStats)
		points = append(points, internal.BytesToBPS(nsDiff.txBytes, d), internal.BytesToBPS(nsDiff.rxBytes, d),
			internal.NumberToRate(nsDiff.txPkts, d), internal.NumberToRate(nsDiff.rxPkts, d))
	}

	return points
}

func adjust(cur, prev uint64) uint64 {
	if cur >= prev {
		return cur - prev
	}

	return cur + ^uint64(0) - prev
}

func getNetDevStatDiff(cur netDevStat, pre netStat) netDevStat {
	for _, p := range pre.netDevStats {
		if p.interfaceName == cur.interfaceName {
			cur.rxBytes = adjust(cur.rxBytes, p.rxBytes)
			cur.txBytes = adjust(cur.txBytes, p.txBytes)
			cur.rxPkts = adjust(cur.rxPkts, p.rxPkts)
			cur.txPkts = adjust(cur.txPkts, p.txPkts)
			break
		}
	}
	return cur
}

type netStat struct {
	netDevStats []netDevStat
}

type netDevStat struct {
	interfaceName string
	rxBytes       uint64
	txBytes       uint64
	rxPkts        uint64
	txPkts        uint64
}
