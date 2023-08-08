package blow

import (
	"context"
	_ "embed"
	"fmt"
	"io"
	"log"
	"net/url"
	"os"
	"strings"
	"time"

	"github.com/bingoohuang/berf"
	"github.com/bingoohuang/berf/pkg/blow/internal"
	"github.com/bingoohuang/berf/pkg/util"
	"github.com/bingoohuang/gg/pkg/filex"
	"github.com/bingoohuang/gg/pkg/fla9"
	"github.com/bingoohuang/gg/pkg/osx"
	"github.com/bingoohuang/gg/pkg/rest"
	"github.com/bingoohuang/gg/pkg/ss"
)

var (
	pUrls   = fla9.Strings("url", nil, "URL")
	pBody   = fla9.String("body,b", "", "HTTP request body, or @file to read from, or @file:stream to enable chunked encoding for the file, or @file:line to read line by line")
	pUpload = fla9.String("upload,u", "", "HTTP upload multipart form file or directory or glob pattern like ./*.jpg, \n"+
		"      prefix file: to set form field name\n"+
		"      extension: rand.png,rand.art,rand.jpg,rand.json\n"+
		"      env export UPLOAD_INDEX=%clear%y%M%d.%H%m%s.%i%ext to append index to the file base name, \n"+
		"                                %clear: 清除原始文件名\n"+
		"                                %y: 4位年 %M: 2位月 %d: 2位日 %H: 2位时 %m: 2位分 %s: 2位秒\n"+
		"                                %i: 自增长序号, %05i： 补齐5位的自增长序号（前缀补0)\n"+
		"      env export UPLOAD_EXIT=1 to exit when all files are uploaded\n"+
		"      env export UPLOAD_SHUFFLE=1 to shuffle the upload files (only for glob pattern)")
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
	pDir        = fla9.String("dir", "", "download dir, use :temp for temp dir")
	pCertKey    = fla9.String("cert", "", "Path to the client's TLS Cert and private key file, eg. ca.pem,ca.key")
	pRootCert   = fla9.String("root-ca", "", "Ca root certificate file to verify TLS")
	pTlcpCerts  = fla9.String("tlcp-certs", "", "format: sign.cert.pem,sign.key.pem,enc.cert.pem,enc.key.pem")
	pTimeout    = fla9.String("timeout", "", "Timeout for each http request, e.g. 5s for do:5s,dial:5s,write:5s,read:5s")
	pPrint      = fla9.String("print,p", "", "a: all, R: req all, H: req headers, B: req body, r: resp all, h: resp headers b: resp body c: status code")
	pStatusName = fla9.String("status", "", "Status name in json, like resultCode")

	pCreateEnvFile = fla9.Bool("demo.env", false, "create a demo .env in current dir.\n"+
		"       env LOCAL_IP       指定网卡IP, e.g. LOCAL_IP=192.168.1.2 berf ...\n"+
		"       env TLCP           使用传输层密码协议(TLCP)，遵循《GB/T 38636-2020 信息安全技术 传输层密码协议》, e.g. TLCP=1 berf ...\n")
)

const (
	printReqHeader uint8 = 1 << iota
	printReqBody
	printRespOption
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
		"o": printRespOption,
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
	if v := opt.urls; len(v) > 0 {
		return strings.Join(v, ",")
	}

	return "profiles " + strings.Join(*pProfiles, ",")
}

func (b *Bench) Final(_ context.Context, conf *berf.Config) error {
	opt := b.invoker.opt

	if opt.logf != nil {
		defer func() {
			opt.logf.Close()
			// 关闭日志文件后，如果日志文件是仅仅由于 -n1 引起，则删除之
			if conf.Verbose == 0 {
				opt.logf.Remove()
			}
		}()
	}

	if conf.N == 1 && opt.logf != nil && opt.printOption == 0 {
		if v := opt.logf.GetLastLog(); v != "" {
			v = colorJSON(v, opt.pretty)
			_, _ = os.Stdout.WriteString(v)
		}
	}
	return nil
}

