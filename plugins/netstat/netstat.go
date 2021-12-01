package net

import (
	"fmt"
	"syscall"

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
	netconns, err := ns.ps.NetConnections()
	if err != nil {
		return nil, fmt.Errorf("error getting net connections info: %s", err)
	}
	c := make(map[string]int)
	c["UDP"] = 0

	for _, netcon := range netconns {
		if netcon.Type == syscall.SOCK_DGRAM {
			c["UDP"]++
			continue // UDP has no status
		}
		if x, ok := c[netcon.Status]; !ok {
			c[netcon.Status] = 0
		} else {
			c[netcon.Status] = x + 1
		}
	}

	return []interface{}{c["ESTABLISHED"], c["TIME_WAIT"], c["CLOSE_WAIT"], c["LISTEN"], c["CLOSING"], c["UDP"]}, nil
}

func init() {
	plugins.Add("netstat", func() plugins.Input {
		return &NetStats{ps: system.NewSystemPS()}
	})
}
