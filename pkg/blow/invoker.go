package blow

import (
	"bufio"
	"bytes"
	"context"
	"crypto/tls"
	"crypto/x509"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net/url"
	"os"
	"regexp"
	"strconv"
	"strings"
	"sync"
	"sync/atomic"
	"time"
	"unicode"

	"github.com/bingoohuang/berf"
	"github.com/bingoohuang/berf/pkg/blow/internal"
	"github.com/bingoohuang/gg/pkg/codec/b64"
	"github.com/bingoohuang/gg/pkg/fla9"
	"github.com/bingoohuang/gg/pkg/gz"
	"github.com/bingoohuang/gg/pkg/iox"
	"github.com/bingoohuang/gg/pkg/man"
	"github.com/bingoohuang/gg/pkg/osx"
	"github.com/bingoohuang/gg/pkg/osx/env"
	"github.com/bingoohuang/gg/pkg/ss"
	"github.com/bingoohuang/gg/pkg/vars"
	"github.com/bingoohuang/jj"
	"github.com/thoas/go-funk"
	"github.com/valyala/fasthttp"
)

type Invoker struct {
	pieArg     HttpieArg
	printLock  sync.Locker
	httpHeader *fasthttp.RequestHeader
	pieBody    *HttpieArgBody
	opt        *Opt
	uploadChan chan *internal.UploadChanValue

	httpInvoke      func(*fasthttp.Request, *fasthttp.Response) error
	uploadFileField string
	upload          string
	requestUriExpr  vars.Subs
	writeBytes      int64
	readBytes       int64
	isTLS           bool
	uploadCache     bool
}

