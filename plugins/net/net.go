package net

import (
	"fmt"
	"net"
	"os"
	"sort"
	"time"

	"github.com/bingoohuang/berf/pkg/filter"
	"github.com/bingoohuang/berf/pkg/util"
	"github.com/bingoohuang/berf/plugins"
	"github.com/bingoohuang/berf/plugins/internal"
	"github.com/bingoohuang/berf/plugins/system"
	"github.com/bingoohuang/gg/pkg/ss"
)

type NetIOStats struct {
	Interfaces []string

	filter filter.Filter
	ps     system.PS

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
		if iface.Flags&net.FlagLoopback == net.FlagLoopback || iface.Flags&net.FlagUp != net.FlagUp {
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
		p := v + ":"
		s.Series = append(s.Series, p+"Tx", p+"Rx", p+"TxP", p+"RxP")
		s.Selected = append(s.Selected, p+"Tx", p+"Rx")
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

/*
root@bjca-PC:~/bingoohuang# ifconfig -a
enp125s0f0: flags=4163<UP,BROADCAST,RUNNING,MULTICAST>  mtu 1500
        inet 192.168.126.41  netmask 255.255.255.0  broadcast 192.168.126.255
        inet6 fe80::2987:1d0e:899f:e026  prefixlen 64  scopeid 0x20<link>
        ether 44:67:47:97:2b:6f  txqueuelen 1000  (Ethernet)
        RX packets 11706357  bytes 3039493482 (2.8 GiB)
        RX errors 0  dropped 17491  overruns 0  frame 0
        TX packets 5977242  bytes 2373840921 (2.2 GiB)
        TX errors 0  dropped 0 overruns 0  carrier 0  collisions 0

enp125s0f1: flags=4099<UP,BROADCAST,MULTICAST>  mtu 1500
        ether 44:67:47:97:2b:70  txqueuelen 1000  (Ethernet)
        RX packets 0  bytes 0 (0.0 B)
        RX errors 0  dropped 0  overruns 0  frame 0
        TX packets 0  bytes 0 (0.0 B)
        TX errors 0  dropped 0 overruns 0  carrier 0  collisions 0

lo: flags=73<UP,LOOPBACK,RUNNING>  mtu 65536
        inet 127.0.0.1  netmask 255.0.0.0
        inet6 ::1  prefixlen 128  scopeid 0x10<host>
        loop  txqueuelen 1000  (Local Loopback)
        RX packets 465500033  bytes 100560591774 (93.6 GiB)
        RX errors 0  dropped 0  overruns 0  frame 0
        TX packets 465500033  bytes 100560591774 (93.6 GiB)
        TX errors 0  dropped 0 overruns 0  carrier 0  collisions 0
*/

func init() {
	plugins.Add("net", func() plugins.Input {
		ifaces := []string{"eth*", "en*"}
		if s := os.Getenv("BERF_NET"); s != "" {
			ifaces = ss.Split(s)
		}
		return &NetIOStats{
			Interfaces: ifaces,
			ps:         system.NewSystemPS(),
		}
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
		points = append(points, util.BytesToBPS(nsDiff.txBytes, d), util.BytesToBPS(nsDiff.rxBytes, d),
			util.NumberToRate(nsDiff.txPkts, d), util.NumberToRate(nsDiff.rxPkts, d))
	}

	return points
}

func getNetDevStatDiff(cur netDevStat, pre netStat) netDevStat {
	for _, p := range pre.netDevStats {
		if p.interfaceName == cur.interfaceName {
			cur.rxBytes = internal.Adjust(cur.rxBytes, p.rxBytes)
			cur.txBytes = internal.Adjust(cur.txBytes, p.txBytes)
			cur.rxPkts = internal.Adjust(cur.rxPkts, p.rxPkts)
			cur.txPkts = internal.Adjust(cur.txPkts, p.txPkts)
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
