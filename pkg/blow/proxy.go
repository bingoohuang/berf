package blow

import (
	"bufio"
	"crypto/tls"
	"crypto/x509"
	"encoding/base64"
	"fmt"
	"log"
	"net"
	"net/url"
	"os"
	"sync/atomic"
	"time"

	"gitee.com/Trisia/gotlcp/tlcp"
	"github.com/bingoohuang/gg/pkg/osx/env"
	"github.com/bingoohuang/gg/pkg/rest"
	"github.com/bingoohuang/gg/pkg/ss"
	"github.com/valyala/fasthttp"
	"golang.org/x/net/http/httpproxy"
)

type Dialer interface {
	Dial(addr string) (net.Conn, error)
	DialTimeout(addr string, timeout time.Duration) (net.Conn, error)
}

var Debug = env.Bool("DEBUG", false)

var (
	localIpIndex atomic.Uint64
	localIps     = func() (addrs []*net.TCPAddr) {
		ips := ss.Split(os.Getenv("LOCAL_IP"),
			ss.WithSeps(","), ss.WithTrimSpace(true), ss.WithIgnoreEmpty(true))
		for _, ip := range ips {
			ipAddr, err := net.ResolveIPAddr("ip", ip)
			if err != nil {
				log.Panicf("resolving IP %s: %v", ip, err)
			}
			addrs = append(addrs, &net.TCPAddr{IP: ipAddr.IP})
		}
		if Debug {
			log.Printf("LOCAL_IP resovled: %v", addrs)
		}

		return addrs
	}()
)

func getLocalAddr() *net.TCPAddr {
	localIpsLen := len(localIps)
	if localIpsLen == 0 {
		return nil
	}

	idx := int(localIpIndex.Add(1)-1) % localIpsLen
	return localIps[idx]
}

var dialer = func() Dialer {
	d := &fasthttp.TCPDialer{
		Concurrency:   1000,
		LocalAddrFunc: getLocalAddr,
	}

	return d
}()

const (
	httpsScheme = "https"
	httpScheme  = "http"
)

func getEnvAny(names ...string) string {
	for _, n := range names {
		if val := os.Getenv(n); val != "" {
			return val
		}
	}
	return ""
}

var proxyFunc = func() func(host string, tls bool) (*url.URL, error) {
	proxy := getEnvAny("PROXY", "proxy")
	proxyFunc := httpproxy.FromEnvironment().ProxyFunc()
	if proxy == "" {
		return func(host string, tls bool) (*url.URL, error) {
			reqURL := &url.URL{Host: host, Scheme: httpScheme}
			if tls {
				reqURL.Scheme = httpsScheme
			}
			return proxyFunc(reqURL)
		}
	}
	if proxy == "off" || proxy == "0" {
		return func(addr string, tls bool) (*url.URL, error) {
			return nil, nil
		}
	}
	return func(addr string, tls bool) (*url.URL, error) {
		return rest.FixURI(proxy, rest.WithFatalErr(true)).Data, nil
	}
}()

// ProxyHTTPDialerTimeout returns a fasthttp.DialFunc that dials using
// code original from fasthttpproxy.ProxyHTTPDialerTimeout
// the env(HTTP_PROXY, HTTPS_PROXY and NO_PROXY) configured HTTP proxy using the given timeout.
//
// Example usage:
//
//	c := &fasthttp.Client{
//		Dial: ProxyHTTPDialerTimeout(time.Second * 2),
//	}
func ProxyHTTPDialerTimeout(timeout time.Duration, dialer Dialer, tls bool) fasthttp.DialFunc {
	return func(addr string) (net.Conn, error) {
		proxyURL, err := proxyFunc(addr, tls)
		if err != nil {
			return nil, err
		}

		if proxyURL == nil {
			if timeout == 0 {
				return dialer.Dial(addr)
			}
			return dialer.DialTimeout(addr, timeout)
		}

		var conn net.Conn
		if timeout == 0 {
			conn, err = dialer.Dial(proxyURL.Host)
		} else {
			conn, err = dialer.DialTimeout(proxyURL.Host, timeout)
		}
		if err != nil {
			return nil, err
		}

		if !tls {
			return conn, nil
		}

		req := "CONNECT " + addr + " HTTP/1.1\r\n"

		if proxyURL.User != nil {
			authBarrier := base64.StdEncoding.EncodeToString([]byte(proxyURL.User.String()))
			req += "Proxy-Authorization: Basic " + authBarrier + "\r\n"
		}
		req += "\r\n"

		if _, err := conn.Write([]byte(req)); err != nil {
			return nil, err
		}

		res := fasthttp.AcquireResponse()
		defer fasthttp.ReleaseResponse(res)

		res.SkipBody = true

		if err := res.Read(bufio.NewReader(conn)); err != nil {
			if connErr := conn.Close(); connErr != nil {
				return nil, fmt.Errorf("conn close err %v precede by read conn err %w", connErr, err)
			}
			return nil, err
		}
		if res.Header.StatusCode() != 200 {
			if connErr := conn.Close(); connErr != nil {
				return nil, fmt.Errorf(
					"conn close err %w precede by connect to proxy: code: %d body %q",
					connErr, res.StatusCode(), string(res.Body()))
			}
			return nil, fmt.Errorf("could not connect to proxy: code: %d body %q", res.StatusCode(), string(res.Body()))
		}

		return conn, nil
	}
}

