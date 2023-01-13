package main

import (
	"crypto/aes"
	"crypto/cipher"
	"encoding/base64"
	"time"

	"github.com/bingoohuang/berf"
	"github.com/bingoohuang/gg/pkg/chinaid"
)

type aesBench struct {
	key   []byte
	nonce []byte
	plain []byte
}

func (b *aesBench) Init(conf *berf.Config) {
	b.key = randBytes(32)
	b.nonce = randBytes(12)
	plain := conf.Get("plain")
	if plain == "" {
		plain = chinaid.Address()
	}
	b.plain = []byte(plain)
}

func (b *aesBench) Invoke(*berf.Config) (*berf.Result, error) {
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

func (b *aesBench) aesGcm(plaintext []byte) (ciphertext []byte) {
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
