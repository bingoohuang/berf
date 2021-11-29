package blow

import (
	"bufio"
	"bytes"
	"context"
	"crypto/tls"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"io"
	"io/ioutil"
	"net"
	"net/url"
	"os"
	"strconv"
	"strings"
	"sync/atomic"
	"time"

	"github.com/bingoohuang/perf/pkg/blow/internal"

	"github.com/bingoohuang/gg/pkg/gz"
	"github.com/bingoohuang/jj"
	"github.com/bingoohuang/perf"
	"github.com/valyala/fasthttp"
	"github.com/valyala/fasthttp/fasthttpproxy"
)

type Invoker struct {
	clientOpt       *ClientOpt
	httpHeader      *fasthttp.RequestHeader
	noUploadCache   bool
	uploadFileField string
	upload          string
	uploadChan      chan string

	httpClientDo func(req *fasthttp.Request, rsp *fasthttp.Response) error
	readBytes    int64
	writeBytes   int64
	isTLS        bool
}

func NewInvoker(ctx context.Context, clientOpt *ClientOpt) (*Invoker, error) {
	r := &Invoker{
		clientOpt: clientOpt,
	}

	header, err := r.buildRequestClient(clientOpt)
	if err != nil {
		return nil, err
	}
	r.httpHeader = header

	if clientOpt.upload != "" {
		const nocacheTag = ":nocache"
		if strings.HasSuffix(clientOpt.upload, nocacheTag) {
			r.noUploadCache = true
			clientOpt.upload = strings.TrimSuffix(clientOpt.upload, nocacheTag)
		}

		if pos := strings.IndexRune(clientOpt.upload, ':'); pos > 0 {
			r.uploadFileField = clientOpt.upload[:pos]
			r.upload = clientOpt.upload[pos+1:]
		} else {
			r.uploadFileField = "file"
			r.upload = clientOpt.upload
		}
	}

	if r.upload != "" {
		r.uploadChan = make(chan string, 1)
		go internal.DealUploadFilePath(ctx, r.upload, r.uploadChan)
	}

	return r, nil
}

func (r *Invoker) buildRequestClient(opt *ClientOpt) (*fasthttp.RequestHeader, error) {
	var u *url.URL
	var err error

	if opt.url != "" {
		u, err = url.Parse(opt.url)
	} else if len(opt.profiles) > 0 {
		u, err = url.Parse(opt.profiles[0].URL)
	}

	if err != nil {
		return nil, err
	}

	r.isTLS = u.Scheme == "https"

	httpClient := &fasthttp.HostClient{
		Addr:         addMissingPort(u.Host, u.Scheme == "https"),
		IsTLS:        r.isTLS,
		Name:         "blow",
		MaxConns:     opt.maxConns,
		ReadTimeout:  opt.readTimeout,
		WriteTimeout: opt.writeTimeout,

		DisableHeaderNamesNormalizing: true,
	}

	if opt.socks5Proxy != "" {
		if !strings.Contains(opt.socks5Proxy, "://") {
			opt.socks5Proxy = "socks5://" + opt.socks5Proxy
		}
		httpClient.Dial = fasthttpproxy.FasthttpSocksDialer(opt.socks5Proxy)
	} else {
		httpClient.Dial = fasthttpproxy.FasthttpProxyHTTPDialerTimeout(opt.dialTimeout)
	}

	httpClient.Dial = internal.ThroughputStatDial(internal.NetworkWrap(opt.network), httpClient.Dial, &r.readBytes, &r.writeBytes)

	tlsConfig, err := buildTLSConfig(opt)
	if err != nil {
		return nil, err
	}
	httpClient.TLSConfig = tlsConfig

	var h fasthttp.RequestHeader
	h.SetContentType(adjustContentType(opt))
	if opt.host != "" {
		h.SetHost(opt.host)
	} else {
		h.SetHost(u.Host)
	}

	h.SetMethod(adjustMethod(opt))
	h.SetRequestURI(u.RequestURI())
	for _, v := range opt.headers {
		n := strings.SplitN(v, ":", 2)
		if len(n) != 2 {
			return nil, fmt.Errorf("invalid header: %s", v)
		}
		h.Set(n[0], n[1])
	}

	if opt.basicAuth != "" {
		h.Set("Authorization", "Basic "+base64.StdEncoding.EncodeToString([]byte(opt.basicAuth)))
	}

	if r.clientOpt.doTimeout > 0 {
		r.httpClientDo = func(req *fasthttp.Request, rsp *fasthttp.Response) error {
			return httpClient.DoTimeout(req, rsp, r.clientOpt.doTimeout)
		}
	} else {
		r.httpClientDo = httpClient.Do
	}

	return &h, nil
}

