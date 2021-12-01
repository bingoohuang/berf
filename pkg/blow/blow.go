package blow

import (
	"context"
	"os"
	"strings"
	"time"

	"github.com/bingoohuang/gg/pkg/rest"

	"github.com/bingoohuang/berf/pkg/blow/internal"

	"github.com/bingoohuang/berf"
	"github.com/bingoohuang/gg/pkg/fla9"
	"github.com/bingoohuang/gg/pkg/osx"
	"github.com/bingoohuang/gg/pkg/ss"
)

func init() {
	fla9.EnvPrefix = "BLOW"
}

var (
	pURL             = fla9.String("url", "", "URL")
	pBody            = fla9.String("body,b", "", "HTTP request body, or @file to read from")
	pUpload          = fla9.String("upload,u", "", "HTTP upload multipart form file or directory, or add prefix file: to set form field name ")
	pStream          = fla9.Bool("stream", false, "Specify stream file specified by '--body @file' using chunked encoding")
	pMethod          = fla9.String("method,m", "", "HTTP method")
	pNetwork         = fla9.String("network", "", "Network simulation, local: simulates local network, lan: local, wan: wide, bad: bad network, or BPS:latency like 20M:20ms")
	pHeaders         = fla9.Strings("header,H", nil, "Custom HTTP headers, K:V")
	pProfileArg      = fla9.Strings("profile,P", nil, "Profile file, append :new to create a demo profile, or :tag to run only specified profile")
	pEnableGzip      = fla9.Bool("gzip", false, "Enabled gzip if gzipped content is less more")
	pBasicAuth       = fla9.String("basic", "", "basic auth username:password")
	pContentType     = fla9.String("content,T", "", "Content-Type header")
	pCertKey         = fla9.String("cert", "", "Path to the client's TLS Cert and private key file, eg. ca.pem,ca.key")
	pInsecure        = fla9.Bool("insecure,k", false, "Controls whether a client verifies the server's certificate chain and host name")
	pTimeout         = fla9.Duration("timeout", 0, "Timeout for each http request")
	pDialTimeout     = fla9.Duration("dial-timeout", 0, "Timeout for dial addr")
	pReqWriteTimeout = fla9.Duration("req-timeout", 0, "Timeout for full request writing")
	pRspReadTimeout  = fla9.Duration("rsp-timeout", 0, "Timeout for full response reading")
	pSocks5          = fla9.String("socks5", "", "Socks5 proxy, ip:port")
	pStatusName      = fla9.String("status", "", "Status name in json, like resultCode")
)

func StatusName() string { return *pStatusName }

type Bench struct {
	invoker *Invoker
}

func (b *Bench) Name(context.Context, *berf.Config) string {
	opt := b.invoker.clientOpt
	if v := opt.url; v != "" {
		return v
	}

	return "profiles " + strings.Join(*pProfileArg, ",")
}

func (b *Bench) Final(_ context.Context, conf *berf.Config) error {
	opt := b.invoker.clientOpt
	if conf.N == 1 && opt.logf != nil {
		if v := opt.logf.GetLastLog(); v != "" {
			os.Stdout.WriteString(v)
		}
	}
	return nil
}

func (b *Bench) Init(ctx context.Context, conf *berf.Config) error {
	b.invoker = Blow(ctx, conf)
	return nil
}

func (b *Bench) Invoke(context.Context, *berf.Config) (*berf.Result, error) {
	return b.invoker.Run()
}

type ClientOpt struct {
	url       string
	method    string
	headers   []string
	bodyBytes []byte
	bodyFile  string

	certPath string
	keyPath  string

	insecure bool

	doTimeout    time.Duration
	readTimeout  time.Duration
	writeTimeout time.Duration
	dialTimeout  time.Duration

	socks5Proxy string
	contentType string
	upload      string

	basicAuth  string
	network    string
	logf       *internal.LogFile
	maxConns   int
	enableGzip bool
	verbose    int
	profiles   []*internal.Profile
	statusName string
}

func IsBlowEnv() bool {
	if *pURL != "" {
		return true
	}

	if isBlow := len(*pProfileArg) > 0; isBlow {
		return true
	}

	if len(fla9.Args()) == 1 {
		if _, err := rest.FixURI(fla9.Args()[0]); err == nil {
			return true
		}
	}

	return false
}

func Blow(ctx context.Context, conf *berf.Config) *Invoker {
	urlAddr := *pURL

	if len(fla9.Args()) > 0 {
		urlAddr, _ = rest.FixURI(fla9.Args()[0])
	}

	bodyFile, bodyBytes := internal.ParseBodyArg(*pBody, *pStream)
	cert, key := ss.Split2(*pCertKey)

	opt := &ClientOpt{
		url:       urlAddr,
		method:    *pMethod,
		headers:   *pHeaders,
		bodyBytes: bodyBytes,
		bodyFile:  bodyFile,
		upload:    *pUpload,

		certPath: cert,
		keyPath:  key,
		insecure: *pInsecure,

		doTimeout:    *pTimeout,
		readTimeout:  *pRspReadTimeout,
		writeTimeout: *pReqWriteTimeout,
		dialTimeout:  *pDialTimeout,

		socks5Proxy: *pSocks5,
		contentType: *pContentType,

		network:   *pNetwork,
		basicAuth: *pBasicAuth,
		maxConns:  conf.Goroutines,

		enableGzip: *pEnableGzip,
		verbose:    conf.Verbose,
		statusName: *pStatusName,
	}

	opt.logf = internal.CreateLogFile(opt.verbose)

	opt.profiles = internal.ParseProfileArg(*pProfileArg)
	invoker, err := NewInvoker(ctx, opt)
	osx.ExitIfErr(err)
	return invoker
}
