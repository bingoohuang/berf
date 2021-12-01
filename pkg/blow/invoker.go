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

	"github.com/thoas/go-funk"

	"github.com/bingoohuang/gg/pkg/ss"

	"github.com/bingoohuang/berf/pkg/blow/internal"

	"github.com/bingoohuang/berf"
	"github.com/bingoohuang/gg/pkg/gz"
	"github.com/bingoohuang/jj"
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

	httpInvoke func(req *fasthttp.Request, rsp *fasthttp.Response) error
	readBytes  int64
	writeBytes int64
	isTLS      bool
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

	cli := &fasthttp.HostClient{
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
		cli.Dial = fasthttpproxy.FasthttpSocksDialer(opt.socks5Proxy)
	} else {
		cli.Dial = fasthttpproxy.FasthttpProxyHTTPDialerTimeout(opt.dialTimeout)
	}

	wrap := internal.NetworkWrap(opt.network)
	cli.Dial = internal.ThroughputStatDial(wrap, cli.Dial, &r.readBytes, &r.writeBytes)
	if cli.TLSConfig, err = opt.buildTLSConfig(); err != nil {
		return nil, err
	}

	var h fasthttp.RequestHeader
	h.SetContentType(adjustContentType(opt))

	host := ""
	for _, hdr := range opt.headers {
		k, v := ss.Split2(hdr, ss.WithSeps(":"))
		if strings.EqualFold(k, "Host") {
			host = v
			break
		}
	}
	h.SetHost(ss.If(host != "", host, u.Host))

	opt.headers = funk.FilterString(opt.headers, func(hdr string) bool {
		k, _ := ss.Split2(hdr, ss.WithSeps(":"))
		return !strings.EqualFold(k, "Host")
	})

	h.SetMethod(adjustMethod(opt))
	h.SetRequestURI(u.RequestURI())
	for _, hdr := range opt.headers {
		h.Set(ss.Split2(hdr, ss.WithSeps(":")))
	}

	if opt.basicAuth != "" {
		h.Set("Authorization", "Basic "+base64.StdEncoding.EncodeToString([]byte(opt.basicAuth)))
	}

	if r.clientOpt.doTimeout == 0 {
		r.httpInvoke = cli.Do
	} else {
		r.httpInvoke = func(req *fasthttp.Request, rsp *fasthttp.Response) error {
			return cli.DoTimeout(req, rsp, r.clientOpt.doTimeout)
		}
	}

	return &h, nil
}

func (r *Invoker) Run() (*berf.Result, error) {
	req := fasthttp.AcquireRequest()
	resp := fasthttp.AcquireResponse()
	defer fasthttp.ReleaseRequest(req)
	defer fasthttp.ReleaseResponse(resp)

	if r.clientOpt.logf != nil {
		r.clientOpt.logf.MarkPos()
	}

	if len(r.clientOpt.profiles) > 0 {
		return r.runProfiles(req, resp)
	}

	r.httpHeader.CopyTo(&req.Header)
	if r.isTLS {
		req.URI().SetScheme("https")
		req.URI().SetHostBytes(req.Header.Host())
	}

	if r.clientOpt.enableGzip {
		req.Header.Set("Accept-Encoding", "gzip")
	}
	return r.runOne(req, resp)
}

func (r *Invoker) runOne(req *fasthttp.Request, resp *fasthttp.Response) (*berf.Result, error) {
	closers, err := r.setBody(req)
	defer closers.Close()

	rr := &berf.Result{}
	if err == nil {
		err = r.doRequest(req, resp, rr)
	}
	r.updateThroughput(rr)

	return rr, err
}

func (r *Invoker) updateThroughput(rr *berf.Result) {
	rr.ReadBytes = atomic.SwapInt64(&r.readBytes, 0)
	rr.WriteBytes = atomic.SwapInt64(&r.writeBytes, 0)
}

func (r *Invoker) doRequest(req *fasthttp.Request, rsp *fasthttp.Response, rr *berf.Result) (err error) {
	t1 := time.Now()
	err = r.httpInvoke(req, rsp)
	rr.Cost = time.Since(t1)
	if err != nil {
		return err
	}

	return r.processRsp(req, rsp, rr)
}

func (r *Invoker) processRsp(req *fasthttp.Request, rsp *fasthttp.Response, rr *berf.Result) error {
	rr.Status = append(rr.Status, parseStatus(rsp, r.clientOpt.statusName))
	if r.clientOpt.verbose >= 1 {
		rr.Counting = append(rr.Counting, rsp.LocalAddr().String()+"->"+rsp.RemoteAddr().String())
	}

	if r.clientOpt.logf == nil {
		return rsp.BodyWriteTo(ioutil.Discard)
	}

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
		if d, err := rsp.BodyGunzip(); err != nil {
			return err
		} else {
			b.Write(d)
		}
	} else if err := rsp.BodyWriteTo(b); err != nil {
		return err
	}

	_, _ = b.Write([]byte("\n\n"))
	return nil
}

func (r *Invoker) runProfiles(req *fasthttp.Request, rsp *fasthttp.Response) (*berf.Result, error) {
	rr := &berf.Result{}
	defer r.updateThroughput(rr)

	for _, p := range r.clientOpt.profiles {
		if err := r.runOneProfile(p, req, rsp, rr); err != nil {
			return rr, err
		}

		if code := rsp.StatusCode(); code < 200 || code > 300 {
			break
		}

		req.Reset()
		rsp.Reset()
	}

	return rr, nil
}

func (r *Invoker) runOneProfile(p *internal.Profile, req *fasthttp.Request, rsp *fasthttp.Response, rr *berf.Result) error {
	closers, err := p.CreateReq(r.isTLS, req, r.clientOpt.enableGzip)
	defer closers.Close()

	if err != nil {
		return err
	}

	t1 := time.Now()
	err = r.httpInvoke(req, rsp)
	rr.Cost += time.Since(t1)
	if err != nil {
		return err
	}

	return r.processRsp(req, rsp, rr)
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
		if d, err := gz.Gzip(bodyBytes); err == nil && len(d) < len(bodyBytes) {
			bodyBytes = d
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

	return net.JoinHostPort(addr, ss.If(isTLS, "443", "80"))
}

func (opt *ClientOpt) buildTLSConfig() (*tls.Config, error) {
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
