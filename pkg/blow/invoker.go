package blow

import (
	"bufio"
	"bytes"
	"context"
	"crypto/tls"
	"encoding/json"
	"fmt"
	"io"
	"io/ioutil"
	"log"
	"net"
	"net/url"
	"os"
	"regexp"
	"strconv"
	"strings"
	"sync"
	"sync/atomic"
	"time"
	"unicode"

	"github.com/bingoohuang/gg/pkg/codec/b64"

	"github.com/bingoohuang/gg/pkg/iox"

	"github.com/bingoohuang/gg/pkg/man"

	"github.com/bingoohuang/gg/pkg/vars"

	"github.com/bingoohuang/gg/pkg/fla9"

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
	opt             *Opt
	httpHeader      *fasthttp.RequestHeader
	uploadCache     bool
	uploadFileField string
	upload          string
	uploadChan      chan *internal.UploadChanValue

	httpInvoke     func(req *fasthttp.Request, rsp *fasthttp.Response) error
	readBytes      int64
	writeBytes     int64
	isTLS          bool
	printLock      sync.Locker
	pieArg         HttpieArg
	pieBody        *HttpieArgBody
	requestUriExpr vars.Subs
}

func NewInvoker(ctx context.Context, opt *Opt) (*Invoker, error) {
	r := &Invoker{opt: opt}
	r.printLock = NewConditionalLock(r.opt.printOption > 0)

	header, err := r.buildRequestClient(opt)
	if err != nil {
		return nil, err
	}

	requestURI := string(header.RequestURI())
	if opt.eval {
		r.requestUriExpr = vars.ParseExpr(requestURI)
	}
	r.httpHeader = header

	if opt.upload != "" {
		const cacheTag = ":cache"
		if strings.HasSuffix(opt.upload, cacheTag) {
			r.uploadCache = true
			opt.upload = strings.TrimSuffix(opt.upload, cacheTag)
		}

		if pos := strings.IndexRune(opt.upload, ':'); pos > 0 {
			r.uploadFileField = opt.upload[:pos]
			r.upload = opt.upload[pos+1:]
		} else {
			r.uploadFileField = "file"
			r.upload = opt.upload
		}
	}

	if r.upload != "" {
		uploadReader := internal.CreateFileReader(r.uploadFileField, r.upload, r.opt.saveRandDir)
		r.uploadChan = make(chan *internal.UploadChanValue)
		go internal.DealUploadFilePath(ctx, uploadReader, r.uploadChan, r.uploadCache)
	}

	return r, nil
}

