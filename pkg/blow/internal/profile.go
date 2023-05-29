package internal

import (
	"bufio"
	_ "embed"
	"encoding/base64"
	"errors"
	"io"
	"net/textproto"
	"net/url"
	"os"
	"regexp"
	"strings"
	"unicode"

	"github.com/bingoohuang/gg/pkg/gz"
	"github.com/bingoohuang/gg/pkg/iox"
	"github.com/bingoohuang/gg/pkg/rest"
	"github.com/bingoohuang/jj"
	"github.com/valyala/fasthttp"
)

func (p *Profile) CreateReq(isTLS bool, req *fasthttp.Request, enableGzip, uploadIndex bool) (Closers, error) {
	p.requestHeader.CopyTo(&req.Header)
	if !p.Init && p.Eval {
		req.Header.SetRequestURI(Gen(p.URL, IgnoreJSON))
	}

	if isTLS {
		req.URI().SetScheme("https")
		req.URI().SetHostBytes(req.Header.Host())
	}

	const acceptEncodingHeader = "Accept-Encoding"
	if enableGzip && p.Header[acceptEncodingHeader] != "" {
		req.Header.Set(acceptEncodingHeader, "gzip")
	}

	if p.bodyFileName != "" {
		file, err := os.Open(p.bodyFileName)
		if err != nil {
			return nil, err
		}
		req.SetBodyStream(file, -1)
		return []io.Closer{file}, nil
	}

	var bodyBytes []byte
	if len(p.bodyFileData) > 0 {
		bodyBytes = p.bodyFileData
	}
	if p.Body != "" {
		bodyBytes = []byte(p.Body)
	}

	if len(bodyBytes) > 0 {
		if p.Eval {
			bodyBytes = []byte(Gen(string(bodyBytes), If(p.JsonBody, SureJSON, MayJSON)))
		}

		bodyBytes = []byte(p.EnvVars.Eval(string(bodyBytes)))

		if enableGzip {
			if v, err := gz.Gzip(bodyBytes); err == nil && len(v) < len(p.bodyFileData) {
				bodyBytes = v
				req.Header.Set("Content-Encoding", "gzip")
			}
		}

		req.SetBodyRaw(bodyBytes)
		return nil, nil
	}

	for k, v := range p.Form {
		// 先处理，只上传一个文件的情形
		if strings.HasPrefix(v, "@") {
			fr := &fileReader{File: v[1:], uploadFileField: k}
			uv := fr.Read(false)
			data := uv.Data()
			multi := data.CreateFileField(k, uploadIndex)
			for k, v := range multi.Headers {
				SetHeader(req, k, v)
			}
			req.Header.Set("Beefs-Original", data.Payload.Original)
			req.SetBodyStream(multi.NewReader(), int(multi.Size))
			return nil, nil
		}
	}

	return nil, nil
}

func (p *Profile) createHeader() error {
	u, err := url.Parse(p.URL)
	if err != nil {
		return err
	}

	contentType := p.Header[ContentTypeName]
	if contentType == "" {
		contentType = `plain/text; charset=utf-8`
	} else {
		delete(p.Header, ContentTypeName)
	}

	host := u.Host
	if v := p.Header["Host"]; v != "" {
		host = v
		delete(p.Header, "Host")
	}

	p.requestHeader = &fasthttp.RequestHeader{}
	p.requestHeader.SetHost(host)
	p.requestHeader.SetContentType(contentType)
	p.requestHeader.SetMethod(p.Method)
	u.RawQuery = p.makeQuery(u.Query()).Encode()
	p.requestHeader.SetRequestURI(u.RequestURI())

	if v := p.Header["Basic"]; v != "" {
		b := base64.StdEncoding.EncodeToString([]byte(v))
		p.requestHeader.Set("Authorization", "Basic "+b)
		delete(p.Header, "Basic")
	}

	p.requestHeader.Set("Accept", "application/json")
	for k, v := range p.Header {
		p.requestHeader.Set(k, v)
	}

	return nil
}

