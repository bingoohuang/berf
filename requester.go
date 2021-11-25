package perf

import (
	"context"
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
	concurrency int
	verbose     int
	requests    int64
	duration    time.Duration

	recordChan chan *ReportRecord

	closeOnce sync.Once
	wg        sync.WaitGroup

	readBytes  int64
	writeBytes int64

	cancel func()
	think  *thinktime.ThinkTime

	// Qps is the rate limit in queries per second.
	QPS float64

	ctx    context.Context
	fn     Fn
	config *Config
}

type ClientOpt struct {
	url       string
	method    string
	headers   []string
	bodyBytes []byte
	bodyFile  string

	certPath string
	keyPath  string

	insecure bool

	maxConns     int
	doTimeout    time.Duration
	readTimeout  time.Duration
	writeTimeout time.Duration
	dialTimeout  time.Duration
}

func NewRequester(concurrency, verbose int, requests int64, duration time.Duration, fn Fn, config *Config) (*Requester, error) {
	maxResult := concurrency * 100
	if maxResult > 8192 {
		maxResult = 8192
	}

	ctx, cancelFunc := context.WithCancel(context.Background())

	think, err := thinktime.ParseThinkTime(*thinkTime)
	ExitIfErr(err)

	r := &Requester{
		concurrency: concurrency,
		requests:    requests,
		duration:    duration,
		recordChan:  make(chan *ReportRecord, maxResult),
		verbose:     verbose,
		QPS:         *pQps,
		ctx:         ctx,
		cancel:      cancelFunc,
		fn:          fn,
		think:       think,
		config:      config,
	}

	return r, nil
}

func (r *Requester) closeRecord() {
	r.closeOnce.Do(func() {
		close(r.recordChan)
	})
}

func (r *Requester) doRequest(rr *ReportRecord) (err error) {
	var result *FnResult
	t1 := time.Now()
	result, err = r.fn(r.config)
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
		r.cancel()
	}()
	startTime = time.Now()
	if r.duration > 0 {
		time.AfterFunc(r.duration, func() {
			r.closeRecord()
			r.cancel()
		})
	}

	semaphore := r.requests
	for i := 0; i < r.concurrency; i++ {
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
				if r.requests > 0 && atomic.AddInt64(&semaphore, -1) < 0 {
					r.cancel()
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
