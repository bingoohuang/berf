package berf

import (
	"context"
	"errors"
	"io"
	"os"
	"os/signal"
	"sync"
	"sync/atomic"
	"syscall"
	"time"

	"github.com/bingoohuang/gg/pkg/osx"
	"github.com/bingoohuang/gg/pkg/ss"
	"github.com/bingoohuang/gg/pkg/thinktime"
)

// Requester is request structure.
type Requester struct {
	ctx context.Context

	benchable     Benchable
	ctxCancelFunc func()

	recordChan chan *ReportRecord

	thinkFn func(thinkNow bool) (thinkTime time.Duration)

	config *Config

	wg sync.WaitGroup

	duration time.Duration

	// QPS is the rate limit in queries per second.
	QPS float64

	verbose    int
	goroutines int
	n          int

	concurrent int64
}

func (c *Config) newRequester(ctx context.Context, fn Benchable) *Requester {
	maxResult := c.Goroutines * 100
	ctx, cancelFunc := context.WithCancel(ctx)
	return &Requester{
		goroutines:    c.Goroutines,
		n:             c.N,
		duration:      c.Duration,
		recordChan:    make(chan *ReportRecord, ss.Ifi(maxResult > 8192, 8192, maxResult)),
		verbose:       c.Verbose,
		QPS:           c.QPS,
		ctx:           ctx,
		ctxCancelFunc: cancelFunc,
		benchable:     fn,
		thinkFn:       c.createThinkFn(),
		config:        c,
	}
}

func (c *Config) createThinkFn() func(thinkNow bool) (thinkTime time.Duration) {
	think, err := thinktime.ParseThinkTime(c.ThinkTime)
	osx.ExitIfErr(err)
	if think != nil {
		return think.Think
	}

	return func(thinkNow bool) (thinkTime time.Duration) { return 0 }
}

func (r *Requester) doRequest(ctx context.Context, rr *ReportRecord) (err error) {
	var result *Result
	t1 := time.Now()
	result, err = r.benchable.Invoke(ctx, r.config)
	if err != nil {
		return err
	}

	if result.Cost > 0 {
		rr.cost = result.Cost
	} else {
		rr.cost = time.Since(t1)
	}

	rr.code = result.Status
	if r.verbose >= 1 {
		rr.counting = result.Counting
	}

	rr.readBytes = result.ReadBytes
	rr.writeBytes = result.WriteBytes

	return nil
}

func (r *Requester) run() {
	sigs := make(chan os.Signal, 1)
	signal.Notify(sigs, os.Interrupt, syscall.SIGTERM) // handle ctrl-c

	go func() {
		for i := 0; ; i++ {
			<-sigs
			if i == 0 {
				r.ctxCancelFunc()
			} else {
				os.Exit(-1)
			}
		}
	}()

	startTime = time.Now()
	if r.duration > 0 {
		time.AfterFunc(r.duration, r.ctxCancelFunc)
	}

	throttle := func() bool { return r.ctx.Err() == nil }
	if r.QPS > 0 {
		t := time.NewTicker(time.Duration(1e6/(r.QPS)) * time.Microsecond)
		defer t.Stop()
		throttle = func() bool {
			select {
			case <-t.C:
				return true
			case <-r.ctx.Done():
				return false
			}
		}
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

	r.ctxCancelFunc()
	close(r.recordChan)
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
		for i := max - 1; i >= 0; {
			<-t.C
			for j := i; i > j-down && i >= 0; i-- {
				cancels[i]()
			}
		}

		keepTimes(t.C, 1)
	}
}

func keepTimes(c <-chan time.Time, times int) {
	for i := 0; i < times; i++ {
		<-c
	}
}

func (r *Requester) loopWork(ctx context.Context, semaphore *int64, throttle func() bool) {
	atomic.AddInt64(&r.concurrent, 1)
	defer func() {
		r.wg.Done()
		atomic.AddInt64(&r.concurrent, -1)
	}()

	for {
		if r.n > 0 && atomic.AddInt64(semaphore, -1) < 0 {
			return
		}
		if !throttle() {
			return
		}

		rr := recordPool.Get().(*ReportRecord)
		rr.Reset()
		if err := r.runOne(ctx, rr); errors.Is(err, io.EOF) {
			return
		}

		r.recordChan <- rr
		r.thinkFn(true)
	}
}

func (r *Requester) runOne(ctx context.Context, rr *ReportRecord) error {
	err := r.doRequest(ctx, rr)
	if err != nil && !errors.Is(err, io.EOF) {
		rr.error = err.Error()
	}

	return err
}