func (r *Invoker) Run() (*perf.Result, error) {
	req := fasthttp.AcquireRequest()
	resp := fasthttp.AcquireResponse()
	defer fasthttp.ReleaseRequest(req)
	defer fasthttp.ReleaseResponse(resp)

	if len(r.clientOpt.profiles) == 0 {
		r.httpHeader.CopyTo(&req.Header)
		if r.isTLS {
			req.URI().SetScheme("https")
			req.URI().SetHostBytes(req.Header.Host())
		}

		if r.clientOpt.enableGzip {
			req.Header.Set("Accept-Encoding", "gzip")
		}
	}

	if r.clientOpt.logf != nil {
		r.clientOpt.logf.MarkPos()
	}

	if len(r.clientOpt.profiles) == 0 {
		return r.runOne(req, resp)
	}

	return r.runProfiles(req, resp)
}

func (r *Invoker) runOne(req *fasthttp.Request, resp *fasthttp.Response) (*perf.Result, error) {
	closers, err := r.setBody(req)
	defer closers.Close()

	rr := &perf.Result{}
	if err == nil {
		err = r.doRequest(req, resp, rr)
	}

	if err != nil {
		return nil, err
	}

	r.updateThroughput(rr)
	return rr, nil
}

func (r *Invoker) updateThroughput(rr *perf.Result) {
	rr.ReadBytes = atomic.LoadInt64(&r.readBytes)
	atomic.StoreInt64(&r.readBytes, 0)
	rr.WriteBytes = atomic.LoadInt64(&r.writeBytes)
	atomic.StoreInt64(&r.writeBytes, 0)
}

func (r *Invoker) doRequest(req *fasthttp.Request, rsp *fasthttp.Response, rr *perf.Result) (err error) {
	t1 := time.Now()
	err = r.httpClientDo(req, rsp)
	rr.Cost = time.Since(t1)
	if err != nil {
		return err
	}

	rr.Status = []string{parseStatus(rsp, r.clientOpt.statusName)}
	if r.clientOpt.verbose >= 1 {
		rr.Counting = []string{rsp.LocalAddr().String() + "->" + rsp.RemoteAddr().String()}
	}
	if r.clientOpt.logf != nil {
		return r.logDetail(req, rsp, rr)
	}

	return rsp.BodyWriteTo(ioutil.Discard)
}

func (r *Invoker) logDetail(req *fasthttp.Request, rsp *fasthttp.Response, rr *perf.Result) error {
	b := &bytes.Buffer{}
	defer r.clientOpt.logf.Write(b)

	conn := rsp.LocalAddr().String() + "->" + rsp.RemoteAddr().String()
	_, _ = b.WriteString(fmt.Sprintf("### %s time: %s cost: %s\n",
		conn, time.Now().Format(time.RFC3339Nano), rr.Cost))

	bw := bufio.NewWriter(b)
	_ = req.Write(bw)
	_ = bw.Flush()
	_, _ = b.Write([]byte("\n"))

	_, _ = b.Write(rsp.Header.Header())

	if string(rsp.Header.Peek("Content-Encoding")) == "gzip" {
		bodyGunzip, err := rsp.BodyGunzip()
		if err != nil {
			return err
		}
		b.Write(bodyGunzip)
	} else {
		if err := rsp.BodyWriteTo(b); err != nil {
			return err
		}
	}

	_, _ = b.Write([]byte("\n\n"))
	return nil
}

