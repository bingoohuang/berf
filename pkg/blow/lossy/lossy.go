// Package lossy simulates bandwidth, latency and packet loss for net.PacketConn and net.Conn interfaces.
// `Its main usage is to test robustness of applications and network protocols run over unreliable transport protocols such as UDP or IP.
// `As a side benefit, it can also be used as outbound bandwidth limiter.
// `
// `lossy only alters the writing side of the connection, reading side is kept as it is.
// `
// `from github.com/cevatbarisyilmaz/lossy
package lossy

import (
	"math/rand"
	"net"
	"sync"
	"time"
)

type conn struct {
	net.Conn
	minLatency        time.Duration
	maxLatency        time.Duration
	packetLossRate    float64
	writeDeadline     time.Time
	closed            bool
	mu                *sync.Mutex
	timeToWaitPerByte float64
}

// NewConn wraps the given net.Conn with a latency connection.
//
// bandwidth is in bytes/second.
// i.e. enter 1024 * 1024 for a 8 Mbit/s connection.
// Enter 0 or a negative value for an unlimited bandwidth.
//
// minLatency and maxLatency is used to create a random latency for each packet.
// maxLatency should be equal or greater than minLatency.
// If bandwidth is not unlimited and there's no other packets waiting to be delivered,
// time to deliver a packet is (len(packet) + headerOverhead) / bandwidth + randomDuration(minLatency, maxLatency)
func NewConn(c net.Conn, bandwidth uint64, minLatency, maxLatency time.Duration) net.Conn {
	return &conn{
		Conn:              c,
		minLatency:        minLatency,
		maxLatency:        maxLatency,
		writeDeadline:     time.Time{},
		closed:            false,
		mu:                &sync.Mutex{},
		timeToWaitPerByte: parseTimeToWaitPerByte(bandwidth),
	}
}

func parseTimeToWaitPerByte(bandwidth uint64) float64 {
	if bandwidth <= 0 {
		return 0
	}
	return float64(time.Second) / float64(bandwidth)
}

func (c *conn) Write(b []byte) (int, error) {
	c.mu.Lock()
	defer c.mu.Unlock()
	if c.closed || !c.writeDeadline.Equal(time.Time{}) && c.writeDeadline.Before(time.Now()) {
		return c.Conn.Write(b)
	}

	d := time.Duration(c.timeToWaitPerByte*(float64(len(b)))) + c.minLatency
	if c.minLatency < c.maxLatency {
		d += time.Duration(float64(c.maxLatency-c.minLatency) * randFloat64())
	}
	time.Sleep(d)
	_, _ = c.Conn.Write(b)
	return len(b), nil
}

func (c *conn) Close() error {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.closed = true
	return c.Conn.Close()
}

func (c *conn) SetDeadline(t time.Time) error {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.writeDeadline = t
	return c.Conn.SetDeadline(t)
}

func (c *conn) SetWriteDeadline(t time.Time) error {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.writeDeadline = t
	return c.Conn.SetWriteDeadline(t)
}

var rng = struct {
	sync.Mutex
	rand *rand.Rand
}{
	rand: rand.New(rand.NewSource(time.Now().UnixNano())),
}

func randFloat64() float64 {
	rng.Lock()
	defer rng.Unlock()

	return rng.rand.Float64()
}
