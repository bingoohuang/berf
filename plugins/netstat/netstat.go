package net

import (
	"fmt"
	"syscall"

	"github.com/shirou/gopsutil/v3/net"

	"github.com/bingoohuang/berf/plugins"
	"github.com/bingoohuang/berf/plugins/system"
)

type NetStats struct {
	ps system.PS
}

func (ns *NetStats) Series() plugins.Series {
	return plugins.Series{
		Series:   []string{"established", "time_wait", "close_wait", "listen", "closing", "udp"},
		Selected: []string{"established", "time_wait", "close_wait"},
	}
}

func (ns *NetStats) Gather() ([]interface{}, error) {
	c := make(map[string]int)
	c["UDP"] = 0

	walker := func(netcon net.ConnectionStat) error {
		if netcon.Type == syscall.SOCK_DGRAM {
			c["UDP"]++
			return nil // UDP has no status
		}
		if x, ok := c[netcon.Status]; !ok {
			c[netcon.Status] = 0
		} else {
			c[netcon.Status] = x + 1
		}
		return nil
	}

	netconns, err := ns.ps.NetConnections(net.WithWalker(walker), net.WithKind("all"))
	if err != nil {
		return nil, fmt.Errorf("error getting net connections info: %s", err)
	}

	for _, netcon := range netconns {
		walker(netcon)
	}

	return []interface{}{c["ESTABLISHED"], c["TIME_WAIT"], c["CLOSE_WAIT"], c["LISTEN"], c["CLOSING"], c["UDP"]}, nil
}

func init() {
	plugins.Add("netstat", func() plugins.Input {
		return &NetStats{ps: system.NewSystemPS()}
	})
}