func NewInvoker(ctx context.Context, opt *Opt) (*Invoker, error) {
	r := &Invoker{opt: opt}
	r.printLock = NewConditionalLock(r.opt.printOption > 0)

	header, err := r.buildRequestClient(ctx, opt)
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

func (r *Invoker) buildRequestClient(ctx context.Context, opt *Opt) (*fasthttp.RequestHeader, error) {
	var u *url.URL
	var err error

	switch {
	case opt.url != "":
		u, err = url.Parse(opt.url)
	case len(opt.profiles) > 0:
		u, err = url.Parse(opt.profiles[0].URL)
	default:
		return nil, fmt.Errorf("failed to parse url")
	}

	if err != nil {
		return nil, err
	}

	originSchemeHTTPS := u.Scheme == "https"
	envTLCP := env.Bool("TLCP", false)
	if originSchemeHTTPS && envTLCP {
		u.Scheme = "http"
	}

	r.isTLS = u.Scheme == "https"

	cli := &fasthttp.Client{
		Name:            "blow",
		MaxConnsPerHost: opt.maxConns,
		ReadTimeout:     opt.readTimeout,
		WriteTimeout:    opt.writeTimeout,
	}

	cli.Dial = ProxyHTTPDialerTimeout(opt.dialTimeout, dialer)

	wrap := internal.NetworkWrap(opt.network)
	cli.Dial = internal.ThroughputStatDial(wrap, cli.Dial, &r.readBytes, &r.writeBytes)
	if originSchemeHTTPS && envTLCP {
		cli.Dial = createTlcpDialer(ctx, cli.Dial, r.opt.certPath, r.opt.tlcpCerts, r.opt.HasPrintOption, r.opt.tlsVerify)
	}

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

func (r *Invoker) Run(ctx context.Context, _ *berf.Config, initial bool) (*berf.Result, error) {
	req := fasthttp.AcquireRequest()
	resp := fasthttp.AcquireResponse()
	defer fasthttp.ReleaseRequest(req)
	defer fasthttp.ReleaseResponse(resp)

	if r.opt.logf != nil {
		r.opt.logf.MarkPos()
	}

	if len(r.opt.profiles) > 0 {
		return r.runProfiles(req, resp, initial)
	}

	r.httpHeader.CopyTo(&req.Header)
	if len(r.requestUriExpr) > 0 && r.requestUriExpr.CountVars() > 0 {
		result := r.requestUriExpr.Eval(internal.Valuer)
		if v, ok := result.(string); ok {
			req.SetRequestURI(v)
		}
	}
	if r.isTLS {
		uri := req.URI()
		uri.SetScheme("https")
		uri.SetHostBytes(req.Header.Host())
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

	return r.processRsp(req, rsp, rr, nil)
}

func (r *Invoker) processRsp(req *fasthttp.Request, rsp *fasthttp.Response, rr *berf.Result, responseJSONValuer func(jsonBody []byte)) error {
	rr.Status = append(rr.Status, parseStatus(rsp, r.opt.statusName))
	if r.opt.verbose >= 1 {
		rr.Counting = append(rr.Counting, rsp.LocalAddr().String()+"->"+rsp.RemoteAddr().String())
	}

	if r.opt.logf == nil && r.opt.printOption == 0 {
		return rsp.BodyWriteTo(io.Discard)
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
	bb1 := b1

	var body *bytes.Buffer

	if responseJSONValuer != nil && bytes.HasPrefix(rsp.Header.Peek("Content-Type"), []byte("application/json")) {
		body = &bytes.Buffer{}
		bb1 = body
	}

	if string(rsp.Header.Peek("Content-Encoding")) == "gzip" {
		if d, err := rsp.BodyGunzip(); err != nil {
			return err
		} else {
			bb1.Write(d)
		}
	} else if err := rsp.BodyWriteTo(bb1); err != nil {
		return err
	}

	if responseJSONValuer != nil && body != nil {
		i := body.Bytes()
		responseJSONValuer(i)
		b1.Write(i)
	}

	_, _ = b1.Write([]byte("\n\n"))
	r.printResp(b1, bx, rsp, statusCode)

	return nil
}

func (r *Invoker) printReq(b *bytes.Buffer, bx io.Writer, ignoreBody bool, statusCode int) {
	if r.opt.printOption == 0 && bx == nil {
		return
	}

	if !logStatus(r.opt.berfConfig.N, statusCode) {
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
	if r.opt.HasPrintOption(printReqHeader) {
		fmt.Println(ColorfulHeader(string(dumpHeader)))
		printNum++
	}
	if r.opt.HasPrintOption(printReqBody) {
		if strings.TrimSpace(string(dumpBody)) != "" {
			printBody(dumpBody, printNum, r.opt.pretty)
			printNum++
		}
	}

	if printNum > 0 {
		fmt.Println()
	}
}

var logStatus = func() func(n, code int) bool {
	if env := os.Getenv("BLOW_STATUS"); env != "" {
		excluded := ss.HasPrefix(env, "-")
		if excluded {
			env = env[1:]
		}
		status := ss.ParseInt(env)
		return func(n, code int) bool {
			if n == 1 {
				return true
			}
			if excluded {
				return code != status
			}
			return code == status
		}
	}

	return func(n, code int) bool {
		if n == 1 {
			return true
		}
		return code < 200 || code >= 300
	}
}()

func (r *Invoker) printResp(b *bytes.Buffer, bx io.Writer, rsp *fasthttp.Response, statusCode int) {
	if r.opt.printOption == 0 && bx == nil {
		return
	}

	if !logStatus(r.opt.berfConfig.N, statusCode) {
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

	maxBody := 4096
	if env := os.Getenv("MAX_BODY"); env != "" {
		if envValue, err := man.ParseBytes(env); err == nil {
			maxBody = int(envValue)
		} else {
			log.Printf("bad environment value format: %s", env)
		}
	}

	if !ignoreBody && (cl == 0 || (maxBody > 0 && cl > maxBody)) {
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
	body = strings.TrimFunc(body, unicode.IsSpace)
	fmt.Println(body)
}

func (r *Invoker) runProfiles(req *fasthttp.Request, rsp *fasthttp.Response, initial bool) (*berf.Result, error) {
	rr := &berf.Result{}
	defer r.updateThroughput(rr)

	profiles := r.opt.profiles
	if initial {
		initProfiles := make([]*internal.Profile, 0, len(profiles))
		nonInitial := make([]*internal.Profile, 0, len(profiles))
		for _, p := range r.opt.profiles {
			if p.Init {
				initProfiles = append(initProfiles, p)
			} else {
				nonInitial = append(nonInitial, p)
			}
		}
		profiles = initProfiles
		r.opt.profiles = nonInitial
	}

	for _, p := range profiles {
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

	f := createJSONValuer(p)
	return r.processRsp(req, rsp, rr, f)
}

func createJSONValuer(p *internal.Profile) func(jsonBody []byte) {
	if p.Init {
		if expr := p.ResultExpr; len(expr) > 0 {
			return func(jsonBody []byte) {
				for ek, ev := range expr {
					if jr := jj.GetBytes(jsonBody, ev); jr.Type != jj.Null {
						internal.Valuer.Register(ek, func(string) interface{} {
							return jr.String()
						})
					}
				}
			}
		}
	}

	return nil
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

type noSessionCache struct{}

func (noSessionCache) Get(string) (*tls.ClientSessionState, bool) { return nil, false }
func (noSessionCache) Put(string, *tls.ClientSessionState)        { /* no-op */ }

func (o *Opt) buildTLSConfig() (*tls.Config, error) {
	var certs []tls.Certificate
	if o.certPath != "" && o.keyPath != "" {
		c, err := tls.LoadX509KeyPair(o.certPath, o.keyPath)
		if err != nil {
			return nil, err
		}
		certs = append(certs, c)
	}

	t := &tls.Config{
		InsecureSkipVerify: !o.tlsVerify,
		Certificates:       certs,
		// 关闭 HTTP 客户端的会话缓存
		SessionTicketsDisabled: o.noTLSessionTickets,
	}

	if cacheSize := env.Int(`TLS_SESSION_CACHE`, 32); cacheSize > 0 {
		t.ClientSessionCache = tls.NewLRUClientSessionCache(cacheSize)
	} else {
		t.ClientSessionCache = &noSessionCache{}
	}

	if o.rootCert != "" {
		pool := x509.NewCertPool()
		pool.AppendCertsFromPEM(osx.ReadFile(o.rootCert, osx.WithFatalOnError(true)).Data)
		t.RootCAs = pool
	}

	return t, nil
}