//go:embed .env
var envFileDemo []byte

func (b *Bench) Init(ctx context.Context, conf *berf.Config) (*berf.BenchOption, error) {
	if *pCreateEnvFile {
		return nil, b.createEnvFileDemo()
	}

	b.invoker = Blow(ctx, conf)
	b.invoker.Run(ctx, conf, true)
	return &berf.BenchOption{
		NoReport: b.invoker.opt.printOption > 0,
	}, nil
}

func (b *Bench) createEnvFileDemo() error {
	if filex.Exists(".env") {
		return fmt.Errorf(".env file already exists, please remove or rename it first")
	}

	if err := os.WriteFile(".env", envFileDemo, 0o644); err != nil {
		return fmt.Errorf("create .env file: %w", err)
	}

	log.Printf(".env file created")

	return io.EOF
}

func (b *Bench) Invoke(ctx context.Context, conf *berf.Config) (*berf.Result, error) {
	return b.invoker.Run(ctx, conf, false)
}

type Opt struct {
	berfConfig    *berf.Config
	logf          *internal.LogFile
	bodyLinesChan chan string
	urls          []string
	parsedUrls    []*url.URL
	upload        string

	rootCert string
	certPath string
	keyPath  string

	downloadDir string

	method  string
	network string

	auth        string
	saveRandDir string
	statusName  string

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
	ant         bool

	printOption uint8
	form        bool
	noKeepalive bool

	tlsVerify          bool
	pretty             bool
	noTLSessionTickets bool
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

	StartBlow()
	return true
}

func StartBlow() {
	berf.StartBench(context.Background(),
		&Bench{},
		berf.WithOkStatus(ss.Or(*pStatusName, "200")),
		berf.WithCounting("Connections"))
}

func IsBlowEnv() bool {
	if len(*pUrls) > 0 {
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
	var urlAddrs []string
	urlAddrs = append(urlAddrs, *pUrls...)
	if len(urlAddrs) == 0 {
		if urlAddr := parseUrlFromArgs(); urlAddr != "" {
			urlAddrs = append(urlAddrs, urlAddr)
		}
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
		urls:           urlAddrs,
		method:         *pMethod,
		headers:        *pHeaders,
		bodyLinesChan:  linesChan,
		bodyBytes:      bodyBytes,
		bodyStreamFile: bodyStreamFile,
		upload:         *pUpload,

		rootCert:    *pRootCert,
		certPath:    cert,
		keyPath:     key,
		tlcpCerts:   *pTlcpCerts,
		tlsVerify:   opts.HasAny("tlsVerify"),
		downloadDir: *pDir,

		doTimeout:    timeout.Get("do"),
		readTimeout:  timeout.Get("read", "r"),
		writeTimeout: timeout.Get("write", "w"),
		dialTimeout:  timeout.Get("dial", "d"),

		network:  *pNetwork,
		auth:     *pAuth,
		maxConns: conf.Goroutines,

		enableGzip:         opts.HasAny("g", "gzip"),
		uploadIndex:        opts.HasAny("uploadIndex", "ui"),
		noKeepalive:        opts.HasAny("no-keepalive", "no-ka"),
		noTLSessionTickets: opts.HasAny("no-tls-session-tickets", "no-tst"),
		form:               opts.HasAny("form"),
		pretty:             opts.HasAny("pretty"),
		eval:               opts.HasAny("eval"),
		jsonBody:           opts.HasAny("json"),
		ant:                opts.HasAny("ant"),
		saveRandDir:        opts.Get("saveRandDir"),
		verbose:            conf.Verbose,
		statusName:         *pStatusName,
		printOption:        parsePrintOption(*pPrint),
		berfConfig:         conf,
	}

	if opt.downloadDir == ":temp" {
		opt.downloadDir = os.TempDir()
	}

	if opts.HasAny("notty") {
		hasStdoutDevice = false
	}

	opt.logf = internal.CreateLogFile(opt.verbose, conf.N)
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
