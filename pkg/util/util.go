package util

import (
	"bufio"
	"compress/gzip"
	"fmt"
	"io"
	"io/ioutil"
	"log"
	"os"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/bingoohuang/gg/pkg/iox"

	"github.com/bingoohuang/gg/pkg/osx"

	"go.uber.org/multierr"

	"github.com/bingoohuang/gg/pkg/ss"
)

func ParseEnvDuration(name string, defaultValue time.Duration) time.Duration {
	if e := os.Getenv(name); e != "" {
		if v, err := time.ParseDuration(e); err != nil {
			log.Printf("W! env $%s %s is invalid, default %s is used, error: %v", name, e, defaultValue, err)
		} else if v > 0 {
			return v
		}
	}

	return defaultValue
}

type Float64 float64

func (f Float64) MarshalJSON() ([]byte, error) {
	b := []byte(strconv.FormatFloat(float64(f), 'f', 1, 64))
	i := len(b) - 1
	for ; i >= 0; i-- {
		if b[i] != '0' {
			if b[i] != '.' {
				i++
			}
			break
		}
	}

	return b[:i], nil
}

type SizeUnit int

const (
	KILO SizeUnit = 1000
	MEGA          = 1000 * KILO
	GIGA          = 1000 * MEGA
)

func BytesToGiga(bytes uint64) Float64 {
	return Float64(float64(bytes) / float64(GIGA))
}

func BytesToMEGA(bytes uint64) Float64 {
	return Float64(float64(bytes) / float64(MEGA))
}

func BytesToBPS(bytes uint64, d time.Duration) Float64 {
	return Float64(float64(bytes*8) / float64(MEGA) / d.Seconds())
}

func BytesToMBS(bytes uint64, d time.Duration) Float64 {
	return Float64(float64(bytes) / float64(MEGA) / d.Seconds())
}

func NumberToRate(num uint64, d time.Duration) Float64 {
	return Float64(float64(num) / d.Seconds())
}

type JSONLogFile struct {
	F *os.File
	*sync.Mutex
	Dry     bool
	Closed  bool
	HasRows bool
	Name    string
}

const (
	DrySuffix = ":dry"
	GzSuffix  = ".gz"
)

func IsDrySuffix(file string) bool {
	return strings.HasSuffix(file, DrySuffix) || strings.HasSuffix(file, GzSuffix)
}

func TrimDrySuffix(file string) string {
	return strings.TrimSuffix(file, DrySuffix)
}

func NewJsonLogFile(file string) *JSONLogFile {
	dry := IsDrySuffix(file)
	if file == "" {
		file = "berf_" + time.Now().Format(`200601021504`) + ".log"
	} else if dry {
		file = TrimDrySuffix(file)
	}
	logFile := &JSONLogFile{Name: file, Mutex: &sync.Mutex{}, Dry: dry}

	if dry {
		return logFile
	}

	f, err := os.OpenFile(file, os.O_RDWR|os.O_CREATE, 0o600)
	if err != nil {
		log.Printf("E! Fail to open log file %s error: %v", file, err)
	}
	logFile.F = f
	if n, err := f.Seek(0, io.SeekEnd); err != nil {
		log.Printf("E! fail to seek file %s error: %v", file, err)
	} else if n == 0 {
		_, _ = f.WriteString("[]\n")
	} else {
		logFile.HasRows = true
	}
	return logFile
}

// LogErr1 logs an error.
func LogErr1(err error) {
	if err != nil {
		log.Printf("failed %v", err)
	}
}

func LogErr2[T any](t T, err error) T {
	if err != nil {
		log.Printf("failed %v", err)
	}
	return t
}

func (f JSONLogFile) ReadAll() []byte {
	f.Lock()
	defer f.Unlock()

	if f.F == nil || f.Closed {
		return osx.ReadFile(f.Name, osx.WithFatalOnError(true)).Data
	}

	_, _ = f.F.Seek(0, io.SeekStart)
	defer LogErr2(f.F.Seek(0, io.SeekEnd))

	data, err := ReadFile(f.F)
	if err != nil {
		log.Printf("E! fail to read log file %s, error: %v", f.F.Name(), err)
	}
	return data
}

// ReadFile reads the named file and returns the contents.
// A successful call returns err == nil, not err == EOF.
// Because ReadFile reads the whole file, it does not treat an EOF from Read
// as an error to be reported.
func ReadFile(f *os.File) ([]byte, error) {
	var size int
	if info, err := f.Stat(); err == nil {
		size64 := info.Size()
		if int64(int(size64)) == size64 {
			size = int(size64)
		}
	}
	size++ // one byte for final read at EOF

	// If a file claims a small Size, read at least 512 bytes.
	// In particular, files in Linux's /proc claim Size 0 but
	// then do not work right if read in small pieces,
	// so an initial read of 1 byte would not work correctly.
	if size < 512 {
		size = 512
	}

	data := make([]byte, 0, size)
	for {
		if len(data) >= cap(data) {
			d := append(data[:cap(data)], 0)
			data = d[:len(data)]
		}
		n, err := f.Read(data[len(data):cap(data)])
		data = data[:len(data)+n]
		if err != nil {
			if err == io.EOF {
				err = nil
			}
			return data, err
		}
	}
}

