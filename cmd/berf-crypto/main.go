package main

import (
	"context"
	"crypto/rand"
	"io"
	"strings"

	"github.com/bingoohuang/berf"
	"github.com/bingoohuang/gg/pkg/ctl"
	"github.com/bingoohuang/gg/pkg/fla9"
	"github.com/bingoohuang/gg/pkg/sigx"
)

var (
	pVersion = fla9.Bool("version", false, "Show version and exit")
	pInit    = fla9.Bool("init", false, "Create initial ctl and exit")
)

func init() {
	fla9.Parse()
	ctl.Config{Initing: *pInit, PrintVersion: *pVersion}.ProcessInit()
}

func main() {
	sigx.RegisterSignalProfile()
	berf.StartBench(context.Background(), &bench{})
}

type bench struct {
	benchable
	alg string
}

func (b *bench) Name(_ context.Context, conf *berf.Config) string {
	return "profiles " + strings.ToUpper(b.alg)
}

type benchable interface {
	Init(*berf.Config)
	Invoke(conf *berf.Config) (*berf.Result, error)
}

func (b *bench) Init(_ context.Context, conf *berf.Config) (*berf.BenchOption, error) {
	b.alg = conf.GetOr("alg", "aes")

	switch b.alg {
	case "sm4":
		b.benchable = &sm4Bench{}
	default:
		b.benchable = &aesBench{}
	}

	b.benchable.Init(conf)
	return &berf.BenchOption{}, nil
}

func (b *bench) Invoke(_ context.Context, conf *berf.Config) (*berf.Result, error) {
	return b.benchable.Invoke(conf)
}

func (b *bench) Final(context.Context, *berf.Config) error { return nil }

func randBytes(n int) []byte {
	key := make([]byte, n)
	if _, err := io.ReadFull(rand.Reader, key); err != nil {
		panic(err.Error())
	}
	return key
}
