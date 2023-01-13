package main

import (
	"context"
	"crypto/aes"
	"crypto/cipher"
	"crypto/rand"
	"encoding/base64"
	"io"
	"time"

	"github.com/bingoohuang/berf"
	"github.com/bingoohuang/gg/pkg/chinaid"
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
	key   []byte
	nonce []byte
	plain []byte
}

func (b *bench) Name(context.Context, *berf.Config) string {
	return "profiles AES"
}

func (b *bench) Init(ctx context.Context, conf *berf.Config) (*berf.BenchOption, error) {
	b.key = randBytes(32)
	b.nonce = randBytes(12)
	plain := conf.Get("plain")
	if plain == "" {
		plain = chinaid.Address()
	}
	b.plain = []byte(plain)
	return &berf.BenchOption{}, nil
}

func (b *bench) Invoke(context.Context, *berf.Config) (*berf.Result, error) {
	start := time.Now()
	ciphertext := b.aesGcm(b.plain)
	base64Cipher := base64.StdEncoding.EncodeToString(ciphertext)
	end := time.Now()
	return &berf.Result{
		ReadBytes:  int64(len(b.plain)),
		WriteBytes: int64(len(base64Cipher)),
		Cost:       end.Sub(start),
	}, nil
}

func (b *bench) Final(context.Context, *berf.Config) error { return nil }

func (b *bench) aesGcm(plaintext []byte) (ciphertext []byte) {
	block, err := aes.NewCipher(b.key)
	if err != nil {
		panic(err.Error())
	}

	c, err := cipher.NewGCM(block)
	if err != nil {
		panic(err.Error())
	}

	return c.Seal(nil, b.nonce, plaintext, nil)
}

func randBytes(n int) []byte {
	key := make([]byte, n)
	if _, err := io.ReadFull(rand.Reader, key); err != nil {
		panic(err.Error())
	}
	return key
}
