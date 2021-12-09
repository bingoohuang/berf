package blow

import (
	"context"
	"fmt"
	"github.com/bingoohuang/jj"
	"log"
	"os"
	"strings"
	"time"

	"github.com/bingoohuang/berf/pkg/util"

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
	pURL        = fla9.String("url", "", "URL")
	pBody       = fla9.String("body,b", "", "HTTP request body, or @file to read from, or @file:stream to enable chunked encoding for the file")
	pUpload     = fla9.String("upload,u", "", "HTTP upload multipart form file or directory, or add prefix file: to set form field name ")
	pMethod     = fla9.String("method,m", "", "HTTP method")
	pNetwork    = fla9.String("network", "", "Network simulation, local: simulates local network, lan: local, wan: wide, bad: bad network, or BPS:latency like 20M:20ms")
	pHeaders    = fla9.Strings("header,H", nil, "Custom HTTP headers, K:V, e.g. Content-Type")
	pProfiles   = fla9.Strings("profile,P", nil, "Profile file, append :new to create a demo profile, or :tag to run only specified profile")
	pOpts       = fla9.Strings("opt", nil, "Options. gzip: Enabled content gzip, k: not verify the server's cert chain and host name")
	pBasicAuth  = fla9.String("basic", "", "basic auth username:password")
	pCertKey    = fla9.String("cert", "", "Path to the client's TLS Cert and private key file, eg. ca.pem,ca.key")
	pTimeout    = fla9.String("timeout", "", "Timeout for each http request, e.g. 5s for do:5s,dial:5s,write:5s,read:5s")
	pSocks5     = fla9.String("socks5", "", "Socks5 proxy, ip:port")
	pStatusName = fla9.String("status", "", "Status name in json, like resultCode")
	pPretty     = fla9.Bool("pretty", false, "Pretty JSON output")
)

func StatusName() string { return *pStatusName }

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
	if conf.N == 1 && opt.logf != nil {
		if v := opt.logf.GetLastLog(); v != "" {
			v = colorJSON(v, *pPretty)
			os.Stdout.WriteString(v)
		}
	}
	return nil
}

func colorJSON(v string, pretty bool) string {
	p := strings.Index(v, "\n{")
	if p < 0 {
		p = strings.Index(v, "\n[")
	}

	if p < 0 {
		return v
	}

	s2 := v[p+1:]
	q := jj.StreamParse([]byte(s2), nil)
	if q < 0 {
		q = -q
	}
	if q > 0 {
		s := []byte(v[p+1 : p+1+q])
		if pretty {
			s = jj.Pretty(s)
		}
		s = jj.Color(s, nil)
		return v[:p+1] + string(s) + colorJSON(v[p+1+q:], pretty)
	}

	return v
}

func (b *Bench) Init(ctx context.Context, conf *berf.Config) error {
	b.invoker = Blow(ctx, conf)
	return nil
}

func (b *Bench) Invoke(context.Context, *berf.Config) (*berf.Result, error) {
	return b.invoker.Run()
}

type Opt struct {
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

	if isBlow := len(*pProfiles) > 0; isBlow {
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

	stream := strings.HasSuffix(*pBody, ":stream")
	if stream {
		*pBody = strings.TrimSuffix(*pBody, ":stream")
	}
	bodyFile, bodyBytes := internal.ParseBodyArg(*pBody, stream)
	cert, key := ss.Split2(*pCertKey)

	opts := util.NewFeatures(*pOpts...)

	timeout, err := parseDurations(*pTimeout)
	if err != nil {
		log.Fatal(err.Error())
	}

	opt := &Opt{
		url:       urlAddr,
		method:    *pMethod,
		headers:   *pHeaders,
		bodyBytes: bodyBytes,
		bodyFile:  bodyFile,
		upload:    *pUpload,

		certPath: cert,
		keyPath:  key,
		insecure: opts.HasAny("k", "insecure"),

		doTimeout:    timeout.Get("do"),
		readTimeout:  timeout.Get("read", "r"),
		writeTimeout: timeout.Get("write", "w"),
		dialTimeout:  timeout.Get("dial", "d"),

		socks5Proxy: *pSocks5,

		network:   *pNetwork,
		basicAuth: *pBasicAuth,
		maxConns:  conf.Goroutines,

		enableGzip: opts.HasAny("g", "gzip"),
		verbose:    conf.Verbose,
		statusName: *pStatusName,
	}

	opt.logf = internal.CreateLogFile(opt.verbose)

	opt.profiles = internal.ParseProfileArg(*pProfiles)
	invoker, err := NewInvoker(ctx, opt)
	osx.ExitIfErr(err)
	return invoker
}

type Durations struct {
	Default time.Duration
	Map     map[string]time.Duration
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
