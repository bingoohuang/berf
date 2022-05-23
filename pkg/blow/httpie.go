package blow

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"io/ioutil"
	"log"
	"mime/multipart"
	"net"
	"net/url"
	"os"
	"regexp"
	"strings"

	"github.com/bingoohuang/gg/pkg/ss"

	"github.com/bingoohuang/gg/pkg/iox"
	"github.com/bingoohuang/gg/pkg/rest"
)

var keyReq = regexp.MustCompile(`^([\d\w_.\-]+)(==|:=|=|:|@)(.*)`)

type Pair struct {
	V1, V2 string
}

type HttpieArg struct {
	header  map[string]string
	jsonmap map[string]interface{} // post json
	query   []Pair                 // get query
	param   map[string]string      // post json/form or get query
	files   map[string]string      // post multipart
}

func (a *HttpieArg) MaybePost() bool {
	return len(a.jsonmap) > 0 || len(a.files) > 0 || len(a.param) > 0
}

type HttpieArgBody struct {
	Multipart   bool
	ContentType string
	Body        io.ReadCloser

	BodyString string
}

func (a *HttpieArg) SetJsonMap(k string, v interface{})    { a.jsonmap[k] = v }
func (a *HttpieArg) SetParam(k, v string)                  { a.param[k] = v }
func (a *HttpieArg) AddQuery(k, v string)                  { a.query = append(a.query, Pair{V1: k, V2: v}) }
func (a *HttpieArg) SetHeader(k, v string)                 { a.header[k] = v }
func (a *HttpieArg) SetPostFile(formname, filename string) { a.files[formname] = filename }

func excludeHttpieLikeArgs(args []string) []string {
	var remains []string

	for _, arg := range args {
		if submatch := keyReq.FindStringSubmatch(arg); len(submatch) > 0 {
			if urlAddr := rest.FixURI(arg); urlAddr.OK() {
				remains = append(remains, arg)
			}
		} else {
			remains = append(remains, arg)
		}
	}

	return remains
}

func parseHttpieLikeArgs(args []string) (pieArg HttpieArg) {
	pieArg = HttpieArg{
		header:  map[string]string{},
		jsonmap: map[string]interface{}{},
		param:   map[string]string{},
		files:   map[string]string{},
	}
	pieArg.SetHeader("Accept-Encoding", "gzip, deflate")
	// https://httpie.io/docs#request-items
	// Item Type	Description
	// HTTP Headers Name:Value	Arbitrary HTTP header, e.g. X-API-Token:123
	// URL parameters name==value	Appends the given name/value pair as a querystring parameter to the URL. The == separator is used.
	// Data Fields field=value, field=@file.txt	Request data fields to be serialized as a JSON object (default), to be form-encoded (with --form, -f), or to be serialized as multipart/form-data (with --multipart)
	// Raw JSON fields field:=json	Useful when sending JSON and one or more fields need to be a Boolean, Number, nested Object, or an Array, e.g., meals:='["ham","spam"]' or pies:=[1,2,3] (note the quotes)
	// File upload fields field@/dir/file, field@file;type=mime	Only available with --form, -f and --multipart. For example screenshot@~/Pictures/img.png, or 'cv@cv.txt;type=text/markdown'. With --form, the presence of a file field results in a --multipart request
	for _, arg := range args {
		if ss.HasPrefix(arg, "http://", "https://") {
			continue
		}

		subs := keyReq.FindStringSubmatch(arg)
		if len(subs) == 0 {
			continue
		}

		switch k, op, v := subs[1], subs[2], subs[3]; op {
		case ":=": // Json raws
			if v, fn, err := readFile(v); err != nil {
				log.Fatalf("Read File %s failed: %v", fn, err)
			} else if fn != "" {
				var j interface{}
				if err := json.Unmarshal(v, &j); err != nil {
					log.Fatalf("Unmarshal File %s failed: %v", fn, err)
				}
				pieArg.SetJsonMap(k, j)
			} else {
				pieArg.SetJsonMap(k, json.RawMessage(v))
			}
		case "==": // Queries
			pieArg.AddQuery(k, tryReadFile(v))
		case "=": // Params
			pieArg.SetParam(k, tryReadFile(v))
		case ":": // Headers
			if ip := net.ParseIP(k); ip == nil {
				pieArg.SetHeader(k, v)
			}
		case "@": // files
			pieArg.SetPostFile(k, v)
		}
	}

	return
}

func tryReadFile(s string) string {
	if v, _, err := readFile(s); err != nil {
		// log.Fatalf("Read File %s failed: %v", s, err)
		return s
	} else {
		return string(v)
	}
}

func readFile(s string) (data []byte, fn string, e error) {
	if !strings.HasPrefix(s, "@") {
		return []byte(s), "", nil
	}

	s = strings.TrimPrefix(s, "@")
	f, err := os.Open(s)
	if err != nil {
		return nil, s, err
	}
	defer iox.Close(f)
	content, err := io.ReadAll(f)
	if err != nil {
		return nil, s, err
	}
	return content, s, nil
}

func (a *HttpieArg) Build(method string, form bool) *HttpieArgBody {
	b := &HttpieArgBody{}

	switch method {
	case "POST", "PUT", "PATCH":
	default:
		return b
	}

	if len(a.files) > 0 {
		pr, pw := io.Pipe()
		bodyWriter := multipart.NewWriter(pw)
		go func() {
			for formName, filename := range a.files {
				fileWriter, err := bodyWriter.CreateFormFile(formName, filename)
				if err != nil {
					log.Fatal(err)
				}
				fh, err := os.Open(filename)
				if err != nil {
					log.Fatal(err)
				}
				_, err = io.Copy(fileWriter, fh)
				iox.Close(fh)
				if err != nil {
					log.Fatal(err)
				}
			}
			for k, v := range a.param {
				_ = bodyWriter.WriteField(k, v)
			}
			iox.Close(bodyWriter)
			iox.Close(pw)
		}()
		b.Multipart = true
		b.ContentType = bodyWriter.FormDataContentType()
		b.Body = ioutil.NopCloser(pr)
		return b
	}

	if len(a.jsonmap) > 0 || len(a.param) > 0 {
		m := make(map[string]interface{})
		for k, v := range a.param {
			m[k] = v
		}
		for k, v := range a.jsonmap {
			m[k] = v
		}

		if form {
			b.BodyString = createParamBody(m)
			b.ContentType = "application/x-www-form-urlencoded"
		} else {
			buf := bytes.NewBuffer(nil)
			enc := json.NewEncoder(buf)
			if err := enc.Encode(m); err != nil {
				log.Fatalf("failed to json encoding, err: %v", err)
			}
			b.BodyString = buf.String()
			b.ContentType = "application/json; charset=utf-8"
		}
		return b
	}

	return b
}

func createParamBody(params map[string]interface{}) string {
	b := make(url.Values)
	for k, v := range params {
		switch vv := v.(type) {
		case string:
			b.Add(k, vv)
		default:
			b.Add(k, fmt.Sprintf("%v", v))
		}
	}

	return b.Encode()
}
