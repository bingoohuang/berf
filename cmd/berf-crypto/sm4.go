package main

import (
	"encoding/base64"
	"fmt"
	"strings"
	"time"

	"github.com/bingoohuang/berf"
	"github.com/bingoohuang/gg/pkg/codec/b64"
	"github.com/deatil/go-cryptobin/cryptobin/crypto"
)

type sm4Bench struct {
	input  []byte
	key    []byte
	iv     []byte
	decode bool
}

func (s *sm4Bench) Init(conf *berf.Config) {
	inputSize := conf.GetInt("size", 64)
	var inputType, keyType, ivType ValueType
	s.input, inputType = getConf(conf, "input", inputSize)
	s.key, keyType = getConf(conf, "key", 16)
	s.iv, ivType = getConf(conf, "iv", 16)
	s.decode = conf.Has("decode")

	if conf.N >= 1 && conf.N <= 10 {
		fmt.Printf("Input : %s\n", inputType.String(s.input))
		fmt.Printf("Key   : %s\n", keyType.String(s.key))
		fmt.Printf("IV    : %s\n", ivType.String(s.iv))
	}
}

type ValueType int

func (t ValueType) String(value []byte) string {
	switch t {
	case TypeRawManual:
		return string(value)
	default:
		return base64.RawURLEncoding.EncodeToString(value) + " (base64.RawURLEncoding)"
	}
}

const (
	TypeBase64 ValueType = iota
	TypeRawManual
	TypeRawRandom
)

func getConf(conf *berf.Config, key string, size int) ([]byte, ValueType) {
	val := conf.Get(key)
	if val == "" {
		return randBytes(size), TypeRawRandom
	}

	const prefixRaw = "raw:"
	if strings.HasPrefix(val, prefixRaw) {
		return []byte(val[len(prefixRaw):]), TypeRawManual
	}

	const prefixBase64 = "base64:"
	if strings.HasPrefix(val, prefixBase64) {
		return b64.DecodeString2Bytes(val[len(prefixBase64):]), TypeBase64
	}

	data, err := b64.DecodeBytes([]byte(val))
	if err == nil {
		return data, TypeBase64
	}

	return []byte(val), TypeRawManual
}

func (s *sm4Bench) Invoke(conf *berf.Config) (*berf.Result, error) {
	start := time.Now()

	result := crypto.FromBytes(s.input).
		SetKey(string(s.key)).
		SetIv(string(s.iv)).
		SM4().CBC().PKCS7Padding()
	if s.decode {
		result = result.Decrypt()
	} else {
		result = result.Encrypt()
	}
	end := time.Now()
	data := result.ToBytes()

	if len(result.Errors) > 0 {
		return nil, result.Error()
	}

	if conf.N >= 1 && conf.N < 10 {
		if s.decode {
			fmt.Printf("Result: %s\n", data)
		}

		fmt.Printf("Base64RawURL: %s\n", base64.RawURLEncoding.EncodeToString(data))
	}

	return &berf.Result{
		ReadBytes:  int64(len(s.input)),
		WriteBytes: int64(len(data)),
		Cost:       end.Sub(start),
	}, nil
}
