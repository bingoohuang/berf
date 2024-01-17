package blow

import (
	"context"
	"net"
	"os"
	"strings"
	_ "unsafe"

	"gitee.com/Trisia/gotlcp/tlcp"
	"github.com/bingoohuang/gg/pkg/osx"
	"github.com/bingoohuang/gg/pkg/osx/env"
	"github.com/emmansun/gmsm/smx509"
	"github.com/valyala/fasthttp"
)

func createTlcpDialer(
	ctx context.Context,
	dialFunc fasthttp.DialFunc,
	caFile string,
	hasPrintOption func(feature uint8) bool,
	tlsVerify bool,
) fasthttp.DialFunc {
	// 使用传输层密码协议(TLCP)，TLCP协议遵循《GB/T 38636-2020 信息安全技术 传输层密码协议》。
	c := &tlcp.Config{
		InsecureSkipVerify: !tlsVerify,
	}

	//  HTTP 客户端的会话缓存
	if envTLSSessionCache > 0 {
		c.SessionCache = tlcp.NewLRUSessionCache(envTLSSessionCache)
	}

	c.EnableDebug = hasPrintOption(printDebug)

	if caFile != "" {
		rootCert, err := smx509.ParseCertificatePEM(osx.ReadFile(caFile, osx.WithFatalOnError(true)).Data)
		if err != nil {
			panic(err)
		}
		pool := smx509.NewCertPool()
		pool.AddCert(rootCert)
		c.RootCAs = pool
	}

	if len(envTLCPCerts) > 0 {
		c.Certificates = envTLCPCerts

		switch len(envTLCPCerts) {
		case 2:
			// 单证书
			c.CipherSuites = []uint16{tlcp.ECC_SM4_GCM_SM3, tlcp.ECC_SM4_CBC_SM3}
		case 4:
			// ECDHE系列套件 同时需要 认证密钥对 与 加密密钥对,
			// 注意：不能出现 ECC 系列套件，否则服务端可能选择ECC系列套件。
			c.CipherSuites = []uint16{tlcp.ECDHE_SM4_CBC_SM3, tlcp.ECDHE_SM4_GCM_SM3}
		}
	}

	return func(addr string) (net.Conn, error) {
		return dial(ctx, dialFunc, addr, c)
	}
}

func dial(ctx context.Context, dialFunc fasthttp.DialFunc, addr string, config *tlcp.Config) (*tlcp.Conn, error) {
	rawConn, err := dialFunc(addr)
	if err != nil {
		return nil, err
	}

	if config == nil {
		config = &emptyConfig
	}

	conn := tlcp.Client(rawConn, config)
	if err := conn.HandshakeContext(ctx); err != nil {
		_ = rawConn.Close()
		return nil, err
	}

	return conn, nil
}

var (
	// emptyConfig 默认的空配置对象
	emptyConfig tlcp.Config

	envTLSSessionCache = env.Int(`TLS_SESSION_CACHE`, 32)

	envTLCPCerts = func() (certs []tlcp.Certificate) {
		env := os.Getenv(`TLCP_CERTS`)
		if env == "" {
			return nil
		}
		// TLCP 1.1，套件ECDHE-SM2-SM4-CBC-SM3，设置客户端双证书
		certsFiles := strings.Split(env, ",")
		switch len(certsFiles) {
		case 0, 2, 4:
		default:
			panic("env $TLCP_CERTS should be sign.cert.pem[,sign.key.pem,enc.cert.pem,enc.key.pem]")
		}
		if len(certsFiles) >= 2 {
			signCertKeypair, err := tlcp.X509KeyPair(osx.ReadFile(certsFiles[0], osx.WithFatalOnError(true)).Data,
				osx.ReadFile(certsFiles[1], osx.WithFatalOnError(true)).Data)
			if err != nil {
				panic(err)
			}
			certs = append(certs, signCertKeypair)
		}
		if len(certsFiles) >= 4 {
			encCertKeypair, err := tlcp.X509KeyPair(osx.ReadFile(certsFiles[2], osx.WithFatalOnError(true)).Data,
				osx.ReadFile(certsFiles[3], osx.WithFatalOnError(true)).Data)
			if err != nil {
				panic(err)
			}
			certs = append(certs, encCertKeypair)
		}

		return certs
	}()
)
