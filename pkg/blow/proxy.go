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
	tlsPort     = "443"
)

// ProxyHTTPDialerTimeout returns a fasthttp.DialFunc that dials using
// code original from fasthttpproxy.ProxyHTTPDialerTimeout
// the env(HTTP_PROXY, HTTPS_PROXY and NO_PROXY) configured HTTP proxy using the given timeout.
//
// Example usage:
//
//	c := &fasthttp.Client{
//		Dial: ProxyHTTPDialerTimeout(time.Second * 2),
//	}
func ProxyHTTPDialerTimeout(timeout time.Duration, dialer Dialer) fasthttp.DialFunc {
	proxyFunc := httpproxy.FromEnvironment().ProxyFunc()

	// encoded auth barrier for http and https proxy.
	authHTTPStorage := &atomic.Value{}
	authHTTPSStorage := &atomic.Value{}

	return func(addr string) (net.Conn, error) {
		port, _, err := net.SplitHostPort(addr)
		if err != nil {
			return nil, fmt.Errorf("unexpected addr format: %w", err)
		}

		reqURL := &url.URL{Host: addr, Scheme: httpScheme}
		if port == tlsPort {
			reqURL.Scheme = httpsScheme
		}
		proxyURL, err := proxyFunc(reqURL)
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

		req := "CONNECT " + addr + " HTTP/1.1\r\n"

		if proxyURL.User != nil {
			authBarrierStorage := authHTTPStorage
			if port == tlsPort {
				authBarrierStorage = authHTTPSStorage
			}

			auth := authBarrierStorage.Load()
			if auth == nil {
				authBarrier := base64.StdEncoding.EncodeToString([]byte(proxyURL.User.String()))
				auth = &authBarrier
				authBarrierStorage.Store(auth)
			}

			req += "Proxy-Authorization: Basic " + *auth.(*string) + "\r\n"
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
