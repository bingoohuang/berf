package internal

import (
	"io"
	"strings"

	"github.com/bingoohuang/gg/pkg/ss"

	"github.com/bingoohuang/gg/pkg/filex"
	"github.com/bingoohuang/gg/pkg/fla9"

	"github.com/valyala/fasthttp"
	"go.uber.org/multierr"
)

func Quoted(s, open, close string) (string, bool) {
	p1 := strings.Index(s, open)
	if p1 != 0 {
		return "", false
	}

	s = s[len(open):]
	if !strings.HasSuffix(s, close) {
		return "", false
	}

	return strings.TrimSuffix(s, close), true
}

type Closers []io.Closer

func (closers Closers) Close() (err error) {
	for _, c := range closers {
		err = multierr.Append(err, c.Close())
	}

	return
}

// SetHeader set request header if value is not empty.
func SetHeader(r *fasthttp.Request, header, value string) {
	if value != "" {
		r.Header.Set(header, value)
	}
}

func ParseBodyArg(body string, stream bool) (streamFileName string, bodyBytes []byte) {
	if strings.HasPrefix(body, "@") {
		streamFileName = (body)[1:]
		if !filex.Exists(streamFileName) {
			return "", []byte(body)
		}
	}

	streamFileName, bodyBytes = fla9.ParseFileArg(body)
	return ss.If(stream, streamFileName, ""), bodyBytes
}
