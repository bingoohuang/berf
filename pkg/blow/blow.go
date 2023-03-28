package blow

import (
	"context"
	"fmt"
	"log"
	"os"
	"strings"
	"time"

	"github.com/bingoohuang/berf"
	"github.com/bingoohuang/berf/pkg/blow/internal"
	"github.com/bingoohuang/berf/pkg/util"
	"github.com/bingoohuang/gg/pkg/fla9"
	"github.com/bingoohuang/gg/pkg/osx"
	"github.com/bingoohuang/gg/pkg/rest"
	"github.com/bingoohuang/gg/pkg/ss"
)

var (
	pURL      = fla9.String("url", "", "URL")
	pBody     = fla9.String("body,b", "", "HTTP request body, or @file to read from, or @file:stream to enable chunked encoding for the file, or @file:line to read line by line")
	pUpload   = fla9.String("upload,u", "", "HTTP upload multipart form file or directory, or add prefix file: to set form field name, extension: rand.png,rand.art,rand.jpg,rand.json")
	pMethod   = fla9.String("method,m", "", "HTTP method")
	pNetwork  = fla9.String("network", "", "Network simulation, local: simulates local network, lan: local, wan: wide, bad: bad network, or BPS:latency like 20M:20ms")
	pHeaders  = fla9.Strings("header,H", nil, "Custom HTTP headers, K:V, e.g. Content-Type")
	pProfiles = fla9.Strings("profile,P", nil, "Profile file, append :new to create a demo profile, or :tag to run only specified profile, or range :tag1,tag3-tag5")
	pEnv      = fla9.String("env", "", "Profile env name selected")
	pOpts     = fla9.Strings("opt", nil, "options, multiple by comma: \n"+
		"      gzip:               enabled content gzip  \n"+
		"      tlsVerify:          verify the server's cert chain and host name \n"+
		"      no-keepalive/no-ka: disable keepalive \n"+
		"      form:               use form instead of json \n"+
		"      pretty:             pretty JSON \n"+
		"      uploadIndex:        insert upload index to filename \n"+
		"      saveRandDir:        save rand generated request files to dir \n"+
		"      json:               set Content-Type=application/json; charset=utf-8 \n"+
		"      eval:               evaluate url and body's variables \n"+
		"      notty:              no tty color \n")
	pAuth       = fla9.String("auth", "", "basic auth, eg. scott:tiger or direct base64 encoded like c2NvdHQ6dGlnZXI")
	pCertKey    = fla9.String("cert", "", "Path to the client's TLS Cert and private key file, eg. ca.pem,ca.key")
	pRootCert   = fla9.String("root-ca", "", "Ca root certificate file to verify TLS")
	pTlcpCerts  = fla9.String("tlcp-certs", "", "format: sign.cert.pem,sign.key.pem,enc.cert.pem,enc.key.pem")
	pTimeout    = fla9.String("timeout", "", "Timeout for each http request, e.g. 5s for do:5s,dial:5s,write:5s,read:5s")
	pSocks5     = fla9.String("socks5", "", "Socks5 proxy, ip:port")
	pPrint      = fla9.String("print,p", "", "a: all, R: req all, H: req headers, B: req body, r: resp all, h: resp headers b: resp body c: status code")
	pStatusName = fla9.String("status", "", "Status name in json, like resultCode")
)

const (
	printReqHeader uint8 = 1 << iota
	printReqBody
	printRespHeader
	printRespBody
	printRespStatusCode
	printDebug
)

func parsePrintOption(s string) (printOption uint8) {
	for r, v := range map[string]uint8{
		"A": printReqHeader | printReqBody | printRespHeader | printRespBody,
		"a": printReqHeader | printReqBody | printRespHeader | printRespBody,
		"R": printReqHeader | printReqBody,
		"r": printRespHeader | printRespBody,
		"H": printReqHeader,
		"B": printReqBody,
		"h": printRespHeader,
		"b": printRespBody,
		"c": printRespStatusCode,
		"d": printDebug,
	} {
		if strings.Contains(s, r) {
			printOption |= v
			s = strings.ReplaceAll(s, r, "")
		}
	}

	if s = strings.TrimSpace(s); s != "" {
		log.Printf("unknown print options, %s", s)
		os.Exit(1)
	}

	if printOption&printRespHeader == printRespHeader {
		printOption &^= printRespStatusCode
	}

	return printOption
}

type Bench struct {
	invoker *Invoker
}

func (b *Bench) Name(context.Context, *berf.Config) string {
	opt := b.invoker.opt
	if v := opt.url; v != "" {
		return v
	}

	return "profiles " + strings.Join(*pProfiles, ",")
}

func (b *Bench) Final(_ context.Context, conf *berf.Config) error {
	opt := b.invoker.opt

	if opt.logf != nil {
		defer opt.logf.Close()
	}

	if conf.N == 1 && opt.logf != nil && opt.printOption == 0 {
		if v := opt.logf.GetLastLog(); v != "" {
			v = colorJSON(v, opt.pretty)
			_, _ = os.Stdout.WriteString(v)
		}
	}
	return nil
}

func (b *Bench) Init(ctx context.Context, conf *berf.Config) (*berf.BenchOption, error) {
	b.invoker = Blow(ctx, conf)
	b.invoker.Run(ctx, conf, true)
	return &berf.BenchOption{
		NoReport: b.invoker.opt.printOption > 0,
	}, nil
}

func (b *Bench) Invoke(ctx context.Context, conf *berf.Config) (*berf.Result, error) {
	return b.invoker.Run(ctx, conf, false)
}