func (r *Invoker) buildRequestClient(opt *Opt) (*fasthttp.RequestHeader, error) {
	var u *url.URL
	var err error

	if opt.url != "" {
		u, err = url.Parse(opt.url)
	} else if len(opt.profiles) > 0 {
		u, err = url.Parse(opt.profiles[0].URL)
	} else {
		return nil, fmt.Errorf("failed to parse url")
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

	r.pieArg = parseHttpieLikeArgs(fla9.Args())

	var h fasthttp.RequestHeader

	host := ""
	contentType := ""
	for _, hdr := range opt.headers {
		k, v := ss.Split2(hdr, ss.WithSeps(":"))
		if strings.EqualFold(k, "Host") {
			host = v
		} else if strings.EqualFold(k, "Content-Type") {
			contentType = v
		}
	}

	h.SetContentType(adjustContentType(opt, contentType))
	h.SetHost(ss.If(host != "", host, u.Host))

	opt.headers = funk.FilterString(opt.headers, func(hdr string) bool {
		k, _ := ss.Split2(hdr, ss.WithSeps(":"))
		return !strings.EqualFold(k, "Host") && !strings.EqualFold(k, "Content-Type")
	})

	method := detectMethod(opt, r.pieArg)
	h.SetMethod(method)
	r.pieBody = r.pieArg.Build(method, opt.form)

	query := u.Query()
	for _, v := range r.pieArg.query {
		query.Add(v.V1, v.V2)
	}

	if method == "GET" {
		for k, v := range r.pieArg.param {
			query.Add(k, v)
		}
	}

	u.RawQuery = query.Encode()
	h.SetRequestURI(u.RequestURI())

	h.Set("Accept", "application/json")
	for k, v := range r.pieArg.header {
		h.Set(k, v)
	}
	for _, hdr := range opt.headers {
		h.Set(ss.Split2(hdr, ss.WithSeps(":")))
	}

	if opt.auth != "" {
		b := opt.auth
		if c, err := b64.DecodeString(b); err != nil { // check if it is already set by base64 encoded
			b, _ = b64.EncodeString(b)
		} else {
			b, _ = b64.EncodeString(c)
		}

		h.Set("Authorization", "Basic "+b)
	}
	if r.opt.enableGzip {
		h.Set("Accept-Encoding", "gzip")
	}
	if r.opt.noKeepalive {
		h.Set("Connection", "close")
	}

	if r.opt.doTimeout == 0 {
		r.httpInvoke = cli.Do
	} else {
		r.httpInvoke = func(req *fasthttp.Request, rsp *fasthttp.Response) error {
			return cli.DoTimeout(req, rsp, r.opt.doTimeout)
		}
	}

	return &h, nil
}

func (r *Invoker) Run(*berf.Config) (*berf.Result, error) {
	req := fasthttp.AcquireRequest()
	resp := fasthttp.AcquireResponse()
	defer fasthttp.ReleaseRequest(req)
	defer fasthttp.ReleaseResponse(resp)

	if r.opt.logf != nil {
		r.opt.logf.MarkPos()
	}

	if len(r.opt.profiles) > 0 {
		return r.runProfiles(req, resp)
	}

	r.httpHeader.CopyTo(&req.Header)
	if len(r.requestUriExpr) > 0 && r.requestUriExpr.CountVars() > 0 {
		result := r.requestUriExpr.Eval(internal.Valuer)
		if v, ok := result.(string); ok {
			req.SetRequestURI(v)
		}
	}
	if r.isTLS {
		req.URI().SetScheme("https")
		req.URI().SetHostBytes(req.Header.Host())
	}

	return r.runOne(req, resp)
}

func (r *Invoker) runOne(req *fasthttp.Request, resp *fasthttp.Response) (*berf.Result, error) {
	closers, err := r.setBody(req)
	if err != nil {
		return nil, err
	}

	defer iox.Close(closers)

	rr := &berf.Result{}
	err = r.doRequest(req, resp, rr)
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
	rr.Status = append(rr.Status, parseStatus(rsp, r.opt.statusName))
	if r.opt.verbose >= 1 {
		rr.Counting = append(rr.Counting, rsp.LocalAddr().String()+"->"+rsp.RemoteAddr().String())
	}

	if r.opt.logf == nil && r.opt.printOption == 0 {
		return rsp.BodyWriteTo(ioutil.Discard)
	}

	bx := io.Discard
	b1 := &bytes.Buffer{}

	if r.opt.logf != nil {
		bb := &bytes.Buffer{}
		bx = bb
		defer r.opt.logf.Write(bb)
	}

	conn := rsp.LocalAddr().String() + "->" + rsp.RemoteAddr().String()
	_, _ = b1.WriteString(fmt.Sprintf("### %s time: %s cost: %s\n",
		conn, time.Now().Format(time.RFC3339Nano), rr.Cost))

	bw := bufio.NewWriter(b1)
	_ = req.Header.Write(bw)
	_ = req.BodyWriteTo(bw)
	_ = bw.Flush()

	r.printLock.Lock()
	defer r.printLock.Unlock()

	h := &req.Header
	ignoreBody := h.IsGet() || h.IsHead()
	statusCode := rsp.StatusCode()
	r.printReq(b1, bx, ignoreBody, statusCode)
	b1.Reset()

	header := rsp.Header.Header()
	_, _ = b1.Write(header)

	if string(rsp.Header.Peek("Content-Encoding")) == "gzip" {
		if d, err := rsp.BodyGunzip(); err != nil {
			return err
		} else {
			b1.Write(d)
		}
	} else if err := rsp.BodyWriteTo(b1); err != nil {
		return err
	}

	_, _ = b1.Write([]byte("\n\n"))
	r.printResp(b1, bx, rsp, statusCode)

	return nil
}

func (r *Invoker) printReq(b *bytes.Buffer, bx io.Writer, ignoreBody bool, statusCode int) {
	if r.opt.printOption == 0 && bx == nil {
		return
	}

	if !logStatus(statusCode) {
		bx = nil
	}

	dumpHeader, dumpBody := r.dump(b, bx, ignoreBody)
	if bx != nil {
		_, _ = bx.Write([]byte("\n"))
	}

	if r.opt.printOption == 0 {
		return
	}

	printNum := 0
	if r.opt.printOption&printReqHeader == printReqHeader {
		fmt.Println(ColorfulHeader(string(dumpHeader)))
		printNum++
	}
	if r.opt.printOption&printReqBody == printReqBody {
		if strings.TrimSpace(string(dumpBody)) != "" {
			printBody(dumpBody, printNum, r.opt.pretty)
			printNum++
		}
	}

	if printNum > 0 {
		fmt.Println()
	}
}

var logStatus = func() func(int) bool {
	if env := os.Getenv("BLOW_STATUS"); env != "" {
		excluded := ss.HasPrefix(env, "-")
		if excluded {
			env = env[1:]
		}
		status := ss.ParseInt(env)
		return func(code int) bool {
			if excluded {
				return code != status
			}
			return code == status
		}
	}

	return func(code int) bool {
		return code < 200 || code >= 300
	}
}()

func (r *Invoker) printResp(b *bytes.Buffer, bx io.Writer, rsp *fasthttp.Response, statusCode int) {
	if r.opt.printOption == 0 && bx == nil {
		return
	}

	if !logStatus(statusCode) {
		bx = nil
	}

	dumpHeader, dumpBody := r.dump(b, bx, false)

	if r.opt.printOption == 0 {
		return
	}

	printNum := 0
	if r.opt.printOption&printRespStatusCode == printRespStatusCode {
		fmt.Println(Color(strconv.Itoa(rsp.StatusCode()), Magenta))
		printNum++
	}
	if r.opt.printOption&printRespHeader == printRespHeader {
		fmt.Println(ColorfulHeader(string(dumpHeader)))
		printNum++
	}
	if r.opt.printOption&printRespBody == printRespBody {
		if string(dumpBody) != "\r\n" {
			if r.opt.statusName != "" {
				dumpBody = []byte(parseStatus(rsp, r.opt.statusName))
			}
			printBody(dumpBody, printNum, r.opt.pretty)
			printNum++
		}
	}
	if printNum > 0 && r.opt.printOption != printRespStatusCode {
		fmt.Println()
	}
}

var cLengthReg = regexp.MustCompile(`Content-Length: (\d+)`)

func (r *Invoker) dump(b *bytes.Buffer, bx io.Writer, ignoreBody bool) (dumpHeader, dumpBody []byte) {
	dump := b.String()
	dps := strings.Split(dump, "\n")
	for i, line := range dps {
		if len(strings.Trim(line, "\r\n ")) == 0 {
			dumpHeader = []byte(strings.Join(dps[:i], "\n"))
			dumpBody = []byte("\n" + strings.Join(dps[i:], "\n"))
			break
		}
	}

	if bx != nil {
		_, _ = bx.Write(dumpHeader)
	}
	cl := -1
	if subs := cLengthReg.FindStringSubmatch(string(dumpHeader)); len(subs) > 0 {
		cl = ss.ParseInt(subs[1])
	}

	blowMaxBody := 4096
	if env := os.Getenv("BERF_MAX_BODY"); env != "" {
		if envValue, err := man.ParseBytes(env); err == nil {
			blowMaxBody = int(envValue)
		} else {
			log.Printf("bad environment value format: %s", env)
		}
	}

	if !ignoreBody && (cl == 0 || (blowMaxBody > 0 && cl > blowMaxBody)) {
		dumpBody = []byte("\n\n--- streamed or too long, ignored ---\n")
	}

	if bx != nil {
		_, _ = bx.Write(dumpBody)
	}
	return dumpHeader, dumpBody
}

func printBody(dumpBody []byte, printNum int, pretty bool) {
	if printNum > 0 {
		fmt.Println()
	}

	body := formatResponseBody(dumpBody, pretty, berf.IsStdoutTerminal)
	body = strings.TrimFunc(body, func(r rune) bool { return unicode.IsSpace(r) })
	fmt.Println(body)
}

func (r *Invoker) runProfiles(req *fasthttp.Request, rsp *fasthttp.Response) (*berf.Result, error) {
	rr := &berf.Result{}
	defer r.updateThroughput(rr)

	for _, p := range r.opt.profiles {
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
	closers, err := p.CreateReq(r.isTLS, req, r.opt.enableGzip, r.opt.uploadIndex)
	defer iox.Close(closers)

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

func parseStatus(rsp *fasthttp.Response, statusName string) string {
	if statusName == "" {
		return strconv.Itoa(rsp.StatusCode())
	}

	status := jj.GetBytes(rsp.Body(), statusName).String()
	return ss.Or(status, "NA")
}

func (r *Invoker) setBody(req *fasthttp.Request) (internal.Closers, error) {
	if r.opt.bodyStreamFile != "" {
		file, err := os.Open(r.opt.bodyStreamFile)
		if err != nil {
			return nil, err
		}
		req.SetBodyStream(file, -1)
		return []io.Closer{file}, nil
	}

	if r.upload != "" {
		uv := <-r.uploadChan
		data := uv.Data()
		multi := data.CreateFileField(r.uploadFileField, r.opt.uploadIndex)
		for k, v := range multi.Headers {
			internal.SetHeader(req, k, v)
		}
		req.SetBodyStream(multi.NewReader(), int(multi.Size))
		return nil, nil
	}

	bodyBytes := r.opt.bodyBytes
	if len(bodyBytes) == 0 && r.pieBody.BodyString != "" {
		internal.SetHeader(req, "Content-Type", r.pieBody.ContentType)
		bodyBytes = []byte(r.pieBody.BodyString)
	}

	if len(bodyBytes) == 0 && r.opt.bodyLinesChan != nil {
		line, ok := <-r.opt.bodyLinesChan
		if !ok { // lines is read over
			return nil, io.EOF
		}
		bodyBytes = []byte(line)
	}

	if len(bodyBytes) > 0 && r.opt.eval {
		bodyBytes = []byte(internal.Gen(string(bodyBytes), internal.MayJSON))
	}

	if len(bodyBytes) > 0 {
		if r.opt.enableGzip {
			if d, err := gz.Gzip(bodyBytes); err == nil && len(d) < len(bodyBytes) {
				bodyBytes = d
				req.Header.Set("Content-Encoding", "gzip")
			}
		}
	} else if r.pieBody.Body != nil {
		internal.SetHeader(req, "Content-Type", r.pieBody.ContentType)
		req.SetBodyStream(r.pieBody.Body, -1)
		return nil, nil
	}

	req.SetBodyRaw(bodyBytes)
	return nil, nil
}

func adjustContentType(opt *Opt, contentType string) string {
	if contentType != "" {
		return contentType
	}

	if opt.jsonBody || json.Valid(opt.bodyBytes) {
		return `application/json; charset=utf-8`
	}

	return `plain/text; charset=utf-8`
}

func detectMethod(opt *Opt, arg HttpieArg) string {
	if opt.method != "" {
		return opt.method
	}

	if opt.MaybePost() || arg.MaybePost() {
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

func (o *Opt) buildTLSConfig() (*tls.Config, error) {
	var certs []tls.Certificate
	if o.certPath != "" && o.keyPath != "" {
		c, err := tls.LoadX509KeyPair(o.certPath, o.keyPath)
		if err != nil {
			return nil, err
		}
		certs = append(certs, c)
	}
	return &tls.Config{
		InsecureSkipVerify: o.insecure,
		Certificates:       certs,
	}, nil
}
