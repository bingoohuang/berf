package blow

import (
	"bytes"
	"encoding/json"
	"fmt"
	"os"
	"strings"

	"github.com/bingoohuang/berf"
	"github.com/bingoohuang/jj"
)

const (
	Gray = uint8(iota + 90)
	_    // Red
	Green
	_ // Yellow
	_ // Blue
	Magenta
	Cyan
	_ // White

	EndColor = "\033[0m"
)

var hasStdoutDevice = func() bool {
	fi, err := os.Stdout.Stat()
	if err != nil {
		return false
	}
	return fi.Mode()&os.ModeDevice == os.ModeDevice
}()

func Color(str string, color uint8) string {
	if !hasStdoutDevice {
		return str
	}

	return fmt.Sprintf("%s%s%s", ColorStart(color), str, EndColor)
}

func ColorStart(color uint8) string {
	return fmt.Sprintf("\033[%dm", color)
}

func ColorfulHeader(str string) string {
	if !hasStdoutDevice {
		return str
	}

	lines := strings.Split(str, "\n")
	firstLineProcessed := false
	for i, line := range lines {
		if strings.HasPrefix(line, "#") {
			continue
		}

		if !firstLineProcessed {
			firstLineProcessed = true

			strs := strings.Split(line, " ")
			strs[0] = Color(strs[0], Magenta)
			if len(strs) > 1 {
				strs[1] = Color(strs[1], Cyan)
			}
			if len(strs) > 2 {
				strs[2] = Color(strs[2], Magenta)
			}
			lines[i] = strings.Join(strs, " ")
			continue
		}

		substr := strings.Split(line, ":")
		if len(substr) < 2 {
			continue
		}
		substr[0] = Color(substr[0], Gray)
		substr[1] = Color(strings.Join(substr[1:], ":"), Cyan)
		lines[i] = strings.Join(substr[:2], ":")
	}
	return strings.Join(lines, "\n")
}

func ColorfulResponse(str string, isJSON bool, pretty bool) string {
	if isJSON {
		return colorJSON(str, pretty)
	}

	return ColorfulHTML(str)
}

func ColorfulHTML(str string) string {
	return Color(str, Green)
}

func formatResponseBody(body []byte, pretty, hasDevice bool) string {
	return formatBytes(body, pretty, hasDevice)
}

func formatBytes(body []byte, pretty, hasDevice bool) string {
	isJSON := json.Valid(body)
	if pretty && isJSON {
		var output bytes.Buffer
		if err := json.Indent(&output, body, "", "  "); err == nil {
			body = output.Bytes()
		}
	}

	if hasDevice {
		return ColorfulResponse(string(body), isJSON, pretty)
	}

	return string(body)
}

func colorJSON(v string, pretty bool) string {
	if !berf.IsStdoutTerminal {
		return v
	}

	p := strings.Index(v, "{")
	if p < 0 {
		p = strings.Index(v, "[")
	}

	if p < 0 {
		return v
	}

	s2 := v[p:]
	q := jj.StreamParse([]byte(s2), nil)
	if q < 0 {
		q = -q
	}
	if q > 0 {
		s := []byte(v[p : p+q])
		if pretty {
			s = jj.Pretty(s)
		}

		if hasStdoutDevice {
			s = jj.Color(s, nil, nil)
		}
		return v[:p] + string(s) + colorJSON(v[p+q:], pretty)
	}

	return v
}
