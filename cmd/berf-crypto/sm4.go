package main

import (
	"encoding/base64"
	"fmt"
	"time"

	"github.com/bingoohuang/berf"
	"github.com/bingoohuang/gg/pkg/codec/b64"
	"github.com/deatil/go-cryptobin/cryptobin/crypto"
)

type sm4Bench struct {
	plain []byte
	key   []byte
	iv    []byte
}

func (s *sm4Bench) Init(conf *berf.Config) {
	plainSize := conf.GetInt("size", 64)
	s.plain = getConf(conf, "plain", plainSize)
	s.key = getConf(conf, "key", 16)
	s.iv = getConf(conf, "iv", 16)

	if conf.N >= 1 && conf.N <= 10 {
		fmt.Printf("Plain : %s\n", base64.StdEncoding.EncodeToString(s.plain))
		fmt.Printf("Key   : %s\n", base64.StdEncoding.EncodeToString(s.key))
		fmt.Printf("IV    : %s\n", base64.StdEncoding.EncodeToString(s.iv))
	}
}

func getConf(conf *berf.Config, key string, size int) []byte {
	val := conf.Get(key)
	if val == "" {
		return randBytes(size)
	}

	return b64.DecodeString2Bytes(val)
}

func (s *sm4Bench) Invoke(conf *berf.Config) (*berf.Result, error) {
	start := time.Now()

	result := crypto.FromBytes(s.plain).
		SetKey(string(s.key)).
		SetIv(string(s.iv)).
		SM4().CBC().PKCS7Padding().
		Encrypt()
	end := time.Now()
	data := result.ToBytes()

	if len(result.Errors) > 0 {
		return nil, result.Error()
	}

	if conf.N >= 1 && conf.N < 10 {
		fmt.Printf("Result: %s\n", base64.StdEncoding.EncodeToString(data))
	}

	return &berf.Result{
		ReadBytes:  int64(len(s.plain)),
		WriteBytes: int64(len(data)),
		Cost:       end.Sub(start),
	}, nil
}