type Opt struct {
	berfConfig    *berf.Config
	logf          *internal.LogFile
	bodyLinesChan chan string
	url           string
	upload        string

	rootCert string
	certPath string
	keyPath  string

	method  string
	network string

	auth        string
	saveRandDir string
	statusName  string

	socks5Proxy    string
	bodyStreamFile string

	tlcpCerts string
	bodyBytes []byte

	profiles []*internal.Profile

	headers []string

	doTimeout    time.Duration
	verbose      int
	readTimeout  time.Duration
	writeTimeout time.Duration
	dialTimeout  time.Duration

	maxConns    int
	jsonBody    bool
	eval        bool
	uploadIndex bool
	enableGzip  bool

	printOption uint8
	form        bool
	noKeepalive bool

	tlsVerify bool
	pretty    bool
}

func (o *Opt) HasPrintOption(feature uint8) bool {
	return o.printOption&feature == feature
}

func (o *Opt) MaybePost() bool {
	return o.upload != "" || len(o.bodyBytes) > 0 || o.bodyStreamFile != "" || o.bodyLinesChan != nil
}

func TryStartAsBlow() bool {
	if !IsBlowEnv() {
		return false
	}

	berf.StartBench(context.Background(),
		&Bench{},
		berf.WithOkStatus(ss.Or(*pStatusName, "200")),
		berf.WithCounting("Connections"))
	return true
}

func IsBlowEnv() bool {
	if *pURL != "" {
		return true
	}

	if isBlow := len(*pProfiles) > 0; isBlow {
		return true
	}

	return parseUrlFromArgs() != ""
}

func parseUrlFromArgs() string {
	if args := excludeHttpieLikeArgs(fla9.Args()); len(args) > 0 {
		urlAddr := rest.FixURI(args[0])
		if urlAddr.OK() {
			return urlAddr.Data.String()
		}
	}

	return ""
}

func Blow(ctx context.Context, conf *berf.Config) *Invoker {
	urlAddr := *pURL
	if urlAddr == "" {
		urlAddr = parseUrlFromArgs()
	}

	stream := strings.HasSuffix(*pBody, ":stream")
	if stream {
		*pBody = strings.TrimSuffix(*pBody, ":stream")
	}
	lineMode := strings.HasSuffix(*pBody, ":line")
	if lineMode {
		*pBody = strings.TrimSuffix(*pBody, ":line")
	}
	bodyStreamFile, bodyBytes, linesChan := internal.ParseBodyArg(*pBody, stream, lineMode)
	cert, key := ss.Split2(*pCertKey)

	opts := util.NewFeatures(*pOpts...)

	timeout, err := parseDurations(*pTimeout)
	if err != nil {
		log.Fatal(err.Error())
	}

	opt := &Opt{
		url:            urlAddr,
		method:         *pMethod,
		headers:        *pHeaders,
		bodyLinesChan:  linesChan,
		bodyBytes:      bodyBytes,
		bodyStreamFile: bodyStreamFile,
		upload:         *pUpload,

		rootCert:  *pRootCert,
		certPath:  cert,
		keyPath:   key,
		tlcpCerts: *pTlcpCerts,
		tlsVerify: opts.HasAny("tlsVerify"),

		doTimeout:    timeout.Get("do"),
		readTimeout:  timeout.Get("read", "r"),
		writeTimeout: timeout.Get("write", "w"),
		dialTimeout:  timeout.Get("dial", "d"),

		socks5Proxy: *pSocks5,

		network:  *pNetwork,
		auth:     *pAuth,
		maxConns: conf.Goroutines,

		enableGzip:  opts.HasAny("g", "gzip"),
		uploadIndex: opts.HasAny("uploadIndex", "ui"),
		noKeepalive: opts.HasAny("no-keepalive", "no-ka"),
		form:        opts.HasAny("form"),
		pretty:      opts.HasAny("pretty"),
		eval:        opts.HasAny("eval"),
		jsonBody:    opts.HasAny("json"),
		saveRandDir: opts.Get("saveRandDir"),
		verbose:     conf.Verbose,
		statusName:  *pStatusName,
		printOption: parsePrintOption(*pPrint),
		berfConfig:  conf,
	}

	if opts.HasAny("notty") {
		hasStdoutDevice = false
	}

	opt.logf = internal.CreateLogFile(opt.verbose, opt.printOption)
	opt.profiles = internal.ParseProfileArg(*pProfiles, *pEnv)
	invoker, err := NewInvoker(ctx, opt)
	osx.ExitIfErr(err)
	return invoker
}

type Durations struct {
	Map     map[string]time.Duration
	Default time.Duration
}

func (d *Durations) Get(keys ...string) time.Duration {
	for _, key := range keys {
		if v, ok := d.Map[strings.ToLower(key)]; ok {
			return v
		}
	}
	return d.Default
}

// parseDurations parses expression like do:5s,dial:5s,write:5s,read:5s to Durations struct.
func parseDurations(s string) (*Durations, error) {
	d := &Durations{Map: make(map[string]time.Duration)}
	var err error
	for _, one := range ss.Split(s, ss.WithSeps(","), ss.WithTrimSpace(true), ss.WithIgnoreEmpty(true)) {
		if p := strings.IndexRune(one, ':'); p > 0 {
			k, v := strings.TrimSpace(one[:p]), strings.TrimSpace(one[p+1:])
			d.Map[strings.ToLower(k)], err = time.ParseDuration(v)
			if err != nil {
				return nil, fmt.Errorf("failed to parse expressions %s, err: %w", s, err)
			}
		} else {
			if d.Default, err = time.ParseDuration(one); err != nil {
				return nil, fmt.Errorf("failed to parse expressions %s, err: %w", s, err)
			}
		}
	}

	return d, nil
}
