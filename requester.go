package perf

import (
	"context"
	"os"
	"os/signal"
	"sync"
	"sync/atomic"
	"syscall"
	"time"

	"github.com/bingoohuang/perf/pkg/util"

	"github.com/bingoohuang/gg/pkg/ss"

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
	goroutines int
	n          int
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

	concurrent int64
}

func (c *Config) NewRequester(fn F) (*Requester, error) {
	maxResult := c.Goroutines * 100
	think, err := thinktime.ParseThinkTime(c.ThinkTime)
	util.ExitIfErr(err)

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

func (r *Requester) doRequest(ctx context.Context, rr *ReportRecord) (err error) {
	var result *Result
	t1 := time.Now()
	result, err = r.fn(ctx, r.config)
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

	throttle := func() {}
	if r.QPS > 0 {
		t := time.NewTicker(time.Duration(1e6/(r.QPS)) * time.Microsecond)
		defer t.Stop()
		throttle = func() { <-t.C }
	}

	semaphore := int64(r.n)

	if r.config.Incr.IsEmpty() {
		for i := 0; i < r.goroutines; i++ {
			r.wg.Add(1)
			go r.loopWork(r.ctx, &semaphore, throttle)
		}
	} else {
		ch := make(chan context.Context)
		go r.generateTokens(ch)

		for ctx := range ch {
			r.wg.Add(1)
			go r.loopWork(ctx, &semaphore, throttle)
		}
	}

	r.wg.Wait()
	r.closeRecord()
}

func (r *Requester) generateTokens(ch chan context.Context) {
	defer close(ch)

	dur := r.config.Incr.Dur
	if dur <= 0 {
		dur = time.Minute
	}

	max := r.config.Goroutines
	cancels := make([]context.CancelFunc, max)
	var ctx context.Context

	t := time.NewTicker(dur)
	defer t.Stop()

	if up := r.config.Incr.Up; up <= 0 {
		for i := 0; i < max; i++ {
			ctx, cancels[i] = context.WithCancel(r.ctx)
			ch <- ctx
		}
	} else {
		for i := 0; i < max; i += up {
			for j := i; j < i+up && j < max; j++ {
				ctx, cancels[j] = context.WithCancel(r.ctx)
				ch <- ctx
			}
			<-t.C
		}
	}

	keepTimes(t.C, 3)

	if down := r.config.Incr.Down; down > 0 {
		for i := max - 1; i >= 0; i-- {
			<-t.C
			for j := i; j > i-down && j >= 0; j-- {
				cancels[j]()
			}
		}

		keepTimes(t.C, 3)
	}
}

func keepTimes(c <-chan time.Time, times int) {
	for i := 0; i < times; i++ {
		<-c
	}
}

func (r *Requester) loopWork(ctx context.Context, semaphore *int64, throttle func()) {
	atomic.AddInt64(&r.concurrent, 1)
	defer func() {
		r.wg.Done()
		atomic.AddInt64(&r.concurrent, -1)
		if v := recover(); v != nil && v != sendOnCloseError {
			panic(v)
		}
	}()

	for ctx.Err() == nil {
		if r.n > 0 && atomic.AddInt64(semaphore, -1) < 0 {
			r.cancelFunc()
			return
		}

		throttle()

		rr := recordPool.Get().(*ReportRecord)
		rr.Reset()
		r.runOne(ctx, rr)
		r.recordChan <- rr
		r.thinking()
	}
}

func (r *Requester) runOne(ctx context.Context, rr *ReportRecord) *ReportRecord {
	err := r.doRequest(ctx, rr)
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