type tlcpConnectionStater interface {
	ConnectionState() tlcp.ConnectionState
}
type tlsConnectionStater interface {
	ConnectionState() tls.ConnectionState
}

func printConnectState(conn net.Conn) {
	if cs, ok := conn.(tlsConnectionStater); ok {
		printTLSConnectState(conn, cs.ConnectionState())
	} else if cs, ok := conn.(tlcpConnectionStater); ok {
		printTLCPConnectState(conn, cs.ConnectionState())
	}
}

func printTLSConnectState(conn net.Conn, state tls.ConnectionState) {
	tlsVersion := func(version uint16) string {
		switch version {
		case tls.VersionTLS10:
			return "TLSv10"
		case tls.VersionTLS11:
			return "TLSv11"
		case tls.VersionTLS12:
			return "TLSv12"
		case tls.VersionTLS13:
			return "TLSv13"
		default:
			return "Unknown"
		}
	}(state.Version)

	fmt.Printf("option Conn type: %T\n", conn)
	fmt.Printf("option TLS.Version: %s\n", tlsVersion)
	for _, v := range state.PeerCertificates {
		fmt.Println("option TLS.Subject:", v.Subject)
		fmt.Println("option TLS.KeyUsage:", KeyUsageString(v.KeyUsage))
	}
	fmt.Printf("option TLS.HandshakeComplete: %t\n", state.HandshakeComplete)
	fmt.Printf("option TLS.DidResume: %t\n", state.DidResume)
	for _, suit := range tls.CipherSuites() {
		if suit.ID == state.CipherSuite {
			fmt.Printf("option TLS.CipherSuite: %+v", suit)
			break
		}
	}
	fmt.Println()
}

func printTLCPConnectState(conn net.Conn, state tlcp.ConnectionState) {
	tlsVersion := func(version uint16) string {
		switch version {
		case tlcp.VersionTLCP:
			return "TLCP"
		default:
			return "Unknown"
		}
	}(state.Version)

	fmt.Printf("option Conn type: %T\n", conn)
	fmt.Printf("option TLCP.Version: %s\n", tlsVersion)
	for _, v := range state.PeerCertificates {
		fmt.Println("option TLCP.Subject:", v.Subject)
		fmt.Println("option TLCP.KeyUsage:", KeyUsageString(v.KeyUsage))
	}
	fmt.Printf("option TLCP.HandshakeComplete: %t\n", state.HandshakeComplete)
	fmt.Printf("option TLCP.DidResume: %t\n", state.DidResume)
	for _, suit := range tlcp.CipherSuites() {
		if suit.ID == state.CipherSuite {
			fmt.Printf("option TLCP.CipherSuite: %+v", suit)
			break
		}
	}
	fmt.Println()
}

// KeyUsageString convert x509.KeyUsage to string.
func KeyUsageString(k x509.KeyUsage) []string {
	var usages []string

	if k&x509.KeyUsageDigitalSignature != 0 {
		usages = append(usages, "DigitalSignature")
	}
	if k&x509.KeyUsageContentCommitment != 0 {
		usages = append(usages, "ContentCommitment")
	}
	if k&x509.KeyUsageKeyEncipherment != 0 {
		usages = append(usages, "KeyEncipherment")
	}
	if k&x509.KeyUsageDataEncipherment != 0 {
		usages = append(usages, "DataEncipherment")
	}
	if k&x509.KeyUsageKeyAgreement != 0 {
		usages = append(usages, "KeyAgreement")
	}
	if k&x509.KeyUsageCertSign != 0 {
		usages = append(usages, "CertSign")
	}
	if k&x509.KeyUsageCRLSign != 0 {
		usages = append(usages, "CRLSign")
	}
	if k&x509.KeyUsageEncipherOnly != 0 {
		usages = append(usages, "EncipherOnly")
	}
	if k&x509.KeyUsageDecipherOnly != 0 {
		usages = append(usages, "DecipherOnly")
	}

	return usages
}