func (p *Profile) makeQuery(query url.Values) url.Values {
	switch p.Method {
	case "GET", "HEAD", "CONNECT", "OPTIONS", "TRACE":
		for k, v := range p.Form {
			query.Set(k, v)
		}
		for k, v := range p.Query {
			query.Set(k, v)
		}
	}
	return query
}

type Option struct {
	// 从结果 JSON 中 使用 jj.Get 提取值, 参见 demo.http 中写法
	// 例如：result.id=chinaID，表示设置 @id = jj.Get(responseJSON, "chinaID")
	// 一般配合初始化调用使用，例如从登录结果中提取 accessToken 等
	ResultExpr map[string]string `prefix:"result."`
	Tag        string
	Eval       bool
	JsonBody   bool

	// 作为初始化调用，例如登录
	Init bool
}

type Profile struct {
	Option

	requestHeader *fasthttp.RequestHeader
	Query         map[string]string
	RawJSON       map[string]string
	Form          map[string]string
	Header        map[string]string
	EnvVars       EnvVars
	Body          string
	URL           string
	Method        string
	bodyFileName  string
	Comments      []string

	bodyFileData []byte
}

var (
	envRegexp    = regexp.MustCompile(`(?i)\benv:\s*`)
	exportRegexp = regexp.MustCompile(`(?i)^\s*export\s+(\w[\w_\d-]+)\s*=\s*(.+?)\s*$`)
)

// regexFollows tests where s matches reg and then follows the following string.
func regexFollows(reg *regexp.Regexp, s, following string) (isEnv, matchFollower bool) {
	subs := reg.FindAllStringSubmatchIndex(s, -1)
	if len(subs) == 0 {
		return false, false
	}

	s1 := s[subs[0][1]:]
	if !strings.HasSuffix(strings.ToUpper(s1), strings.ToUpper(following)) {
		return true, false
	}

	if following == "" {
		return true, true
	}

	s1 = s1[len(following):]
	if s1 == "" {
		return true, true
	}

	return true, unicode.IsSpace(rune(s1[0]))
}

//go:embed demo.http
var DemoProfile []byte

func ParseProfileFile(fileName string, envName string) ([]*Profile, error) {
	f, err := os.Open(fileName)
	if err != nil {
		panic(err.Error())
	}
	defer iox.Close(f)

	return ParseProfiles(f, envName)
}

func ParseProfiles(r io.Reader, envName string) ([]*Profile, error) {
	buf := bufio.NewReader(r)
	profiles, err := parseRequests(buf, envName)
	if err != nil {
		if errors.Is(err, io.EOF) {
			return profiles, nil
		}

		return nil, err
	}

	return profiles, nil
}

type EnvVars map[string]string

func (e EnvVars) Eval(s string) string {
	for _, element := range os.Environ() {
		parts := strings.Split(element, "=")
		k, v := parts[0], parts[1]
		if _, ok := e[k]; ok {
			v = e[k]
		}
		s = strings.ReplaceAll(s, `${`+k+`}`, v)
	}

	for k, v := range e {
		s = strings.ReplaceAll(s, `${`+k+`}`, v)
	}
	return s
}

func parseRequests(buf *bufio.Reader, envName string) (profiles []*Profile, err error) {
	var p *Profile
	var l string

	envVars := EnvVars{}

	var isEnv, hasFollower bool
	for err == nil || len(l) > 0 {
		if len(l) > 0 {
			if strings.HasPrefix(l, "###") {
				isEnv, hasFollower = regexFollows(envRegexp, l, envName)
			} else if isEnv && hasFollower && !strings.HasPrefix(l, "#") {
				if vars := exportRegexp.FindStringSubmatch(l); len(vars) > 0 {
					envVars[vars[1]] = vars[2]
				}
			}

			if !isEnv {
				if p1 := processLine(p, l, envVars); p1 != p {
					profiles = append(profiles, p1)
					p = p1
				}
			}
		}

		l, err = buf.ReadString('\n')
		l = strings.TrimSpace(l)
	}

	if err = postProcessProfiles(profiles); err != nil {
		return nil, err
	}

	return
}

