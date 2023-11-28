package blow

import (
	"bufio"
	"encoding/base64"
	"fmt"
	"log"
	"net"
	"net/url"
	"os"
	"strings"
	"sync/atomic"
	"time"

	"github.com/bingoohuang/gg/pkg/osx/env"
	"github.com/bingoohuang/gg/pkg/rest"
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
		ips := strings.Split(os.Getenv("LOCAL_IP"), ",")
		for _, ip := range ips {
			ipAddr, err := net.ResolveIPAddr("ip", ip)
			if err != nil {
				log.Panicf("resolving IP %s: %v", ip, err)
			}
			addrs = append(addrs, &net.TCPAddr{IP: ipAddr.IP})
		}
		if Debug {
			log.Printf("LOCAL_IP resovled: %s", addrs)
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

var proxyFunc = func() func(addr string, tls bool) (*url.URL, error) {
	proxy := getEnvAny("PROXY", "proxy")
	proxyFunc := httpproxy.FromEnvironment().ProxyFunc()
	if proxy == "" {
		return func(addr string, tls bool) (*url.URL, error) {
			reqURL := &url.URL{Host: addr, Scheme: httpScheme}
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