func (f *JSONLogFile) WriteJSON(data []byte) error {
	if f.F == nil {
		return nil
	}

	f.Lock()
	defer f.Unlock()

	_, _ = f.F.Seek(-2, io.SeekEnd) // \n]
	var err0 error

	if !f.HasRows {
		f.HasRows = true
		_, err0 = f.F.WriteString("\n")
	} else {
		_, err0 = f.F.WriteString(",\n")
	}
	_, err1 := f.F.Write(data)
	_, err2 := f.F.WriteString("\n]")
	return multierr.Combine(err0, err1, err2)
}

func (f JSONLogFile) IsDry() bool { return f.Dry }

func (f *JSONLogFile) Close() error {
	if f.F == nil {
		return nil
	}

	f.Lock()
	defer f.Unlock()
	f.Closed = true

	compress := false
	if stat, err := f.F.Stat(); err == nil && stat.Size() > 3 {
		compress = true
	}

	iox.Close(f.F)

	if compress {
		_ = gzipFile(f.Name)
	}

	return nil
}

func gzipFile(name string) error {
	f, err := os.Open(name)
	if err != nil {
		return err
	}

	reader := bufio.NewReader(f)
	content, err := ioutil.ReadAll(reader)
	if err != nil {
		return err
	}

	if f, err = os.Create(name + ".gz"); err != nil {
		return err
	}
	w := gzip.NewWriter(f)
	_, _ = w.Write(content)
	if err := w.Close(); err == nil {
		_ = f.Close()
		_ = os.Remove(name)
	}

	return err
}

func NewFeatures(features ...string) Features {
	m := make(Features)
	m.Setup(features)
	return m
}

// Features defines a feature map.
type Features map[string]string

// Setup sets up a feature map by features string, which separates feature names by comma.
func (f *Features) Setup(featuresArr []string) {
	for _, features := range featuresArr {
		for k, v := range ss.SplitToMap(features, ":=", ",") {
			(*f)[strings.ToLower(k)] = v
		}
	}
}

func (f *Features) IsNop() bool { return f.Has("nop") }

// Get gets the feature map contains a features.
func (f *Features) Get(feature string) string {
	s, _ := (*f)[strings.ToLower(feature)]
	return s
}

// Has tells the feature map contains a features.
func (f *Features) Has(feature string) bool {
	_, ok := (*f)[strings.ToLower(feature)]
	return ok
}

// HasAny tells the feature map contains any of the features.
func (f *Features) HasAny(features ...string) bool {
	for _, feature := range features {
		if f.Has(feature) {
			return true
		}
	}

	return false
}

type WidthHeight struct {
	W, H int
}

func (h WidthHeight) WidthPx() string  { return fmt.Sprintf("%dpx", h.W) }
func (h WidthHeight) HeightPx() string { return fmt.Sprintf("%dpx", h.H) }

func ParseWidthHeight(val string, defaultWidth, defaultHeight int) WidthHeight {
	wh := WidthHeight{
		W: defaultWidth,
		H: defaultHeight,
	}
	if val != "" {
		val = strings.ToLower(val)
		parts := strings.SplitN(val, "x", 2)
		if len(parts) == 2 {
			if v := ss.ParseInt(parts[0]); v > 0 {
				wh.W = v
			}
			if v := ss.ParseInt(parts[1]); v > 0 {
				wh.H = v
			}
		}
	}
	return wh
}

type GoroutineIncr struct {
	Up   int
	Dur  time.Duration
	Down int
}

func (i GoroutineIncr) Modifier() string {
	return ss.If(i.Up > 0, "max ", "")
}

func (i GoroutineIncr) IsEmpty() bool {
	return i.Up <= 0 && i.Down <= 0
}

// ParseGoIncr parse a GoIncr expressions like:
// 1. (empty) => GoroutineIncr{}
// 2. 0       => GoroutineIncr{}
// 3. 1       => GoroutineIncr{Up: 1}
// 4. 1:10s   => GoroutineIncr{Up: 1, Dur:10s}
// 5. 1:10s:1 => GoroutineIncr{Up: 1, Dur:10s, Down:1}
func ParseGoIncr(s string) GoroutineIncr {
	s = strings.TrimSpace(s)
	if s == "" {
		return GoroutineIncr{Up: 0, Dur: 0}
	}

	var err error
	parts := ss.Split(s, ss.WithIgnoreEmpty(true), ss.WithTrimSpace(true), ss.WithSeps(":"))
	v := ss.ParseInt(parts[0])
	ret := GoroutineIncr{Up: v, Dur: 0}
	if len(parts) >= 2 {
		ret.Dur, err = time.ParseDuration(parts[1])
		if err != nil {
			log.Printf("W! %s is invalid", s)
		}
	}
	if len(parts) >= 3 {
		ret.Down = ss.ParseInt(parts[2])
	}

	return ret
}

func MergeCodes(codes []string) string {
	n := 0
	last := ""
	merged := ""
	for _, code := range codes {
		if code != last {
			if last != "" {
				merged = mergeCodes(merged, n, last)
			}
			last = code
			n = 1
		} else {
			n++
		}
	}

	if n > 0 {
		merged = mergeCodes(merged, n, last)
	}

	return merged
}

func mergeCodes(merged string, n int, last string) string {
	if merged != "" {
		merged += ","
	}
	if n > 1 {
		merged += fmt.Sprintf("%sx%d", last, n)
	} else {
		merged += fmt.Sprintf("%s", last)
	}
	return merged
}