var tagRegexp = regexp.MustCompile(`\[.+]`)

func postProcessProfiles(profiles []*Profile) error {
	for _, p := range profiles {
		if len(p.Body) > 0 {
			p.bodyFileName, p.bodyFileData, _ = ParseBodyArg(p.Body, false, false)

			if p.Header[ContentTypeName] == "" && jj.Valid(p.Body) {
				p.Header[ContentTypeName] = ContentTypeJSON
			}

			p.JsonBody = p.Header[ContentTypeName] == ContentTypeJSON
		}

		if len(p.Comments) > 0 {
			for _, c := range p.Comments {
				subs := tagRegexp.FindStringSubmatch(c)
				for _, sub := range subs {
					jj.ParseConf(sub[1:len(sub)-1], &p.Option)
				}
			}
		}

		if err := p.createHeader(); err != nil {
			return err
		}
	}
	return nil
}

const (
	ContentTypeName = "Content-Type"
	ContentTypeJSON = "application/json;charset=utf-8"
)

var headerReg = regexp.MustCompile(`(^\w+(?:-\w+)*)(==|:=|=|:|@)\s*(.*)$`)

var lastComments []string

func processLine(p *Profile, l string, envVars EnvVars) *Profile {
	if option, ok := Quoted(l, "[", "]"); ok {
		if p != nil {
			jj.ParseConf(option, &p.Option)
		}
		return p
	}
	if m, ok := hasAnyPrefix(strings.ToUpper(l),
		"GET", "HEAD", "POST", "PUT",
		"PATCH", "DELETE", "CONNECT", "OPTIONS", "TRACE"); ok {
		addr := strings.TrimSpace(l[len(m):])
		addr = envVars.Eval(addr)
		addr = fixUrl("", addr)

		p1 := &Profile{
			Method:   m,
			URL:      addr,
			Comments: lastComments,
			Header:   map[string]string{},
			Query:    map[string]string{},
			Form:     map[string]string{},
			RawJSON:  map[string]string{},
			EnvVars:  envVars,
		}
		lastComments = nil
		return p1
	}

	if strings.HasPrefix(l, "#") { // 遇到注释了
		lastComments = append(lastComments, l)
		return p
	}

	p.Comments = append(p.Comments, lastComments...)
	lastComments = nil

	if len(p.Body) == 0 {
		if subs := headerReg.FindStringSubmatch(l); len(subs) > 0 {
			k, op, v := envVars.Eval(subs[1]), subs[2], envVars.Eval(subs[3])
			// refer https://httpie.io/docs#request-items
			switch op {
			case "==": //  query string parameter
				p.Query[k] = v
			case ":=": // Raw JSON fields
				p.RawJSON[k] = v
			case ":": // Header fields
				ck := textproto.CanonicalMIMEHeaderKey(k)
				p.Header[ck] = v
			case "@":
				// File upload fields: field@/dir/file, field@file;type=mime
				// For example: screenshot@~/Pictures/img.png, cv@cv.txt;type=text/markdown
				// the presence of a file field results in a --multipart request
				p.Form[k] = "@" + v
			case "=":
				// Data Fields field=value, field=@file.txt
				// Request Data fields to be serialized as a JSON object (default),
				// to be form-encoded (with --form, -f),
				// or to be serialized as multipart/form-Data (with --multipart)
				p.Form[k] = v
			}

			return p
		}
	}

	p.Body += l
	return p
}

func hasAnyPrefix(s string, subs ...string) (string, bool) {
	for _, sub := range subs {
		if l := len(sub); len(s) > l && strings.HasPrefix(s, sub) {
			if unicode.IsSpace(rune(s[l])) {
				return sub, true
			}
		}
	}

	return "", false
}

func fixUrl(baseUrl, s string) string {
	if baseUrl != "" {
		return s
	}

	v := rest.FixURI(s, rest.WithFatalErr(true))
	return v.Data.String()
}