func (r *Invoker) runProfiles(req *fasthttp.Request, rsp *fasthttp.Response) (*perf.Result, error) {
	rr := &perf.Result{}
	defer r.updateThroughput(rr)

	for _, p := range r.clientOpt.profiles {
		if err := r.runOneProfile(p, req, rsp, rr); err != nil {
			return rr, err
		}

		if rsp.StatusCode() < 200 || rsp.StatusCode() > 300 {
			break
		}

		req.Reset()
		rsp.Reset()
	}

	return rr, nil
}

func (r *Invoker) runOneProfile(p *internal.Profile, req *fasthttp.Request, rsp *fasthttp.Response, rr *perf.Result) error {
	closers, err := p.CreateReq(r.isTLS, req, r.clientOpt.enableGzip)
	defer closers.Close()

	if err != nil {
		return err
	}

	t1 := time.Now()
	err = r.httpClientDo(req, rsp)
	rr.Cost += time.Since(t1)
	if err != nil {
		return err
	}

	rr.Status = append(rr.Status, parseStatus(rsp, r.clientOpt.statusName))
	if r.clientOpt.verbose >= 1 {
		rr.Counting = append(rr.Counting, rsp.LocalAddr().String()+"->"+rsp.RemoteAddr().String())
	}
	if r.clientOpt.logf != nil {
		return r.logDetail(req, rsp, rr)
	}

	return rsp.BodyWriteTo(ioutil.Discard)
}

func parseStatus(resp *fasthttp.Response, statusName string) string {
	if statusName == "" {
		return strconv.Itoa(resp.StatusCode())
	}

	return jj.GetBytes(resp.Body(), statusName).String()
}

func (r *Invoker) setBody(req *fasthttp.Request) (internal.Closers, error) {
	if r.clientOpt.bodyFile != "" {
		file, err := os.Open(r.clientOpt.bodyFile)
		if err != nil {
			return nil, err
		}
		req.SetBodyStream(file, -1)
		return []io.Closer{file}, nil
	}
	if r.upload != "" {
		file := <-r.uploadChan
		data, cType, err := internal.ReadMultipartFile(r.noUploadCache, r.uploadFileField, file)
		if err != nil {
			panic(err)
		}
		internal.SetHeader(req, "Content-Type", cType)
		req.SetBody(data)
		return nil, nil
	}

	bodyBytes := r.clientOpt.bodyBytes

	if r.clientOpt.enableGzip {
		gzBytes, _ := gz.Gzip(bodyBytes)
		if len(gzBytes) < len(bodyBytes) {
			bodyBytes = gzBytes
			req.Header.Set("Content-Encoding", "gzip")
		}
	}

	req.SetBodyRaw(bodyBytes)
	return nil, nil
}

func adjustContentType(opt *ClientOpt) string {
	if opt.contentType != "" {
		return opt.contentType
	}

	if json.Valid(opt.bodyBytes) {
		return `application/json; charset=utf-8`
	}

	return `plain/text; charset=utf-8`
}

func adjustMethod(opt *ClientOpt) string {
	if opt.method != "" {
		return opt.method
	}

	if opt.upload != "" || len(opt.bodyBytes) > 0 || opt.bodyFile != "" {
		return "POST"
	}

	return "GET"
}

func addMissingPort(addr string, isTLS bool) string {
	if addr == "" {
		return ""
	}

	if n := strings.Index(addr, ":"); n >= 0 {
		return addr
	}
	p := 80
	if isTLS {
		p = 443
	}
	return net.JoinHostPort(addr, strconv.Itoa(p))
}

func buildTLSConfig(opt *ClientOpt) (*tls.Config, error) {
	var certs []tls.Certificate
	if opt.certPath != "" && opt.keyPath != "" {
		c, err := tls.LoadX509KeyPair(opt.certPath, opt.keyPath)
		if err != nil {
			return nil, err
		}
		certs = append(certs, c)
	}
	return &tls.Config{
		InsecureSkipVerify: opt.insecure,
		Certificates:       certs,
	}, nil
}
