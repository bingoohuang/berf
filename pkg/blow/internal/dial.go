package internal

import (
	"net"
	"strings"
	"sync/atomic"
	"time"

	"github.com/bingoohuang/berf/pkg/blow/lossy"
	"github.com/dustin/go-humanize"
	"github.com/valyala/fasthttp"
)

type MyConn struct {
	net.Conn
	r, w *int64
}

func NewMyConn(conn net.Conn, r, w *int64) (*MyConn, error) {
	return &MyConn{Conn: conn, r: r, w: w}, nil
}

func (c *MyConn) Read(b []byte) (n int, err error) {
	if n, err = c.Conn.Read(b); n > 0 {
		atomic.AddInt64(c.r, int64(n))
	}
	return
}

func (c *MyConn) Write(b []byte) (n int, err error) {
	if n, err = c.Conn.Write(b); n > 0 {
		atomic.AddInt64(c.w, int64(n))
	}
	return
}

type networkWrapper func(net.Conn) net.Conn

func ThroughputStatDial(wrap networkWrapper, dial fasthttp.DialFunc, r *int64, w *int64) fasthttp.DialFunc {
	return func(addr string) (net.Conn, error) {
		conn, err := dial(addr)
		if err != nil {
			return nil, err
		}

		return NewMyConn(wrap(conn), r, w)
	}
}

func NetworkWrap(network string) networkWrapper {
	var bandwidth uint64
	var latency time.Duration
	noop := func(conn net.Conn) net.Conn { return conn }
	switch strings.ToLower(network) {
	case "", "local":
	case "lan": // 100M
		bandwidth = 100 * 1024 * 1024
		latency = 2 * time.Millisecond
	case "wan": // 20M
		bandwidth = 20 * 1024 * 1024
		latency = 30 * time.Millisecond
	case "bad":
		bandwidth = 20 * 1024 * 1024
		latency = 200 * time.Millisecond
	default:
		parts := strings.SplitN(network, ":", -1)
		if len(parts) >= 1 {
			bandwidth, _ = humanize.ParseBytes(parts[0])
		}
		if len(parts) >= 2 {
			latency, _ = time.ParseDuration(parts[1])
		}
	}

	if bandwidth == 0 && latency == 0 {
		return noop
	}

	return func(conn net.Conn) net.Conn {
		return lossy.NewConn(conn, bandwidth, latency, latency)
	}
}
