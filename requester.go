package perf

import (
	"context"
	"github.com/bingoohuang/gg/pkg/ss"
	"os"
	"os/signal"
	"sync"
	"sync/atomic"
	"syscall"
	"time"

	"github.com/bingoohuang/gg/pkg/thinktime"
)

var sendOnCloseError interface{}

func init() {
	defer func() {
		sendOnCloseError = recover()
	}()
	func() {
		cc := make(chan struct{}, 1)
		close(cc)
		cc <- struct{}{}
	}()
}

type Requester struct {
	goroutines int64
	n          int64
	verbose    int
	duration   time.Duration

	recordChan chan *ReportRecord
	closeOnce  sync.Once
	wg         sync.WaitGroup

	readBytes  int64
	writeBytes int64

	cancelFunc func()
	think      *thinktime.ThinkTime

	// Qps is the rate limit in queries per second.
	QPS float64

	ctx    context.Context
	fn     F
	config *Config
}

func (c *Config) NewRequester(fn F) (*Requester, error) {
	maxResult := c.Goroutines * 100
	think, err := thinktime.ParseThinkTime(c.ThinkTime)
	ExitIfErr(err)

	ctx, cancelFunc := context.WithCancel(context.Background())
	r := &Requester{
		goroutines: c.Goroutines,
		n:          c.N,
		duration:   c.Duration,
		recordChan: make(chan *ReportRecord, ss.Ifi(maxResult > 8192, 8192, int(maxResult))),
		verbose:    c.Verbose,
		QPS:        c.Qps,
		ctx:        ctx,
		cancelFunc: cancelFunc,
		fn:         fn,
		think:      think,
		config:     c,
	}

	return r, nil
}

func (r *Requester) closeRecord() {
	r.closeOnce.Do(func() {
		close(r.recordChan)
	})
}

func (r *Requester) doRequest(rr *ReportRecord) (err error) {
	var result *Result
	t1 := time.Now()
	result, err = r.fn(r.ctx, r.config)
	rr.cost = time.Since(t1)
	if err != nil {
		return err
	}

	rr.code = []string{result.Status}
	if r.verbose >= 1 {
		rr.counting = []string{result.Counting}
	}

	return nil
}

func (r *Requester) Run() {
	// handle ctrl-c
	sigs := make(chan os.Signal, 1)
	signal.Notify(sigs, os.Interrupt, syscall.SIGTERM)
	defer signal.Stop(sigs)

	go func() {
		<-sigs
		r.closeRecord()
		r.cancelFunc()
	}()
	startTime = time.Now()
	if r.duration > 0 {
		time.AfterFunc(r.duration, func() {
			r.closeRecord()
			r.cancelFunc()
		})
	}

	semaphore := r.n
	for i := int64(0); i < r.goroutines; i++ {
		r.wg.Add(1)
		go func() {
			defer func() {
				r.wg.Done()
				v := recover()
				if v != nil && v != sendOnCloseError {
					panic(v)
				}
			}()

			throttle := func() {}
			if r.QPS > 0 {
				t := time.Tick(time.Duration(1e6/(r.QPS)) * time.Microsecond)
				throttle = func() { <-t }
			}

			for r.ctx.Err() == nil {
				if r.n > 0 && atomic.AddInt64(&semaphore, -1) < 0 {
					r.cancelFunc()
					return
				}

				throttle()

				rr := recordPool.Get().(*ReportRecord)
				rr.Reset()
				r.runOne(rr)
				r.recordChan <- rr
				r.thinking()
			}
		}()
	}

	r.wg.Wait()
	r.closeRecord()
}

func (r *Requester) runOne(rr *ReportRecord) *ReportRecord {
	err := r.doRequest(rr)
	if err != nil {
		rr.error = err.Error()
	}

	rr.readBytes = atomic.LoadInt64(&r.readBytes)
	rr.writeBytes = atomic.LoadInt64(&r.writeBytes)
	return rr
}

func (r *Requester) thinking() {
	if r.think != nil {
		r.think.Think(true)
	}
}
